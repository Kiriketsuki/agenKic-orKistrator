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

# Multiplicative tints applied on top of the class base color.
const STATE_TINTS: Dictionary = {
	AnimState.IDLE:      Color(1.0,  1.0,  1.0,  1.0),
	AnimState.WALKING:   Color(0.8,  0.9,  1.0,  1.0),
	AnimState.WORKING:   Color(0.9,  0.95, 0.6,  1.0),
	AnimState.REPORTING: Color(0.9,  0.85, 0.4,  1.0),
	AnimState.STUNNED:   Color(0.5,  0.3,  0.3,  0.8),
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

@onready var _body: ColorRect = $Body
@onready var _class_label: Label = $ClassLabel
@onready var _animated_sprite: AnimatedSprite2D = $AnimatedSprite2D
@onready var _click_area: Area2D = $ClickArea
@onready var _effect_particles: CPUParticles2D = $EffectParticles
@onready var _ambient_particles: CPUParticles2D = $AmbientParticles


func _ready() -> void:
	_class_label.add_theme_font_size_override("font_size", 7)
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
	_animated_sprite.material = _shader_material
	_apply_class_visuals()
	_apply_state_tint()
	_apply_provider_visuals()
	_push_power_uniforms()
	_apply_particles()


func _process(delta: float) -> void:
	if _anim_state != AnimState.WORKING:
		return
	_pulse_time += delta * 4.0
	var pulse: float = 0.85 + sin(_pulse_time) * 0.15
	_body.modulate = STATE_TINTS[AnimState.WORKING] * Color(pulse, pulse, pulse, 1.0)


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
	var floor_root: Node = get_parent().get_parent()
	reparent(floor_root)
	var tween: Tween = create_tween()
	tween.tween_property(self, "modulate:a", 0.0, 0.4)
	tween.tween_callback(queue_free)


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
	_body.color = Color.WHITE
	_class_label.text = CLASS_LABELS[_character_class]
	if _shader_material != null:
		_shader_material.set_shader_parameter("class_color", CLASS_COLORS[_character_class])


func _apply_state_tint() -> void:
	if _anim_state != AnimState.WORKING:
		_body.modulate = STATE_TINTS[_anim_state]


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
		_effect_particles.texture = ParticleTextures.get_texture(style)
		_effect_particles.color = accent
		_configure_tier_shape(params["tier"], float(params["orbit_radius"]), float(params["orbit_speed"]), float(params["plume_velocity"]))

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
		_ambient_particles.texture = ParticleTextures.get_texture("dot")
		_ambient_particles.color = Color(accent.r, accent.g, accent.b, 0.18)
		_ambient_particles.emission_shape = CPUParticles2D.EMISSION_SHAPE_SPHERE
		_ambient_particles.emission_sphere_radius = 14.0
		_ambient_particles.direction = Vector2(0.0, -1.0)
		_ambient_particles.spread = 30.0
		_ambient_particles.initial_velocity_min = 3.0
		_ambient_particles.initial_velocity_max = 6.0
		_ambient_particles.gravity = Vector2(0.0, -2.0)
		_ambient_particles.scale_amount_min = 1.2
		_ambient_particles.scale_amount_max = 2.0


## Per-tier CPUParticles2D shape/velocity configuration for $EffectParticles.
## Only called while emitting == true (see _apply_particles()).
func _configure_tier_shape(tier: int, orbit_radius: float, orbit_speed: float, plume_velocity: float) -> void:
	match tier:
		ParticleMath.Tier.SPARKLE:
			# Sparse sparkles: point-emit, drift gently upward.
			_effect_particles.emission_shape = CPUParticles2D.EMISSION_SHAPE_POINT
			_effect_particles.direction = Vector2(0.0, -1.0)
			_effect_particles.spread = 60.0
			_effect_particles.initial_velocity_min = 4.0
			_effect_particles.initial_velocity_max = 10.0
			_effect_particles.gravity = Vector2(0.0, -6.0)
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
			_effect_particles.orbit_velocity_min = orbit_speed
			_effect_particles.orbit_velocity_max = orbit_speed
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
			_effect_particles.orbit_velocity_min = orbit_speed
			_effect_particles.orbit_velocity_max = orbit_speed
			_effect_particles.scale_amount_min = 0.9
			_effect_particles.scale_amount_max = 1.4
		_:
			pass


func _on_area_input_event(_viewport: Node, event: InputEvent, _shape_idx: int) -> void:
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed and mb.button_index == MOUSE_BUTTON_LEFT:
			character_clicked.emit(agent_id)
		elif mb.pressed and mb.button_index == MOUSE_BUTTON_RIGHT:
			character_right_clicked.emit(agent_id)
