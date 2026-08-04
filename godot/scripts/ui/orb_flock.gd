extends Control
## OrbFlock — F2 (#175) HUD control replacing the flat SummonBar toolbar
## with three draggable, dockable orbs (Grimoire, Panels, Power). Owns the
## drag/flick/dock state machine and delegates all spring, momentum,
## ricochet, and stack-offset math to OrbPhysics (orb_physics.gd), which
## stays pure and node-free so it is headlessly tested on its own
## (tests/orb_physics_test.gd). This script only wires that math to Orb
## (orb.gd) child controls and to the expand/collapse/flyout UI.
##
## State machine:
##   DOCKED    — flock rests as a stacked deck against a screen edge.
##   DRAGGING  — the pointer holds an orb; the flock chases it with a
##               chained trail (each trailing orb eases toward the one
##               ahead of it, per Must-Have "drag with chained trail").
##   FLYING    — released with velocity: momentum carries the flock, top
##               and bottom edges ricochet, then it springs into DOCKED
##               once the momentum bleeds off.
##   OPEN      — a tap expanded the flock into a horizontal row; the active
##               orb's flyout Control shows below the row.
##
## Flyout registration API (F3/F4/F5 plug in without touching this file):
##   register_flyout(orb_id: String, control: Control) -> void
## The Grimoire flyout (grimoire_flyout.gd) is a child scene node under this
## node; see main.tscn, where it is registered in _ready() the same way the
## F2-era interim SummonBar panel used to be.

class_name OrbFlock

enum State { DOCKED, DRAGGING, FLYING, OPEN }

## True while a flyout is open or a drag is live. PanelManager reads this
## the same way it already reads KeyPassthrough.hover_active: a hotkey gate
## a second, independent system can set, so an open flyout or a live drag
## cannot lose a keystroke to a panel action underneath it.
static var suspend_hotkeys: bool = false

const ORB_DEFS: Array[Dictionary] = [
	{"id": "grimoire", "glyph": "✦", "color": Color("#c9a227")},
	{"id": "panels", "glyph": "◈", "color": Color("#6fb8a8")},
	{"id": "power", "glyph": "⚡", "color": Color("#a4443a")},
]

const ORB_RADIUS: float = 26.0
const ORB_SPACING_PX: float = 64.0
const ROW_TOP_MARGIN_PX: float = 24.0
const FLYOUT_TOP_MARGIN_PX: float = 12.0
const FLYOUT_DEFAULT_SIZE: Vector2 = Vector2(420.0, 220.0)
const DRAG_THRESHOLD_PX: float = 6.0
const MAX_FLICK_SPEED: float = 2200.0
const FLICK_MIN_SPEED: float = 40.0
const CHASE_RATE: float = 14.0

var _orbs: Array[Orb] = []
var _flyouts: Dictionary = {}
var _flyout_host: Control
var _row_rect: Rect2 = Rect2()
var _flyout_rect: Rect2 = Rect2()

var _state: State = State.DOCKED
var _edge: OrbPhysics.Edge = OrbPhysics.Edge.RIGHT
var _dock_height: float = 0.0
var _lead_position: Vector2 = Vector2.ZERO
var _velocity: Vector2 = Vector2.ZERO
var _trail_positions: Array[Vector2] = []

var _dragging_orb_index: int = -1
var _has_dragged: bool = false
var _drag_start_position: Vector2 = Vector2.ZERO
var _drag_previous_position: Vector2 = Vector2.ZERO
var _drag_previous_time: float = 0.0

var _active_flyout_id: String = ""


func _ready() -> void:
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_flyout_host = Control.new()
	_flyout_host.name = "FlyoutHost"
	_flyout_host.mouse_filter = Control.MOUSE_FILTER_STOP
	_flyout_host.visible = false
	add_child(_flyout_host)

	for def: Dictionary in ORB_DEFS:
		var orb: Orb = Orb.new()
		orb.name = "Orb_" + String(def["id"])
		orb.orb_id = String(def["id"])
		orb.glyph = String(def["glyph"])
		orb.accent_color = def["color"]
		orb.orb_radius = ORB_RADIUS
		orb.press_started.connect(_on_orb_press_started)
		orb.press_moved.connect(_on_orb_press_moved)
		orb.press_ended.connect(_on_orb_press_ended)
		add_child(orb)
		_orbs.append(orb)
		_trail_positions.append(Vector2.ZERO)

	_dock_height = get_viewport_rect().size.y * 0.5
	_snap_dock_positions()
	get_viewport().size_changed.connect(_on_viewport_resized)

	# Grimoire Summoning (F3) — the sigil grid and placement-mode controller
	# lives as a child scene node (see main.tscn).
	var grimoire_flyout: Node = get_node_or_null("GrimoireFlyout")
	if grimoire_flyout is Control:
		register_flyout("grimoire", grimoire_flyout as Control)


