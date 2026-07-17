extends HBoxContainer
## SummonBar — top-left toolbar for spawning worker agents from the UI.
## Buttons POST /api/agents/spawn via BridgeManager; the result label shows
## the last spawn outcome (name + kind, or the server's error).

const KINDS: Array[Dictionary] = [
	{"kind": "claude", "label": "Summon Claude"},
	{"kind": "codex", "label": "Summon Codex"},
	{"kind": "opencode", "label": "Summon OpenCode"},
	{"kind": "sim", "label": "Summon Sim"},
]

var _result_label: Label


func _ready() -> void:
	for entry: Dictionary in KINDS:
		var button := Button.new()
		button.text = entry["label"]
		var kind: String = entry["kind"]
		button.pressed.connect(func() -> void: _on_summon_pressed(kind))
		add_child(button)

	_result_label = Label.new()
	_result_label.text = ""
	_result_label.add_theme_color_override("font_color", Color(0.85, 0.9, 1.0, 0.9))
	add_child(_result_label)

	var bridge: Node = get_node_or_null("/root/BridgeManager")
	if bridge:
		bridge.connect("command_succeeded", _on_command_succeeded)
		bridge.connect("command_failed", _on_command_failed)


func _on_summon_pressed(kind: String) -> void:
	var bridge: Node = get_node_or_null("/root/BridgeManager")
	if bridge == null:
		_result_label.text = "bridge unavailable"
		return
	_result_label.text = "summoning %s..." % kind
	bridge.spawn_agent(kind)


func _on_command_succeeded(path: String, _code: int, response_body: String) -> void:
	if path != "/api/agents/spawn":
		return
	var parsed: Variant = JSON.parse_string(response_body)
	if parsed is Dictionary:
		var d: Dictionary = parsed as Dictionary
		_result_label.text = "%s the %s agent joins the tower" % [d.get("name", "?"), d.get("kind", "?")]
	else:
		_result_label.text = "agent summoned"


func _on_command_failed(path: String, _code: int, reason: String) -> void:
	if path != "/api/agents/spawn":
		return
	_result_label.text = "summon failed: " + reason
