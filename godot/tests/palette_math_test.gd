# palette_math_test.gd — Regression guard for T16 (#125) palette-swap
# power-band math and the class/provider color matrices it composes with.
#
# No GUT (or other Godot test runner) is vendored in this project yet, so
# this is a standalone script runnable headless, mirroring floor_morph_test.gd:
#
#   godot --headless --path godot --script tests/palette_math_test.gd
#
# Asserts (acceptance criteria for #125):
#   #2 — the 5 named power bands (Novice/Adept/Master/Grandmaster/Legendary)
#        yield strictly distinct effect-amount tuples.
#   #6 — glow_for() is smooth and monotonic across a fine sweep, never
#        stepped, even though the 5 bands exist as documented sample points.
#   #3/#4 — the 7 CLASS_COLORS are pairwise distinct, and each of the 5
#        provider LUTs samples to a pairwise-distinct color at equal
#        luminance keys, giving class_color x provider_lut x power_level as
#        three independent axes — the structural 7 x 5 x (continuous power)
#        = 35+ variant matrix the shader composes at render time (see
#        palette_swap.gdshader's header comment for the full pipeline; this
#        test asserts each axis is independently distinct, which is what
#        makes the product of the three axes distinct).
#   #4 CROSS TERM — the per-axis checks above cannot catch two CLASS_COLORS
#        that converge to (near-)identical luminance, which is the actual
#        key the shader uses to sample provider_lut (`float key =
#        luminance(styled); texture(provider_lut, vec2(key, 0.5))`, see
#        palette_swap.gdshader fragment()). Two classes with near-equal
#        luminance would sample near-identical colors under EVERY provider,
#        silently collapsing part of the 7 x 5 variant matrix even though
#        both per-axis checks above still pass. This test computes
#        luminance(class_color) for all 7 classes exactly as the shader
#        does and asserts (a) no two classes are closer than a minimum
#        luminance gap, and (b) every class, run through every provider's
#        LUT at its own luminance key, is still pairwise-distinct from
#        every other class under that same provider.
#
# Exits 1 on any failure so it can be wired into CI later.

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_band_distinctness_cases(failures)
	_run_glow_monotonic_smooth_case(failures)
	_run_class_color_distinctness_case(failures)
	_run_provider_lut_distinctness_case(failures)
	_run_class_through_provider_lut_distinctness_case(failures)
	if failures.is_empty():
		print("palette_math_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("palette_math_test: FAIL — " + message)
		quit(1)


## Acceptance #2 — the 5 named bands must produce visually distinct effect
## tuples (saturation, metallic, glow, thread, iridescence), not just
## distinct power_level floats.
func _run_band_distinctness_cases(failures: Array[String]) -> void:
	var band_names: Array = PaletteMath.BANDS.keys()
	var tuples: Array = []
	for band_name: String in band_names:
		var power: float = PaletteMath.BANDS[band_name]
		tuples.append(PaletteMath.effects_for(power))
	for i: int in range(tuples.size()):
		for j: int in range(i + 1, tuples.size()):
			if _dict_approx_equal(tuples[i], tuples[j]):
				failures.append(
					"band distinctness: %s and %s produced identical effect tuples" %
					[band_names[i], band_names[j]]
				)


func _dict_approx_equal(a: Dictionary, b: Dictionary) -> bool:
	for key: String in a.keys():
		if not is_equal_approx(float(a[key]), float(b[key])):
			return false
	return true


## Acceptance #6 — glow_for(power) must be smooth (no discontinuous jumps
## at band boundaries) and monotonically non-decreasing across the full
## 0..1 domain, never discretized into steps.
func _run_glow_monotonic_smooth_case(failures: Array[String]) -> void:
	var prev: float = -1.0
	var max_step: float = 0.0
	var steps: int = 200
	for i: int in range(steps + 1):
		var p: float = float(i) / float(steps)
		var g: float = PaletteMath.glow_for(p)
		if g < prev - 0.0001:
			failures.append("glow_for not monotonic at p=%f: %f < prev %f" % [p, g, prev])
			break
		if prev >= 0.0:
			max_step = maxf(max_step, g - prev)
		prev = g
	# A single fine-grained step (1/steps of the domain) should never move
	# glow by more than a small fraction of its total range — a stepped
	# implementation would show one huge jump at each band boundary instead.
	if max_step > 0.05:
		failures.append("glow_for shows a discontinuous jump (%f) — looks stepped, not smooth" % max_step)
	if not is_equal_approx(PaletteMath.glow_for(0.0), 0.0):
		failures.append("glow_for(0.0) expected 0.0, got %f" % PaletteMath.glow_for(0.0))
	if not is_equal_approx(PaletteMath.glow_for(1.0), 1.0):
		failures.append("glow_for(1.0) expected 1.0, got %f" % PaletteMath.glow_for(1.0))


## Acceptance #3/#4 — the 7 character classes must have pairwise-distinct
## base colors (one axis of the class_color x provider_lut x power_level
## variant matrix).
func _run_class_color_distinctness_case(failures: Array[String]) -> void:
	var colors: Array = AgentCharacter.CLASS_COLORS.values()
	for i: int in range(colors.size()):
		for j: int in range(i + 1, colors.size()):
			var a: Color = colors[i]
			var b: Color = colors[j]
			if a.is_equal_approx(b):
				failures.append("class color distinctness: entries %d and %d are equal" % [i, j])
	if colors.size() != 7:
		failures.append("expected 7 CLASS_COLORS entries, found %d" % colors.size())


## Acceptance #3/#4 — the 5 provider LUTs (+ unknown fallback) must sample
## to pairwise-distinct colors at the same luminance key, so the provider
## axis actually differentiates the visual output.
func _run_provider_lut_distinctness_case(failures: Array[String]) -> void:
	var providers: Array[String] = ["claude", "gemini", "openai", "ollama", "deepseek"]
	var samples: Array = []
	for provider: String in providers:
		var tex: GradientTexture1D = ProviderPalette.get_lut(provider) as GradientTexture1D
		if tex == null:
			failures.append("provider_lut: get_lut(%s) did not return a GradientTexture1D" % provider)
			continue
		# Sample the gradient directly at its midpoint (0.5) — equivalent to
		# what the shader's luminance-keyed texture() lookup would read for a
		# mid-luminance input, without needing a running renderer.
		var sample: Color = tex.gradient.sample(0.5)
		samples.append(sample)
	for i: int in range(samples.size()):
		for j: int in range(i + 1, samples.size()):
			if (samples[i] as Color).is_equal_approx(samples[j] as Color):
				failures.append(
					"provider LUT distinctness: %s and %s sample identically at 0.5" %
					[providers[i], providers[j]]
				)
	# unknown must be a neutral grey and must never receive a nonzero blend.
	if not is_equal_approx(ProviderPalette.get_lut_mix("unknown"), 0.0):
		failures.append("provider_palette: get_lut_mix('unknown') expected 0.0")
	if not is_equal_approx(ProviderPalette.get_lut_mix(""), 0.0):
		failures.append("provider_palette: get_lut_mix('') expected 0.0")
	if is_equal_approx(ProviderPalette.get_lut_mix("claude"), 0.0):
		failures.append("provider_palette: get_lut_mix('claude') expected nonzero (recognized provider)")


## Acceptance #4 CROSS TERM (review finding 2) — the per-axis checks above
## (raw CLASS_COLORS pairwise-distinct; LUT samples pairwise-distinct at a
## FIXED key) cannot catch two classes whose *luminance* converges, even
## though luminance(class_color) is exactly the key the shader uses to
## sample provider_lut (see palette_swap.gdshader fragment(): `float key =
## clamp(luminance(styled), 0.0, 1.0); texture(provider_lut, vec2(key, 0.5))`).
## The saturation mix step (`mix(vec3(base_luma), base, saturation)`) is a
## luminance-preserving blend at saturation == 1.0, so at power_level == 0
## (saturation ~= 0.55, still centered on base_luma) styled's luminance
## tracks class_color's luminance closely — this test uses class_color's
## raw luminance directly as a conservative proxy for that shader key.
func _run_class_through_provider_lut_distinctness_case(failures: Array[String]) -> void:
	var providers: Array[String] = ["claude", "gemini", "openai", "ollama", "deepseek"]
	var class_ids: Array = AgentCharacter.CLASS_COLORS.keys()
	var class_names: Array = []
	for class_id: int in class_ids:
		class_names.append(AgentCharacter.CLASS_LABELS.get(class_id, str(class_id)))

	# (a) No two classes may sit closer than this in luminance — closer than
	# this and every provider's LUT (sampled at that near-identical key)
	# will return near-identical colors for both classes, regardless of how
	# distinct the LUTs or the raw class RGB values are individually. 0.01
	# is set just above the ~0.002 ARCHMAGE/LIBRARIAN collision this test
	# was written to catch (see class doc-comment fix in agent_character.gd)
	# — tight enough to fail on a true near-duplicate, loose enough not to
	# demand redesigning the other hand-picked class colors, some of which
	# sit ~0.012-0.016 apart by luminance already (e.g. ALCHEMIST/SCRIBE,
	# SCRIBE/WARDKEEPER) without being visually indistinguishable.
	var luma_min_gap: float = 0.01
	var lumas: Array[float] = []
	for class_id: int in class_ids:
		lumas.append(_luminance(AgentCharacter.CLASS_COLORS[class_id] as Color))
	for i: int in range(class_ids.size()):
		for j: int in range(i + 1, class_ids.size()):
			var gap: float = absf(lumas[i] - lumas[j])
			if gap < luma_min_gap:
				failures.append(
					("class luminance collision: %s (luma=%.4f) and %s (luma=%.4f) differ by " +
					"only %.4f (< %.4f) — the shader keys provider_lut lookup by luminance, " +
					"so these two classes would render as near-identical colors under every " +
					"provider LUT") %
					[class_names[i], lumas[i], class_names[j], lumas[j], gap, luma_min_gap]
				)

	# (b) Directly reproduce the shader's lookup: for every provider, sample
	# its LUT at each class's own luminance key and require the 7 resulting
	# colors to be pairwise distinct under that provider.
	for provider: String in providers:
		var tex: GradientTexture1D = ProviderPalette.get_lut(provider) as GradientTexture1D
		if tex == null:
			continue
		var swapped: Array[Color] = []
		for luma: float in lumas:
			swapped.append(tex.gradient.sample(clampf(luma, 0.0, 1.0)))
		for i: int in range(swapped.size()):
			for j: int in range(i + 1, swapped.size()):
				if swapped[i].is_equal_approx(swapped[j]):
					failures.append(
						"class x provider LUT distinctness: %s and %s sample identically through provider '%s' at their own luminance keys" %
						[class_names[i], class_names[j], provider]
					)


## Rec. 601-ish perceptual luminance, mirroring palette_swap.gdshader's
## luminance() function exactly so this test keys the LUT the same way the
## shader does.
func _luminance(c: Color) -> float:
	return c.r * 0.299 + c.g * 0.587 + c.b * 0.114
