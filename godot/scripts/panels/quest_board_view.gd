extends Control
## QuestBoardView — "commission a quest" task-submission UI (#118), mounted
## into a PanelBase's ContentRoot when the panel is in "quest" mode. Unlike
## the spell-scroll/terminal views, the quest board is not agent-scoped —
## `agent_data` passed to setup() is always null (see
## PanelManager.open_quest_board()).
##
## Two sub-forms, toggled by a Single/Chain segmented pair:
##   SINGLE — one task: description, target floor, project, priority, and an
##            optional quest id (blank = server-generated).
##   CHAIN  — a small DAG: N task rows (auto node_id n1..nN) plus from/to
##            edges between them.
##
## Priority mapping (IMPORTANT): the orchestrator's task queue is a min-heap
## (DequeueTask == ZPOPMIN — LOWEST score dequeues FIRST). So "High" priority
## maps to a LOW number and "Low" priority maps to a HIGH number. Getting this
## backwards would make high-priority quests run *last*.

class_name QuestBoardView

const PRIORITY_HIGH: float = 1.0
const PRIORITY_NORMAL: float = 5.0
const PRIORITY_LOW: float = 10.0

const INK_COLOR: Color = Color(0.28, 0.18, 0.08, 1.0)
const ERROR_COLOR: Color = Color(0.55, 0.12, 0.1, 1.0)
const SUCCESS_COLOR: Color = Color(0.16, 0.4, 0.16, 1.0)
const FIELD_BG: Color = Color(0.88, 0.79, 0.62, 1.0)
const FIELD_BORDER: Color = Color(0.45, 0.32, 0.16, 1.0)

const PATH_SUBMIT_TASK: String = "/api/tasks"
const PATH_SUBMIT_DAG: String = "/api/dags"

@onready var _parchment: ColorRect = $Parchment
@onready var _single_mode_button: Button = $Margin/Root/ModeToggle/SingleModeButton
@onready var _chain_mode_button: Button = $Margin/Root/ModeToggle/ChainModeButton
@onready var _status_label: Label = $Margin/Root/StatusLabel
@onready var _single_form: VBoxContainer = $Margin/Root/SingleForm
@onready var _chain_form: VBoxContainer = $Margin/Root/ChainForm

@onready var _description_edit: TextEdit = $Margin/Root/SingleForm/DescriptionEdit
@onready var _floor_option: OptionButton = $Margin/Root/SingleForm/RowFloorProject/FloorOption
@onready var _project_edit: LineEdit = $Margin/Root/SingleForm/RowFloorProject/ProjectEdit
@onready var _priority_option: OptionButton = $Margin/Root/SingleForm/RowPriorityId/PriorityOption
@onready var _quest_id_edit: LineEdit = $Margin/Root/SingleForm/RowPriorityId/IdEdit
@onready var _single_seal: WaxSeal = $Margin/Root/SingleForm/SingleSubmitRow/SingleSeal

@onready var _task_rows: VBoxContainer = $Margin/Root/ChainForm/TaskRows
@onready var _add_task_button: Button = $Margin/Root/ChainForm/AddTaskButton
@onready var _edge_from_option: OptionButton = $Margin/Root/ChainForm/EdgeRow/EdgeFromOption
@onready var _edge_to_option: OptionButton = $Margin/Root/ChainForm/EdgeRow/EdgeToOption
@onready var _link_button: Button = $Margin/Root/ChainForm/EdgeRow/LinkButton
@onready var _edge_list: VBoxContainer = $Margin/Root/ChainForm/EdgeList
@onready var _chain_seal: WaxSeal = $Margin/Root/ChainForm/ChainSubmitRow/ChainSeal

var _panel: PanelBase = null
var _bridge: Node = null
var _tower: Node = null
var _signals_connected: bool = false

## Chain-mode state. Each task row entry: {node_id, hbox, desc_edit, priority_option}.
var _chain_rows: Array[Dictionary] = []
## Each edge entry: {from, to, hbox}.
var _chain_edges: Array[Dictionary] = []
var _next_node_num: int = 1

