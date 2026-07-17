extends PopupMenu
## AgentContextMenu — the single reusable right-click context menu for agent
## characters (T14 / #119). Themed parchment/ink to match the rest of the
## Magic Tower UI (palette borrowed from spell_scroll_view.gd / quest_board_view.gd).
##
## Lives under OverlayLayer as a sibling of StatusOverlayManager so the
## relative node paths below resolve the same way StatusOverlayManager's do.
## One instance serves every agent — _target_id is retargeted on every
## invocation and all dispatched actions read the *current* agent state via
## BridgeManager.get_agent() rather than caching a snapshot, so a despawn
## between popup() and an action press degrades to a graceful no-op instead
## of touching a stale node.

class_name AgentContextMenu

const ITEM_VIEW_DETAILS: int = 0
const ITEM_OPEN_TERMINAL: int = 1
const ITEM_OPEN_SCROLL: int = 2
const ITEM_REASSIGN: int = 3
const ITEM_CANCEL: int = 4
const ITEM_COPY_OUTPUT: int = 5

## Reassign submenu item ids are offset so they never collide with the
## top-level menu's ids (0-5 above).
const REASSIGN_ID_BASE: int = 100
const REASSIGN_PROVIDERS: Array[String] = ["claude", "gemini", "openai", "ollama", "deepseek"]

const COPY_LINES: int = 200

const INK_COLOR: Color = Color(0.22, 0.16, 0.10, 1.0)
const INK_DISABLED_COLOR: Color = Color(0.22, 0.16, 0.10, 0.4)
const PARCHMENT_BG: Color = Color(0.90, 0.84, 0.70, 1.0)
const PARCHMENT_BORDER: Color = Color(0.45, 0.32, 0.16, 1.0)
const SUCCESS_COLOR: Color = Color(0.16, 0.4, 0.16, 1.0)
const ERROR_COLOR: Color = Color(0.55, 0.12, 0.1, 1.0)

var _bridge: Node = null
var _tower: Node = null
var _panels: Node = null
var _overlay: Node = null

var _target_id: String = ""
var _reassign_submenu: PopupMenu = null

var _copy_pending: bool = false
var _copy_agent_id: String = ""

var _toast: Label = null
var _toast_tween: Tween = null


func _ready() -> void:
	_bridge = get_node_or_null("/root/BridgeManager")
	_tower = get_node_or_null("../../Tower")
	_panels = get_node_or_null("../../UILayer/PanelManager")
	_overlay = get_node_or_null("../StatusOverlayManager")

	_apply_theme()
	_build_items()

	if _tower != null and _tower.has_signal("agent_context_menu_requested"):
		_tower.connect("agent_context_menu_requested", _on_context_menu_requested)

	id_pressed.connect(_on_id_pressed)

	if _bridge != null:
		if _bridge.has_signal("agent_output_history"):
			_bridge.connect("agent_output_history", _on_copy_history)
		if _bridge.has_signal("command_succeeded"):
			_bridge.connect("command_succeeded", _on_command_succeeded)
		if _bridge.has_signal("command_failed"):
			_bridge.connect("command_failed", _on_command_failed)


# ---------------------------------------------------------------------------
# Menu construction / theming
# ---------------------------------------------------------------------------

func _build_items() -> void:
	clear()
	add_item("View details", ITEM_VIEW_DETAILS)
	add_item("Open terminal", ITEM_OPEN_TERMINAL)
	add_item("Open scroll", ITEM_OPEN_SCROLL)
	add_separator()

	# add_submenu_item (rather than the 4.3+ add_submenu_node_item, unavailable
	# on this project's target 4.2) resolves its submenu by child node NAME —
	# ReassignSubmenu must already be present as a child (either the one
	# instantiated here, or the .tscn-authored node of the same name; either
	# way get_node_or_null keeps this idempotent across repeated _build_items
	# calls).
	_reassign_submenu = get_node_or_null("ReassignSubmenu") as PopupMenu
	if _reassign_submenu == null:
		_reassign_submenu = PopupMenu.new()
		_reassign_submenu.name = "ReassignSubmenu"
		add_child(_reassign_submenu)
	else:
		_reassign_submenu.clear()
	for i: int in range(REASSIGN_PROVIDERS.size()):
		_reassign_submenu.add_item(REASSIGN_PROVIDERS[i].capitalize(), REASSIGN_ID_BASE + i)
	if not _reassign_submenu.id_pressed.is_connected(_on_reassign_id_pressed):
		_reassign_submenu.id_pressed.connect(_on_reassign_id_pressed)
	_apply_submenu_theme(_reassign_submenu)
	add_submenu_item("Reassign task", _reassign_submenu.name, ITEM_REASSIGN)

	add_item("Cancel task", ITEM_CANCEL)
	add_separator()
	add_item("Copy output", ITEM_COPY_OUTPUT)


