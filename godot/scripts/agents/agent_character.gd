extends Node2D
## AgentCharacter — visual representation of an orchestrator agent on a tower floor.
## Phase 1 uses placeholder colored rectangles; T22 will replace these with real sprites.

class_name AgentCharacter

signal character_clicked(agent_id: String)
signal character_right_clicked(agent_id: String)
signal character_hovered(agent_id: String)
signal character_unhovered(agent_id: String)

enum CharacterClass {
	ALCHEMIST,
	SCRIBE,
	ARCHMAGE,
	WARDKEEPER,
	LIBRARIAN,
	ENCHANTER,
	APPRENTICE,
}

enum AnimState {
	IDLE,
	WALKING,
	WORKING,
	REPORTING,
	STUNNED,
}

## Local idle-wander sub-state (Task 3) — orthogonal to AnimState. AnimState
## reflects the bridge-driven agent lifecycle (idle/assigned/working/...);
## WanderPhase only tracks whether THIS agent is currently standing at its
## desk (DWELLING) or shuffling toward a nearby wander target (MOVING), and
## only advances while _is_wander_eligible() is true (see below).
enum WanderPhase {
	DWELLING,
	MOVING,
}

# T16 (#125 review finding 2) — LIBRARIAN was originally Color(0.65, 0.4,
# 0.15, 1.0), luminance ~0.4463, only ~0.002 away from ARCHMAGE's ~0.4442.
# palette_swap.gdshader keys its provider_lut lookup by luminance(styled),
# so the two classes would sample near-identical colors under EVERY
# provider LUT, collapsing part of the class x provider variant matrix
# (acceptance #4). Darkened here to widen the luminance gap while keeping
# LIBRARIAN's brown/tan identity — see
# tests/palette_math_test.gd:_run_class_through_provider_lut_distinctness_case.
const CLASS_COLORS: Dictionary = {
	CharacterClass.ALCHEMIST:  Color(0.85, 0.55, 0.0,  1.0),
	CharacterClass.SCRIBE:     Color(0.35, 0.6,  0.95, 1.0),
	CharacterClass.ARCHMAGE:   Color(0.75, 0.2,  0.9,  1.0),
	CharacterClass.WARDKEEPER: Color(0.25, 0.75, 0.3,  1.0),
	CharacterClass.LIBRARIAN:  Color(0.55, 0.32, 0.08, 1.0),
	CharacterClass.ENCHANTER:  Color(0.1,  0.85, 0.8,  1.0),
	CharacterClass.APPRENTICE: Color(0.65, 0.65, 0.75, 1.0),
}

# T22 design-pack frames: the shipped kit SpriteFrames resources. Each cuts
# the 64x100 sheet into 16x20 cells, one looping animation per AnimState row
# (idle 4, walking 8, working 8, reporting 6, stunned 6 fps). See
# assets/asset_manifest.json and PORTING.md.
const CLASS_FRAMES: Dictionary = {
	CharacterClass.ALCHEMIST:  preload("res://assets/sprites/alchemist_frames.tres"),
	CharacterClass.SCRIBE:     preload("res://assets/sprites/scribe_frames.tres"),
	CharacterClass.ARCHMAGE:   preload("res://assets/sprites/archmage_frames.tres"),
	CharacterClass.WARDKEEPER: preload("res://assets/sprites/wardkeeper_frames.tres"),
	CharacterClass.LIBRARIAN:  preload("res://assets/sprites/librarian_frames.tres"),
	CharacterClass.ENCHANTER:  preload("res://assets/sprites/enchanter_frames.tres"),
	CharacterClass.APPRENTICE: preload("res://assets/sprites/apprentice_frames.tres"),
}

const ANIM_BY_STATE: Dictionary = {
	AnimState.IDLE:      "idle",
	AnimState.WALKING:   "walking",
	AnimState.WORKING:   "working",
	AnimState.REPORTING: "reporting",
	AnimState.STUNNED:   "stunned",
}

const CLASS_LABELS: Dictionary = {
	CharacterClass.ALCHEMIST:  "ALC",
	CharacterClass.SCRIBE:     "SCR",
	CharacterClass.ARCHMAGE:   "ARC",
	CharacterClass.WARDKEEPER: "WRD",
	CharacterClass.LIBRARIAN:  "LIB",
	CharacterClass.ENCHANTER:  "ENC",
	CharacterClass.APPRENTICE: "APP",
}

