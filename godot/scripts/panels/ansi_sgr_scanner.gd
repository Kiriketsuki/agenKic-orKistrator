# ansi_sgr_scanner.gd — Shared CSI-scanning / SGR-folding core extracted from
# ansi_sepia_parser.gd (T9). Converts raw terminal output (with ANSI SGR
# escape codes) into RichTextLabel BBCode against a caller-supplied palette,
# so both the sepia scroll rendering (T9) and the standard-terminal fallback
# rendering (T10) share one scanner instead of duplicating it.
#
# - Literal `[` and `]` in the source text are escaped to `[lb]`/`[rb]` BEFORE any
#   BBCode is emitted around them, so raw agent output can never inject markup.
# - SGR (ESC[...m) runs are tracked for foreground color (30-37, 90-97, 39=reset)
#   and bold (1); everything else collapses to a single reset (0).
# - All other CSI sequences (cursor movement, erase, etc.) and stray `\r` are
#   stripped silently — they have no rendering equivalent in a RichTextLabel.
#
# Pure static functions, no shared mutable state.

class_name AnsiSgrScanner

## Unicode codepoint for ESC (0x1B) — CSI sequences are ESC followed by `[`.
const ESC_CODE: int = 0x1B

## Standard xterm 16-color palette for the standard SGR foreground codes
## (30-37 normal, 90-97 bright). Used by the T10 raw-terminal read-only
## fallback, which renders on a dark background rather than T9's sepia
## parchment.
const STANDARD_PALETTE: Dictionary = {
	30: "#1e1e1e", # black
	31: "#e06c75", # red
	32: "#98c379", # green
	33: "#e5c07b", # yellow
	34: "#61afef", # blue
	35: "#c678dd", # magenta
	36: "#56b6c2", # cyan
	37: "#dcdfe4", # white
	90: "#5c6370", # bright black
	91: "#e78a99", # bright red
	92: "#b1e18b", # bright green
	93: "#f0d090", # bright yellow
	94: "#82c0ff", # bright blue
	95: "#d9a3ec", # bright magenta
	96: "#7fd6e2", # bright cyan
	97: "#f4f6fa", # bright white
}

## Default foreground for the standard palette: used for SGR reset (0), code
## 39 (default fg), and any text before the first SGR sequence — tuned for
## a dark terminal background.
const DEFAULT_STANDARD_FG: String = "#e5e5e5"


## Converts a raw output chunk into BBCode ready for RichTextLabel.append_text(),
## using `palette` for SGR foreground codes 30-37/90-97 and `default_color` for
## reset (0), code 39, and any leading unstyled text.
static func to_bbcode(raw: String, palette: Dictionary, default_color: String) -> String:
	if raw.is_empty():
		return ""
	var out: String = ""
	var length: int = raw.length()
	var i: int = 0
	var current_color: String = default_color
	var bold: bool = false
	var open_tags: bool = false
	while i < length:
		var ch: String = raw[i]
		if ch.unicode_at(0) == ESC_CODE and i + 1 < length and raw[i + 1] == "[":
			var end: int = i + 2
			while end < length and not _is_final_byte(raw[end]):
				end += 1
			if end >= length:
				# Unterminated sequence at the end of this chunk — drop the remainder.
				break
			var final_byte: String = raw[end]
			if final_byte == "m":
				var params_str: String = raw.substr(i + 2, end - (i + 2))
				var result: Dictionary = _apply_sgr(params_str, current_color, bold, palette, default_color)
				var new_color: String = result["color"]
				var new_bold: bool = result["bold"]
				if new_color != current_color or new_bold != bold:
					if open_tags:
						out += _close_tags(bold)
						open_tags = false
					current_color = new_color
					bold = new_bold
			# Every other CSI sequence (cursor movement, erase-line, etc.) is stripped.
			i = end + 1
			continue
		if ch == "\r":
			i += 1
			continue
		if not open_tags:
			out += "[color=%s]" % current_color
			if bold:
				out += "[b]"
			open_tags = true
		if ch == "[":
			out += "[lb]"
		elif ch == "]":
			out += "[rb]"
		else:
			out += ch
		i += 1
	if open_tags:
		out += _close_tags(bold)
	return out


static func _is_final_byte(ch: String) -> bool:
	# CSI sequences terminate on a byte in the range 0x40-0x7E (@ through ~).
	var code: int = ch.unicode_at(0)
	return code >= 0x40 and code <= 0x7E


static func _close_tags(bold: bool) -> String:
	var s: String = ""
	if bold:
		s += "[/b]"
	s += "[/color]"
	return s


## Folds a `;`-separated SGR parameter list onto the running (color, bold) state.
static func _apply_sgr(
	params_str: String, current_color: String, current_bold: bool, palette: Dictionary, default_color: String
) -> Dictionary:
	var color: String = current_color
	var bold: bool = current_bold
	var parts: PackedStringArray = params_str.split(";")
	if parts.size() == 1 and parts[0] == "":
		parts = PackedStringArray(["0"])
	for part: String in parts:
		var code: int = part.to_int() if not part.is_empty() else 0
		if code == 0:
			color = default_color
			bold = false
		elif code == 1:
			bold = true
		elif code == 39:
			color = default_color
		elif palette.has(code):
			color = palette[code]
	return {"color": color, "bold": bold}
