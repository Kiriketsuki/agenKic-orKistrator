class_name FloatingRune
extends Node2D

const PROVIDER_COLORS: Dictionary = {
	"claude":   {"base": Color("#D4AF37"), "keyword": Color("#FFD75E")},
	"gemini":   {"base": Color("#5A9BD5"), "keyword": Color("#8DC4FF")},
	"openai":   {"base": Color("#00BFA5"), "keyword": Color("#4DFFE5")},
	"ollama":   {"base": Color("#F4851E"), "keyword": Color("#FFB366")},
	"deepseek": {"base": Color("#8B5CF6"), "keyword": Color("#B794FF")},
	"unknown":  {"base": Color("#888888"), "keyword": Color("#BBBBBB")},
}

const _GLOW_SHADER_CODE: String = """
shader_type canvas_item;
uniform vec4 glow_color : source_color = vec4(1.0, 1.0, 1.0, 1.0);
uniform float glow_intensity : hint_range(0.0, 2.0) = 0.6;

void fragment() {
    vec4 tex = texture(TEXTURE, UV);
    COLOR = tex;
    COLOR.rgb += glow_color.rgb * glow_intensity * tex.a;
    COLOR.a = tex.a;
}
"""

const _LIFETIME: float = 7.0
const _RISE_SPEED: float = 8.0
const _DRIFT_AMPLITUDE: float = 4.0

static var _glow_shader: Shader

@onready var _label: RichTextLabel = $Label

var _elapsed: float = 0.0
var _origin_x: float = 0.0
var _phase_offset: float = 0.0
var _accelerating: bool = false
var _shader_material: ShaderMaterial = null


func _ready() -> void:
	_phase_offset = randf() * TAU


func setup(text: String, keywords: PackedStringArray, provider: String) -> void:
	_origin_x = position.x

	var colors: Dictionary = PROVIDER_COLORS.get(provider, PROVIDER_COLORS["unknown"])
	var base_color: Color = colors["base"]
	var keyword_color: Color = colors["keyword"]

	var base_hex: String = base_color.to_html(false)
	var keyword_hex: String = keyword_color.to_html(false)

	var bbcode: String = _build_bbcode(text, keywords, base_hex, keyword_hex)
	_label.text = bbcode

	if _glow_shader == null:
		_glow_shader = Shader.new()
		_glow_shader.code = _GLOW_SHADER_CODE

	_shader_material = ShaderMaterial.new()
	_shader_material.shader = _glow_shader
	_shader_material.set_shader_parameter("glow_color", keyword_color)
	_shader_material.set_shader_parameter("glow_intensity", 0.6)
	_label.material = _shader_material


static func _escape_bbcode(raw: String) -> String:
	return raw.replace("[", "[lb]")


func _build_bbcode(
	text: String,
	keywords: PackedStringArray,
	base_hex: String,
	keyword_hex: String
) -> String:
	if keywords.is_empty():
		return "[color=#%s]%s[/color]" % [base_hex, _escape_bbcode(text)]

	# Find all keyword ranges, sorted by position (search on raw text)
	var ranges: Array = []
	for kw in keywords:
		if kw.is_empty():
			continue
		var search_pos: int = 0
		while true:
			var idx: int = text.findn(kw, search_pos)
			if idx == -1:
				break
			ranges.append({"start": idx, "end": idx + kw.length(), "keyword": kw})
			search_pos = idx + kw.length()

	if ranges.is_empty():
		return "[color=#%s]%s[/color]" % [base_hex, _escape_bbcode(text)]

	# Sort by start position
	ranges.sort_custom(func(a: Dictionary, b: Dictionary) -> bool:
		return a["start"] < b["start"]
	)

	# Merge overlapping ranges
	var merged: Array = []
	for r in ranges:
		if not merged.is_empty() and r["start"] < merged[-1]["end"]:
			merged[-1]["end"] = max(merged[-1]["end"], r["end"])
		else:
			merged.append(r.duplicate())

	# Build BBCode string — escape each text segment individually
	var result: String = ""
	var cursor: int = 0
	for r in merged:
		if cursor < r["start"]:
			result += "[color=#%s]%s[/color]" % [base_hex, _escape_bbcode(text.substr(cursor, r["start"] - cursor))]
		result += "[b][color=#%s]%s[/color][/b]" % [keyword_hex, _escape_bbcode(text.substr(r["start"], r["end"] - r["start"]))]
		cursor = r["end"]

	if cursor < text.length():
		result += "[color=#%s]%s[/color]" % [base_hex, _escape_bbcode(text.substr(cursor))]

	return result


func _process(delta: float) -> void:
	if _accelerating:
		return

	_elapsed += delta

	# Rise upward
	position.y -= _RISE_SPEED * delta

	# Horizontal sine drift
	position.x = _origin_x + sin(_elapsed * TAU + _phase_offset) * _DRIFT_AMPLITUDE

	# Opacity fade
	var alpha: float = 1.0 - (_elapsed / _LIFETIME)
	modulate.a = alpha

	# Update glow intensity proportionally to current opacity
	if _shader_material != null:
		_shader_material.set_shader_parameter("glow_intensity", 0.6 * alpha)

	if _elapsed >= _LIFETIME:
		queue_free()


func accelerate_fade() -> void:
	_accelerating = true
	var tween: Tween = create_tween()
	tween.tween_property(self, "modulate:a", 0.0, 0.3)
	tween.tween_callback(queue_free)
