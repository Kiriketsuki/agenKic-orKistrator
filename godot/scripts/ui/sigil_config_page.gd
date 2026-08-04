class_name SigilConfigPage
extends Control
## SigilConfigPage — Grimoire Summoning (F3) sigil config: provider toggles,
## order, and per-provider defaults (tier, name pool). Reachable from the
## Grimoire flyout and the title screen's GRIMOIRE entry (see
## title_screen.gd/grimoire_flyout.gd), so this builds its own rows in code
## rather than living in a shared .tscn, and both callers can drop it inside
## whatever container they already have.
##
## This is a true modal: the root Control is a full-rect dim backdrop
## (ColorRect, mouse_filter STOP) that blocks input and hides whatever sits
## beneath it, and a centered PanelContainer on top carries the actual rows.
## A caller that wants the modal above every HUD layer reparents this node
## to a high CanvasLayer (see grimoire_flyout.gd's open_config_page) before
## it calls open_page. Escape or the BACK button both close it the same way.
##
## Persists to user://orki_settings.cfg via SigilConfig. Every edit saves
## immediately — there is no separate "Apply" step, so a keeper who closes
## the page without an explicit save action never loses a change.

## Emitted when the keeper dismisses the page, either through BACK or
## Escape. The caller hides/reparents the page and restores whatever the
## modal covered.
signal closed()

## True while any SigilConfigPage is showing. OrbFlock reads this in its
## _input the same way it reads GrimoireFlyout.is_placing: while the modal
## is up, the flock must not treat clicks on it as outside-clicks and
## collapse (round 6 keeper feedback, "BACK does not return to the grid").
static var is_open: bool = false

const TIERS: Array[String] = ["", "haiku", "sonnet", "opus", "adept", "novice"]

## Row grid separation, doubled from the pre-2x base (10px, 8px) to match
## the doubled project.godot viewport (see defect 3, epic 169).
const GRID_H_SEPARATION: int = 20
const GRID_V_SEPARATION: int = 24
const BACKDROP_COLOR: Color = Color(0.02, 0.02, 0.05, 0.78)
const PANEL_MIN_SIZE: Vector2 = Vector2(1200.0, 760.0)
const TITLE_FONT_SIZE: int = 44
const ROW_FONT_SIZE: int = 26
## Solid parchment-dark panel behind the rows. The old unstyled
## PanelContainer let the hall show through and the rows were hard to read
## (round 6 keeper feedback, "overhaul it").
const PANEL_BG_COLOR: Color = Color(0.075, 0.082, 0.125, 0.985)
const PANEL_BORDER_COLOR: Color = Color(0.788, 0.635, 0.153, 1.0)
const PANEL_BORDER_WIDTH: int = 3
const PANEL_CONTENT_MARGIN: int = 40
const ROW_BG_COLOR: Color = Color(1.0, 1.0, 1.0, 0.04)

## Folder-picker chrome (see _build_dialog_theme). The picker is the one
## in-engine Window this page opens, so it carries its own scale constants
## rather than reusing the panel's row sizes.
const DIALOG_FONT_SIZE: int = 24
const DIALOG_TITLE_HEIGHT: int = 44
const DIALOG_CONTENT_MARGIN: int = 20
const DIALOG_FIELD_COLOR: Color = Color(0.043, 0.047, 0.075, 1.0)
const DIALOG_TEXT_COLOR: Color = Color(0.855, 0.867, 0.914, 1.0)
const DIALOG_BUTTON_COLOR: Color = Color(0.118, 0.129, 0.188, 1.0)
const DIALOG_BUTTON_HOVER_COLOR: Color = Color(0.176, 0.192, 0.263, 1.0)
const DIALOG_BUTTON_PRESSED_COLOR: Color = Color(0.235, 0.196, 0.086, 1.0)

## True for the Grimoire flyout's F3 page (see grimoire_flyout.gd), where
## this node owns its own dim backdrop and centered panel. The title
## screen's GRIMOIRE entry (title_screen.gd) already wraps this page in its
## own bordered GrimoirePanel, so it sets this false before the node enters
## the tree, and this page then just fills whatever box that panel gives
## it instead of drawing a second backdrop on top of the first.
@export var standalone_modal: bool = true

var _config: SigilConfig
var _providers: Array = []
var _rows_grid: GridContainer
var _panel: PanelContainer
var _workdir_edit: LineEdit
var _folder_dialog: FileDialog
var _open_panels_spin: SpinBox
var _panel_width_spin: SpinBox
var _panel_height_spin: SpinBox