var _submitting: bool = false
## Tracked via BridgeManager.connection_status_changed; optimistic default
## avoids a false "offline" flash before the first signal fires. Purely
## cosmetic — the command queue auto-flushes on reconnect regardless
## (BridgeManager._enqueue_command), so this only affects the status label.
var _connection_status: String = "connected"


func _ready() -> void:
	_configure_parchment_material()
	_wire_mode_toggle()
	_populate_priority_option(_priority_option)
	_style_line_edit(_project_edit)
	_style_line_edit(_quest_id_edit)
	_style_text_edit(_description_edit)
	_add_task_button.pressed.connect(_on_add_task_pressed)
	_link_button.pressed.connect(_on_link_pressed)
	_single_seal.pressed.connect(_on_single_submit_pressed)
	_chain_seal.pressed.connect(_on_chain_submit_pressed)
	_set_mode("single")
	_clear_status()
	# Chain mode always starts with one task row so "Link" has something to
	# work with immediately.
	_add_task_row()


# ---------------------------------------------------------------------------
# Public API (PanelContentRouter convention)
# ---------------------------------------------------------------------------

## Called once by PanelContentRouter right after this view is added to the
## tree. `agent_data` is always null for the quest board — it is not an
## agent-scoped panel.
func setup(panel: PanelBase, _agent_data: BridgeData.AgentData, bridge: Node) -> void:
	_panel = panel
	_bridge = bridge
	_tower = _find_tower()
	_restyle_close_button()
	_disable_mode_toggle()
	_connect_bridge_signals()
	_connect_tower_signals()
	_refresh_floor_options()


# ---------------------------------------------------------------------------
# Setup helpers
# ---------------------------------------------------------------------------

func _configure_parchment_material() -> void:
	if _parchment == null:
		return
	var material: ShaderMaterial = _parchment.material as ShaderMaterial
	if material == null:
		return
	if material.get_shader_parameter("fibre_noise") != null:
		return
	var noise: FastNoiseLite = FastNoiseLite.new()
	noise.noise_type = FastNoiseLite.TYPE_PERLIN
	noise.frequency = 0.045
	noise.fractal_octaves = 3
	var noise_texture: NoiseTexture2D = NoiseTexture2D.new()
	noise_texture.width = 256
	noise_texture.height = 256
	noise_texture.seamless = true
	noise_texture.noise = noise
	material.set_shader_parameter("fibre_noise", noise_texture)


## The quest board is a dedicated panel type, not a generic mode among
## others (mirrors SpellScrollView._disable_mode_toggle()) — toggling the
## shared T8 title bar's Mode button here would persist a stray
## mode_preferences entry. In practice PanelManager.open_quest_board() uses
## an empty agent_id, so _save_layout()'s `if not agent_id.is_empty()` guard
## already prevents that persistence — this is defense in depth.
func _disable_mode_toggle() -> void:
	if _panel == null:
		return
	var button: Button = _panel.get_mode_button()
	if button == null:
		return
	button.visible = false
	button.disabled = true


func _restyle_close_button() -> void:
	if _panel == null:
		return
	var button: Button = _panel.get_close_button()
	if button == null:
		return
	button.tooltip_text = "Close the quest board"


func _wire_mode_toggle() -> void:
	var group: ButtonGroup = ButtonGroup.new()
	_single_mode_button.toggle_mode = true
	_chain_mode_button.toggle_mode = true
	_single_mode_button.button_group = group
	_chain_mode_button.button_group = group
	_single_mode_button.button_pressed = true
	_single_mode_button.pressed.connect(func() -> void: _set_mode("single"))
	_chain_mode_button.pressed.connect(func() -> void: _set_mode("chain"))


func _set_mode(mode: String) -> void:
	_single_form.visible = mode == "single"
	_chain_form.visible = mode == "chain"


func _populate_priority_option(option: OptionButton) -> void:
	option.clear()
	# Item text communicates the intent; item id/metadata carries the actual
	# ZPOPMIN score (see class doc — High -> LOW score, dequeued first).
	option.add_item("High", 0)
	option.set_item_metadata(0, PRIORITY_HIGH)
	option.add_item("Normal", 1)
	option.set_item_metadata(1, PRIORITY_NORMAL)
	option.add_item("Low", 2)
	option.set_item_metadata(2, PRIORITY_LOW)
	option.select(1) # Normal by default.


