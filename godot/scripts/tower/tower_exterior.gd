extends Node2D
## TowerExterior — the roof cap seated on the top slab and the plinth under the
## bottom slab. Both live in art pixels, like the rest of the tower world.

## Mirrors TowerManager.FLOOR_SPACING / BASE_FLOOR_HEIGHT. The manager passes
## tower_height = floor_count * spacing, so the top slab is derived from these.
const FLOOR_SPACING: float = 56.0
const FLOOR_HEIGHT: float = 40.0

## Demo roof cap: a 60x14 pentagon in the canopy green.
const ROOF_WIDTH: float = 60.0
const ROOF_HEIGHT: float = 14.0
const ROOF_COLOR: Color = Color("#3f4a3a")
const ROOF_OUTLINE: Color = Color("#2b3327")
## Phase 5 plinth: shaft width, not plaza width. It reads as a base the tower
## stands on, so it stays a narrow dark rect tucked under the bottom slab. A
## hexagon here read as a floating pedestal.
const PLINTH_COLOR: Color = Color("#14161f")
const PLINTH_OUTLINE: Color = Color("#0e1017")
const PLINTH_HALF_WIDTH: float = 24.0
const PLINTH_HEIGHT: float = 12.0
## How far the plinth top hides behind the bottom slab.
const PLINTH_TUCK: float = 4.0

var _polygon_sides: int = 6
var _tower_radius: float = 40.0
var _roof_polygon: Polygon2D
var _roof_outline: Line2D
var _base_polygon: Polygon2D
var _base_outline: Line2D


func _ready() -> void:
	_roof_polygon = Polygon2D.new()
	_roof_polygon.color = ROOF_COLOR
	add_child(_roof_polygon)
	_roof_outline = _make_outline(ROOF_OUTLINE)
	_base_polygon = Polygon2D.new()
	_base_polygon.color = PLINTH_COLOR
	add_child(_base_polygon)
	_base_outline = _make_outline(PLINTH_OUTLINE)


## A 1 px darker border keeps a flat fill from reading as raw vector art next to
## the pixel tiles.
func _make_outline(color: Color) -> Line2D:
	var line := Line2D.new()
	line.width = 1.0
	line.closed = true
	line.default_color = color
	add_child(line)
	return line


func configure(polygon_sides: int, tower_height: float, tower_radius: float = 40.0) -> void:
	_polygon_sides = polygon_sides
	_tower_radius = tower_radius
	_draw_roof(tower_height)
	_draw_base()


func _draw_roof(tower_height: float) -> void:
	# Floors stack upward in negative Y. The topmost floor centre sits at
	# -(tower_height - FLOOR_SPACING), so its slab top is half a floor above it.
	var slab_top: float = -tower_height + FLOOR_SPACING - FLOOR_HEIGHT / 2.0
	var w: float = ROOF_WIDTH
	var h: float = ROOF_HEIGHT
	var left: float = -w / 2.0
	var top: float = slab_top - h
	var points := PackedVector2Array([
		Vector2(left + 0.5 * w, top),
		Vector2(left + 1.0 * w, top + 0.55 * h),
		Vector2(left + 0.84 * w, top + h),
		Vector2(left + 0.16 * w, top + h),
		Vector2(left, top + 0.55 * h),
	])
	_roof_polygon.polygon = points
	if _roof_outline:
		_roof_outline.points = points


func _draw_base() -> void:
	# The bottom slab centre sits at y = 0, so its bottom edge is at
	# FLOOR_HEIGHT / 2. Start the plinth PLINTH_TUCK px above that edge, so the
	# slab hides its top.
	var top_y: float = FLOOR_HEIGHT / 2.0 - PLINTH_TUCK
	var points := PackedVector2Array([
		Vector2(-PLINTH_HALF_WIDTH, top_y), Vector2(PLINTH_HALF_WIDTH, top_y),
		Vector2(PLINTH_HALF_WIDTH, top_y + PLINTH_HEIGHT),
		Vector2(-PLINTH_HALF_WIDTH, top_y + PLINTH_HEIGHT),
	])
	_base_polygon.polygon = points
	if _base_outline:
		_base_outline.points = points
