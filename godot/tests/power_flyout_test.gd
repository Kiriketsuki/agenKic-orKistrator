# power_flyout_test.gd — Regression guard for Power Controls (F4) flyout
# action wiring: the banish-all snapshot rule and the toast path matching.
#
# Standalone headless script:
#   godot --headless --path godot --script tests/power_flyout_test.gd
#
# PowerFlyout's action handlers talk to the live BridgeManager autoload.
# banish_all_ids, is_despawn_path, and is_restart_path delegate to
# PowerFlyoutMath, an autoload-free script holding the pure logic (mirroring
# title_focus_math.gd's split from title_screen.gd). This test preloads and
# calls PowerFlyoutMath directly instead of preloading power_flyout.gd,
# which sits in the OrbFlock -> GrimoireFlyout -> BridgeManager preload
# chain and fails to compile outside a full Godot boot (see
# tests/title_screen_focus_test.gd's doc-comment for the full explanation).
#
# Exits 1 on any failure.

extends SceneTree

## The one res:// script this suite's assertions depend on, loaded and
## verified at runtime instead of via a parse-time preload const. A
## dependency's own compile failure (e.g. one that reaches an unresolved
## autoload identifier) makes load() return null or the script report
## can_instantiate() == false; either case aborts the suite with quit(1)
## before any assertion runs, instead of letting a stale cached class serve
## a false green.
const REQUIRED_DEPENDENCIES: Array[String] = [
	"res://scripts/ui/power_flyout_math.gd",
]


func _verify_dependencies() -> void:
	for path: String in REQUIRED_DEPENDENCIES:
		var dep: Script = load(path) as Script
		if dep == null or not dep.can_instantiate():
			printerr("power_flyout_test: FAIL — dependency failed to compile: %s" % path)
			quit(1)


## Kept as a second, defense-in-depth guard behind _verify_dependencies: if
## an assertion helper itself errors out mid-run for a reason the dependency
## check does not cover, ran_count still falls short and the suite still
## fails closed.
const MIN_EXPECTED_ASSERTIONS: int = 9

var _ran_count: int = 0


func _init() -> void:
	_verify_dependencies()
	var failures: Array[String] = []
	_run_banish_all_ids_case(failures)
	_run_banish_all_ids_skips_non_agent_case(failures)
	_run_banish_all_ids_empty_snapshot_case(failures)
	_run_despawn_path_case(failures)
	_run_restart_path_case(failures)
	_run_unrelated_path_case(failures)
	if _ran_count < MIN_EXPECTED_ASSERTIONS:
		failures.append(
			"expected at least %d assertions to run, only %d ran — a script error likely aborted the suite early" % [MIN_EXPECTED_ASSERTIONS, _ran_count]
		)
	if failures.is_empty():
		print("power_flyout_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("power_flyout_test: FAIL — " + message)
		quit(1)


func _assert(condition: bool, message: String, failures: Array[String]) -> void:
	_ran_count += 1
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
	var ids: Array = PowerFlyoutMath.banish_all_ids(agents)
	_assert(ids.size() == 3, "expected 3 ids extracted, got %d" % ids.size(), failures)
	if ids.size() == 3:
		_assert(ids[0] == "A" and ids[1] == "B" and ids[2] == "C", "expected ids in snapshot order [A, B, C], got %s" % str(ids), failures)


## A malformed snapshot entry must never crash banish-all.
func _run_banish_all_ids_skips_non_agent_case(failures: Array[String]) -> void:
	var agents: Array = [_make_agent("A"), "not an agent", null, _make_agent("B")]
	var ids: Array = PowerFlyoutMath.banish_all_ids(agents)
	_assert(ids.size() == 2, "expected malformed entries skipped, got %d ids" % ids.size(), failures)


## Acceptance: an empty snapshot (no live agents) despawns nothing.
func _run_banish_all_ids_empty_snapshot_case(failures: Array[String]) -> void:
	var ids: Array = PowerFlyoutMath.banish_all_ids([])
	_assert(ids.is_empty(), "expected no ids from an empty snapshot, got %s" % str(ids), failures)


## Acceptance: the despawn toast fires for the despawn command path and no other.
func _run_despawn_path_case(failures: Array[String]) -> void:
	_assert(PowerFlyoutMath.is_despawn_path("/api/agents/agent-a/despawn"), "despawn path should match", failures)
	_assert(not PowerFlyoutMath.is_despawn_path("/api/agents/agent-a/cancel"), "cancel path should not match despawn", failures)


## Acceptance: the restart toast fires for the admin restart path and no other.
func _run_restart_path_case(failures: Array[String]) -> void:
	_assert(PowerFlyoutMath.is_restart_path("/api/admin/restart"), "admin restart path should match", failures)
	_assert(not PowerFlyoutMath.is_restart_path("/api/agents/agent-a/despawn"), "despawn path should not match restart", failures)


## A path this flyout does not act on triggers neither helper.
func _run_unrelated_path_case(failures: Array[String]) -> void:
	_assert(not PowerFlyoutMath.is_despawn_path("/api/tasks"), "unrelated path should not match despawn", failures)
	_assert(not PowerFlyoutMath.is_restart_path("/api/tasks"), "unrelated path should not match restart", failures)