const CLASS_BY_NAME: Dictionary = {
	"alchemist":  CharacterClass.ALCHEMIST,
	"scribe":     CharacterClass.SCRIBE,
	"archmage":   CharacterClass.ARCHMAGE,
	"wardkeeper": CharacterClass.WARDKEEPER,
	"librarian":  CharacterClass.LIBRARIAN,
	"enchanter":  CharacterClass.ENCHANTER,
	"apprentice": CharacterClass.APPRENTICE,
}

const STATE_BY_NAME: Dictionary = {
	"idle":      AnimState.IDLE,
	"assigned":  AnimState.WALKING,
	"working":   AnimState.WORKING,
	"reporting": AnimState.REPORTING,
	"crashed":   AnimState.STUNNED,
}

# Multiplicative tints applied on top of the class base color. Softened for
# the real colored kit art. The earlier values were tuned for flat ColorRects
# and they muddied real pixel colors.
const STATE_TINTS: Dictionary = {
	AnimState.IDLE:      Color(1.0,  1.0,  1.0,  1.0),
	AnimState.WALKING:   Color(0.92, 0.96, 1.0,  1.0),
	AnimState.WORKING:   Color(1.0,  1.0,  0.88, 1.0),
	AnimState.REPORTING: Color(1.0,  0.94, 0.78, 1.0),
	AnimState.STUNNED:   Color(0.6,  0.45, 0.45, 0.85),
}

const FLOATING_RUNE_SCENE: PackedScene = preload("res://scenes/floating_rune.tscn")
const MAX_RUNES: int = 5

## T16 (#125) — palette-swap shader shared by _body (ColorRect, today) and
## _animated_sprite (AnimatedSprite2D, T22-ready). See the composition
## contract doc-comment on _apply_class_visuals()/set_power_level() below and
## the shader's own header comment for the full pipeline.
const PALETTE_SHADER: Shader = preload("res://shaders/palette_swap.gdshader")

## T17 (#127) — HONEST-MINIMAL renderer justification: project.godot sets
## rendering_method="gl_compatibility" (and .mobile). In Godot 4.2 the
## Compatibility/GLES3-limited backend does NOT process GPU particles —
## GPUParticles2D support for the Compatibility renderer only landed in
## Godot 4.3. Issue #127 names GPUParticles2D, but under this repo's
## configured 4.2 Compatibility renderer a GPUParticles2D node would emit
## and draw NOTHING, failing every behavioral acceptance criterion while
## satisfying only the literal class name. CPUParticles2D is CPU-simulated,
## works on every backend, is a full per-instance Node2D (sidestepping T16's
## one-ShaderMaterial-per-agent canvas_item constraint above — each agent
## freely configures its own emitters), and exposes every property these
## sparse tier effects need. See particle_math.gd / particle_textures.gd for
## the pure math and procedural-texture halves of this feature.
## Default per-agent particle budget (acceptance #5) — overwritten by
## set_particle_budget() once TowerManager threads tower.json's
## max_particles_per_agent through FloorScene.configure_particle_budget().
const DEFAULT_PARTICLE_BUDGET: int = 24

## T22 art-scale correction. ParticleMath returns spatial values authored for
## the dead 2.5x sprite scale, and particle_math.gd stays untouched because
## tests/particle_math_test.gd asserts those numbers. This factor rescales
## orbit_radius and plume_velocity at the call site only.
const PARTICLE_SPACE_SCALE: float = 0.4

## Materialize animation (Task 1) — agents used to pop into existence
## instantly. On first entering the tree, the character now fades in
## (modulate alpha 0 -> 1) with a small scale settle and a brief upward
## drift, so a new desk arrival reads as an event instead of a frame-1 pop.
const SPAWN_FADE_SEC: float = 0.65
const SPAWN_SCALE_START: float = 0.82
const SPAWN_DRIFT_PX: float = 6.0

