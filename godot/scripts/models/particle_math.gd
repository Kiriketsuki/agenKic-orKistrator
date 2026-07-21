class_name ParticleMath
## T17 (#127) — Pure power-level -> particle-emission math for the tier
## particle effects wired onto AgentCharacter's $EffectParticles /
## $AmbientParticles (CPUParticles2D — see agent_character.gd doc-comment
## for the CPUParticles2D-vs-GPUParticles2D renderer justification). Mirrors
## the T15 FloorMorph / T16 PaletteMath idiom: all tier/curve math lives here
## as static, node-free functions so it is testable headlessly (see
## tests/particle_math_test.gd) without a running engine.
##
## TIER THRESHOLDS key OFF PaletteMath.BANDS — this file does NOT define a
## second band table. The five named PaletteMath.BANDS reach-thresholds are
## reused directly:
##   power < BANDS.master (0.6)        -> TIER.NONE       (Novice/Adept — the
##                                        Adept glow is shader-only, T16)
##   BANDS.master <= power < .grandmaster (0.6..0.8)  -> TIER.SPARKLE
##   BANDS.grandmaster <= power < .legendary (0.8..1.0) -> TIER.ORBIT
##   power >= BANDS.legendary (1.0)    -> TIER.TRAIL       (+ ambient shimmer)
##
## Within a tier, emission amount / lifetime / orbit radius+speed / plume
## velocity scale CONTINUOUSLY via smoothstep between the tier's own band and
## the next band up ("scale continuously within a tier where sensible") —
## mirroring PaletteMath's smooth-curve idiom, never a hard step.
##
## BUDGET CAP (acceptance #5): emission_amount_for()/ambient_amount_for()
## both clamp against a caller-supplied `budget` (tower.json
## max_particles_per_agent, threaded once-per-floor — see
## floor_scene.configure_particle_budget()). Ambient gets a small fraction of
## the same global budget via AMBIENT_CAP_CONST so total per-agent particle
## count stays bounded and documented, never a second unbounded knob.

enum Tier { NONE, SPARKLE, ORBIT, TRAIL }

## Ambient shimmer layer is capped independently of the main budget — it is
## always sparse (a handful of slow motes), so a small constant ceiling is
## honest regardless of how generous max_particles_per_agent is configured.
const AMBIENT_CAP_CONST: int = 6


## Reach-tier for a given power level, keyed off PaletteMath.BANDS (single
## source of truth — see class doc-comment).
static func tier_for(power: float) -> Tier:
	var p: float = clampf(power, 0.0, 1.0)
	if p >= PaletteMath.BANDS["legendary"]:
		return Tier.TRAIL
	if p >= PaletteMath.BANDS["grandmaster"]:
		return Tier.ORBIT
	if p >= PaletteMath.BANDS["master"]:
		return Tier.SPARKLE
	return Tier.NONE


## Whether $EffectParticles should be emitting at all for this power level.
## NONE tier (Novice/Adept) never emits — Adept's glow is shader-only (T16).
static func emitting_for(power: float) -> bool:
	return tier_for(power) != Tier.NONE


## Raw (pre-budget-clamp) particle count for the current tier, scaling
## continuously within the tier via smoothstep(tier_band, next_band, power).
static func _raw_amount_for(power: float) -> int:
	var p: float = clampf(power, 0.0, 1.0)
	var tier: Tier = tier_for(p)
	match tier:
		Tier.SPARKLE:
			var t: float = smoothstep(PaletteMath.BANDS["master"], PaletteMath.BANDS["grandmaster"], p)
			return int(round(lerpf(3.0, 8.0, t)))
		Tier.ORBIT:
			var t: float = smoothstep(PaletteMath.BANDS["grandmaster"], PaletteMath.BANDS["legendary"], p)
			return int(round(lerpf(8.0, 14.0, t)))
		Tier.TRAIL:
			# TRAIL only reaches fully at p == BANDS.legendary (1.0) — see
			# PaletteMath.iridescence_for's identical smoothstep(0.78, 1.0)
			# top-of-curve precedent. A dense, short-lived directional plume.
			return 20
		_:
			return 0


