class_name PanelManagerMath
extends RefCounted
## PanelManagerMath — pure arrangement-filtering math for PanelManager,
## extracted out of panel_manager.gd so it carries no autoload dependency.
## panel_manager.gd sits in the OrbFlock -> GrimoireFlyout -> BridgeManager
## preload chain (grimoire_flyout.gd touches the BridgeManager autoload at
## parse time), so a headless test that preloads panel_manager.gd to reach
## this math fails to compile outside a full Godot boot. This script has no
## such dependency, so tests/layout_presets_test.gd preloads it directly
## instead. See tests/title_screen_focus_test.gd's doc-comment for the full
## explanation of why the preload chain fails headless.


## Drops any arrangement entry whose agent_id names an agent absent from
## `known_agent_ids`. An empty agent_id (the quest board, say) always
## survives, since it names no agent. PanelManager.filter_arrangement_for_agents
## delegates here so both the production path and the headless test share
## one implementation.
static func filter_arrangement_for_agents(arrangement: Dictionary, known_agent_ids: Array) -> Dictionary:
	var kept: Array[Dictionary] = []
	for entry: Variant in arrangement.get("panels", []):
		if not (entry is Dictionary):
			continue
		var agent_id: String = (entry as Dictionary).get("agent_id", "")
		if agent_id.is_empty() or known_agent_ids.has(agent_id):
			kept.append(entry as Dictionary)
	return {"panels": kept}
