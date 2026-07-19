extends Button
## WaxSeal — a procedural (no binary assets) wax-seal submit button for the
## quest board (#118). Draws a deep-red circular seal with a lighter
## top-left emboss highlight, a darker rim, and a centered monogram glyph.
## Reacts to hover/press with a subtle radius change; keeps Button's native
## `pressed` signal for wiring, so callers connect exactly as they would to
## any other Button.

class_name WaxSeal

const SEAL_COLOR: Color = Color(0.48, 0.09, 0.09, 1.0)
const SEAL_COLOR_HOVER: Color = Color(0.56, 0.11, 0.11, 1.0)
const SEAL_COLOR_PRESSED: Color = Color(0.4, 0.07, 0.07, 1.0)
const RIM_COLOR: Color = Color(0.24, 0.03, 0.03, 1.0)
const EMBOSS_COLOR: Color = Color(0.72, 0.28, 0.24, 0.55)
const GLYPH_COLOR: Color = Color(0.86, 0.74, 0.42, 1.0)
const GLYPH: String = "❦"

@export var seal_size: float = 84.0

var _font: Font = ThemeDB.fallback_font


func _ready() -> void:
	flat = true
	focus_mode = Control.FOCUS_NONE
	custom_minimum_size = Vector2(seal_size, seal_size)
	mouse_default_cursor_shape = Control.CURSOR_POINTING_HAND
	# Suppress the default Button chrome (background/border/label) — this
	# control is entirely custom-drawn in _draw().
	add_theme_stylebox_override("normal", StyleBoxEmpty.new())
	add_theme_stylebox_override("hover", StyleBoxEmpty.new())
	add_theme_stylebox_override("pressed", StyleBoxEmpty.new())
	add_theme_stylebox_override("disabled", StyleBoxEmpty.new())
	add_theme_stylebox_override("focus", StyleBoxEmpty.new())
	mouse_entered.connect(queue_redraw)
	mouse_exited.connect(queue_redraw)
	button_down.connect(queue_redraw)
	button_up.connect(queue_redraw)


func _draw() -> void:
	var center: Vector2 = size * 0.5
	var radius: float = minf(size.x, size.y) * 0.5
	var pressed_offset: float = 0.0
	var fill_color: Color = SEAL_COLOR
	# NOTE: `button_pressed` only reflects the held-down state when
	# toggle_mode is true (Godot 4 BaseButton: set_pressed() early-returns for
	# non-toggle buttons, so status.pressed never updates for a momentary
	# Button). SingleSeal/ChainSeal are plain, non-toggle Buttons, so the
	# pressed visual must key off is_pressed() instead, which BaseButton
	# derives from press_attempt/pressing_inside for momentary buttons too.
	if disabled:
		fill_color = SEAL_COLOR.darkened(0.35)
	elif is_pressed():
		fill_color = SEAL_COLOR_PRESSED
		pressed_offset = 1.0
		radius -= 2.0
	elif is_hovered():
		fill_color = SEAL_COLOR_HOVER
		radius += 1.5

	# Rim (drawn slightly larger, underneath the fill).
	draw_circle(center, radius + 2.0, RIM_COLOR)
	# Main seal fill.
	draw_circle(center, radius, fill_color)
	# Emboss highlight — a soft arc toward the upper-left, suggesting a
	# stamped light source without any texture asset.
	if not disabled:
		draw_arc(center, radius * 0.72, PI * 1.05, PI * 1.85, 24, EMBOSS_COLOR, radius * 0.18, true)
	# Centered monogram glyph.
	var glyph_size: float = radius * 0.9
	var glyph_pos: Vector2 = center + Vector2(pressed_offset, pressed_offset)
	draw_string(
		_font,
		glyph_pos - Vector2(glyph_size * 0.32, -glyph_size * 0.32),
		GLYPH,
		HORIZONTAL_ALIGNMENT_CENTER,
		-1.0,
		int(glyph_size),
		GLYPH_COLOR
	)
