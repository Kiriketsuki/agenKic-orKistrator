class_name GrimoireFlyout
extends Control
## GrimoireFlyout — Grimoire Summoning (F3) sigil grid and drag-to-floor
## placement. Replaces the interim SummonBar panel OrbFlock hosted for F2
## (see orb_flock.gd's doc-comment). Registers itself under the "grimoire"
## orb the same way SummonBar did, via OrbFlock.register_flyout.
##
## Sigil grid: one button per enabled provider from GET /api/providers,
## filtered and ordered by SigilConfig (see ProviderRoster). A drag past the
## threshold enters placement mode: the grid dims, a sigil ghost follows the
## cursor, and TowerManager.hit_test_floor classifies the drop target as the
## cursor crosses the tower. Escape or a release outside any zone cancels.

signal spawn_requested(kind: String, floor: int)

## True while a sigil drag has crossed the placement threshold. OrbFlock
## reads this before it handles ui_cancel in its own _input(), so Escape
## cancels an in-flight placement first instead of the flock collapsing out
## from under it (see orb_flock.gd's _input doc-comment).
static var is_placing: bool = false

const DRAG_THRESHOLD_PX: float = 6.0
const COLOR_ACCEPT: Color = Color(0.435, 0.722, 0.663, 0.35)
const COLOR_REJECT: Color = Color(0.780, 0.267, 0.227, 0.35)
const COLOR_NEW_FLOOR: Color = Color(0.788, 0.635, 0.153, 0.35)
const REJECT_SHAKE_DUR: float = 0.3
const REJECT_SHAKE_PX: float = 6.0

var _config: SigilConfig
var _providers: Array = []
var _grid: GridContainer
var _config_page: SigilConfigPage
var _config_button: Button

var _dragging: bool = false
var _drag_kind: String = ""
var _drag_start_pos: Vector2 = Vector2.ZERO
var _drag_pos: Vector2 = Vector2.ZERO
var _hit_result: Dictionary = {}
var _reject_shake_t: float = -1.0


func _ready() -> void:
	# Static var, no reset on scene reload otherwise. Mirrors
	# OrbFlock.suspend_hotkeys's own _ready reset for the same reason.
	is_placing = false
	set_anchors_preset(Control.PRESET_TOP_WIDE)
	custom_minimum_size = Vector2(420.0, 160.0)
	size = Vector2(420.0, 160.0)

	_config = SigilConfig.load_from_file()

	var box := VBoxContainer.new()
	box.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(box)

	_grid = GridContainer.new()
	_grid.columns = 5
	box.add_child(_grid)

	_config_button = Button.new()
	_config_button.text = "sigil config..."
	_config_button.pressed.connect(open_config_page)
	box.add_child(_config_button)

	_config_page = SigilConfigPage.new()
	_config_page.visible = false
	_config_page.closed.connect(func() -> void: _config_page.visible = false)
	add_child(_config_page)

	BridgeManager.providers_fetched.connect(_on_providers_fetched)
	BridgeManager.fetch_providers()


func _on_providers_fetched(providers: Array) -> void:
	_providers = providers
	_rebuild_grid()


func _rebuild_grid() -> void:
	for child: Node in _grid.get_children():
		child.queue_free()
	var visible_providers: Array = ProviderRoster.filtered_and_ordered(_providers, _config.enabled, _config.order)
	for provider: Dictionary in visible_providers:
		_grid.add_child(_build_sigil_button(provider))


func _build_sigil_button(provider: Dictionary) -> Button:
	var kind: String = provider["kind"]
	var button := Button.new()
	button.text = String(provider["display"])
	button.custom_minimum_size = Vector2(72.0, 48.0)
	var accent: Color = Color(String(provider["accent"]))
	button.add_theme_color_override("font_color", accent)
	button.gui_input.connect(func(event: InputEvent) -> void: _on_sigil_input(kind, event))
	return button


## Opens the sigil config page (F3). Public so the Power flyout's SETTINGS
## action (F4) can reach it after switching the flock to the Grimoire orb.
func open_config_page() -> void:
	_config_page.open_page(_providers, _config)
	_config_page.visible = true


func _on_sigil_input(kind: String, event: InputEvent) -> void:
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.button_index != MOUSE_BUTTON_LEFT:
			return
		if mb.pressed:
			_drag_kind = kind
			_drag_start_pos = mb.global_position
			_drag_pos = mb.global_position
		elif _dragging and _drag_kind == kind:
			_end_placement(mb.global_position)
		else:
			_drag_kind = ""
	elif event is InputEventMouseMotion and not _drag_kind.is_empty():
		var mm: InputEventMouseMotion = event as InputEventMouseMotion
		_drag_pos = mm.global_position
		if not _dragging and _drag_pos.distance_to(_drag_start_pos) >= DRAG_THRESHOLD_PX:
			_begin_placement()


func _begin_placement() -> void:
	_dragging = true
	is_placing = true
	OrbFlock.suspend_hotkeys = true
	modulate.a = 0.35
	set_process(true)
	set_process_unhandled_input(true)