func _ready() -> void:
	set_process_unhandled_input(true)
	# Static var, reset on scene reload for the same reason as
	# OrbFlock.suspend_hotkeys (see that _ready). Only one config page shows
	# at a time, so tracking visibility of this instance is safe.
	is_open = visible
	visibility_changed.connect(func() -> void: is_open = visible)

	var content := VBoxContainer.new()
	content.add_theme_constant_override("separation", GRID_V_SEPARATION)

	if standalone_modal:
		set_anchors_preset(Control.PRESET_FULL_RECT)
		mouse_filter = Control.MOUSE_FILTER_STOP

		var backdrop := ColorRect.new()
		backdrop.set_anchors_preset(Control.PRESET_FULL_RECT)
		backdrop.color = BACKDROP_COLOR
		backdrop.mouse_filter = Control.MOUSE_FILTER_STOP
		add_child(backdrop)

		# A full-rect CenterContainer keeps the panel centered at ANY content
		# size. Center anchors with fixed offsets drift once the content grows
		# past PANEL_MIN_SIZE, which the doubled fonts always do.
		var center := CenterContainer.new()
		center.set_anchors_preset(Control.PRESET_FULL_RECT)
		center.mouse_filter = Control.MOUSE_FILTER_IGNORE
		add_child(center)

		_panel = PanelContainer.new()
		_panel.custom_minimum_size = PANEL_MIN_SIZE
		_panel.mouse_filter = Control.MOUSE_FILTER_STOP
		_panel.add_theme_stylebox_override("panel", _build_panel_style())
		center.add_child(_panel)
		_panel.add_child(content)
	else:
		size_flags_horizontal = Control.SIZE_EXPAND_FILL
		size_flags_vertical = Control.SIZE_EXPAND_FILL
		content.set_anchors_preset(Control.PRESET_FULL_RECT)
		add_child(content)

	var title := Label.new()
	title.text = "SIGIL CONFIG"
	title.add_theme_font_size_override("font_size", TITLE_FONT_SIZE)
	title.add_theme_color_override("font_color", Color(0.788, 0.635, 0.153, 1.0))
	content.add_child(title)

	content.add_child(_build_workdir_row())
	content.add_child(_build_open_panels_row())
	content.add_child(_build_panel_size_row())

	_rows_grid = GridContainer.new()
	_rows_grid.columns = 6
	_rows_grid.add_theme_constant_override("h_separation", GRID_H_SEPARATION)
	_rows_grid.add_theme_constant_override("v_separation", GRID_V_SEPARATION)
	content.add_child(_rows_grid)

	var close_button := Button.new()
	close_button.text = "BACK"
	close_button.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	close_button.pressed.connect(func() -> void: closed.emit())
	content.add_child(close_button)


## Solid, bordered stylebox for the modal panel: opaque dark fill, gold
## border, rounded corners, generous content margins. Built in code because
## this page builds all of its Controls in code (see the class doc-comment).
func _build_panel_style() -> StyleBoxFlat:
	var style := StyleBoxFlat.new()
	style.bg_color = PANEL_BG_COLOR
	style.border_color = PANEL_BORDER_COLOR
	style.set_border_width_all(PANEL_BORDER_WIDTH)
	style.set_corner_radius_all(8)
	style.set_content_margin_all(PANEL_CONTENT_MARGIN)
	return style


## Project-folder row: label, editable path field, and a BROWSE button that
## opens a native folder picker. The chosen folder is where every summoned
## agent starts (spawn workdir); empty keeps the server's scratch default.
func _build_workdir_row() -> HBoxContainer:
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", GRID_H_SEPARATION)

	var label := Label.new()
	label.text = "PROJECT FOLDER"
	label.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	row.add_child(label)

	_workdir_edit = LineEdit.new()
	_workdir_edit.placeholder_text = "(server default — temp scratch dir)"
	_workdir_edit.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	_workdir_edit.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_workdir_edit.text_submitted.connect(_set_workdir)
	# Save on focus loss too, so a typed path is not lost when the keeper
	# clicks BACK instead of pressing Enter.
	_workdir_edit.focus_exited.connect(func() -> void: _set_workdir(_workdir_edit.text))
	row.add_child(_workdir_edit)

	var browse := Button.new()
	browse.text = "BROWSE..."
	browse.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	browse.pressed.connect(_open_folder_dialog)
	row.add_child(browse)

	_folder_dialog = FileDialog.new()
	_folder_dialog.title = "CHOOSE PROJECT FOLDER"
	_folder_dialog.file_mode = FileDialog.FILE_MODE_OPEN_DIR
	_folder_dialog.access = FileDialog.ACCESS_FILESYSTEM
	# In-engine dialog, not the OS one: a native (portal/GTK) picker cannot
	# be themed at all, and a stock grey system dialog in the middle of the
	# parchment-and-gold tower reads as a different application. The Godot
	# dialog takes the theme built in _build_dialog_theme().
	_folder_dialog.use_native_dialog = false
	_folder_dialog.theme = _build_dialog_theme()
	_folder_dialog.dir_selected.connect(_set_workdir)
	add_child(_folder_dialog)
	return row


