extends Control
## PanelBase — reusable floating/docked panel chrome with drag, resize, and state transitions.

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

@onready var _background: ColorRect = $Background
@onready var _title_bar: ColorRect = $TitleBar
@onready var _title_label: Label = $TitleBar/TitleLabel
@onready var _mode_button: Button = $TitleBar/ModeButton
@onready var _close_button: Button = $TitleBar/CloseButton
@onready var _content_root: MarginContainer = $ContentRoot
@onready var _particles: GPUParticles2D = $MaterializeParticles

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


func set_panel_title(value: String) -> void:
	panel_title = value
	if is_inside_tree():
		_title_label.text = panel_title


func set_mode(value: String) -> void:
	mode = value
	if is_inside_tree():
		_mode_button.text = mode.capitalize()
	mode_changed.emit(self, mode)


func set_panel_state(next_state: PanelState) -> void:
	if state == next_state:
		return
	previous_state = state
	state = next_state
	_apply_visual_state()


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
	_particles.restart()
	_particles.emitting = true
	_materialize_tween = create_tween().set_parallel(true)
	_materialize_tween.tween_property(self, "modulate:a", 1.0, MATERIALIZE_DURATION).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)
	_materialize_tween.tween_property(self, "scale", Vector2.ONE, MATERIALIZE_DURATION).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	animation_hook_requested.emit(self, &"materialize")


func play_undock_animation() -> void:
	var tween: Tween = create_tween().set_parallel(true)
	tween.tween_property(self, "scale", Vector2(1.03, 1.03), 0.1).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "scale", Vector2.ONE, 0.14).set_delay(0.1).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	animation_hook_requested.emit(self, &"undock")


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
			remember_restore_rect()
			_dragging = true
			_drag_offset = mb.global_position - global_position
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


func _configure_particles() -> void:
	if _particles.process_material != null:
		return
	var material: ParticleProcessMaterial = ParticleProcessMaterial.new()
	material.emission_shape = ParticleProcessMaterial.EMISSION_SHAPE_SPHERE
	material.emission_sphere_radius = 84.0
	material.initial_velocity_min = 90.0
	material.initial_velocity_max = 170.0
	material.scale_min = 1.0
	material.scale_max = 2.4
	material.direction = Vector3(0.0, 0.0, 0.0)
	material.gravity = Vector3.ZERO
	_particles.process_material = material


func _max_vec2(a: Vector2, b: Vector2) -> Vector2:
	return Vector2(maxf(a.x, b.x), maxf(a.y, b.y))
