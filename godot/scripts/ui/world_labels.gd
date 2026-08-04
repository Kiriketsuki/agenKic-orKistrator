extends Control
## WorldLabels — floor names and agent nameplates, drawn on UILayer at window
## resolution and projected onto the world every frame.
##
## PARITY (2026-08-04) — the tower world renders at an integer camera zoom, so
## any vector font placed in world space is magnified 4x to 8x and turns to
## mush. This node keeps the text at window resolution instead. It reads the
## anchor through get_global_transform_with_canvas(), which already folds in the
## camera transform and its zoom, so no zoom correction is applied here.
##
## Read-only toward the tower: it calls the public TowerManager API and walks
## FloorsContainer without mutating anything.

const FOCUS_COLOR: Color = Color("#c8a84e")
const DIM_COLOR: Color = Color("#8a9a7a")
const FLOOR_LABEL_SIZE: int = 14
const NAMEPLATE_TEXT_SIZE: int = 11
## Screen-space offsets, in window pixels.
const FLOOR_LABEL_OFFSET: Vector2 = Vector2(-140.0, -40.0)
const NAMEPLATE_OFFSET: Vector2 = Vector2(0.0, -26.0)
const NAMEPLATE_SIZE: Vector2 = Vector2(96.0, 20.0)

const NAMEPLATE_TEXTURE: Texture2D = preload("res://assets/ui/nameplate_frame.png")

@export var tower_path: NodePath = NodePath("/root/Main/Tower")

var _tower: Node = null
var _font: Font = null
var _floor_labels: Array[Label] = []
## agent_id -> the NinePatchRect plate holding that agent's Label.
var _nameplates: Dictionary = {}


func _ready() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE
	set_anchors_preset(Control.PRESET_FULL_RECT)
	_font = _make_font()
	_tower = get_node_or_null(tower_path)
	if _tower == null:
		set_process(false)
		return
	if _tower.has_signal("floors_changed"):
		_tower.connect("floors_changed", _rebuild_floor_labels)
	_rebuild_floor_labels()


## Fira Code where the system has it, plain monospace otherwise.
func _make_font() -> Font:
	var font := SystemFont.new()
	font.font_names = PackedStringArray(["Fira Code", "monospace"])
	return font


func _process(_delta: float) -> void:
	if _tower == null or not is_instance_valid(_tower):
		return
	_update_floor_labels()
	_update_nameplates()


# --- floor labels ---

func _rebuild_floor_labels() -> void:
	for label: Label in _floor_labels:
		if is_instance_valid(label):
			label.queue_free()
	_floor_labels.clear()
	var infos: Array = _tower.get_floor_infos()
	for info: Dictionary in infos:
		var label := Label.new()
		label.text = String(info.get("label", "")).to_upper()
		label.mouse_filter = Control.MOUSE_FILTER_IGNORE
		label.add_theme_font_override("font", _font)
		label.add_theme_font_size_override("font_size", FLOOR_LABEL_SIZE)
		label.add_theme_constant_override("outline_size", 4)
		label.add_theme_color_override("font_outline_color", Color(0.05, 0.05, 0.07, 0.9))
		label.add_theme_constant_override("line_spacing", 2)
		add_child(label)
		_floor_labels.append(label)


func _update_floor_labels() -> void:
	var container: Node = _tower.get_node_or_null("FloorsContainer")
	if container == null:
		return
	var floors: Array = container.get_children()
	var focus: int = _tower.get_focus_index()
	for i: int in range(_floor_labels.size()):
		var label: Label = _floor_labels[i]
		if i >= floors.size() or not (floors[i] is Node2D):
			label.visible = false
			continue
		# Mirrors FloorScene.set_show_interior: only the focused floor and its
		# two neighbours carry readable text.
		if absi(i - focus) > 1:
			label.visible = false
			continue
		label.visible = true
		label.add_theme_color_override("font_color", FOCUS_COLOR if i == focus else DIM_COLOR)
		var origin: Vector2 = (floors[i] as Node2D).get_global_transform_with_canvas().origin
		label.position = origin + FLOOR_LABEL_OFFSET


# --- agent nameplates ---

func _update_nameplates() -> void:
	var container: Node = _tower.get_node_or_null("FloorsContainer")
	if container == null:
		return
	var floors: Array = container.get_children()
	var focus: int = _tower.get_focus_index()
	var seen: Dictionary = {}
	if focus >= 0 and focus < floors.size():
		var slots: Node = floors[focus].get_node_or_null("AgentSlots")
		if slots != null and slots.visible:
			for child: Node in slots.get_children():
				if child is AgentCharacter:
					_place_nameplate(child as AgentCharacter)
					seen[(child as AgentCharacter).agent_id] = true
	for agent_id: String in _nameplates.keys():
		if not seen.has(agent_id):
			var plate: Node = _nameplates[agent_id]
			if is_instance_valid(plate):
				plate.queue_free()
			_nameplates.erase(agent_id)


func _place_nameplate(char_node: AgentCharacter) -> void:
	var plate: NinePatchRect = _nameplates.get(char_node.agent_id, null)
	if plate == null or not is_instance_valid(plate):
		plate = _make_nameplate()
		add_child(plate)
		_nameplates[char_node.agent_id] = plate
	var label: Label = plate.get_child(0) as Label
	var class_id: int = char_node.get("_character_class")
	label.text = String(AgentCharacter.CLASS_LABELS.get(class_id, "APP"))
	label.add_theme_color_override("font_color", _accent_for(class_id))
	var origin: Vector2 = char_node.get_global_transform_with_canvas().origin
	plate.position = origin + NAMEPLATE_OFFSET - Vector2(NAMEPLATE_SIZE.x / 2.0, 0.0)


## Class accent, taken from the character's own palette table (PORTING.md).
func _accent_for(class_id: int) -> Color:
	return AgentCharacter.CLASS_COLORS.get(class_id, FOCUS_COLOR)


func _make_nameplate() -> NinePatchRect:
	var plate := NinePatchRect.new()
	plate.texture = NAMEPLATE_TEXTURE
	plate.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	plate.patch_margin_left = 6
	plate.patch_margin_top = 4
	plate.patch_margin_right = 6
	plate.patch_margin_bottom = 4
	plate.size = NAMEPLATE_SIZE
	plate.mouse_filter = Control.MOUSE_FILTER_IGNORE
	var label := Label.new()
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	label.set_anchors_preset(Control.PRESET_FULL_RECT)
	label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	label.add_theme_font_override("font", _font)
	label.add_theme_font_size_override("font_size", NAMEPLATE_TEXT_SIZE)
	plate.add_child(label)
	return plate
