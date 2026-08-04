extends Control
## PanelBase — reusable floating/docked panel chrome with drag, resize, and state transitions.
##
## T18 (#129) panel floaty animations:
## - CPUParticles2D, never GPUParticles2D. Same justification T17 recorded in
##   agent_character.gd: godot/project.godot sets
##   renderer/rendering_method="gl_compatibility", and Godot 4.2's
##   Compatibility backend does not process GPU particles at all (that
##   landed in 4.3), so a GPUParticles2D node renders NOTHING under this
##   project's renderer. CPUParticles2D configures emission via direct node
##   properties, never a ParticleProcessMaterial (that class is GPU-only).
## - Bug fixed here: MaterializeParticles shipped in T8 as a GPUParticles2D
##   fed a ParticleProcessMaterial — under this renderer the T8 materialise
##   burst has been emitting nothing since it was written. Converted to
##   CPUParticles2D as part of T18.
## - VisualRoot exists SOLELY so the idle bob never enters PanelBase.position.
##   position is read/written directly by drag (_input()'s
##   global_position = ...), _apply_resize(), remember_restore_rect(),
##   set_floating_at(), and by PanelManager's _save_layout(),
##   _restore_layout(), _tween_panel_to_rect(), and _enter_fullscreen()
##   (both of which tween "position"). If a live bob offset were resident in
##   `position` when any of those snapshot or tween it, panels would drift
##   by +/- amplitude every drag/dock/fullscreen/save-restore cycle, and the
##   position tweens would fight the per-frame bob write. So the bob is
##   applied ONLY to $VisualRoot.position.y — a purely visual child
##   transform nothing in T8/T9/T13/T14 or panel_manager.gd reads — and it
##   is always ASSIGNED (never +=), making drift structurally impossible.
## - Handles is intentionally OUTSIDE VisualRoot so resize hit-zones stay
##   anchored to the true panel rect and do not slide with the bob.
## - Terminal panels get NO effects (acceptance: "raw terminal panel stays
##   crisp"). The gate lives here, keyed on mode == "terminal"
##   (see effects_active()); terminal_view.gd is untouched. The quest board
##   (mode == "quest") is a themed panel and KEEPS the effects.

class_name PanelBase

signal close_requested(panel: PanelBase)
signal focus_requested(panel: PanelBase)
signal drag_started(panel: PanelBase)
signal drag_moved(panel: PanelBase, global_rect: Rect2)
signal drag_finished(panel: PanelBase, global_rect: Rect2)
signal resize_started(panel: PanelBase)
signal resize_finished(panel: PanelBase, global_rect: Rect2)
signal fullscreen_requested(panel: PanelBase)
signal restore_requested(panel: PanelBase)
signal mode_changed(panel: PanelBase, mode: String)
signal animation_hook_requested(panel: PanelBase, hook_name: StringName)

enum PanelState {
	FLOATING,
	DOCKING,
	DOCKED,
	UNDOCKING,
	FULLSCREEN,
}

enum ResizeEdge {
	NONE,
	LEFT,
	RIGHT,
	TOP,
	BOTTOM,
	TOP_LEFT,
	TOP_RIGHT,
	BOTTOM_LEFT,
	BOTTOM_RIGHT,
}

const MATERIALIZE_DURATION: float = 0.22
const HANDLE_THICKNESS: float = 12.0
const TITLE_BAR_HEIGHT: float = 36.0
const DEFAULT_BORDER_SHIMMER_COLOR: Color = Color(0.62, 0.72, 0.95, 1.0)

@export var panel_id: String = ""
@export var agent_id: String = ""
@export var panel_title: String = "Panel"
@export var mode: String = "scroll"

var state: PanelState = PanelState.FLOATING
var previous_state: PanelState = PanelState.FLOATING
var restore_rect: Rect2 = Rect2()
var dock_side: String = ""

