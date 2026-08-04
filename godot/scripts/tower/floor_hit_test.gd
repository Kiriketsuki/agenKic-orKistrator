class_name FloorHitTest
## FloorHitTest — pure world-space math for Grimoire Summoning drag-out
## placement (F3). Maps a world-space cursor position to a tower floor
## index, the "new floor" gap above the top floor, or nothing.
##
## Kept free of any Node/scene dependency so the mapping math has a headless
## test (see tests/floor_hit_test_math_test.gd) independent of camera and
## viewport scaling — see the spec's "Drop hit-testing fights the camera and
## viewport scaling" risk entry.

enum ZoneType { NONE, RESERVED, FLOOR, NEW_FLOOR }

## How far above the top floor's top edge the "new floor" gap extends, in
## world (art) pixels. Wide enough to comfortably drop on, narrow enough that
## it does not swallow the sky above it.
const NEW_FLOOR_ZONE_HEIGHT: float = 40.0


## World-space Y of floor `index`'s center. Mirrors
## TowerManager._layout_floors: floor i sits at Vector2(0, i * -spacing).
static func floor_center_y(index: int, floor_spacing: float) -> float:
	return -float(index) * floor_spacing


## Classifies a world-space Y position against the tower's floor stack.
## `floor_count` is TowerManager.get_floor_count(). Index 0 is always
## RESERVED (the archmage's ground floor, per the spec), regardless of
## whether a real floor node occupies it. Returns a Dictionary:
## {type: ZoneType, index: int} — index is the floor index for FLOOR/RESERVED,
## the index the new floor would take for NEW_FLOOR, and -1 for NONE.
static func classify(world_y: float, floor_count: int, floor_spacing: float, floor_height: float) -> Dictionary:
	for i: int in range(floor_count):
		var cy: float = floor_center_y(i, floor_spacing)
		if world_y >= cy - floor_height / 2.0 and world_y <= cy + floor_height / 2.0:
			if i == 0:
				return {"type": ZoneType.RESERVED, "index": 0}
			return {"type": ZoneType.FLOOR, "index": i}
	if floor_count > 0:
		var top_edge: float = floor_center_y(floor_count - 1, floor_spacing) - floor_height / 2.0
		if world_y < top_edge and world_y >= top_edge - NEW_FLOOR_ZONE_HEIGHT:
			return {"type": ZoneType.NEW_FLOOR, "index": floor_count}
	return {"type": ZoneType.NONE, "index": -1}