## Registers `control` as the flyout panel for `orb_id`. The control is
## reparented under this flock's flyout host and hidden until that orb's
## flyout is the active one. Safe to call before or after _ready() runs on
## the flock, and safe to call more than once for the same orb_id (the
## later registration replaces the earlier one).
func register_flyout(orb_id: String, control: Control) -> void:
	if _flyouts.has(orb_id) and _flyouts[orb_id] != control:
		var old: Control = _flyouts[orb_id]
		if old.get_parent() == _flyout_host:
			_flyout_host.remove_child(old)
	_flyouts[orb_id] = control
	if control.get_parent() != _flyout_host:
		if control.get_parent() != null:
			control.get_parent().remove_child(control)
		_flyout_host.add_child(control)
	control.visible = (_state == State.OPEN and _active_flyout_id == orb_id)


func _process(delta: float) -> void:
	match _state:
		State.DRAGGING:
			_update_chain(delta)
			_apply_positions()
		State.FLYING:
			_update_flying(delta)
			_update_chain(delta)
			_apply_positions()
		_:
			pass


func _update_chain(delta: float) -> void:
	_trail_positions[0] = _lead_position
	var rate: float = clampf(CHASE_RATE * delta, 0.0, 1.0)
	for i in range(1, _trail_positions.size()):
		_trail_positions[i] = _trail_positions[i].lerp(_trail_positions[i - 1], rate)


func _update_flying(delta: float) -> void:
	var viewport_size: Vector2 = get_viewport_rect().size
	if not OrbPhysics.momentum_settled(_velocity):
		var result: Dictionary = OrbPhysics.momentum_step(_lead_position, _velocity, delta, viewport_size, ORB_RADIUS, OrbPhysics.DEFAULT_RICOCHET_FRACTION)
		_lead_position = result["position"]
		_velocity = result["velocity"]
		return
	_edge = OrbPhysics.nearest_edge(_lead_position, viewport_size)
	var target: Vector2 = OrbPhysics.edge_dock_anchor(_edge, viewport_size, ORB_RADIUS, _lead_position.y)
	var spring: Dictionary = OrbPhysics.spring_step(_lead_position, target, _velocity, delta)
	_lead_position = spring["position"]
	_velocity = spring["velocity"]
	if spring["settled"]:
		_dock_height = _lead_position.y
		_set_state(State.DOCKED)


func _apply_positions() -> void:
	for i in _orbs.size():
		_orbs[i].position = _trail_positions[i] - Vector2(ORB_RADIUS, ORB_RADIUS)


func _snap_dock_positions() -> void:
	var viewport_size: Vector2 = get_viewport_rect().size
	var anchor: Vector2 = OrbPhysics.edge_dock_anchor(_edge, viewport_size, ORB_RADIUS, _dock_height)
	for i in _orbs.size():
		var pos: Vector2 = anchor + OrbPhysics.stack_offset(i, _edge)
		_trail_positions[i] = pos
		_orbs[i].position = pos - Vector2(ORB_RADIUS, ORB_RADIUS)
	_lead_position = anchor
	# Lead orb (index 0) draws on top of the stack.
	if _orbs.size() > 0:
		move_child(_orbs[0], _orbs.size() - 1)


func _on_viewport_resized() -> void:
	if _state == State.DOCKED:
		_snap_dock_positions()
	elif _state == State.OPEN:
		_layout_row()


func _on_orb_press_started(orb: Orb, _at_position: Vector2) -> void:
	if _state == State.OPEN:
		return
	_dragging_orb_index = _orbs.find(orb)
	_has_dragged = false
	_drag_start_position = orb.global_position + Vector2(ORB_RADIUS, ORB_RADIUS)
	_drag_previous_position = _drag_start_position
	_drag_previous_time = Time.get_ticks_msec() / 1000.0


