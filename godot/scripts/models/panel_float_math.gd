class_name PanelFloatMath
## T18 (#129) — panel floaty animations, pure static math. Mirrors the T15
## FloorMorph / T16 PaletteMath / T17 ParticleMath idiom: every curve lives
## here as a static, node-free function so it is headlessly testable
## (tests/panel_float_math_test.gd) without a running engine. No Node,
## Tween, SceneTree, Control, or CPUParticles2D reference anywhere in this
## file. Enabled/disabled gating (the "should this panel have effects at
## all" question) is entirely the caller's concern (see
## PanelBase.effects_active()) — none of the curves below take an `enabled`
## flag.
##
## The bob is an idle ambient float on FLOATING panels only — a slow
## continuous sine, NOT a mouse-hover response (user decision A; do not
## "fix" this into a hover reaction later). Docked and fullscreen panels do
## not bob.
##
## The bob value this file computes is consumed by the caller as a
## child-node (`$VisualRoot`) transform and is NEVER added into
## `PanelBase.position` — see panel_base.gd's class doc-comment for the full
## reasoning (drag/resize/save-restore/tween code all reads/writes
## `position` directly and would drift if the bob lived there).
##
## Budget clamping in drag_trail_params() mirrors
## ParticleMath.emission_amount_for() exactly: a budget of 0, or a panel
## barely moving, yields emitting:false / amount:0 — never floored up to 1
## just to satisfy CPUParticles2D's amount >= 1 constraint.

## Defaults mirror tower.json's T18 block; tower.json is the source of truth
## at runtime, these are the fallbacks when config is absent.
const DEFAULT_BOB_AMPLITUDE_PX: float = 2.0
const DEFAULT_BOB_PERIOD_SEC: float = 4.0
const DEFAULT_FLUTTER_AMPLITUDE_PX: float = 1.5
const DEFAULT_MAX_DRAG_TRAIL_PARTICLES: int = 12

## Border shimmer — a slow, faint alpha breathe on the panel's border ring.
const SHIMMER_PERIOD_SEC: float = 3.4
const SHIMMER_ALPHA_MIN: float = 0.05
const SHIMMER_ALPHA_MAX: float = 0.20

## Parchment flutter runs faster than the body bob so the two never look
## phase-locked; expressed as a multiple of the bob's angular frequency.
const FLUTTER_SPEED_RATIO: float = 1.7

## Drag trail speed response. Below MIN the panel is "barely moving" and the
## trail is off entirely (never a 1-particle dribble).
const DRAG_SPEED_MIN_PX_SEC: float = 45.0
const DRAG_SPEED_FULL_PX_SEC: float = 900.0
const TRAIL_AMOUNT_MIN: int = 2
const TRAIL_AMOUNT_MAX: int = 10
const TRAIL_LIFETIME_MIN: float = 0.24
const TRAIL_LIFETIME_MAX: float = 0.50
const TRAIL_VELOCITY_MIN: float = 6.0
const TRAIL_VELOCITY_MAX: float = 20.0

## Deterministic-phase bucket count. 3600 buckets ≈ 0.1° resolution — far
## finer than perceptible, while keeping the modulo integral and stable.
const PHASE_BUCKETS: int = 3600


## Deterministic per-panel phase in [0, TAU). Two panels with different ids
## get different phases so several open panels never bob in lockstep; the
## SAME id always yields the SAME phase, so a panel restored from the saved
## layout resumes with the phase it had before (no visible pop on restart).
## String.hash() is a stable, engine-version-independent 32-bit hash; absi()
## is defensive against any signed reinterpretation. Empty id -> 0.0.
static func phase_for_id(panel_id: String) -> float:
	if panel_id.is_empty():
		return 0.0
	return float(absi(panel_id.hash()) % PHASE_BUCKETS) / float(PHASE_BUCKETS) * TAU


