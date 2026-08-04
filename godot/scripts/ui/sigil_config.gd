class_name SigilConfig
## SigilConfig — persisted keeper preferences for the Grimoire sigil grid
## (F3 Grimoire Summoning): which providers show, in what order, and each
## provider's spawn defaults (tier, name pool). Backed by a ConfigFile at
## user://orki_settings.cfg, section [sigils].
##
## A RefCounted, not a Node, so it has a headless round-trip test
## (tests/sigil_config_test.gd) with no scene tree involved.

const SECTION: String = "sigils"
const DEFAULT_PATH: String = "user://orki_settings.cfg"

## Kinds the keeper wants visible. Empty means "show every roster entry" —
## see ProviderRoster.filtered_and_ordered.
var enabled: Array = []
## Sigil grid order, by kind. Kinds absent here sort after the ordered ones.
var order: Array = []
## kind -> {"tier": String, "names": Array[String]}. A provider absent here
## uses the server's own defaults (empty tier, no name).
var provider_defaults: Dictionary = {}
## Absolute path of the project folder agents spawn in. Empty means the
## server's default (a per-agent scratch directory under the OS temp dir).
var workdir: String = ""

## The file this config was loaded from, and the one a no-argument
## save_to_file() writes back to. Remembered so a config loaded from a
## non-default path (a test fixture, most importantly) can never save
## itself over the keeper's real settings — the original bug here was a
## headless test whose in-memory config wrote straight to
## user://orki_settings.cfg and clobbered a real project folder.
var config_path: String = DEFAULT_PATH

## How many agent scroll/terminal panels may sit open at once. Opening one
## past the cap closes the least-recently-focused panel. Clamped to
## MIN_SCROLL_PANELS..MAX_SCROLL_PANELS on both read and write, so a
## hand-edited config file can never wedge the UI at zero panels.
var max_scroll_panels: int = DEFAULT_SCROLL_PANELS

const MIN_SCROLL_PANELS: int = 1
const MAX_SCROLL_PANELS: int = 3
const DEFAULT_SCROLL_PANELS: int = 1

## Agent chat panel's default open size, as a fraction of the viewport.
## These set where a panel STARTS; PanelBase's own drag-resize still applies
## on top, so this is the default the keeper stops having to re-drag every
## session rather than a hard limit. Defaults reproduce the original
## right-40%-full-height placement exactly.
var scroll_width_ratio: float = DEFAULT_SCROLL_WIDTH_RATIO
var scroll_height_ratio: float = DEFAULT_SCROLL_HEIGHT_RATIO

const MIN_SCROLL_RATIO: float = 0.2
const MAX_SCROLL_RATIO: float = 1.0
const DEFAULT_SCROLL_WIDTH_RATIO: float = 0.4
const DEFAULT_SCROLL_HEIGHT_RATIO: float = 1.0


## Clamps `value` into the supported open-panel range.
static func clamp_scroll_panels(value: int) -> int:
	return clampi(value, MIN_SCROLL_PANELS, MAX_SCROLL_PANELS)


## Clamps a panel size fraction. A too-small panel would open unusably tiny
## and a >1.0 one would start off-screen, so both ends are bounded rather
## than trusting a hand-edited config file.
static func clamp_scroll_ratio(value: float) -> float:
	return clampf(value, MIN_SCROLL_RATIO, MAX_SCROLL_RATIO)


## Loads from `path` (defaults to user://orki_settings.cfg). A missing or
## unreadable file yields an empty (all-enabled, no overrides) config rather
## than an error, so a first run never blocks the flyout.
static func load_from_file(path: String = DEFAULT_PATH) -> SigilConfig:
	var cfg := SigilConfig.new()
	# Remembered even when the load fails, so a first-run config saves back
	# to the path it was asked for rather than silently to the default.
	cfg.config_path = path
	var file := ConfigFile.new()
	if file.load(path) != OK:
		return cfg
	cfg.enabled = file.get_value(SECTION, "enabled", [])
	cfg.order = file.get_value(SECTION, "order", [])
	cfg.provider_defaults = file.get_value(SECTION, "provider_defaults", {})
	cfg.workdir = String(file.get_value(SECTION, "workdir", ""))
	cfg.max_scroll_panels = clamp_scroll_panels(int(file.get_value(SECTION, "max_scroll_panels", DEFAULT_SCROLL_PANELS)))
	cfg.scroll_width_ratio = clamp_scroll_ratio(float(file.get_value(SECTION, "scroll_width_ratio", DEFAULT_SCROLL_WIDTH_RATIO)))
	cfg.scroll_height_ratio = clamp_scroll_ratio(float(file.get_value(SECTION, "scroll_height_ratio", DEFAULT_SCROLL_HEIGHT_RATIO)))
	return cfg


## Writes this config to `path`. Returns the ConfigFile save() error code
## (OK on success).
## An empty `path` means "write back where this config came from" (see
## config_path). Callers that want a specific file still pass one.
func save_to_file(path: String = "") -> int:
	var target: String = path if not path.is_empty() else config_path
	var file := ConfigFile.new()
	# Preserve any other section already on disk (e.g. a future [display]
	# block) instead of clobbering the whole file with just [sigils].
	file.load(target)
	file.set_value(SECTION, "enabled", enabled)
	file.set_value(SECTION, "order", order)
	file.set_value(SECTION, "provider_defaults", provider_defaults)
	file.set_value(SECTION, "workdir", workdir)
	file.set_value(SECTION, "max_scroll_panels", clamp_scroll_panels(max_scroll_panels))
	file.set_value(SECTION, "scroll_width_ratio", clamp_scroll_ratio(scroll_width_ratio))
	file.set_value(SECTION, "scroll_height_ratio", clamp_scroll_ratio(scroll_height_ratio))
	return file.save(target)


## True when `kind` is in the enabled set, or the set is empty (show-all).
func is_enabled(kind: String) -> bool:
	return enabled.is_empty() or enabled.has(kind)


func set_enabled(kind: String, is_on: bool) -> void:
	if is_on:
		if not enabled.has(kind):
			enabled.append(kind)
	else:
		# An empty `enabled` means show-all, so disabling one kind out of an
		# empty set needs every OTHER known kind added explicitly first.
		# Callers therefore pass the full known-kind list via
		# expand_enabled_for_disable before calling this with is_on=false.
		enabled.erase(kind)


## Materializes an implicit show-all `enabled` into an explicit list before a
## single kind is disabled, so disabling one provider does not silently hide
## every other one too.
func expand_enabled_for_disable(all_kinds: Array) -> void:
	if enabled.is_empty():
		enabled = all_kinds.duplicate()


func tier_default(kind: String) -> String:
	return String(provider_defaults.get(kind, {}).get("tier", ""))


func name_pool(kind: String) -> Array:
	return provider_defaults.get(kind, {}).get("names", [])


func set_tier_default(kind: String, tier: String) -> void:
	var d: Dictionary = provider_defaults.get(kind, {})
	d["tier"] = tier
	provider_defaults[kind] = d


func set_name_pool(kind: String, names: Array) -> void:
	var d: Dictionary = provider_defaults.get(kind, {})
	d["names"] = names
	provider_defaults[kind] = d
