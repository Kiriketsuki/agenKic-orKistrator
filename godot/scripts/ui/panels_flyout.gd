class_name PanelsFlyout
extends Control
## PanelsFlyout — Panels (F5) flyout for the "panels" orb. Registers itself
## under OrbFlock the same way GrimoireFlyout and PowerFlyout do (see
## orb_flock.gd's register_flyout / main.tscn).
##
## Four regions:
##   Agent list  — live rows from BridgeManager: name, provider accent color,
##                 state. Tapping a row opens or focuses that agent's spell
##                 scroll through PanelManager.
##   Toggles     — QUEST BOARD and MINIMAP, mirroring the existing hotkey
##                 actions ("toggle_quest_board", "toggle_minimap").
##   Presets     — GRID and FOCUS built-ins, plus any named custom save.
##   Save        — a name field and SAVE LAYOUT button that captures the
##                 current arrangement under that name.
##
## Reads BridgeManager directly for the agent list (per the spec's technical
## plan) rather than through PanelManager, so the row list stays live even
## while PanelManager is mid-restore.

const CUSTOM_PRESET_LABEL_PREFIX: String = "★ "

var _bridge: Node = null
var _panel_manager: Node = null
var _minimap: Node = null

var _agent_list: ItemList = null
var _preset_list: ItemList = null
var _preset_name_field: LineEdit = null


func _ready() -> void:
	custom_minimum_size = Vector2(300.0, 420.0)
	size = Vector2(300.0, 420.0)

	_bridge = get_node_or_null("/root/BridgeManager")
	_panel_manager = get_tree().get_first_node_in_group("panel_manager")
	_minimap = get_tree().get_first_node_in_group("minimap")

	var box := VBoxContainer.new()
	box.set_anchors_preset(Control.PRESET_FULL_RECT)
	box.add_theme_constant_override("separation", 8)
	add_child(box)

	var agents_label := Label.new()
	agents_label.text = "AGENTS"
	box.add_child(agents_label)

	_agent_list = ItemList.new()
	_agent_list.custom_minimum_size = Vector2(0.0, 140.0)
	_agent_list.item_selected.connect(_on_agent_row_selected)
	box.add_child(_agent_list)

	var quest_board_button := Button.new()
	quest_board_button.text = "QUEST BOARD"
	quest_board_button.pressed.connect(_on_quest_board_pressed)
	box.add_child(quest_board_button)

	var minimap_button := Button.new()
	minimap_button.text = "MINIMAP"
	minimap_button.pressed.connect(_on_minimap_pressed)
	box.add_child(minimap_button)

	var presets_label := Label.new()
	presets_label.text = "LAYOUTS"
	box.add_child(presets_label)

	_preset_list = ItemList.new()
	_preset_list.custom_minimum_size = Vector2(0.0, 100.0)
	_preset_list.item_selected.connect(_on_preset_row_selected)
	box.add_child(_preset_list)

	var save_row := HBoxContainer.new()
	box.add_child(save_row)
	_preset_name_field = LineEdit.new()
	_preset_name_field.placeholder_text = "preset name"
	_preset_name_field.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	save_row.add_child(_preset_name_field)
	var save_button := Button.new()
	save_button.text = "SAVE LAYOUT"
	save_button.pressed.connect(_on_save_layout_pressed)
	save_row.add_child(save_button)

	if _bridge != null:
		if _bridge.has_signal("agent_registered"):
			_bridge.connect("agent_registered", _on_agent_registered)
		if _bridge.has_signal("agent_deregistered"):
			_bridge.connect("agent_deregistered", _on_agent_deregistered)
		if _bridge.has_signal("agent_state_changed"):
			_bridge.connect("agent_state_changed", _on_agent_state_changed)

	visibility_changed.connect(_on_visibility_changed)
	_rebuild_agent_list()
	_rebuild_preset_list()


func _on_visibility_changed() -> void:
	if visible:
		_rebuild_agent_list()
		_rebuild_preset_list()