## Open-panels-cap row (#169): label plus a SpinBox bounded to
## SigilConfig.MIN_SCROLL_PANELS..MAX_SCROLL_PANELS, letting the keeper choose
## how many agent scroll/terminal panels PanelManager may hold open at once
## before it starts closing the least-recently-focused one. Saves immediately
## on change, matching every other row on this page.
func _build_open_panels_row() -> HBoxContainer:
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", GRID_H_SEPARATION)

	var label := Label.new()
	label.text = "OPEN PANELS"
	label.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	row.add_child(label)

	_open_panels_spin = SpinBox.new()
	_open_panels_spin.min_value = SigilConfig.MIN_SCROLL_PANELS
	_open_panels_spin.max_value = SigilConfig.MAX_SCROLL_PANELS
	_open_panels_spin.step = 1
	_open_panels_spin.value = SigilConfig.DEFAULT_SCROLL_PANELS
	_open_panels_spin.get_line_edit().add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	_open_panels_spin.value_changed.connect(_set_max_scroll_panels)
	row.add_child(_open_panels_spin)

	return row


## Chat panel default open size, as a percentage of the screen. Two spin
## boxes rather than a pixel size, so the setting survives a resolution or
## monitor change instead of pinning the panel to one screen's geometry.
## These set where a panel OPENS — PanelBase's drag-resize still applies on
## top, so this is the default a keeper stops having to re-drag every
## session.
func _build_panel_size_row() -> HBoxContainer:
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", GRID_H_SEPARATION)

	var label := Label.new()
	label.text = "CHAT PANEL SIZE (%)"
	label.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	row.add_child(label)

	_panel_width_spin = _build_ratio_spin(_set_scroll_width_ratio)
	row.add_child(_wrap_spin_with_caption("W", _panel_width_spin))

	_panel_height_spin = _build_ratio_spin(_set_scroll_height_ratio)
	row.add_child(_wrap_spin_with_caption("H", _panel_height_spin))

	return row


## One percentage SpinBox over the SigilConfig ratio range. The config
## stores 0.2..1.0 fractions but the keeper reads percentages, so the
## widget works in whole percent and the setters divide.
func _build_ratio_spin(on_changed: Callable) -> SpinBox:
	var spin := SpinBox.new()
	spin.min_value = SigilConfig.MIN_SCROLL_RATIO * 100.0
	spin.max_value = SigilConfig.MAX_SCROLL_RATIO * 100.0
	spin.step = 1
	spin.suffix = "%"
	spin.get_line_edit().add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	spin.value_changed.connect(on_changed)
	return spin


func _wrap_spin_with_caption(caption: String, spin: SpinBox) -> HBoxContainer:
	var box := HBoxContainer.new()
	var label := Label.new()
	label.text = caption
	label.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	box.add_child(label)
	box.add_child(spin)
	return box


func _set_scroll_width_ratio(percent: float) -> void:
	if _config == null:
		return
	_config.scroll_width_ratio = SigilConfig.clamp_scroll_ratio(percent / 100.0)
	_config.save_to_file()


func _set_scroll_height_ratio(percent: float) -> void:
	if _config == null:
		return
	_config.scroll_height_ratio = SigilConfig.clamp_scroll_ratio(percent / 100.0)
	_config.save_to_file()


func _set_max_scroll_panels(value: float) -> void:
	if _config == null:
		return
	var clamped: int = SigilConfig.clamp_scroll_panels(int(value))
	if _config.max_scroll_panels == clamped:
		return
	_config.max_scroll_panels = clamped
	_config.save_to_file()