func _selected_priority(option: OptionButton) -> float:
	var index: int = option.selected
	if index < 0:
		return PRIORITY_NORMAL
	var meta: Variant = option.get_item_metadata(index)
	if meta is float or meta is int:
		return float(meta)
	return PRIORITY_NORMAL


# ---------------------------------------------------------------------------
# TowerManager access (floor list for the target-floor picker)
# ---------------------------------------------------------------------------

## The quest board is mounted deep inside PanelManager's floating layer, far
## from TowerManager's actual scene position (a sibling of PanelManager's
## ancestor UILayer) — so a short relative path like panel_manager.gd's
## "../../Tower" does not reach it from here. get_tree().current_scene is
## the main scene root ("Main") regardless of nesting depth, and is stable
## across the panel's own reparenting (docking/undocking/fullscreen).
func _find_tower() -> Node:
	var scene: Node = get_tree().current_scene
	if scene == null:
		return null
	return scene.get_node_or_null("Tower")


func _connect_tower_signals() -> void:
	if _tower != null and _tower.has_signal("floors_changed"):
		if not _tower.floors_changed.is_connected(_refresh_floor_options):
			_tower.floors_changed.connect(_refresh_floor_options)


## Repopulates the target-floor OptionButton from TowerManager.get_floor_infos().
## Always includes a leading "Any" entry (empty-string floor, meaning "let the
## orchestrator pick"). Safe to call with no TowerManager or an empty floor
## list — the picker then only shows "Any".
func _refresh_floor_options() -> void:
	if _floor_option == null:
		return
	var previous_floor: String = _selected_floor_name()
	_floor_option.clear()
	_floor_option.add_item("Any", 0)
	_floor_option.set_item_metadata(0, "")
	if _tower != null and _tower.has_method("get_floor_infos"):
		var infos: Array = _tower.get_floor_infos()
		var idx: int = 1
		for info: Variant in infos:
			if not info is Dictionary:
				continue
			var dict: Dictionary = info as Dictionary
			var floor_name: String = String(dict.get("name", ""))
			var label: String = String(dict.get("label", floor_name))
			_floor_option.add_item(label, idx)
			_floor_option.set_item_metadata(idx, floor_name)
			idx += 1
	_reselect_floor(previous_floor)


func _selected_floor_name() -> String:
	if _floor_option == null or _floor_option.item_count == 0:
		return ""
	var index: int = _floor_option.selected
	if index < 0:
		return ""
	var meta: Variant = _floor_option.get_item_metadata(index)
	return String(meta) if meta != null else ""


func _reselect_floor(floor_name: String) -> void:
	for i: int in range(_floor_option.item_count):
		var meta: Variant = _floor_option.get_item_metadata(i)
		if String(meta) == floor_name:
			_floor_option.select(i)
			return
	_floor_option.select(0)


# ---------------------------------------------------------------------------
# Bridge signals (feedback)
# ---------------------------------------------------------------------------

func _connect_bridge_signals() -> void:
	if _signals_connected or _bridge == null:
		return
	if _bridge.has_signal("command_succeeded"):
		_bridge.connect("command_succeeded", _on_command_succeeded)
	if _bridge.has_signal("command_failed"):
		_bridge.connect("command_failed", _on_command_failed)
	if _bridge.has_signal("connection_status_changed"):
		_bridge.connect("connection_status_changed", _on_connection_status_changed)
	_signals_connected = true


func _on_connection_status_changed(status: String) -> void:
	_connection_status = status


func _on_command_succeeded(path: String, _code: int, response_body: String) -> void:
	if path != PATH_SUBMIT_TASK and path != PATH_SUBMIT_DAG:
		return
	if not _submitting:
		return
	_submitting = false
	_set_seals_disabled(false)
	var quest_id: String = _extract_response_id(response_body)
	_show_status("Quest accepted — %s" % quest_id if quest_id != "" else "Quest accepted", SUCCESS_COLOR)
	if path == PATH_SUBMIT_TASK:
		_clear_single_form()
	else:
		_clear_chain_form()


