# title_screen_focus_test.gd — Regression guard for F1 (OrKi title screen)
# menu focus-order and activation logic.
#
# No GUT (or other Godot test runner) is vendored in this project yet, so
# this is a standalone script runnable headless, mirroring
# floor_morph_test.gd:
#
#   godot --headless --path godot --script tests/title_screen_focus_test.gd
#
# TitleScreen.next_focus_index() is the pure-function core of the menu's
# arrow-key navigation, extracted so the wrap-around math can run without a
# live Control tree or a viewport. This test exercises it directly, plus the
# TitleMenuEntry enum ordering the menu and _activate_entry() rely on.
#
# Exits 1 on any failure so it can be wired into CI later.

extends SceneTree

const TitleScreenScript: Script = preload("res://scripts/ui/title_screen.gd")
## TitleScreen is a class_name-registered global. The preload above still
## loads the .gd file explicitly so this test fails loudly (missing file)
## rather than silently (unresolved global) if the script ever moves.


func _init() -> void:
	var failures: Array[String] = []
	_run_wrap_forward_cases(failures)
	_run_wrap_backward_cases(failures)
	_run_menu_entry_enum_cases(failures)
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
		var actual: int = TitleScreen.next_focus_index(current, 3, 1)
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
		var actual: int = TitleScreen.next_focus_index(current, 3, -1)
		if actual != expected:
			failures.append(
				"next_focus_index(%d, 3, -1): expected %d got %d" % [current, expected, actual]
			)


## The menu entry enum must stay ENTER, GRIMOIRE, DEPART in that order —
## the scene wires _menu_entries in the same order, and a reorder here
## without a matching scene edit would silently misroute activation.
func _run_menu_entry_enum_cases(failures: Array[String]) -> void:
	if TitleScreen.TitleMenuEntry.ENTER != 0:
		failures.append("TitleMenuEntry.ENTER expected 0")
	if TitleScreen.TitleMenuEntry.GRIMOIRE != 1:
		failures.append("TitleMenuEntry.GRIMOIRE expected 1")
	if TitleScreen.TitleMenuEntry.DEPART != 2:
		failures.append("TitleMenuEntry.DEPART expected 2")