## Row text is "name · provider — state"; the accent color (ProviderPalette,
## same source floating_rune.gd and particle tint already read) rides as the
## row's foreground color instead of an inline swatch, so no extra Control
## per row is needed. Sorted by id for a stable listing across rebuilds.
func _rebuild_agent_list() -> void:
	if _agent_list == null:
		return
	_agent_list.clear()
	if _bridge == null or not _bridge.has_method("get_registered_agents"):
		return
	var agents: Array = _bridge.get_registered_agents()
	var rows: Array = build_agent_rows(agents)
	for row: Dictionary in rows:
		var index: int = _agent_list.add_item(row.get("label", ""))
		_agent_list.set_item_metadata(index, row.get("agent_id", ""))
		_agent_list.set_item_custom_fg_color(index, row.get("accent", Color.WHITE))


## Pure extraction step: turns a snapshot Array of BridgeData.AgentData into
## the ordered row list {agent_id, label, accent}. Split out from
## _rebuild_agent_list so the row-building logic is testable headlessly
## without a live BridgeManager or ItemList, mirroring power_flyout.gd's
## banish_all_ids.
static func build_agent_rows(agents: Array) -> Array[Dictionary]:
	var typed: Array[BridgeData.AgentData] = []
	for agent: Variant in agents:
		if agent is BridgeData.AgentData:
			typed.append(agent as BridgeData.AgentData)
	typed.sort_custom(func(a: BridgeData.AgentData, b: BridgeData.AgentData) -> bool: return a.id < b.id)
	var rows: Array[Dictionary] = []
	for agent: BridgeData.AgentData in typed:
		var label: String = "%s — %s" % [agent.display_name(), agent.state]
		if not agent.provider.is_empty():
			label = "%s · %s — %s" % [agent.display_name(), agent.provider, agent.state]
		rows.append({
			"agent_id": agent.id,
			"label": label,
			"accent": ProviderPalette.get_accent_color(agent.provider),
		})
	return rows


func _on_agent_row_selected(index: int) -> void:
	var agent_id: String = String(_agent_list.get_item_metadata(index))
	if agent_id.is_empty() or _panel_manager == null:
		return
	if _panel_manager.has_method("open_scroll_panel"):
		_panel_manager.call("open_scroll_panel", agent_id)


func _on_agent_registered(_agent_data: BridgeData.AgentData) -> void:
	_rebuild_agent_list()


func _on_agent_deregistered(_agent_id: String) -> void:
	_rebuild_agent_list()


func _on_agent_state_changed(_agent_id: String, _old_state: String, _new_state: String, _task_id: String) -> void:
	_rebuild_agent_list()


## Mirrors PanelManager's "toggle_quest_board" hotkey path exactly — same
## public method the hotkey handler calls internally.
func _on_quest_board_pressed() -> void:
	if _panel_manager != null and _panel_manager.has_method("toggle_quest_board"):
		_panel_manager.call("toggle_quest_board")


## Mirrors minimap.gd's own "toggle_minimap" hotkey handler (visible = not
## visible) rather than duplicating a second toggle path.
func _on_minimap_pressed() -> void:
	if _minimap != null and _minimap is CanvasItem:
		(_minimap as CanvasItem).visible = not (_minimap as CanvasItem).visible


func _rebuild_preset_list() -> void:
	if _preset_list == null:
		return
	_preset_list.clear()
	_preset_list.add_item(PanelManager.GRID_PRESET_NAME)
	_preset_list.add_item(PanelManager.FOCUS_PRESET_NAME)
	if _panel_manager != null and _panel_manager.has_method("custom_preset_names"):
		for preset_name: String in _panel_manager.call("custom_preset_names"):
			_preset_list.add_item(CUSTOM_PRESET_LABEL_PREFIX + preset_name)


func _on_preset_row_selected(index: int) -> void:
	if _panel_manager == null or not _panel_manager.has_method("apply_named_preset"):
		return
	var label: String = _preset_list.get_item_text(index)
	var preset_name: String = label.trim_prefix(CUSTOM_PRESET_LABEL_PREFIX)
	_panel_manager.call("apply_named_preset", preset_name)


func _on_save_layout_pressed() -> void:
	var preset_name: String = _preset_name_field.text.strip_edges()
	if preset_name.is_empty() or _panel_manager == null:
		return
	if _panel_manager.has_method("save_named_preset"):
		_panel_manager.call("save_named_preset", preset_name)
	_preset_name_field.text = ""
	_rebuild_preset_list()
