class_name PowerFlyout
extends Control
## PowerFlyout — Power Controls (F4) flyout for the "power" orb. Registers
## itself under OrbFlock the same way GrimoireFlyout registers under
## "grimoire" (see orb_flock.gd's register_flyout / main.tscn).
##
## Four actions:
##   BANISH ALL AGENTS  — despawns every live agent, behind a confirm dialog.
##   RESTART ORCHESTRATOR — restarts the server process, behind a confirm
##                           dialog.
##   SETTINGS            — switches the flock to the Grimoire orb and opens
##                          its sigil config page (F3), rather than
##                          duplicating that page here.
##   QUIT TO TITLE        — returns to the title screen.
##
## Toast feedback mirrors agent_context_menu.gd's command_succeeded/
## command_failed pattern, filtered to the paths this flyout's actions hit.

const TITLE_SCENE_PATH: String = "res://scenes/title_screen.tscn"

const SUCCESS_COLOR: Color = Color(0.16, 0.4, 0.16, 1.0)
const ERROR_COLOR: Color = Color(0.55, 0.12, 0.1, 1.0)
const INK_COLOR: Color = Color(0.22, 0.16, 0.10, 1.0)

var _bridge: Node = null

var _banish_all_confirm: ConfirmationDialog = null
var _restart_confirm: ConfirmationDialog = null

var _toast: Label = null
var _toast_tween: Tween = null


func _ready() -> void:
	custom_minimum_size = Vector2(520.0, 400.0)
	size = Vector2(520.0, 400.0)

	_bridge = get_node_or_null("/root/BridgeManager")

	var box := VBoxContainer.new()
	box.set_anchors_preset(Control.PRESET_FULL_RECT)
	box.add_theme_constant_override("separation", 8)
	add_child(box)

	var banish_button := Button.new()
	banish_button.text = "BANISH ALL AGENTS"
	banish_button.pressed.connect(_on_banish_all_pressed)
	box.add_child(banish_button)

	var restart_button := Button.new()
	restart_button.text = "RESTART ORCHESTRATOR"
	restart_button.pressed.connect(_on_restart_pressed)
	box.add_child(restart_button)

	var settings_button := Button.new()
	settings_button.text = "SETTINGS"
	settings_button.pressed.connect(_on_settings_pressed)
	box.add_child(settings_button)

	var quit_button := Button.new()
	quit_button.text = "QUIT TO TITLE"
	quit_button.pressed.connect(_on_quit_pressed)
	box.add_child(quit_button)

	_banish_all_confirm = ConfirmationDialog.new()
	_banish_all_confirm.name = "BanishAllConfirm"
	_banish_all_confirm.title = "Banish all agents"
	_banish_all_confirm.dialog_text = "This despawns every live agent and drops their tasks. Continue?"
	_banish_all_confirm.confirmed.connect(_on_banish_all_confirmed)
	add_child(_banish_all_confirm)

	_restart_confirm = ConfirmationDialog.new()
	_restart_confirm.name = "RestartConfirm"
	_restart_confirm.title = "Restart orchestrator"
	_restart_confirm.dialog_text = "This restarts the orchestrator process. The tower reconnects on its own. Continue?"
	_restart_confirm.confirmed.connect(_on_restart_confirmed)
	add_child(_restart_confirm)

	if _bridge != null:
		if _bridge.has_signal("command_succeeded"):
			_bridge.connect("command_succeeded", _on_command_succeeded)
		if _bridge.has_signal("command_failed"):
			_bridge.connect("command_failed", _on_command_failed)


func _on_banish_all_pressed() -> void:
	_banish_all_confirm.popup_centered()


## Banish-all iterates a snapshot of the agent list (spec risk: "Banish-all
## racing new spawns"). get_registered_agents() already returns a fresh
## Array built from the live dictionary each call, so a spawn that lands
## after this loop starts is not in the snapshot and survives, per spec.
func _on_banish_all_confirmed() -> void:
	if _bridge == null or not _bridge.has_method("get_registered_agents") or not _bridge.has_method("despawn_agent"):
		return
	var agents: Array = _bridge.get_registered_agents()
	for agent_id: String in banish_all_ids(agents):
		_bridge.despawn_agent(agent_id)


