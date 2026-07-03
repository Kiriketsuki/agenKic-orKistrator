extends Node2D
## TowerManager — fisheye layout engine, floor ordering, scroll/zoom, signal routing.

signal agent_panel_requested(agent_id: String)

const FLOOR_SCENE: PackedScene = preload("res://scenes/floor_scene.tscn")
const FOCUSED_SCALE: float = 1.0
const ADJACENT_SCALE: float = 0.4
const ZOOM_MIN: float = 0.5
const ZOOM_MAX: float = 2.0
const ZOOM_STEP: float = 0.1
const MAX_QUEUE_SIZE: int = 2
const BASE_FLOOR_WIDTH: float = 280.0
const BASE_FLOOR_HEIGHT: float = 40.0
const BASE_TOWER_RADIUS: float = 40.0

@export var config_path: String = "res://config/tower.json"

var _config: TowerConfig
var _floors: Array[Node2D] = []  # ordered bottom to top
var _focused_index: int = 0
var _agent_assignments: Dictionary = {}  # agent_id → {floor: String, edge: int}
var _scroll_tween: Tween = null
var _fisheye_tween: Tween = null
var _is_overscrolling: bool = false
var _input_queue: Array[int] = []
var _floor_spacing: float = 50.0
var _floor_width: float = BASE_FLOOR_WIDTH
var _floor_height: float = BASE_FLOOR_HEIGHT
var _tower_radius: float = BASE_TOWER_RADIUS
var _master_region: Rect2 = Rect2()

@onready var _floors_container: Node2D = $FloorsContainer
@onready var _camera: Camera2D = $Camera
@onready var _tower_exterior: Node2D = $TowerExterior


func _ready() -> void:
	_config = TowerConfig.from_file(config_path)
	get_viewport().size_changed.connect(_on_viewport_size_changed)
	_spawn_permanent_floors()
	_recalculate_layout_metrics()
	_layout_floors()
	_update_tower_frame()
	_apply_fisheye_layout()
	_sync_tower_exterior()
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
	elif event.is_action_pressed("scroll_up"):
		_scroll_focus(1)
		get_viewport().set_input_as_handled()
	elif event.is_action_pressed("scroll_down"):
		_scroll_focus(-1)
		get_viewport().set_input_as_handled()
	elif event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed:
			if mb.button_index == MOUSE_BUTTON_WHEEL_UP:
				if mb.ctrl_pressed:
					_zoom(-ZOOM_STEP)
				else:
					_scroll_focus(1)
				get_viewport().set_input_as_handled()
			elif mb.button_index == MOUSE_BUTTON_WHEEL_DOWN:
				if mb.ctrl_pressed:
					_zoom(ZOOM_STEP)
				else:
					_scroll_focus(-1)
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
	instance.agent_clicked.connect(func(agent_id: String) -> void:
		agent_panel_requested.emit(agent_id)
	)
	_floors_container.add_child(instance)
	return instance


## Sets absolute Y positions for all floors. Call after any change to _floors.
func _layout_floors() -> void:
	for i: int in range(_floors.size()):
		_floors[i].position = Vector2(0.0, i * -_floor_spacing)
		if _floors[i].has_method("set_floor_dimensions"):
			_floors[i].set_floor_dimensions(_floor_width, _floor_height)


## Tweens scale and opacity of all floors based on distance from _focused_index.
func _apply_fisheye_layout() -> void:
	if _fisheye_tween:
		_fisheye_tween.kill()
	_fisheye_tween = create_tween().set_parallel(true)
	for i: int in range(_floors.size()):
		var floor_node: Node2D = _floors[i]
		var distance: int = absi(i - _focused_index)
		var target_scale: Vector2
		var target_alpha: float
		var show_interior: bool
		if distance == 0:
			target_scale = Vector2(FOCUSED_SCALE, FOCUSED_SCALE)
			target_alpha = 1.0 if floor_node.get_floor_state() != floor_node.FloorState.LINGERING else 0.5
			show_interior = true
		elif distance == 1:
			target_scale = Vector2(ADJACENT_SCALE, ADJACENT_SCALE)
			target_alpha = 0.7
			show_interior = true
		else:
			target_scale = Vector2(ADJACENT_SCALE * 0.6, ADJACENT_SCALE * 0.6)
			target_alpha = 0.4
			show_interior = false
		_fisheye_tween.tween_property(floor_node, "scale", target_scale, 0.2).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)
		_fisheye_tween.tween_property(floor_node, "modulate:a", target_alpha, 0.2).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)
		floor_node.set_show_interior(show_interior)


