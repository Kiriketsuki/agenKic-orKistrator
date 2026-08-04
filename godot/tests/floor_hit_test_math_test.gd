# floor_hit_test_math_test.gd — Regression guard for F3 Grimoire Summoning's
# drag-out placement mode hit-test mapping (FloorHitTest.classify).
#
# Standalone headless script, mirroring the project's existing test pattern
# (see palette_math_test.gd's doc-comment):
#
#   godot --headless --path godot --script tests/floor_hit_test_math_test.gd
#
# Exits 1 on any failure.

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_reserved_floor_case(failures)
	_run_real_floor_case(failures)
	_run_gap_between_floors_case(failures)
	_run_new_floor_zone_case(failures)
	_run_beyond_new_floor_zone_case(failures)
	_run_empty_tower_case(failures)
	if failures.is_empty():
		print("floor_hit_test_math_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("floor_hit_test_math_test: FAIL — " + message)
		quit(1)


func _assert(condition: bool, message: String, failures: Array[String]) -> void:
	if not condition:
		failures.append(message)


## Acceptance: floor 0 is always RESERVED, never a drop target.
func _run_reserved_floor_case(failures: Array[String]) -> void:
	var result: Dictionary = FloorHitTest.classify(0.0, 3, 56.0, 40.0)
	_assert(result["type"] == FloorHitTest.ZoneType.RESERVED, "world_y=0 on a 3-floor tower should classify RESERVED, got %d" % result["type"], failures)
	_assert(result["index"] == 0, "RESERVED result should report index 0, got %d" % result["index"], failures)


## Acceptance: a real floor (index > 0) inside its band classifies FLOOR.
func _run_real_floor_case(failures: Array[String]) -> void:
	# Floor 1 center is at y = -56. Its band spans [-76, -36].
	var result: Dictionary = FloorHitTest.classify(-56.0, 3, 56.0, 40.0)
	_assert(result["type"] == FloorHitTest.ZoneType.FLOOR, "world_y=-56 should classify FLOOR, got %d" % result["type"], failures)
	_assert(result["index"] == 1, "world_y=-56 should hit floor index 1, got %d" % result["index"], failures)

	# Floor 2 center is at y = -112.
	var result2: Dictionary = FloorHitTest.classify(-112.0, 3, 56.0, 40.0)
	_assert(result2["type"] == FloorHitTest.ZoneType.FLOOR, "world_y=-112 should classify FLOOR, got %d" % result2["type"], failures)
	_assert(result2["index"] == 2, "world_y=-112 should hit floor index 2, got %d" % result2["index"], failures)


## The gap between two floor slabs (56px spacing, 40px height leaves a 16px
## gap) belongs to neither floor, so it must classify NONE, not spill into
## a neighbour.
func _run_gap_between_floors_case(failures: Array[String]) -> void:
	# Between floor 0 (band [-20, 20]) and floor 1 (band [-76, -36]) the gap
	# is (-36, -20).
	var result: Dictionary = FloorHitTest.classify(-28.0, 3, 56.0, 40.0)
	_assert(result["type"] == FloorHitTest.ZoneType.NONE, "world_y=-28 (inter-floor gap) should classify NONE, got %d" % result["type"], failures)


## Acceptance: the gap above the top floor offers "new floor".
func _run_new_floor_zone_case(failures: Array[String]) -> void:
	# 3 floors: top floor index 2, center y=-112, top edge at -132.
	# The new-floor zone extends NEW_FLOOR_ZONE_HEIGHT (40) above that, to -172.
	var result: Dictionary = FloorHitTest.classify(-150.0, 3, 56.0, 40.0)
	_assert(result["type"] == FloorHitTest.ZoneType.NEW_FLOOR, "world_y=-150 (above top floor) should classify NEW_FLOOR, got %d" % result["type"], failures)
	_assert(result["index"] == 3, "NEW_FLOOR should propose index 3 (one above the current top), got %d" % result["index"], failures)


## Above the new-floor zone, there is nothing to drop on.
func _run_beyond_new_floor_zone_case(failures: Array[String]) -> void:
	var result: Dictionary = FloorHitTest.classify(-500.0, 3, 56.0, 40.0)
	_assert(result["type"] == FloorHitTest.ZoneType.NONE, "world_y=-500 (far above the tower) should classify NONE, got %d" % result["type"], failures)


## A tower with zero floors still reserves the ground-floor slot.
func _run_empty_tower_case(failures: Array[String]) -> void:
	var result: Dictionary = FloorHitTest.classify(0.0, 0, 56.0, 40.0)
	_assert(result["type"] == FloorHitTest.ZoneType.NONE, "an empty tower should classify NONE at y=0 (no floor_count to test against), got %d" % result["type"], failures)