var _dragging: bool = false
var _resizing: bool = false
var _drag_offset: Vector2 = Vector2.ZERO
var _resize_edge: ResizeEdge = ResizeEdge.NONE
var _resize_origin_rect: Rect2 = Rect2()
var _resize_origin_mouse: Vector2 = Vector2.ZERO
var _materialize_tween: Tween = null

var _effects_enabled: bool = true
var _bob_amplitude_px: float = PanelFloatMath.DEFAULT_BOB_AMPLITUDE_PX
var _bob_period_sec: float = PanelFloatMath.DEFAULT_BOB_PERIOD_SEC
var _flutter_amplitude_px: float = PanelFloatMath.DEFAULT_FLUTTER_AMPLITUDE_PX
var _max_drag_trail_particles: int = PanelFloatMath.DEFAULT_MAX_DRAG_TRAIL_PARTICLES

var _effect_time: float = 0.0
var _bob_phase: float = 0.0
var _drag_trail_last_global_position: Vector2 = Vector2.ZERO

@onready var _visual_root: Control = $VisualRoot
@onready var _background: ColorRect = $VisualRoot/Background
@onready var _title_bar: ColorRect = $VisualRoot/TitleBar
@onready var _title_label: Label = $VisualRoot/TitleBar/TitleLabel
@onready var _mode_button: Button = $VisualRoot/TitleBar/ModeButton
@onready var _close_button: Button = $VisualRoot/TitleBar/CloseButton
@onready var _content_root: MarginContainer = $VisualRoot/ContentRoot
@onready var _border_glow: Panel = $VisualRoot/BorderGlow
@onready var _particles: CPUParticles2D = $VisualRoot/MaterializeParticles
@onready var _drag_trail: CPUParticles2D = $DragTrailParticles

@onready var _handle_left: Control = $Handles/Left
@onready var _handle_right: Control = $Handles/Right
@onready var _handle_top: Control = $Handles/Top
@onready var _handle_bottom: Control = $Handles/Bottom
@onready var _handle_top_left: Control = $Handles/TopLeft
@onready var _handle_top_right: Control = $Handles/TopRight
@onready var _handle_bottom_left: Control = $Handles/BottomLeft
@onready var _handle_bottom_right: Control = $Handles/BottomRight


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_STOP
	focus_mode = Control.FOCUS_ALL
	custom_minimum_size = _scaled_minimum_size()
	size = _max_vec2(size, custom_minimum_size)
	_title_label.text = panel_title
	_mode_button.text = mode.capitalize()
	_title_bar.gui_input.connect(_on_title_bar_input)
	_close_button.pressed.connect(func() -> void: close_requested.emit(self))
	_mode_button.pressed.connect(_on_mode_button_pressed)
	get_viewport().size_changed.connect(_on_viewport_size_changed)
	_configure_particles()
	_configure_handles()
	_bob_phase = PanelFloatMath.phase_for_id(panel_id)
	_configure_border_glow()
	_configure_drag_trail()
	_reset_effect_visuals()
	_refresh_effect_processing()
	_apply_visual_state()
	play_materialize_animation()


func _gui_input(event: InputEvent) -> void:
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed and mb.button_index == MOUSE_BUTTON_LEFT:
			grab_focus()
			focus_requested.emit(self)