func _apply_theme() -> void:
	var panel_style: StyleBoxFlat = StyleBoxFlat.new()
	panel_style.bg_color = PARCHMENT_BG
	panel_style.border_color = PARCHMENT_BORDER
	panel_style.set_border_width_all(2)
	panel_style.set_corner_radius_all(2)
	add_theme_stylebox_override("panel", panel_style)
	add_theme_color_override("font_color", INK_COLOR)
	add_theme_color_override("font_hover_color", PARCHMENT_BG)
	add_theme_color_override("font_disabled_color", INK_DISABLED_COLOR)
	add_theme_color_override("font_separator_color", INK_COLOR)


func _apply_submenu_theme(menu: PopupMenu) -> void:
	var panel_style: StyleBoxFlat = StyleBoxFlat.new()
	panel_style.bg_color = PARCHMENT_BG
	panel_style.border_color = PARCHMENT_BORDER
	panel_style.set_border_width_all(2)
	panel_style.set_corner_radius_all(2)
	menu.add_theme_stylebox_override("panel", panel_style)
	menu.add_theme_color_override("font_color", INK_COLOR)
	menu.add_theme_color_override("font_hover_color", PARCHMENT_BG)
	menu.add_theme_color_override("font_disabled_color", INK_DISABLED_COLOR)


# ---------------------------------------------------------------------------
# TowerManager relay handler
# ---------------------------------------------------------------------------

func _on_context_menu_requested(agent_id: String, screen_position: Vector2) -> void:
	if _bridge == null:
		return
	var agent_data: BridgeData.AgentData = _bridge.get_agent(agent_id) if _bridge.has_method("get_agent") else null
	if agent_data == null:
		return
	_target_id = agent_id

	var has_task: bool = not agent_data.current_task_id.is_empty()
	set_item_disabled(get_item_index(ITEM_REASSIGN), not has_task)
	set_item_disabled(get_item_index(ITEM_CANCEL), not has_task)

	# Popup's Rect2i position is in viewport/screen pixels — matches
	# get_viewport().get_mouse_position() the same way
	# StatusOverlayManager._position_at() treats its anchor_pos, assuming the
	# project's default embedded-subwindows setting (gui_embed_subwindows on).
	popup(Rect2i(Vector2i(screen_position), Vector2i.ZERO))


# ---------------------------------------------------------------------------
# Action dispatch
# ---------------------------------------------------------------------------

func _on_id_pressed(id: int) -> void:
	if not is_instance_valid(self) or _target_id.is_empty():
		return
	if _bridge == null or (_bridge.has_method("get_agent") and _bridge.get_agent(_target_id) == null):
		# Agent despawned between popup() and this press — no-op.
		return

	match id:
		ITEM_VIEW_DETAILS:
			if _overlay != null and _overlay.has_method("show_details"):
				_overlay.show_details(_target_id)
		ITEM_OPEN_TERMINAL:
			if _panels != null and _panels.has_method("open_agent_terminal"):
				_panels.open_agent_terminal(_target_id)
		ITEM_OPEN_SCROLL:
			if _panels != null and _panels.has_method("open_scroll_panel"):
				_panels.open_scroll_panel(_target_id)
		ITEM_CANCEL:
			if _bridge.has_method("cancel_agent_task"):
				_bridge.cancel_agent_task(_target_id)
		ITEM_COPY_OUTPUT:
			_start_copy_output(_target_id)
		# ITEM_REASSIGN has no direct action — it only opens the submenu.


