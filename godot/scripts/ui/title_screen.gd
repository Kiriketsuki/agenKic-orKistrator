class_name TitleScreen
extends Control
## TitleScreen — boot front door for OrKi (F1).
##
## Splits into a left wordmark half and a right menu half. The menu supports
## keyboard (arrow keys plus Enter) and mouse activation, with a teal focus
## highlight on the active entry. ENTER THE TOWER fades to black and loads
## the tower scene. GRIMOIRE opens a placeholder settings panel with a back
## action. DEPART quits the app. The bridge status rune polls
## BridgeManager's connection state and shows a teal "connected" state or a
## dim "disconnected" state.

const MAIN_SCENE_PATH: String = "res://scenes/main.tscn"
const VERSION_FILE_PATH: String = "res://../VERSION"
const FADE_DURATION: float = 0.6
const RUNE_POLL_INTERVAL: float = 0.5

const COLOR_GOLD: Color = Color(0.788, 0.635, 0.153, 1.0)      # #c9a227
const COLOR_TEAL: Color = Color(0.435, 0.722, 0.663, 1.0)      # #6fb8a8
const COLOR_PARCHMENT: Color = Color(0.910, 0.835, 0.627, 1.0) # #e8d5a0
const COLOR_BG: Color = Color(0.051, 0.039, 0.078, 1.0)        # #0d0a14
const COLOR_DIM_GRAY: Color = Color(0.4, 0.4, 0.42, 1.0)

enum TitleMenuEntry { ENTER, GRIMOIRE, DEPART }

@onready var _menu_entries: Array[Button] = [
	%EnterButton,
	%GrimoireButton,
	%DepartButton,
]
@onready var _fade_rect: ColorRect = %FadeRect
@onready var _version_label: Label = %VersionLabel
@onready var _rune_icon: ColorRect = %RuneIcon
@onready var _rune_label: Label = %RuneLabel
@onready var _grimoire_panel: Control = %GrimoirePanel
@onready var _grimoire_back_button: Button = %GrimoireBackButton
@onready var _menu_root: Control = %MenuRoot

var _focused_index: int = 0
var _rune_poll_timer: float = 0.0
var _transitioning: bool = false


func _ready() -> void:
	for i: int in range(_menu_entries.size()):
		var entry: Button = _menu_entries[i]
		entry.focus_entered.connect(_on_entry_focus_entered.bind(i))
		entry.pressed.connect(_on_entry_pressed.bind(i))

	_grimoire_back_button.pressed.connect(_close_grimoire)

	_version_label.text = _read_version_label()
	_fade_rect.color = Color(0, 0, 0, 0)
	_fade_rect.visible = false
	_grimoire_panel.visible = false

	_set_focus(0)
	_update_rune(BridgeManager.get_connection_state_name())


func _process(delta: float) -> void:
	_rune_poll_timer += delta
	if _rune_poll_timer < RUNE_POLL_INTERVAL:
		return
	_rune_poll_timer = 0.0
	_update_rune(BridgeManager.get_connection_state_name())


func _unhandled_input(event: InputEvent) -> void:
	if _transitioning or _grimoire_panel.visible:
		return
	if event.is_action_pressed("ui_down"):
		_move_focus(1)
		get_viewport().set_input_as_handled()
	elif event.is_action_pressed("ui_up"):
		_move_focus(-1)
		get_viewport().set_input_as_handled()
	elif event.is_action_pressed("ui_accept"):
		_activate_entry(_focused_index)
		get_viewport().set_input_as_handled()


func _move_focus(delta: int) -> void:
	var count: int = _menu_entries.size()
	_set_focus((_focused_index + delta + count) % count)


func _set_focus(index: int) -> void:
	_focused_index = index
	_menu_entries[index].grab_focus()


func _on_entry_focus_entered(index: int) -> void:
	_focused_index = index


func _on_entry_pressed(index: int) -> void:
	_activate_entry(index)


func _activate_entry(index: int) -> void:
	match index:
		TitleMenuEntry.ENTER:
			_activate_enter()
		TitleMenuEntry.GRIMOIRE:
			_open_grimoire()
		TitleMenuEntry.DEPART:
			_activate_depart()


func _activate_enter() -> void:
	if _transitioning:
		return
	_transitioning = true
	_fade_rect.visible = true
	var tween: Tween = create_tween()
	tween.tween_property(_fade_rect, "color:a", 1.0, FADE_DURATION)
	tween.finished.connect(_on_fade_complete)


func _on_fade_complete() -> void:
	get_tree().change_scene_to_file(MAIN_SCENE_PATH)


func _activate_depart() -> void:
	get_tree().quit()


func _open_grimoire() -> void:
	_grimoire_panel.visible = true
	_menu_root.visible = false
	_grimoire_back_button.grab_focus()


func _close_grimoire() -> void:
	_grimoire_panel.visible = false
	_menu_root.visible = true
	_set_focus(_focused_index)


func _update_rune(state_name: String) -> void:
	var connected: bool = state_name == "connected"
	_rune_icon.color = COLOR_TEAL if connected else COLOR_DIM_GRAY
	_rune_label.text = "orchestrator: connected" if connected else "orchestrator: offline"
	_rune_label.modulate = COLOR_TEAL if connected else COLOR_DIM_GRAY


## Reads the repo VERSION file for the boot-screen label. Falls back to
## "dev" when the file cannot be read, which keeps the title screen usable
## from a build layout where the VERSION file is not reachable.
func _read_version_label() -> String:
	var file: FileAccess = FileAccess.open(VERSION_FILE_PATH, FileAccess.READ)
	if file == null:
		return "dev"
	var version: String = file.get_as_text().strip_edges()
	file.close()
	if version.is_empty():
		return "dev"
	return "v" + version


## Pure focus-order helper for headless tests: given the current focused
## index, the menu entry count, and a direction (+1/-1), returns the next
## focused index, wrapping at both ends. Mirrors _move_focus/_set_focus
## without touching any live Control, so tests/title_screen_focus_test.gd
## can exercise the logic without a running Godot instance's viewport.
static func next_focus_index(current: int, count: int, direction: int) -> int:
	return (current + direction + count) % count
