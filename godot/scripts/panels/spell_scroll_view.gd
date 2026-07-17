extends Control
## SpellScrollView — parchment-themed output detail view mounted into a
## PanelBase's ContentRoot when the panel is in "scroll" mode. Renders one
## agent's full output history and live stream as sepia ink on aged
## parchment, with a quill-writing reveal animation for new live output.

class_name SpellScrollView

const CHARS_PER_SEC: float = 60.0
const QUILL_BOB_SPEED: float = 9.0
const QUILL_BOB_AMPLITUDE: float = 0.22
const HISTORY_BACKFILL_LINES: int = 200

## LLM provider -> (glyph, ink color). Empty key is the fallback.
const PROVIDER_GLYPHS: Dictionary = {
	"claude": {"glyph": "✦", "color": Color(0.55, 0.32, 0.14, 1.0)},
	"gemini": {"glyph": "♦", "color": Color(0.32, 0.42, 0.52, 1.0)},
	"openai": {"glyph": "◆", "color": Color(0.4, 0.4, 0.4, 1.0)},
	"ollama": {"glyph": "▲", "color": Color(0.3, 0.5, 0.3, 1.0)},
	"deepseek": {"glyph": "◈", "color": Color(0.45, 0.25, 0.55, 1.0)},
	"": {"glyph": "○", "color": Color(0.4, 0.35, 0.3, 1.0)},
}

@onready var _parchment: ColorRect = $Parchment
@onready var _class_badge: ColorRect = $Header/ClassBadge
@onready var _class_badge_label: Label = $Header/ClassBadge/ClassBadgeLabel
@onready var _name_label: Label = $Header/NameLabel
@onready var _provider_badge: Label = $Header/ProviderBadge
@onready var _state_label: Label = $Header/StateLabel
@onready var _history: RichTextLabel = $History
@onready var _quill: Label = $QuillGlyph
@onready var _disenchant_button: Button = $Header/DisenchantButton

var _panel: PanelBase = null
var _bridge: Node = null
var _agent_id: String = ""
var _signals_connected: bool = false
var _bob_time: float = 0.0
## True from the moment a backfill fetch is issued until its response (or a
## superseding swap_agent()) lands. While true, live output is held in
## _pending_live_chunks instead of being appended directly, so backfilled
## (older) history can never be inserted below live (newer) output that
## raced ahead of it, and the live chunk's quill reveal is never
## short-circuited by the backfill's instant-reveal.
var _backfill_pending: bool = false
var _pending_live_chunks: Array = []


func _ready() -> void:
	_configure_parchment_material()
	_configure_font()
	_quill.visible = false


func _process(delta: float) -> void:
	if _history == null:
		return
	var total: int = _history.get_total_character_count()
	if _history.visible_characters < total:
		var advance: int = int(ceil(CHARS_PER_SEC * delta))
		_history.visible_characters = mini(_history.visible_characters + advance, total)
		_bob_time += delta * QUILL_BOB_SPEED
		_quill.visible = true
		_quill.rotation = sin(_bob_time) * QUILL_BOB_AMPLITUDE
	elif _quill.visible:
		_quill.visible = false
		_quill.rotation = 0.0
		_bob_time = 0.0


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

## Called once by PanelContentRouter right after this view is added to the
## tree. `agent_data` may be null if the agent isn't known yet.
func setup(panel: PanelBase, agent_data: BridgeData.AgentData, bridge: Node) -> void:
	_panel = panel
	_bridge = bridge
	_restyle_close_button()
	_disable_mode_toggle()
	_wire_disenchant_button()
	_clear_history()
	_apply_agent(agent_data)
	_connect_bridge_signals()
	if _panel != null and not _panel.animation_hook_requested.is_connected(_on_panel_animation_hook):
		_panel.animation_hook_requested.connect(_on_panel_animation_hook)
	_play_unroll_flourish.call_deferred()
	_request_backfill()


## Called by PanelManager when the singleton scroll panel is retargeted to a
## different agent (clicking another agent while a scroll is already open).
func swap_agent(agent_data: BridgeData.AgentData) -> void:
	_clear_history()
	_apply_agent(agent_data)
	_request_backfill()


# ---------------------------------------------------------------------------
# Setup helpers
# ---------------------------------------------------------------------------

func _configure_parchment_material() -> void:
	if _parchment == null:
		return
	var material: ShaderMaterial = _parchment.material as ShaderMaterial
	if material == null:
		return
	if material.get_shader_parameter("fibre_noise") != null:
		return
	var noise: FastNoiseLite = FastNoiseLite.new()
	noise.noise_type = FastNoiseLite.TYPE_PERLIN
	noise.frequency = 0.045
	noise.fractal_octaves = 3
	var noise_texture: NoiseTexture2D = NoiseTexture2D.new()
	noise_texture.width = 256
	noise_texture.height = 256
	noise_texture.seamless = true
	noise_texture.noise = noise
	material.set_shader_parameter("fibre_noise", noise_texture)


func _configure_font() -> void:
	var font: SystemFont = SystemFont.new()
	font.font_names = PackedStringArray(["Monospace", "DejaVu Sans Mono", "Courier New", "Consolas"])
	font.allow_system_fallback = true
	_history.add_theme_font_override("normal_font", font)
	_history.add_theme_font_override("bold_font", font)
	_history.add_theme_font_size_override("normal_font_size", 14)
	_history.add_theme_font_size_override("bold_font_size", 14)
	_name_label.add_theme_font_override("font", font)