func _input(event: InputEvent) -> void:
	if _dragging and event is InputEventMouseMotion:
		var motion: InputEventMouseMotion = event as InputEventMouseMotion
		global_position = motion.global_position - _drag_offset
		drag_moved.emit(self, get_global_rect())
		get_viewport().set_input_as_handled()
	elif _dragging and event is InputEventMouseButton:
		var mb_drag: InputEventMouseButton = event as InputEventMouseButton
		if not mb_drag.pressed and mb_drag.button_index == MOUSE_BUTTON_LEFT:
			_dragging = false
			on_drag_visual_changed(false)
			if _drag_trail != null:
				_drag_trail.emitting = false
			drag_finished.emit(self, get_global_rect())
			get_viewport().set_input_as_handled()
	elif _resizing and event is InputEventMouseMotion:
		var resize_motion: InputEventMouseMotion = event as InputEventMouseMotion
		_apply_resize(resize_motion.global_position)
		get_viewport().set_input_as_handled()
	elif _resizing and event is InputEventMouseButton:
		var mb_resize: InputEventMouseButton = event as InputEventMouseButton
		if not mb_resize.pressed and mb_resize.button_index == MOUSE_BUTTON_LEFT:
			_resizing = false
			_resize_edge = ResizeEdge.NONE
			resize_finished.emit(self, get_global_rect())
			get_viewport().set_input_as_handled()


func get_content_root() -> MarginContainer:
	return _content_root


func get_close_button() -> Button:
	return _close_button


func get_mode_button() -> Button:
	return _mode_button


func get_title_bar() -> ColorRect:
	return _title_bar


func set_panel_title(value: String) -> void:
	panel_title = value
	if is_inside_tree():
		_title_label.text = panel_title


func set_mode(value: String) -> void:
	mode = value
	if is_inside_tree():
		_mode_button.text = mode.capitalize()
	mode_changed.emit(self, mode)
	if is_inside_tree():
		_reset_effect_visuals()
		_refresh_effect_processing()


func set_panel_state(next_state: PanelState) -> void:
	if state == next_state:
		return
	var prior_state: PanelState = state
	previous_state = state
	state = next_state
	_apply_visual_state()
	on_panel_state_changed(prior_state, next_state)


func set_docked(side: String) -> void:
	dock_side = side
	set_panel_state(PanelState.DOCKED)


func set_floating_at(rect: Rect2) -> void:
	restore_rect = rect
	position = rect.position
	size = _max_vec2(rect.size, custom_minimum_size)
	dock_side = ""
	set_panel_state(PanelState.FLOATING)


func toggle_fullscreen() -> void:
	if state == PanelState.FULLSCREEN:
		restore_requested.emit(self)
	else:
		fullscreen_requested.emit(self)


func remember_restore_rect() -> void:
	restore_rect = Rect2(position, size)


func play_materialize_animation() -> void:
	if _materialize_tween:
		_materialize_tween.kill()
	scale = Vector2(0.94, 0.94)
	modulate.a = 0.0
	_particles.position = size * 0.5
	_particles.restart()
	_particles.emitting = true
	on_animation_hook(&"materialize")
	_materialize_tween = create_tween().set_parallel(true)
	_materialize_tween.tween_property(self, "modulate:a", 1.0, MATERIALIZE_DURATION).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)
	_materialize_tween.tween_property(self, "scale", Vector2.ONE, MATERIALIZE_DURATION).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)


func play_undock_animation() -> void:
	on_animation_hook(&"undock")
	var tween: Tween = create_tween().set_parallel(true)
	tween.tween_property(self, "scale", Vector2(1.03, 1.03), 0.1).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "scale", Vector2.ONE, 0.14).set_delay(0.1).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)


func _configure_handles() -> void:
	_bind_handle(_handle_left, ResizeEdge.LEFT, Control.CURSOR_HSIZE)
	_bind_handle(_handle_right, ResizeEdge.RIGHT, Control.CURSOR_HSIZE)
	_bind_handle(_handle_top, ResizeEdge.TOP, Control.CURSOR_VSIZE)
	_bind_handle(_handle_bottom, ResizeEdge.BOTTOM, Control.CURSOR_VSIZE)
	_bind_handle(_handle_top_left, ResizeEdge.TOP_LEFT, Control.CURSOR_FDIAGSIZE)
	_bind_handle(_handle_top_right, ResizeEdge.TOP_RIGHT, Control.CURSOR_BDIAGSIZE)
	_bind_handle(_handle_bottom_left, ResizeEdge.BOTTOM_LEFT, Control.CURSOR_BDIAGSIZE)
	_bind_handle(_handle_bottom_right, ResizeEdge.BOTTOM_RIGHT, Control.CURSOR_FDIAGSIZE)


