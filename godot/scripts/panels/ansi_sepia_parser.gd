# ansi_sepia_parser.gd — Converts raw terminal output (with ANSI SGR escape codes)
# into RichTextLabel BBCode, mapped onto a warm sepia "fantasy ink" palette.
#
# The CSI-scanning / SGR-folding core is shared with the T10 standard-terminal
# fallback via AnsiSgrScanner (ansi_sgr_scanner.gd) — this file now holds only
# the sepia PALETTE/DEFAULT_INK constants and delegates. Public API (to_bbcode
# signature) is unchanged so SpellScrollView needs no changes.

class_name AnsiSepiaParser

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
	return AnsiSgrScanner.to_bbcode(raw, PALETTE, DEFAULT_INK)
