# layout_presets_test.gd — Regression guard for the Panels Orb and Restyle
# (F5) layout preset system: LayoutPresets round-trips to
# user://orki_settings.cfg, PanelManagerMath.filter_arrangement_for_agents skips
# a banished agent without error, and PanelsFlyoutMath.build_agent_rows extracts
# a stable, accent-colored row list from a BridgeManager snapshot.
#
# Standalone headless script:
#   godot --headless --path godot --script tests/layout_presets_test.gd
#
# PanelManager and PanelsFlyout talk to a live scene tree and BridgeManager
# autoload for most of their behavior. PanelManager.filter_arrangement_for_agents
# and PanelsFlyout.build_agent_rows delegate to PanelManagerMath and
# PanelsFlyoutMath respectively, two autoload-free scripts holding the pure
# logic (mirroring title_focus_math.gd's split from title_screen.gd). This
# test preloads and calls the Math scripts directly instead of preloading
# panel_manager.gd/panels_flyout.gd, which sit in the OrbFlock ->
# GrimoireFlyout -> BridgeManager preload chain and fail to compile outside
# a full Godot boot (see tests/title_screen_focus_test.gd's doc-comment for
# the full explanation).
#
# Exits 1 on any failure. Uses a dedicated user:// path (not the real
# orki_settings.cfg) so the test never clobbers a keeper's actual config.

extends SceneTree

const TEST_PATH: String = "user://layout_presets_test.cfg"

## Every res:// script this suite's assertions depend on, loaded and
## verified at runtime instead of via a parse-time preload const. A
## dependency's own compile failure (e.g. one that reaches an unresolved
## autoload identifier) makes load() return null or the script report
## can_instantiate() == false; either case aborts the suite with quit(1)
## before any assertion runs, instead of letting a stale cached class serve
## a false green.
const REQUIRED_DEPENDENCIES: Array[String] = [
	"res://scripts/layout_presets.gd",
	"res://scripts/panel_manager_math.gd",
	"res://scripts/ui/panels_flyout_math.gd",
]


func _verify_dependencies() -> void:
	for path: String in REQUIRED_DEPENDENCIES:
		var dep: Script = load(path) as Script
		if dep == null or not dep.can_instantiate():
			printerr("layout_presets_test: FAIL — dependency failed to compile: %s" % path)
			quit(1)


## Kept as a second, defense-in-depth guard behind _verify_dependencies: if
## an assertion helper itself errors out mid-run for a reason the dependency
## check does not cover, ran_count still falls short and the suite still
## fails closed.
const MIN_EXPECTED_ASSERTIONS: int = 17

var _ran_count: int = 0


