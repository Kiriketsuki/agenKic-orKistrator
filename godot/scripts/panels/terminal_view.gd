extends Control
## TerminalView — dark raw-terminal panel mounted into a PanelBase's
## ContentRoot when the panel is in "terminal" mode (the T9 spell scroll's
## "Disenchant" counterpart). Two bodies, picked by a capability fallback
## chain in setup():
##
## 1. LIVE PTY — Linux/macOS, godot-xterm's Terminal+PTY GDExtension classes
##    present, and the bridge is talking to a loopback orchestrator. Attaches
##    directly to the agent's local tmux session (`tmux attach -t agent-<id>`)
##    for live bidirectional input/output.
## 2. CHAT BODY — Windows, extension absent, or non-loopback bridge. A
##    RichTextLabel that polls the orchestrator for a visible-pane snapshot of
##    the agent tmux session and replaces its content on every frame, plus a
##    LineEdit that posts typed text to the agent input endpoint. The user
##    holds a conversation with the interactive CLI this way. Rendering uses
##    the same ANSI-driven pattern as SpellScrollView, through the standard
##    (non-sepia) palette.
##
## godot-xterm classes are referenced only via ClassDB.instantiate(&"...")
## (never `Terminal.new()`/`PTY.new()`/`as Terminal`) so this script parses
## and loads even when the addon is not installed — the project must run
## cleanly without it (identical behavior to the Windows fallback path).

class_name TerminalView

## Seconds between visible-pane snapshot polls while the chat body is mounted.
const SCREEN_POLL_SECONDS: float = 0.7

## Mirrors SpellScrollView.PROVIDER_GLYPHS — kept as a separate copy (not a
## shared const) because the two views are visually independent and each
## may evolve its own badge treatment.
const PROVIDER_GLYPHS: Dictionary = {
	"claude": {"glyph": "✦", "color": Color(0.85, 0.65, 0.35, 1.0)},
	"gemini": {"glyph": "♦", "color": Color(0.5, 0.7, 0.9, 1.0)},
	"openai": {"glyph": "◆", "color": Color(0.8, 0.8, 0.8, 1.0)},
	"ollama": {"glyph": "▲", "color": Color(0.55, 0.85, 0.55, 1.0)},
	"deepseek": {"glyph": "◈", "color": Color(0.8, 0.55, 0.9, 1.0)},
	"": {"glyph": "○", "color": Color(0.7, 0.7, 0.7, 1.0)},
}

@onready var _class_badge: ColorRect = $Header/ClassBadge
@onready var _class_badge_label: Label = $Header/ClassBadge/ClassBadgeLabel
@onready var _name_label: Label = $Header/NameLabel
@onready var _provider_badge: Label = $Header/ProviderBadge
@onready var _state_label: Label = $Header/StateLabel
@onready var _enchant_button: Button = $Header/EnchantButton
@onready var _body_root: MarginContainer = $BodyRoot

var _panel: PanelBase = null
var _bridge: Node = null
var _agent_id: String = ""
var _signals_connected: bool = false

## Live PTY path state — Node/Control typed (not Terminal/PTY) so this script
## stays parseable without the godot-xterm addon installed.
var _pty: Node = null
var _pty_terminal: Control = null
var _live_mode: bool = false
## The exact bound Callable passed to `pty.connect("exited", ...)` for the
## current `_pty`, kept so `_kill_pty()` can disconnect the same Callable
## instance (`_on_pty_exited.bind(pty)` is a distinct object each call —
## `is_connected`/`disconnect` need the matching instance, not a fresh bind).
var _pty_exited_callable: Callable = Callable()

## Chat body state.
var _output_label: RichTextLabel = null
var _input_line: LineEdit = null
var _poll_timer: Timer = null
## True while the pointer sits over the output body, which is the mouse-based
## signal that every keystroke forwards to tmux. See _set_body_hover.
var _body_hover: bool = false


func _ready() -> void:
	_enchant_button.pressed.connect(_on_enchant_pressed)


func _exit_tree() -> void:
	_kill_pty()
	# A view freed mid-hover must not leave the global hotkey gate stuck.
	if _body_hover:
		KeyPassthrough.hover_active = false


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

## Called once by PanelContentRouter right after this view is added to the tree.
func setup(panel: PanelBase, agent_data: BridgeData.AgentData, bridge: Node) -> void:
	_panel = panel
	_bridge = bridge
	_apply_agent(agent_data)
	_mount_body()