func _on_command_failed(path: String, _code: int, reason: String) -> void:
	if path != PATH_SUBMIT_TASK and path != PATH_SUBMIT_DAG:
		return
	if not _submitting:
		return
	_submitting = false
	_set_seals_disabled(false)
	_show_status("Quest rejected — %s" % reason, ERROR_COLOR)


func _extract_response_id(response_body: String) -> String:
	if response_body == "":
		return ""
	var parsed: Variant = JSON.parse_string(response_body)
	if not parsed is Dictionary:
		return ""
	var dict: Dictionary = parsed as Dictionary
	if dict.has("task_id"):
		return String(dict["task_id"])
	if dict.has("dag_execution_id"):
		return String(dict["dag_execution_id"])
	return ""


func _show_status(text: String, color: Color) -> void:
	_status_label.text = text
	_status_label.add_theme_color_override("font_color", color)


func _clear_status() -> void:
	_status_label.text = ""
	_status_label.add_theme_color_override("font_color", INK_COLOR)


func _set_seals_disabled(value: bool) -> void:
	_single_seal.disabled = value
	_chain_seal.disabled = value
	_single_seal.queue_redraw()
	_chain_seal.queue_redraw()


# ---------------------------------------------------------------------------
# SINGLE submit
# ---------------------------------------------------------------------------

func _on_single_submit_pressed() -> void:
	if _submitting:
		return
	var description: String = _description_edit.text.strip_edges()
	if description == "":
		_show_status("A quest needs a description.", ERROR_COLOR)
		return
	if _bridge == null or not _bridge.has_method("submit_task"):
		_show_status("Quest rejected — orchestrator bridge unavailable", ERROR_COLOR)
		return
	var floor_name: String = _selected_floor_name()
	var project: String = _project_edit.text.strip_edges()
	var priority: float = _selected_priority(_priority_option)
	var quest_id: String = _quest_id_edit.text.strip_edges()
	if _connection_status != "connected":
		# BridgeManager._enqueue_command only sends the request (and thus only
		# ever fires command_succeeded/command_failed) once _connection_state
		# is CONNECTED — while offline it just appends to a queue that is
		# flushed on reconnect. Locking _submitting/the seals here would leave
		# the quest board frozen indefinitely if reconnection never completes
		# in this session. So: queue the command but don't wait on an ack.
		_show_status("Quest queued — orchestrator offline, will submit on reconnect", INK_COLOR)
		_bridge.call("submit_task", description, priority, floor_name, project, quest_id)
		return
	_submitting = true
	_set_seals_disabled(true)
	_show_status("Sealing the quest...", INK_COLOR)
	_bridge.call("submit_task", description, priority, floor_name, project, quest_id)


func _clear_single_form() -> void:
	_description_edit.text = ""
	_project_edit.text = ""
	_quest_id_edit.text = ""
	_priority_option.select(1)
	_reselect_floor("")


# ---------------------------------------------------------------------------
# CHAIN — task rows
# ---------------------------------------------------------------------------

func _on_add_task_pressed() -> void:
	_add_task_row()


func _add_task_row() -> void:
	var node_id: String = "n%d" % _next_node_num
	_next_node_num += 1

	var hbox: HBoxContainer = HBoxContainer.new()
	hbox.add_theme_constant_override("separation", 6)

	var id_label: Label = Label.new()
	id_label.text = node_id
	id_label.custom_minimum_size = Vector2(32, 0)
	id_label.add_theme_color_override("font_color", INK_COLOR)
	hbox.add_child(id_label)

	var desc_edit: LineEdit = LineEdit.new()
	desc_edit.placeholder_text = "Task description"
	desc_edit.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_style_line_edit(desc_edit)
	hbox.add_child(desc_edit)

	var priority_option: OptionButton = OptionButton.new()
	_populate_priority_option(priority_option)
	hbox.add_child(priority_option)

	var remove_button: Button = Button.new()
	remove_button.text = "✕"
	remove_button.tooltip_text = "Remove this task"
	hbox.add_child(remove_button)

	var row: Dictionary = {
		"node_id": node_id,
		"hbox": hbox,
		"desc_edit": desc_edit,
		"priority_option": priority_option,
	}
	remove_button.pressed.connect(func() -> void: _remove_task_row(row))

	_task_rows.add_child(hbox)
	_chain_rows.append(row)
	_refresh_edge_options()