func _bind_handle(handle: Control, edge: ResizeEdge, cursor_shape: Control.CursorShape) -> void:
	handle.mouse_filter = Control.MOUSE_FILTER_STOP
	handle.mouse_default_cursor_shape = cursor_shape
	handle.gui_input.connect(func(event: InputEvent) -> void:
		if state == PanelState.FULLSCREEN:
			return
		if event is InputEventMouseButton:
			var mb: InputEventMouseButton = event as InputEventMouseButton
			if mb.pressed and mb.button_index == MOUSE_BUTTON_LEFT:
				remember_restore_rect()
				_resizing = true
				_resize_edge = edge
				_resize_origin_rect = Rect2(position, size)
				_resize_origin_mouse = mb.global_position
				resize_started.emit(self)
				get_viewport().set_input_as_handled()
	)


func _apply_resize(global_mouse: Vector2) -> void:
	var delta: Vector2 = global_mouse - _resize_origin_mouse
	var next_rect: Rect2 = _resize_origin_rect
	match _resize_edge:
		ResizeEdge.LEFT:
			next_rect.position.x += delta.x
			next_rect.size.x -= delta.x
		ResizeEdge.RIGHT:
			next_rect.size.x += delta.x
		ResizeEdge.TOP:
			next_rect.position.y += delta.y
			next_rect.size.y -= delta.y
		ResizeEdge.BOTTOM:
			next_rect.size.y += delta.y
		ResizeEdge.TOP_LEFT:
			next_rect.position += delta
			next_rect.size -= delta
		ResizeEdge.TOP_RIGHT:
			next_rect.position.y += delta.y
			next_rect.size.y -= delta.y
			next_rect.size.x += delta.x
		ResizeEdge.BOTTOM_LEFT:
			next_rect.position.x += delta.x
			next_rect.size.x -= delta.x
			next_rect.size.y += delta.y
		ResizeEdge.BOTTOM_RIGHT:
			next_rect.size += delta
	var min_size: Vector2 = custom_minimum_size
	if next_rect.size.x < min_size.x:
		if _resize_edge in [ResizeEdge.LEFT, ResizeEdge.TOP_LEFT, ResizeEdge.BOTTOM_LEFT]:
			next_rect.position.x = _resize_origin_rect.end.x - min_size.x
		next_rect.size.x = min_size.x
	if next_rect.size.y < min_size.y:
		if _resize_edge in [ResizeEdge.TOP, ResizeEdge.TOP_LEFT, ResizeEdge.TOP_RIGHT]:
			next_rect.position.y = _resize_origin_rect.end.y - min_size.y
		next_rect.size.y = min_size.y
	position = next_rect.position
	size = next_rect.size


func _apply_visual_state() -> void:
	_background.color = Color(0.08, 0.1, 0.12, 0.92 if state == PanelState.FLOATING else 0.97)
	_title_bar.color = Color(0.17, 0.21, 0.25, 0.98)
	var resize_visible: bool = state == PanelState.FLOATING
	$Handles.visible = resize_visible


func _on_title_bar_input(event: InputEvent) -> void:
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed and mb.button_index == MOUSE_BUTTON_LEFT:
			grab_focus()
			focus_requested.emit(self)
			if mb.double_click:
				toggle_fullscreen()
				get_viewport().set_input_as_handled()
				return
			if state == PanelState.FULLSCREEN:
				get_viewport().set_input_as_handled()
				return
			remember_restore_rect()
			_dragging = true
			_drag_offset = mb.global_position - global_position
			_drag_trail_last_global_position = global_position
			on_drag_visual_changed(true)
			drag_started.emit(self)
			get_viewport().set_input_as_handled()


