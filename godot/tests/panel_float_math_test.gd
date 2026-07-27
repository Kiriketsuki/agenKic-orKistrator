# panel_float_math_test.gd — Regression guard for T18 (#129) panel floaty
# animation math (panel_float_math.gd).
#
# No GUT (or other Godot test runner) is vendored in this project yet, so
# this is a standalone script runnable headless, mirroring
# particle_math_test.gd / palette_math_test.gd / floor_morph_test.gd:
#
#   godot --headless --path godot --script tests/panel_float_math_test.gd

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_bob_bounds_and_period_cases(failures)
	_run_bob_degenerate_cases(failures)
	_run_bob_snapped_cases(failures)
	_run_phase_determinism_case(failures)
	_run_phase_desynchronises_bob_case(failures)
	_run_flutter_param_cases(failures)
	_run_shimmer_bounds_case(failures)
	_run_trail_budget_cap_case(failures)
	_run_trail_zero_budget_disables_case(failures)
	_run_trail_below_threshold_case(failures)
	_run_trail_monotonic_case(failures)
	_run_trail_keys_case(failures)
	if failures.is_empty():
		print("panel_float_math_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("panel_float_math_test: FAIL — " + message)
		quit(1)


## bob_offset() must stay within [-amplitude, amplitude], start at 0 at
## t=0, repeat every `period`, and actually reach near both extremes
## somewhere in the sweep (proving it oscillates rather than sitting flat).
func _run_bob_bounds_and_period_cases(failures: Array[String]) -> void:
	var amplitude: float = 2.0
	var period: float = 4.0
	var phase: float = 0.0
	var max_v: float = -INF
	var min_v: float = INF
	var t: float = 0.0
	var step: float = 0.01
	var limit: float = 3.0 * period
	while t <= limit:
		var v: float = PanelFloatMath.bob_offset(t, amplitude, period, phase)
		if absf(v) > amplitude + 1e-5:
			failures.append("bob_offset(%f) exceeded amplitude: %f" % [t, v])
		max_v = maxf(max_v, v)
		min_v = minf(min_v, v)
		t += step
	if absf(PanelFloatMath.bob_offset(0.0, amplitude, period, 0.0)) > 1e-5:
		failures.append("bob_offset(0.0, ..., phase=0.0) expected ~0.0")
	for check_t: float in [0.3, 1.1, 2.7]:
		var a: float = PanelFloatMath.bob_offset(check_t, amplitude, period, phase)
		var b: float = PanelFloatMath.bob_offset(check_t + period, amplitude, period, phase)
		if not is_equal_approx(a, b):
			failures.append("bob_offset not periodic at t=%f: %f vs %f" % [check_t, a, b])
	if max_v < amplitude * 0.99:
		failures.append("bob_offset never reached +amplitude within sweep (max=%f)" % max_v)
	if min_v > -amplitude * 0.99:
		failures.append("bob_offset never reached -amplitude within sweep (min=%f)" % min_v)


## Degenerate configs (zero/negative amplitude or period) must return exactly
## 0.0 — no NaN, no divide-by-zero.
func _run_bob_degenerate_cases(failures: Array[String]) -> void:
	var degenerate_cases: Array = [
		[0.0, 4.0], [-5.0, 4.0], [2.0, 0.0], [2.0, -1.0],
	]
	for c: Array in degenerate_cases:
		var amplitude: float = c[0]
		var period: float = c[1]
		for t: float in [0.0, 1.0, 2.5]:
			var v: float = PanelFloatMath.bob_offset(t, amplitude, period, 0.5)
			if not is_equal_approx(v, 0.0):
				failures.append(
					"bob_offset(%f, amp=%f, period=%f) expected 0.0, got %f" % [t, amplitude, period, v]
				)


## bob_offset_snapped() must always return an integral pixel value within
## ceil(amplitude), and degenerate configs still return 0.0.
func _run_bob_snapped_cases(failures: Array[String]) -> void:
	var amplitude: float = 2.0
	var period: float = 4.0
	var phase: float = 0.9
	var t: float = 0.0
	var step: float = 0.05
	var limit: float = 2.0 * period
	while t <= limit:
		var v: float = PanelFloatMath.bob_offset_snapped(t, amplitude, period, phase)
		if not is_equal_approx(v, roundf(v)):
			failures.append("bob_offset_snapped(%f) not integral: %f" % [t, v])
		if absf(v) > ceilf(amplitude) + 1e-5:
			failures.append("bob_offset_snapped(%f) exceeded ceil(amplitude): %f" % [t, v])
		t += step
	if not is_equal_approx(PanelFloatMath.bob_offset_snapped(1.0, 0.0, period, phase), 0.0):
		failures.append("bob_offset_snapped with amplitude 0.0 expected 0.0")


## phase_for_id() must be deterministic, pairwise-distinct across different
## ids, always in [0, TAU), and 0.0 for an empty id.
func _run_phase_determinism_case(failures: Array[String]) -> void:
	var ids: Array[String] = ["spell-scroll", "quest-board", "agent-alpha", "agent-beta", "agent-gamma"]
	var first: float = PanelFloatMath.phase_for_id("spell-scroll")
	var second: float = PanelFloatMath.phase_for_id("spell-scroll")
	if not is_equal_approx(first, second):
		failures.append("phase_for_id not deterministic: %f vs %f" % [first, second])
	var phases: Array[float] = []
	for id: String in ids:
		var p: float = PanelFloatMath.phase_for_id(id)
		if p < 0.0 or p >= TAU:
			failures.append("phase_for_id(%s) out of [0, TAU): %f" % [id, p])
		phases.append(p)
	for i: int in range(phases.size()):
		for j: int in range(i + 1, phases.size()):
			if is_equal_approx(phases[i], phases[j]):
				failures.append("phase_for_id collision: %s and %s" % [ids[i], ids[j]])
	if not is_equal_approx(PanelFloatMath.phase_for_id(""), 0.0):
		failures.append("phase_for_id('') expected 0.0")


## The deterministic per-panel phase must actually desynchronise the bob
## curve across panels at a fixed instant — the acceptance intent behind it.
func _run_phase_desynchronises_bob_case(failures: Array[String]) -> void:
	var ids: Array[String] = ["spell-scroll", "quest-board", "agent-alpha", "agent-beta", "agent-gamma"]
	var offsets: Array[float] = []
	for id: String in ids:
		var phase: float = PanelFloatMath.phase_for_id(id)
		offsets.append(PanelFloatMath.bob_offset(1.7, 2.0, 4.0, phase))
	var all_equal: bool = true
	for i: int in range(1, offsets.size()):
		if not is_equal_approx(offsets[i], offsets[0]):
			all_equal = false
			break
	if all_equal:
		failures.append("bob_offset values identical across distinct phases at fixed t (lockstep bug)")


## flutter_uv_amplitude()/flutter_speed_rad() edge cases and value checks.
func _run_flutter_param_cases(failures: Array[String]) -> void:
	var uv: float = PanelFloatMath.flutter_uv_amplitude(1.5, 300.0)
	if not is_equal_approx(uv, 0.005):
		failures.append("flutter_uv_amplitude(1.5, 300.0) expected ~0.005, got %f" % uv)
	if not is_equal_approx(PanelFloatMath.flutter_uv_amplitude(1.5, 0.0), 0.0):
		failures.append("flutter_uv_amplitude with zero height expected 0.0")
	if not is_equal_approx(PanelFloatMath.flutter_uv_amplitude(0.0, 300.0), 0.0):
		failures.append("flutter_uv_amplitude with zero amplitude expected 0.0")
	if not is_equal_approx(PanelFloatMath.flutter_uv_amplitude(1.5, -10.0), 0.0):
		failures.append("flutter_uv_amplitude with negative height expected 0.0")
	var low: float = PanelFloatMath.flutter_uv_amplitude(1.0, 300.0)
	var high: float = PanelFloatMath.flutter_uv_amplitude(2.0, 300.0)
	if not (high > low):
		failures.append("flutter_uv_amplitude not monotone increasing in amplitude")
	var speed: float = PanelFloatMath.flutter_speed_rad(4.0)
	var expected_speed: float = TAU / 4.0 * PanelFloatMath.FLUTTER_SPEED_RATIO
	if not is_equal_approx(speed, expected_speed):
		failures.append("flutter_speed_rad(4.0) expected %f, got %f" % [expected_speed, speed])
	if not is_equal_approx(PanelFloatMath.flutter_speed_rad(0.0), 0.0):
		failures.append("flutter_speed_rad(0.0) expected 0.0")
	if not is_equal_approx(PanelFloatMath.flutter_speed_rad(-1.0), 0.0):
		failures.append("flutter_speed_rad(-1.0) expected 0.0")


## shimmer_alpha() must stay within [SHIMMER_ALPHA_MIN, SHIMMER_ALPHA_MAX],
## approach both extremes, and repeat every SHIMMER_PERIOD_SEC.
func _run_shimmer_bounds_case(failures: Array[String]) -> void:
	var phase: float = 0.3
	var max_v: float = -INF
	var min_v: float = INF
	var t: float = 0.0
	var step: float = 0.02
	var limit: float = 2.0 * PanelFloatMath.SHIMMER_PERIOD_SEC
	while t <= limit:
		var v: float = PanelFloatMath.shimmer_alpha(t, phase)
		if v < PanelFloatMath.SHIMMER_ALPHA_MIN - 1e-5 or v > PanelFloatMath.SHIMMER_ALPHA_MAX + 1e-5:
			failures.append("shimmer_alpha(%f) out of bounds: %f" % [t, v])
		max_v = maxf(max_v, v)
		min_v = minf(min_v, v)
		t += step
	var range_span: float = PanelFloatMath.SHIMMER_ALPHA_MAX - PanelFloatMath.SHIMMER_ALPHA_MIN
	if max_v < PanelFloatMath.SHIMMER_ALPHA_MAX - 0.01 * range_span:
		failures.append("shimmer_alpha never approached max (max=%f)" % max_v)
	if min_v > PanelFloatMath.SHIMMER_ALPHA_MIN + 0.01 * range_span:
		failures.append("shimmer_alpha never approached min (min=%f)" % min_v)
	for check_t: float in [0.2, 1.0, 2.0]:
		var a: float = PanelFloatMath.shimmer_alpha(check_t, phase)
		var b: float = PanelFloatMath.shimmer_alpha(check_t + PanelFloatMath.SHIMMER_PERIOD_SEC, phase)
		if not is_equal_approx(a, b):
			failures.append("shimmer_alpha not periodic at t=%f" % check_t)


## drag_trail_params()'s amount must never exceed the budget (clamped to >=0)
## across a matrix of budgets and speeds.
func _run_trail_budget_cap_case(failures: Array[String]) -> void:
	var budgets: Array[int] = [0, 1, 3, 12, 100]
	var speeds: Array[float] = [0.0, 44.0, 45.0, 200.0, 900.0, 5000.0]
	for budget: int in budgets:
		for speed: float in speeds:
			var params: Dictionary = PanelFloatMath.drag_trail_params(speed, budget)
			var amount: int = int(params["amount"])
			if amount > maxi(budget, 0):
				failures.append(
					"drag_trail_params(speed=%f, budget=%d): amount %d exceeded budget" % [speed, budget, amount]
				)
			if amount < 0:
				failures.append("drag_trail_params(speed=%f, budget=%d): negative amount %d" % [speed, budget, amount])


## A budget of 0 must disable emission at every speed — amount must be
## exactly 0, never floored up to 1.
func _run_trail_zero_budget_disables_case(failures: Array[String]) -> void:
	for speed: float in [0.0, 50.0, 500.0, 5000.0]:
		var params: Dictionary = PanelFloatMath.drag_trail_params(speed, 0)
		if bool(params["emitting"]):
			failures.append("drag_trail_params(speed=%f, budget=0): emitting expected false" % speed)
		if int(params["amount"]) != 0:
			failures.append("drag_trail_params(speed=%f, budget=0): amount expected 0, got %d" % [speed, params["amount"]])


## Below the speed threshold, the trail must be fully off regardless of a
## generous budget.
func _run_trail_below_threshold_case(failures: Array[String]) -> void:
	for speed: float in [0.0, 10.0, 44.9]:
		var params: Dictionary = PanelFloatMath.drag_trail_params(speed, 100)
		if bool(params["emitting"]):
			failures.append("drag_trail_params(speed=%f, budget=100): emitting expected false" % speed)
		if int(params["amount"]) != 0:
			failures.append("drag_trail_params(speed=%f): amount expected 0" % speed)
		if not is_equal_approx(float(params["lifetime"]), 0.0):
			failures.append("drag_trail_params(speed=%f): lifetime expected 0.0" % speed)
		if not is_equal_approx(float(params["velocity"]), 0.0):
			failures.append("drag_trail_params(speed=%f): velocity expected 0.0" % speed)


## With a generous budget, amount/lifetime/velocity must be non-decreasing as
## speed rises through the response range.
func _run_trail_monotonic_case(failures: Array[String]) -> void:
	var budget: int = 100
	var speeds: Array[float] = [45.0, 100.0, 300.0, 600.0, 900.0, 2000.0]
	var prev_amount: int = -1
	var prev_lifetime: float = -1.0
	var prev_velocity: float = -1.0
	for speed: float in speeds:
		var params: Dictionary = PanelFloatMath.drag_trail_params(speed, budget)
		var amount: int = int(params["amount"])
		var lifetime: float = float(params["lifetime"])
		var velocity: float = float(params["velocity"])
		if amount < prev_amount:
			failures.append("drag_trail_params amount not monotonic at speed=%f" % speed)
		if lifetime < prev_lifetime - 0.0001:
			failures.append("drag_trail_params lifetime not monotonic at speed=%f" % speed)
		if velocity < prev_velocity - 0.0001:
			failures.append("drag_trail_params velocity not monotonic at speed=%f" % speed)
		prev_amount = amount
		prev_lifetime = lifetime
		prev_velocity = velocity


## drag_trail_params() must return exactly the keys panel_base.gd reads.
func _run_trail_keys_case(failures: Array[String]) -> void:
	var expected_keys: Array[String] = ["emitting", "amount", "lifetime", "velocity"]
	var params: Dictionary = PanelFloatMath.drag_trail_params(500.0, 12)
	for key: String in expected_keys:
		if not params.has(key):
			failures.append("drag_trail_params() missing expected key '%s'" % key)
	if params.size() != expected_keys.size():
		failures.append("drag_trail_params() returned unexpected extra keys: %s" % [params.keys()])
