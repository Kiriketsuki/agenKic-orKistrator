extends Node2D
## TowerExterior — draws procedural roof and base polygons.

var _polygon_sides: int = 6
var _tower_radius: float = 44.0
var _roof_polygon: Polygon2D
var _base_polygon: Polygon2D


func _ready() -> void:
	_roof_polygon = Polygon2D.new()
	_roof_polygon.color = Color(0.25, 0.30, 0.22, 1.0)  # mossy stone
	add_child(_roof_polygon)
	_base_polygon = Polygon2D.new()
	_base_polygon.color = Color(0.15, 0.18, 0.15, 1.0)  # dark stone
	add_child(_base_polygon)


func configure(polygon_sides: int, tower_height: float, tower_radius: float = 44.0) -> void:
	_polygon_sides = polygon_sides
	_tower_radius = tower_radius
	_draw_roof(tower_height)
	_draw_base()


func _draw_roof(tower_top_y: float) -> void:
	var roof_radius: float = _tower_radius * 0.78
	var points: PackedVector2Array = _regular_polygon_points(roof_radius)
	var offset := Vector2(0.0, tower_top_y - roof_radius)
	for i: int in range(points.size()):
		points[i] += offset
	_roof_polygon.polygon = points


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
