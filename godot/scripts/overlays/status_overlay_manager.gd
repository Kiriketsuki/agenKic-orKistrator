extends Control
## StatusOverlayManager — owns the single "enchanted nameplate" tooltip and
## drives it from hover/click signals relayed up through TowerManager and
## live SSE updates from BridgeManager. Lives on a high CanvasLayer above the
## floor scene and T8 panels; every child Control is mouse_filter=IGNORE so
## the overlay can never intercept a click.

const STATUS_OVERLAY_SCENE: PackedScene = preload("res://scenes/status_overlay.tscn")
const UPTIME_TICK_INTERVAL: float = 1.0
const HOVER_ANCHOR_OFFSET: Vector2 = Vector2(0.0, -14.0)
const VIEWPORT_MARGIN: float = 4.0

var _tower_manager: Node = null
var _bridge_manager: Node = null
var _view: StatusOverlay = null

var _agent_id: String = ""
var _pinned: bool = false
var _pin_pos: Vector2 = Vector2.ZERO
var _mouse_over_agent: bool = false
var _uptime_origin: float = 0.0
var _uptime_tick_accum: float = 0.0

## agent_id -> unix seconds first observed. Fallback for agents whose
## registered_at field is 0 (unset by the orchestrator).
var _first_seen: Dictionary = {}


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	_tower_manager = get_node_or_null("../../Tower")
	_bridge_manager = get_node_or_null("/root/BridgeManager")

	if _tower_manager != null:
		if _tower_manager.has_signal("agent_hover_requested"):
			_tower_manager.connect("agent_hover_requested", _on_hover_requested)
		if _tower_manager.has_signal("agent_unhover_requested"):
			_tower_manager.connect("agent_unhover_requested", _on_unhover_requested)
		if _tower_manager.has_signal("agent_panel_requested"):
			_tower_manager.connect("agent_panel_requested", _on_panel_requested)

	if _bridge_manager != null:
		if _bridge_manager.has_signal("agent_state_changed"):
			_bridge_manager.connect("agent_state_changed", _on_agent_state_changed)
		if _bridge_manager.has_signal("agent_deregistered"):
			_bridge_manager.connect("agent_deregistered", _on_agent_deregistered)
		if _bridge_manager.has_signal("agent_registered"):
			_bridge_manager.connect("agent_registered", _on_agent_registered)


func _process(delta: float) -> void:
	if _agent_id.is_empty() or _view == null or not is_instance_valid(_view):
		return

	_uptime_tick_accum += delta
	if _uptime_tick_accum >= UPTIME_TICK_INTERVAL:
		_uptime_tick_accum = 0.0
		_view.set_uptime_seconds(int(Time.get_unix_time_from_system() - _uptime_origin))

	if _pinned:
		_position_at(_pin_pos)
		return

	var char_node: AgentCharacter = null
	if _tower_manager != null and _tower_manager.has_method("get_agent_character"):
		char_node = _tower_manager.get_agent_character(_agent_id)
	# Agents can despawn (deregister) or be culled from view (edge rotation,
	# fisheye distance) while hovered — dismiss rather than follow a stale node.
	if char_node == null or not is_instance_valid(char_node) or not char_node.is_visible_in_tree():
		_dismiss()
		return
	_position_over_character(char_node)


func _unhandled_input(event: InputEvent) -> void:
	if not _pinned:
		return
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed and mb.button_index == MOUSE_BUTTON_LEFT and not _mouse_over_agent:
			_dismiss()


# ---------------------------------------------------------------------------
# TowerManager relay handlers
# ---------------------------------------------------------------------------

## While the nameplate is pinned (click-anchored, scroll panel open), hovering
## a *different* agent must not hijack it — the pin owns _agent_id/_pin_pos
## until the user dismisses it by clicking elsewhere. Re-hovering the pinned
## agent itself is a harmless no-op (already showing its data).
func _on_hover_requested(agent_id: String) -> void:
	if _pinned:
		if agent_id == _agent_id:
			_mouse_over_agent = true
		return
	_mouse_over_agent = true
	_show_for_agent(agent_id)


func _on_unhover_requested(agent_id: String) -> void:
	if _pinned:
		if agent_id == _agent_id:
			_mouse_over_agent = false
		return
	if agent_id != _agent_id:
		return
	_mouse_over_agent = false
	_dismiss()