## Aura liveliness (Task 2) — the additive particle aura used to read as one
## static overlapping blob: every particle sat at full alpha for its whole
## lifetime, so a dense tier just looked like a steady smear rather than
## something alive. AURA_ALPHA_PEAK caps the color_ramp's peak alpha (each
## particle now fades in/out instead of holding full brightness), and
## AURA_LIFETIME_RANDOMNESS desyncs individual particles' fade timing so the
## aura visibly breathes rather than pulsing in lockstep.
## AURA_ORBIT_VELOCITY_JITTER staggers ORBIT/TRAIL tier orbit speeds (as a
## fraction of the tier's base orbit_speed) so a ring of motes drifts apart
## over time instead of rotating as one rigid, blob-reading disc.
const AURA_ALPHA_PEAK: float = 0.65
const AURA_LIFETIME_RANDOMNESS: float = 0.35
const AURA_ORBIT_VELOCITY_JITTER: float = 0.18

## Idle wander (Task 3) tunables. WANDER_MARGIN_PX is subtracted from half
## the desk-slot pitch (EdgeLayout.DESK_WIDTH + EdgeLayout.DESK_SPACING) —
## see WanderMath.half_range_px() — so a wandering agent always stays well
## short of a neighbouring desk. WANDER_SPEED_PX_SEC is a slow, deliberate
## shuffle: at the derived ~9px half-range a walk takes well under a second.
const WANDER_MARGIN_PX: float = 4.0
const WANDER_SPEED_PX_SEC: float = 14.0

## Set by the owner (FloorScene) before add_child so it is ready in _ready().
var agent_id: String = ""

var _character_class: CharacterClass = CharacterClass.APPRENTICE
var _anim_state: AnimState = AnimState.IDLE
var _pulse_time: float = 0.0
var _provider: String = ""
var _active_runes: Array[Node2D] = []
var _power_level: float = 0.0
var _shader_material: ShaderMaterial = null
var _particle_budget: int = DEFAULT_PARTICLE_BUDGET

## Task 1 — killed in _exit_tree() so a mid-fade despawn (play_exit_animation
## reparenting/queue_free) never leaves a tween callback touching a freed
## node, mirroring floor_scene.gd's _kill_morph_tween() pattern.
var _spawn_tween: Tween = null

## Task 3 — idle wander state. _home_x/_wander_min_x/_wander_max_x are
## derived lazily on the first eligible _process() frame (see _init_wander())
## rather than in _ready(), because FloorScene sets this node's desk position
## AFTER add_child() — position is still (0, 0) while _ready() runs.
var _wander_phase: WanderPhase = WanderPhase.DWELLING
var _wander_initialized: bool = false
var _home_x: float = 0.0
var _wander_min_x: float = 0.0
var _wander_max_x: float = 0.0
var _wander_target_x: float = 0.0
var _wander_dwell_timer: float = 0.0
## Advances every wander-cycle draw so WanderMath.pseudo_random_for() yields
## a fresh-looking value each time while staying deterministic per (agent_id,
## salt) — see that function's doc-comment.
var _wander_salt: int = 0

@onready var _body: AnimatedSprite2D = $Body
@onready var _click_area: Area2D = $ClickArea
@onready var _effect_particles: CPUParticles2D = $EffectParticles
@onready var _ambient_particles: CPUParticles2D = $AmbientParticles


func _ready() -> void:
	_click_area.input_event.connect(_on_area_input_event)
	_click_area.mouse_entered.connect(func() -> void: character_hovered.emit(agent_id))
	_click_area.mouse_exited.connect(func() -> void: character_unhovered.emit(agent_id))
	# One ShaderMaterial shared by _body and _animated_sprite — only one is
	# visible at a time (T22 will flip visibility when real sprite frames
	# land), so sharing is safe. See set_character_class() for the WHITE
	# class_color hook T22 must flip when the sprite becomes visible.
	_shader_material = ShaderMaterial.new()
	_shader_material.shader = PALETTE_SHADER
	_body.material = _shader_material
	_apply_class_visuals()
	_apply_state_tint()
	_apply_provider_visuals()
	_push_power_uniforms()
	_apply_particles()
	_play_spawn_animation()


func _process(delta: float) -> void:
	if _anim_state == AnimState.WORKING:
		_pulse_time += delta * 4.0
		var pulse: float = 0.92 + sin(_pulse_time) * 0.08
		_body.modulate = STATE_TINTS[AnimState.WORKING] * Color(pulse, pulse, pulse, 1.0)
		return
	_update_wander(delta)


