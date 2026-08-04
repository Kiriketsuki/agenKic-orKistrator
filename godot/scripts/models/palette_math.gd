class_name PaletteMath
## T16 (#125) — Pure power-level -> effect-amount math for the palette-swap
## shader (palette_swap.gdshader). Mirrors the T15 FloorMorph idiom: all
## curve math lives here as static, node-free functions so it is testable
## headlessly (see tests/palette_math_test.gd) without a running engine. The
## shader itself only consumes the resulting scalars as uniforms — it does
## no power->effect mapping of its own.
##
## POWER LEVEL SEMANTICS — HONEST-MINIMAL (placeholder pending a real
## server-side tier signal; see agent_character.gd / tower_manager.gd for the
## full wiring note): no per-agent power/tier field currently reaches the
## client (SSEAgentRegistered/SSEAgentOutput carry neither; TaskMeta.Tier is
## a hint the assign loop ignores — T14). power_level is therefore sourced
## entirely from tower.json config (default_power_level, with an optional
## class_power_levels demo map) until a real signal exists.
##
## The five named bands below are DOCUMENTED SAMPLE POINTS on the continuous
## curve, not discrete states — every effect_for() function is smooth
## (mix/pow/smoothstep) across the full 0..1 domain. glow_for() in particular
## must never be stepped (acceptance criterion #6).
const BANDS: Dictionary = {
	"novice": 0.1,
	"adept": 0.35,
	"master": 0.6,
	"grandmaster": 0.8,
	"legendary": 1.0,
}


## Saturation multiplier: muted (<1.0) at low power, oversaturated (>1.0) at
## high power. Smooth linear ramp across the whole domain.
static func saturation_for(power: float) -> float:
	var p: float = clampf(power, 0.0, 1.0)
	return lerpf(0.55, 1.35, p)


## Metallic sheen intensity: near-zero until the Master band, ramping up
## through Grandmaster/Legendary.
static func metallic_for(power: float) -> float:
	var p: float = clampf(power, 0.0, 1.0)
	return smoothstep(0.35, 0.65, p)


## Emissive glow intensity: SMOOTH and monotonic across the full domain —
## never stepped, even though the 5 named bands exist (acceptance #6).
static func glow_for(power: float) -> float:
	var p: float = clampf(power, 0.0, 1.0)
	return pow(p, 1.5)


## Gold-thread overlay strength: only reads from Grandmaster onward.
static func thread_for(power: float) -> float:
	var p: float = clampf(power, 0.0, 1.0)
	return smoothstep(0.55, 0.8, p)


## Iridescent hue-cycle strength: only reads at the top of the curve
## (Legendary).
static func iridescence_for(power: float) -> float:
	var p: float = clampf(power, 0.0, 1.0)
	return smoothstep(0.78, 1.0, p)


## TIME-pulse speed for the shader's metallic sweep / glow pulse / thread
## animation — higher power reads as more "alive".
static func pulse_speed_for(power: float) -> float:
	var p: float = clampf(power, 0.0, 1.0)
	return lerpf(1.5, 6.0, p)


## Convenience bundle of every effect amount for a given power_level, in the
## exact uniform names palette_swap.gdshader expects. Used by
## agent_character.gd to push all uniforms in one call, and by
## palette_math_test.gd to assert cross-band distinctness.
static func effects_for(power: float) -> Dictionary:
	return {
		"saturation": saturation_for(power),
		"metallic": metallic_for(power),
		"glow_amount": glow_for(power),
		"thread_amount": thread_for(power),
		"iridescence": iridescence_for(power),
		"pulse_speed": pulse_speed_for(power),
	}
