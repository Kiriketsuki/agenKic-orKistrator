class_name LayoutPresets
## LayoutPresets — persisted keeper panel-arrangement presets (F5 Panels Orb
## and Restyle). Backed by a ConfigFile at user://orki_settings.cfg, section
## [layouts]. Stores a name-to-arrangement map, where an arrangement is the
## Dictionary shape PanelManager.capture_arrangement() returns.
##
## A RefCounted, not a Node, so it has a headless round-trip test
## (tests/layout_presets_test.gd) with no scene tree involved. Mirrors
## SigilConfig's load_from_file/save_to_file shape (F3 Grimoire Summoning).

const SECTION: String = "layouts"
const DEFAULT_PATH: String = "user://orki_settings.cfg"

## preset name -> arrangement Dictionary (see PanelManager.capture_arrangement).
var presets: Dictionary = {}


## Loads from `path` (defaults to user://orki_settings.cfg). A missing or
## unreadable file yields an empty preset map rather than an error, so a
## first run never blocks the flyout.
static func load_from_file(path: String = DEFAULT_PATH) -> LayoutPresets:
	var config := LayoutPresets.new()
	var file := ConfigFile.new()
	if file.load(path) != OK:
		return config
	config.presets = file.get_value(SECTION, "presets", {})
	return config


## Writes this preset map to `path`. Returns the ConfigFile save() error code
## (OK on success). Preserves any other section already on disk (e.g.
## [sigils]) instead of clobbering the whole file with just [layouts].
func save_to_file(path: String = DEFAULT_PATH) -> int:
	var file := ConfigFile.new()
	file.load(path)
	file.set_value(SECTION, "presets", presets)
	return file.save(path)


func has_preset(preset_name: String) -> bool:
	return presets.has(preset_name)


func get_preset(preset_name: String) -> Dictionary:
	return presets.get(preset_name, {})


func set_preset(preset_name: String, arrangement: Dictionary) -> void:
	presets[preset_name] = arrangement


func remove_preset(preset_name: String) -> void:
	presets.erase(preset_name)


## Names of every saved custom preset, sorted for a stable flyout listing.
func preset_names() -> Array:
	var names: Array = presets.keys()
	names.sort()
	return names
