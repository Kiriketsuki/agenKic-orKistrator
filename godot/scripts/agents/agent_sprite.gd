## AgentSprite — drop-in AnimatedSprite2D wrapper for the T22 class sheets.
## Attach to an AnimatedSprite2D, set character_class, call set_anim_state().
class_name AgentSprite
extends AnimatedSprite2D

enum CharacterClass { ALCHEMIST, SCRIBE, ARCHMAGE, WARDKEEPER, LIBRARIAN, ENCHANTER, APPRENTICE }
enum AnimState { IDLE, WALKING, WORKING, REPORTING, STUNNED }

const CLASS_NAMES := ["alchemist", "scribe", "archmage", "wardkeeper", "librarian", "enchanter", "apprentice"]
const STATE_NAMES := ["idle", "walking", "working", "reporting", "stunned"]

## Accent colors used by nameplates / selection glows (from the HTML demos).
const CLASS_COLORS := {
	CharacterClass.ALCHEMIST: Color("#d98c00"),
	CharacterClass.SCRIBE: Color("#5999f2"),
	CharacterClass.ARCHMAGE: Color("#bf33e6"),
	CharacterClass.WARDKEEPER: Color("#40bf4d"),
	CharacterClass.LIBRARIAN: Color("#a66626"),
	CharacterClass.ENCHANTER: Color("#1ad9cc"),
	CharacterClass.APPRENTICE: Color("#a6a6bf"),
}

@export var character_class: CharacterClass = CharacterClass.APPRENTICE:
	set(value):
		character_class = value
		_load_frames()

func _ready() -> void:
	texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	_load_frames()
	play("idle")

func _load_frames() -> void:
	var path := "res://assets/sprites/%s_frames.tres" % CLASS_NAMES[character_class]
	sprite_frames = load(path)

func set_anim_state(state: AnimState) -> void:
	play(STATE_NAMES[state])

func accent_color() -> Color:
	return CLASS_COLORS[character_class]
