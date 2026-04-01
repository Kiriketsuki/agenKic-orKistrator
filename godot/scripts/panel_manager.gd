extends Control
## PanelManager — owns floating panels, dock zones, and master-width layout state.

class_name PanelManager

signal master_region_changed(region: Rect2)

const PANEL_BASE_SCENE: PackedScene = preload("res://scenes/panel_base.tscn")
const MASTER_RATIO_DEFAULT: float = 0.6
const MASTER_RATIO_MIN: float = 0.3
const MASTER_RATIO_MAX: float = 0.8
const MASTER_RATIO_SNAP_POINTS: Array[float] = [0.25, 0.5, 0.75]
const DIVIDER_WIDTH: float = 8.0
const DOCK_PREVIEW_WIDTH: float = 72.0

var master_ratio: float = MASTER_RATIO_DEFAULT
var left_tree: DwindleTree = DwindleTree.new("left")
var right_tree: DwindleTree = DwindleTree.new("right")
var panels_by_id: Dictionary = {}

var _dragging_master_boundary: bool = false
var _last_layout: Dictionary = {}

@onready var _dimmer: ColorRect = $Dimmer
@onready var _left_preview: ColorRect = $DockPreviews/LeftPreview
@onready var _right_preview: ColorRect = $DockPreviews/RightPreview
@onready var _left_zone: Control = $DockZones/LeftZone
@onready var _right_zone: Control = $DockZones/RightZone
@onready var _floating_layer: Control = $FloatingLayer
@onready var _left_divider: ColorRect = $Dividers/LeftDivider
@onready var _right_divider: ColorRect = $Dividers/RightDivider


func _ready() -> void:
	anchors_preset = Control.PRESET_FULL_RECT
	offset_left = 0.0
	offset_top = 0.0
	offset_right = 0.0
	offset_bottom = 0.0
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	get_viewport().size_changed.connect(_refresh_layout)
	_bind_divider(_left_divider, "left")
	_bind_divider(_right_divider, "right")
	_refresh_layout()


func open_panel(panel_id: String, title: String, agent_id: String = "", preferred_mode: String = "scroll") -> PanelBase:
	if panels_by_id.has(panel_id):
		var existing: PanelBase = panels_by_id[panel_id]
		focus_panel(existing)
		return existing
	var panel: PanelBase = PANEL_BASE_SCENE.instantiate() as PanelBase
	panel.panel_id = panel_id
	panel.agent_id = agent_id
	panel.set_panel_title(title)
	panel.set_mode(preferred_mode)
	panel.position = _default_floating_position(panels_by_id.size())
	_floating_layer.add_child(panel)
	panel.size = Vector2(maxf(420.0, panel.custom_minimum_size.x), maxf(280.0, panel.custom_minimum_size.y))
	_wire_panel(panel)
	panels_by_id[panel_id] = panel
	focus_panel(panel)
	return panel


func close_panel(panel_id: String) -> void:
	if not panels_by_id.has(panel_id):
		return
	var panel: PanelBase = panels_by_id[panel_id]
	left_tree.remove_panel(panel_id)
	right_tree.remove_panel(panel_id)
	panels_by_id.erase(panel_id)
	panel.queue_free()
	_refresh_layout()


func focus_panel(panel: PanelBase) -> void:
	if panel.get_parent() == _floating_layer:
		_floating_layer.move_child(panel, _floating_layer.get_child_count() - 1)


func has_panel(panel_id: String) -> bool:
	return panels_by_id.has(panel_id)


func get_panel(panel_id: String) -> PanelBase:
	return panels_by_id.get(panel_id, null)


func show_dock_preview(side: String, visible_flag: bool) -> void:
	var preview: ColorRect = _left_preview if side == "left" else _right_preview
	preview.visible = visible_flag


func _wire_panel(panel: PanelBase) -> void:
	panel.focus_requested.connect(func(p: PanelBase) -> void:
		focus_panel(p)
	)
	panel.close_requested.connect(func(p: PanelBase) -> void:
		close_panel(p.panel_id)
	)


func _bind_divider(divider: ColorRect, side: String) -> void:
	divider.mouse_filter = Control.MOUSE_FILTER_STOP
	divider.gui_input.connect(func(event: InputEvent) -> void:
		if event is InputEventMouseButton:
			var mb: InputEventMouseButton = event as InputEventMouseButton
			if mb.button_index != MOUSE_BUTTON_LEFT:
				return
			_dragging_master_boundary = mb.pressed and _divider_visible(side)
			if _dragging_master_boundary:
				get_viewport().set_input_as_handled()
		elif _dragging_master_boundary and event is InputEventMouseMotion:
			var motion: InputEventMouseMotion = event as InputEventMouseMotion
			_update_master_ratio_from_pointer(side, motion.position.x)
			get_viewport().set_input_as_handled()
	)