## Pure extraction step for banish-all: turns a snapshot Array of
## BridgeData.AgentData into the ordered list of ids to despawn. Delegates
## to PowerFlyoutMath, an autoload-free script, so the snapshot behavior is
## testable headlessly without a live BridgeManager (see
## tests/power_flyout_test.gd) and without preloading this file's own
## autoload-bound OrbFlock reference.
static func banish_all_ids(agents: Array) -> Array[String]:
	return PowerFlyoutMath.banish_all_ids(agents)


func _on_restart_pressed() -> void:
	_restart_confirm.popup_centered()


func _on_restart_confirmed() -> void:
	if _bridge != null and _bridge.has_method("restart_orchestrator"):
		_bridge.restart_orchestrator()


## Switches the flock to the Grimoire orb and opens its sigil config page
## (F3) rather than hosting a second copy of that page here. PowerFlyout is
## reparented under OrbFlock's flyout host (see orb_flock.gd register_flyout),
## so the flock sits two levels up: get_parent() is the flyout host,
## get_parent().get_parent() is OrbFlock itself.
func _on_settings_pressed() -> void:
	var host: Node = get_parent()
	if host == null:
		return
	var flock: Node = host.get_parent()
	if flock == null:
		return
	if flock.has_method("switch_to"):
		flock.switch_to("grimoire")
	var grimoire: Node = flock.get_node_or_null("GrimoireFlyout")
	if grimoire != null and grimoire.has_method("open_config_page"):
		grimoire.open_config_page()


## OrbFlock.suspend_hotkeys is a static var, so it survives
## change_scene_to_file. This flyout only reaches QUIT TO TITLE while the
## flock is OPEN (suspend_hotkeys true), so without clearing it here the
## title scene loads with hotkeys gated forever (panel_manager.gd's
## _handle_hotkeys checks the same static var). See orb_flock.gd's own
## _ready() reset for the matching defense on the other side of the swap.
func _on_quit_pressed() -> void:
	OrbFlock.suspend_hotkeys = false
	get_tree().change_scene_to_file(TITLE_SCENE_PATH)


func _on_command_succeeded(path: String, _code: int, _body: String) -> void:
	if is_despawn_path(path):
		_toast_message("Agent banished", SUCCESS_COLOR)
	elif is_restart_path(path):
		_toast_message("Orchestrator restarting", SUCCESS_COLOR)


func _on_command_failed(path: String, _code: int, reason: String) -> void:
	if is_despawn_path(path) or is_restart_path(path):
		_toast_message(reason, ERROR_COLOR)


## Path-matching helpers, delegating to PowerFlyoutMath (autoload-free) so
## the toast wiring is testable headlessly without a live
## command_succeeded/command_failed emission (see tests/power_flyout_test.gd)
## and without preloading this file's own autoload-bound OrbFlock reference.
static func is_despawn_path(path: String) -> bool:
	return PowerFlyoutMath.is_despawn_path(path)


static func is_restart_path(path: String) -> bool:
	return PowerFlyoutMath.is_restart_path(path)


func _toast_message(text: String, color: Color) -> void:
	_ensure_toast()
	if _toast == null:
		return
	_toast.text = text
	_toast.add_theme_color_override("font_color", color)
	_toast.modulate.a = 1.0
	_toast.visible = true

	if _toast_tween != null and _toast_tween.is_valid():
		_toast_tween.kill()
	_toast_tween = create_tween()
	_toast_tween.tween_interval(1.2)
	_toast_tween.tween_property(_toast, "modulate:a", 0.0, 0.4)
	_toast_tween.tween_callback(func() -> void:
		if is_instance_valid(_toast):
			_toast.visible = false
	)


func _ensure_toast() -> void:
	if _toast != null and is_instance_valid(_toast):
		return
	_toast = Label.new()
	_toast.name = "PowerFlyoutToast"
	_toast.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_toast.add_theme_color_override("font_color", INK_COLOR)
	_toast.position = Vector2(0.0, 176.0)
	_toast.visible = false
	add_child(_toast)
