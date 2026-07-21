# particle_math_test.gd — Regression guard for T17 (#127) tier particle
# effects math (particle_math.gd) and the procedural texture / accent-color
# cache it composes with (particle_textures.gd / provider_palette.gd).
#
# No GUT (or other Godot test runner) is vendored in this project yet, so
# this is a standalone script runnable headless, mirroring
# palette_math_test.gd / floor_morph_test.gd:
#
#   godot --headless --path godot --script tests/particle_math_test.gd
#
# Asserts (acceptance criteria for #127):
#   #1/#3 — tier thresholds are derived from PaletteMath.BANDS (no second
#        band table) and particles scale with power level (none -> sparse ->
#        orbiting -> trail), including exact-boundary and just-below cases.
#   #5    — budget-cap enforcement: emission_amount_for()/ambient_amount_for()
#        never exceed the caller-supplied budget, at any power level.
#   NONE tier (Novice/Adept) never emits (Adept's glow is shader-only, T16).
#   Amounts/lifetimes scale monotonically WITHIN a tier as power rises.
#   Ambient shimmer is enabled ONLY at Legendary (paired with TRAIL).
#   params_for() bundles every key _apply_particles() reads.
#   #2/#4 — ParticleTextures.get_texture() returns a distinct, non-null
#        Texture2D per known style (+ unknown-style fallback), and
#        ProviderPalette.get_accent_color() is pairwise-distinct across the
#        5 configured providers (parallel to T16's LUT-collision guard).
#
# Exits 1 on any failure so it can be wired into CI later.

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_tier_threshold_cases(failures)
	_run_none_tier_emits_nothing_case(failures)
	_run_budget_cap_cases(failures)
	_run_within_tier_monotonic_cases(failures)
	_run_ambient_only_legendary_case(failures)
	_run_params_for_keys_case(failures)
	_run_texture_distinctness_case(failures)
	_run_accent_color_distinctness_case(failures)
	if failures.is_empty():
		print("particle_math_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("particle_math_test: FAIL — " + message)
		quit(1)


## Tier boundaries must key exactly off PaletteMath.BANDS, including
## just-below-threshold cases (0.59/0.79/0.99) landing one tier lower.
func _run_tier_threshold_cases(failures: Array[String]) -> void:
	var cases: Array = [
		[0.0, ParticleMath.Tier.NONE],
		[PaletteMath.BANDS["novice"], ParticleMath.Tier.NONE],
		[PaletteMath.BANDS["adept"], ParticleMath.Tier.NONE],
		[0.59, ParticleMath.Tier.NONE],
		[PaletteMath.BANDS["master"], ParticleMath.Tier.SPARKLE],
		[0.7, ParticleMath.Tier.SPARKLE],
		[0.79, ParticleMath.Tier.SPARKLE],
		[PaletteMath.BANDS["grandmaster"], ParticleMath.Tier.ORBIT],
		[0.9, ParticleMath.Tier.ORBIT],
		[0.99, ParticleMath.Tier.ORBIT],
		[PaletteMath.BANDS["legendary"], ParticleMath.Tier.TRAIL],
		[1.0, ParticleMath.Tier.TRAIL],
	]
	for c: Array in cases:
		var power: float = c[0]
		var expected: int = c[1]
		var actual: int = ParticleMath.tier_for(power)
		if actual != expected:
			failures.append(
				"tier_for(%f) expected %d, got %d" % [power, expected, actual]
			)


## NONE tier (below Master / 0.6) must never emit and must produce zero
## amount at any budget.
func _run_none_tier_emits_nothing_case(failures: Array[String]) -> void:
	var none_powers: Array[float] = [0.0, 0.1, 0.35, 0.5, 0.59]
	for p: float in none_powers:
		if ParticleMath.emitting_for(p):
			failures.append("emitting_for(%f) expected false (NONE tier)" % p)
		var amount: int = ParticleMath.emission_amount_for(p, 100)
		if amount != 0:
			failures.append("emission_amount_for(%f, 100) expected 0, got %d" % [p, amount])
		if ParticleMath.ambient_enabled_for(p):
			failures.append("ambient_enabled_for(%f) expected false (NONE tier)" % p)


## Acceptance #5 — budget cap must be respected at every tier, including a
## budget small enough to actually clamp the raw amount.
func _run_budget_cap_cases(failures: Array[String]) -> void:
	var powers: Array[float] = [0.6, 0.7, 0.8, 0.9, 1.0]
	var small_budget: int = 2
	for p: float in powers:
		var amount: int = ParticleMath.emission_amount_for(p, small_budget)
		if amount > small_budget:
			failures.append(
				"emission_amount_for(%f, %d) exceeded budget: got %d" % [p, small_budget, amount]
			)
	# Large budget should not clamp below the raw amount computed with a
	# generous cap.
	var generous: int = 1000
	if ParticleMath.emission_amount_for(1.0, generous) <= 0:
		failures.append("emission_amount_for(1.0, 1000) expected > 0")
	# Ambient must also respect a tiny budget.
	var ambient_amount: int = ParticleMath.ambient_amount_for(1.0, 1)
	if ambient_amount > 1:
		failures.append("ambient_amount_for(1.0, 1) exceeded budget: got %d" % ambient_amount)
	# Negative budget must never yield a negative/invalid amount.
	if ParticleMath.emission_amount_for(1.0, -5) != 0:
		failures.append("emission_amount_for(1.0, -5) expected 0 for negative budget")
	if ParticleMath.ambient_amount_for(1.0, -5) != 0:
		failures.append("ambient_amount_for(1.0, -5) expected 0 for negative budget")


## Within a tier, amount/lifetime must be monotonically non-decreasing as
## power rises toward the next band (continuous scaling, not a hard step).
func _run_within_tier_monotonic_cases(failures: Array[String]) -> void:
	var budget: int = 1000
	var ranges: Array = [
		["SPARKLE", PaletteMath.BANDS["master"], PaletteMath.BANDS["grandmaster"]],
		["ORBIT", PaletteMath.BANDS["grandmaster"], PaletteMath.BANDS["legendary"]],
	]
	for r: Array in ranges:
		var label: String = r[0]
		var lo: float = r[1]
		var hi: float = r[2]
		var prev_amount: int = -1
		var prev_lifetime: float = -1.0
		var steps: int = 10
		for i: int in range(steps + 1):
			var p: float = lerpf(lo, hi - 0.001, float(i) / float(steps))
			var amount: int = ParticleMath.emission_amount_for(p, budget)
			var lifetime: float = ParticleMath.lifetime_for(p)
			if amount < prev_amount:
				failures.append(
					"%s tier: emission_amount_for not monotonic at p=%f (%d < prev %d)" %
					[label, p, amount, prev_amount]
				)
			if lifetime < prev_lifetime - 0.0001:
				failures.append(
					"%s tier: lifetime_for not monotonic at p=%f (%f < prev %f)" %
					[label, p, lifetime, prev_lifetime]
				)
			prev_amount = amount
			prev_lifetime = lifetime


## Ambient shimmer ($AmbientParticles) must be enabled ONLY at the Legendary
## band (paired with TRAIL — spec Decision 2/5), never below it.
func _run_ambient_only_legendary_case(failures: Array[String]) -> void:
	var below: Array[float] = [0.0, 0.6, 0.8, 0.9, 0.999]
	for p: float in below:
		if ParticleMath.ambient_enabled_for(p):
			failures.append("ambient_enabled_for(%f) expected false (below Legendary)" % p)
	if not ParticleMath.ambient_enabled_for(1.0):
		failures.append("ambient_enabled_for(1.0) expected true (Legendary)")
	if ParticleMath.ambient_amount_for(1.0, 1000) <= 0:
		failures.append("ambient_amount_for(1.0, 1000) expected > 0")


## params_for() must bundle every key _apply_particles() reads.
func _run_params_for_keys_case(failures: Array[String]) -> void:
	var expected_keys: Array[String] = [
		"tier", "emitting", "amount", "lifetime", "orbit_radius",
		"orbit_speed", "plume_velocity", "ambient_enabled", "ambient_amount",
	]
	var params: Dictionary = ParticleMath.params_for(1.0, 24)
	for key: String in expected_keys:
		if not params.has(key):
			failures.append("params_for() missing expected key '%s'" % key)


## Acceptance #2/#4 — each known shape style must yield a distinct, non-null
## texture; an unknown style must fall back to "dot" (same cached instance).
func _run_texture_distinctness_case(failures: Array[String]) -> void:
	ParticleTextures._reset_cache_for_tests()
	var styles: Array[String] = ["dot", "spark", "shard", "ring"]
	var textures: Array = []
	for style: String in styles:
		var tex: Texture2D = ParticleTextures.get_texture(style)
		if tex == null:
			failures.append("ParticleTextures.get_texture('%s') returned null" % style)
			continue
		textures.append(tex)
	for i: int in range(textures.size()):
		for j: int in range(i + 1, textures.size()):
			if textures[i] == textures[j]:
				failures.append(
					"ParticleTextures: styles '%s' and '%s' returned the same texture instance" %
					[styles[i], styles[j]]
				)
	var unknown_tex: Texture2D = ParticleTextures.get_texture("does-not-exist")
	var dot_tex: Texture2D = ParticleTextures.get_texture("dot")
	if unknown_tex != dot_tex:
		failures.append("ParticleTextures.get_texture('does-not-exist') expected to fall back to 'dot'")


## Acceptance #2 cross term (parallel to T16's LUT-collision guard) — the 5
## configured providers' particle accent colors must be pairwise distinct,
## and unknown/empty must be a neutral grey.
func _run_accent_color_distinctness_case(failures: Array[String]) -> void:
	ProviderPalette._reset_cache_for_tests()
	var providers: Array[String] = ["claude", "gemini", "openai", "ollama", "deepseek"]
	var colors: Array[Color] = []
	for provider: String in providers:
		colors.append(ProviderPalette.get_accent_color(provider))
	for i: int in range(colors.size()):
		for j: int in range(i + 1, colors.size()):
			if colors[i].is_equal_approx(colors[j]):
				failures.append(
					"accent color distinctness: %s and %s produced equal colors" %
					[providers[i], providers[j]]
				)
	if not ProviderPalette.get_accent_color("unknown").is_equal_approx(Color(0.7, 0.7, 0.7, 1.0)):
		failures.append("get_accent_color('unknown') expected neutral grey")
	if not ProviderPalette.get_accent_color("").is_equal_approx(Color(0.7, 0.7, 0.7, 1.0)):
		failures.append("get_accent_color('') expected neutral grey")
	# get_particle_style: known styles ride through unchanged; unknown
	# provider or missing key falls back to 'dot'.
	if ProviderPalette.get_particle_style("gemini") != "shard":
		failures.append("get_particle_style('gemini') expected 'shard'")
	if ProviderPalette.get_particle_style("does-not-exist") != "dot":
		failures.append("get_particle_style('does-not-exist') expected fallback 'dot'")
