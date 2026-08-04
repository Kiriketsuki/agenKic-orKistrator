# panel_manager_math_test.gd — Regression guard for T19 (#169) multi-scroll-
# panel cascade math (PanelManagerMath.tile_scroll_rect).
#
# No GUT (or other Godot test runner) is vendored in this project yet, so
# this is a standalone script runnable headless, mirroring
# panel_float_math_test.gd / particle_math_test.gd / palette_math_test.gd:
#
#   godot --headless --path godot --script tests/panel_manager_math_test.gd

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_index_zero_matches_original_placement_case(failures)
	_run_cascade_shifts_left_and_down_case(failures)
	_run_negative_index_clamps_to_zero_case(failures)
	_run_rect_stays_within_viewport_case(failures)
	if failures.is_empty():
		print("panel_manager_math_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("panel_manager_math_test: FAIL — " + message)
		quit(1)


## index=0 must reproduce the original single-scroll placement exactly: full
## viewport height, right-anchored at (viewport_width - panel_width).
func _run_index_zero_matches_original_placement_case(failures: Array[String]) -> void:
	var viewport_size := Vector2(1920.0, 1080.0)
	var panel_width := 768.0
	var cascade_offset := Vector2(36.0, 28.0)
	var rect: Rect2 = PanelManagerMath.tile_scroll_rect(viewport_size, panel_width, 0, cascade_offset)
	if not is_equal_approx(rect.position.x, viewport_size.x - panel_width):
		failures.append("tile_scroll_rect(index=0) x expected %f, got %f" % [viewport_size.x - panel_width, rect.position.x])
	if not is_equal_approx(rect.position.y, 0.0):
		failures.append("tile_scroll_rect(index=0) y expected 0.0, got %f" % rect.position.y)
	if not is_equal_approx(rect.size.x, panel_width):
		failures.append("tile_scroll_rect(index=0) width expected %f, got %f" % [panel_width, rect.size.x])
	if not is_equal_approx(rect.size.y, viewport_size.y):
		failures.append("tile_scroll_rect(index=0) height expected %f, got %f" % [viewport_size.y, rect.size.y])


## Each subsequent index must shift strictly left and down from the previous
## one by exactly cascade_offset, and shrink height by cascade_offset.y —
## the visible "cascade" the feature is named for.
func _run_cascade_shifts_left_and_down_case(failures: Array[String]) -> void:
	var viewport_size := Vector2(1920.0, 1080.0)
	var panel_width := 768.0
	var cascade_offset := Vector2(36.0, 28.0)
	var previous: Rect2 = PanelManagerMath.tile_scroll_rect(viewport_size, panel_width, 0, cascade_offset)
	for index: int in range(1, 3):
		var rect: Rect2 = PanelManagerMath.tile_scroll_rect(viewport_size, panel_width, index, cascade_offset)
		if not is_equal_approx(rect.position.x, previous.position.x - cascade_offset.x):
			failures.append("tile_scroll_rect(index=%d) x did not shift left by cascade_offset.x" % index)
		if not is_equal_approx(rect.position.y, previous.position.y + cascade_offset.y):
			failures.append("tile_scroll_rect(index=%d) y did not shift down by cascade_offset.y" % index)
		if not is_equal_approx(rect.size.y, previous.size.y - cascade_offset.y):
			failures.append("tile_scroll_rect(index=%d) height did not shrink by cascade_offset.y" % index)
		if not is_equal_approx(rect.size.x, panel_width):
			failures.append("tile_scroll_rect(index=%d) width changed from panel_width, got %f" % [index, rect.size.x])
		previous = rect


## A negative index (a stale/not-found lookup upstream) must degrade to the
## same rect as index 0, never producing a rect pushed off-viewport.
func _run_negative_index_clamps_to_zero_case(failures: Array[String]) -> void:
	var viewport_size := Vector2(1920.0, 1080.0)
	var panel_width := 768.0
	var cascade_offset := Vector2(36.0, 28.0)
	var zero_rect: Rect2 = PanelManagerMath.tile_scroll_rect(viewport_size, panel_width, 0, cascade_offset)
	var negative_rect: Rect2 = PanelManagerMath.tile_scroll_rect(viewport_size, panel_width, -1, cascade_offset)
	if zero_rect != negative_rect:
		failures.append("tile_scroll_rect(index=-1) expected to clamp to index=0's rect")


## Even at the maximum supported cap (3 panels, index up to 2), the rect must
## never report a negative origin or negative size — cascade math must clamp
## rather than run off the top/left of the viewport.
func _run_rect_stays_within_viewport_case(failures: Array[String]) -> void:
	var viewport_size := Vector2(1920.0, 1080.0)
	var panel_width := 768.0
	var cascade_offset := Vector2(36.0, 28.0)
	for index: int in range(0, 3):
		var rect: Rect2 = PanelManagerMath.tile_scroll_rect(viewport_size, panel_width, index, cascade_offset)
		if rect.position.x < 0.0 or rect.position.y < 0.0:
			failures.append("tile_scroll_rect(index=%d) produced a negative origin: %s" % [index, rect.position])
		if rect.size.x < 0.0 or rect.size.y < 0.0:
			failures.append("tile_scroll_rect(index=%d) produced a negative size: %s" % [index, rect.size])
