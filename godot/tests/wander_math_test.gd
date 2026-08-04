# wander_math_test.gd — Regression guard for the idle wander/walk math
# (wander_math.gd) that drives AgentCharacter's Task 3 idle wander state
# machine.
#
# No GUT (or other Godot test runner) is vendored in this project yet, so
# this is a standalone script runnable headless, mirroring
# particle_math_test.gd / panel_float_math_test.gd:
#
#   godot --headless --path godot --script tests/wander_math_test.gd
#
# Asserts:
#   - half_range_px() derives a sane, non-negative range from desk metrics,
#     and degenerates to 0 rather than negative when the margin swallows it.
#   - wander_bounds() centres correctly on home_x and collapses to a single
#     point at home_x when half_range_px <= 0.
#   - pseudo_random_for() is deterministic (same id+salt -> same value),
#     stays within [0, 1), and varies across different salts/ids so agents
#     don't wander in lockstep.
#   - pick_target_x() always lands within [min_x, max_x], including at the
#     rng extremes (0.0 -> min_x, 1.0 -> max_x) and with an out-of-range rng
#     input clamped defensively.
#   - has_arrived() respects ARRIVAL_THRESHOLD_PX exactly at the boundary.
#   - dwell_duration() always lands within [DWELL_MIN_SEC, DWELL_MAX_SEC].
#   - direction_sign() returns -1/0/+1 correctly, including the
#     is_equal_approx "already there" case.
#   - step_toward() never overshoots the target and never moves backward for
#     a negative speed/delta.
#
# Exits 1 on any failure so it can be wired into CI later.

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_half_range_cases(failures)
	_run_wander_bounds_cases(failures)
	_run_pseudo_random_cases(failures)
	_run_pick_target_x_cases(failures)
	_run_has_arrived_cases(failures)
	_run_dwell_duration_cases(failures)
	_run_direction_sign_cases(failures)
	_run_step_toward_cases(failures)
	if failures.is_empty():
		print("wander_math_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("wander_math_test: FAIL — " + message)
		quit(1)


## half_range_px() must derive a sane range from EdgeLayout-style desk
## metrics, and must never go negative when the margin exceeds the slot.
func _run_half_range_cases(failures: Array[String]) -> void:
	# EdgeLayout.DESK_WIDTH=20, DESK_SPACING=6 -> (20+6)/2=13, minus a 4px
	# margin -> 9.0.
	var r: float = WanderMath.half_range_px(20.0, 6.0, 4.0)
	if not is_equal_approx(r, 9.0):
		failures.append("half_range_px(20, 6, 4) expected 9.0, got %f" % r)
	# Margin swallowing the whole slot must clamp to 0.0, not negative.
	var r_zero: float = WanderMath.half_range_px(20.0, 6.0, 100.0)
	if r_zero != 0.0:
		failures.append("half_range_px(20, 6, 100) expected 0.0, got %f" % r_zero)


## wander_bounds() must centre on home_x and collapse to a point when
## half_range_px is non-positive.
func _run_wander_bounds_cases(failures: Array[String]) -> void:
	var bounds: Vector2 = WanderMath.wander_bounds(100.0, 9.0)
	if not is_equal_approx(bounds.x, 91.0) or not is_equal_approx(bounds.y, 109.0):
		failures.append("wander_bounds(100, 9) expected (91, 109), got %s" % bounds)
	var degenerate: Vector2 = WanderMath.wander_bounds(50.0, 0.0)
	if not is_equal_approx(degenerate.x, 50.0) or not is_equal_approx(degenerate.y, 50.0):
		failures.append("wander_bounds(50, 0) expected (50, 50), got %s" % degenerate)
	var negative_range: Vector2 = WanderMath.wander_bounds(50.0, -5.0)
	if not is_equal_approx(negative_range.x, 50.0) or not is_equal_approx(negative_range.y, 50.0):
		failures.append("wander_bounds(50, -5) expected (50, 50), got %s" % negative_range)


## pseudo_random_for() must be deterministic, stay in [0, 1), and vary
## across salts/ids (agents must not wander in lockstep).
func _run_pseudo_random_cases(failures: Array[String]) -> void:
	var a1: float = WanderMath.pseudo_random_for("agent-1", 0)
	var a1_again: float = WanderMath.pseudo_random_for("agent-1", 0)
	if a1 != a1_again:
		failures.append("pseudo_random_for('agent-1', 0) not deterministic: %f vs %f" % [a1, a1_again])
	if a1 < 0.0 or a1 >= 1.0:
		failures.append("pseudo_random_for('agent-1', 0) out of [0,1): %f" % a1)
	if WanderMath.pseudo_random_for("", 0) != 0.0:
		failures.append("pseudo_random_for('', 0) expected 0.0")
	# Different salts for the same agent should (almost always) differ —
	# collect several draws and require at least some variety, rather than
	# asserting every pair differs (a hash collision is theoretically legal).
	var draws: Dictionary = {}
	for salt: int in range(8):
		draws[WanderMath.pseudo_random_for("agent-1", salt)] = true
	if draws.size() < 6:
		failures.append("pseudo_random_for('agent-1', salt) too few distinct draws across 8 salts: %d" % draws.size())
	# Different agents at the same salt should also (almost always) differ.
	if WanderMath.pseudo_random_for("agent-1", 0) == WanderMath.pseudo_random_for("agent-2", 0):
		failures.append("pseudo_random_for differs-by-id case unexpectedly collided")


## pick_target_x() must always land within [min_x, max_x], including the rng
## extremes and an out-of-range rng input.
func _run_pick_target_x_cases(failures: Array[String]) -> void:
	var lo: float = -9.0
	var hi: float = 9.0
	var at_zero: float = WanderMath.pick_target_x(lo, hi, 0.0)
	if not is_equal_approx(at_zero, lo):
		failures.append("pick_target_x(-9, 9, 0.0) expected -9.0, got %f" % at_zero)
	var at_one: float = WanderMath.pick_target_x(lo, hi, 1.0)
	if not is_equal_approx(at_one, hi):
		failures.append("pick_target_x(-9, 9, 1.0) expected 9.0, got %f" % at_one)
	var at_half: float = WanderMath.pick_target_x(lo, hi, 0.5)
	if not is_equal_approx(at_half, 0.0):
		failures.append("pick_target_x(-9, 9, 0.5) expected 0.0, got %f" % at_half)
	var clamped_low: float = WanderMath.pick_target_x(lo, hi, -5.0)
	if clamped_low < lo or clamped_low > hi:
		failures.append("pick_target_x(-9, 9, -5.0) out of bounds: %f" % clamped_low)
	var clamped_high: float = WanderMath.pick_target_x(lo, hi, 5.0)
	if clamped_high < lo or clamped_high > hi:
		failures.append("pick_target_x(-9, 9, 5.0) out of bounds: %f" % clamped_high)


## has_arrived() must respect ARRIVAL_THRESHOLD_PX exactly at the boundary.
func _run_has_arrived_cases(failures: Array[String]) -> void:
	var threshold: float = WanderMath.ARRIVAL_THRESHOLD_PX
	if not WanderMath.has_arrived(10.0, 10.0):
		failures.append("has_arrived(10, 10) expected true")
	if not WanderMath.has_arrived(10.0, 10.0 + threshold):
		failures.append("has_arrived at exact threshold expected true")
	if WanderMath.has_arrived(10.0, 10.0 + threshold + 0.01):
		failures.append("has_arrived just past threshold expected false")


## dwell_duration() must always land within [DWELL_MIN_SEC, DWELL_MAX_SEC].
func _run_dwell_duration_cases(failures: Array[String]) -> void:
	var min_d: float = WanderMath.dwell_duration(0.0)
	if not is_equal_approx(min_d, WanderMath.DWELL_MIN_SEC):
		failures.append("dwell_duration(0.0) expected DWELL_MIN_SEC, got %f" % min_d)
	var max_d: float = WanderMath.dwell_duration(1.0)
	if not is_equal_approx(max_d, WanderMath.DWELL_MAX_SEC):
		failures.append("dwell_duration(1.0) expected DWELL_MAX_SEC, got %f" % max_d)
	for i: int in range(11):
		var rng: float = float(i) / 10.0
		var d: float = WanderMath.dwell_duration(rng)
		if d < WanderMath.DWELL_MIN_SEC - 0.0001 or d > WanderMath.DWELL_MAX_SEC + 0.0001:
			failures.append("dwell_duration(%f) out of bounds: %f" % [rng, d])


## direction_sign() must return -1/0/+1 correctly, including the
## is_equal_approx "already there" case.
func _run_direction_sign_cases(failures: Array[String]) -> void:
	if WanderMath.direction_sign(0.0, 10.0) != 1:
		failures.append("direction_sign(0, 10) expected 1")
	if WanderMath.direction_sign(10.0, 0.0) != -1:
		failures.append("direction_sign(10, 0) expected -1")
	if WanderMath.direction_sign(5.0, 5.0) != 0:
		failures.append("direction_sign(5, 5) expected 0")
	if WanderMath.direction_sign(5.0, 5.0000001) != 0:
		failures.append("direction_sign(5, 5.0000001) expected 0 (is_equal_approx)")


## step_toward() must never overshoot the target and must never move
## backward for a negative speed/delta.
func _run_step_toward_cases(failures: Array[String]) -> void:
	var stepped: float = WanderMath.step_toward(0.0, 10.0, 5.0, 1.0)
	if not is_equal_approx(stepped, 5.0):
		failures.append("step_toward(0, 10, 5, 1) expected 5.0, got %f" % stepped)
	var overshoot_guard: float = WanderMath.step_toward(0.0, 10.0, 5.0, 100.0)
	if not is_equal_approx(overshoot_guard, 10.0):
		failures.append("step_toward(0, 10, 5, 100) expected clamped 10.0, got %f" % overshoot_guard)
	var negative_speed: float = WanderMath.step_toward(0.0, 10.0, -5.0, 1.0)
	if not is_equal_approx(negative_speed, 0.0):
		failures.append("step_toward with negative speed expected to hold position at 0.0, got %f" % negative_speed)
	var negative_delta: float = WanderMath.step_toward(0.0, 10.0, 5.0, -1.0)
	if not is_equal_approx(negative_delta, 0.0):
		failures.append("step_toward with negative delta expected to hold position at 0.0, got %f" % negative_delta)
