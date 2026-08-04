class_name WanderMath
## T-wander (agent idle wander/walk, tower UI/UX overhaul) — pure math for the
## per-agent idle wander state machine driven from agent_character.gd. Mirrors
## the FloorMorph / PaletteMath / ParticleMath / PanelFloatMath idiom: every
## curve/helper here is static and node-free, so it is headlessly testable
## (tests/wander_math_test.gd) without a running engine. No Node, Tween,
## SceneTree, or AnimatedSprite2D reference anywhere in this file —
## agent_character.gd owns the actual per-frame position mutation and
## AnimatedSprite2D/flip_h wiring, and calls into these helpers for target
## selection, bounds clamping, arrival testing, and dwell timing.
##
## Bounds are deliberately NOT hardcoded pixel constants: callers derive
## half_range_px() from EdgeLayout.DESK_WIDTH / EdgeLayout.DESK_SPACING (the
## same desk-slot metrics floor_scene.gd already uses to lay agents out), so
## an agent's wander range always stays inside its own desk slot and never
## reaches a neighbour's — see agent_character.gd's _init_wander().

## Dwell (standing still, playing "idle") duration bounds, in seconds.
const DWELL_MIN_SEC: float = 1.5
const DWELL_MAX_SEC: float = 4.0

## An agent is considered "arrived" once within this many px of its target —
## exact float equality would stall on floating-point rounding.
const ARRIVAL_THRESHOLD_PX: float = 0.5

## Deterministic-phase bucket count. 3600 buckets ≈ 0.1° resolution — far
## finer than perceptible, while keeping the modulo integral and stable.
## Mirrors PanelFloatMath.PHASE_BUCKETS.
const RNG_BUCKETS: int = 3600


## Half-range (px) an agent may wander from its home/desk x, in EACH
## direction. Derived from the desk-slot metrics (desk width + the spacing to
## the next desk), minus a safety margin, so a wandering agent can never
## reach — let alone cross — a neighbouring desk. A non-positive result
## (desk_width/spacing too small, or margin too generous) clamps to 0.0, i.e.
## the agent simply does not wander rather than wandering a negative range.
static func half_range_px(desk_width: float, desk_spacing: float, margin_px: float) -> float:
	return maxf((desk_width + desk_spacing) / 2.0 - margin_px, 0.0)


## Wander bounds [min_x, max_x] centred on home_x. A non-positive
## half_range_px collapses both bounds to home_x — a degenerate "no room to
## wander" config must produce a single valid point, never a NaN or an
## inverted range.
static func wander_bounds(home_x: float, half_range_px: float) -> Vector2:
	var r: float = maxf(half_range_px, 0.0)
	return Vector2(home_x - r, home_x + r)


## Deterministic pseudo-random value in [0, 1) for (agent_id, salt). The SAME
## (agent_id, salt) pair always yields the SAME value — stable across
## restarts/reconnects — while different salts for the same agent (the
## caller increments salt every wander cycle) yield a fresh-looking draw, and
## different agent_ids yield different values at the same salt so agents
## don't wander in lockstep. String.hash() is a stable, engine-version-
## independent 32-bit hash; absi() is defensive against signed
## reinterpretation. Mirrors PanelFloatMath.phase_for_id's approach. Empty
## agent_id -> 0.0.
static func pseudo_random_for(agent_id: String, salt: int) -> float:
	if agent_id.is_empty():
		return 0.0
	var combined: String = "%s:%d" % [agent_id, salt]
	return float(absi(combined.hash()) % RNG_BUCKETS) / float(RNG_BUCKETS)


## Picks a new wander target x within [min_x, max_x] from a [0, 1) rng value.
## rng_value is clamped defensively — an out-of-range input still yields an
## in-bounds target rather than overshooting the wander range.
static func pick_target_x(min_x: float, max_x: float, rng_value: float) -> float:
	return lerpf(min_x, max_x, clampf(rng_value, 0.0, 1.0))


## True once current_x is within ARRIVAL_THRESHOLD_PX of target_x.
static func has_arrived(current_x: float, target_x: float) -> bool:
	return absf(target_x - current_x) <= ARRIVAL_THRESHOLD_PX


## Dwell duration (seconds) at the destination before picking a new target,
## derived from a [0, 1) rng value. rng_value is clamped defensively, same
## reasoning as pick_target_x().
static func dwell_duration(rng_value: float) -> float:
	return lerpf(DWELL_MIN_SEC, DWELL_MAX_SEC, clampf(rng_value, 0.0, 1.0))


## Signed walking direction from current_x toward target_x: -1 (target is to
## the left), 0 (already there), +1 (target is to the right). Callers use
## this to drive $Body.flip_h.
static func direction_sign(current_x: float, target_x: float) -> int:
	if is_equal_approx(current_x, target_x):
		return 0
	return -1 if target_x < current_x else 1


## Advances current_x toward target_x at speed_px_sec over delta seconds,
## clamped so it never overshoots the target (move_toward semantics).
## Negative speed/delta are treated as 0 — a malformed call holds position
## rather than moving backward or teleporting.
static func step_toward(current_x: float, target_x: float, speed_px_sec: float, delta: float) -> float:
	return move_toward(current_x, target_x, maxf(speed_px_sec, 0.0) * maxf(delta, 0.0))
