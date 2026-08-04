extends Control
## Orb — one circular HUD control in the orb flock (F2, #175). Procedurally
## draws a filled circle in the orb's accent color, a soft outer glow ring,
## and a centered rune glyph. Owns no flock logic: OrbFlock drives position,
## drag, and state; this script only renders and reports raw pointer events
## through its signals so OrbFlock can tell a tap from the start of a drag.

class_name Orb

signal press_started(orb: Orb, at_position: Vector2)
signal press_moved(orb: Orb, at_position: Vector2)
signal press_ended(orb: Orb, at_position: Vector2)

const GLOW_COLOR: Color = Color(1.0, 1.0, 1.0, 0.18)
const RIM_COLOR: Color = Color(0.0, 0.0, 0.0, 0.35)
const GLYPH_COLOR: Color = Color(0.96, 0.94, 0.88, 0.95)
const PRESSED_SCALE: float = 0.92

## Orb identity. Set by OrbFlock before adding the node to the tree.
@export var orb_id: String = ""
@export var accent_color: Color = Color(0.79, 0.64, 0.15)
@export var glyph: String = "?"
@export var orb_radius: float = 26.0

var _pressed: bool = false
var _font: Font = ThemeDB.fallback_font
var _font_size: int = 40


func _ready() -> void:
	custom_minimum_size = Vector2(orb_radius * 2.0, orb_radius * 2.0)
	size = custom_minimum_size
	mouse_filter = Control.MOUSE_FILTER_STOP
	mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
	gui_input.connect(_on_gui_input)


func _on_gui_input(event: InputEvent) -> void:
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.button_index != MOUSE_BUTTON_LEFT:
			return
		if mb.pressed:
			_pressed = true
			queue_redraw()
			press_started.emit(self, mb.global_position)
		else:
			_pressed = false
			queue_redraw()
			press_ended.emit(self, mb.global_position)
		get_viewport().set_input_as_handled()
	elif event is InputEventMouseMotion and _pressed:
		var motion: InputEventMouseMotion = event as InputEventMouseMotion
		press_moved.emit(self, motion.global_position)
		get_viewport().set_input_as_handled()


func _draw() -> void:
	var center: Vector2 = size * 0.5
	var radius: float = minf(size.x, size.y) * 0.5
	var draw_radius: float = radius * (PRESSED_SCALE if _pressed else 1.0)
	# Outer glow: a wider, faint ring behind the fill.
	draw_circle(center, draw_radius + 12.0, GLOW_COLOR)
	draw_circle(center, draw_radius, accent_color)
	draw_arc(center, draw_radius, 0.0, TAU, 48, RIM_COLOR, 4.0, true)
	if _font != null and not glyph.is_empty():
		var text_size: Vector2 = _font.get_string_size(glyph, HORIZONTAL_ALIGNMENT_CENTER, -1.0, _font_size)
		var text_origin: Vector2 = center - Vector2(text_size.x * 0.5, -text_size.y * 0.32)
		draw_string(_font, text_origin, glyph, HORIZONTAL_ALIGNMENT_CENTER, -1.0, _font_size, GLYPH_COLOR)


## True while the pointer holds this orb down. OrbFlock reads this to tell
## a tap (press and release without leaving drag threshold) from a drag.
func is_pressed() -> bool:
	return _pressed