func _scroll_focus(direction: int) -> void:
	if _is_overscrolling or _floors.is_empty():
		return
	var new_index: int = _focused_index + direction
	if new_index < 0 or new_index >= _floors.size():
		_input_queue.clear()
		_elastic_overscroll(direction)
		return
	if _scroll_tween != null and _scroll_tween.is_running():
		if _input_queue.size() < MAX_QUEUE_SIZE:
			_input_queue.append(direction)
		return
	_focused_index = new_index
	_do_scroll_tween()


func _do_scroll_tween() -> void:
	if _scroll_tween:
		_scroll_tween.kill()
	_scroll_tween = create_tween()
	var target_y: float = _focused_index * -_floor_spacing
	_scroll_tween.tween_property(_camera, "position:y", target_y, 0.3).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	_scroll_tween.tween_callback(_on_scroll_tween_finished)
	_apply_fisheye_layout()


func _on_scroll_tween_finished() -> void:
	if not _input_queue.is_empty():
		var next_direction: int = _input_queue.pop_front()
		_scroll_focus(next_direction)


func _elastic_overscroll(direction: int) -> void:
	if _is_overscrolling:
		return
	_is_overscrolling = true
	var original_y: float = _focused_index * -_floor_spacing
	var overshoot_y: float = original_y + (direction * _floor_spacing * -0.5)
	var tween: Tween = create_tween()
	tween.tween_property(_camera, "position:y", overshoot_y, 0.15).set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_OUT)
	tween.tween_property(_camera, "position:y", original_y, 0.2).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	tween.tween_callback(func() -> void: _is_overscrolling = false)


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
	var slide_offset: float = maxf(_master_region.size.x * 0.18, 320.0) * (-direction)
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
	_layout_floors()
	_apply_fisheye_layout()
	_update_tower_frame()
	_sync_tower_exterior()


func _on_floor_removed(floor_name: String) -> void:
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			if floor_node.is_permanent:
				return
			floor_node.begin_linger(_config.linger_duration_sec)
			floor_node.tree_exiting.connect(func() -> void:
				_floors.erase(floor_node)
				_focused_index = clampi(_focused_index, 0, maxi(_floors.size() - 1, 0))
				_layout_floors()
				_update_tower_frame()
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


func set_master_region(region: Rect2) -> void:
	_master_region = region
	_recalculate_layout_metrics()
	_layout_floors()
	_update_tower_frame()
	_sync_tower_exterior()


func _on_viewport_size_changed() -> void:
	set_master_region(Rect2(Vector2.ZERO, get_viewport_rect().size))


func _recalculate_layout_metrics() -> void:
	if _master_region.size == Vector2.ZERO:
		_master_region = Rect2(Vector2.ZERO, get_viewport_rect().size)
	var viewport_size: Vector2 = _master_region.size
	_floor_width = clampf(viewport_size.x * 0.24, BASE_FLOOR_WIDTH, 520.0)
	_floor_height = clampf(_floor_width * 0.16, BASE_FLOOR_HEIGHT, 88.0)
	_floor_spacing = clampf(viewport_size.y * 0.1, 50.0, 128.0)
	_tower_radius = clampf(_floor_width * 0.14, BASE_TOWER_RADIUS, 88.0)


func _update_tower_frame() -> void:
	var center: Vector2 = _master_region.position + (_master_region.size * 0.5)
	position = center
	_camera.position = Vector2(0.0, _focused_index * -_floor_spacing)


func _sync_tower_exterior() -> void:
	_tower_exterior.configure(_config.polygon_sides, _floors.size() * _floor_spacing, _tower_radius)
