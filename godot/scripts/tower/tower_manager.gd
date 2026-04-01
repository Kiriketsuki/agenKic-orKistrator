extends Node2D
## TowerManager — fisheye layout engine, floor ordering, scroll/zoom, signal routing.

const FLOOR_SCENE: PackedScene = preload("res://scenes/floor_scene.tscn")
const FLOOR_SPACING: float = 50.0
const FOCUSED_SCALE: float = 1.0
const ADJACENT_SCALE: float = 0.4
const ZOOM_MIN: float = 0.5
const ZOOM_MAX: float = 2.0
const ZOOM_STEP: float = 0.1

@export var config_path: String = "res://config/tower.json"

var _config: TowerConfig
var _floors: Array[Node2D] = []  # ordered bottom to top
var _focused_index: int = 0
var _agent_assignments: Dictionary = {}  # agent_id → {floor: String, edge: int}

@onready var _floors_container: Node2D = $FloorsContainer
@onready var _camera: Camera2D = $Camera
@onready var _tower_exterior: Node2D = $TowerExterior


func _ready() -> void:
	_config = TowerConfig.from_file(config_path)
	_spawn_permanent_floors()
	_apply_fisheye_layout()
	_tower_exterior.configure(_config.polygon_sides, _floors.size() * FLOOR_SPACING)
	var bridge: Node = Engine.get_singleton("BridgeManager") if Engine.has_singleton("BridgeManager") else get_node_or_null("/root/BridgeManager")
	if bridge:
		bridge.connect("floor_created", _on_floor_created)
		bridge.connect("floor_removed", _on_floor_removed)
		bridge.connect("agent_registered", _on_agent_registered)
		bridge.connect("agent_state_changed", _on_agent_state_changed)
		bridge.connect("agent_deregistered", _on_agent_deregistered)
		bridge.connect("agent_output", _on_agent_output)
		bridge.connect("connection_status_changed", _on_connection_status_changed)


func _unhandled_input(event: InputEvent) -> void:
	if event.is_action_pressed("rotate_left"):
		_rotate_focused_edge(-1)
		get_viewport().set_input_as_handled()
	elif event.is_action_pressed("rotate_right"):
		_rotate_focused_edge(1)
		get_viewport().set_input_as_handled()
	elif event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed:
			if mb.button_index == MOUSE_BUTTON_WHEEL_UP:
				if mb.ctrl_pressed:
					_zoom(-ZOOM_STEP)
				else:
					_scroll_focus(-1)
				get_viewport().set_input_as_handled()
			elif mb.button_index == MOUSE_BUTTON_WHEEL_DOWN:
				if mb.ctrl_pressed:
					_zoom(ZOOM_STEP)
				else:
					_scroll_focus(1)
				get_viewport().set_input_as_handled()


func _spawn_permanent_floors() -> void:
	for floor_def: Dictionary in _config.permanent_floors:
		var floor_scene: Node2D = _create_floor(
			floor_def.get("name", ""),
			floor_def.get("label", ""),
			true
		)
		_floors.append(floor_scene)
	if not _floors.is_empty():
		_focused_index = 0


func _create_floor(floor_name: String, label: String, permanent: bool) -> Node2D:
	var instance: Node2D = FLOOR_SCENE.instantiate()
	instance.floor_name = floor_name
	instance.floor_label = label
	instance.is_permanent = permanent
	instance.polygon_sides = _config.polygon_sides
	instance.set_meta("floor_name", floor_name)
	_floors_container.add_child(instance)
	return instance


func _apply_fisheye_layout() -> void:
	for i: int in range(_floors.size()):
		var floor_node: Node2D = _floors[i]
		var distance: int = absi(i - _focused_index)
		var y_pos: float = (i - _focused_index) * -FLOOR_SPACING
		floor_node.position = Vector2(0.0, y_pos)
		if distance == 0:
			floor_node.scale = Vector2(FOCUSED_SCALE, FOCUSED_SCALE)
			floor_node.modulate.a = 1.0 if floor_node.get_floor_state() != floor_node.FloorState.LINGERING else 0.5
			floor_node.set_show_interior(true)
		elif distance == 1:
			floor_node.scale = Vector2(ADJACENT_SCALE, ADJACENT_SCALE)
			floor_node.modulate.a = 0.7
			floor_node.set_show_interior(true)
		else:
			floor_node.scale = Vector2(ADJACENT_SCALE * 0.6, ADJACENT_SCALE * 0.6)
			floor_node.modulate.a = 0.4
			floor_node.set_show_interior(false)


func _scroll_focus(direction: int) -> void:
	var new_index: int = clampi(_focused_index + direction, 0, _floors.size() - 1)
	if new_index == _focused_index:
		return
	_focused_index = new_index
	_apply_fisheye_layout()


func _zoom(amount: float) -> void:
	var new_zoom: float = clampf(_camera.zoom.x + amount, ZOOM_MIN, ZOOM_MAX)
	_camera.zoom = Vector2(new_zoom, new_zoom)


func _rotate_focused_edge(direction: int) -> void:
	if _floors.is_empty() or _focused_index >= _floors.size():
		return
	var floor_node: Node2D = _floors[_focused_index]
	var current_edge: int = floor_node.get_active_edge()
	var new_edge: int = (current_edge + direction) % _config.polygon_sides
	if new_edge < 0:
		new_edge += _config.polygon_sides
	var old_x: float = floor_node.position.x
	var slide_offset: float = 320.0 * (-direction)
	var tween: Tween = create_tween()
	tween.tween_property(floor_node, "position:x", old_x + slide_offset, 0.15)
	tween.tween_callback(func() -> void:
		floor_node.set_active_edge(new_edge)
		floor_node.position.x = old_x - slide_offset
	)
	tween.tween_property(floor_node, "position:x", old_x, 0.15)