## Kills any in-flight spawn tween before this node is freed — an un-killed
## tween whose next frame writes to self/_body after either is gone would
## crash. Mirrors floor_scene.gd's _kill_morph_tween()/NOTIFICATION_PREDELETE
## pattern (see that file's doc-comment for the underlying hazard).
func _exit_tree() -> void:
	_kill_spawn_tween()


func _kill_spawn_tween() -> void:
	if _spawn_tween != null and _spawn_tween.is_valid():
		_spawn_tween.kill()
	_spawn_tween = null


## Task 1 — materialize animation. Only ever touches THIS node's own
## properties (modulate/scale) and _body's LOCAL position offset, never an
## absolute world position — at _ready() time FloorScene has not yet placed
## this node at its desk x (see _init_wander()'s doc-comment for the same
## ordering hazard), so a local, relative drift is the only safe choice here.
func _play_spawn_animation() -> void:
	modulate.a = 0.0
	scale = Vector2(SPAWN_SCALE_START, SPAWN_SCALE_START)
	_body.position.y = -SPAWN_DRIFT_PX
	_spawn_tween = create_tween().set_parallel(true) \
		.set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)
	_spawn_tween.tween_property(self, "modulate:a", 1.0, SPAWN_FADE_SEC)
	_spawn_tween.tween_property(self, "scale", Vector2.ONE, SPAWN_FADE_SEC)
	_spawn_tween.tween_property(_body, "position:y", 0.0, SPAWN_FADE_SEC)


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

func set_character_class(class_name_str: String) -> void:
	_character_class = CLASS_BY_NAME.get(class_name_str, CharacterClass.APPRENTICE)
	if is_inside_tree():
		_apply_class_visuals()
		_apply_state_tint()


func set_animation_state(state_name: String) -> void:
	var new_state: AnimState = STATE_BY_NAME.get(state_name, AnimState.IDLE)
	if new_state == _anim_state:
		return
	_anim_state = new_state
	_pulse_time = 0.0
	# Task 3 — the moment the bridge drives this agent into a busy state,
	# wandering stops and the agent returns to its desk. WORKING/REPORTING/
	# STUNNED are the only states _is_wander_eligible() excludes, so this
	# list must stay in sync with that function.
	if new_state == AnimState.WORKING or new_state == AnimState.REPORTING or new_state == AnimState.STUNNED:
		_return_to_wander_home()
	if is_inside_tree():
		_apply_state_tint()


func get_anim_state() -> AnimState:
	return _anim_state


func set_provider(p: String) -> void:
	_provider = p
	_apply_provider_visuals()
	_apply_particles()


## T16 (#125) — HONEST-MINIMAL: `p` is sourced by the caller (TowerManager)
## from tower.json config (default_power_level / class_power_levels), not a
## real per-agent server signal — see palette_math.gd doc-comment. Fully
## wired to the shader regardless, so a real tier signal can be dropped in
## later with zero shader/wiring changes.
func set_power_level(p: float) -> void:
	_power_level = clampf(p, 0.0, 1.0)
	_push_power_uniforms()
	_apply_particles()


## T17 (#127) acceptance #5 — global per-agent particle budget cap, threaded
## once-per-floor by FloorScene._rebuild_interior() (see
## FloorScene.configure_particle_budget()) BEFORE set_power_level() so the
## budget is already in place when particles first configure. Not a
## per-agent slot value like power_level — every agent on a floor currently
## shares the same cap.
func set_particle_budget(budget: int) -> void:
	_particle_budget = maxi(budget, 0)
	_apply_particles()


