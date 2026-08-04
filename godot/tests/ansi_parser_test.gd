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
	_run_osc_cases(failures)
	_run_trim_cases(failures)
	_run_chrome_strip_cases(failures)
	_run_rune_filter_cases(failures)
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
		["\u001b[31mred\u001b[0m", "[color=#8b3a2b]red[/color]"],
		["\u001b[1;32mbold green\u001b[0m", "[color=#6b7a3d][b]bold green[/b][/color]"],
		["a[b]c", "[color=#3b2a1a]a[lb]b[rb]c[/color]"],
		["\u001b[2J\u001b[Hcleared", "[color=#3b2a1a]cleared[/color]"],
		["\u001b[39mdefault fg", "[color=#3b2a1a]default fg[/color]"],
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
		["\u001b[31mred\u001b[0m", "[color=#e06c75]red[/color]"],
		["\u001b[1;94mbold blue\u001b[0m", "[color=#82c0ff][b]bold blue[/b][/color]"],
		["\u001b[39mdefault fg", "[color=#e5e5e5]default fg[/color]"],
	]
	for case: Array in cases:
		var raw: String = case[0]
		var expected: String = case[1]
		var actual: String = AnsiSgrScanner.to_bbcode(raw, AnsiSgrScanner.STANDARD_PALETTE, AnsiSgrScanner.DEFAULT_STANDARD_FG)
		if actual != expected:
			failures.append("standard case %s: expected %s got %s" % [raw, expected, actual])


## OSC sequences must be consumed whole. Claude Code emits OSC 8 hyperlinks
## around its status line, and a CSI-only scanner leaks their payload as
## literal text such as "]8;id=1;https://..." into the panel.
func _run_osc_cases(failures: Array[String]) -> void:
	var esc: String = char(0x1B)
	var bel: String = char(0x07)
	var cases: Array = [
		# OSC 8 hyperlink terminated by ST, visible link text kept.
		[
			esc + "]8;id=1vsv0hl;https://example.com/x" + esc + "\\" + "link text" + esc + "]8;;" + esc + "\\",
			"[color=#e5e5e5]link text[/color]",
		],
		# OSC 8 terminated by BEL.
		[
			esc + "]8;;https://example.com" + bel + "text" + esc + "]8;;" + bel,
			"[color=#e5e5e5]text[/color]",
		],
		# OSC 0 window title, no visible text at all.
		[esc + "]0;a title" + bel + "after", "[color=#e5e5e5]after[/color]"],
		# Color state survives across an OSC sequence.
		[
			esc + "[31mred" + esc + "]0;t" + bel + "still red" + esc + "[0m",
			"[color=#e06c75]redstill red[/color]",
		],
		# Unterminated OSC at the end of a chunk drops the remainder.
		[esc + "]8;;https://example.com", ""],
		# Charset designation escape takes three bytes in total.
		[esc + "(Bplain", "[color=#e5e5e5]plain[/color]"],
		# A stray BEL never renders.
		["ding" + bel, "[color=#e5e5e5]ding[/color]"],
	]
	for case: Array in cases:
		var raw: String = case[0]
		var expected: String = case[1]
		var actual: String = AnsiSgrScanner.to_bbcode(
			raw, AnsiSgrScanner.STANDARD_PALETTE, AnsiSgrScanner.DEFAULT_STANDARD_FG
		)
		if actual != expected:
			failures.append("osc case %s: expected %s got %s" % [raw.c_escape(), expected, actual])


## trim_blank_lines() drops the padding rows that tmux capture-pane returns for
## a short TUI frame. Those rows caused the blank region above the chat body.
func _run_trim_cases(failures: Array[String]) -> void:
	var esc: String = char(0x1B)
	var cases: Array = [
		["", ""],
		["\n\n\ncontent\n\n\n", "content"],
		["a\n\nb", "a\n\nb"],
		["\n\n", ""],
		# A row holding only SGR escapes carries no visible glyph, so it goes.
		[esc + "[0m\n" + esc + "[32mreal" + esc + "[0m\n   \n", esc + "[32mreal" + esc + "[0m"],
	]
	for case: Array in cases:
		var raw: String = case[0]
		var expected: String = case[1]
		var actual: String = AnsiSgrScanner.trim_blank_lines(raw)
		if actual != expected:
			failures.append(
				"trim case %s: expected %s got %s" % [raw.c_escape(), expected.c_escape(), actual.c_escape()]
			)


## strip_bottom_chrome() drops the CLI input box and statusline at the pane
## bottom before the scroll's sepia render.
func _run_chrome_strip_cases(failures: Array[String]) -> void:
	var rule: String = "─".repeat(40)
	var chrome: String = "\n".join([
		rule,
		"❯ ",
		rule,
		"  ── ◆ Claude Fable 5 ── v2.1.221",
		"  usage bars and account line",
	])
	var cases: Array = [
		["", ""],
		# Plain prose stays whole.
		["a\nb\nc", "a\nb\nc"],
		# The chrome block goes, the prose above stays.
		["prose one\nprose two\n" + chrome, "prose one\nprose two"],
		# Trailing blank rows after the chrome do not hide it.
		["prose\n" + chrome + "\n\n\n", "prose"],
		# A rule line far above the bottom window stays, only the bottom
		# block goes.
		[rule + "\n" + "x\n".repeat(14) + chrome, rule + "\n" + "x\n".repeat(14).trim_suffix("\n")],
	]
	for case: Array in cases:
		var raw: String = case[0]
		var expected: String = case[1]
		var actual: String = AnsiSgrScanner.strip_bottom_chrome(raw)
		if actual != expected:
			failures.append(
				"chrome case %s: expected %s got %s" % [raw.c_escape(), expected.c_escape(), actual.c_escape()]
			)


## RuneFilter compiled its ANSI pattern with a PCRE2-invalid escape, so every
## output chunk threw. Guard the compile and the OSC strip.
func _run_rune_filter_cases(failures: Array[String]) -> void:
	var esc: String = char(0x1B)
	var bel: String = char(0x07)
	var chunk: BridgeData.AgentOutputChunk = BridgeData.AgentOutputChunk.new()
	chunk.agent_id = "rune-filter-test"
	chunk.significant = true
	chunk.payload = (
		esc + "[31m" + esc + "]8;;https://example.com" + bel + "error in main.go"
		+ esc + "]8;;" + bel + esc + "[0m"
	)
	RuneFilter.reset_rate_limits()
	var result: Dictionary = RuneFilter.process(chunk)
	if not bool(result.get(&"show", false)):
		failures.append("rune filter case: expected the chunk to show")
		return
	var text: String = result[&"text"]
	if text != "error in main.go":
		failures.append("rune filter case: expected 'error in main.go' got %s" % text.c_escape())
