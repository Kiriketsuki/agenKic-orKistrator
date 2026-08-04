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
const NAMEPLATE_TEXT_SIZE: int = 22
const NAMEPLATE_SIZE: Vector2 = Vector2(192.0, 40.0)
## Half-height of a character sprite, in art px (sprite asset space, not the
## design-canvas UI space). The plate hangs above the head, so its offset
## scales with the camera zoom instead of a fixed pixel count that lands on
## the face at high zoom. The character sprite asset did not change size
## when project.godot's design canvas doubled, so this constant is not
## doubled.
const CHARACTER_HALF_HEIGHT: float = 12.0
## Screen-space gap between the sprite head and the nameplate, in design-
## canvas pixels. Doubled alongside every other screen-space UI size.
const NAMEPLATE_HEAD_GAP: float = 8.0

const NAMEPLATE_TEXTURE: Texture2D = preload("res://assets/ui/nameplate_frame.png")
## Frame tint for agent plates. The floor banner shares the frame texture in
## its native gold, so the plates carry a teal wash to read as a different
## class of label.
const PLATE_FRAME_TINT: Color = Color("#6fb8a8")

@export var tower_path: NodePath = NodePath("/root/Main/Tower")

var _tower: Node = null
var _font: Font = null
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


## Fira Code where the system has it, plain monospace otherwise.
func _make_font() -> Font:
	var font := SystemFont.new()
	font.font_names = PackedStringArray(["Fira Code", "monospace"])
	return font


func _process(_delta: float) -> void:
	if _tower == null or not is_instance_valid(_tower):
		return
	_update_nameplates()


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
	var xf: Transform2D = char_node.get_global_transform_with_canvas()
	var head_clear: float = CHARACTER_HALF_HEIGHT * xf.get_scale().y + NAMEPLATE_HEAD_GAP
	plate.position = xf.origin - Vector2(NAMEPLATE_SIZE.x / 2.0, head_clear + NAMEPLATE_SIZE.y)


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
	# The floor banner uses the same frame texture in its native gold. A teal
	# tint here keeps the small agent plates visually distinct from the banner.
	plate.self_modulate = PLATE_FRAME_TINT
	var label := Label.new()
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	label.set_anchors_preset(Control.PRESET_FULL_RECT)
	label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	label.add_theme_font_override("font", _font)
	label.add_theme_font_size_override("font_size", NAMEPLATE_TEXT_SIZE)
	plate.add_child(label)
	return plate