func receive_output(chunk: BridgeData.AgentOutputChunk) -> void:
	if _anim_state == AnimState.IDLE:
		return
	var result: Dictionary = RuneFilter.process(chunk)
	if not result.get(&"show", false):
		return
	var provider: String = chunk.provider if not chunk.provider.is_empty() else _provider
	if provider.is_empty():
		provider = "unknown"
	# Enforce rune cap — accelerate oldest on overflow.
	if _active_runes.size() >= MAX_RUNES:
		var oldest: Node2D = _active_runes[0]
		_active_runes.remove_at(0)
		if is_instance_valid(oldest) and oldest.has_method("accelerate_fade"):
			oldest.accelerate_fade()
	var rune: FloatingRune = FLOATING_RUNE_SCENE.instantiate() as FloatingRune
	add_child(rune)
	rune.position = Vector2(0.0, -14.0)
	rune.setup(result[&"text"], result[&"keywords"], provider)
	_active_runes.append(rune)
	rune.tree_exiting.connect(func() -> void:
		_active_runes.erase(rune)
	)


## Fade out over 0.4 s then free self. Called by TowerManager on agent.deregistered.
## Reparents self to the floor root before tweening so concurrent _rebuild_interior
## calls on _agent_slots_node do not queue_free this node mid-fade.
func play_exit_animation() -> void:
	# A despawn arriving mid-materialize would otherwise leave two tweens
	# fighting over modulate:a (the fade-in racing the fade-out).
	_kill_spawn_tween()
	var floor_root: Node = get_parent().get_parent()
	reparent(floor_root)
	var tween: Tween = create_tween()
	tween.tween_property(self, "modulate:a", 0.0, 0.4)
	tween.tween_callback(queue_free)


# ---------------------------------------------------------------------------
# Idle wander (Task 3)
# ---------------------------------------------------------------------------

## True while this agent is allowed to wander — everything except the three
## "busy" bridge states. Keep in sync with set_animation_state()'s
## _return_to_wander_home() gate above.
func _is_wander_eligible() -> bool:
	return _anim_state != AnimState.WORKING and _anim_state != AnimState.REPORTING and _anim_state != AnimState.STUNNED


## Per-frame wander tick, called from _process() whenever this agent is not
## WORKING (which owns _process() for its own pulse tint above).
func _update_wander(delta: float) -> void:
	if not _is_wander_eligible():
		return
	if not _wander_initialized:
		_init_wander()
	match _wander_phase:
		WanderPhase.DWELLING:
			_wander_dwell_timer -= delta
			if _wander_dwell_timer <= 0.0:
				_begin_wander_walk()
		WanderPhase.MOVING:
			position.x = WanderMath.step_toward(position.x, _wander_target_x, WANDER_SPEED_PX_SEC, delta)
			if WanderMath.has_arrived(position.x, _wander_target_x):
				_end_wander_walk()


## Lazy first-time setup. Runs on the first eligible _process() frame rather
## than _ready() — FloorScene calls add_child() (which fires _ready()
## synchronously) and only assigns this node's desk `position` on the very
## next line, in _rebuild_interior()/_reposition_interior(). By the first
## _process() frame that assignment has long since happened, so position.x
## here is the true desk/home x, not the (0, 0) it would be in _ready().
func _init_wander() -> void:
	_home_x = position.x
	var half_range: float = WanderMath.half_range_px(EdgeLayout.DESK_WIDTH, EdgeLayout.DESK_SPACING, WANDER_MARGIN_PX)
	var bounds: Vector2 = WanderMath.wander_bounds(_home_x, half_range)
	_wander_min_x = bounds.x
	_wander_max_x = bounds.y
	_wander_dwell_timer = WanderMath.dwell_duration(WanderMath.pseudo_random_for(agent_id, _wander_salt))
	_wander_salt += 1
	_wander_phase = WanderPhase.DWELLING
	_wander_initialized = true


func _begin_wander_walk() -> void:
	_wander_target_x = WanderMath.pick_target_x(_wander_min_x, _wander_max_x, WanderMath.pseudo_random_for(agent_id, _wander_salt))
	_wander_salt += 1
	_wander_phase = WanderPhase.MOVING
	_body.flip_h = WanderMath.direction_sign(position.x, _wander_target_x) < 0
	_body.play(ANIM_BY_STATE[AnimState.WALKING])


func _end_wander_walk() -> void:
	_wander_phase = WanderPhase.DWELLING
	_wander_dwell_timer = WanderMath.dwell_duration(WanderMath.pseudo_random_for(agent_id, _wander_salt))
	_wander_salt += 1
	_body.play(ANIM_BY_STATE[AnimState.IDLE])


