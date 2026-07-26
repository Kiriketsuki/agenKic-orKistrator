extends VBoxContainer
## FloorTabs — vertical tab strip, one parchment tab per floor (highest floor
## at the strip top), each showing the floor name + an agent-count badge.
## Reads TowerManager on demand; never polls or touches BridgeManager directly.
## Toggled independently with "toggle_floor_tabs".

const TAB_BG: Color = Color(0.16, 0.13, 0.09, 0.92)
const TAB_BORDER: Color = Color(0.45, 0.35, 0.22, 1.0)
const TAB_BORDER_FOCUSED: Color = Color(0.95, 0.85, 0.45, 1.0)
const NAME_COLOR: Color = Color(0.92, 0.86, 0.72, 1.0)
const BADGE_BG: Color = Color(0.35, 0.25, 0.12, 1.0)
const BADGE_COLOR: Color = Color(0.95, 0.9, 0.8, 1.0)

var _tower: Node = null
var _tabs_by_index: Dictionary = {}  # index -> PanelContainer


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	clip_contents = true
	_tower = get_node_or_null("../../Tower")
	if _tower:
		if _tower.has_signal("floors_changed"):
			_tower.floors_changed.connect(_rebuild)
		if _tower.has_signal("floor_focus_changed"):
			_tower.floor_focus_changed.connect(func(_index: int) -> void: _restyle_focus())
	visibility_changed.connect(_rebuild)
	_rebuild()


func _input(event: InputEvent) -> void:
	# Deliberately _input (not _unhandled_input): Godot's GUI focus traversal
	# consumes Tab (ui_focus_next) before _unhandled_input ever runs, so the
	# toggle must win here first.
	# Guard: if a Control currently owns keyboard focus (a LineEdit doing its
	# own Tab-traversal, or a live godot-xterm Terminal that wants Tab for
	# shell autocompletion), do not steal Tab from it — mirrors
	# panel_manager.gd's _focus_owner_is_live_terminal() guard on toggle_terminal.
	if event.is_action_pressed("toggle_floor_tabs"):
		if _focus_owner_present():
			return
		visible = not visible
		get_viewport().set_input_as_handled()


## True when the viewport's current keyboard focus owner is some Control —
## i.e. the user is actively focused on a LineEdit, live PTY terminal, or any
## other focusable widget that should receive Tab itself rather than have it
## intercepted here as a global hotkey.
func _focus_owner_present() -> bool:
	var viewport: Viewport = get_viewport()
	if viewport == null:
		return false
	return viewport.gui_get_focus_owner() != null


func _rebuild() -> void:
	for child: Node in get_children():
		child.queue_free()
	_tabs_by_index.clear()
	if _tower == null or not visible:
		return
	var infos: Array[Dictionary] = _tower.get_floor_infos()
	for i: int in range(infos.size() - 1, -1, -1):
		var info: Dictionary = infos[i]
		add_child(_build_tab(info))
	_restyle_focus()


func _build_tab(info: Dictionary) -> PanelContainer:
	var index: int = info.get("index", 0)
	var tab: PanelContainer = PanelContainer.new()
	tab.focus_mode = Control.FOCUS_NONE
	tab.mouse_filter = Control.MOUSE_FILTER_STOP
	# Expand-fill + a small minimum lets the VBoxContainer distribute the
	# strip's available height evenly across however many floors exist —
	# mirrors minimap.gd's `cell_h = size.y / count` rescale-to-fit-any-count
	# behavior instead of hardcoding a fixed per-tab height that would
	# overflow the strip (and go unreachable) once floor count grows.
	tab.size_flags_vertical = Control.SIZE_EXPAND_FILL
	tab.custom_minimum_size = Vector2(0.0, 14.0)
	tab.add_theme_stylebox_override("panel", _make_tab_style(false))
	tab.gui_input.connect(func(event: InputEvent) -> void:
		if event is InputEventMouseButton:
			var mb: InputEventMouseButton = event as InputEventMouseButton
			if mb.pressed and mb.button_index == MOUSE_BUTTON_LEFT and _tower:
				_tower.jump_to_floor(index)
	)

	var margin: MarginContainer = MarginContainer.new()
	margin.add_theme_constant_override("margin_left", 8)
	margin.add_theme_constant_override("margin_right", 8)
	margin.add_theme_constant_override("margin_top", 4)
	margin.add_theme_constant_override("margin_bottom", 4)
	margin.mouse_filter = Control.MOUSE_FILTER_IGNORE
	tab.add_child(margin)

	var hbox: HBoxContainer = HBoxContainer.new()
	hbox.mouse_filter = Control.MOUSE_FILTER_IGNORE
	margin.add_child(hbox)

	var name_label: Label = Label.new()
	name_label.text = String(info.get("label", ""))
	name_label.add_theme_color_override("font_color", NAME_COLOR)
	name_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	name_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	hbox.add_child(name_label)

	var badge_panel: PanelContainer = PanelContainer.new()
	badge_panel.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var badge_style: StyleBoxFlat = StyleBoxFlat.new()
	badge_style.bg_color = BADGE_BG
	badge_style.set_corner_radius_all(6)
	badge_style.content_margin_left = 6.0
	badge_style.content_margin_right = 6.0
	badge_style.content_margin_top = 1.0
	badge_style.content_margin_bottom = 1.0
	badge_panel.add_theme_stylebox_override("panel", badge_style)
	hbox.add_child(badge_panel)

	var badge: Label = Label.new()
	badge.text = str(int(info.get("agent_count", 0)))
	badge.add_theme_color_override("font_color", BADGE_COLOR)
	badge.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	badge.custom_minimum_size = Vector2(16, 0)
	badge.mouse_filter = Control.MOUSE_FILTER_IGNORE
	badge_panel.add_child(badge)

	_tabs_by_index[index] = tab
	return tab


func _restyle_focus() -> void:
	if _tower == null:
		return
	var focus: int = _tower.get_focus_index()
	for index: int in _tabs_by_index:
		var tab: PanelContainer = _tabs_by_index[index]
		if is_instance_valid(tab):
			tab.add_theme_stylebox_override("panel", _make_tab_style(index == focus))


func _make_tab_style(focused: bool) -> StyleBoxFlat:
	var sb: StyleBoxFlat = StyleBoxFlat.new()
	sb.bg_color = TAB_BG
	sb.set_corner_radius_all(4)
	sb.set_border_width_all(2 if focused else 1)
	sb.border_color = TAB_BORDER_FOCUSED if focused else TAB_BORDER
	return sb
