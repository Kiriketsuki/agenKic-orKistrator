class_name FloorMorph
## T15 — Pure geometry/bucket helpers for floor polygon morphing (#124).
##
## HONEST METRIC NOTE (mirrors the T14 honest-minimal precedent): composite_load
## is computed client-side in TowerManager from data that actually flows over
## the bridge SSE stream. The spec's third component, token_cost_norm, has no
## data source anywhere in the orchestrator — token counts live only inside
## internal/gateway/litellm.go response structs, are never persisted to the
## store, and are never published as an SSE event; the Bridge struct also has
## no reference to the gateway. Emitting a Go-side token/floor_load metric
## would additionally require inventing a server-side floor<->agent binding
## (agents carry no floor field today — see BridgeData.AgentData.floor_name,
## which the orchestrator never populates). Both are out of scope "fantasy
## plumbing". composite_load is therefore:
##
##   composite_load = w_active * active_agents_norm + w_thru * task_throughput_norm
##   w_active = 0.4 / (0.4 + 0.3) ≈ 0.571,  w_thru = 0.3 / (0.4 + 0.3) ≈ 0.429
##
## with token_cost_norm dropped and the remaining two weights renormalized.
## See TowerManager._recompute_floor_load() for the aggregation itself; this
## file only contains the pure, node-free geometry/bucket math consumed by it
## and by FloorScene.

## Side-count buckets. Index i applies while load < _THRESHOLDS[i]; the final
## entry in _SIDES applies for anything at/above the last threshold.
const _THRESHOLDS: Array[float] = [0.2, 0.4, 0.6, 0.8]
const _SIDES: Array[int] = [6, 7, 8, 10, 12]


## Maps a composite_load in [0,1] to a side count via the fixed bucket table.
static func side_count_for_load(load: float) -> int:
	for i: int in range(_THRESHOLDS.size()):
		if load < _THRESHOLDS[i]:
			return _SIDES[i]
	return _SIDES[_SIDES.size() - 1]


static func _bucket_index_for_sides(sides: int) -> int:
	var idx: int = _SIDES.find(sides)
	return idx if idx != -1 else 0


## Same bucket mapping as side_count_for_load, but with a deadband around the
## bucket edge adjacent to `current_sides` so a load hovering on a boundary
## does not oscillate the side count every call. Moves at most one bucket per
## call — composite_load changes are event-driven (agent state transitions),
## so this converges within a couple of calls rather than needing to jump
## multiple buckets in a single frame.
static func side_count_for_load_hysteresis(load: float, current_sides: int, hysteresis: float) -> int:
	var current_idx: int = _bucket_index_for_sides(current_sides)
	var raw_idx: int = _bucket_index_for_sides(side_count_for_load(load))
	if raw_idx == current_idx:
		return current_sides
	if raw_idx > current_idx:
		var boundary: float = _THRESHOLDS[current_idx]
		if load >= boundary + hysteresis:
			return _SIDES[current_idx + 1]
		return current_sides
	else:
		var boundary: float = _THRESHOLDS[current_idx - 1]
		if load <= boundary - hysteresis:
			return _SIDES[current_idx - 1]
		return current_sides


## Raw n-gon vertices on a unit circle (circumradius 1), n points.
static func regular_ngon_unit(n: int, rotation: float = 0.0) -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array()
	if n < 3:
		return pts
	for i: int in range(n):
		var theta: float = rotation + TAU * float(i) / float(n)
		pts.append(Vector2(cos(theta), sin(theta)))
	return pts


## Resamples the boundary of a regular n-gon (unit circumradius 1) at k
## equally-spaced angles. Every returned point lies exactly on the n-gon's
## edges (not just its vertices), so a fixed k works for any n — this is
## what lets floor_scene.gd lerp between an old n and a new n without any
## popping: both arrays have identical length k, and each element is an
## exact point on its respective polygon boundary.
static func resample_ngon(n: int, k: int, rotation: float = 0.0) -> PackedVector2Array:
	var pts: PackedVector2Array = PackedVector2Array()
	if n < 3 or k < 3:
		return pts
	var sector: float = TAU / float(n)
	var apothem: float = cos(sector / 2.0)  # circumradius 1 -> apothem = cos(pi/n)
	for i: int in range(k):
		var theta: float = rotation + TAU * float(i) / float(k)
		var rel: float = fmod(theta - rotation, sector)
		if rel < 0.0:
			rel += sector
		var theta_local: float = rel - sector / 2.0
		var r: float = apothem / cos(theta_local)
		pts.append(Vector2(r * cos(theta), r * sin(theta)))
	return pts


## Continuous breathe scale — floors "breathe" outward as load rises, even
## when the bucketed side count does not change. Linear in load; callers
## clamp `load` to [0,1] before calling.
static func breathe_scale_for_load(load: float, min_scale: float, max_scale: float) -> float:
	return lerpf(min_scale, max_scale, clampf(load, 0.0, 1.0))


## Element-wise lerp between two equal-length unit-shape arrays (as produced
## by resample_ngon at the same k). Returns `b` unchanged if lengths differ
## (defensive — should never happen when both arrays came from resample_ngon
## with the same k).
static func lerp_unit_arrays(a: PackedVector2Array, b: PackedVector2Array, t: float) -> PackedVector2Array:
	if a.size() != b.size():
		return b
	var out: PackedVector2Array = PackedVector2Array()
	out.resize(a.size())
	for i: int in range(a.size()):
		out[i] = a[i].lerp(b[i], t)
	return out


## Scales a unit-shape array (as produced by resample_ngon /
## regular_ngon_unit) to the flattened footprint of a floor: x half-extent
## `half_width`, y half-extent `half_height`.
static func scale_unit_array(unit_pts: PackedVector2Array, half_width: float, half_height: float) -> PackedVector2Array:
	var out: PackedVector2Array = PackedVector2Array()
	out.resize(unit_pts.size())
	for i: int in range(unit_pts.size()):
		var p: Vector2 = unit_pts[i]
		out[i] = Vector2(p.x * half_width, p.y * half_height)
	return out
