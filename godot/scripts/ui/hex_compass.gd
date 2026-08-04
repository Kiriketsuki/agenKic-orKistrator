extends Control
## HexCompass — a small n-gon diagram in the bottom-left corner that shows
## which edge of the focused floor faces the viewer.
##
## PARITY (Phase 4) — edge rotation moves only the floor contents. When both
## the old edge and the new edge are empty, nothing on the slab changes, so the
## turn reads as a dead click. This widget marks the turn, so the action is
## never silent. The side count comes straight from the floor, so the diagram
## also doubles as the composite-load readout that T15 drives.
##
## Phase 6 replaced this with the pagination dots (edge_compass.gd). The user
## asked for both, so this diagram is restored alongside the dots. The dots
## are the click target, this compass stays display-only.
##
## The n-gon itself NEVER rotates. A compass is a fixed frame of reference,
## so the polygon stays put and only the highlighted edge walks around it.
##
## Read-only toward the tower: it calls the public TowerManager API and walks
## FloorsContainer without mutating anything.

const ACTIVE_COLOR: Color = Color("#FBB13C")
const IDLE_COLOR: Color = Color("#363a4f")
const FILL_COLOR: Color = Color(0.08, 0.09, 0.12, 0.72)
const HUB_COLOR: Color = Color("#2a2f45")

@export var tower_path: NodePath = NodePath("/root/Main/Tower")
## Circumradius of the drawn n-gon, in window pixels.
@export var radius: float = 56.0

var _tower: Node = null
var _sides: int = 6
var _active_edge: int = 0


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	custom_minimum_size = Vector2(144.0, 144.0)
	_tower = get_node_or_null(tower_path)
	if _tower == null:
		set_process(false)
		return
	_sides = _read_sides()
	_active_edge = _read_active_edge()


func _process(_delta: float) -> void:
	if _tower == null or not is_instance_valid(_tower):
		return
	var sides: int = _read_sides()
	var edge: int = _read_active_edge()
	if sides != _sides or edge != _active_edge:
		_sides = sides
		_active_edge = edge
		queue_redraw()


## Reads the focused floor. Returns null when the tower has no floors yet.
func _focused_floor() -> Node:
	var container: Node = _tower.get_node_or_null("FloorsContainer")
	if container == null:
		return null
	var floors: Array = container.get_children()
	if floors.is_empty():
		return null
	var focus: int = _tower.get_focus_index()
	if focus < 0 or focus >= floors.size():
		return null
	return floors[focus]


func _read_sides() -> int:
	var floor_node: Node = _focused_floor()
	if floor_node == null:
		return _sides
	return maxi(3, int(floor_node.polygon_sides))


func _read_active_edge() -> int:
	var floor_node: Node = _focused_floor()
	if floor_node == null or not floor_node.has_method("get_active_edge"):
		return _active_edge
	return int(floor_node.get_active_edge())


func _draw() -> void:
	if _sides < 3:
		return
	var centre := Vector2(size.x / 2.0, size.y / 2.0)
	# Fixed orientation: edge 0 sits at the bottom, and the highlight walks
	# from edge to edge while the polygon stays put.
	var base: float = PI / 2.0 - TAU / float(_sides) / 2.0
	var points: PackedVector2Array = PackedVector2Array()
	for i: int in range(_sides):
		var theta: float = base + TAU * float(i) / float(_sides)
		points.append(centre + Vector2(cos(theta), sin(theta)) * radius)
	draw_colored_polygon(points, FILL_COLOR)
	for i: int in range(_sides):
		var a: Vector2 = points[i]
		var b: Vector2 = points[(i + 1) % _sides]
		var is_active: bool = i == posmod(_active_edge, _sides)
		draw_line(a, b, ACTIVE_COLOR if is_active else IDLE_COLOR, 6.0 if is_active else 3.0)
	draw_circle(centre, 4.0, HUB_COLOR)