func _on_reassign_id_pressed(id: int) -> void:
	if _target_id.is_empty() or _bridge == null:
		return
	if _bridge.has_method("get_agent") and _bridge.get_agent(_target_id) == null:
		return
	var idx: int = id - REASSIGN_ID_BASE
	if idx < 0 or idx >= REASSIGN_PROVIDERS.size():
		return
	if _bridge.has_method("reassign_agent"):
		_bridge.reassign_agent(_target_id, {"provider": REASSIGN_PROVIDERS[idx]})


# ---------------------------------------------------------------------------
# Copy output (async, one-shot)
# ---------------------------------------------------------------------------

func _start_copy_output(agent_id: String) -> void:
	if _copy_pending:
		return
	if _bridge == null or not _bridge.has_method("fetch_agent_output_history"):
		return
	_copy_pending = true
	_copy_agent_id = agent_id
	_bridge.fetch_agent_output_history(agent_id, COPY_LINES)


## Connected once in _ready (not CONNECT_ONE_SHOT on the signal itself,
## since this node's whole lifetime spans many copy requests) — the
## _copy_pending guard plus the aid == _copy_agent_id check give the
## equivalent one-shot-per-request semantics without repeated
## connect/disconnect churn. fetch_agent_output_history always emits exactly
## once per call (even with an empty [] on failure), so _copy_pending is
## always cleared.
func _on_copy_history(agent_id: String, chunks: Array) -> void:
	if agent_id != _copy_agent_id:
		# Not our in-flight request (e.g. a concurrent scroll-panel backfill
		# sharing the same signal) — ignore, keep waiting for ours.
		return
	_copy_pending = false

	var lines: Array = []
	for chunk: Variant in chunks:
		if chunk is BridgeData.AgentOutputChunk:
			lines.append((chunk as BridgeData.AgentOutputChunk).payload)
		elif chunk is String:
			lines.append(chunk)
	var text: String = "\n".join(lines)
	if text.strip_edges().is_empty():
		_toast_message("No output to copy", ERROR_COLOR)
		return
	DisplayServer.clipboard_set(text)
	_toast_message("Copied %d lines" % lines.size(), SUCCESS_COLOR)


# ---------------------------------------------------------------------------
# Feedback toast (reuses the T13 command_succeeded/command_failed pattern —
# see quest_board_view._show_status — rather than a new notification
# framework)
# ---------------------------------------------------------------------------

func _on_command_succeeded(path: String, _code: int, _body: String) -> void:
	if path.ends_with("/cancel"):
		_toast_message("Task cancelled", SUCCESS_COLOR)
	elif path.ends_with("/reassign"):
		_toast_message("Task reassigned", SUCCESS_COLOR)


func _on_command_failed(path: String, _code: int, reason: String) -> void:
	if path.ends_with("/cancel") or path.ends_with("/reassign"):
		_toast_message(reason, ERROR_COLOR)


func _toast_message(text: String, color: Color) -> void:
	_ensure_toast()
	if _toast == null:
		return
	_toast.text = text
	_toast.add_theme_color_override("font_color", color)
	_toast.modulate.a = 1.0
	_toast.visible = true
	_toast.position = get_viewport().get_mouse_position() + Vector2(12.0, 12.0)

	if _toast_tween != null and _toast_tween.is_valid():
		_toast_tween.kill()
	_toast_tween = create_tween()
	_toast_tween.tween_interval(1.2)
	_toast_tween.tween_property(_toast, "modulate:a", 0.0, 0.4)
	_toast_tween.tween_callback(func() -> void:
		if is_instance_valid(_toast):
			_toast.visible = false
	)


func _ensure_toast() -> void:
	if _toast != null and is_instance_valid(_toast):
		return
	var parent: Node = get_parent()
	if parent == null:
		return
	_toast = Label.new()
	_toast.name = "MenuToast"
	_toast.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_toast.add_theme_color_override("font_color", INK_COLOR)
	_toast.visible = false
	parent.add_child(_toast)