## Theme for the in-engine folder picker so it matches the config panel's
## dark-parchment fill and gold border instead of Godot's default grey
## chrome. Built in code for the same reason every other Control on this
## page is (see the class doc-comment).
func _build_dialog_theme() -> Theme:
	var theme := Theme.new()
	theme.default_font_size = DIALOG_FONT_SIZE

	var window_panel := StyleBoxFlat.new()
	window_panel.bg_color = PANEL_BG_COLOR
	window_panel.border_color = PANEL_BORDER_COLOR
	window_panel.set_border_width_all(PANEL_BORDER_WIDTH)
	window_panel.set_corner_radius_all(8)
	window_panel.set_content_margin_all(DIALOG_CONTENT_MARGIN)
	theme.set_stylebox("panel", "Window", window_panel)
	theme.set_color("title_color", "Window", PANEL_BORDER_COLOR)
	theme.set_constant("title_height", "Window", DIALOG_TITLE_HEIGHT)

	# The file/dir list and the path field sit on a slightly lifted fill so
	# their bounds stay legible against the panel behind them.
	var field := StyleBoxFlat.new()
	field.bg_color = DIALOG_FIELD_COLOR
	field.border_color = PANEL_BORDER_COLOR
	field.set_border_width_all(1)
	field.set_corner_radius_all(4)
	field.set_content_margin_all(10)
	for type: String in ["LineEdit", "ItemList", "Tree", "PanelContainer"]:
		theme.set_stylebox("panel" if type == "PanelContainer" else "normal", type, field)
	theme.set_stylebox("focus", "LineEdit", field)

	theme.set_color("font_color", "Label", DIALOG_TEXT_COLOR)
	theme.set_color("font_color", "LineEdit", DIALOG_TEXT_COLOR)
	theme.set_color("font_color", "ItemList", DIALOG_TEXT_COLOR)
	theme.set_color("font_color", "Tree", DIALOG_TEXT_COLOR)
	theme.set_color("font_color", "Button", DIALOG_TEXT_COLOR)
	theme.set_color("font_hover_color", "Button", PANEL_BORDER_COLOR)
	theme.set_color("font_pressed_color", "Button", PANEL_BORDER_COLOR)

	for state: String in ["normal", "hover", "pressed", "disabled"]:
		theme.set_stylebox("normal" if state == "normal" else state, "Button", _build_dialog_button_style(state))
	return theme


## One button state's stylebox for the folder picker. Hover/pressed lift the
## fill and brighten the gold border so the picker's own buttons read as the
## same family as the config panel's.
func _build_dialog_button_style(state: String) -> StyleBoxFlat:
	var style := StyleBoxFlat.new()
	match state:
		"hover":
			style.bg_color = DIALOG_BUTTON_HOVER_COLOR
		"pressed":
			style.bg_color = DIALOG_BUTTON_PRESSED_COLOR
		"disabled":
			style.bg_color = DIALOG_FIELD_COLOR
		_:
			style.bg_color = DIALOG_BUTTON_COLOR
	style.border_color = PANEL_BORDER_COLOR if state != "disabled" else DIALOG_TEXT_COLOR
	style.set_border_width_all(2)
	style.set_corner_radius_all(6)
	style.set_content_margin_all(12)
	return style


func _open_folder_dialog() -> void:
	if _config != null and not _config.workdir.is_empty():
		_folder_dialog.current_dir = _config.workdir
	_folder_dialog.popup_centered_ratio(0.7)


func _set_workdir(path: String) -> void:
	if _config == null:
		return
	var trimmed: String = path.strip_edges()
	if _config.workdir == trimmed:
		return
	_config.workdir = trimmed
	_config.save_to_file()
	_workdir_edit.text = trimmed


## Populates the page from a provider roster (ProviderRoster.parse_response
## output) and the persisted config. Safe to call more than once — a fresh
## roster fetch (e.g. a reconnect) simply rebuilds the rows.
## Both widget syncs are null-guarded because open_page is reachable from a
## BridgeManager signal (title_screen's _on_providers_fetched), and a roster
## response that lands before this node's _ready has built its rows would
## otherwise assign onto a null LineEdit.
func open_page(providers: Array, config: SigilConfig) -> void:
	_providers = providers
	_config = config
	if _workdir_edit != null:
		_workdir_edit.text = config.workdir
	# set_value_no_signal, not .value: assigning .value emits value_changed,
	# whose handler saves the config. Merely OPENING the page would then
	# write to disk — and worse, write back whatever the SpinBox clamped the
	# value to. Syncing silently keeps open_page a pure read.
	if _open_panels_spin != null:
		_open_panels_spin.set_value_no_signal(config.max_scroll_panels)
	# The size spins work in whole percent while the config stores a
	# fraction, so these convert on the way in exactly as the setters
	# convert on the way out.
	if _panel_width_spin != null:
		_panel_width_spin.set_value_no_signal(config.scroll_width_ratio * 100.0)
	if _panel_height_spin != null:
		_panel_height_spin.set_value_no_signal(config.scroll_height_ratio * 100.0)
	if _rows_grid != null:
		_rebuild_rows()