func _restyle_close_button() -> void:
	if _panel == null:
		return
	var button: Button = _panel.get_close_button()
	if button == null:
		return
	button.text = "🕯"
	button.tooltip_text = "Seal the scroll"
	button.add_theme_color_override("font_color", Color(0.55, 0.12, 0.1, 1.0))


## The spell scroll is a dedicated panel type, not a generic mode among
## others — toggling the shared T8 title bar's Mode button away from
## "scroll" here would persist a non-scroll mode_preferences entry for this
## agent (PanelManager._wire_panel's mode_changed handler) and silently
## replace the scroll with the generic placeholder on the next click,
## for this panel and for any future scroll open for the same agent.
func _disable_mode_toggle() -> void:
	if _panel == null:
		return
	var button: Button = _panel.get_mode_button()
	if button == null:
		return
	button.visible = false
	button.disabled = true


## Disenchant is the scroll's dedicated toggle into "terminal" mode — routed
## through the same PanelBase.set_mode() -> mode_changed plumbing the T8
## generic Mode button used, so PanelManager._wire_panel's mode_changed
## handler (mount + persist mode_preferences) applies unchanged.
func _wire_disenchant_button() -> void:
	if _panel == null or _disenchant_button == null:
		return
	if not _disenchant_button.pressed.is_connected(_on_disenchant_pressed):
		_disenchant_button.pressed.connect(_on_disenchant_pressed)


func _on_disenchant_pressed() -> void:
	if _panel != null:
		_panel.set_mode("terminal")


func _apply_agent(agent_data: BridgeData.AgentData) -> void:
	if agent_data == null:
		_agent_id = ""
		_name_label.text = "Unknown Agent"
		_state_label.text = ""
		_class_badge.color = Color(0.5, 0.5, 0.5, 1.0)
		_class_badge_label.text = "?"
		_provider_badge.text = ""
		if _panel != null:
			_panel.set_panel_title("Spell Scroll")
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
		_panel.set_panel_title("%s — Spell Scroll" % agent_data.id)


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


func _clear_history() -> void:
	_history.clear()
	# Set explicitly (rather than leaving the -1 "show everything" sentinel) so
	# that any text appended after this point starts hidden and can be
	# revealed either instantly (backfill) or via the quill writer (live).
	_history.visible_characters = 0
	_quill.visible = false
	_bob_time = 0.0
	_backfill_pending = false
	_pending_live_chunks.clear()


# ---------------------------------------------------------------------------
# Bridge signal handlers
# ---------------------------------------------------------------------------

func _on_history_backfill(agent_id: String, chunks: Array) -> void:
	if agent_id != _agent_id:
		return
	if not _backfill_pending:
		# Already superseded by a later swap_agent()/_request_backfill() call
		# for this same agent id (e.g. rapid re-open) — drop it rather than
		# risk appending stale history after newer content.
		return
	_backfill_pending = false
	for item: Variant in chunks:
		var chunk: BridgeData.AgentOutputChunk = item as BridgeData.AgentOutputChunk
		if chunk == null:
			continue
		_history.append_text(AnsiSepiaParser.to_bbcode(chunk.payload) + "\n")
	# Backfilled history appears instantly — only new live output is quill-written.
	# Cap the instant reveal at the backfill's own end so it can never race
	# ahead of and short-circuit a live chunk appended below it.
	_history.visible_characters = _history.get_total_character_count()
	# Flush any live output that arrived while the backfill was in flight —
	# appended now (after, i.e. below, the backfilled history) and left
	# unrevealed so _process() quill-writes it normally.
	var pending: Array = _pending_live_chunks
	_pending_live_chunks = []
	for chunk: BridgeData.AgentOutputChunk in pending:
		_history.append_text(AnsiSepiaParser.to_bbcode(chunk.payload) + "\n")


func _on_live_output(chunk: BridgeData.AgentOutputChunk) -> void:
	if chunk == null or chunk.agent_id != _agent_id:
		return
	if _backfill_pending:
		# Hold until the backfill lands so older history can't be inserted
		# below this newer line, and so this line's quill reveal can't be
		# short-circuited by the backfill's instant visible_characters set.
		_pending_live_chunks.append(chunk)
		return
	_history.append_text(AnsiSepiaParser.to_bbcode(chunk.payload) + "\n")
	# visible_characters is left behind on purpose — _process() advances it.


func _on_state_changed(agent_id: String, _old_state: String, new_state: String, _task_id: String) -> void:
	if agent_id != _agent_id:
		return
	_state_label.text = new_state.capitalize()


func _on_panel_animation_hook(_panel_ref: PanelBase, hook_name: StringName) -> void:
	if hook_name == &"rescroll":
		_play_unroll_flourish()


# ---------------------------------------------------------------------------
# Animation
# ---------------------------------------------------------------------------

func _play_unroll_flourish() -> void:
	if not is_inside_tree():
		return
	modulate.a = 0.0
	scale = Vector2(1.0, 0.85)
	var tween: Tween = create_tween().set_parallel(true)
	tween.tween_property(self, "modulate:a", 1.0, 0.22).set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)
	tween.tween_property(self, "scale", Vector2.ONE, 0.22).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