## Budget-capped emission amount — acceptance #5. Never exceeds the
## caller-supplied budget (tower.json max_particles_per_agent).
static func emission_amount_for(power: float, budget: int) -> int:
	var raw: int = _raw_amount_for(power)
	return mini(raw, maxi(budget, 0))


## Particle lifetime (seconds), scaling continuously within a tier. TRAIL's
## plume is deliberately short-lived (a "trail" reads as a directional burst
## for a mostly-stationary desk agent, not a literal motion trail — honest
## minimal, see spec Decision 5).
static func lifetime_for(power: float) -> float:
	var p: float = clampf(power, 0.0, 1.0)
	var tier: Tier = tier_for(p)
	match tier:
		Tier.SPARKLE:
			var t: float = smoothstep(PaletteMath.BANDS["master"], PaletteMath.BANDS["grandmaster"], p)
			return lerpf(0.6, 1.0, t)
		Tier.ORBIT:
			var t: float = smoothstep(PaletteMath.BANDS["grandmaster"], PaletteMath.BANDS["legendary"], p)
			return lerpf(1.0, 1.8, t)
		Tier.TRAIL:
			return 0.5
		_:
			return 0.0


## Orbit radius (px) — only meaningful for ORBIT/TRAIL tiers, but defined
## across the domain (0 below ORBIT) so callers never branch on tier.
static func orbit_radius_for(power: float) -> float:
	var p: float = clampf(power, 0.0, 1.0)
	var tier: Tier = tier_for(p)
	match tier:
		Tier.ORBIT:
			var t: float = smoothstep(PaletteMath.BANDS["grandmaster"], PaletteMath.BANDS["legendary"], p)
			return lerpf(10.0, 16.0, t)
		Tier.TRAIL:
			return 16.0
		_:
			return 0.0


## Orbit angular speed (radians/sec via CPUParticles2D's orbit_velocity,
## which is expressed as revolutions/sec — see agent_character.gd wiring).
static func orbit_speed_for(power: float) -> float:
	var p: float = clampf(power, 0.0, 1.0)
	var tier: Tier = tier_for(p)
	match tier:
		Tier.ORBIT:
			var t: float = smoothstep(PaletteMath.BANDS["grandmaster"], PaletteMath.BANDS["legendary"], p)
			return lerpf(0.25, 0.6, t)
		Tier.TRAIL:
			return 0.6
		_:
			return 0.0


## Directional plume speed (px/sec initial_velocity) for the TRAIL tier's
## emission burst. Zero outside TRAIL.
static func plume_velocity_for(power: float) -> float:
	var p: float = clampf(power, 0.0, 1.0)
	if tier_for(p) == Tier.TRAIL:
		return lerpf(24.0, 40.0, smoothstep(PaletteMath.BANDS["legendary"], 1.0, p))
	return 0.0


## Ambient shimmer layer ($AmbientParticles) is Legendary-only — reads
## true only at PaletteMath.BANDS.legendary (top of curve), same threshold
## TRAIL uses (they always co-occur, per spec Decision 2/5).
static func ambient_enabled_for(power: float) -> bool:
	return clampf(power, 0.0, 1.0) >= PaletteMath.BANDS["legendary"]


## Budget-capped ambient particle count — always the lesser of
## AMBIENT_CAP_CONST and the shared per-agent budget, so ambient never blows
## past the same global cap the primary emitter respects.
static func ambient_amount_for(power: float, budget: int) -> int:
	if not ambient_enabled_for(power):
		return 0
	return mini(AMBIENT_CAP_CONST, maxi(budget, 0))


## Convenience bundle of every derived param for a given (power_level,
## budget) pair, mirroring PaletteMath.effects_for(). Used by
## agent_character.gd._apply_particles() to configure both emitters in one
## call, and by particle_math_test.gd to assert cross-tier distinctness.
static func params_for(power: float, budget: int) -> Dictionary:
	return {
		"tier": tier_for(power),
		"emitting": emitting_for(power),
		"amount": emission_amount_for(power, budget),
		"lifetime": lifetime_for(power),
		"orbit_radius": orbit_radius_for(power),
		"orbit_speed": orbit_speed_for(power),
		"plume_velocity": plume_velocity_for(power),
		"ambient_enabled": ambient_enabled_for(power),
		"ambient_amount": ambient_amount_for(power, budget),
	}
