# power_flyout_test.gd — Regression guard for Power Controls (F4) flyout
# action wiring: the banish-all snapshot rule and the toast path matching.
#
# Standalone headless script:
#   godot --headless --path godot --script tests/power_flyout_test.gd
#
# PowerFlyout's action handlers talk to the live BridgeManager autoload, so
# this test exercises the pure static helpers those handlers delegate to
# (banish_all_ids, is_despawn_path, is_restart_path) rather than driving a
# live scene tree, matching the pattern in provider_roster_test.gd.
#
# Exits 1 on any failure.

extends SceneTree

const PowerFlyoutScript: Script = preload("res://scripts/ui/power_flyout.gd")
## PowerFlyout is a class_name-registered global. The preload above still
## loads the .gd file explicitly so this test fails loudly (missing file)
## rather than silently (unresolved global) if the script ever moves.


func _init() -> void:
	var failures: Array[String] = []
	_run_banish_all_ids_case(failures)
	_run_banish_all_ids_skips_non_agent_case(failures)
	_run_banish_all_ids_empty_snapshot_case(failures)
	_run_despawn_path_case(failures)
	_run_restart_path_case(failures)
	_run_unrelated_path_case(failures)
	if failures.is_empty():
		print("power_flyout_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("power_flyout_test: FAIL — " + message)
		quit(1)


func _assert(condition: bool, message: String, failures: Array[String]) -> void:
	if not condition:
		failures.append(message)


func _make_agent(id: String) -> BridgeData.AgentData:
	var agent: BridgeData.AgentData = BridgeData.AgentData.new()
	agent.id = id
	return agent


## Acceptance: "Banish all" extracts every agent id from the snapshot, in
## order, so every live agent at snapshot time gets despawned.
func _run_banish_all_ids_case(failures: Array[String]) -> void:
	var agents: Array = [_make_agent("A"), _make_agent("B"), _make_agent("C")]
	var ids: Array = PowerFlyout.banish_all_ids(agents)
	_assert(ids.size() == 3, "expected 3 ids extracted, got %d" % ids.size(), failures)
	if ids.size() == 3:
		_assert(ids[0] == "A" and ids[1] == "B" and ids[2] == "C", "expected ids in snapshot order [A, B, C], got %s" % str(ids), failures)


## A malformed snapshot entry must never crash banish-all.
func _run_banish_all_ids_skips_non_agent_case(failures: Array[String]) -> void:
	var agents: Array = [_make_agent("A"), "not an agent", null, _make_agent("B")]
	var ids: Array = PowerFlyout.banish_all_ids(agents)
	_assert(ids.size() == 2, "expected malformed entries skipped, got %d ids" % ids.size(), failures)


## Acceptance: an empty snapshot (no live agents) despawns nothing.
func _run_banish_all_ids_empty_snapshot_case(failures: Array[String]) -> void:
	var ids: Array = PowerFlyout.banish_all_ids([])
	_assert(ids.is_empty(), "expected no ids from an empty snapshot, got %s" % str(ids), failures)


## Acceptance: the despawn toast fires for the despawn command path and no other.
func _run_despawn_path_case(failures: Array[String]) -> void:
	_assert(PowerFlyout.is_despawn_path("/api/agents/agent-a/despawn"), "despawn path should match", failures)
	_assert(not PowerFlyout.is_despawn_path("/api/agents/agent-a/cancel"), "cancel path should not match despawn", failures)


## Acceptance: the restart toast fires for the admin restart path and no other.
func _run_restart_path_case(failures: Array[String]) -> void:
	_assert(PowerFlyout.is_restart_path("/api/admin/restart"), "admin restart path should match", failures)
	_assert(not PowerFlyout.is_restart_path("/api/agents/agent-a/despawn"), "despawn path should not match restart", failures)


## A path this flyout does not act on triggers neither helper.
func _run_unrelated_path_case(failures: Array[String]) -> void:
	_assert(not PowerFlyout.is_despawn_path("/api/tasks"), "unrelated path should not match despawn", failures)
	_assert(not PowerFlyout.is_restart_path("/api/tasks"), "unrelated path should not match restart", failures)
