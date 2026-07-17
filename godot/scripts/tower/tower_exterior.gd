extends Node2D
## TowerExterior — draws procedural roof and base polygons.

var _polygon_sides: int = 6
var _tower_radius: float = 44.0
var _roof_polygon: Polygon2D
var _base_polygon: Polygon2D


func _ready() -> void:
	_roof_polygon = Polygon2D.new()
	_roof_polygon.color = Color(0.32, 0.24, 0.42, 1.0)  # slate-purple shingles
	add_child(_roof_polygon)
	_base_polygon = Polygon2D.new()
	_base_polygon.color = Color(0.22, 0.23, 0.28, 1.0)  # stone plinth
	add_child(_base_polygon)


func configure(polygon_sides: int, tower_height: float, tower_radius: float = 44.0) -> void:
	_polygon_sides = polygon_sides
	_tower_radius = tower_radius
	_draw_roof(tower_height)
	_draw_base()


func _draw_roof(tower_height: float) -> void:
	# Floors stack upward in negative Y (floor i sits at y = i * -spacing),
	# so the roof spire goes ABOVE the topmost floor, not below floor 0.
	var top_y: float = -tower_height + _tower_radius * 0.4
	var w: float = _tower_radius * 1.6
	_roof_polygon.polygon = PackedVector2Array([
		Vector2(-w, top_y),
		Vector2(w, top_y),
		Vector2(0.0, top_y - _tower_radius * 2.2),  # spire point
	])


func _draw_base() -> void:
	var points: PackedVector2Array = _regular_polygon_points(_tower_radius)
	var offset := Vector2(0.0, _tower_radius * 0.75)
	for i: int in range(points.size()):
		points[i] += offset
	_base_polygon.polygon = points


func _regular_polygon_points(radius: float) -> PackedVector2Array:
	var points: PackedVector2Array = PackedVector2Array()
	for i: int in range(_polygon_sides):
		var angle: float = (TAU / _polygon_sides) * i - PI / 2.0
		points.append(Vector2(cos(angle) * radius, sin(angle) * radius))
	return points