func _remove_task_row(row: Dictionary) -> void:
	var node_id: String = row.get("node_id", "")
	_chain_edges = _chain_edges.filter(func(e: Dictionary) -> bool:
		return e.get("from", "") != node_id and e.get("to", "") != node_id
	)
	_rebuild_edge_list_ui()
	var hbox: Node = row.get("hbox", null)
	if hbox != null and is_instance_valid(hbox):
		hbox.queue_free()
	_chain_rows = _chain_rows.filter(func(r: Dictionary) -> bool: return r.get("node_id", "") != node_id)
	_refresh_edge_options()


func _refresh_edge_options() -> void:
	var node_ids: Array[String] = []
	for row: Dictionary in _chain_rows:
		node_ids.append(String(row.get("node_id", "")))
	_fill_node_option(_edge_from_option, node_ids)
	_fill_node_option(_edge_to_option, node_ids)


func _fill_node_option(option: OptionButton, node_ids: Array[String]) -> void:
	var previous: String = ""
	if option.selected >= 0 and option.selected < option.item_count:
		previous = option.get_item_text(option.selected)
	option.clear()
	for i: int in range(node_ids.size()):
		option.add_item(node_ids[i], i)
	for i: int in range(option.item_count):
		if option.get_item_text(i) == previous:
			option.select(i)
			return
	if option.item_count > 0:
		option.select(0)


# ---------------------------------------------------------------------------
# CHAIN — edges
# ---------------------------------------------------------------------------

func _on_link_pressed() -> void:
	if _edge_from_option.item_count == 0 or _edge_to_option.item_count == 0:
		return
	var from_id: String = _edge_from_option.get_item_text(_edge_from_option.selected)
	var to_id: String = _edge_to_option.get_item_text(_edge_to_option.selected)
	if from_id == "" or to_id == "":
		return
	if from_id == to_id:
		_show_status("A quest cannot depend on itself.", ERROR_COLOR)
		return
	for edge: Dictionary in _chain_edges:
		if edge.get("from", "") == from_id and edge.get("to", "") == to_id:
			return # Duplicate edge — ignore.
	var edge: Dictionary = {"from": from_id, "to": to_id}
	_add_edge_row(edge)
	_chain_edges.append(edge)


func _add_edge_row(edge: Dictionary) -> void:
	var hbox: HBoxContainer = HBoxContainer.new()
	var label: Label = Label.new()
	label.text = "%s → %s" % [edge.get("from", ""), edge.get("to", "")]
	label.add_theme_color_override("font_color", INK_COLOR)
	label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	hbox.add_child(label)
	var remove_button: Button = Button.new()
	remove_button.text = "✕"
	remove_button.tooltip_text = "Remove this link"
	hbox.add_child(remove_button)
	remove_button.pressed.connect(func() -> void:
		_chain_edges.erase(edge)
		hbox.queue_free()
	)
	_edge_list.add_child(hbox)


func _rebuild_edge_list_ui() -> void:
	for child: Node in _edge_list.get_children():
		child.queue_free()
	for edge: Dictionary in _chain_edges:
		_add_edge_row(edge)


# ---------------------------------------------------------------------------
# CHAIN submit
# ---------------------------------------------------------------------------

