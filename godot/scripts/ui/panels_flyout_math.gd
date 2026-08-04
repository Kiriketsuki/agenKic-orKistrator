class_name PanelsFlyoutMath
extends RefCounted
## PanelsFlyoutMath — pure agent-row-building math for PanelsFlyout (F5),
## extracted out of panels_flyout.gd so it carries no autoload dependency.
## panels_flyout.gd sits in the OrbFlock -> GrimoireFlyout -> BridgeManager
## preload chain (grimoire_flyout.gd touches the BridgeManager autoload at
## parse time), so a headless test that preloads panels_flyout.gd to reach
## this math fails to compile outside a full Godot boot. This script has no
## such dependency, so tests/layout_presets_test.gd preloads it directly
## instead. See tests/title_screen_focus_test.gd's doc-comment for the full
## explanation of why the preload chain fails headless.


## Turns a snapshot Array of BridgeData.AgentData into the ordered row list
## {agent_id, label, accent}. PanelsFlyout.build_agent_rows delegates here
## so both the production path and the headless test share one
## implementation.
static func build_agent_rows(agents: Array) -> Array[Dictionary]:
	var typed: Array[BridgeData.AgentData] = []
	for agent: Variant in agents:
		if agent is BridgeData.AgentData:
			typed.append(agent as BridgeData.AgentData)
	typed.sort_custom(func(a: BridgeData.AgentData, b: BridgeData.AgentData) -> bool: return a.id < b.id)
	var rows: Array[Dictionary] = []
	for agent: BridgeData.AgentData in typed:
		var label: String = "%s — %s" % [agent.display_name(), agent.state]
		if not agent.provider.is_empty():
			label = "%s · %s — %s" % [agent.display_name(), agent.provider, agent.state]
		rows.append({
			"agent_id": agent.id,
			"label": label,
			"accent": ProviderPalette.get_accent_color(agent.provider),
		})
	return rows
