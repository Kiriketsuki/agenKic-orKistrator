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

const TIERS: Array[String] = ["", "haiku", "sonnet", "opus", "adept", "novice"]

## Row grid separation, doubled from the pre-2x base (10px, 8px) to match
## the doubled project.godot viewport (see defect 3, epic 169).
const GRID_H_SEPARATION: int = 20
const GRID_V_SEPARATION: int = 16
const BACKDROP_COLOR: Color = Color(0.0, 0.0, 0.0, 0.6)
const PANEL_MIN_SIZE: Vector2 = Vector2(1200.0, 760.0)
const TITLE_FONT_SIZE: int = 40
const ROW_FONT_SIZE: int = 24

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


func _ready() -> void:
	set_process_unhandled_input(true)

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

		_panel = PanelContainer.new()
		_panel.set_anchors_preset(Control.PRESET_CENTER)
		_panel.custom_minimum_size = PANEL_MIN_SIZE
		_panel.offset_left = -PANEL_MIN_SIZE.x * 0.5
		_panel.offset_top = -PANEL_MIN_SIZE.y * 0.5
		_panel.offset_right = PANEL_MIN_SIZE.x * 0.5
		_panel.offset_bottom = PANEL_MIN_SIZE.y * 0.5
		_panel.mouse_filter = Control.MOUSE_FILTER_STOP
		add_child(_panel)
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


## Populates the page from a provider roster (ProviderRoster.parse_response
## output) and the persisted config. Safe to call more than once — a fresh
## roster fetch (e.g. a reconnect) simply rebuilds the rows.
func open_page(providers: Array, config: SigilConfig) -> void:
	_providers = providers
	_config = config
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
	name_pool_edit.placeholder_text = "name pool (comma separated)"
	name_pool_edit.add_theme_font_size_override("font_size", ROW_FONT_SIZE)
	name_pool_edit.custom_minimum_size = Vector2(280.0, 0.0)
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
