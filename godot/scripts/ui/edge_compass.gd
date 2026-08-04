extends Control
## EdgeCompass — the edge pagination dots under the focused floor.
##
## PARITY (Phase 6 section 3) — the demo shows one pip per edge below the
## tower, so the viewer reads at a glance where they look, where the agents
## are, and how many edges the room has. This node took over this file from
## the corner n-gon diagram. The user later asked for both indicators, so the
## diagram lives on as hex_compass.gd in the bottom-left corner and these
## dots stay the click target.
##
## Each pip is a flat-top hexagon, the Chrysaki status-pip shape. A click on a
## pip turns the room to that edge the short way round.
##
## The dots track the focused floor through get_global_transform_with_canvas(),
## the same projection WorldLabels and FloorBanner use, so the camera zoom never
## magnifies them.
##
## Read-only toward the tower, apart from the click, which goes through the
## public TowerManager rotate path.

## Gold. The edge the viewer looks at.
const ACTIVE_COLOR: Color = Color("#FBB13C")
## Emerald #1a8a6a and teal #197278 at an even mix, like the demo pip row. An
## edge that holds agents. The mix is written out because GDScript needs a
## constant expression here.
const AGENT_COLOR: Color = Color("#1a7e71")
## Outline only. An empty edge.
const EMPTY_COLOR: Color = Color("#363a4f")
const HEX_WIDTH: float = 20.0
const HEX_GAP: float = 12.0
## Screen-space offset from the floor origin, in window pixels. The row clears
## the slab bottom edge.
const ROW_OFFSET_Y: float = 232.0
const OUTLINE_WIDTH: float = 3.0

@export var tower_path: NodePath = NodePath("/root/Main/Tower")

var _tower: Node = null
var _sides: int = 6
var _active_edge: int = 0
## One entry per edge, true when that edge holds at least one agent.
var _occupied: Array[bool] = []


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_STOP
	_tower = get_node_or_null(tower_path)
	if _tower == null:
		set_process(false)
		return
	if _tower.has_signal("floors_changed"):
		_tower.connect("floors_changed", _rebuild)
	_rebuild()


func _process(_delta: float) -> void:
	if _tower == null or not is_instance_valid(_tower):
		return
	_rebuild()
	_track_floor()


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


## Pulls side count, active edge and per-edge occupancy from the focused floor.
## The rotate path changes the active edge without a signal, so this runs every
## frame rather than on a signal alone.
func _rebuild() -> void:
	var floor_node: Node = _focused_floor()
	if floor_node == null:
		visible = false
		return
	visible = true
	_sides = maxi(3, int(floor_node.polygon_sides))
	_active_edge = posmod(int(floor_node.get_active_edge()), _sides)
	var occupied: Array[bool] = []
	for i: int in range(_sides):
		var count: int = 0
		if floor_node.has_method("get_agent_count_on_edge"):
			count = int(floor_node.get_agent_count_on_edge(i))
		occupied.append(count > 0)
	_occupied = occupied
	custom_minimum_size = Vector2(row_width(_sides), HEX_WIDTH)
	size = custom_minimum_size
	queue_redraw()


## Total width of the pip row, in window pixels.
func row_width(sides: int) -> float:
	return float(sides) * HEX_WIDTH + float(maxi(sides - 1, 0)) * HEX_GAP


## Centre x of the pip for one edge, relative to the row's left edge.
func pip_centre_x(index: int) -> float:
	return float(index) * (HEX_WIDTH + HEX_GAP) + HEX_WIDTH / 2.0


## The pip a point falls in, or -1 when it falls in a gap or outside the row.
func pip_at(local_x: float, sides: int) -> int:
	var pitch: float = HEX_WIDTH + HEX_GAP
	if local_x < 0.0 or local_x >= row_width(sides):
		return -1
	var index: int = int(floor(local_x / pitch))
	if local_x - float(index) * pitch > HEX_WIDTH:
		return -1
	return mini(index, sides - 1)


## Flat-top hexagon corners around a centre. Flat top means two vertices sit at
## the left and right extremes, and the top and bottom read as flat edges.
func _hex_points(centre: Vector2, width: float) -> PackedVector2Array:
	var r: float = width / 2.0
	var points: PackedVector2Array = PackedVector2Array()
	for i: int in range(6):
		var theta: float = TAU * float(i) / 6.0
		points.append(centre + Vector2(cos(theta), sin(theta)) * r)
	return points


func _draw() -> void:
	if _occupied.size() != _sides:
		return
	for i: int in range(_sides):
		var centre := Vector2(pip_centre_x(i), HEX_WIDTH / 2.0)
		var points: PackedVector2Array = _hex_points(centre, HEX_WIDTH)
		if i == _active_edge:
			draw_colored_polygon(points, ACTIVE_COLOR)
		elif _occupied[i]:
			draw_colored_polygon(points, AGENT_COLOR)
		else:
			var loop: PackedVector2Array = points.duplicate()
			loop.append(points[0])
			draw_polyline(loop, EMPTY_COLOR, OUTLINE_WIDTH)


func _gui_input(event: InputEvent) -> void:
	if not (event is InputEventMouseButton):
		return
	var button := event as InputEventMouseButton
	if not button.pressed or button.button_index != MOUSE_BUTTON_LEFT:
		return
	var index: int = pip_at(button.position.x, _sides)
	if index < 0 or index == _active_edge:
		return
	if _tower != null and _tower.has_method("rotate_focused_to_edge"):
		_tower.rotate_focused_to_edge(index)
	accept_event()


## Follows the focused floor every frame, so the row rides the refocus tween
## and the elastic overscroll without a second animation.
func _track_floor() -> void:
	var floor_node: Node = _focused_floor()
	if floor_node == null or not (floor_node is Node2D):
		return
	var origin: Vector2 = (floor_node as Node2D).get_global_transform_with_canvas().origin
	position = origin + Vector2(-size.x / 2.0, ROW_OFFSET_Y)
