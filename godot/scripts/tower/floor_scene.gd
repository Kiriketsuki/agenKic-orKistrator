extends Node2D
## FloorScene — a single floor in the tower. Manages edge rotation,
## AgentCharacter spawning, and the ephemeral lifecycle state machine.

signal agent_clicked(agent_id: String)
signal agent_right_clicked(agent_id: String)
signal agent_hovered(agent_id: String)
signal agent_unhovered(agent_id: String)

const AGENT_CHARACTER_SCENE: PackedScene = preload("res://scenes/agent_character.tscn")
const WALL_TEXTURE: Texture2D = preload("res://assets/tiles/sliced/stone_wall.png")
const FLOOR_PLANE_TEXTURE: Texture2D = preload("res://assets/tiles/sliced/stone_floor.png")
const WINDOW_TEXTURE: Texture2D = preload("res://assets/sprites/deco/window.png")
const TORCH_TEXTURE: Texture2D = preload("res://assets/sprites/deco/torch.png")
const PROPS_TEXTURE: Texture2D = preload("res://assets/props/workstations.png")
const NAMEPLATE_TEXTURE: Texture2D = preload("res://assets/ui/nameplate_frame.png")

## 3/4-view depth: how far the walkable floor plane extends toward the
## viewer below the wall, and how much wider it gets at the front edge.
const PLANE_DEPTH: float = 26.0
const PLANE_FLARE: float = 18.0
const PROP_COUNT: int = 8

## Active states — matches BridgeData.AgentData's doc-comment vocabulary.
## Idle and crashed agents read as dim on the minimap/badges.
const ACTIVE_STATES: Array[String] = ["assigned", "working", "reporting"]

enum FloorState { ACTIVE, LINGERING, DISSOLVING }

@export var floor_name: String = ""
@export var floor_label: String = ""
@export var is_permanent: bool = false
@export var polygon_sides: int = 6

var _state: FloorState = FloorState.ACTIVE
var _active_edge: int = 0
var _floor_width: float = 280.0
var _floor_height: float = 40.0
## Each entry: {agent_id, edge_index, character_class, state}
var _agent_slots: Array[Dictionary] = []
var _linger_timer: float = 0.0
var _linger_duration: float = 30.0
var _dressing: Node2D = null
var _torches: Array[Sprite2D] = []
var _flicker_time: float = 0.0

@onready var _background: Polygon2D = $Background
@onready var _interior: Node2D = $Interior
@onready var _agent_slots_node: Node2D = $AgentSlots
@onready var _name_label: Label = $NameLabel


func _ready() -> void:
	_name_label.text = floor_label if floor_label != "" else floor_name
	# Parchment nameplate behind the floor name (T22 UI texture).
	var plate := NinePatchRect.new()
	plate.texture = NAMEPLATE_TEXTURE
	plate.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	plate.patch_margin_left = 6
	plate.patch_margin_top = 4
	plate.patch_margin_right = 6
	plate.patch_margin_bottom = 4
	plate.position = Vector2(-10.0, -9.0)
	plate.size = Vector2(100.0, 30.0)
	plate.show_behind_parent = true
	_name_label.add_child(plate)
	_name_label.add_theme_color_override("font_color", Color(0.18, 0.16, 0.09))
	_rebuild_background()
	_rebuild_interior()


func _process(delta: float) -> void:
	_flicker_time += delta
	for i: int in range(_torches.size()):
		var torch: Sprite2D = _torches[i]
		if is_instance_valid(torch):
			var flicker: float = 0.82 + 0.18 * sin(_flicker_time * 9.0 + i * 2.1)
			torch.modulate = Color(1.0, flicker, flicker * 0.85, 1.0)
	if _state == FloorState.LINGERING:
		_linger_timer -= delta
		if _linger_timer <= 0.0:
			_state = FloorState.DISSOLVING
			queue_free()


func get_floor_state() -> FloorState:
	return _state


func get_active_edge() -> int:
	return _active_edge


func set_active_edge(edge: int) -> void:
	_active_edge = edge % polygon_sides
	_rebuild_interior()


func begin_linger(duration: float) -> void:
	if is_permanent:
		return
	_state = FloorState.LINGERING
	_linger_duration = duration
	_linger_timer = duration
	modulate.a = 0.5


func reactivate() -> void:
	_state = FloorState.ACTIVE
	_linger_timer = 0.0
	modulate.a = 1.0


func set_floor_dimensions(width: float, height: float) -> void:
	_floor_width = width
	_floor_height = height
	if is_inside_tree():
		_rebuild_background()
		_rebuild_interior()


