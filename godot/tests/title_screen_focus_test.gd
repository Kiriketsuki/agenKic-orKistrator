# title_screen_focus_test.gd — Regression guard for F1 (OrKi title screen)
# menu focus-order and activation logic.
#
# No GUT (or other Godot test runner) is vendored in this project yet, so
# this is a standalone script runnable headless, mirroring
# floor_morph_test.gd:
#
#   godot --headless --path godot --script tests/title_screen_focus_test.gd
#
# TitleFocusMath.next_focus_index() is the pure-function core of the menu's
# arrow-key navigation. It lives in its own autoload-free script
# (title_focus_math.gd) precisely so this test can preload it without also
# preloading title_screen.gd, which preloads the BridgeManager autoload at
# parse time and fails to compile under `--script` (BridgeManager is not
# resolved outside a full Godot boot). Preloading title_screen.gd here
# previously made every case in this file error out silently, while the
# suite still printed "all cases passed" and exited 0 — this file's
# case-count check below guards against that regressing again.
#
# TitleMenuEntry now lives on title_focus_math.gd too (TitleScreen mirrors
# it), so the enum-ordering check below reads it off the same preloaded,
# autoload-free script instead of touching the TitleScreen global class name
# — referencing TitleScreen here would still force-compile title_screen.gd
# and reintroduce the same hazard, even without a direct preload of it.
#
# Exits 1 on any failure, or if fewer cases ran than expected, so it can be
# wired into CI later.

extends SceneTree

const TitleFocusMathScript: Script = preload("res://scripts/ui/title_focus_math.gd")
## TitleFocusMath carries no autoload dependency, unlike title_screen.gd. The
## preload above still loads the .gd file explicitly so this test fails
## loudly (missing file) rather than silently (unresolved global) if the
## script ever moves.

## Total number of individual assertions this suite is expected to run.
## Each _run_*_case function below increments _ran_count once per case it
## exercises. If a case is skipped (e.g. a script failed to compile and a
## call errored out before incrementing), ran_count falls short of this
## total and the suite fails closed instead of reporting a false green.
const EXPECTED_CASE_COUNT: int = 9

var _ran_count: int = 0


func _init() -> void:
	var failures: Array[String] = []
	_run_wrap_forward_cases(failures)
	_run_wrap_backward_cases(failures)
	_run_menu_entry_enum_cases(failures)
	if _ran_count != EXPECTED_CASE_COUNT:
		failures.append(
			"expected %d cases to run, only %d ran — a script error likely aborted the suite early" % [EXPECTED_CASE_COUNT, _ran_count]
		)
	if failures.is_empty():
		print("title_screen_focus_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("title_screen_focus_test: FAIL — " + message)
		quit(1)


## Down-arrow (direction +1) steps through ENTER -> GRIMOIRE -> DEPART and
## wraps back to ENTER.
func _run_wrap_forward_cases(failures: Array[String]) -> void:
	var cases: Array = [
		[0, 1], [1, 2], [2, 0],
	]
	for case: Array in cases:
		var current: int = case[0]
		var expected: int = case[1]
		var actual: int = TitleFocusMathScript.next_focus_index(current, 3, 1)
		_ran_count += 1
		if actual != expected:
			failures.append(
				"next_focus_index(%d, 3, +1): expected %d got %d" % [current, expected, actual]
			)


## Up-arrow (direction -1) steps backward and wraps from the first entry to
## the last.
func _run_wrap_backward_cases(failures: Array[String]) -> void:
	var cases: Array = [
		[2, 1], [1, 0], [0, 2],
	]
	for case: Array in cases:
		var current: int = case[0]
		var expected: int = case[1]
		var actual: int = TitleFocusMathScript.next_focus_index(current, 3, -1)
		_ran_count += 1
		if actual != expected:
			failures.append(
				"next_focus_index(%d, 3, -1): expected %d got %d" % [current, expected, actual]
			)


## The menu entry enum must stay ENTER, GRIMOIRE, DEPART in that order —
## the scene wires _menu_entries in the same order, and a reorder here
## without a matching scene edit would silently misroute activation.
func _run_menu_entry_enum_cases(failures: Array[String]) -> void:
	_ran_count += 1
	if TitleFocusMathScript.TitleMenuEntry.ENTER != 0:
		failures.append("TitleMenuEntry.ENTER expected 0")
	_ran_count += 1
	if TitleFocusMathScript.TitleMenuEntry.GRIMOIRE != 1:
		failures.append("TitleMenuEntry.GRIMOIRE expected 1")
	_ran_count += 1
	if TitleFocusMathScript.TitleMenuEntry.DEPART != 2:
		failures.append("TitleMenuEntry.DEPART expected 2")
