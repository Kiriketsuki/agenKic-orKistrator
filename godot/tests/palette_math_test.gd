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
#
# Exits 1 on any failure so it can be wired into CI later.

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_band_distinctness_cases(failures)
	_run_glow_monotonic_smooth_case(failures)
	_run_class_color_distinctness_case(failures)
	_run_provider_lut_distinctness_case(failures)
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