func _process(delta: float) -> void:
	if not _dragging and _reject_shake_t < 0.0:
		set_process(false)
		return
	if _dragging:
		var tower: Node = get_tree().get_first_node_in_group("tower_manager")
		if tower != null:
			var world_pos: Vector2 = tower.world_position_from_screen(_drag_pos)
			_hit_result = tower.hit_test_floor(world_pos)
	if _reject_shake_t >= 0.0:
		_reject_shake_t -= delta
		if _reject_shake_t < 0.0:
			_reject_shake_t = -1.0
			_hit_result = {}
	queue_redraw()


func _unhandled_input(event: InputEvent) -> void:
	if not _dragging:
		return
	if event.is_action_pressed("ui_cancel"):
		_cancel_placement()
		get_viewport().set_input_as_handled()


func _end_placement(screen_pos: Vector2) -> void:
	_drag_pos = screen_pos
	var tower: Node = get_tree().get_first_node_in_group("tower_manager")
	var result: Dictionary = {}
	if tower != null:
		result = tower.hit_test_floor(tower.world_position_from_screen(screen_pos))
	# Keep the final hit result live before _stop_dragging runs, so a reject
	# shake has a floor to highlight instead of an empty dictionary.
	_hit_result = result
	var kind: String = _drag_kind
	var zone_type: int = result.get("type", FloorHitTest.ZoneType.NONE)
	var is_reject: bool = zone_type == FloorHitTest.ZoneType.FLOOR and result.get("is_full", false)
	# _show_reject raises the shake timer before _stop_dragging runs, so its
	# `_reject_shake_t < 0.0` guard sees the timer already armed and leaves
	# _hit_result alone instead of clearing it.
	if is_reject:
		_show_reject()
	_stop_dragging()
	if is_reject:
		return
	match zone_type:
		FloorHitTest.ZoneType.FLOOR:
			_spawn(kind, int(result["index"]))
		FloorHitTest.ZoneType.NEW_FLOOR:
			_spawn(kind, int(result["index"]))
		_:
			# RESERVED (floor 0) or NONE — no drop zone, cancel silently.
			pass


func _cancel_placement() -> void:
	_stop_dragging()


## Symmetric with _begin_placement: that function turns both set_process and
## set_process_unhandled_input on, so this turns both off. set_process stays
## on past this point when a reject shake is still running (see _process's
## own turn-off, once the shake timer runs out) — the shake must still
## render even though dragging itself has already ended.
func _stop_dragging() -> void:
	_dragging = false
	_drag_kind = ""
	is_placing = false
	OrbFlock.suspend_hotkeys = false
	modulate.a = 1.0
	set_process_unhandled_input(false)
	if _reject_shake_t < 0.0:
		_hit_result = {}
		set_process(false)
	queue_redraw()


func _show_reject() -> void:
	_reject_shake_t = REJECT_SHAKE_DUR
	set_process(true)
	queue_redraw()


func _spawn(kind: String, floor: int) -> void:
	var tier: String = _config.tier_default(kind)
	var names: Array = _config.name_pool(kind)
	var agent_name: String = String(names[randi() % names.size()]) if not names.is_empty() else ""
	BridgeManager.spawn_agent(kind, agent_name, tier, floor)
	spawn_requested.emit(kind, floor)


func _draw() -> void:
	if not _dragging and _reject_shake_t < 0.0:
		return
	if _dragging:
		# Sigil ghost following the cursor, drawn in this control's local space.
		var local_pos: Vector2 = get_local_mouse_position()
		var shake_offset: Vector2 = Vector2.ZERO
		if _reject_shake_t >= 0.0:
			shake_offset.x = sin(_reject_shake_t * 60.0) * REJECT_SHAKE_PX
		draw_circle(local_pos + shake_offset, 14.0, Color(1.0, 1.0, 1.0, 0.6))

	# Floor highlight overlay, drawn full-width at the hit floor's screen Y.
	var tower: Node = get_tree().get_first_node_in_group("tower_manager")
	if tower == null or _hit_result.is_empty():
		return
	var zone_type: int = _hit_result.get("type", FloorHitTest.ZoneType.NONE)
	if zone_type == FloorHitTest.ZoneType.NONE or zone_type == FloorHitTest.ZoneType.RESERVED:
		return
	var index: int = int(_hit_result["index"])
	var spacing: float = tower.get_floor_spacing()
	var height: float = tower.get_floor_height()
	var world_center_y: float = FloorHitTest.floor_center_y(index, spacing)
	var top_screen: Vector2 = tower.screen_position_from_world(Vector2(-1000.0, world_center_y - height / 2.0))
	var bottom_screen: Vector2 = tower.screen_position_from_world(Vector2(1000.0, world_center_y + height / 2.0))
	var rect := Rect2(Vector2(0.0, top_screen.y) - global_position, Vector2(get_viewport_rect().size.x, bottom_screen.y - top_screen.y))
	var color: Color = COLOR_ACCEPT
	if zone_type == FloorHitTest.ZoneType.NEW_FLOOR:
		color = COLOR_NEW_FLOOR
	elif _hit_result.get("is_full", false):
		color = COLOR_REJECT
	draw_rect(rect, color, true)