## Continuous idle-float offset (pixels, signed) at `time_sec`.
## amplitude <= 0 or period <= 0 -> 0.0 (a disabled/degenerate config must
## produce a flat zero curve, not a divide-by-zero or a NaN).
static func bob_offset(time_sec: float, amplitude_px: float, period_sec: float, phase: float) -> float:
	var amp: float = maxf(amplitude_px, 0.0)
	if amp <= 0.0 or period_sec <= 0.0:
		return 0.0
	return sin(TAU * time_sec / period_sec + phase) * amp


## Pixel-snapped variant — THIS is what PanelBase uses. The panel body is
## text (RichTextLabel / Label), and translating a Control by a fractional
## pixel forces sub-pixel text re-rasterisation every frame, which reads as
## shimmering/blurring glyphs rather than a gentle float. Rounding to whole
## pixels also matches the project's pixel-art integer-scaling aesthetic.
## At the 2px default this yields a 5-step staircase (-2..+2), which is the
## intended retro look, not a defect.
static func bob_offset_snapped(time_sec: float, amplitude_px: float, period_sec: float, phase: float) -> float:
	return roundf(bob_offset(time_sec, amplitude_px, period_sec, phase))


## Parchment edge-flutter amplitude converted to UV units for the shader
## uniform (the shader warps UV.y, so a pixel amplitude must be divided by
## the parchment's pixel height). Non-positive height or amplitude -> 0.0.
static func flutter_uv_amplitude(amplitude_px: float, panel_height_px: float) -> float:
	if amplitude_px <= 0.0 or panel_height_px <= 0.0:
		return 0.0
	return maxf(amplitude_px, 0.0) / panel_height_px


## Parchment flutter angular speed (radians/sec) for the shader uniform,
## derived from the same bob period so the two effects share one config knob.
## period <= 0 -> 0.0 (frozen, matching bob_offset's disabled behaviour).
static func flutter_speed_rad(period_sec: float) -> float:
	if period_sec <= 0.0:
		return 0.0
	return (TAU / period_sec) * FLUTTER_SPEED_RATIO


## Border shimmer alpha in [SHIMMER_ALPHA_MIN, SHIMMER_ALPHA_MAX]. Uses the
## same per-panel `phase` as the bob so a panel's chrome breathes coherently
## with its float, but on a different period so they slowly beat against each
## other instead of pulsing in step.
static func shimmer_alpha(time_sec: float, phase: float) -> float:
	var t: float = sin(TAU * time_sec / SHIMMER_PERIOD_SEC + phase) * 0.5 + 0.5
	return lerpf(SHIMMER_ALPHA_MIN, SHIMMER_ALPHA_MAX, t)


## Drag-trail emission params from instantaneous drag speed (px/sec) and the
## caller-supplied particle budget (tower.json max_drag_trail_particles).
##
## Budget clamp mirrors ParticleMath.emission_amount_for(): mini(raw,
## maxi(budget, 0)). `emitting` is gated on BOTH the speed threshold AND
## amount > 0, so a budget of 0 (a legitimate "disable the trail" config
## value) silently disables emission rather than the node layer flooring
## amount up to 1 to satisfy CPUParticles2D's amount >= 1 constraint.
## Keys: emitting(bool), amount(int), lifetime(float), velocity(float).
static func drag_trail_params(drag_speed_px_sec: float, budget: int) -> Dictionary:
	var speed: float = maxf(drag_speed_px_sec, 0.0)
	if speed < DRAG_SPEED_MIN_PX_SEC:
		return {"emitting": false, "amount": 0, "lifetime": 0.0, "velocity": 0.0}
	var t: float = smoothstep(DRAG_SPEED_MIN_PX_SEC, DRAG_SPEED_FULL_PX_SEC, speed)
	var raw: int = int(round(lerpf(float(TRAIL_AMOUNT_MIN), float(TRAIL_AMOUNT_MAX), t)))
	var amount: int = mini(raw, maxi(budget, 0))
	return {
		"emitting": amount > 0,
		"amount": amount,
		"lifetime": lerpf(TRAIL_LIFETIME_MIN, TRAIL_LIFETIME_MAX, t) if amount > 0 else 0.0,
		"velocity": lerpf(TRAIL_VELOCITY_MIN, TRAIL_VELOCITY_MAX, t) if amount > 0 else 0.0,
	}