## Called by PanelManager when the singleton terminal-mode panel (mirroring
## the scroll singleton) is retargeted to a different agent.
func swap_agent(agent_data: BridgeData.AgentData) -> void:
	_kill_pty()
	_apply_agent(agent_data)
	_mount_body()


# ---------------------------------------------------------------------------
# Header
# ---------------------------------------------------------------------------

func _apply_agent(agent_data: BridgeData.AgentData) -> void:
	if agent_data == null:
		_agent_id = ""
		_name_label.text = "Unknown Agent"
		_state_label.text = ""
		_class_badge.color = Color(0.5, 0.5, 0.5, 1.0)
		_class_badge_label.text = "?"
		_provider_badge.text = ""
		if _panel != null:
			_panel.set_panel_title("Raw Terminal")
		return
	_agent_id = agent_data.id
	# Show the fantasy name the orchestrator assigned at spawn time.
	# display_name falls back to the raw UUID when no name exists.
	var shown_name: String = agent_data.display_name()
	_name_label.text = shown_name
	_state_label.text = agent_data.state.capitalize()
	var class_enum: int = AgentCharacter.CLASS_BY_NAME.get(
		agent_data.character_class, AgentCharacter.CharacterClass.APPRENTICE
	)
	_class_badge.color = AgentCharacter.CLASS_COLORS[class_enum]
	_class_badge_label.text = AgentCharacter.CLASS_LABELS[class_enum]
	var provider_info: Dictionary = PROVIDER_GLYPHS.get(agent_data.provider, PROVIDER_GLYPHS[""])
	_provider_badge.text = provider_info["glyph"]
	_provider_badge.add_theme_color_override("font_color", provider_info["color"])
	if _panel != null:
		_panel.set_panel_title("%s — Raw Terminal" % shown_name)


func _on_enchant_pressed() -> void:
	if _panel != null:
		_panel.set_mode("scroll")


# ---------------------------------------------------------------------------
# Capability fallback chain
# ---------------------------------------------------------------------------

func _mount_body() -> void:
	_clear_body()
	if _agent_id.is_empty():
		_mount_chat_body()
		return
	if _live_pty_available():
		_mount_live_body()
	else:
		_mount_chat_body()


func _clear_body() -> void:
	_kill_pty()
	for child: Node in _body_root.get_children():
		child.queue_free()
	_output_label = null
	_input_line = null
	_poll_timer = null
	# The hovered body is gone, so the passthrough signal must not linger.
	_body_hover = false
	KeyPassthrough.hover_active = false
	if _panel != null:
		_panel.set_passthrough_active(false)


func _live_pty_available() -> bool:
	var os_name: String = OS.get_name()
	if os_name != "Linux" and os_name != "macOS":
		return false
	if not ClassDB.class_exists(&"Terminal") or not ClassDB.class_exists(&"PTY"):
		return false
	return _bridge_base_url_is_loopback()


func _bridge_base_url_is_loopback() -> bool:
	if _bridge == null:
		return false
	var base_url: Variant = _bridge.get("base_url")
	if typeof(base_url) != TYPE_STRING:
		return false
	var url: String = base_url as String
	var host: String = url
	host = host.trim_prefix("https://").trim_prefix("http://")
	var slash_index: int = host.find("/")
	if slash_index != -1:
		host = host.substr(0, slash_index)
	var colon_index: int = host.rfind(":")
	if colon_index != -1:
		host = host.substr(0, colon_index)
	return host == "localhost" or host == "127.0.0.1" or host == "::1"


# ---------------------------------------------------------------------------
# Live PTY body (Linux/macOS + godot-xterm installed + loopback bridge)
# ---------------------------------------------------------------------------

func _mount_live_body() -> void:
	var terminal: Control = ClassDB.instantiate(&"Terminal") as Control
	if terminal == null:
		_mount_chat_body()
		return
	terminal.name = "Terminal"
	terminal.set_anchors_preset(Control.PRESET_FULL_RECT)
	_apply_dark_terminal_theme(terminal)
	_body_root.add_child(terminal)
	var pty: Node = ClassDB.instantiate(&"PTY")
	if pty == null:
		terminal.queue_free()
		_mount_chat_body()
		return
	pty.name = "PTY"
	_body_root.add_child(pty)
	# terminal_path auto-wires both directions (Terminal keystrokes -> PTY
	# stdin, PTY data_received -> Terminal render) per godot-xterm's PTY API.
	pty.set("terminal_path", terminal.get_path())
	if pty.has_signal("exited"):
		_pty_exited_callable = _on_pty_exited.bind(pty)
		pty.connect("exited", _pty_exited_callable)
	_pty = pty
	_pty_terminal = terminal
	_live_mode = true
	var fork_error: int = pty.call(
		"fork", "tmux", PackedStringArray(["attach", "-t", "agent-" + _agent_id]), ".", 80, 24
	)
	if fork_error != OK:
		_on_pty_exited(fork_error, 0, pty)


