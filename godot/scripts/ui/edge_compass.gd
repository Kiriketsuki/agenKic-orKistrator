extends Control
## EdgeCompass — a small n-gon diagram in the bottom-left corner that shows
## which edge of the focused floor faces the viewer.
##
## PARITY (Phase 4) — edge rotation moves only the floor contents. When both
## the old edge and the new edge are empty, nothing on the slab changes, so the
## turn reads as a dead click. This widget always turns, so the action is never
## silent. The side count comes straight from the floor, so the diagram also
## doubles as the composite-load readout that T15 drives.
##
## Read-only toward the tower: it calls the public TowerManager API and walks
## FloorsContainer without mutating anything.

const ACTIVE_COLOR: Color = Color("#FBB13C")
const IDLE_COLOR: Color = Color("#363a4f")
const FILL_COLOR: Color = Color(0.08, 0.09, 0.12, 0.72)
const HUB_COLOR: Color = Color("#2a2f45")
## Turn duration and ease, matched to the floor carousel in FloorScene.
const TURN_DURATION: float = 0.55
## Phase 5 section 3 — the widget shouts during a turn and recedes when idle.
const ALPHA_ACTIVE: float = 1.0
const ALPHA_IDLE: float = 0.4
const ALPHA_DECAY: float = 0.3

@export var tower_path: NodePath = NodePath("/root/Main/Tower")
## Circumradius of the drawn n-gon, in window pixels.
@export var radius: float = 28.0

var _tower: Node = null
var _sides: int = 6
var _active_edge: int = 0
## Fractional edge index of the highlight. The polygon orientation stays fixed
## and this value sweeps around it, so the widget shows which way the room
## turned. A whole number lands exactly on one edge.
var _highlight: float = 0.0
var _turn_tween: Tween = null
var _alpha_tween: Tween = null


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	custom_minimum_size = Vector2(72.0, 72.0)
	_tower = get_node_or_null(tower_path)
	if _tower == null:
		set_process(false)
		return
	_sides = _read_sides()
	_active_edge = _read_active_edge()
	_highlight = float(_active_edge)
	modulate.a = ALPHA_IDLE


func _process(_delta: float) -> void:
	if _tower == null or not is_instance_valid(_tower):
		return
	var sides: int = _read_sides()
	var edge: int = _read_active_edge()
	if sides != _sides:
		# A side-count change redraws the diagram at once. Only an edge step
		# earns the eased turn.
		_sides = sides
		_active_edge = edge
		_highlight = float(_active_edge)
		queue_redraw()
		return
	if edge != _active_edge:
		_turn_to(edge)
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


## Sweeps the highlight to the new edge, over the shorter way round. The polygon
## itself never rotates. A rotating polygon with a fixed highlight told the user
## nothing, because both frames looked the same.
func _turn_to(new_edge: int) -> void:
	var delta: int = new_edge - _active_edge
	# Wrap the step into [-sides/2, sides/2], so a 5 to 0 move sweeps one step
	# forward instead of five steps back.
	var half: int = _sides / 2
	while delta > half:
		delta -= _sides
	while delta < -half:
		delta += _sides
	var target: float = _highlight + float(delta)
	_active_edge = new_edge
	if _turn_tween != null and _turn_tween.is_valid():
		_turn_tween.kill()
	_turn_tween = create_tween()
	_turn_tween.set_trans(Tween.TRANS_EXPO).set_ease(Tween.EASE_OUT)
	_turn_tween.tween_property(self, "_highlight", target, TURN_DURATION)
	_flash()


## Brings the widget to full alpha for the turn, then lets it fade back.
func _flash() -> void:
	if _alpha_tween != null and _alpha_tween.is_valid():
		_alpha_tween.kill()
	modulate.a = ALPHA_ACTIVE
	_alpha_tween = create_tween()
	_alpha_tween.tween_interval(TURN_DURATION)
	_alpha_tween.tween_property(self, "modulate:a", ALPHA_IDLE, ALPHA_DECAY)


## Vertices of the fixed polygon. Vertex 0 starts a half step back from the
## bottom, so edge 0 sits flat at the bottom of the diagram.
func _polygon_points(centre: Vector2) -> PackedVector2Array:
	var base: float = PI / 2.0 - TAU / float(_sides) / 2.0
	var points: PackedVector2Array = PackedVector2Array()
	for i: int in range(_sides):
		var theta: float = base + TAU * float(i) / float(_sides)
		points.append(centre + Vector2(cos(theta), sin(theta)) * radius)
	return points


## Point on the polygon perimeter at a fractional edge index. The whole part
## picks the edge, the fraction walks along it.
func _perimeter_point(points: PackedVector2Array, param: float) -> Vector2:
	var wrapped: float = fposmod(param, float(_sides))
	var index: int = int(floor(wrapped))
	var t: float = wrapped - float(index)
	var a: Vector2 = points[index % _sides]
	var b: Vector2 = points[(index + 1) % _sides]
	return a.lerp(b, t)


func _draw() -> void:
	if _sides < 3:
		return
	var centre := Vector2(size.x / 2.0, size.y / 2.0)
	var points: PackedVector2Array = _polygon_points(centre)
	draw_colored_polygon(points, FILL_COLOR)
	for i: int in range(_sides):
		draw_line(points[i], points[(i + 1) % _sides], IDLE_COLOR, 1.5)
	# The highlight rides the perimeter, so it can straddle two edges mid-turn.
	var start: Vector2 = _perimeter_point(points, _highlight)
	var segment: PackedVector2Array = PackedVector2Array([start])
	var first_corner: int = int(ceil(_highlight))
	for k: int in range(first_corner, first_corner + _sides + 1):
		if float(k) >= _highlight + 1.0:
			break
		segment.append(points[posmod(k, _sides)])
	segment.append(_perimeter_point(points, _highlight + 1.0))
	draw_polyline(segment, ACTIVE_COLOR, 3.0)
	draw_circle(centre, 2.0, HUB_COLOR)
