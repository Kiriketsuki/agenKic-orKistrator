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


## Loads from `path` (defaults to user://orki_settings.cfg). A missing or
## unreadable file yields an empty (all-enabled, no overrides) config rather
## than an error, so a first run never blocks the flyout.
static func load_from_file(path: String = DEFAULT_PATH) -> SigilConfig:
	var cfg := SigilConfig.new()
	var file := ConfigFile.new()
	if file.load(path) != OK:
		return cfg
	cfg.enabled = file.get_value(SECTION, "enabled", [])
	cfg.order = file.get_value(SECTION, "order", [])
	cfg.provider_defaults = file.get_value(SECTION, "provider_defaults", {})
	return cfg


## Writes this config to `path`. Returns the ConfigFile save() error code
## (OK on success).
func save_to_file(path: String = DEFAULT_PATH) -> int:
	var file := ConfigFile.new()
	# Preserve any other section already on disk (e.g. a future [display]
	# block) instead of clobbering the whole file with just [sigils].
	file.load(path)
	file.set_value(SECTION, "enabled", enabled)
	file.set_value(SECTION, "order", order)
	file.set_value(SECTION, "provider_defaults", provider_defaults)
	return file.save(path)


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