## Called the moment set_animation_state() drives this agent into a busy
## state — snaps it back to its desk x and stops any in-flight wander walk.
## A future pass could tween this trip instead of snapping; out of scope
## here (the state transition itself is usually already visually loud —
## tint/animation change — so the snap is not the focal cue).
func _return_to_wander_home() -> void:
	if _wander_initialized:
		position.x = _home_x
	_wander_phase = WanderPhase.DWELLING
	_body.flip_h = false


# ---------------------------------------------------------------------------
# Private helpers
# ---------------------------------------------------------------------------

## T16 (#125) composition contract: the class color no longer lives in
## _body.color — it moves into the shader's `class_color` uniform so the
## SAME shader path works on today's flat ColorRect (TEXTURE == default
## white 1x1) and tomorrow's grayscale sprite art (TEXTURE == real texel
## luminance). _body.color/_animated_sprite therefore carry ONLY modulate
## (STATE_TINTS + the working-pulse below) — unchanged from before this task.
##
## HOOK FOR T22: when the sprite becomes the visible node, class_color must
## switch to WHITE (so the sprite's own grayscale art drives the palette
## instead of a flat class tint) — not automatic today; wire it alongside
## whatever T22 uses to flip _body/_animated_sprite visibility.
func _apply_class_visuals() -> void:
	_body.sprite_frames = CLASS_FRAMES[_character_class]
	_body.play(ANIM_BY_STATE[_anim_state])
	if _shader_material != null:
		# Body is real sprite art now (asset port kit) — per the T16 contract,
		# class_color stays WHITE so the sheet's own colors drive the palette.
		_shader_material.set_shader_parameter("class_color", Color.WHITE)


func _apply_state_tint() -> void:
	if _anim_state != AnimState.WORKING:
		_body.modulate = STATE_TINTS[_anim_state]
	if _body.sprite_frames != null:
		_body.play(ANIM_BY_STATE[_anim_state])


## Pushes provider_lut/lut_mix uniforms from the current _provider. Guarded
## on _shader_material != null so calls arriving before _ready() (e.g. if a
## future caller sets provider pre-tree, mirroring set_composite_load's
## pre-tree handling elsewhere) don't crash — _ready() re-derives the same
## state from _provider once the material exists.
func _apply_provider_visuals() -> void:
	if _shader_material == null:
		return
	var lut_provider: String = _provider if not _provider.is_empty() else "unknown"
	_shader_material.set_shader_parameter("provider_lut", ProviderPalette.get_lut(lut_provider))
	_shader_material.set_shader_parameter("lut_mix", ProviderPalette.get_lut_mix(_provider))


## Pushes power_level + all PaletteMath-derived effect-amount uniforms.
## Guarded on _shader_material != null for the same pre-tree reason as
## _apply_provider_visuals().
func _push_power_uniforms() -> void:
	if _shader_material == null:
		return
	_shader_material.set_shader_parameter("power_level", _power_level)
	var effects: Dictionary = PaletteMath.effects_for(_power_level)
	for key: String in effects.keys():
		_shader_material.set_shader_parameter(key, effects[key])


