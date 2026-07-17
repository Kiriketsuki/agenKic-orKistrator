extends Control
## Minimap — procedural vertical tower silhouette. One cell per floor,
## colored by activity level, with a bracket outline on the focused floor and
## the current viewport band. Reads TowerManager on demand; never polls or
## touches BridgeManager directly. Toggled independently with "toggle_minimap".

const DIM_COLOR: Color = Color(0.22, 0.2, 0.17, 1.0)  # cold stone
const ACTIVE_COLOR: Color = Color(0.85, 0.6, 0.16, 1.0)  # warm amber/gold
const BRACKET_COLOR: Color = Color(0.95, 0.85, 0.45, 1.0)  # bright rune gold
const BAND_TINT: Color = Color(1.0, 1.0, 1.0, 0.12)
const ACTIVITY_SCALE: float = 3.0
const CELL_MARGIN: float = 1.0
const BRACKET_WIDTH: float = 2.0

var _tower: Node = null


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_STOP
	_tower = get_node_or_null("../../Tower")
	if _tower:
		if _tower.has_signal("floor_focus_changed"):
			_tower.floor_focus_changed.connect(func(_index: int) -> void: queue_redraw())
		if _tower.has_signal("floors_changed"):
			_tower.floors_changed.connect(queue_redraw)
	visibility_changed.connect(queue_redraw)


func _unhandled_input(event: InputEvent) -> void:
	if event.is_action_pressed("toggle_minimap"):
		visible = not visible
		get_viewport().set_input_as_handled()


func _draw() -> void:
	if _tower == null:
		return
	var infos: Array[Dictionary] = _tower.get_floor_infos()
	var count: int = infos.size()
	if count <= 0:
		return
	var focus: int = _tower.get_focus_index()
	var cell_h: float = size.y / float(count)
	for info: Dictionary in infos:
		var idx: int = info.get("index", 0)
		var active_count: int = info.get("active_count", 0)
		var row: int = count - 1 - idx  # topmost floor (highest index) drawn first
		var y: float = row * cell_h
		var t: float = clampf(float(active_count) / ACTIVITY_SCALE, 0.0, 1.0)
		var fill_color: Color = DIM_COLOR.lerp(ACTIVE_COLOR, t)
		var distance: int = absi(idx - focus)
		if distance == 1:
			fill_color = fill_color.lerp(Color.WHITE, 0.12)
		draw_rect(Rect2(0.0, y + CELL_MARGIN, size.x, cell_h - CELL_MARGIN * 2.0), fill_color, true)
		if idx == focus:
			draw_rect(
				Rect2(0.0, y + CELL_MARGIN, size.x, cell_h - CELL_MARGIN * 2.0),
				BRACKET_COLOR,
				false,
				BRACKET_WIDTH
			)


func _gui_input(event: InputEvent) -> void:
	if _tower == null:
		return
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed and mb.button_index == MOUSE_BUTTON_LEFT:
			var infos: Array[Dictionary] = _tower.get_floor_infos()
			var count: int = infos.size()
			if count <= 0:
				return
			var cell_h: float = size.y / float(count)
			var row: int = floori(mb.position.y / cell_h)
			var idx: int = clampi(count - 1 - row, 0, count - 1)
			_tower.jump_to_floor(idx)
			accept_event()
