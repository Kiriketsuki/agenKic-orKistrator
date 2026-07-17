extends Node2D
## AgentCharacter — visual representation of an orchestrator agent on a tower floor.
## Phase 1 uses placeholder colored rectangles; T22 will replace these with real sprites.

class_name AgentCharacter

signal character_clicked(agent_id: String)
signal character_right_clicked(agent_id: String)
signal character_hovered(agent_id: String)
signal character_unhovered(agent_id: String)

enum CharacterClass {
	ALCHEMIST,
	SCRIBE,
	ARCHMAGE,
	WARDKEEPER,
	LIBRARIAN,
	ENCHANTER,
	APPRENTICE,
}

enum AnimState {
	IDLE,
	WALKING,
	WORKING,
	REPORTING,
	STUNNED,
}

const CLASS_COLORS: Dictionary = {
	CharacterClass.ALCHEMIST:  Color(0.85, 0.55, 0.0,  1.0),
	CharacterClass.SCRIBE:     Color(0.35, 0.6,  0.95, 1.0),
	CharacterClass.ARCHMAGE:   Color(0.75, 0.2,  0.9,  1.0),
	CharacterClass.WARDKEEPER: Color(0.25, 0.75, 0.3,  1.0),
	CharacterClass.LIBRARIAN:  Color(0.65, 0.4,  0.15, 1.0),
	CharacterClass.ENCHANTER:  Color(0.1,  0.85, 0.8,  1.0),
	CharacterClass.APPRENTICE: Color(0.65, 0.65, 0.75, 1.0),
}

# T22 design-pack sheets: 16x20 cells, 4 columns x 5 rows, row order matches
# AnimState (idle, walking, working, reporting, stunned). See
# assets/asset_manifest.json.
const CLASS_SHEETS: Dictionary = {
	CharacterClass.ALCHEMIST:  preload("res://assets/sprites/alchemist.png"),
	CharacterClass.SCRIBE:     preload("res://assets/sprites/scribe.png"),
	CharacterClass.ARCHMAGE:   preload("res://assets/sprites/archmage.png"),
	CharacterClass.WARDKEEPER: preload("res://assets/sprites/wardkeeper.png"),
	CharacterClass.LIBRARIAN:  preload("res://assets/sprites/librarian.png"),
	CharacterClass.ENCHANTER:  preload("res://assets/sprites/enchanter.png"),
	CharacterClass.APPRENTICE: preload("res://assets/sprites/apprentice.png"),
}

const SHEET_FRAME_SIZE: Vector2i = Vector2i(16, 20)
const SHEET_COLUMNS: int = 4
const SHEET_ANIMS: Array = [
	["idle", 4.0], ["walking", 8.0], ["working", 8.0],
	["reporting", 6.0], ["stunned", 6.0],
]

const ANIM_BY_STATE: Dictionary = {
	AnimState.IDLE:      "idle",
	AnimState.WALKING:   "walking",
	AnimState.WORKING:   "working",
	AnimState.REPORTING: "reporting",
	AnimState.STUNNED:   "stunned",
}

## SpriteFrames per class, built once and shared by every character instance.
static var _frames_cache: Dictionary = {}

const CLASS_LABELS: Dictionary = {
	CharacterClass.ALCHEMIST:  "ALC",
	CharacterClass.SCRIBE:     "SCR",
	CharacterClass.ARCHMAGE:   "ARC",
	CharacterClass.WARDKEEPER: "WRD",
	CharacterClass.LIBRARIAN:  "LIB",
	CharacterClass.ENCHANTER:  "ENC",
	CharacterClass.APPRENTICE: "APP",
}

const CLASS_BY_NAME: Dictionary = {
	"alchemist":  CharacterClass.ALCHEMIST,
	"scribe":     CharacterClass.SCRIBE,
	"archmage":   CharacterClass.ARCHMAGE,
	"wardkeeper": CharacterClass.WARDKEEPER,
	"librarian":  CharacterClass.LIBRARIAN,
	"enchanter":  CharacterClass.ENCHANTER,
	"apprentice": CharacterClass.APPRENTICE,
}

const STATE_BY_NAME: Dictionary = {
	"idle":      AnimState.IDLE,
	"assigned":  AnimState.WALKING,
	"working":   AnimState.WORKING,
	"reporting": AnimState.REPORTING,
	"crashed":   AnimState.STUNNED,
}

# Multiplicative tints applied on top of the class base color.
const STATE_TINTS: Dictionary = {
	AnimState.IDLE:      Color(1.0,  1.0,  1.0,  1.0),
	AnimState.WALKING:   Color(0.8,  0.9,  1.0,  1.0),
	AnimState.WORKING:   Color(0.9,  0.95, 0.6,  1.0),
	AnimState.REPORTING: Color(0.9,  0.85, 0.4,  1.0),
	AnimState.STUNNED:   Color(0.5,  0.3,  0.3,  0.8),
}

const FLOATING_RUNE_SCENE: PackedScene = preload("res://scenes/floating_rune.tscn")
const MAX_RUNES: int = 5

## Set by the owner (FloorScene) before add_child so it is ready in _ready().
var agent_id: String = ""

var _character_class: CharacterClass = CharacterClass.APPRENTICE
var _anim_state: AnimState = AnimState.IDLE
var _pulse_time: float = 0.0
var _provider: String = ""
var _active_runes: Array[Node2D] = []