## Last roster this page was opened with — read back by a caller (e.g.
## title_screen.gd) that wants to reopen the page without a fresh fetch.
func get_providers() -> Array:
	return _providers


func _unhandled_input(event: InputEvent) -> void:
	if not visible:
		return
	if event.is_action_pressed("ui_cancel"):
		closed.emit()
		get_viewport().set_input_as_handled()


func _rebuild_rows() -> void:
	for child: Node in _rows_grid.get_children():
		child.queue_free()
	var all_kinds: Array = _providers.map(func(p: Dictionary) -> String: return p["kind"])
	for i: int in range(_providers.size()):
		_add_row_cells(_providers[i], i, all_kinds)


## Fixed six-column layout, one column each for: enable checkbox, provider
## name, up arrow, down arrow, tier dropdown, name field. Cells append
## straight into the shared GridContainer so every row lines up on the same
## column boundaries, instead of each row owning its own HBoxContainer.
func _add_row_cells(provider: Dictionary, index: int, all_kinds: Array) -> void:
	var kind: String = provider["kind"]

	var toggle := CheckBox.new()
	toggle.button_pressed = _config.is_enabled(kind)
	toggle.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	toggle.toggled.connect(func(is_on: bool) -> void:
		if not is_on:
			_config.expand_enabled_for_disable(all_kinds)
		_config.set_enabled(kind, is_on)
		_config.save_to_file()
	)
	_rows_grid.add_child(toggle)

	var name_label := Label.new()
	name_label.text = String(provider["display"])
	name_label.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	name_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	_rows_grid.add_child(name_label)

	var up_button := Button.new()
	up_button.text = "^"
	up_button.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	up_button.disabled = index == 0
	up_button.pressed.connect(func() -> void: _move_order(kind, -1, all_kinds))
	_rows_grid.add_child(up_button)

	var down_button := Button.new()
	down_button.text = "v"
	down_button.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	down_button.disabled = index == _providers.size() - 1
	down_button.pressed.connect(func() -> void: _move_order(kind, 1, all_kinds))
	_rows_grid.add_child(down_button)

	var tier_button := OptionButton.new()
	tier_button.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	for tier: String in TIERS:
		tier_button.add_item(tier if not tier.is_empty() else "(default)")
	var current_tier: String = _config.tier_default(kind)
	var tier_idx: int = TIERS.find(current_tier)
	tier_button.selected = maxi(tier_idx, 0)
	tier_button.item_selected.connect(func(item_index: int) -> void:
		_config.set_tier_default(kind, TIERS[item_index])
		_config.save_to_file()
	)
	_rows_grid.add_child(tier_button)

	var name_pool_edit := LineEdit.new()
	name_pool_edit.placeholder_text = "names, comma separated"
	name_pool_edit.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	name_pool_edit.custom_minimum_size = Vector2(420.0, 0.0)
	name_pool_edit.text = ",".join(PackedStringArray(_config.name_pool(kind)))
	name_pool_edit.text_submitted.connect(func(new_text: String) -> void:
		var names: Array = []
		for entry: String in new_text.split(","):
			var trimmed: String = entry.strip_edges()
			if not trimmed.is_empty():
				names.append(trimmed)
		_config.set_name_pool(kind, names)
		_config.save_to_file()
	)
	_rows_grid.add_child(name_pool_edit)


## Reorders `order` in the config by swapping `kind` with its neighbour in
## `direction` (-1 up, +1 down), materializing the roster's arrival order
## first so a swap on an unconfigured (empty) order list has something to
## swap against.
func _move_order(kind: String, direction: int, all_kinds: Array) -> void:
	if _config.order.is_empty():
		_config.order = all_kinds.duplicate()
	var idx: int = _config.order.find(kind)
	if idx == -1:
		return
	var target: int = idx + direction
	if target < 0 or target >= _config.order.size():
		return
	var tmp: Variant = _config.order[idx]
	_config.order[idx] = _config.order[target]
	_config.order[target] = tmp
	_config.save_to_file()
	# Re-derive _providers' displayed order from the new config order, then
	# rebuild so the up/down buttons reflect the new neighbours.
	_providers = ProviderRoster.filtered_and_ordered(_providers, _config.enabled, _config.order)
	_rebuild_rows()
