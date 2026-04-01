# rune_filter.gd — Static utility for filtering and processing agent output lines
# before they become floating runes in the tower UI.

class_name RuneFilter

## Rate limit: minimum ms between fallback (non-significant) runes per agent.
const RATE_LIMIT_MS: int = 2000

## Max display length for rune text.
const MAX_CHARS: int = 40

# --- Lazy-init regex storage ---
# GDScript 4 static vars allow lazy init inside static functions.
static var _ansi_re: RegEx
static var _timestamp_re: RegEx
static var _loglevel_re: RegEx
static var _keyword_re: RegEx

## Per-agent last fallback emission time (agent_id → float ms).
static var _last_fallback_time: Dictionary = {}

# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------

## Process an output chunk.
## Returns {show: true, text: String, keywords: PackedStringArray}
## or      {show: false}
static func process(chunk: BridgeData.AgentOutputChunk) -> Dictionary:
	var text: String = chunk.payload
	var is_significant: bool = chunk.significant

	# Step 1 — ANSI strip
	text = _strip_ansi(text)

	# Step 2 — log-level / timestamp strip
	text = _strip_log_prefix(text)

	# Step 3 — insignificance check (skip for significant lines)
	if not is_significant and _is_insignificant(text):
		return {&"show": false}

	# Step 4 — keyword extraction
	var keywords: PackedStringArray = _extract_keywords(text)

	# Step 5 — fallback keyword gate (non-significant must have at least one keyword)
	if not is_significant and keywords.is_empty():
		return {&"show": false}

	# Step 6 — rate limit (fallback only)
	if not is_significant:
		var now: float = float(Time.get_ticks_msec())
		var last: float = _last_fallback_time.get(chunk.agent_id, -INF)
		if now - last < float(RATE_LIMIT_MS):
			return {&"show": false}
		_last_fallback_time[chunk.agent_id] = now

	# Step 7 — truncate
	text = _truncate(text, keywords)

	return {&"show": true, &"text": text, &"keywords": keywords}


## Clear all rate-limit state (useful for tests or scene resets).
static func reset_rate_limits() -> void:
	_last_fallback_time.clear()


# ---------------------------------------------------------------------------
# ANSI stripping
# ---------------------------------------------------------------------------

static func _strip_ansi(text: String) -> String:
	if _ansi_re == null:
		_ansi_re = RegEx.new()
		# Matches ESC [ … letter (CSI sequences) and bare ESC + single char
		_ansi_re.compile("\\u001b(?:\\[[0-9;]*[A-Za-z]|[^\\[\\u001b])")
	var result: String = _ansi_re.sub(text, "", true)
	return result


# ---------------------------------------------------------------------------
# Log-prefix / timestamp stripping
# ---------------------------------------------------------------------------

static func _strip_log_prefix(text: String) -> String:
	if _loglevel_re == null:
		_loglevel_re = RegEx.new()
		# Matches [LEVEL] at line start, optionally followed by whitespace
		_loglevel_re.compile("^\\[(?:INFO|DEBUG|WARN|WARNING|ERROR|CRITICAL|TRACE|FATAL)\\]\\s*")
	if _timestamp_re == null:
		_timestamp_re = RegEx.new()
		# ISO-8601 date-time: 2026-04-01T12:00:00 or 2026-04-01 12:00:00
		_timestamp_re.compile("^\\d{4}-\\d{2}-\\d{2}[T ]\\d{2}:\\d{2}:\\d{2}(?:\\.\\d+)?(?:Z|[+-]\\d{2}:?\\d{2})?\\s*")

	var result: String = text
	# Strip log level first, then timestamp (they can appear in either order)
	result = _loglevel_re.sub(result, "")
	result = _timestamp_re.sub(result, "")
	# If there was a log level after the timestamp, strip it again
	result = _loglevel_re.sub(result, "")
	return result.strip_edges()


# ---------------------------------------------------------------------------
# Insignificant line detection
# ---------------------------------------------------------------------------

static func _is_insignificant(text: String) -> bool:
	var stripped: String = text.strip_edges()

	# Blank or too short
	if stripped.length() < 3:
		return true

	# Progress bar detection: count block characters vs non-space total
	var block_chars: int = 0
	var non_space: int = 0
	for ch: String in stripped:
		if ch != " ":
			non_space += 1
			if ch == "█" or ch == "░" or ch == "▓" or ch == "▒":
				block_chars += 1

	if non_space > 0 and float(block_chars) / float(non_space) > 0.5:
		return true

	return false


# ---------------------------------------------------------------------------
# Keyword extraction
# ---------------------------------------------------------------------------

static func _extract_keywords(text: String) -> PackedStringArray:
	if _keyword_re == null:
		_keyword_re = RegEx.new()
		# Order: filenames, error words, commands, model names
		# filenames: any word ending in a recognised extension
		# error words: case-insensitive
		# commands / models: exact lowercase match as whole word
		_keyword_re.compile(
			"(?i)(?:" +
			"\\b\\w+\\.(?:py|gd|yaml|yml|json|toml|ts|go|rs|js|md|cfg|ini)\\b" +
			"|\\b(?:ERROR|WARN|FAIL|CRITICAL|panic|exception|traceback|fatal)\\b" +
			"|\\b(?:git|go|npm|cargo|docker|pip|godot|make|cmake)\\b" +
			"|\\b(?:claude|gemini|gpt|ollama|deepseek|sonnet|opus|haiku)\\b" +
			")"
		)

	var results: PackedStringArray = PackedStringArray()
	for m: RegExMatch in _keyword_re.search_all(text):
		var word: String = m.get_string()
		# Deduplicate
		if not results.has(word):
			results.append(word)
	return results


# ---------------------------------------------------------------------------
# Truncation
# ---------------------------------------------------------------------------

static func _truncate(text: String, keywords: PackedStringArray) -> String:
	if text.length() <= MAX_CHARS:
		return text

	var cut: int = MAX_CHARS

	# Extend cut point if it falls inside a keyword
	for kw: String in keywords:
		var pos: int = text.find(kw)
		if pos == -1:
			continue
		var kw_end: int = pos + kw.length()
		if pos < cut and kw_end > cut:
			# Cutting here would split this keyword — extend
			cut = kw_end

	if cut >= text.length():
		return text

	return text.substr(0, cut) + "…"