func _on_chain_submit_pressed() -> void:
	if _submitting:
		return
	if _chain_rows.is_empty():
		_show_status("Add at least one task with a description.", ERROR_COLOR)
		return
	# Validate BEFORE building the payload. The previous behavior silently
	# dropped any row with a blank description (and any edge touching it),
	# so the DAG actually POSTed could silently diverge from the one the
	# user built and confirmed on screen (#118 council finding). Block
	# submission instead and name the offending rows.
	var missing_ids: PackedStringArray = []
	for row: Dictionary in _chain_rows:
		var desc_edit: LineEdit = row.get("desc_edit", null)
		var description: String = desc_edit.text.strip_edges() if desc_edit != null else ""
		if description == "":
			missing_ids.append(String(row.get("node_id", "")))
	if not missing_ids.is_empty():
		_show_status("Give %s a description before sealing the quest chain." % ", ".join(missing_ids), ERROR_COLOR)
		return
	var nodes: Array = []
	var seen_ids: Dictionary = {}
	for row: Dictionary in _chain_rows:
		var desc_edit: LineEdit = row.get("desc_edit", null)
		var description: String = desc_edit.text.strip_edges() if desc_edit != null else ""
		var node_id: String = String(row.get("node_id", ""))
		var priority_option: OptionButton = row.get("priority_option", null)
		var priority: float = _selected_priority(priority_option) if priority_option != null else PRIORITY_NORMAL
		nodes.append({
			"node_id": node_id,
			"task_id": "",
			"priority": priority,
			"description": description,
		})
		seen_ids[node_id] = true
	var edges: Array = []
	for edge: Dictionary in _chain_edges:
		var from_id: String = String(edge.get("from", ""))
		var to_id: String = String(edge.get("to", ""))
		if from_id == to_id:
			continue
		if not seen_ids.has(from_id) or not seen_ids.has(to_id):
			continue # Defense in depth — _remove_task_row already prunes edges.
		edges.append({"from": from_id, "to": to_id})
	if _bridge == null or not _bridge.has_method("submit_dag"):
		_show_status("Quest rejected — orchestrator bridge unavailable", ERROR_COLOR)
		return
	if _connection_status != "connected":
		# See _on_single_submit_pressed for why offline submits must not lock
		# _submitting/the seals — no ack ever arrives until reconnect.
		_show_status("Quest chain queued — orchestrator offline, will submit on reconnect", INK_COLOR)
		_bridge.call("submit_dag", nodes, edges)
		return
	_submitting = true
	_set_seals_disabled(true)
	_show_status("Sealing the quest chain...", INK_COLOR)
	_bridge.call("submit_dag", nodes, edges)


func _clear_chain_form() -> void:
	for row: Dictionary in _chain_rows.duplicate():
		var hbox: Node = row.get("hbox", null)
		if hbox != null and is_instance_valid(hbox):
			hbox.queue_free()
	_chain_rows.clear()
	for child: Node in _edge_list.get_children():
		child.queue_free()
	_chain_edges.clear()
	_next_node_num = 1
	_add_task_row()


# ---------------------------------------------------------------------------
# Styling
# ---------------------------------------------------------------------------

func _style_line_edit(edit: LineEdit) -> void:
	var style: StyleBoxFlat = StyleBoxFlat.new()
	style.bg_color = FIELD_BG
	style.border_color = FIELD_BORDER
	style.set_border_width_all(1)
	style.set_corner_radius_all(3)
	style.content_margin_left = 6.0
	style.content_margin_right = 6.0
	style.content_margin_top = 3.0
	style.content_margin_bottom = 3.0
	edit.add_theme_stylebox_override("normal", style)
	edit.add_theme_stylebox_override("focus", style)
	edit.add_theme_color_override("font_color", INK_COLOR)


func _style_text_edit(edit: TextEdit) -> void:
	var style: StyleBoxFlat = StyleBoxFlat.new()
	style.bg_color = FIELD_BG
	style.border_color = FIELD_BORDER
	style.set_border_width_all(1)
	style.set_corner_radius_all(3)
	style.content_margin_left = 6.0
	style.content_margin_right = 6.0
	style.content_margin_top = 4.0
	style.content_margin_bottom = 4.0
	edit.add_theme_stylebox_override("normal", style)
	edit.add_theme_stylebox_override("focus", style)
	edit.add_theme_color_override("font_color", INK_COLOR)