func _on_mode_button_pressed() -> void:
	set_mode("terminal" if mode == "scroll" else "scroll")


func _on_viewport_size_changed() -> void:
	custom_minimum_size = _scaled_minimum_size()
	size = _max_vec2(size, custom_minimum_size)


func _scaled_minimum_size() -> Vector2:
	var viewport_height: float = maxf(get_viewport_rect().size.y, 1080.0)
	var scale_factor: float = viewport_height / 1080.0
	return Vector2(roundf(300.0 * scale_factor), roundf(200.0 * scale_factor))


## T18 (#129) — MaterializeParticles was a GPUParticles2D fed a
## ParticleProcessMaterial in T8. Under this project's gl_compatibility
## renderer (project.godot renderer/rendering_method) Godot 4.2 does not
## process GPU particles at all — Compatibility-backend GPUParticles2D
## support only landed in 4.3 — so the materialise burst has been invisible
## since it was written. Converted to CPUParticles2D, which is CPU-simulated
## and renders correctly on every backend; it is configured through direct
## node properties because ParticleProcessMaterial is a GPU-only class.
func _configure_particles() -> void:
	if _particles == null:
		return
	_particles.emitting = false
	_particles.texture = ParticleTextures.get_texture("dot")
	_particles.emission_shape = CPUParticles2D.EMISSION_SHAPE_SPHERE
	_particles.emission_sphere_radius = 84.0
	_particles.direction = Vector2(0.0, 0.0)
	_particles.spread = 180.0
	_particles.initial_velocity_min = 90.0
	_particles.initial_velocity_max = 170.0
	_particles.gravity = Vector2.ZERO
	_particles.scale_amount_min = 1.0
	_particles.scale_amount_max = 2.4
	_particles.color = Color(0.72, 0.82, 1.0, 0.85)


## T18 (#129) — panel effect config, threaded ONCE from PanelManager
## (tower.json -> TowerConfig -> here), mirroring T17's
## FloorScene.configure_particle_budget(). Deliberately NOT read from disk
## per panel. Called from PanelManager._wire_panel() immediately after the
## panel enters the tree and BEFORE PanelContentRouter.mount(), so a
## SpellScrollView's setup() can read get_effect_settings() and apply its
## flutter uniforms in the same frame.
##
## `enabled == false` must disable ALL FOUR effects (bob, parchment flutter,
## drag trail, border shimmer) — not merely the particles.
func configure_panel_effects(
	enabled: bool,
	bob_amplitude_px: float,
	bob_period_sec: float,
	flutter_amplitude_px: float,
	max_drag_trail_particles: int
) -> void:
	_effects_enabled = enabled
	_bob_amplitude_px = maxf(bob_amplitude_px, 0.0)
	_bob_period_sec = maxf(bob_period_sec, 0.0)
	_flutter_amplitude_px = maxf(flutter_amplitude_px, 0.0)
	_max_drag_trail_particles = maxi(max_drag_trail_particles, 0)
	_bob_phase = PanelFloatMath.phase_for_id(panel_id)
	# Re-pin the trail's particle pool. _configure_drag_trail() already ran in
	# _ready(), i.e. BEFORE PanelManager threaded the real config in, so at
	# that point it sized the pool from the built-in default. Without this
	# second call the configured max_drag_trail_particles would never reach
	# the node: the per-frame path deliberately does not touch `amount` (that
	# reallocates the pool and wipes a live trail), so `amount` is only ever
	# written here and in _configure_drag_trail(). A budget of 0 still
	# disables the trail through drag_trail_params()'s emitting gate rather
	# than through the pool size, which CPUParticles2D requires to be >= 1.
	_configure_drag_trail()
	_reset_effect_visuals()
	_refresh_effect_processing()


