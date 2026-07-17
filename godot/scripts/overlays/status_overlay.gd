extends PanelContainer
## StatusOverlay — the "enchanted nameplate" tooltip view. Purely presentational:
## fills its labels from agent data and applies a procedural glow border themed
## in the agent's provider color. Owned/positioned by StatusOverlayManager.

class_name StatusOverlay

## LLM provider -> (glyph, ink color). Copied verbatim from SpellScrollView so
## the nameplate's provider theming matches the spell scroll and terminal views.
const PROVIDER_GLYPHS: Dictionary = {
	"claude": {"glyph": "✦", "color": Color(0.55, 0.32, 0.14, 1.0)},
	"gemini": {"glyph": "♦", "color": Color(0.32, 0.42, 0.52, 1.0)},
	"openai": {"glyph": "◆", "color": Color(0.4, 0.4, 0.4, 1.0)},
	"ollama": {"glyph": "▲", "color": Color(0.3, 0.5, 0.3, 1.0)},
	"deepseek": {"glyph": "◈", "color": Color(0.45, 0.25, 0.55, 1.0)},
	"": {"glyph": "○", "color": Color(0.4, 0.35, 0.3, 1.0)},
}

## Character class -> themed power tier. No real tier field exists on
## BridgeData.AgentData — this is flavor text derived from class, not a
## gateway-reported value. See T11 spec risk notes.
const TIER_BY_CLASS: Dictionary = {
	"archmage":   "S — Archmage",
	"enchanter":  "A — Enchanter",
	"wardkeeper": "A — Wardkeeper",
	"alchemist":  "B — Alchemist",
	"librarian":  "B — Librarian",
	"scribe":     "C — Scribe",
	"apprentice": "D — Apprentice",
}

@onready var _name_label: Label = $Margin/VBox/NameLabel
@onready var _class_label: Label = $Margin/VBox/Grid/ClassLabel
@onready var _provider_label: Label = $Margin/VBox/Grid/ProviderLabel
@onready var _tier_label: Label = $Margin/VBox/Grid/TierLabel
@onready var _state_label: Label = $Margin/VBox/Grid/StateLabel
@onready var _task_label: Label = $Margin/VBox/Grid/TaskLabel
@onready var _uptime_label: Label = $Margin/VBox/Grid/UptimeLabel


## Fills every label from the given agent snapshot and re-applies the glow
## border themed in the provider color. `task_id` is passed separately from
## `agent_data.current_task_id` because BridgeManager does not update that
## field on agent.state_changed — callers pass the freshest known task id.
func populate(agent_id: String, agent_data: BridgeData.AgentData, task_id: String) -> void:
	_name_label.text = agent_id
	var character_class: String = "apprentice"
	var provider: String = ""
	var state: String = "idle"
	if agent_data != null:
		character_class = agent_data.character_class
		provider = agent_data.provider
		state = agent_data.state

	var class_enum: int = AgentCharacter.CLASS_BY_NAME.get(
		character_class, AgentCharacter.CharacterClass.APPRENTICE
	)
	_class_label.text = character_class.capitalize()
	_class_label.add_theme_color_override("font_color", AgentCharacter.CLASS_COLORS[class_enum])

	var provider_info: Dictionary = PROVIDER_GLYPHS.get(provider, PROVIDER_GLYPHS[""])
	var provider_display: String = provider if not provider.is_empty() else "unknown"
	_provider_label.text = "%s %s" % [provider_info["glyph"], provider_display.capitalize()]
	_provider_label.add_theme_color_override("font_color", provider_info["color"])

	_tier_label.text = TIER_BY_CLASS.get(character_class, "D — Apprentice")
	_state_label.text = state.capitalize()
	_task_label.text = task_id if not task_id.is_empty() else "—"

	_apply_glow_border(provider_info["color"])


## Formats seconds as "Hh Mm" / "Mm Ss" / "Ss" — the coarsest two units.
func set_uptime_seconds(total_seconds: int) -> void:
	var s: int = maxi(total_seconds, 0)
	var hours: int = s / 3600
	var minutes: int = (s % 3600) / 60
	var seconds: int = s % 60
	if hours > 0:
		_uptime_label.text = "%dh %dm" % [hours, minutes]
	elif minutes > 0:
		_uptime_label.text = "%dm %ds" % [minutes, seconds]
	else:
		_uptime_label.text = "%ds" % seconds


func _apply_glow_border(glow_color: Color) -> void:
	var sb: StyleBoxFlat = StyleBoxFlat.new()
	sb.bg_color = Color(0.08, 0.08, 0.1, 0.88)
	sb.set_corner_radius_all(4)
	sb.set_border_width_all(2)
	sb.border_color = glow_color
	sb.shadow_color = Color(glow_color.r, glow_color.g, glow_color.b, 0.55)
	sb.shadow_size = 10
	sb.content_margin_left = 2.0
	sb.content_margin_top = 2.0
	sb.content_margin_right = 2.0
	sb.content_margin_bottom = 2.0
	add_theme_stylebox_override("panel", sb)
