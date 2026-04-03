extends Node2D
## FloorScene — a single floor in the tower. Manages edge rotation,
## AgentCharacter spawning, and the ephemeral lifecycle state machine.

const AGENT_CHARACTER_SCENE: PackedScene = preload("res://scenes/agent_character.tscn")

enum FloorState { ACTIVE, LINGERING, DISSOLVING }

@export var floor_name: String = ""
@export var floor_label: String = ""
@export var is_permanent: bool = false
@export var polygon_sides: int = 6

var _state: FloorState = FloorState.ACTIVE
var _active_edge: int = 0
## Each entry: {agent_id, edge_index, character_class, state}
var _agent_slots: Array[Dictionary] = []
var _linger_timer: float = 0.0
var _linger_duration: float = 30.0

@onready var _background: Polygon2D = $Background
@onready var _interior: Node2D = $Interior
@onready var _agent_slots_node: Node2D = $AgentSlots
@onready var _name_label: Label = $NameLabel


func _ready() -> void:
	_name_label.text = floor_label if floor_label != "" else floor_name
	_rebuild_background()
	_rebuild_interior()


func _process(delta: float) -> void:
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
	var w: float = 280.0
	var h: float = 40.0
	_background.polygon = PackedVector2Array([
		Vector2(-w / 2.0, -h / 2.0),
		Vector2(w / 2.0, -h / 2.0),
		Vector2(w / 2.0, h / 2.0),
		Vector2(-w / 2.0, h / 2.0),
	])
	_background.color = Color(0.18, 0.22, 0.18, 1.0)  # dark stone green


func _rebuild_interior() -> void:
	for child: Node in _agent_slots_node.get_children():
		child.queue_free()
	var edge_agents: Array[Dictionary] = []
	for slot: Dictionary in _agent_slots:
		if slot["edge_index"] == _active_edge:
			edge_agents.append(slot)
	if edge_agents.is_empty():
		return
	var edge_width: float = EdgeLayout.edge_width_for_polygon(polygon_sides, 280.0)
	var positions: Array[Vector2] = EdgeLayout.calculate_positions(edge_agents.size(), edge_width)
	# Offset converts EdgeLayout's top-left corner to AgentCharacter's center origin.
	var center_offset: Vector2 = Vector2(EdgeLayout.DESK_WIDTH / 2.0, EdgeLayout.DESK_HEIGHT / 2.0)
	for i: int in range(edge_agents.size()):
		var slot: Dictionary = edge_agents[i]
		var char_node: AgentCharacter = AGENT_CHARACTER_SCENE.instantiate() as AgentCharacter
		char_node.agent_id = slot["agent_id"]
		_agent_slots_node.add_child(char_node)
		char_node.position = positions[i] + center_offset
		char_node.set_character_class(slot.get("character_class", "apprentice"))
		char_node.set_animation_state(slot.get("state", "idle"))
		char_node.set_provider(slot.get("provider", ""))
