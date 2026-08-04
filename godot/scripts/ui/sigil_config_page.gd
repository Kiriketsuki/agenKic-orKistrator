class_name SigilConfigPage
extends VBoxContainer
## SigilConfigPage — Grimoire Summoning (F3) sigil config: provider toggles,
## order, and per-provider defaults (tier, name pool). Reachable from the
## Grimoire flyout and the title screen's GRIMOIRE entry (see
## title_screen.gd/grimoire_flyout.gd), so this builds its own rows in code
## rather than living in a shared .tscn, and both callers can drop it inside
## whatever container they already have.
##
## Persists to user://orki_settings.cfg via SigilConfig. Every edit saves
## immediately — there is no separate "Apply" step, so a keeper who closes
## the page without an explicit save action never loses a change.

signal closed()

const TIERS: Array[String] = ["", "haiku", "sonnet", "opus", "adept", "novice"]

var _config: SigilConfig
var _providers: Array = []
var _rows_box: VBoxContainer


func _ready() -> void:
	add_theme_constant_override("separation", 12)
	var title := Label.new()
	title.text = "SIGIL CONFIG"
	title.add_theme_font_size_override("font_size", 22)
	title.add_theme_color_override("font_color", Color(0.788, 0.635, 0.153, 1.0))
	add_child(title)

	_rows_box = VBoxContainer.new()
	_rows_box.add_theme_constant_override("separation", 6)
	add_child(_rows_box)

	var close_button := Button.new()
	close_button.text = "BACK"
	close_button.pressed.connect(func() -> void: closed.emit())
	add_child(close_button)


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


func _rebuild_rows() -> void:
	for child: Node in _rows_box.get_children():
		child.queue_free()
	var all_kinds: Array = _providers.map(func(p: Dictionary) -> String: return p["kind"])
	for i: int in range(_providers.size()):
		_rows_box.add_child(_build_row(_providers[i], i, all_kinds))


func _build_row(provider: Dictionary, index: int, all_kinds: Array) -> Control:
	var kind: String = provider["kind"]
	var row := HBoxContainer.new()
	row.add_theme_constant_override("separation", 8)

	var toggle := CheckBox.new()
	toggle.text = String(provider["display"])
	toggle.button_pressed = _config.is_enabled(kind)
	toggle.toggled.connect(func(is_on: bool) -> void:
		if not is_on:
			_config.expand_enabled_for_disable(all_kinds)
		_config.set_enabled(kind, is_on)
		_config.save_to_file()
	)
	row.add_child(toggle)

	var up_button := Button.new()
	up_button.text = "^"
	up_button.disabled = index == 0
	up_button.pressed.connect(func() -> void: _move_order(kind, -1, all_kinds))
	row.add_child(up_button)

	var down_button := Button.new()
	down_button.text = "v"
	down_button.disabled = index == _providers.size() - 1
	down_button.pressed.connect(func() -> void: _move_order(kind, 1, all_kinds))
	row.add_child(down_button)

	var tier_button := OptionButton.new()
	for tier: String in TIERS:
		tier_button.add_item(tier if not tier.is_empty() else "(default)")
	var current_tier: String = _config.tier_default(kind)
	var tier_idx: int = TIERS.find(current_tier)
	tier_button.selected = maxi(tier_idx, 0)
	tier_button.item_selected.connect(func(item_index: int) -> void:
		_config.set_tier_default(kind, TIERS[item_index])
		_config.save_to_file()
	)
	row.add_child(tier_button)

	var name_pool_edit := LineEdit.new()
	name_pool_edit.placeholder_text = "name pool (comma separated)"
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
	row.add_child(name_pool_edit)

	return row


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