func _apply_dark_terminal_theme(terminal: Control) -> void:
	# godot-xterm theme item names for the "Terminal" type, verified against
	# addons/godot_xterm/native/src/terminal.cpp (set_default_theme_items) at
	# the pinned commit: ansi_0_color..ansi_15_color for the standard 16-color
	# SGR palette, plus background_color/foreground_color.
	var theme: Theme = Theme.new()
	var ansi_colors: Array[String] = [
		"#1e1e1e", "#e06c75", "#98c379", "#e5c07b", "#61afef", "#c678dd", "#56b6c2", "#dcdfe4",
		"#5c6370", "#e78a99", "#b1e18b", "#f0d090", "#82c0ff", "#d9a3ec", "#7fd6e2", "#f4f6fa",
	]
	for index: int in range(ansi_colors.size()):
		theme.set_color("ansi_%d_color" % index, "Terminal", Color(ansi_colors[index]))
	theme.set_color("background_color", "Terminal", Color(0.09, 0.09, 0.11, 1.0))
	theme.set_color("foreground_color", "Terminal", Color("#e5e5e5"))
	terminal.theme = theme


func _on_pty_exited(_exit_code: int, _signal_code: int, exiting_pty: Node) -> void:
	# Guard against a stale 'exited' emission from a PTY that has already
	# been superseded (killed + replaced by swap_agent()/_mount_body()) in
	# the same frame. Godot only *defers* node deletion via queue_free(), so
	# a killed PTY's async libuv exit callback can still fire on this signal
	# after a new PTY has been assigned to _pty. Without this identity
	# check, that stale callback would tear down the freshly-mounted live
	# terminal and silently collapse it to the read-only fallback.
	if exiting_pty != _pty:
		return
	_live_mode = false
	_pty = null
	_pty_exited_callable = Callable()
	_pty_terminal = null
	_mount_chat_body("No live tmux session for this agent. Showing the polled terminal.")


func _kill_pty() -> void:
	if _pty != null and is_instance_valid(_pty):
		if (
			_pty_exited_callable.is_valid()
			and _pty.has_signal("exited")
			and _pty.is_connected("exited", _pty_exited_callable)
		):
			_pty.disconnect("exited", _pty_exited_callable)
		if _pty.has_method("kill"):
			# Signal 15 == SIGTERM == PTY.IPCSIGNAL_SIGTERM. The bare
			# identifier `PTY` (needed to reference the enum constant
			# directly) is never used anywhere in this script — only
			# ClassDB.instantiate(&"PTY") — so the script still parses
			# without the godot-xterm addon installed.
			_pty.call("kill", 15)
		_pty.queue_free()
	_pty = null
	_pty_exited_callable = Callable()
	if _pty_terminal != null and is_instance_valid(_pty_terminal):
		_pty_terminal.queue_free()
	_pty_terminal = null
	_live_mode = false


# ---------------------------------------------------------------------------
# Chat body (Windows, extension absent, or non-loopback bridge)
# ---------------------------------------------------------------------------

func _mount_chat_body(banner: String = "") -> void:
	for child: Node in _body_root.get_children():
		child.queue_free()
	var container: VBoxContainer = VBoxContainer.new()
	container.name = "ChatBody"
	container.set_anchors_preset(Control.PRESET_FULL_RECT)
	_body_root.add_child(container)
	if not banner.is_empty():
		var banner_label: Label = Label.new()
		banner_label.text = banner
		banner_label.add_theme_color_override("font_color", Color(0.85, 0.65, 0.3, 1.0))
		banner_label.autowrap_mode = TextServer.AUTOWRAP_WORD
		container.add_child(banner_label)
	var output: RichTextLabel = RichTextLabel.new()
	output.name = "Output"
	output.size_flags_vertical = Control.SIZE_EXPAND_FILL
	output.bbcode_enabled = true
	output.scroll_following = true
	output.scroll_active = true
	output.focus_mode = Control.FOCUS_NONE
	output.add_theme_color_override("default_color", Color(AnsiSgrScanner.DEFAULT_STANDARD_FG))
	output.mouse_entered.connect(_set_body_hover.bind(true))
	output.mouse_exited.connect(_set_body_hover.bind(false))
	container.add_child(output)
	_output_label = output

	var footer: HBoxContainer = HBoxContainer.new()
	footer.name = "InputRow"
	container.add_child(footer)
	var input_line: LineEdit = LineEdit.new()
	input_line.name = "Input"
	input_line.placeholder_text = "type to the agent..."
	input_line.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	input_line.text_submitted.connect(_on_input_submitted)
	footer.add_child(input_line)
	_input_line = input_line
	input_line.call_deferred("grab_focus")
	var send_button: Button = Button.new()
	send_button.name = "Send"
	send_button.text = "Send"
	send_button.pressed.connect(_on_send_pressed)
	footer.add_child(send_button)

	# The timer is a child of the body, so _clear_body() frees it. The poll
	# stops when the panel closes or swaps to the live PTY body.
	var timer: Timer = Timer.new()
	timer.name = "ScreenPoll"
	timer.wait_time = SCREEN_POLL_SECONDS
	timer.autostart = true
	timer.timeout.connect(_on_poll_tick)
	container.add_child(timer)
	_poll_timer = timer

	_connect_bridge_signals()
	_on_poll_tick()