func add_agent_slot(agent_id: String, edge_index: int, character_class: String = "apprentice", provider: String = "") -> void:
	for slot: Dictionary in _agent_slots:
		if slot["agent_id"] == agent_id:
			return
	_agent_slots.append({
		"agent_id": agent_id,
		"edge_index": edge_index,
		"character_class": character_class,
		"state": "idle",
		"provider": provider,
	})
	if edge_index == _active_edge:
		_rebuild_interior()


func remove_agent_slot(agent_id: String) -> void:
	_agent_slots = _agent_slots.filter(
		func(s: Dictionary) -> bool: return s["agent_id"] != agent_id
	)
	_rebuild_interior()


## Total agents assigned to this floor, across all edges.
func get_agent_count() -> int:
	return _agent_slots.size()


## Agents on this floor currently in an active state (assigned/working/reporting).
func get_active_count() -> int:
	var count: int = 0
	for slot: Dictionary in _agent_slots:
		if slot.get("state", "idle") in ACTIVE_STATES:
			count += 1
	return count


func get_agent_count_on_edge(edge: int) -> int:
	var count: int = 0
	for slot: Dictionary in _agent_slots:
		if slot["edge_index"] == edge:
			count += 1
	return count


## Update the stored state for an agent and propagate to its live node if visible.
func update_agent_state(agent_id: String, state: String) -> void:
	for slot: Dictionary in _agent_slots:
		if slot["agent_id"] == agent_id:
			slot["state"] = state
			break
	var char_node: AgentCharacter = get_agent_character(agent_id)
	if char_node:
		char_node.set_animation_state(state)


## Return the live AgentCharacter node for an agent, or null if not on the active edge.
func get_agent_character(agent_id: String) -> AgentCharacter:
	for child: Node in _agent_slots_node.get_children():
		if child is AgentCharacter and (child as AgentCharacter).agent_id == agent_id:
			return child as AgentCharacter
	return null


func set_show_interior(visible_flag: bool) -> void:
	_interior.visible = visible_flag
	_agent_slots_node.visible = visible_flag
	_name_label.visible = visible_flag


func _rebuild_background() -> void:
	_background.polygon = PackedVector2Array([
		Vector2(-_floor_width / 2.0, -_floor_height / 2.0),
		Vector2(_floor_width / 2.0, -_floor_height / 2.0),
		Vector2(_floor_width / 2.0, _floor_height / 2.0),
		Vector2(-_floor_width / 2.0, _floor_height / 2.0),
	])
	_background.texture = WALL_TEXTURE
	_background.texture_repeat = CanvasItem.TEXTURE_REPEAT_ENABLED
	_background.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	# UVs in texture pixels so the 16px tile repeats across the wall.
	_background.uv = _background.polygon
	_background.color = Color(0.82, 0.82, 0.88, 1.0)  # back wall sits in shadow
	_rebuild_dressing()