## T17 (#127) — configures $EffectParticles (primary tier visual:
## sparkle -> orbit -> trail) and $AmbientParticles (Legendary-only additive
## shimmer) from ParticleMath.params_for(_power_level, _particle_budget) plus
## provider theming (ProviderPalette.get_accent_color/get_particle_style +
## ParticleTextures). Null-guarded on both @onready refs — mirrors
## _apply_provider_visuals()/_push_power_uniforms()'s pre-tree guard, since
## set_provider()/set_power_level()/set_particle_budget() may all be called
## by the owner (FloorScene) before add_child() finishes wiring @onready.
## Does NOT touch _body.modulate or the WORKING pulse in _process() — CPU
## particles are independent CanvasItem children, and this node's own
## modulate (set by play_exit_animation()'s fade tween) propagates to them
## automatically, so no separate particle teardown/fade is needed.
func _apply_particles() -> void:
	if _effect_particles == null or _ambient_particles == null:
		return
	var params: Dictionary = ParticleMath.params_for(_power_level, _particle_budget)
	var style: String = ProviderPalette.get_particle_style(_provider)
	var accent: Color = ProviderPalette.get_accent_color(_provider)

	var emitting: bool = params["emitting"]
	_effect_particles.emitting = emitting
	if emitting:
		# ParticleMath.params_for() gates `emitting` on amount > 0 (budget-cap
		# aware — see particle_math.gd doc-comment), so amount is guaranteed
		# >= 1 here already; CPUParticles2D.amount must stay >= 1 (0 is
		# invalid), which this gating satisfies without ever flooring a
		# budget-clamped-to-0 amount back up to 1. Reassigning amount
		# restarts the system — safe here since tier changes are
		# config-rare, not per-frame.
		_effect_particles.amount = int(params["amount"])
		_effect_particles.lifetime = maxf(0.05, float(params["lifetime"]))
		# Task 2 — desyncs each particle's own fade timing so the aura
		# breathes rather than every particle fading in lockstep.
		_effect_particles.lifetime_randomness = AURA_LIFETIME_RANDOMNESS
		_effect_particles.texture = ParticleTextures.get_texture(style)
		_effect_particles.color = accent
		# Task 2 — color_ramp multiplies against `color` over each particle's
		# lifetime (fade in, hold at AURA_ALPHA_PEAK, fade out) instead of
		# every particle sitting at full alpha the whole time. That full-alpha
		# overlap across many simultaneous particles is what read as one
		# static blob; ramping it down and desyncing it (lifetime_randomness
		# above) is what makes the aura visibly move and breathe instead.
		_effect_particles.color_ramp = _build_aura_alpha_ramp(accent, AURA_ALPHA_PEAK)
		_configure_tier_shape(
			params["tier"],
			float(params["orbit_radius"]) * PARTICLE_SPACE_SCALE,
			float(params["orbit_speed"]),
			float(params["plume_velocity"]) * PARTICLE_SPACE_SCALE
		)

	var ambient_enabled: bool = params["ambient_enabled"]
	_ambient_particles.emitting = ambient_enabled
	if ambient_enabled:
		# Same budget-aware gating as $EffectParticles above — ambient_amount
		# is guaranteed >= 1 whenever ambient_enabled is true.
		_ambient_particles.amount = int(params["ambient_amount"])
		# Sparse, large, very-low-alpha, slow-rising motes (additive blend,
		# see agent_character.tscn's CanvasItemMaterial) — an honest shimmer
		# layer, NOT screen-space distortion (gl_compatibility can't cheaply
		# warp per-agent via BackBufferCopy). Reads as heat-shimmer/aura.
		_ambient_particles.lifetime = 2.5
		_ambient_particles.lifetime_randomness = AURA_LIFETIME_RANDOMNESS
		_ambient_particles.texture = ParticleTextures.get_texture("dot")
		_ambient_particles.color = Color(accent.r, accent.g, accent.b, 0.18)
		# Same breathing fade as $EffectParticles above, at the shimmer's own
		# (already low) peak alpha — keeps the "sparse, very-low-alpha" intent
		# from the doc-comment above while avoiding a static-looking hold.
		_ambient_particles.color_ramp = _build_aura_alpha_ramp(accent, 0.18)
		_ambient_particles.emission_shape = CPUParticles2D.EMISSION_SHAPE_SPHERE
		_ambient_particles.emission_sphere_radius = 6.0
		_ambient_particles.direction = Vector2(0.0, -1.0)
		_ambient_particles.spread = 30.0
		_ambient_particles.initial_velocity_min = 1.5
		_ambient_particles.initial_velocity_max = 2.5
		_ambient_particles.gravity = Vector2(0.0, -1.0)
		_ambient_particles.scale_amount_min = 0.6
		_ambient_particles.scale_amount_max = 1.0