# --- Signal Handlers ---

# TODO(dynamic-floor): Two known cases where the idempotency guard at _on_agent_registered:169
# blocks recovery, to be resolved together in the dynamic floor lifecycle task:
# 1. Floor removal: non-permanent floors leave stale _agent_assignments entries when removed —
#    _on_floor_removed does not clean _agent_assignments, so reconnect re-registration is silently
#    dropped for agents that were on the removed floor.
# 2. Rapid deregister→re-register: _on_agent_deregistered defers _agent_assignments.erase() by
#    0.45s (exit animation window). If agent.registered fires for the same agent within that window
#    — e.g., an agent crash-restart under supervision — the guard at line 169 returns early and the
#    re-registration is permanently lost. The dropped SSE event is not re-emitted by the orchestrator.
func _on_floor_created(floor_data: BridgeData.FloorData) -> void:
	for existing: Node2D in _floors:
		if existing.get_meta("floor_name", "") == floor_data.name:
			return
	var floor_node: Node2D = _create_floor(floor_data.name, floor_data.name, floor_data.is_permanent)
	_floors.append(floor_node)
	_apply_fisheye_layout()
	_tower_exterior.configure(_config.polygon_sides, _floors.size() * FLOOR_SPACING)


func _on_floor_removed(floor_name: String) -> void:
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			if floor_node.is_permanent:
				return
			floor_node.begin_linger(_config.linger_duration_sec)
			floor_node.tree_exiting.connect(func() -> void:
				_floors.erase(floor_node)
				_focused_index = clampi(_focused_index, 0, maxi(_floors.size() - 1, 0))
				_apply_fisheye_layout()
			)
			return


func _on_agent_registered(agent_data: BridgeData.AgentData) -> void:
	if _agent_assignments.has(agent_data.id):
		return
	var floor_name: String = agent_data.floor_name
	if floor_name.is_empty() or not _has_floor(floor_name):
		floor_name = _floors[0].get_meta("floor_name", "main") if not _floors.is_empty() else "main"
	var edge: int = _find_best_edge_for_agent(floor_name)
	assign_agent_to_edge(agent_data.id, floor_name, edge, agent_data.character_class, agent_data.provider)


func _on_agent_state_changed(agent_id: String, _old_state: String, new_state: String, _task_id: String) -> void:
	var assignment: Dictionary = _agent_assignments.get(agent_id, {})
	if assignment.is_empty():
		return
	var floor_name: String = assignment.get("floor", "")
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			floor_node.update_agent_state(agent_id, new_state)
			return


func _on_agent_deregistered(agent_id: String) -> void:
	var assignment: Dictionary = _agent_assignments.get(agent_id, {})
	if assignment.is_empty():
		_agent_assignments.erase(agent_id)
		return
	var floor_name: String = assignment.get("floor", "")
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			var char_node: AgentCharacter = floor_node.get_agent_character(agent_id)
			if char_node:
				char_node.play_exit_animation()
				# Remove slot after exit animation (0.4 s) so rebuild doesn't cull the fading node.
				var timer: SceneTreeTimer = get_tree().create_timer(0.45)
				timer.timeout.connect(func() -> void:
					if is_instance_valid(floor_node):
						floor_node.remove_agent_slot(agent_id)
					_agent_assignments.erase(agent_id)
				)
			else:
				# Agent is on a non-active edge — remove immediately.
				floor_node.remove_agent_slot(agent_id)
				_agent_assignments.erase(agent_id)
			return
	_agent_assignments.erase(agent_id)


func _on_agent_output(chunk: BridgeData.AgentOutputChunk) -> void:
	var assignment: Dictionary = _agent_assignments.get(chunk.agent_id, {})
	if assignment.is_empty():
		return
	var floor_name: String = assignment.get("floor", "")
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			var char_node: AgentCharacter = floor_node.get_agent_character(chunk.agent_id)
			if char_node:
				char_node.receive_output(chunk)
			return


func _on_connection_status_changed(status: String) -> void:
	match status:
		"disconnected", "reconnecting":
			modulate = Color(0.6, 0.6, 0.7, 1.0)
		"connected":
			modulate = Color(1.0, 1.0, 1.0, 1.0)
			RuneFilter.reset_rate_limits()


# --- Agent Assignment ---

func assign_agent_to_edge(agent_id: String, floor_name: String, edge_index: int, character_class: String = "apprentice", provider: String = "") -> void:
	_agent_assignments[agent_id] = {"floor": floor_name, "edge": edge_index}
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			floor_node.add_agent_slot(agent_id, edge_index, character_class, provider)
			if floor_node.get_floor_state() == floor_node.FloorState.LINGERING:
				floor_node.reactivate()
			return


func _find_best_edge_for_agent(floor_name: String) -> int:
	var edge_counts: Dictionary = {}
	for i: int in range(_config.polygon_sides):
		edge_counts[i] = 0
	for existing_id: String in _agent_assignments:
		var assignment: Dictionary = _agent_assignments[existing_id]
		if assignment.get("floor", "") == floor_name:
			var e: int = assignment.get("edge", 0)
			edge_counts[e] = edge_counts.get(e, 0) + 1
	var min_edge: int = 0
	var min_count: int = 999
	for e: int in edge_counts:
		if edge_counts[e] < min_count:
			min_count = edge_counts[e]
			min_edge = e
	return min_edge


func _has_floor(floor_name: String) -> bool:
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			return true
	return false
