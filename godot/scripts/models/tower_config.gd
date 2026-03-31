class_name TowerConfig
## Configuration for the tower layout, loaded from tower.json.

var polygon_sides: int = 6
var linger_duration_sec: float = 30.0
var permanent_floors: Array[Dictionary] = []


static func from_file(path: String) -> TowerConfig:
	var config := TowerConfig.new()
	var file := FileAccess.open(path, FileAccess.READ)
	if file == null:
		push_warning("TowerConfig: could not open %s, using defaults" % path)
		return config
	var parsed: Variant = JSON.parse_string(file.get_as_text())
	if not parsed is Dictionary:
		push_warning("TowerConfig: invalid JSON in %s, using defaults" % path)
		return config
	var d: Dictionary = parsed as Dictionary
	config.polygon_sides = d.get("polygon_sides", 6)
	config.linger_duration_sec = d.get("linger_duration_sec", 30.0)
	var floors_raw: Variant = d.get("permanent_floors", [])
	if floors_raw is Array:
		for item: Variant in (floors_raw as Array):
			if item is Dictionary:
				config.permanent_floors.append(item as Dictionary)
	return config
