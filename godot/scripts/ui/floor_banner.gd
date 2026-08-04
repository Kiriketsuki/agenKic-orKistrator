extends Control
## FloorBanner — the nameplate banner above the focused floor.
##
## PARITY (Phase 6 section 2) — the demo prints the room name and the edge
## readout on a small gold plate over the tower. Godot had no such readout, so
## the rotation state lived only in the corner compass.
##
## The banner renders at window resolution on UILayer and tracks the focused
## floor through get_global_transform_with_canvas(), the same projection
## WorldLabels uses. The camera zoom folds into that transform, so the plate
## never magnifies with the world.
##
## Read-only toward the tower. It calls the public TowerManager API and walks
## FloorsContainer without mutating anything.

const FRAME_TEXTURE_PATH: String = "res://assets/ui/nameplate_frame.png"
## Nine-patch margins, from the demo's border-image "4 6 fill". The first
## number is the vertical inset, the second is the horizontal inset.
const PATCH_V: int = 4
const PATCH_H: int = 6
const INTERIOR_COLOR: Color = Color("#1a1626")
const TEXT_COLOR: Color = Color("#d9a94a")
const TEXT_SIZE: int = 28
const PLATE_HEIGHT: float = 52.0
const PLATE_PADDING: float = 36.0
## Screen-space offset from the floor origin, in window (design-canvas)
## pixels. The plate sits above the floor label. Doubled alongside every
## other screen-space UI size when project.godot's design canvas doubled
## from 1920x1080 to 3840x2160.
const PLATE_OFFSET_Y: float = -376.0
## Fade applied when no floor is in focus yet.
const FADE_DURATION: float = 0.2

@export var tower_path: NodePath = NodePath("/root/Main/Tower")

var _tower: Node = null
var _frame: NinePatchRect = null
var _interior: ColorRect = null
var _label: Label = null
var _font: Font = null
var _last_text: String = ""


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_anchors_preset(Control.PRESET_FULL_RECT)
	_font = _make_font()
	_build_plate()
	_tower = get_node_or_null(tower_path)
	if _tower == null:
		set_process(false)
		_frame.visible = false
		return
	if _tower.has_signal("floor_focus_changed"):
		_tower.connect("floor_focus_changed", _on_focus_changed)
	if _tower.has_signal("floors_changed"):
		_tower.connect("floors_changed", _refresh_text)
	_refresh_text()


## Fira Code where the system has it, plain monospace otherwise. The glyph
## spacing gives the plate the demo's letter-spaced look, which Label alone
## does not offer.
func _make_font() -> Font:
	var base := SystemFont.new()
	base.font_names = PackedStringArray(["Fira Code", "monospace"])
	var variation := FontVariation.new()
	variation.base_font = base
	variation.spacing_glyph = 1
	return variation


func _build_plate() -> void:
	_frame = NinePatchRect.new()
	_frame.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_frame.texture = load(FRAME_TEXTURE_PATH)
	_frame.patch_margin_left = PATCH_H
	_frame.patch_margin_right = PATCH_H
	_frame.patch_margin_top = PATCH_V
	_frame.patch_margin_bottom = PATCH_V
	_frame.draw_center = false
	add_child(_frame)

	# The frame texture carries the border only. The interior fill sits under
	# the text, inside the patch margins.
	_interior = ColorRect.new()
	_interior.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_interior.color = INTERIOR_COLOR
	_interior.set_anchors_preset(Control.PRESET_FULL_RECT)
	_interior.offset_left = float(PATCH_H) - 2.0
	_interior.offset_right = -(float(PATCH_H) - 2.0)
	_interior.offset_top = float(PATCH_V) - 1.0
	_interior.offset_bottom = -(float(PATCH_V) - 1.0)
	_frame.add_child(_interior)

	_label = Label.new()
	_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_label.set_anchors_preset(Control.PRESET_FULL_RECT)
	_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	_label.add_theme_font_override("font", _font)
	_label.add_theme_font_size_override("font_size", TEXT_SIZE)
	_label.add_theme_color_override("font_color", TEXT_COLOR)
	_frame.add_child(_label)


func _process(_delta: float) -> void:
	if _tower == null or not is_instance_valid(_tower):
		return
	# The rotate path changes the active edge without a signal, so the text is
	# rebuilt whenever the readout differs from the last frame.
	_refresh_text()
	_track_floor()


func _on_focus_changed(_index: int) -> void:
	_refresh_text()


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


## Builds the readout for the focused floor. The edge number is 1-based, and
## EdgeCompass numbers its dots the same way.
func banner_text(label: String, active_edge: int, sides: int) -> String:
	var total: int = maxi(sides, 1)
	var human_edge: int = posmod(active_edge, total) + 1
	return "✧ %s · EDGE %d/%d ✧" % [label.to_upper(), human_edge, total]


func _refresh_text() -> void:
	var floor_node: Node = _focused_floor()
	if floor_node == null:
		_frame.visible = false
		return
	_frame.visible = true
	var label: String = String(floor_node.floor_label)
	if label == "":
		label = String(floor_node.floor_name)
	var sides: int = maxi(3, int(floor_node.polygon_sides))
	var edge: int = int(floor_node.get_active_edge()) if floor_node.has_method("get_active_edge") else 0
	var text: String = banner_text(label, edge, sides)
	if text == _last_text:
		return
	_last_text = text
	_label.text = text
	_resize_plate()


func _resize_plate() -> void:
	var width: float = _font.get_string_size(
		_label.text, HORIZONTAL_ALIGNMENT_LEFT, -1.0, TEXT_SIZE).x + PLATE_PADDING * 2.0
	_frame.size = Vector2(maxf(width, 192.0), PLATE_HEIGHT)


## Camera zoom at which the plate renders at its authored 1:1 size. Above
## it the plate grows with the world, below it the plate shrinks, so the
## banner always reads as part of the scene instead of a fixed overlay.
const REFERENCE_ZOOM: float = 6.0
const PLATE_SCALE_MIN: float = 0.4
const PLATE_SCALE_MAX: float = 10.0


## Follows the focused floor every frame, so the plate rides the refocus tween
## and the elastic overscroll without a second animation. The plate scales
## with the camera zoom relative to REFERENCE_ZOOM.
func _track_floor() -> void:
	var floor_node: Node = _focused_floor()
	if floor_node == null or not (floor_node is Node2D):
		return
	var xf: Transform2D = (floor_node as Node2D).get_global_transform_with_canvas()
	var zoom_scale: float = clampf(xf.get_scale().x / REFERENCE_ZOOM, PLATE_SCALE_MIN, PLATE_SCALE_MAX)
	_frame.scale = Vector2(zoom_scale, zoom_scale)
	_frame.position = xf.origin + Vector2(-_frame.size.x * zoom_scale / 2.0, PLATE_OFFSET_Y * zoom_scale)