@onready var _body: AnimatedSprite2D = $Body
@onready var _class_label: Label = $ClassLabel
@onready var _click_area: Area2D = $ClickArea


func _ready() -> void:
	_class_label.add_theme_font_size_override("font_size", 7)
	_click_area.input_event.connect(_on_area_input_event)
	_click_area.mouse_entered.connect(func() -> void: character_hovered.emit(agent_id))
	_click_area.mouse_exited.connect(func() -> void: character_unhovered.emit(agent_id))
	_apply_class_visuals()
	_apply_state_tint()


func _process(delta: float) -> void:
	if _anim_state != AnimState.WORKING:
		return
	_pulse_time += delta * 4.0
	var pulse: float = 0.85 + sin(_pulse_time) * 0.15
	_body.modulate = STATE_TINTS[AnimState.WORKING] * Color(pulse, pulse, pulse, 1.0)


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

func set_character_class(class_name_str: String) -> void:
	_character_class = CLASS_BY_NAME.get(class_name_str, CharacterClass.APPRENTICE)
	if is_inside_tree():
		_apply_class_visuals()
		_apply_state_tint()


func set_animation_state(state_name: String) -> void:
	var new_state: AnimState = STATE_BY_NAME.get(state_name, AnimState.IDLE)
	if new_state == _anim_state:
		return
	_anim_state = new_state
	_pulse_time = 0.0
	if is_inside_tree():
		_apply_state_tint()


func get_anim_state() -> AnimState:
	return _anim_state


func set_provider(p: String) -> void:
	_provider = p


func receive_output(chunk: BridgeData.AgentOutputChunk) -> void:
	if _anim_state == AnimState.IDLE:
		return
	var result: Dictionary = RuneFilter.process(chunk)
	if not result.get(&"show", false):
		return
	var provider: String = chunk.provider if not chunk.provider.is_empty() else _provider
	if provider.is_empty():
		provider = "unknown"
	# Enforce rune cap — accelerate oldest on overflow.
	if _active_runes.size() >= MAX_RUNES:
		var oldest: Node2D = _active_runes[0]
		_active_runes.remove_at(0)
		if is_instance_valid(oldest) and oldest.has_method("accelerate_fade"):
			oldest.accelerate_fade()
	var rune: FloatingRune = FLOATING_RUNE_SCENE.instantiate() as FloatingRune
	add_child(rune)
	rune.position = Vector2(0.0, -34.0)
	rune.setup(result[&"text"], result[&"keywords"], provider)
	_active_runes.append(rune)
	rune.tree_exiting.connect(func() -> void:
		_active_runes.erase(rune)
	)


## Fade out over 0.4 s then free self. Called by TowerManager on agent.deregistered.
## Reparents self to the floor root before tweening so concurrent _rebuild_interior
## calls on _agent_slots_node do not queue_free this node mid-fade.
func play_exit_animation() -> void:
	var floor_root: Node = get_parent().get_parent()
	reparent(floor_root)
	var tween: Tween = create_tween()
	tween.tween_property(self, "modulate:a", 0.0, 0.4)
	tween.tween_callback(queue_free)


# ---------------------------------------------------------------------------
# Private helpers
# ---------------------------------------------------------------------------

func _apply_class_visuals() -> void:
	_body.sprite_frames = _frames_for_class(_character_class)
	_body.play(ANIM_BY_STATE[_anim_state])
	_class_label.text = CLASS_LABELS[_character_class]


func _apply_state_tint() -> void:
	if _anim_state != AnimState.WORKING:
		_body.modulate = STATE_TINTS[_anim_state]
	if _body.sprite_frames != null:
		_body.play(ANIM_BY_STATE[_anim_state])


## Builds (or returns cached) SpriteFrames for a class sheet: one animation
## per AnimState row, 4 frames per row, per-state fps from the manifest.
static func _frames_for_class(char_class: CharacterClass) -> SpriteFrames:
	if _frames_cache.has(char_class):
		return _frames_cache[char_class]
	var sheet: Texture2D = CLASS_SHEETS[char_class]
	var frames := SpriteFrames.new()
	frames.remove_animation("default")
	for row: int in range(SHEET_ANIMS.size()):
		var anim_name: String = SHEET_ANIMS[row][0]
		var fps: float = SHEET_ANIMS[row][1]
		frames.add_animation(anim_name)
		frames.set_animation_speed(anim_name, fps)
		frames.set_animation_loop(anim_name, true)
		for col: int in range(SHEET_COLUMNS):
			var atlas := AtlasTexture.new()
			atlas.atlas = sheet
			atlas.region = Rect2(
				col * SHEET_FRAME_SIZE.x, row * SHEET_FRAME_SIZE.y,
				SHEET_FRAME_SIZE.x, SHEET_FRAME_SIZE.y
			)
			frames.add_frame(anim_name, atlas)
	_frames_cache[char_class] = frames
	return frames


func _on_area_input_event(_viewport: Node, event: InputEvent, _shape_idx: int) -> void:
	if event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed and mb.button_index == MOUSE_BUTTON_LEFT:
			character_clicked.emit(agent_id)
		elif mb.pressed and mb.button_index == MOUSE_BUTTON_RIGHT:
			character_right_clicked.emit(agent_id)
