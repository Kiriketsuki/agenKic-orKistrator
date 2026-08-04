# floor_morph_test.gd — Regression guard for T15 (#124) polygon-morphing math.
#
# No GUT (or other Godot test runner) is vendored in this project yet, so this
# is a standalone script runnable headless:
#
#   godot --headless --path godot --script tests/floor_morph_test.gd
#
# Asserts the invariants that keep the load-driven morph coherent without a
# running engine to visually confirm: the bucket table and its hysteresis
# behave as specified, breathe and edge-width scaling stay monotonic, and the
# Phase 6 prism projection culls, orders and dims its faces correctly. Exits 1
# on any failure so it can be wired into CI later.
#
# Phase 6 section 1 deleted the lens resample, so the resample_ngon() boundary
# case and the lerp_unit_arrays() endpoint case went with it. FloorPrism face
# cases replace them.

extends SceneTree


func _init() -> void:
	var failures: Array[String] = []
	_run_bucket_cases(failures)
	_run_hysteresis_cases(failures)
	_run_hysteresis_multi_bucket_cases(failures)
	_run_nearest_valid_side_count_cases(failures)
	_run_prism_face_cases(failures)
	_run_prism_apothem_case(failures)
	_run_breathe_cases(failures)
	_run_edge_width_monotonic_case(failures)
	if failures.is_empty():
		print("floor_morph_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("floor_morph_test: FAIL — " + message)
		quit(1)


func _run_bucket_cases(failures: Array[String]) -> void:
	var cases: Array = [
		[0.0, 6], [0.19, 6],
		[0.2, 7], [0.39, 7],
		[0.4, 8], [0.59, 8],
		[0.6, 10], [0.79, 10],
		[0.8, 12], [1.0, 12],
	]
	for case: Array in cases:
		var load: float = case[0]
		var expected: int = case[1]
		var actual: int = FloorMorph.side_count_for_load(load)
		if actual != expected:
			failures.append("side_count_for_load(%s): expected %d got %d" % [load, expected, actual])


func _run_hysteresis_cases(failures: Array[String]) -> void:
	# Sitting right at a boundary with no margin should NOT flip.
	var held: int = FloorMorph.side_count_for_load_hysteresis(0.2, 6, 0.03)
	if held != 6:
		failures.append("hysteresis: expected no flip at exact boundary, got %d" % held)
	# Clearing the boundary by more than the deadband SHOULD flip.
	var flipped: int = FloorMorph.side_count_for_load_hysteresis(0.24, 6, 0.03)
	if flipped != 7:
		failures.append("hysteresis: expected flip past deadband, got %d" % flipped)
	# Moving back down past the lower deadband from the new bucket should
	# flip back.
	var flipped_back: int = FloorMorph.side_count_for_load_hysteresis(0.16, 7, 0.03)
	if flipped_back != 6:
		failures.append("hysteresis: expected flip back below lower deadband, got %d" % flipped_back)


## Council finding (#124) — a load jump spanning more than one bucket (e.g.
## the periodic decay sweep finally pruning a stale ring) must converge
## directly to the target bucket in a single call, not crawl one bucket at a
## time.
func _run_hysteresis_multi_bucket_cases(failures: Array[String]) -> void:
	# 12 -> 6 is a 4-bucket drop (0.05 load, current bucket is the top one).
	var dropped: int = FloorMorph.side_count_for_load_hysteresis(0.05, 12, 0.03)
	if dropped != 6:
		failures.append("hysteresis multi-bucket: expected direct drop to 6, got %d" % dropped)
	# 6 -> 12 is a 4-bucket climb (0.95 load, current bucket is the bottom one).
	var climbed: int = FloorMorph.side_count_for_load_hysteresis(0.95, 6, 0.03)
	if climbed != 12:
		failures.append("hysteresis multi-bucket: expected direct climb to 12, got %d" % climbed)
	# Adjacent-bucket case must still respect the deadband (regression guard
	# for the >1 branch not swallowing the adjacent case).
	var held_adjacent: int = FloorMorph.side_count_for_load_hysteresis(0.21, 6, 0.03)
	if held_adjacent != 6:
		failures.append("hysteresis multi-bucket: expected adjacent case to still respect deadband, got %d" % held_adjacent)


## Council finding (#124) — nearest_valid_side_count() must always return a
## real _SIDES member ([6, 7, 8, 10, 12]), even for inputs that a misconfigured
## min_sides/max_sides clamp could produce.
func _run_nearest_valid_side_count_cases(failures: Array[String]) -> void:
	var cases: Array = [
		[6, 6], [7, 7], [8, 8], [10, 10], [12, 12],
		[9, 8],    # equidistant from 8 and 10 — ties resolve to the lower one
		[11, 10],  # equidistant from 10 and 12 — ties resolve to the lower one
		[0, 6], [-5, 6],
		[20, 12], [100, 12],
	]
	for case: Array in cases:
		var input_sides: int = case[0]
		var expected: int = case[1]
		var actual: int = FloorMorph.nearest_valid_side_count(input_sides)
		if actual != expected:
			failures.append("nearest_valid_side_count(%d): expected %d got %d" % [input_sides, expected, actual])


## Phase 6 section 1 — the lens resample is gone. FloorPrism draws N flat
## faces instead, so the geometry contract this suite guards is the prism
## projection: exactly the faces that look at the camera survive culling, the
## paint order runs back to front, and the front face projects at scale 1.
func _run_prism_face_cases(failures: Array[String]) -> void:
	for sides: int in [6, 7, 8, 10, 12]:
		var face_w: float = EdgeLayout.edge_width_for_polygon(sides, 280.0)
		# Face 0 at rot 0 looks straight at the camera.
		var front: Vector2 = FloorPrism.face_center_projection(sides, face_w, 0.0, 0)
		if not is_equal_approx(front.x, 0.0):
			failures.append("prism(%d): front face centre x expected 0 got %f" % [sides, front.x])
		if not is_equal_approx(front.y, 1.0):
			failures.append("prism(%d): front face scale expected 1 got %f" % [sides, front.y])
		if not is_equal_approx(FloorPrism.face_dim(sides, 0.0, 0), 1.0):
			failures.append("prism(%d): front face must keep full value" % sides)
		# The face behind the camera never draws.
		var back: int = sides / 2
		if FloorPrism.face_is_front(sides, 0.0, back):
			failures.append("prism(%d): face %d faces away and must cull" % [sides, back])
		# Every drawn face sits nearer than the one before it in paint order.
		var order: Array[int] = FloorPrism.draw_order(sides, 0.0)
		if order.size() != sides:
			failures.append("prism(%d): draw_order returned %d entries" % [sides, order.size()])
			continue
		if order[order.size() - 1] != 0:
			failures.append("prism(%d): front face must paint last, got %d" % [sides, order[order.size() - 1]])
		var prev: float = -2.0
		for k: int in order:
			var c: float = cos(FloorPrism.face_angle(sides, 0.0, k))
			if c < prev - 0.0001:
				failures.append("prism(%d): draw order not back to front at face %d" % [sides, k])
				break
			prev = c
		# A neighbour face is dimmer than the front face and still visible.
		if not FloorPrism.face_is_front(sides, 0.0, 1):
			failures.append("prism(%d): neighbour face must stay visible" % sides)
		var dim: float = FloorPrism.face_dim(sides, 0.0, 1)
		if dim >= 1.0 or dim < FloorPrism.SIDE_DIM:
			failures.append("prism(%d): neighbour dim %f out of range" % [sides, dim])


## The apothem must match the demo geometry: a 6-face prism of 140 px faces
## sits 121 px from the axis.
func _run_prism_apothem_case(failures: Array[String]) -> void:
	var apothem: float = FloorPrism.apothem_for(6, 140.0)
	if absf(apothem - 121.0) > 1.0:
		failures.append("apothem_for(6, 140): expected about 121 got %f" % apothem)


func _run_breathe_cases(failures: Array[String]) -> void:
	var at_zero: float = FloorMorph.breathe_scale_for_load(0.0, 1.0, 1.25)
	var at_one: float = FloorMorph.breathe_scale_for_load(1.0, 1.0, 1.25)
	var at_half: float = FloorMorph.breathe_scale_for_load(0.5, 1.0, 1.25)
	if not is_equal_approx(at_zero, 1.0):
		failures.append("breathe_scale_for_load(0.0): expected 1.0 got %f" % at_zero)
	if not is_equal_approx(at_one, 1.25):
		failures.append("breathe_scale_for_load(1.0): expected 1.25 got %f" % at_one)
	if at_half <= at_zero or at_half >= at_one:
		failures.append("breathe_scale_for_load(0.5): expected strictly between endpoints, got %f" % at_half)


func _run_edge_width_monotonic_case(failures: Array[String]) -> void:
	# EdgeLayout.edge_width_for_polygon must grow monotonically as the
	# breathed floor width grows, for a fixed side count — this is what
	# gives more-loaded floors more horizontal desk room (criterion #124.3).
	var sides: int = 8
	var prev: float = -1.0
	for scale_pct: int in range(100, 126, 5):
		var width: float = 280.0 * (float(scale_pct) / 100.0)
		var edge_width: float = EdgeLayout.edge_width_for_polygon(sides, width)
		if edge_width <= prev:
			failures.append("edge_width_for_polygon not monotonic at scale %d%%: %f <= %f" % [scale_pct, edge_width, prev])
			break
		prev = edge_width