func _on_orb_press_moved(orb: Orb, at_position: Vector2) -> void:
	if _dragging_orb_index == -1:
		return
	if not _has_dragged:
		if at_position.distance_to(_drag_start_position) < DRAG_THRESHOLD_PX:
			return
		_has_dragged = true
		_set_state(State.DRAGGING)
	var now: float = Time.get_ticks_msec() / 1000.0
	_drag_previous_position = _lead_position
	_drag_previous_time = now
	_lead_position = at_position
	_trail_positions[0] = _lead_position
	_apply_positions()


func _on_orb_press_ended(orb: Orb, at_position: Vector2) -> void:
	if _dragging_orb_index == -1:
		return
	if _has_dragged:
		var now: float = Time.get_ticks_msec() / 1000.0
		var dt: float = maxf(now - _drag_previous_time, 1.0 / 120.0)
		var velocity: Vector2 = OrbPhysics.flick_velocity(_drag_previous_position, at_position, dt, MAX_FLICK_SPEED)
		_velocity = velocity if velocity.length() >= FLICK_MIN_SPEED else Vector2.ZERO
		_lead_position = at_position
		_trail_positions[0] = _lead_position
		_set_state(State.FLYING)
	else:
		_on_orb_tapped(orb)
	_dragging_orb_index = -1
	_has_dragged = false


func _on_orb_tapped(orb: Orb) -> void:
	if _state == State.OPEN:
		if _active_flyout_id == orb.orb_id:
			collapse()
		else:
			_show_flyout(orb.orb_id)
		return
	_expand()


func _expand() -> void:
	_set_state(State.OPEN)
	_layout_row()
	_show_flyout(_orbs[0].orb_id if _orbs.size() > 0 else "")


func collapse() -> void:
	if _state != State.OPEN:
		return
	_set_state(State.DOCKED)
	_active_flyout_id = ""
	_flyout_host.visible = false
	for id: String in _flyouts.keys():
		(_flyouts[id] as Control).visible = false
	_snap_dock_positions()


func _layout_row() -> void:
	var viewport_size: Vector2 = get_viewport_rect().size
	var count: int = _orbs.size()
	var total_width: float = ORB_SPACING_PX * float(maxi(count - 1, 0))
	var start_x: float = (viewport_size.x - total_width) * 0.5
	var row_y: float = ROW_TOP_MARGIN_PX + ORB_RADIUS
	for i in count:
		var pos: Vector2 = Vector2(start_x + ORB_SPACING_PX * float(i), row_y)
		_trail_positions[i] = pos
		_orbs[i].position = pos - Vector2(ORB_RADIUS, ORB_RADIUS)
	_row_rect = Rect2(
		Vector2(start_x - ORB_RADIUS, row_y - ORB_RADIUS),
		Vector2(total_width + ORB_RADIUS * 2.0, ORB_RADIUS * 2.0)
	)


func _show_flyout(orb_id: String) -> void:
	_active_flyout_id = orb_id
	_flyout_host.visible = true
	var viewport_size: Vector2 = get_viewport_rect().size
	var flyout_size: Vector2 = FLYOUT_DEFAULT_SIZE
	var control: Control = _flyouts.get(orb_id, null)
	if control != null:
		control.visible = true
		if control.size != Vector2.ZERO:
			flyout_size = control.size
	for id: String in _flyouts.keys():
		if id != orb_id:
			(_flyouts[id] as Control).visible = false
	var flyout_top: float = ROW_TOP_MARGIN_PX + ORB_RADIUS * 2.0 + FLYOUT_TOP_MARGIN_PX
	var flyout_left: float = (viewport_size.x - flyout_size.x) * 0.5
	_flyout_host.position = Vector2(flyout_left, flyout_top)
	_flyout_host.size = flyout_size
	_flyout_rect = Rect2(_flyout_host.position, flyout_size)
	if control != null:
		control.position = Vector2.ZERO


func _set_state(new_state: State) -> void:
	_state = new_state
	suspend_hotkeys = (new_state == State.OPEN or new_state == State.DRAGGING)


func _input(event: InputEvent) -> void:
	if _state != State.OPEN:
		return
	if event.is_action_pressed("ui_cancel"):
		collapse()
		get_viewport().set_input_as_handled()
		return
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if not mb.pressed or mb.button_index != MOUSE_BUTTON_LEFT:
			return
		if _row_rect.has_point(mb.position) or _flyout_rect.has_point(mb.position):
			return
		collapse()
		get_viewport().set_input_as_handled()
