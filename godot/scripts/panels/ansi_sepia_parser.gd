# ansi_sepia_parser.gd — Converts raw terminal output (with ANSI SGR escape codes)
# into RichTextLabel BBCode, mapped onto a warm sepia "fantasy ink" palette.
#
# - Literal `[` and `]` in the source text are escaped to `[lb]`/`[rb]` BEFORE any
#   BBCode is emitted around them, so raw agent output can never inject markup.
# - SGR (ESC[...m) runs are tracked for foreground color (30-37, 90-97, 39=reset)
#   and bold (1); everything else collapses to a single reset (0).
# - All other CSI sequences (cursor movement, erase, etc.) and stray `\r` are
#   stripped silently — they have no sepia-ink equivalent.

class_name AnsiSepiaParser

## Unicode codepoint for ESC (0x1B) — CSI sequences are ESC followed by `[`.
const ESC_CODE: int = 0x1B

## Warm/sepia mapping for standard + bright SGR foreground codes.
const PALETTE: Dictionary = {
	30: "#4a3728", # black -> dark umber
	31: "#8b3a2b", # red -> burnt sienna
	32: "#6b7a3d", # green -> olive
	33: "#a67c3d", # yellow -> amber
	34: "#4a5a6b", # blue -> faded indigo
	35: "#7a4a6b", # magenta -> plum
	36: "#3d6b6b", # cyan -> teal
	37: "#5c4a3a", # white -> warm parchment ink
	90: "#6b5a4a", # bright black -> soft umber
	91: "#a3503c", # bright red -> clay
	92: "#7d8c52", # bright green -> sage
	93: "#b8925a", # bright yellow -> honey
	94: "#5c6f82", # bright blue -> slate indigo
	95: "#8f5c82", # bright magenta -> dusty plum
	96: "#4f8282", # bright cyan -> muted teal
	97: "#6e5c4a", # bright white -> faded ink
}

## Default ink color: used for SGR reset (0), code 39 (default fg), and any
## text that appears before the first SGR sequence.
const DEFAULT_INK: String = "#3b2a1a"


## Converts a raw output chunk into sepia-tinted BBCode ready for
## RichTextLabel.append_text().
static func to_bbcode(raw: String) -> String:
	if raw.is_empty():
		return ""
	var out: String = ""
	var length: int = raw.length()
	var i: int = 0
	var current_color: String = DEFAULT_INK
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
				var result: Dictionary = _apply_sgr(params_str, current_color, bold)
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
static func _apply_sgr(params_str: String, current_color: String, current_bold: bool) -> Dictionary:
	var color: String = current_color
	var bold: bool = current_bold
	var parts: PackedStringArray = params_str.split(";")
	if parts.size() == 1 and parts[0] == "":
		parts = PackedStringArray(["0"])
	for part: String in parts:
		var code: int = part.to_int() if not part.is_empty() else 0
		if code == 0:
			color = DEFAULT_INK
			bold = false
		elif code == 1:
			bold = true
		elif code == 39:
			color = DEFAULT_INK
		elif PALETTE.has(code):
			color = PALETTE[code]
	return {"color": color, "bold": bold}