## The click->scroll-panel flow (character_clicked -> agent_clicked ->
## agent_panel_requested -> PanelManager.open_scroll_panel) is untouched by
## this handler — it only pins the already-independent nameplate so it
## survives while the scroll panel opens.
func _on_panel_requested(agent_id: String) -> void:
	_pinned = true
	_pin_pos = get_viewport().get_mouse_position()
	_show_for_agent(agent_id)


# ---------------------------------------------------------------------------
# BridgeManager signal handlers
# ---------------------------------------------------------------------------

func _on_agent_state_changed(agent_id: String, _old_state: String, _new_state: String, task_id: String) -> void:
	if agent_id != _agent_id or _view == null or not is_instance_valid(_view):
		return
	var agent_data: BridgeData.AgentData = _get_agent(agent_id)
	# agent.state_changed does not update _agents[id].current_task_id on
	# BridgeManager, only .state — use the signal's own task_id, which is
	# the freshest value, rather than the (possibly stale) agent_data field.
	_view.populate(agent_id, agent_data, task_id)


func _on_agent_deregistered(agent_id: String) -> void:
	_first_seen.erase(agent_id)
	if agent_id == _agent_id:
		_dismiss()


func _on_agent_registered(agent_data: BridgeData.AgentData) -> void:
	if agent_data == null:
		return
	if agent_data.registered_at == 0 and not _first_seen.has(agent_data.id):
		_first_seen[agent_data.id] = Time.get_unix_time_from_system()


# ---------------------------------------------------------------------------
# Show / position / dismiss
# ---------------------------------------------------------------------------

func _show_for_agent(agent_id: String) -> void:
	var agent_data: BridgeData.AgentData = _get_agent(agent_id)
	_agent_id = agent_id
	_ensure_view()
	var task_id: String = agent_data.current_task_id if agent_data != null else ""
	_view.populate(agent_id, agent_data, task_id)
	_view.visible = true

	var now: float = Time.get_unix_time_from_system()
	if agent_data != null and agent_data.registered_at > 0:
		_uptime_origin = float(agent_data.registered_at)
	elif _first_seen.has(agent_id):
		_uptime_origin = _first_seen[agent_id]
	else:
		_uptime_origin = now
		_first_seen[agent_id] = now
	_uptime_tick_accum = 0.0
	_view.set_uptime_seconds(int(now - _uptime_origin))

	var char_node: AgentCharacter = null
	if _tower_manager != null and _tower_manager.has_method("get_agent_character"):
		char_node = _tower_manager.get_agent_character(agent_id)
	if char_node != null and is_instance_valid(char_node):
		_position_over_character(char_node)
	else:
		_position_at(get_viewport().get_mouse_position())


func _ensure_view() -> void:
	if _view != null and is_instance_valid(_view):
		return
	_view = STATUS_OVERLAY_SCENE.instantiate() as StatusOverlay
	_view.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_view)


func _dismiss() -> void:
	_agent_id = ""
	_pinned = false
	if _view != null and is_instance_valid(_view):
		_view.visible = false


func _position_over_character(char_node: AgentCharacter) -> void:
	var xform: Transform2D = char_node.get_global_transform_with_canvas()
	_position_at(xform * HOVER_ANCHOR_OFFSET)


## `anchor_pos` is a screen pixel position (this Control's CanvasLayer uses
## the identity transform, so viewport/global pixel coordinates line up).
func _position_at(anchor_pos: Vector2) -> void:
	if _view == null or not is_instance_valid(_view):
		return
	var size: Vector2 = _view.size
	if size == Vector2.ZERO:
		size = _view.get_combined_minimum_size()
	var pos: Vector2 = anchor_pos - Vector2(size.x / 2.0, size.y)
	var viewport_size: Vector2 = get_viewport_rect().size
	pos.x = clampf(pos.x, VIEWPORT_MARGIN, maxf(VIEWPORT_MARGIN, viewport_size.x - size.x - VIEWPORT_MARGIN))
	pos.y = clampf(pos.y, VIEWPORT_MARGIN, maxf(VIEWPORT_MARGIN, viewport_size.y - size.y - VIEWPORT_MARGIN))
	_view.position = pos


func _get_agent(agent_id: String) -> BridgeData.AgentData:
	if _bridge_manager != null and _bridge_manager.has_method("get_agent"):
		return _bridge_manager.get_agent(agent_id)
	return null
