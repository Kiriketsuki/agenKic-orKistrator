class_name PowerFlyoutMath
extends RefCounted
## PowerFlyoutMath — pure banish-all and path-matching math for PowerFlyout
## (F4), extracted out of power_flyout.gd so it carries no autoload
## dependency. power_flyout.gd sits in the OrbFlock -> GrimoireFlyout ->
## BridgeManager preload chain (grimoire_flyout.gd touches the BridgeManager
## autoload at parse time), so a headless test that preloads power_flyout.gd
## to reach this math fails to compile outside a full Godot boot. This
## script has no such dependency, so tests/power_flyout_test.gd preloads it
## directly instead. See tests/title_screen_focus_test.gd's doc-comment for
## the full explanation of why the preload chain fails headless.


## Turns a snapshot Array of BridgeData.AgentData into the ordered list of
## ids to despawn. PowerFlyout.banish_all_ids delegates here so both the
## production path and the headless test share one implementation.
static func banish_all_ids(agents: Array) -> Array[String]:
	var ids: Array[String] = []
	for agent: Variant in agents:
		if agent is BridgeData.AgentData:
			ids.append((agent as BridgeData.AgentData).id)
	return ids


## Path-matching helpers behind the command-succeeded/failed toast wiring.
## PowerFlyout.is_despawn_path/is_restart_path delegate here so both the
## production path and the headless test share one implementation.
static func is_despawn_path(path: String) -> bool:
	return path.ends_with("/despawn")


static func is_restart_path(path: String) -> bool:
	return path.ends_with("/admin/restart")