func _input(event: InputEvent) -> void:
	if not _dragging_master_boundary:
		return
	if event is InputEventMouseMotion:
		var motion: InputEventMouseMotion = event as InputEventMouseMotion
		var side: String = "left" if _left_divider.visible else "right"
		_update_master_ratio_from_pointer(side, motion.position.x)
		get_viewport().set_input_as_handled()
	elif event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if not mb.pressed and mb.button_index == MOUSE_BUTTON_LEFT:
			_dragging_master_boundary = false
			get_viewport().set_input_as_handled()


func _refresh_layout() -> void:
	_last_layout = _compute_regions()
	_apply_regions(_last_layout)
	master_region_changed.emit(_last_layout.get("master", Rect2()))


func _compute_regions() -> Dictionary:
	var viewport_rect: Rect2 = get_viewport_rect()
	var left_active: bool = not left_tree.is_empty()
	var right_active: bool = not right_tree.is_empty()
	if not left_active and not right_active:
		return {
			"master": viewport_rect,
			"left": Rect2(viewport_rect.position, Vector2.ZERO),
			"right": Rect2(viewport_rect.end, Vector2.ZERO),
		}
	var master_width: float = viewport_rect.size.x * clampf(master_ratio, MASTER_RATIO_MIN, MASTER_RATIO_MAX)
	var side_width_total: float = viewport_rect.size.x - master_width
	var left_width: float = 0.0
	var right_width: float = 0.0
	if left_active and right_active:
		left_width = side_width_total * 0.5
		right_width = side_width_total * 0.5
	elif left_active:
		left_width = side_width_total
	else:
		right_width = side_width_total
	var master_rect: Rect2 = Rect2(Vector2(left_width, 0.0), Vector2(master_width, viewport_rect.size.y))
	var right_x: float = master_rect.position.x + master_rect.size.x
	return {
		"master": master_rect,
		"left": Rect2(Vector2.ZERO, Vector2(left_width, viewport_rect.size.y)),
		"right": Rect2(Vector2(right_x, 0.0), Vector2(right_width, viewport_rect.size.y)),
	}


func _apply_regions(regions: Dictionary) -> void:
	var left_rect: Rect2 = regions.get("left", Rect2())
	var right_rect: Rect2 = regions.get("right", Rect2())
	_left_zone.position = left_rect.position
	_left_zone.size = left_rect.size
	_right_zone.position = right_rect.position
	_right_zone.size = right_rect.size
	_left_preview.position = Vector2.ZERO
	_left_preview.size = Vector2(minf(DOCK_PREVIEW_WIDTH, size.x), size.y)
	_right_preview.position = Vector2(maxf(size.x - DOCK_PREVIEW_WIDTH, 0.0), 0.0)
	_right_preview.size = Vector2(minf(DOCK_PREVIEW_WIDTH, size.x), size.y)
	_left_divider.visible = left_rect.size.x > 0.0
	_right_divider.visible = right_rect.size.x > 0.0
	if _left_divider.visible:
		_left_divider.position = Vector2(left_rect.size.x - (DIVIDER_WIDTH * 0.5), 0.0)
		_left_divider.size = Vector2(DIVIDER_WIDTH, left_rect.size.y)
	if _right_divider.visible:
		_right_divider.position = Vector2(right_rect.position.x - (DIVIDER_WIDTH * 0.5), 0.0)
		_right_divider.size = Vector2(DIVIDER_WIDTH, right_rect.size.y)


func _update_master_ratio_from_pointer(side: String, pointer_x: float) -> void:
	var viewport_width: float = maxf(get_viewport_rect().size.x, 1.0)
	var normalized: float
	if side == "left":
		normalized = 1.0 - (pointer_x / viewport_width)
	else:
		normalized = pointer_x / viewport_width
	master_ratio = _snapped_master_ratio(clampf(normalized, MASTER_RATIO_MIN, MASTER_RATIO_MAX))
	_refresh_layout()


func _snapped_master_ratio(value: float) -> float:
	for snap_point: float in MASTER_RATIO_SNAP_POINTS:
		if absf(value - snap_point) <= 0.03:
			return snap_point
	return value


func _divider_visible(side: String) -> bool:
	return _left_divider.visible if side == "left" else _right_divider.visible


func _default_floating_position(index: int) -> Vector2:
	var viewport_size: Vector2 = get_viewport_rect().size
	var base: Vector2 = viewport_size * Vector2(0.56, 0.16)
	var offset: Vector2 = Vector2(28.0 * index, 24.0 * index)
	return base + offset
