# provider_roster_test.gd — Regression guard for F3 Grimoire Summoning's
# GET /api/providers response parsing and the config-driven filter/order
# pass (ProviderRoster).
#
# Standalone headless script:
#   godot --headless --path godot --script tests/provider_roster_test.gd
#
# Exits 1 on any failure.

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_parse_response_case(failures)
	_run_parse_malformed_case(failures)
	_run_parse_skips_kindless_entries_case(failures)
	_run_filter_empty_enabled_shows_all_case(failures)
	_run_filter_respects_enabled_case(failures)
	_run_order_case(failures)
	_run_order_unlisted_kinds_after_case(failures)
	if failures.is_empty():
		print("provider_roster_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("provider_roster_test: FAIL — " + message)
		quit(1)


func _assert(condition: bool, message: String, failures: Array[String]) -> void:
	if not condition:
		failures.append(message)


func _sample_response() -> Dictionary:
	return {
		"providers": [
			{"kind": "sim", "display": "Simulacrum", "accent": "#8892b0"},
			{"kind": "claude", "display": "Claude", "accent": "#cc785c"},
			{"kind": "codex", "display": "Codex", "accent": "#10a37f"},
			{"kind": "opencode", "display": "OpenCode", "accent": "#4f8cff"},
			{"kind": "pi", "display": "Pi", "accent": "#b967ff"},
		]
	}


## Acceptance: GET /api/providers's real 5-entry shape parses fully, in order.
func _run_parse_response_case(failures: Array[String]) -> void:
	var parsed: Array = ProviderRoster.parse_response(_sample_response())
	_assert(parsed.size() == 5, "expected 5 providers parsed, got %d" % parsed.size(), failures)
	if parsed.size() == 5:
		_assert(parsed[0]["kind"] == "sim", "first parsed kind should be sim, got %s" % parsed[0]["kind"], failures)
		_assert(parsed[1]["display"] == "Claude", "second parsed display should be Claude, got %s" % parsed[1]["display"], failures)
		_assert(parsed[2]["accent"] == "#10a37f", "third parsed accent should be #10a37f, got %s" % parsed[2]["accent"], failures)


## A malformed or empty response body must never crash the flyout.
func _run_parse_malformed_case(failures: Array[String]) -> void:
	_assert(ProviderRoster.parse_response(null).is_empty(), "parse_response(null) should return empty", failures)
	_assert(ProviderRoster.parse_response("not a dict").is_empty(), "parse_response(string) should return empty", failures)
	_assert(ProviderRoster.parse_response({}).is_empty(), "parse_response({}) should return empty", failures)
	_assert(ProviderRoster.parse_response({"providers": "not an array"}).is_empty(), "parse_response with non-array providers should return empty", failures)


## An entry missing "kind" is dropped rather than producing an unusable sigil.
func _run_parse_skips_kindless_entries_case(failures: Array[String]) -> void:
	var parsed: Array = ProviderRoster.parse_response({
		"providers": [
			{"display": "No Kind"},
			{"kind": "sim", "display": "Simulacrum"},
		]
	})
	_assert(parsed.size() == 1, "kindless entry should be skipped, expected 1 survivor, got %d" % parsed.size(), failures)


## Acceptance: an empty enabled set means show-all (a fresh install must not
## hide the whole grid).
func _run_filter_empty_enabled_shows_all_case(failures: Array[String]) -> void:
	var providers: Array = ProviderRoster.parse_response(_sample_response())
	var result: Array = ProviderRoster.filtered_and_ordered(providers, [], [])
	_assert(result.size() == 5, "empty enabled set should keep all 5 providers, got %d" % result.size(), failures)


## Acceptance: disabled providers stay hidden from the grid.
func _run_filter_respects_enabled_case(failures: Array[String]) -> void:
	var providers: Array = ProviderRoster.parse_response(_sample_response())
	var result: Array = ProviderRoster.filtered_and_ordered(providers, ["claude", "codex"], [])
	_assert(result.size() == 2, "enabled=[claude, codex] should keep 2 providers, got %d" % result.size(), failures)
	var kinds: Array = result.map(func(p: Dictionary) -> String: return p["kind"])
	_assert(kinds.has("claude") and kinds.has("codex"), "filtered result should contain claude and codex, got %s" % str(kinds), failures)
	_assert(not kinds.has("sim"), "filtered result should not contain sim, got %s" % str(kinds), failures)


## The config's order list drives the grid's left-to-right sequence.
func _run_order_case(failures: Array[String]) -> void:
	var providers: Array = ProviderRoster.parse_response(_sample_response())
	var result: Array = ProviderRoster.filtered_and_ordered(providers, [], ["pi", "sim"])
	_assert(result.size() == 5, "order pass should not drop any provider, got %d" % result.size(), failures)
	_assert(result[0]["kind"] == "pi", "first ordered kind should be pi, got %s" % result[0]["kind"], failures)
	_assert(result[1]["kind"] == "sim", "second ordered kind should be sim, got %s" % result[1]["kind"], failures)


## Kinds absent from `order` sort after the ordered ones, in roster order.
func _run_order_unlisted_kinds_after_case(failures: Array[String]) -> void:
	var providers: Array = ProviderRoster.parse_response(_sample_response())
	var result: Array = ProviderRoster.filtered_and_ordered(providers, [], ["codex"])
	_assert(result[0]["kind"] == "codex", "first ordered kind should be codex, got %s" % result[0]["kind"], failures)
	_assert(result[1]["kind"] == "sim", "unlisted kinds should follow in roster order, expected sim next, got %s" % result[1]["kind"], failures)