## Read by SpellScrollView.setup() to drive the parchment flutter uniforms.
## `enabled` here is the COMBINED gate (config knob AND non-terminal mode),
## so a caller never has to re-derive the terminal rule.
func get_effect_settings() -> Dictionary:
	return {
		"enabled": effects_active(),
		"bob_amplitude_px": _bob_amplitude_px,
		"bob_period_sec": _bob_period_sec,
		"flutter_amplitude_px": _flutter_amplitude_px,
		"max_drag_trail_particles": _max_drag_trail_particles,
	}


## Single source of truth for "should this panel be floaty at all".
## Acceptance: the raw terminal panel has NO floaty animations. The gate
## lives here (keyed on mode) rather than in terminal_view.gd, so a panel
## live-toggled to terminal mode via the title-bar Mode button goes crisp
## immediately without the view needing to know about effects. The quest
## board ("quest") is a themed panel and KEEPS its effects.
func effects_active() -> bool:
	return _effects_enabled and mode != "terminal"


func _refresh_effect_processing() -> void:
	set_process(effects_active())


## Returns every effect to its neutral, zero state. The bob offset is
## ASSIGNED to zero here (never subtracted from an accumulator), which is
## what makes drift structurally impossible — see the class doc-comment.
func _reset_effect_visuals() -> void:
	if _visual_root != null:
		_visual_root.position.y = 0.0
	if _border_glow != null and not _passthrough_active:
		_border_glow.modulate.a = 0.0
	if _drag_trail != null:
		_drag_trail.emitting = false


func _process(delta: float) -> void:
	if not effects_active():
		_reset_effect_visuals()
		return
	_effect_time += delta
	_apply_bob()
	_apply_border_shimmer()
	_apply_drag_trail(delta)


## Idle ambient float (user decision A): a slow continuous sine on FLOATING
## panels only. Docked, docking/undocking, and fullscreen panels do not bob,
## and a panel being actively dragged or resized does not bob either (the
## bob would fight the pointer). The offset is written to VisualRoot — never
## to `position`; and it is always an assignment, never a `+=`.
func _apply_bob() -> void:
	if _visual_root == null:
		return
	var bobbing: bool = state == PanelState.FLOATING and not _dragging and not _resizing
	if not bobbing:
		_visual_root.position.y = 0.0
		return
	_visual_root.position.y = PanelFloatMath.bob_offset_snapped(
		_effect_time, _bob_amplitude_px, _bob_period_sec, _bob_phase
	)


func _apply_border_shimmer() -> void:
	if _border_glow == null:
		return
	# The passthrough signal owns the border while active. Full alpha, no
	# shimmer, so the amber reads as a steady state and not an effect.
	if _passthrough_active:
		_border_glow.modulate.a = 1.0
		return
	_border_glow.modulate.a = PanelFloatMath.shimmer_alpha(_effect_time, _bob_phase)


## Faint particle trail while a panel is being dragged. Emission scales with
## instantaneous drag speed and is budget-clamped by
## PanelFloatMath.drag_trail_params() exactly the way
## ParticleMath.emission_amount_for() clamps — a max_drag_trail_particles of
## 0, or a panel barely moving, yields emitting:false / amount:0 rather than
## a floored-up single particle.
func _apply_drag_trail(delta: float) -> void:
	if _drag_trail == null:
		return
	if not _dragging:
		_drag_trail.emitting = false
		return
	_drag_trail.position = size * 0.5
	_drag_trail.emission_rect_extents = size * 0.5
	var travelled: float = global_position.distance_to(_drag_trail_last_global_position)
	_drag_trail_last_global_position = global_position
	var speed: float = travelled / maxf(delta, 0.0001)
	var params: Dictionary = PanelFloatMath.drag_trail_params(speed, _max_drag_trail_particles)
	var emitting: bool = bool(params["emitting"])
	if not emitting:
		_drag_trail.emitting = false
		return
	# CPUParticles2D reallocates its particle pool (and restarts every live
	# particle) whenever `amount` is assigned. The pool size is therefore only
	# ever written outside the per-frame path — in _configure_drag_trail(),
	# called from _ready() and again from configure_panel_effects() once the
	# real budget arrives. Per-frame speed modulates only non-destructive
	# params (lifetime, velocity) so a live trail is never wiped mid-drag.
	_drag_trail.lifetime = maxf(0.05, float(params["lifetime"]))
	_drag_trail.initial_velocity_min = float(params["velocity"]) * 0.4
	_drag_trail.initial_velocity_max = float(params["velocity"])
	_drag_trail.emitting = true


