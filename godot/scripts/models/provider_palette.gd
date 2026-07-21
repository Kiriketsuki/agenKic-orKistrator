class_name ProviderPalette
## T16 (#125) — static lazy-cached provider -> LUT texture loader for the
## palette-swap shader. Mirrors FloatingRune's static-lazy shader/cache idiom
## (see floating_rune.gd _glow_shader).
##
## LUT STRATEGY: procedural GradientTexture1D built at load time from
## tower.json's `providers` color stops — no committed PNG binaries, so
## "new provider = one JSON block, zero code" holds (acceptance #5). Godot
## 4.2 GL Compatibility supports GradientTexture1D + a `source_color` sampler
## hint on the consuming uniform (declared in palette_swap.gdshader); this
## rests on doc-verified shader/resource legality, not a compiled run (no
## Godot binary on this machine).
##
## NON-REGRESSION CONTRACT: an unknown or empty provider name gets a neutral
## grey ramp AND lut_mix() returns 0.0 — the palette-swap shader therefore
## ignores the LUT entirely for unknown providers and the vivid CLASS colors
## shipped today are preserved unmodified. Only a recognized provider lights
## up the hue-remap, mirroring the existing "unknown" fallback already used
## by floating_rune.gd and agent_character.set_provider().
##
## floating_rune.gd's PROVIDER_COLORS is NOT refactored to source from this
## config in this task (avoids regressing rune tinting) — tower.json's
## `providers` stops are seeded to the exact same hexes as a shared
## reference; unifying the two is a follow-up TODO.

const CONFIG_PATH: String = "res://config/tower.json"
const _LUT_WIDTH: int = 256
const _FALLBACK_STOPS: PackedStringArray = ["#1A1A1A", "#888888", "#DDDDDD"]

## T17 (#127) — offset into the provider's gradient sampled for particle
## accent color. tower.json stops run dark -> mid -> light, so a
## near-the-bright-end offset gives a vivid, legible-against-dark-background
## particle tint without being flat white. Reuses the SAME gradient built for
## the palette-swap LUT (get_lut) — no second color table.
const _ACCENT_OFFSET: float = 0.75
const _DEFAULT_PARTICLE_STYLE: String = "dot"
const _NEUTRAL_ACCENT: Color = Color(0.7, 0.7, 0.7, 1.0)

static var _config: TowerConfig = null
static var _lut_cache: Dictionary = {}


## Returns the cached (or freshly built) GradientTexture1D LUT for `provider`.
## Falls back to a neutral grey ramp for unknown/empty provider names.
static func get_lut(provider: String) -> Texture2D:
	var key: String = provider if not provider.is_empty() else "unknown"
	if _lut_cache.has(key):
		return _lut_cache[key]
	var stops: PackedStringArray = _stops_for(key)
	var tex: GradientTexture1D = _build_gradient_texture(stops)
	_lut_cache[key] = tex
	return tex


## Blend strength toward the provider's LUT hue-remap. 0.0 for unknown/empty
## provider (non-regression — see class doc-comment); config `lut_strength`
## (~0.85) for any recognized provider.
static func get_lut_mix(provider: String) -> float:
	if provider.is_empty() or provider == "unknown":
		return 0.0
	var cfg: TowerConfig = _ensure_config()
	if not (cfg.providers as Dictionary).has(provider):
		return 0.0
	return cfg.lut_strength


## T17 (#127) — particle accent color for `provider`. Samples the SAME
## cached gradient built for the palette-swap LUT at a bright offset
## (_ACCENT_OFFSET), so the color source stays singular (tower.json stops).
## Unknown/empty provider -> neutral grey, mirroring get_lut()'s fallback.
static func get_accent_color(provider: String) -> Color:
	var key: String = provider if not provider.is_empty() else "unknown"
	if key == "unknown":
		return _NEUTRAL_ACCENT
	var tex: GradientTexture1D = get_lut(key) as GradientTexture1D
	if tex == null or tex.gradient == null:
		return _NEUTRAL_ACCENT
	return tex.gradient.sample(_ACCENT_OFFSET)


## T17 (#127) — procedural particle shape family for `provider` (see
## particle_textures.gd doc-comment for the shape catalog). Reads
## tower.json's providers[provider].particle_style; defaults to "dot" for an
## unrecognized/missing provider or a provider block with no style set.
static func get_particle_style(provider: String) -> String:
	if provider.is_empty():
		return _DEFAULT_PARTICLE_STYLE
	var cfg: TowerConfig = _ensure_config()
	var entry: Variant = (cfg.providers as Dictionary).get(provider, null)
	if entry is Dictionary and (entry as Dictionary).get("particle_style", null) is String:
		return (entry as Dictionary)["particle_style"]
	return _DEFAULT_PARTICLE_STYLE


static func _stops_for(provider: String) -> PackedStringArray:
	var cfg: TowerConfig = _ensure_config()
	var entry: Variant = (cfg.providers as Dictionary).get(provider, null)
	if entry is Dictionary and (entry as Dictionary).get("stops", null) is Array:
		var raw: Array = (entry as Dictionary)["stops"]
		var out: PackedStringArray = PackedStringArray()
		for hex: Variant in raw:
			if hex is String:
				out.append(hex as String)
		if out.size() >= 2:
			return out
	return _FALLBACK_STOPS


static func _build_gradient_texture(stops: PackedStringArray) -> GradientTexture1D:
	var gradient: Gradient = Gradient.new()
	var offsets: PackedFloat32Array = PackedFloat32Array()
	var colors: PackedColorArray = PackedColorArray()
	var count: int = stops.size()
	for i: int in range(count):
		offsets.append(float(i) / float(count - 1))
		colors.append(Color(stops[i]))
	gradient.offsets = offsets
	gradient.colors = colors

	var tex: GradientTexture1D = GradientTexture1D.new()
	tex.gradient = gradient
	tex.width = _LUT_WIDTH
	return tex


static func _ensure_config() -> TowerConfig:
	if _config == null:
		_config = TowerConfig.from_file(CONFIG_PATH)
	return _config


## Test-only hook: clears the static cache so palette_math_test.gd (and any
## future headless test) can rebuild from a fresh/injected config without
## engine restart. Not called by production code.
static func _reset_cache_for_tests() -> void:
	_config = null
	_lut_cache.clear()
