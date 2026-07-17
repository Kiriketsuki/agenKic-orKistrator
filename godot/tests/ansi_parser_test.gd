# ansi_parser_test.gd — Regression guard for the T10 ANSI parser refactor.
#
# No GUT (or other Godot test runner) is vendored in this project yet, so this
# is a standalone script runnable headless:
#
#   godot --headless --path godot --script tests/ansi_parser_test.gd
#
# It asserts (a) AnsiSepiaParser.to_bbcode() output is byte-identical to the
# pre-refactor implementation for representative inputs (regression guard for
# the AnsiSgrScanner extraction), and (b) AnsiSgrScanner.to_bbcode() produces
# correct BBCode against the new STANDARD_PALETTE used by the T10 terminal
# fallback. Exits with code 1 on any failure so it can be wired into CI later.

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_sepia_regression_cases(failures)
	_run_standard_palette_cases(failures)
	if failures.is_empty():
		print("ansi_parser_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("ansi_parser_test: FAIL — " + message)
		quit(1)


func _run_sepia_regression_cases(failures: Array[String]) -> void:
	# Each pair is (raw input, expected output captured from the pre-refactor
	# AnsiSepiaParser.to_bbcode() implementation) — byte-identical regression
	# guard for the AnsiSgrScanner extraction.
	var cases: Array = [
		["", ""],
		["plain text", "[color=#3b2a1a]plain text[/color]"],
		["\x1b[31mred\x1b[0m", "[color=#8b3a2b]red[/color]"],
		["\x1b[1;32mbold green\x1b[0m", "[color=#6b7a3d][b]bold green[/b][/color]"],
		["a[b]c", "[color=#3b2a1a]a[lb]b[rb]c[/color]"],
		["\x1b[2J\x1b[Hcleared", "[color=#3b2a1a]cleared[/color]"],
		["\x1b[39mdefault fg", "[color=#3b2a1a]default fg[/color]"],
		["line\rreturn", "[color=#3b2a1a]linereturn[/color]"],
	]
	for case: Array in cases:
		var raw: String = case[0]
		var expected: String = case[1]
		var actual: String = AnsiSepiaParser.to_bbcode(raw)
		if actual != expected:
			failures.append("sepia case %s: expected %s got %s" % [raw, expected, actual])


func _run_standard_palette_cases(failures: Array[String]) -> void:
	var cases: Array = [
		["", ""],
		["plain text", "[color=#e5e5e5]plain text[/color]"],
		["\x1b[31mred\x1b[0m", "[color=#e06c75]red[/color]"],
		["\x1b[1;94mbold blue\x1b[0m", "[color=#82c0ff][b]bold blue[/b][/color]"],
		["\x1b[39mdefault fg", "[color=#e5e5e5]default fg[/color]"],
	]
	for case: Array in cases:
		var raw: String = case[0]
		var expected: String = case[1]
		var actual: String = AnsiSgrScanner.to_bbcode(raw, AnsiSgrScanner.STANDARD_PALETTE, AnsiSgrScanner.DEFAULT_STANDARD_FG)
		if actual != expected:
			failures.append("standard case %s: expected %s got %s" % [raw, expected, actual])