func _configure_drag_trail() -> void:
	if _drag_trail == null:
		return
	_drag_trail.emitting = false
	# Global-space particles: they stay put while the panel moves away, which
	# is what makes this read as a trail rather than an attached puff.
	_drag_trail.local_coords = false
	_drag_trail.texture = ParticleTextures.get_texture("dot")
	_drag_trail.color = Color(0.72, 0.82, 1.0, 0.28)
	_drag_trail.emission_shape = CPUParticles2D.EMISSION_SHAPE_RECTANGLE
	_drag_trail.emission_rect_extents = size * 0.5
	_drag_trail.direction = Vector2(0.0, 1.0)
	_drag_trail.spread = 180.0
	_drag_trail.gravity = Vector2(0.0, 14.0)
	_drag_trail.scale_amount_min = 0.5
	_drag_trail.scale_amount_max = 1.3
	_drag_trail.damping_min = 12.0
	_drag_trail.damping_max = 12.0
	_drag_trail.amount = maxi(_max_drag_trail_particles, 1)


## True while a hovered view forwards every keystroke to the agent's tmux
## session. The border turns solid amber as the visible signal.
var _passthrough_active: bool = false
var _passthrough_stylebox: StyleBoxFlat = null

## Amber border color while key passthrough is live. Matches the tower's
## active-edge accent so the signal reads as "this panel owns the keyboard".
const PASSTHROUGH_BORDER_COLOR: Color = Color("#FBB13C")


## Turns the passthrough border signal on or off. The amber stylebox is a
## duplicate, because the scene's StyleBoxFlat sub-resource is shared across
## every PanelBase instance and a direct mutation would tint all of them.
func set_passthrough_active(active: bool) -> void:
	if _border_glow == null or _passthrough_active == active:
		return
	_passthrough_active = active
	if active:
		if _passthrough_stylebox == null:
			var base: StyleBox = _border_glow.get_theme_stylebox("panel")
			var amber: StyleBoxFlat = base.duplicate() as StyleBoxFlat
			if amber == null:
				amber = StyleBoxFlat.new()
				amber.draw_center = false
				amber.set_border_width_all(2)
			amber.border_color = PASSTHROUGH_BORDER_COLOR
			_passthrough_stylebox = amber
		_border_glow.add_theme_stylebox_override("panel", _passthrough_stylebox)
		_border_glow.modulate.a = 1.0
	else:
		_border_glow.remove_theme_stylebox_override("panel")
		_border_glow.modulate.a = 0.0


func _configure_border_glow() -> void:
	if _border_glow == null:
		return
	_border_glow.mouse_filter = Control.MOUSE_FILTER_IGNORE
	# The shimmer hue already lives on the StyleBoxFlat's `border_color` (see
	# panel_base.tscn). `modulate` multiplies whatever the node draws, so
	# setting its RGB to the same hue here would double-apply the tint
	# (effectively squaring each channel) — only alpha is ours to drive.
	_border_glow.modulate.a = 0.0


func _max_vec2(a: Vector2, b: Vector2) -> Vector2:
	return Vector2(maxf(a.x, b.x), maxf(a.y, b.y))


func on_animation_hook(hook_name: StringName) -> void:
	animation_hook_requested.emit(self, hook_name)


func on_panel_state_changed(_from_state: PanelState, _to_state: PanelState) -> void:
	pass


func on_drag_visual_changed(_active: bool) -> void:
	pass
