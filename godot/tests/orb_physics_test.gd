# orb_physics_test.gd — Regression guard for F2 (#175) orb flock physics
# math (orb_physics.gd).
#
# No GUT (or other Godot test runner) is vendored in this project yet, so
# this is a standalone script runnable headless, mirroring
# panel_float_math_test.gd / particle_math_test.gd / palette_math_test.gd:
#
#   godot --headless --path godot --script tests/orb_physics_test.gd

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_nearest_edge_cases(failures)
	_run_edge_dock_anchor_cases(failures)
	_run_spring_step_converges_case(failures)
	_run_spring_step_settles_case(failures)
	_run_flick_velocity_cases(failures)
	_run_momentum_bounce_case(failures)
	_run_momentum_ricochet_fraction_case(failures)
	_run_momentum_drag_settles_case(failures)
	_run_stack_offset_cases(failures)
	if failures.is_empty():
		print("orb_physics_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("orb_physics_test: FAIL — " + message)
		quit(1)


## nearest_edge() must pick the closer side, and ties between the side axis
## and the vertical axis resolve toward the side (LEFT/RIGHT).
func _run_nearest_edge_cases(failures: Array[String]) -> void:
	var viewport: Vector2 = Vector2(1000.0, 800.0)
	var cases: Array[Dictionary] = [
		{"pos": Vector2(50.0, 400.0), "expect": OrbPhysics.Edge.LEFT},
		{"pos": Vector2(950.0, 400.0), "expect": OrbPhysics.Edge.RIGHT},
		{"pos": Vector2(500.0, 20.0), "expect": OrbPhysics.Edge.TOP},
		{"pos": Vector2(500.0, 780.0), "expect": OrbPhysics.Edge.BOTTOM},
		{"pos": Vector2(0.0, 0.0), "expect": OrbPhysics.Edge.LEFT},
	]
	for c: Dictionary in cases:
		var got: OrbPhysics.Edge = OrbPhysics.nearest_edge(c["pos"], viewport)
		if got != c["expect"]:
			failures.append("nearest_edge(%s) expected %d, got %d" % [c["pos"], c["expect"], got])


## edge_dock_anchor() must sit `radius + margin` off the chosen edge and
## clamp the along-edge coordinate so the whole orb stays on-screen.
func _run_edge_dock_anchor_cases(failures: Array[String]) -> void:
	var viewport: Vector2 = Vector2(1000.0, 800.0)
	var radius: float = 24.0
	var right_anchor: Vector2 = OrbPhysics.edge_dock_anchor(OrbPhysics.Edge.RIGHT, viewport, radius, 400.0)
	var expected_x: float = viewport.x - radius - OrbPhysics.DOCK_MARGIN_PX
	if not is_equal_approx(right_anchor.x, expected_x):
		failures.append("edge_dock_anchor RIGHT.x expected %f, got %f" % [expected_x, right_anchor.x])
	if not is_equal_approx(right_anchor.y, 400.0):
		failures.append("edge_dock_anchor RIGHT.y expected 400.0, got %f" % right_anchor.y)
	var clamped: Vector2 = OrbPhysics.edge_dock_anchor(OrbPhysics.Edge.RIGHT, viewport, radius, -500.0)
	var min_height: float = radius + OrbPhysics.DOCK_MARGIN_PX
	if clamped.y < min_height - 0.01:
		failures.append("edge_dock_anchor RIGHT clamp expected >= %f, got %f" % [min_height, clamped.y])
	var over: Vector2 = OrbPhysics.edge_dock_anchor(OrbPhysics.Edge.RIGHT, viewport, radius, 5000.0)
	var max_height: float = viewport.y - radius - OrbPhysics.DOCK_MARGIN_PX
	if over.y > max_height + 0.01:
		failures.append("edge_dock_anchor RIGHT clamp expected <= %f, got %f" % [max_height, over.y])


## spring_step() must move the position toward the target each tick without
## ever diverging away from it once past the initial transient.
func _run_spring_step_converges_case(failures: Array[String]) -> void:
	var current: Vector2 = Vector2(500.0, 300.0)
	var target: Vector2 = Vector2(950.0, 300.0)
	var velocity: Vector2 = Vector2.ZERO
	var delta: float = 1.0 / 60.0
	var previous_distance: float = current.distance_to(target)
	var steps: int = 0
	while steps < 600:
		var result: Dictionary = OrbPhysics.spring_step(current, target, velocity, delta)
		current = result["position"]
		velocity = result["velocity"]
		var distance: float = current.distance_to(target)
		# Allow a small overshoot tolerance for the underdamped transient;
		# the case that matters is that it does not run away.
		if distance > previous_distance + 50.0:
			failures.append("spring_step diverged: distance grew from %f to %f" % [previous_distance, distance])
			break
		previous_distance = distance
		if result["settled"]:
			break
		steps += 1
	if steps >= 600:
		failures.append("spring_step never settled within 600 steps (10s)")


## spring_step() must report settled:true once within tolerance, and the
## returned position must then equal the target exactly (a hard snap, no
## residual drift).
func _run_spring_step_settles_case(failures: Array[String]) -> void:
	var current: Vector2 = Vector2(100.0, 100.0)
	var target: Vector2 = Vector2(100.4, 100.0)
	var result: Dictionary = OrbPhysics.spring_step(current, target, Vector2.ZERO, 1.0 / 60.0)
	if not result["settled"]:
		failures.append("spring_step expected immediate settle for a tiny displacement at rest")
	if result["position"] != target:
		failures.append("spring_step settled position expected to snap exactly to target")


## flick_velocity() must scale displacement by 1/delta, cap at max_speed,
## and return zero for a non-positive delta instead of dividing by zero.
func _run_flick_velocity_cases(failures: Array[String]) -> void:
	var v: Vector2 = OrbPhysics.flick_velocity(Vector2(0.0, 0.0), Vector2(10.0, 0.0), 0.1, 1000.0)
	if not v.is_equal_approx(Vector2(100.0, 0.0)):
		failures.append("flick_velocity expected (100, 0), got %s" % v)
	var capped: Vector2 = OrbPhysics.flick_velocity(Vector2(0.0, 0.0), Vector2(1000.0, 0.0), 0.01, 500.0)
	if not is_equal_approx(capped.length(), 500.0):
		failures.append("flick_velocity expected capped length 500.0, got %f" % capped.length())
	var zero_delta: Vector2 = OrbPhysics.flick_velocity(Vector2.ZERO, Vector2(10.0, 10.0), 0.0, 500.0)
	if zero_delta != Vector2.ZERO:
		failures.append("flick_velocity(delta=0) expected Vector2.ZERO, got %s" % zero_delta)


## momentum_step() must reflect vertical velocity off the top/bottom bounds
## and clamp position within the viewport.
func _run_momentum_bounce_case(failures: Array[String]) -> void:
	var viewport: Vector2 = Vector2(1000.0, 800.0)
	var radius: float = 24.0
	var position: Vector2 = Vector2(500.0, 30.0)
	var velocity: Vector2 = Vector2(0.0, -900.0)
	var result: Dictionary = OrbPhysics.momentum_step(position, velocity, 1.0 / 60.0, viewport, radius, 0.62)
	var next_position: Vector2 = result["position"]
	var next_velocity: Vector2 = result["velocity"]
	if next_position.y < radius - 0.01:
		failures.append("momentum_step bounce expected y clamped to >= radius, got %f" % next_position.y)
	if next_velocity.y <= 0.0:
		failures.append("momentum_step bounce expected reflected (positive) velocity.y, got %f" % next_velocity.y)


## A top bounce must keep exactly ricochet_fraction of the incoming speed
## (ignoring the small per-step drag), matching acceptance: "a top-edge
## bounce keeps the configured ricochet fraction of its speed".
func _run_momentum_ricochet_fraction_case(failures: Array[String]) -> void:
	var viewport: Vector2 = Vector2(1000.0, 800.0)
	var radius: float = 24.0
	var incoming_speed: float = 900.0
	var position: Vector2 = Vector2(500.0, 30.0)
	var velocity: Vector2 = Vector2(0.0, -incoming_speed)
	var ricochet: float = 0.5
	var result: Dictionary = OrbPhysics.momentum_step(position, velocity, 1.0 / 60.0, viewport, radius, ricochet)
	var next_velocity: Vector2 = result["velocity"]
	var expected_min: float = incoming_speed * ricochet * 0.9
	if next_velocity.y < expected_min:
		failures.append("momentum_step ricochet expected velocity.y >= %f, got %f" % [expected_min, next_velocity.y])


## Momentum must decay to exactly Vector2.ZERO (not merely small) once
## below the stop-speed threshold, so a flick eventually docks rather than
## drifting forever.
func _run_momentum_drag_settles_case(failures: Array[String]) -> void:
	var viewport: Vector2 = Vector2(1000.0, 800.0)
	var radius: float = 24.0
	var position: Vector2 = Vector2(500.0, 400.0)
	var velocity: Vector2 = Vector2(10.0, 0.0)
	var delta: float = 1.0 / 60.0
	var steps: int = 0
	while velocity != Vector2.ZERO and steps < 600:
		var result: Dictionary = OrbPhysics.momentum_step(position, velocity, delta, viewport, radius, 0.62)
		position = result["position"]
		velocity = result["velocity"]
		steps += 1
	if velocity != Vector2.ZERO:
		failures.append("momentum_step drag never settled to Vector2.ZERO within 600 steps")


## stack_offset() must return ZERO for the lead orb and a monotonically
## growing inward/down offset for trailing orbs, so later orbs step behind
## earlier ones instead of overlapping exactly.
func _run_stack_offset_cases(failures: Array[String]) -> void:
	var lead: Vector2 = OrbPhysics.stack_offset(0, OrbPhysics.Edge.RIGHT)
	if lead != Vector2.ZERO:
		failures.append("stack_offset(0, ...) expected Vector2.ZERO, got %s" % lead)
	var second: Vector2 = OrbPhysics.stack_offset(1, OrbPhysics.Edge.RIGHT)
	var third: Vector2 = OrbPhysics.stack_offset(2, OrbPhysics.Edge.RIGHT)
	if absf(third.x) <= absf(second.x):
		failures.append("stack_offset RIGHT expected growing inward magnitude, got second=%s third=%s" % [second, third])
	if third.y <= second.y:
		failures.append("stack_offset RIGHT expected growing downward offset, got second=%s third=%s" % [second, third])
