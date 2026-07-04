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
## 2. READ-ONLY FALLBACK — Windows, extension absent, or non-loopback bridge.
##    A RichTextLabel fed from BridgeManager's live/backfill signals, same
##    ANSI-driven rendering pattern as SpellScrollView, through the standard
##    (non-sepia) palette.
##
## godot-xterm classes are referenced only via ClassDB.instantiate(&"...")
## (never `Terminal.new()`/`PTY.new()`/`as Terminal`) so this script parses
## and loads even when the addon is not installed — the project must run
## cleanly without it (identical behavior to the Windows fallback path).

class_name TerminalView

const HISTORY_BACKFILL_LINES: int = 200

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

## Read-only fallback state.
var _output_label: RichTextLabel = null
var _backfill_pending: bool = false
var _pending_live_chunks: Array = []


func _ready() -> void:
	_enchant_button.pressed.connect(_on_enchant_pressed)


func _exit_tree() -> void:
	_kill_pty()


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
	_name_label.text = agent_data.id
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
		_panel.set_panel_title("%s — Raw Terminal" % agent_data.id)


func _on_enchant_pressed() -> void:
	if _panel != null:
		_panel.set_mode("scroll")


# ---------------------------------------------------------------------------
# Capability fallback chain
# ---------------------------------------------------------------------------

func _mount_body() -> void:
	_clear_body()
	if _agent_id.is_empty():
		_mount_read_only_body()
		return
	if _live_pty_available():
		_mount_live_body()
	else:
		_mount_read_only_body()


func _clear_body() -> void:
	_kill_pty()
	for child: Node in _body_root.get_children():
		child.queue_free()
	_output_label = null


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
		_mount_read_only_body()
		return
	terminal.name = "Terminal"
	terminal.set_anchors_preset(Control.PRESET_FULL_RECT)
	_apply_dark_terminal_theme(terminal)
	_body_root.add_child(terminal)
	var pty: Node = ClassDB.instantiate(&"PTY")
	if pty == null:
		terminal.queue_free()
		_mount_read_only_body()
		return
	pty.name = "PTY"
	_body_root.add_child(pty)
	# terminal_path auto-wires both directions (Terminal keystrokes -> PTY
	# stdin, PTY data_received -> Terminal render) per godot-xterm's PTY API.
	pty.set("terminal_path", terminal.get_path())
	if pty.has_signal("exited"):
		pty.connect("exited", _on_pty_exited)
	_pty = pty
	_pty_terminal = terminal
	_live_mode = true
	var fork_error: int = pty.call(
		"fork", "tmux", PackedStringArray(["attach", "-t", "agent-" + _agent_id]), ".", 80, 24
	)
	if fork_error != OK:
		_on_pty_exited(fork_error, 0)


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


func _on_pty_exited(_exit_code: int, _signal_code: int) -> void:
	_live_mode = false
	_pty = null
	_pty_terminal = null
	_mount_read_only_body("No live tmux session for this agent — showing read-only output.")


func _kill_pty() -> void:
	if _pty != null and is_instance_valid(_pty):
		if _pty.has_method("kill"):
			# Signal 15 == SIGTERM == PTY.IPCSIGNAL_SIGTERM. The bare
			# identifier `PTY` (needed to reference the enum constant
			# directly) is never used anywhere in this script — only
			# ClassDB.instantiate(&"PTY") — so the script still parses
			# without the godot-xterm addon installed.
			_pty.call("kill", 15)
		_pty.queue_free()
	_pty = null
	if _pty_terminal != null and is_instance_valid(_pty_terminal):
		_pty_terminal.queue_free()
	_pty_terminal = null
	_live_mode = false


# ---------------------------------------------------------------------------
# Read-only fallback body (Windows, extension absent, or non-loopback bridge)
# ---------------------------------------------------------------------------

func _mount_read_only_body(banner: String = "") -> void:
	for child: Node in _body_root.get_children():
		child.queue_free()
	var container: VBoxContainer = VBoxContainer.new()
	container.name = "ReadOnlyBody"
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
	container.add_child(output)
	_output_label = output
	_clear_read_only_state()
	_connect_bridge_signals()
	_request_backfill()


func _clear_read_only_state() -> void:
	if _output_label != null:
		_output_label.clear()
	_backfill_pending = false
	_pending_live_chunks.clear()


func _connect_bridge_signals() -> void:
	if _signals_connected or _bridge == null:
		return
	if _bridge.has_signal("agent_output"):
		_bridge.connect("agent_output", _on_live_output)
	if _bridge.has_signal("agent_state_changed"):
		_bridge.connect("agent_state_changed", _on_state_changed)
	if _bridge.has_signal("agent_output_history"):
		_bridge.connect("agent_output_history", _on_history_backfill)
	_signals_connected = true


func _request_backfill() -> void:
	if _bridge == null or _agent_id.is_empty():
		return
	if _bridge.has_method("fetch_agent_output_history"):
		_backfill_pending = true
		_bridge.call("fetch_agent_output_history", _agent_id, HISTORY_BACKFILL_LINES)


func _on_history_backfill(agent_id: String, chunks: Array) -> void:
	if agent_id != _agent_id or _output_label == null:
		return
	if not _backfill_pending:
		# Already superseded by a later swap_agent()/_request_backfill() call
		# for this same agent id — drop it, same in-flight guard as
		# SpellScrollView._on_history_backfill.
		return
	_backfill_pending = false
	for item: Variant in chunks:
		var chunk: BridgeData.AgentOutputChunk = item as BridgeData.AgentOutputChunk
		if chunk == null:
			continue
		_output_label.append_text(
			AnsiSgrScanner.to_bbcode(chunk.payload, AnsiSgrScanner.STANDARD_PALETTE, AnsiSgrScanner.DEFAULT_STANDARD_FG) + "\n"
		)
	var pending: Array = _pending_live_chunks
	_pending_live_chunks = []
	for chunk: BridgeData.AgentOutputChunk in pending:
		_output_label.append_text(
			AnsiSgrScanner.to_bbcode(chunk.payload, AnsiSgrScanner.STANDARD_PALETTE, AnsiSgrScanner.DEFAULT_STANDARD_FG) + "\n"
		)


func _on_live_output(chunk: BridgeData.AgentOutputChunk) -> void:
	if chunk == null or chunk.agent_id != _agent_id or _output_label == null:
		return
	if _backfill_pending:
		_pending_live_chunks.append(chunk)
		return
	_output_label.append_text(
		AnsiSgrScanner.to_bbcode(chunk.payload, AnsiSgrScanner.STANDARD_PALETTE, AnsiSgrScanner.DEFAULT_STANDARD_FG) + "\n"
	)


func _on_state_changed(agent_id: String, _old_state: String, new_state: String, _task_id: String) -> void:
	if agent_id != _agent_id:
		return
	_state_label.text = new_state.capitalize()
