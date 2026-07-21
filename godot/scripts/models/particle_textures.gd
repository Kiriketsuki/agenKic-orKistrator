class_name ParticleTextures
## T17 (#127) — static lazy-cached procedural particle textures for the tier
## particle effects. Mirrors ProviderPalette's static-lazy cache idiom (see
## provider_palette.gd doc-comment) — no committed PNG binaries, consistent
## with T16's procedural-LUT decision.
##
## SHAPE STRATEGY — HONEST-MINIMAL: true per-provider elemental glyph/leaf
## art (rune glyphs for Claude, leaf motes for OpenAI, etc.) is explicitly
## deferred to T22 (the sprite/art task). T17 ships a small documented shape
## FAMILY drawn procedurally at load time via the Godot 4.2 Image API, keyed
## by a per-provider `particle_style` string in tower.json
## (ProviderPalette.get_particle_style()):
##   "dot"   — soft radial falloff disc (default / unknown provider)
##   "spark" — 4-point star burst (Ollama — embers)
##   "shard" — diamond (Gemini — light shards)
##   "ring"  — hollow ring outline (DeepSeek — void ripples)
## "rune" and "leaf" (Claude / OpenAI) intentionally fall back to "dot" today
## — the shape family does not yet include bespoke glyph/leaf geometry; the
## fallback is documented, not silent (see get_texture()'s _SHAPE_FUNCS.get
## default). Color is NEVER baked into the texture — textures are greyscale
## with alpha, tinted per-agent via CPUParticles2D.color /
## ProviderPalette.get_accent_color() at the node level, so a single cached
## texture per style is safely shared across every provider and every agent
## (immutable Texture2D, no per-agent duplication).

const _SIZE: int = 16
const _CENTER: float = float(_SIZE) / 2.0 - 0.5
const _DEFAULT_STYLE: String = "dot"

static var _cache: Dictionary = {}


## Returns the cached (or freshly built) procedural Texture2D for `style`.
## Unknown/empty style names fall back to "dot".
static func get_texture(style: String) -> Texture2D:
	var key: String = style if _is_known_style(style) else _DEFAULT_STYLE
	if _cache.has(key):
		return _cache[key]
	var image: Image = _build_image(key)
	var tex: ImageTexture = ImageTexture.create_from_image(image)
	_cache[key] = tex
	return tex


static func _is_known_style(style: String) -> bool:
	return style in ["dot", "spark", "shard", "ring"]


static func _build_image(style: String) -> Image:
	var image: Image = Image.create(_SIZE, _SIZE, false, Image.FORMAT_RGBA8)
	match style:
		"spark":
			_draw_spark(image)
		"shard":
			_draw_shard(image)
		"ring":
			_draw_ring(image)
		_:
			_draw_dot(image)
	return image


## Soft radial falloff disc — alpha fades smoothly from center to edge.
static func _draw_dot(image: Image) -> void:
	for y: int in range(_SIZE):
		for x: int in range(_SIZE):
			var d: float = Vector2(x, y).distance_to(Vector2(_CENTER, _CENTER)) / _CENTER
			var a: float = clampf(1.0 - d, 0.0, 1.0)
			a = a * a
			image.set_pixel(x, y, Color(1.0, 1.0, 1.0, a))


## 4-point star burst — bright along the cardinal axes, fading elsewhere.
static func _draw_spark(image: Image) -> void:
	for y: int in range(_SIZE):
		for x: int in range(_SIZE):
			var dx: float = absf(float(x) - _CENTER) / _CENTER
			var dy: float = absf(float(y) - _CENTER) / _CENTER
			var axis: float = clampf(1.0 - minf(dx, dy) * 2.0, 0.0, 1.0)
			var d: float = Vector2(x, y).distance_to(Vector2(_CENTER, _CENTER)) / _CENTER
			var radial: float = clampf(1.0 - d, 0.0, 1.0)
			var a: float = maxf(axis, radial * 0.35)
			image.set_pixel(x, y, Color(1.0, 1.0, 1.0, a))


## Diamond shard — filled rotated square (|dx| + |dy| <= radius).
static func _draw_shard(image: Image) -> void:
	for y: int in range(_SIZE):
		for x: int in range(_SIZE):
			var dx: float = absf(float(x) - _CENTER) / _CENTER
			var dy: float = absf(float(y) - _CENTER) / _CENTER
			var manhattan: float = dx + dy
			var a: float = clampf(1.0 - manhattan, 0.0, 1.0)
			image.set_pixel(x, y, Color(1.0, 1.0, 1.0, a))


## Hollow ring outline — alpha peaks at a fixed radius band, near-zero at
## center and edge.
static func _draw_ring(image: Image) -> void:
	var ring_radius: float = 0.65
	var ring_width: float = 0.28
	for y: int in range(_SIZE):
		for x: int in range(_SIZE):
			var d: float = Vector2(x, y).distance_to(Vector2(_CENTER, _CENTER)) / _CENTER
			var band: float = clampf(1.0 - absf(d - ring_radius) / ring_width, 0.0, 1.0)
			image.set_pixel(x, y, Color(1.0, 1.0, 1.0, band))


## Test-only hook: clears the static cache so particle_math_test.gd can
## rebuild fresh, mirroring ProviderPalette._reset_cache_for_tests(). Not
## called by production code.
static func _reset_cache_for_tests() -> void:
	_cache.clear()