## Per-tier CPUParticles2D shape/velocity configuration for $EffectParticles.
## Only called while emitting == true (see _apply_particles()).
func _configure_tier_shape(tier: int, orbit_radius: float, orbit_speed: float, plume_velocity: float) -> void:
	match tier:
		ParticleMath.Tier.SPARKLE:
			# Sparse sparkles: point-emit, drift gently upward.
			_effect_particles.emission_shape = CPUParticles2D.EMISSION_SHAPE_POINT
			_effect_particles.direction = Vector2(0.0, -1.0)
			_effect_particles.spread = 60.0
			_effect_particles.initial_velocity_min = 2.0
			_effect_particles.initial_velocity_max = 4.0
			_effect_particles.gravity = Vector2(0.0, -2.5)
			_effect_particles.orbit_velocity_min = 0.0
			_effect_particles.orbit_velocity_max = 0.0
			_effect_particles.scale_amount_min = 0.5
			_effect_particles.scale_amount_max = 1.0
		ParticleMath.Tier.ORBIT:
			# Orbiting motes: ring-ish spawn, near-zero linear velocity,
			# orbit_velocity does the work.
			_effect_particles.emission_shape = CPUParticles2D.EMISSION_SHAPE_SPHERE
			_effect_particles.emission_sphere_radius = orbit_radius
			_effect_particles.direction = Vector2(0.0, 0.0)
			_effect_particles.spread = 0.0
			_effect_particles.initial_velocity_min = 0.0
			_effect_particles.initial_velocity_max = 0.0
			_effect_particles.gravity = Vector2.ZERO
			# Task 2 — a fixed min==max orbit_velocity spins every mote at the
			# identical rate, so the whole ring holds a rigid relative
			# formation and reads as one static rotating disc rather than
			# several independent motes. Jittering the range lets them drift
			# apart over time.
			_effect_particles.orbit_velocity_min = orbit_speed * (1.0 - AURA_ORBIT_VELOCITY_JITTER)
			_effect_particles.orbit_velocity_max = orbit_speed * (1.0 + AURA_ORBIT_VELOCITY_JITTER)
			_effect_particles.scale_amount_min = 0.8
			_effect_particles.scale_amount_max = 1.2
		ParticleMath.Tier.TRAIL:
			# Trail realized as a directional emission plume (dense,
			# short-lived, directional velocity) rather than a literal motion
			# trail — honest-minimal for a mostly-stationary desk agent (see
			# ParticleMath.lifetime_for()'s doc-comment). Orbiting motes
			# layer on top (orbit_velocity) for the "ambient distortion"
			# co-occurring visual richness at the top of the curve.
			_effect_particles.emission_shape = CPUParticles2D.EMISSION_SHAPE_SPHERE
			_effect_particles.emission_sphere_radius = orbit_radius
			_effect_particles.direction = Vector2(-1.0, -0.4)
			_effect_particles.spread = 45.0
			_effect_particles.initial_velocity_min = plume_velocity * 0.6
			_effect_particles.initial_velocity_max = plume_velocity
			_effect_particles.gravity = Vector2.ZERO
			_effect_particles.orbit_velocity_min = orbit_speed * (1.0 - AURA_ORBIT_VELOCITY_JITTER)
			_effect_particles.orbit_velocity_max = orbit_speed * (1.0 + AURA_ORBIT_VELOCITY_JITTER)
			_effect_particles.scale_amount_min = 0.9
			_effect_particles.scale_amount_max = 1.4
		_:
			pass


## Task 2 — builds a fade-in/fade-out alpha gradient for a tier particle
## emitter's color_ramp: transparent at birth, up to peak_alpha at 18% of the
## particle's life, back to transparent at death. Multiplies against `color`
## (a CPUParticles2D property, set to the opaque `base_color` by the caller),
## so the final rendered alpha is base_color.a * this ramp's alpha — see
## _apply_particles()'s doc-comment for why this replaces a flat full-alpha
## hold.
func _build_aura_alpha_ramp(base_color: Color, peak_alpha: float) -> Gradient:
	var ramp := Gradient.new()
	ramp.offsets = PackedFloat32Array([0.0, 0.18, 1.0])
	ramp.colors = PackedColorArray([
		Color(base_color.r, base_color.g, base_color.b, 0.0),
		Color(base_color.r, base_color.g, base_color.b, peak_alpha),
		Color(base_color.r, base_color.g, base_color.b, 0.0),
	])
	return ramp


func _on_area_input_event(_viewport: Node, event: InputEvent, _shape_idx: int) -> void:
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed and mb.button_index == MOUSE_BUTTON_LEFT:
			character_clicked.emit(agent_id)
		elif mb.pressed and mb.button_index == MOUSE_BUTTON_RIGHT:
			character_right_clicked.emit(agent_id)