func _connect_bridge_signals() -> void:
	if _signals_connected or _bridge == null:
		return
	if _bridge.has_signal("agent_state_changed"):
		_bridge.connect("agent_state_changed", _on_state_changed)
	if _bridge.has_signal("agent_screen"):
		_bridge.connect("agent_screen", _on_screen_snapshot)
	_signals_connected = true


func _on_poll_tick() -> void:
	if _bridge == null or _agent_id.is_empty():
		return
	if _bridge.has_method("fetch_agent_screen"):
		_bridge.call("fetch_agent_screen", _agent_id)


## Full-pane snapshot replace. An interactive TUI redraws its whole screen, so
## the panel never diffs and never appends. An empty payload means a transient
## failure, so the last good frame stays on screen.
func _on_screen_snapshot(agent_id: String, text: String) -> void:
	if agent_id != _agent_id or _output_label == null:
		return
	if text == "":
		return
	# tmux returns the whole pane rectangle, so a short TUI frame arrives padded
	# with empty rows above and below the content. Drop them, otherwise the body
	# shows a large blank region.
	var trimmed: String = AnsiSgrScanner.trim_blank_lines(text)
	if trimmed == "":
		return
	_output_label.clear()
	_output_label.append_text(
		AnsiSgrScanner.to_bbcode(trimmed, AnsiSgrScanner.STANDARD_PALETTE, AnsiSgrScanner.DEFAULT_STANDARD_FG)
	)
	# A full replace resets the scroll position, so scroll_following alone can
	# miss the tail. Jump to the last line explicitly.
	_output_label.scroll_to_line(maxi(_output_label.get_line_count() - 1, 0))


func _on_send_pressed() -> void:
	if _input_line != null:
		_on_input_submitted(_input_line.text)


func _on_input_submitted(text: String) -> void:
	if text.strip_edges().is_empty() or _bridge == null or _agent_id.is_empty():
		return
	if _bridge.has_method("send_input"):
		# The orchestrator appends Enter, so the CLI receives a submitted line.
		_bridge.call("send_input", _agent_id, text)
	if _input_line != null:
		_input_line.clear()


## Mouse-based key passthrough for the chat body.
##
## An interactive CLI such as Claude Code draws a trust prompt that only a
## real key press answers. The old rule keyed off an empty input line and
## missed Space, Escape and chords. The current rule keys off the mouse:
## while the pointer sits over the output body, EVERY keystroke forwards to
## tmux through KeyPassthrough, and the PanelBase border turns amber as the
## signal. While the pointer sits over the input line, keys edit locally.
func _set_body_hover(hovering: bool) -> void:
	_body_hover = hovering
	KeyPassthrough.hover_active = hovering
	if _panel != null:
		_panel.set_passthrough_active(hovering and not _agent_id.is_empty())
	if _input_line == null:
		return
	if hovering:
		# A focused LineEdit consumes keys before _unhandled_key_input runs.
		_input_line.release_focus()
	else:
		_input_line.grab_focus()


func _unhandled_key_input(event: InputEvent) -> void:
	if not _body_hover or _bridge == null or _agent_id.is_empty():
		return
	var key_name: String = KeyPassthrough.key_name_for(event as InputEventKey)
	if key_name.is_empty():
		return
	if _bridge.has_method("send_key"):
		_bridge.call("send_key", _agent_id, key_name)
	get_viewport().set_input_as_handled()


func _on_state_changed(agent_id: String, _old_state: String, new_state: String, _task_id: String) -> void:
	if agent_id != _agent_id:
		return
	_state_label.text = new_state.capitalize()