## Rebuilds the decorative layer: cornice/base trim, glowing windows, and
## flickering torches. Sits directly above the brick background, behind agents.
func _rebuild_dressing() -> void:
	if _dressing != null and is_instance_valid(_dressing):
		_dressing.queue_free()
	_torches.clear()
	_dressing = Node2D.new()
	add_child(_dressing)
	move_child(_dressing, _background.get_index() + 1)

	var half_w: float = _floor_width / 2.0
	var half_h: float = _floor_height / 2.0

	# Cornice (light stone strip along the top).
	var cornice := Polygon2D.new()
	cornice.polygon = PackedVector2Array([
		Vector2(-half_w - 4.0, -half_h - 3.0), Vector2(half_w + 4.0, -half_h - 3.0),
		Vector2(half_w + 4.0, -half_h + 1.0), Vector2(-half_w - 4.0, -half_h + 1.0),
	])
	cornice.color = Color(0.42, 0.44, 0.52, 1.0)
	_dressing.add_child(cornice)

	# 3/4-view walkable floor plane: a trapezoid extending down toward the
	# viewer, wider at the front edge, so the room reads as seen from a
	# slight top-down angle instead of a flat billboard.
	var plane := Polygon2D.new()
	plane.polygon = PackedVector2Array([
		Vector2(-half_w, half_h),
		Vector2(half_w, half_h),
		Vector2(half_w + PLANE_FLARE, half_h + PLANE_DEPTH),
		Vector2(-half_w - PLANE_FLARE, half_h + PLANE_DEPTH),
	])
	plane.texture = FLOOR_PLANE_TEXTURE
	plane.texture_repeat = CanvasItem.TEXTURE_REPEAT_ENABLED
	plane.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	plane.uv = plane.polygon
	plane.color = Color(1.08, 1.08, 1.05, 1.0)  # lit floor, brighter than wall
	_dressing.add_child(plane)

	# Front-edge shadow line under the floor plane.
	var plinth := Polygon2D.new()
	plinth.polygon = PackedVector2Array([
		Vector2(-half_w - PLANE_FLARE, half_h + PLANE_DEPTH - 1.0),
		Vector2(half_w + PLANE_FLARE, half_h + PLANE_DEPTH - 1.0),
		Vector2(half_w + PLANE_FLARE, half_h + PLANE_DEPTH + 3.0),
		Vector2(-half_w - PLANE_FLARE, half_h + PLANE_DEPTH + 3.0),
	])
	plinth.color = Color(0.10, 0.10, 0.14, 1.0)
	_dressing.add_child(plinth)

	# Windows spaced across the wall, skipping the center where the label sits.
	var window_spacing: float = 90.0
	var x: float = -half_w + 45.0
	while x < half_w - 30.0:
		if absf(x) > 60.0:
			var win := Sprite2D.new()
			win.texture = WINDOW_TEXTURE
			win.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
			win.scale = Vector2(2.0, 2.0)
			win.position = Vector2(x, -half_h * 0.25)
			_dressing.add_child(win)
		x += window_spacing

	# Torches flanking the floor near each end.
	for tx: float in [-half_w + 18.0, half_w - 18.0]:
		var torch := Sprite2D.new()
		torch.texture = TORCH_TEXTURE
		torch.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
		torch.scale = Vector2(2.0, 2.0)
		torch.position = Vector2(tx, -half_h * 0.15)
		_dressing.add_child(torch)
		_torches.append(torch)


func _rebuild_interior() -> void:
	for child: Node in _agent_slots_node.get_children():
		child.queue_free()
	var edge_agents: Array[Dictionary] = []
	for slot: Dictionary in _agent_slots:
		if slot["edge_index"] == _active_edge:
			edge_agents.append(slot)
	if edge_agents.is_empty():
		return
	var edge_width: float = EdgeLayout.edge_width_for_polygon(polygon_sides, _floor_width)
	var positions: Array[Vector2] = EdgeLayout.calculate_positions(edge_agents.size(), edge_width)
	# Characters stand ON the 3/4-view floor plane: feet land mid-plane so
	# the down-angle reads. Only x comes from EdgeLayout; y is fixed.
	var feet_y: float = _floor_height / 2.0 + PLANE_DEPTH * 0.55
	var char_y: float = feet_y - EdgeLayout.DESK_HEIGHT / 2.0
	for i: int in range(edge_agents.size()):
		var slot: Dictionary = edge_agents[i]
		var char_node: AgentCharacter = AGENT_CHARACTER_SCENE.instantiate() as AgentCharacter
		char_node.agent_id = slot["agent_id"]

		# Workstation prop behind the agent, back against the wall base.
		var prop := Sprite2D.new()
		var atlas := AtlasTexture.new()
		atlas.atlas = PROPS_TEXTURE
		var prop_idx: int = absi((slot["agent_id"] as String).hash() / 7) % PROP_COUNT
		atlas.region = Rect2(prop_idx * 16, 0, 16, 16)
		prop.texture = atlas
		prop.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
		prop.scale = Vector2(2.5, 2.5)
		prop.position = Vector2(
			positions[i].x + EdgeLayout.DESK_WIDTH / 2.0 - 26.0,
			_floor_height / 2.0 - 4.0
		)
		_agent_slots_node.add_child(prop)

		_agent_slots_node.add_child(char_node)
		char_node.position = Vector2(positions[i].x + EdgeLayout.DESK_WIDTH / 2.0, char_y)
		char_node.set_character_class(slot.get("character_class", "apprentice"))
		char_node.set_animation_state(slot.get("state", "idle"))
		char_node.set_provider(slot.get("provider", ""))
		char_node.character_clicked.connect(func(agent_id: String) -> void:
			agent_clicked.emit(agent_id)
		)
		char_node.character_right_clicked.connect(func(agent_id: String) -> void:
			agent_right_clicked.emit(agent_id)
		)
		char_node.character_hovered.connect(func(agent_id: String) -> void:
			agent_hovered.emit(agent_id)
		)
		char_node.character_unhovered.connect(func(agent_id: String) -> void:
			agent_unhovered.emit(agent_id)
		)