func _init() -> void:
	_verify_dependencies()
	var failures: Array[String] = []
	_cleanup()
	_run_missing_file_defaults_case(failures)
	_run_round_trip_case(failures)
	_run_remove_preset_case(failures)
	_run_filter_keeps_known_agents_case(failures)
	_run_filter_skips_banished_agent_case(failures)
	_run_filter_keeps_agentless_entries_case(failures)
	_run_build_agent_rows_case(failures)
	_run_build_agent_rows_skips_non_agent_case(failures)
	_cleanup()
	if _ran_count < MIN_EXPECTED_ASSERTIONS:
		failures.append(
			"expected at least %d assertions to run, only %d ran — a script error likely aborted the suite early" % [MIN_EXPECTED_ASSERTIONS, _ran_count]
		)
	if failures.is_empty():
		print("layout_presets_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("layout_presets_test: FAIL — " + message)
		quit(1)


func _cleanup() -> void:
	if FileAccess.file_exists(TEST_PATH):
		DirAccess.remove_absolute(TEST_PATH)


func _assert(condition: bool, message: String, failures: Array[String]) -> void:
	_ran_count += 1
	if not condition:
		failures.append(message)


func _make_agent(id: String, provider: String = "claude", state: String = "idle") -> BridgeData.AgentData:
	var agent: BridgeData.AgentData = BridgeData.AgentData.new()
	agent.id = id
	agent.provider = provider
	agent.state = state
	return agent


## A first-run keeper with no config file yet must get a usable (empty)
## preset map rather than an error.
func _run_missing_file_defaults_case(failures: Array[String]) -> void:
	var presets: LayoutPresets = LayoutPresets.load_from_file(TEST_PATH)
	_assert(presets.presets.is_empty(), "a missing config file should yield an empty preset map", failures)
	_assert(presets.preset_names().is_empty(), "a missing config file should yield no preset names", failures)


## Acceptance: "Save and restore a layout" — a saved preset's arrangement
## round-trips through save_to_file/load_from_file exactly.
func _run_round_trip_case(failures: Array[String]) -> void:
	var presets: LayoutPresets = LayoutPresets.new()
	var arrangement: Dictionary = {
		"panels": [
			{"panel_id": "spell-scroll", "agent_id": "agent-a", "title": "Agent A", "mode": "scroll", "position": [10.0, 20.0], "size": [400.0, 600.0]},
		],
	}
	presets.set_preset("review", arrangement)
	var save_err: int = presets.save_to_file(TEST_PATH)
	_assert(save_err == OK, "save_to_file should return OK, got %d" % save_err, failures)

	var reloaded: LayoutPresets = LayoutPresets.load_from_file(TEST_PATH)
	_assert(reloaded.has_preset("review"), "the saved preset name should round-trip", failures)
	_assert(reloaded.preset_names() == ["review"], "preset_names should list the one saved preset", failures)
	var reloaded_arrangement: Dictionary = reloaded.get_preset("review")
	_assert(reloaded_arrangement == arrangement, "the saved arrangement should round-trip exactly, got %s" % str(reloaded_arrangement), failures)


## remove_preset drops exactly the named preset and no other.
func _run_remove_preset_case(failures: Array[String]) -> void:
	var presets: LayoutPresets = LayoutPresets.new()
	presets.set_preset("review", {"panels": []})
	presets.set_preset("focus-two", {"panels": []})
	presets.remove_preset("review")
	_assert(not presets.has_preset("review"), "removed preset should no longer be present", failures)
	_assert(presets.has_preset("focus-two"), "an unrelated preset should survive the removal", failures)


## Acceptance: restoring a preset whose agent is still live keeps that entry.
func _run_filter_keeps_known_agents_case(failures: Array[String]) -> void:
	var arrangement: Dictionary = {
		"panels": [
			{"panel_id": "spell-scroll", "agent_id": "agent-a", "position": [0.0, 0.0], "size": [400.0, 600.0]},
		],
	}
	var filtered: Dictionary = PanelManagerMath.filter_arrangement_for_agents(arrangement, ["agent-a", "agent-b"])
	_assert(filtered.get("panels", []).size() == 1, "a known agent's entry should survive the filter", failures)


## Acceptance: "Restore with a missing agent" — a preset entry naming a
## banished agent is skipped without error, and every surviving entry stays.
func _run_filter_skips_banished_agent_case(failures: Array[String]) -> void:
	var arrangement: Dictionary = {
		"panels": [
			{"panel_id": "spell-scroll", "agent_id": "agent-a", "position": [0.0, 0.0], "size": [400.0, 600.0]},
			{"panel_id": "quest-board", "agent_id": "", "position": [50.0, 50.0], "size": [300.0, 300.0]},
			{"panel_id": "old-scroll", "agent_id": "agent-banished", "position": [10.0, 10.0], "size": [200.0, 200.0]},
		],
	}
	var filtered: Dictionary = PanelManagerMath.filter_arrangement_for_agents(arrangement, ["agent-a"])
	var kept: Array = filtered.get("panels", [])
	_assert(kept.size() == 2, "the banished agent's entry should be dropped, expected 2 survivors, got %d" % kept.size(), failures)
	var kept_ids: Array = []
	for entry: Dictionary in kept:
		kept_ids.append(entry.get("panel_id", ""))
	_assert(kept_ids.has("spell-scroll"), "the live agent's panel should survive", failures)
	_assert(kept_ids.has("quest-board"), "the agentless quest board panel should survive", failures)
	_assert(not kept_ids.has("old-scroll"), "the banished agent's panel should not survive", failures)


## An empty agent_id (e.g. the quest board) never names an agent, so it is
## never dropped regardless of the known-agent list.
func _run_filter_keeps_agentless_entries_case(failures: Array[String]) -> void:
	var arrangement: Dictionary = {"panels": [{"panel_id": "quest-board", "agent_id": "", "position": [0.0, 0.0], "size": [1.0, 1.0]}]}
	var filtered: Dictionary = PanelManagerMath.filter_arrangement_for_agents(arrangement, [])
	_assert(filtered.get("panels", []).size() == 1, "an agentless entry should survive even an empty known-agent list", failures)


## Acceptance: "Open a scroll from the list" — the row list carries name,
## provider accent, and state for each live agent, sorted for a stable
## listing.
func _run_build_agent_rows_case(failures: Array[String]) -> void:
	var agents: Array = [_make_agent("agent-b", "gemini", "working"), _make_agent("agent-a", "claude", "idle")]
	var rows: Array[Dictionary] = PanelsFlyoutMath.build_agent_rows(agents)
	_assert(rows.size() == 2, "expected 2 rows extracted, got %d" % rows.size(), failures)
	if rows.size() == 2:
		_assert(rows[0].get("agent_id", "") == "agent-a", "rows should be sorted by agent id, expected agent-a first, got %s" % rows[0].get("agent_id", ""), failures)
		_assert((rows[0].get("label", "") as String).contains("idle"), "the row label should carry the agent's state", failures)
		_assert((rows[0].get("label", "") as String).contains("claude"), "the row label should carry the agent's provider", failures)


## A malformed snapshot entry must never crash the row-building step.
func _run_build_agent_rows_skips_non_agent_case(failures: Array[String]) -> void:
	var agents: Array = [_make_agent("agent-a"), "not an agent", null]
	var rows: Array[Dictionary] = PanelsFlyoutMath.build_agent_rows(agents)
	_assert(rows.size() == 1, "expected malformed entries skipped, got %d rows" % rows.size(), failures)
