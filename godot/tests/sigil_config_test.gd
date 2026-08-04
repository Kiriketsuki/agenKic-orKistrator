# sigil_config_test.gd — Regression guard for F3 Grimoire Summoning's sigil
# config persistence (SigilConfig round-trip to user://orki_settings.cfg).
#
# Standalone headless script:
#   godot --headless --path godot --script tests/sigil_config_test.gd
#
# Exits 1 on any failure. Uses a dedicated user:// path (not the real
# orki_settings.cfg) so the test never clobbers a keeper's actual config.

extends SceneTree

const TEST_PATH: String = "user://sigil_config_test.cfg"


func _init() -> void:
	var failures: Array[String] = []
	_cleanup()
	_run_missing_file_defaults_case(failures)
	_run_round_trip_case(failures)
	_run_is_enabled_case(failures)
	_run_disable_materializes_enabled_case(failures)
	_run_tier_and_name_pool_case(failures)
	_run_max_scroll_panels_case(failures)
	_run_scroll_size_case(failures)
	_cleanup()
	if failures.is_empty():
		print("sigil_config_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("sigil_config_test: FAIL — " + message)
		quit(1)


func _cleanup() -> void:
	if FileAccess.file_exists(TEST_PATH):
		DirAccess.remove_absolute(TEST_PATH)


func _assert(condition: bool, message: String, failures: Array[String]) -> void:
	if not condition:
		failures.append(message)


## A first-run keeper with no config file yet must get a usable (show-all)
## default rather than an error.
func _run_missing_file_defaults_case(failures: Array[String]) -> void:
	var config: SigilConfig = SigilConfig.load_from_file(TEST_PATH)
	_assert(config.enabled.is_empty(), "a missing config file should yield an empty (show-all) enabled list", failures)
	_assert(config.order.is_empty(), "a missing config file should yield an empty order list", failures)
	_assert(config.provider_defaults.is_empty(), "a missing config file should yield empty provider_defaults", failures)
	_assert(config.workdir.is_empty(), "a missing config file should yield an empty workdir (server default)", failures)


## Acceptance: every field round-trips through save_to_file/load_from_file.
func _run_round_trip_case(failures: Array[String]) -> void:
	var config: SigilConfig = SigilConfig.new()
	config.enabled = ["claude", "codex"]
	config.order = ["codex", "claude", "sim"]
	config.set_tier_default("claude", "opus")
	config.set_name_pool("claude", ["Emberwick", "Thornquill"])
	config.workdir = "/home/keeper/dev/spellbook"
	var save_err: int = config.save_to_file(TEST_PATH)
	_assert(save_err == OK, "save_to_file should return OK, got %d" % save_err, failures)

	var reloaded: SigilConfig = SigilConfig.load_from_file(TEST_PATH)
	_assert(reloaded.enabled == ["claude", "codex"], "enabled should round-trip, got %s" % str(reloaded.enabled), failures)
	_assert(reloaded.order == ["codex", "claude", "sim"], "order should round-trip, got %s" % str(reloaded.order), failures)
	_assert(reloaded.tier_default("claude") == "opus", "tier_default(claude) should round-trip to opus, got %s" % reloaded.tier_default("claude"), failures)
	_assert(reloaded.name_pool("claude") == ["Emberwick", "Thornquill"], "name_pool(claude) should round-trip, got %s" % str(reloaded.name_pool("claude")), failures)
	_assert(reloaded.workdir == "/home/keeper/dev/spellbook", "workdir should round-trip, got %s" % reloaded.workdir, failures)


## is_enabled: empty set means show-all; a non-empty set is exact membership.
func _run_is_enabled_case(failures: Array[String]) -> void:
	var config: SigilConfig = SigilConfig.new()
	_assert(config.is_enabled("claude"), "an empty enabled set should report every kind as enabled", failures)
	config.enabled = ["claude"]
	_assert(config.is_enabled("claude"), "claude should be enabled when explicitly listed", failures)
	_assert(not config.is_enabled("codex"), "codex should not be enabled when only claude is listed", failures)


## Disabling one provider out of an implicit show-all set must not silently
## hide every other provider too.
func _run_disable_materializes_enabled_case(failures: Array[String]) -> void:
	var config: SigilConfig = SigilConfig.new()
	var all_kinds: Array = ["sim", "claude", "codex", "opencode", "pi"]
	config.expand_enabled_for_disable(all_kinds)
	config.set_enabled("codex", false)
	_assert(not config.is_enabled("codex"), "codex should be disabled after set_enabled(codex, false)", failures)
	_assert(config.is_enabled("claude"), "claude should remain enabled after disabling only codex", failures)
	_assert(config.is_enabled("sim"), "sim should remain enabled after disabling only codex", failures)


## Per-provider tier and name-pool defaults are independent per kind.
func _run_tier_and_name_pool_case(failures: Array[String]) -> void:
	var config: SigilConfig = SigilConfig.new()
	_assert(config.tier_default("claude") == "", "an unconfigured provider's tier_default should be empty", failures)
	_assert(config.name_pool("claude").is_empty(), "an unconfigured provider's name_pool should be empty", failures)
	config.set_tier_default("claude", "adept")
	config.set_tier_default("codex", "novice")
	_assert(config.tier_default("claude") == "adept", "claude tier_default should be adept, got %s" % config.tier_default("claude"), failures)
	_assert(config.tier_default("codex") == "novice", "codex tier_default should stay independent (novice), got %s" % config.tier_default("codex"), failures)


## max_scroll_panels round-trips and is clamped into the supported range on
## BOTH save and load, so a hand-edited config file can never wedge the UI
## at zero open panels (or an unbounded number of them).
func _run_max_scroll_panels_case(failures: Array[String]) -> void:
	var config: SigilConfig = SigilConfig.new()
	_assert(config.max_scroll_panels == SigilConfig.DEFAULT_SCROLL_PANELS,
		"a fresh config should default to %d open panels, got %d" % [SigilConfig.DEFAULT_SCROLL_PANELS, config.max_scroll_panels], failures)

	config.max_scroll_panels = 3
	config.save_to_file(TEST_PATH)
	var reloaded: SigilConfig = SigilConfig.load_from_file(TEST_PATH)
	_assert(reloaded.max_scroll_panels == 3, "max_scroll_panels should round-trip as 3, got %d" % reloaded.max_scroll_panels, failures)

	# Out-of-range values clamp rather than persisting a wedged UI.
	config.max_scroll_panels = 99
	config.save_to_file(TEST_PATH)
	var clamped_high: SigilConfig = SigilConfig.load_from_file(TEST_PATH)
	_assert(clamped_high.max_scroll_panels == SigilConfig.MAX_SCROLL_PANELS,
		"an over-range max_scroll_panels should clamp to %d, got %d" % [SigilConfig.MAX_SCROLL_PANELS, clamped_high.max_scroll_panels], failures)

	config.max_scroll_panels = 0
	config.save_to_file(TEST_PATH)
	var clamped_low: SigilConfig = SigilConfig.load_from_file(TEST_PATH)
	_assert(clamped_low.max_scroll_panels == SigilConfig.MIN_SCROLL_PANELS,
		"a zero max_scroll_panels should clamp to %d, got %d" % [SigilConfig.MIN_SCROLL_PANELS, clamped_low.max_scroll_panels], failures)


## Chat panel size ratios round-trip and clamp on both save and load, so a
## hand-edited config can never open a panel unusably small or off-screen.
func _run_scroll_size_case(failures: Array[String]) -> void:
	var config: SigilConfig = SigilConfig.new()
	_assert(is_equal_approx(config.scroll_width_ratio, SigilConfig.DEFAULT_SCROLL_WIDTH_RATIO),
		"a fresh config should default to the original 40%% width, got %f" % config.scroll_width_ratio, failures)
	_assert(is_equal_approx(config.scroll_height_ratio, SigilConfig.DEFAULT_SCROLL_HEIGHT_RATIO),
		"a fresh config should default to full height, got %f" % config.scroll_height_ratio, failures)

	config.scroll_width_ratio = 0.55
	config.scroll_height_ratio = 0.7
	config.save_to_file(TEST_PATH)
	var reloaded: SigilConfig = SigilConfig.load_from_file(TEST_PATH)
	_assert(is_equal_approx(reloaded.scroll_width_ratio, 0.55), "width ratio should round-trip, got %f" % reloaded.scroll_width_ratio, failures)
	_assert(is_equal_approx(reloaded.scroll_height_ratio, 0.7), "height ratio should round-trip, got %f" % reloaded.scroll_height_ratio, failures)

	config.scroll_width_ratio = 5.0
	config.scroll_height_ratio = 0.01
	config.save_to_file(TEST_PATH)
	var clamped: SigilConfig = SigilConfig.load_from_file(TEST_PATH)
	_assert(is_equal_approx(clamped.scroll_width_ratio, SigilConfig.MAX_SCROLL_RATIO),
		"an over-range width should clamp to %f, got %f" % [SigilConfig.MAX_SCROLL_RATIO, clamped.scroll_width_ratio], failures)
	_assert(is_equal_approx(clamped.scroll_height_ratio, SigilConfig.MIN_SCROLL_RATIO),
		"an under-range height should clamp to %f, got %f" % [SigilConfig.MIN_SCROLL_RATIO, clamped.scroll_height_ratio], failures)
