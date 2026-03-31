extends Node2D
## FloorScene — a single floor in the tower. Manages edge rotation,
## agent slot rendering, and the ephemeral lifecycle state machine.

enum FloorState { ACTIVE, LINGERING, DISSOLVING }

@export var floor_name: String = ""
@export var floor_label: String = ""
@export var is_permanent: bool = false
@export var polygon_sides: int = 6

var _state: FloorState = FloorState.ACTIVE
var _active_edge: int = 0
var _agent_slots: Array[Dictionary] = []  # [{agent_id, edge_index}]
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


func add_agent_slot(agent_id: String, edge_index: int) -> void:
	for slot: Dictionary in _agent_slots:
		if slot["agent_id"] == agent_id:
			return
	_agent_slots.append({"agent_id": agent_id, "edge_index": edge_index})
	if edge_index == _active_edge:
		_rebuild_interior()


func remove_agent_slot(agent_id: String) -> void:
	_agent_slots = _agent_slots.filter(func(s: Dictionary) -> bool: return s["agent_id"] != agent_id)
	_rebuild_interior()


func get_agent_count_on_edge(edge: int) -> int:
	var count: int = 0
	for slot: Dictionary in _agent_slots:
		if slot["edge_index"] == edge:
			count += 1
	return count


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
	var slot_width: float = 16.0
	var slot_height: float = 20.0
	var spacing: float = 4.0
	var total_width: float = edge_agents.size() * (slot_width + spacing) - spacing
	var start_x: float = -total_width / 2.0
	for i: int in range(edge_agents.size()):
		var rect := ColorRect.new()
		rect.size = Vector2(slot_width, slot_height)
		rect.position = Vector2(start_x + i * (slot_width + spacing), -slot_height / 2.0)
		rect.color = Color(0.55, 0.45, 0.25, 1.0)  # warm amber for agent desks
		_agent_slots_node.add_child(rect)


func set_show_interior(visible_flag: bool) -> void:
	_interior.visible = visible_flag
	_agent_slots_node.visible = visible_flag
	_name_label.visible = visible_flag
