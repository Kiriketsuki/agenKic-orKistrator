extends Node2D
## FloorScene — a single floor in the tower. Manages edge rotation,
## AgentCharacter spawning, the ephemeral lifecycle state machine, and
## (T15/#124) composite-load-driven polygon morphing.

signal agent_clicked(agent_id: String)
signal agent_right_clicked(agent_id: String)
signal agent_hovered(agent_id: String)
signal agent_unhovered(agent_id: String)

const AGENT_CHARACTER_SCENE: PackedScene = preload("res://scenes/agent_character.tscn")
const WALL_TEXTURE: Texture2D = preload("res://assets/tiles/sliced/stone_wall.png")
## Floor-plane tiles, keyed by the floor_tile export. TowerManager picks one
## per floor so each room reads as its own place.
const FLOOR_TILES: Dictionary = {
	"stone_floor": preload("res://assets/tiles/sliced/stone_floor.png"),
	"wood_floor": preload("res://assets/tiles/sliced/wood_floor.png"),
	"rune_floor": preload("res://assets/tiles/sliced/rune_floor.png"),
}
const WINDOW_TEXTURE: Texture2D = preload("res://assets/sprites/deco/window.png")
const TORCH_TEXTURE: Texture2D = preload("res://assets/sprites/deco/torch.png")
const PROPS_TEXTURE: Texture2D = preload("res://assets/props/workstations.png")
const NAMEPLATE_TEXTURE: Texture2D = preload("res://assets/ui/nameplate_frame.png")

## Phase 4 three-band anatomy, in art px, inside a 40 px slab.
## Band A is the back wall, band B the walkable floor plane, band C the front
## lip. The plane flares toward the viewer, so its back edge sits inset by
## PLANE_FLARE and its front edge reaches the slab width.
const WALL_BAND_HEIGHT: float = 16.0
const PLANE_DEPTH: float = 20.0
const PLANE_FLARE: float = 8.0
const FRONT_LIP_HEIGHT: float = 3.0
const FRONT_LIP_COLOR: Color = Color(0.059, 0.067, 0.090, 1.0)
## Alpha of the per-floor multiply wash over bands A and B.
const WASH_ALPHA: float = 0.15
const PROP_COUNT: int = 8
## Wall dressing spacing, in art px.
const WINDOW_SPACING: float = 48.0
const WINDOW_START: float = 24.0
const TORCH_INSET: float = 12.0

## Active states — matches BridgeData.AgentData's doc-comment vocabulary.
## Idle and crashed agents read as dim on the minimap/badges.
const ACTIVE_STATES: Array[String] = ["assigned", "working", "reporting"]

enum FloorState { ACTIVE, LINGERING, DISSOLVING }

## Number of angular samples used to represent the floor's polygon boundary.
## Fixed across all side counts so an old-N and a new-N shape can be lerped
## element-wise with zero popping (see FloorMorph.resample_ngon).
const RESAMPLE_K: int = 96
## Vertex-lerp morph duration (acceptance criterion #124.2: "~0.5s").
const MORPH_SEC: float = 0.5
## Fixed rotation applied to every polygon sample — kept identical across
## old/new shapes so the morph never introduces a spurious rotation.
const ROTATION: float = 0.0

@export var floor_name: String = ""
@export var floor_label: String = ""
@export var is_permanent: bool = false
@export var polygon_sides: int = 6
## Floor-plane tile name, a key of FLOOR_TILES. TowerManager sets this per
## floor. An unknown name falls back to stone_floor.
@export var floor_tile: String = "stone_floor"
## Per-floor identity wash, multiplied over bands A and B at WASH_ALPHA.
@export var wash_tint: Color = Color(0.227, 0.290, 0.227, 1.0)

var _state: FloorState = FloorState.ACTIVE
var _active_edge: int = 0
var _floor_width: float = 280.0
var _floor_height: float = 40.0
## Each entry: {agent_id, edge_index, character_class, state}
var _agent_slots: Array[Dictionary] = []
var _linger_timer: float = 0.0
var _linger_duration: float = 30.0
var _dressing: Node2D = null
## Phase 4 — the fisheye layout hides the dressing on distant floors, where the
## windows and torches shrink to sub-pixel noise. A rebuild re-applies this.
var _show_dressing: bool = true
## Band polygons, kept as fields so a morph frame can restretch them without
## rebuilding the whole dressing layer. _wall_band also carries the texture
## offset that Phase 4 rotation scrolls.
var _wall_band: Polygon2D = null
var _floor_plane: Polygon2D = null
var _front_lip: Polygon2D = null
var _wash: Polygon2D = null
var _cornice: Polygon2D = null
var _torches: Array[Sprite2D] = []
var _flicker_time: float = 0.0
## Wall scroll position in texture px. This lives on the scene, not on the band,
## because _rebuild_dressing() frees and recreates the band. A separate tween
## drives it, so a rebuild during a turn never aborts the carousel tween.
var _wall_scroll_x: float = 0.0
var _wall_scroll_tween: Tween = null

# --- T15 load-driven morph state ---
var _composite_load: float = 0.0
var _target_sides: int = 6
var _current_sides: int = 6
var _effective_width: float = 280.0
var _morph_tween: Tween = null
var _morph_old_unit: PackedVector2Array = PackedVector2Array()
var _morph_new_unit: PackedVector2Array = PackedVector2Array()
var _morph_old_width: float = 280.0
var _morph_new_width: float = 280.0
var _morph_old_glow: float = 0.0
var _morph_new_glow: float = 0.0
## Cached mid-flight state so an in-progress morph can be interrupted by a
## new one (e.g. rapid load changes) without popping back to a stale shape.
var _morph_last_unit: PackedVector2Array = PackedVector2Array()
var _morph_last_width: float = 280.0
var _morph_last_glow: float = 0.0
## Deferred set_floor_dimensions() input, applied once the in-flight morph
## settles (see set_floor_dimensions / _on_morph_finished) — council finding
## #124: rebuilding mid-morph clobbered _morph_last_* with a stale
## _current_sides shape, causing a one-frame snap-back.
var _pending_dimensions: bool = false
var _pending_width: float = 280.0
var _pending_height: float = 40.0

## Config-driven tunables (see TowerConfig / config/tower.json), pushed by
## TowerManager at floor creation via configure_load_params(). Defaults here
## match tower.json's defaults so a floor works sanely if never configured
## (e.g. in isolated tests).
var _min_sides: int = 6
var _max_sides: int = 12
var _breathe_min_scale: float = 1.0
var _breathe_max_scale: float = 1.25
var _bucket_hysteresis: float = 0.03
## T17 (#127) — global per-agent particle budget cap (acceptance #5), pushed
## by TowerManager._create_floor() via configure_particle_budget(). Default
## matches TowerConfig.max_particles_per_agent's default so a floor works
## sanely if never configured (e.g. in isolated tests).
var _particle_budget: int = 24

@onready var _background: Polygon2D = $Background
@onready var _edge_glow: Line2D = $EdgeGlow
@onready var _active_edge_glow: Line2D = $ActiveEdgeGlow
@onready var _interior: Node2D = $Interior
@onready var _agent_slots_node: Node2D = $AgentSlots
@onready var _name_label: Label = $NameLabel


func _ready() -> void:
	# PARITY — no vector font lives in world space any more. The floor label and
	# the agent nameplates render on UILayer at window resolution (see
	# scripts/ui/world_labels.gd). The node stays for API compatibility, hidden.
	if _name_label != null:
		_name_label.text = floor_label if floor_label != "" else floor_name
		_name_label.visible = false
	_current_sides = polygon_sides
	_target_sides = polygon_sides
	_effective_width = _floor_width
	_rebuild_background()
	_rebuild_interior()
	_update_active_edge_glow()


func _process(delta: float) -> void:
	_flicker_time += delta
	for i: int in range(_torches.size()):
		var torch: Sprite2D = _torches[i]
		if is_instance_valid(torch):
			var flicker: float = 0.82 + 0.18 * sin(_flicker_time * 9.0 + i * 2.1)
			torch.modulate = Color(1.0, flicker, flicker * 0.85, 1.0)
	if _state == FloorState.LINGERING:
		_linger_timer -= delta
		if _linger_timer <= 0.0:
			_state = FloorState.DISSOLVING
			_kill_morph_tween()
			queue_free()


## Kills any in-flight morph tween before this node is freed — an un-killed
## tween whose callback later touches a freed node/material would crash.
func _notification(what: int) -> void:
	if what == NOTIFICATION_PREDELETE:
		_kill_morph_tween()


func _kill_morph_tween() -> void:
	if _morph_tween != null and _morph_tween.is_valid():
		_morph_tween.kill()
	_morph_tween = null


func get_floor_state() -> FloorState:
	return _state


func get_active_edge() -> int:
	return _active_edge


func set_active_edge(edge: int) -> void:
	# Clamp against whichever side count is authoritative right now: mid-morph,
	# polygon_sides/_active_edge already reflect the final (target) side count
	# (see set_composite_load), so this is always safe even while a morph
	# animation is still visually catching up.
	_active_edge = edge % polygon_sides
	_rebuild_interior()
	_update_active_edge_glow()


## Turns the room to a new edge. The slab, the silhouette and the dressing stay
## fixed. Only the contents layer moves, so the building never teleports.
## direction is +1 to rotate right and -1 to rotate left. Returns the Tween so
## the caller can kill it when a click interrupts the turn.
func rotate_to_edge(new_edge: int, direction: int) -> Tween:
	# A killed tween leaves the contents layer part way through a turn. Reset it
	# to home before the next turn starts, so rapid clicks never compound drift.
	_agent_slots_node.position.x = 0.0
	_agent_slots_node.scale.x = 1.0
	_agent_slots_node.modulate.a = 1.0
	var edge_w: float = EdgeLayout.edge_width_for_polygon(polygon_sides, _effective_width)
	var tw: Tween = create_tween().set_parallel(true) \
		.set_trans(Tween.TRANS_EXPO).set_ease(Tween.EASE_OUT)
	tw.tween_property(_agent_slots_node, "position:x", -direction * edge_w, 0.275)
	tw.tween_property(_agent_slots_node, "modulate:a", 0.0, 0.2)
	# Optional C. Scale only, no skew and no rotation, so the carousel reads as
	# depth instead of a bounce.
	tw.tween_property(_agent_slots_node, "scale:x", 0.9, 0.275)
	# The wall band streams one edge width of brick across the whole turn. That
	# sells the cylinder rotation even when both edges hold no agents. It runs on
	# its own tween for two reasons. A 0.55 s tweener inside the first parallel
	# step would delay chain() to t=0.55 and stall the carousel. A rebuild of the
	# dressing layer also frees the band mid-turn, and Godot aborts a tween whose
	# target it cannot find, which would strand the contents layer hidden.
	_start_wall_scroll(_wall_scroll_x + direction * edge_w)
	tw.chain().tween_callback(func() -> void:
		set_active_edge(new_edge)
		_agent_slots_node.position.x = direction * edge_w
	)
	tw.chain().tween_property(_agent_slots_node, "position:x", 0.0, 0.275)
	tw.parallel().tween_property(_agent_slots_node, "modulate:a", 1.0, 0.25)
	tw.parallel().tween_property(_agent_slots_node, "scale:x", 1.0, 0.275)
	return tw


## Scrolls the wall brick to a new offset over the full 0.55 s turn. The tween
## writes through _apply_wall_scroll, so a freed band never aborts it.
func _start_wall_scroll(target_x: float) -> void:
	if _wall_scroll_tween != null and _wall_scroll_tween.is_valid():
		_wall_scroll_tween.kill()
	_wall_scroll_tween = create_tween() \
		.set_trans(Tween.TRANS_EXPO).set_ease(Tween.EASE_OUT)
	_wall_scroll_tween.tween_method(_apply_wall_scroll, _wall_scroll_x, target_x, 0.55)


func _apply_wall_scroll(value: float) -> void:
	_wall_scroll_x = value
	if _wall_band != null and is_instance_valid(_wall_band):
		_wall_band.texture_offset.x = value


func begin_linger(duration: float) -> void:
	if is_permanent:
		return
	_state = FloorState.LINGERING
	_linger_duration = duration
	_linger_timer = duration
	modulate.a = 0.5


func reactivate() -> void:
	_state = FloorState.ACTIVE
	_linger_timer = 0.0
	modulate.a = 1.0


func set_floor_dimensions(width: float, height: float) -> void:
	if _morph_tween != null and _morph_tween.is_valid():
		# Council finding #124 — _rebuild_background()/_rebuild_interior()
		# rebuild from the stale _current_sides shape and stomp
		# _morph_last_*, which the running tween's next frame then reads from
		# for its interrupt-resume path, producing a one-frame snap-back to
		# the old shape (violates "no popping"). Defer instead: the in-flight
		# tween keeps animating with its already-captured old/new widths
		# undisturbed, and the new dimensions are applied via a normal
		# (non-mid-morph) set_floor_dimensions() call once it settles — see
		# _on_morph_finished().
		_pending_width = width
		_pending_height = height
		_pending_dimensions = true
		return
	_floor_width = width
	_floor_height = height
	_effective_width = _floor_width * FloorMorph.breathe_scale_for_load(_composite_load, _breathe_min_scale, _breathe_max_scale)
	if is_inside_tree():
		_rebuild_background()
		_rebuild_interior()
		_update_active_edge_glow()


## Pushes the T15 config tunables from TowerConfig into this floor. Called by
## TowerManager right after instantiation, before the floor enters the tree.
func configure_load_params(min_sides: int, max_sides: int, breathe_min_scale: float, breathe_max_scale: float, bucket_hysteresis: float) -> void:
	_min_sides = min_sides
	_max_sides = max_sides
	_breathe_min_scale = breathe_min_scale
	_breathe_max_scale = breathe_max_scale
	_bucket_hysteresis = bucket_hysteresis


## T17 (#127) — pushes the global per-agent particle budget cap (tower.json
## max_particles_per_agent) from TowerManager into this floor. Called once
## per floor at creation (mirroring configure_load_params()'s pre-tree
## contract), NOT per-agent — every agent slot on this floor shares the same
## cap (see particle_math.gd doc-comment).
func configure_particle_budget(max_per_agent: int) -> void:
	_particle_budget = max_per_agent


## T15 (#124) entry point — called by TowerManager whenever this floor's
## composite_load may have changed (agent register/deregister/state change).
## Side count is bucketed (with hysteresis); breathe scale and glow intensity
## are continuous and always animate, even when the bucket doesn't change.
func set_composite_load(load: float) -> void:
	if not is_inside_tree():
		# Not ready yet (e.g. called before _ready) — just record the value;
		# _ready() will pick it up via the field default path.
		_composite_load = clampf(load, 0.0, 1.0)
		return
	_composite_load = clampf(load, 0.0, 1.0)
	# Council finding #124 — clampi() alone can land on a value that isn't a
	# _SIDES member (e.g. a misconfigured max_sides of 9), silently breaking
	# _bucket_index_for_sides() lookups on the next call. Snap to the nearest
	# real bucket after clamping so this stays total.
	var new_target: int = FloorMorph.nearest_valid_side_count(clampi(
		FloorMorph.side_count_for_load_hysteresis(_composite_load, _target_sides, _bucket_hysteresis),
		_min_sides, _max_sides
	))
	var new_width: float = _floor_width * FloorMorph.breathe_scale_for_load(_composite_load, _breathe_min_scale, _breathe_max_scale)
	var sides_changing: bool = new_target != _target_sides
	var already_settled: bool = not sides_changing and is_equal_approx(new_width, _effective_width) \
		and (_morph_tween == null or not _morph_tween.is_valid())
	if already_settled:
		# Nothing to animate — avoid spinning up a no-op tween on every
		# agent-activity signal when load hasn't actually moved.
		if _edge_glow:
			var mat: ShaderMaterial = _edge_glow.material as ShaderMaterial
			if mat:
				mat.set_shader_parameter("glow", _composite_load)
		_update_active_edge_glow()
		return
	if sides_changing:
		_rehome_for_sides(new_target)
	_start_morph(new_width, sides_changing)


## Remaps agent slots and the active edge onto a smaller/larger side count
## BEFORE any rebuild happens — the highest-correctness-risk path in T15:
## an out-of-range edge_index after a shrink (12 -> 6) would otherwise vanish
## desks or index out of range in EdgeLayout.
func _rehome_for_sides(new_sides: int) -> void:
	for slot: Dictionary in _agent_slots:
		slot["edge_index"] = int(slot["edge_index"]) % new_sides
	_active_edge = _active_edge % new_sides
	_target_sides = new_sides
	polygon_sides = new_sides
	# Structural change (edge membership / active edge) — rebuild desks now,
	# at the OLD effective width; _apply_morph_t reflows their x-positions
	# smoothly as the width breathes toward new_width over MORPH_SEC.
	_rebuild_interior()


func _start_morph(new_width: float, sides_changing: bool) -> void:
	var was_running: bool = _morph_tween != null and _morph_tween.is_valid()
	# Old state: if a morph is already in flight, resume from its last
	# rendered frame (not _current_sides/_effective_width, which are stale
	# until a morph finishes) so an interrupted morph never pops.
	if was_running:
		_morph_old_unit = _morph_last_unit
		_morph_old_width = _morph_last_width
		_morph_old_glow = _morph_last_glow
	else:
		_morph_old_unit = FloorMorph.resample_ngon(_current_sides, RESAMPLE_K, ROTATION)
		_morph_old_width = _effective_width
		_morph_old_glow = _morph_last_glow
	_kill_morph_tween()

	_morph_new_unit = FloorMorph.resample_ngon(_target_sides, RESAMPLE_K, ROTATION)
	_morph_new_width = new_width
	_morph_new_glow = _composite_load

	if not sides_changing and _morph_old_unit.size() != _morph_new_unit.size():
		# Defensive: RESAMPLE_K is fixed so this should never trigger, but if
		# it ever did, fall back to snapping instead of lerping garbage.
		_morph_old_unit = _morph_new_unit

	_morph_tween = create_tween()
	_morph_tween.set_trans(Tween.TRANS_SINE).set_ease(Tween.EASE_OUT)
	_morph_tween.tween_method(_apply_morph_t, 0.0, 1.0, MORPH_SEC)
	_morph_tween.finished.connect(_on_morph_finished)


func _apply_morph_t(t: float) -> void:
	if not is_instance_valid(self) or not is_inside_tree():
		return
	var unit_pts: PackedVector2Array = FloorMorph.lerp_unit_arrays(_morph_old_unit, _morph_new_unit, t)
	var width: float = lerpf(_morph_old_width, _morph_new_width, t)
	var glow: float = lerpf(_morph_old_glow, _morph_new_glow, t)
	_morph_last_unit = unit_pts
	_morph_last_width = width
	_morph_last_glow = glow
	_effective_width = width
	# Silhouette is the fixed chamfered slab; only the width breathes.
	var scaled: PackedVector2Array = _slab_polygon(width)
	_background.polygon = scaled
	# Phase 4 — the slab is a silhouette only. Re-apply the three-band split at
	# the new width so a morph frame never shows the old full-slab wall.
	_apply_band_geometry(width)
	if _edge_glow:
		_edge_glow.points = scaled
		var mat: ShaderMaterial = _edge_glow.material as ShaderMaterial
		if mat:
			mat.set_shader_parameter("glow", glow)
	_reposition_interior(width)
	_update_active_edge_glow()


func _on_morph_finished() -> void:
	if not is_instance_valid(self):
		return
	_current_sides = _target_sides
	_morph_tween = null
	# Endpoint of the tween already equals the exact resample_ngon(target,...)
	# shape (resample_ngon's t=1 endpoint is identical to a fresh call), so no
	# further geometry rebuild is needed here — only settle the desks fully
	# (positions, not the node set, in case anything drifted from rounding).
	_reposition_interior(_effective_width)
	_update_active_edge_glow()
	if _pending_dimensions:
		# A resize came in while this morph was running (see
		# set_floor_dimensions) — apply it now that no tween is in flight, so
		# this always takes the non-mid-morph (immediate rebuild) path.
		_pending_dimensions = false
		var w: float = _pending_width
		var h: float = _pending_height
		set_floor_dimensions(w, h)


func add_agent_slot(agent_id: String, edge_index: int, character_class: String = "apprentice", provider: String = "", power_level: float = 0.0) -> void:
	for slot: Dictionary in _agent_slots:
		if slot["agent_id"] == agent_id:
			return
	_agent_slots.append({
		"agent_id": agent_id,
		"edge_index": edge_index,
		"character_class": character_class,
		"state": "idle",
		"provider": provider,
		"power_level": power_level,
	})
	if edge_index == _active_edge:
		_rebuild_interior()


func remove_agent_slot(agent_id: String) -> void:
	_agent_slots = _agent_slots.filter(
		func(s: Dictionary) -> bool: return s["agent_id"] != agent_id
	)
	_rebuild_interior()


## Total agents assigned to this floor, across all edges.
func get_agent_count() -> int:
	return _agent_slots.size()


## Agents on this floor currently in an active state (assigned/working/reporting).
func get_active_count() -> int:
	var count: int = 0
	for slot: Dictionary in _agent_slots:
		if slot.get("state", "idle") in ACTIVE_STATES:
			count += 1
	return count


func get_agent_count_on_edge(edge: int) -> int:
	var count: int = 0
	for slot: Dictionary in _agent_slots:
		if slot["edge_index"] == edge:
			count += 1
	return count


## Current composite_load, as last set via set_composite_load(). Read by
## TowerManager.get_floor_infos().
func get_composite_load() -> float:
	return _composite_load


## Update the stored state for an agent and propagate to its live node if visible.
func update_agent_state(agent_id: String, state: String) -> void:
	for slot: Dictionary in _agent_slots:
		if slot["agent_id"] == agent_id:
			slot["state"] = state
			break
	var char_node: AgentCharacter = get_agent_character(agent_id)
	if char_node:
		char_node.set_animation_state(state)


## Return the live AgentCharacter node for an agent, or null if not on the active edge.
func get_agent_character(agent_id: String) -> AgentCharacter:
	for child: Node in _agent_slots_node.get_children():
		if child is AgentCharacter and (child as AgentCharacter).agent_id == agent_id:
			return child as AgentCharacter
	return null


## Hides the three bands, the wash, and the windows and torches. TowerManager
## calls this for any floor at fisheye distance 2 or more.
func set_show_dressing(visible_flag: bool) -> void:
	_show_dressing = visible_flag
	if _dressing != null and is_instance_valid(_dressing):
		_dressing.visible = visible_flag


func set_show_interior(visible_flag: bool) -> void:
	_interior.visible = visible_flag
	_agent_slots_node.visible = visible_flag
	# _name_label stays hidden. WorldLabels draws the floor name on UILayer and
	# mirrors this same distance rule.


## Demo-parity slab outline: a rectangle with chamfered short ends (the HTML
## demo's clip-path), NOT the resampled n-gon lens. T15 load still drives the
## width (breathe) and edge glow; the side-count bucket no longer changes the
## silhouette, which the lens shape distorted beyond recognition.
func _slab_polygon(width: float) -> PackedVector2Array:
	var hw: float = width / 2.0
	var hh: float = _floor_height / 2.0
	var ch: float = minf(8.0, hh)
	return PackedVector2Array([
		Vector2(-hw + ch, -hh), Vector2(hw - ch, -hh),
		Vector2(hw, -hh + ch), Vector2(hw, hh - ch),
		Vector2(hw - ch, hh), Vector2(-hw + ch, hh),
		Vector2(-hw, hh - ch), Vector2(-hw, -hh + ch),
	])


func _rebuild_background() -> void:
	var scaled: PackedVector2Array = _slab_polygon(_effective_width)
	# Phase 4 — the slab carries no wall texture any more. It is the silhouette
	# behind the three bands, so it only fills the gaps the bands leave.
	_background.polygon = scaled
	_background.texture = null
	_background.uv = PackedVector2Array()
	_background.color = Color(0.129, 0.145, 0.184, 1.0)
	if _edge_glow:
		_edge_glow.points = scaled
		var mat: ShaderMaterial = _edge_glow.material as ShaderMaterial
		if mat:
			mat.set_shader_parameter("glow", _composite_load)
	_morph_last_unit = FloorMorph.resample_ngon(_current_sides, RESAMPLE_K, ROTATION)
	_morph_last_width = _effective_width
	_morph_last_glow = _composite_load
	_rebuild_dressing()


## Rebuilds the decorative layer: the three depth bands, the cornice, the
## per-floor wash, glowing windows, and flickering torches. Sits directly above
## the slab silhouette, behind agents.
func _rebuild_dressing() -> void:
	if _dressing != null and is_instance_valid(_dressing):
		_dressing.queue_free()
	_torches.clear()
	_dressing = Node2D.new()
	add_child(_dressing)
	move_child(_dressing, _background.get_index() + 1)
	_dressing.visible = _show_dressing

	var half_w: float = _floor_width / 2.0
	var half_h: float = _floor_height / 2.0

	# Band A. Back wall, the top 16 px of the slab.
	_wall_band = Polygon2D.new()
	_wall_band.texture = WALL_TEXTURE
	_wall_band.texture_repeat = CanvasItem.TEXTURE_REPEAT_ENABLED
	_wall_band.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	_wall_band.color = Color(0.82, 0.82, 0.88, 1.0)  # back wall sits in shadow
	_wall_band.texture_offset.x = _wall_scroll_x  # a rebuild keeps the scroll
	_dressing.add_child(_wall_band)

	# Band B. The 3/4-view walkable plane, inside the slab, below the wall line.
	_floor_plane = Polygon2D.new()
	_floor_plane.texture = FLOOR_TILES.get(floor_tile, FLOOR_TILES["stone_floor"])
	_floor_plane.texture_repeat = CanvasItem.TEXTURE_REPEAT_ENABLED
	_floor_plane.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	_floor_plane.color = Color(1.08, 1.08, 1.05, 1.0)  # lit floor, brighter than wall
	_dressing.add_child(_floor_plane)

	# Band C. Front lip, a 3 px plinth shadow at the slab bottom.
	_front_lip = Polygon2D.new()
	_front_lip.color = FRONT_LIP_COLOR
	_dressing.add_child(_front_lip)

	# Per-floor identity wash over bands A and B, multiplied.
	_wash = Polygon2D.new()
	var wash_mat := CanvasItemMaterial.new()
	wash_mat.blend_mode = CanvasItemMaterial.BLEND_MODE_MUL
	_wash.material = wash_mat
	# The multiply blend resolves to dst * (src_rgb + 1 - src_a). A raw tint at
	# alpha 0.15 gives a factor above 1.0, which brightens the bands instead of
	# tinting them. Pre-scale the tint toward white and keep alpha at 1.0, so the
	# factor is exactly a 15 percent multiply toward wash_tint.
	var wash_rgb: Color = Color.WHITE.lerp(wash_tint, WASH_ALPHA)
	_wash.color = Color(wash_rgb.r, wash_rgb.g, wash_rgb.b, 1.0)
	_dressing.add_child(_wash)

	# Cornice (light stone strip along the top).
	_cornice = Polygon2D.new()
	_cornice.color = Color(0.42, 0.44, 0.52, 1.0)
	_dressing.add_child(_cornice)

	_apply_band_geometry(_effective_width)

	# Windows spaced across the wall band, skipping the center band.
	var x: float = -half_w + WINDOW_START
	while x < half_w - 16.0:
		if absf(x) > 32.0:
			var win := Sprite2D.new()
			win.texture = WINDOW_TEXTURE
			win.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
			win.position = Vector2(x, -half_h + WALL_BAND_HEIGHT / 2.0)
			_dressing.add_child(win)
		x += WINDOW_SPACING

	# Torches flanking the wall band near each end.
	for tx: float in [-half_w + TORCH_INSET, half_w - TORCH_INSET]:
		var torch := Sprite2D.new()
		torch.texture = TORCH_TEXTURE
		torch.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
		torch.position = Vector2(tx, -half_h + WALL_BAND_HEIGHT / 2.0)
		_dressing.add_child(torch)
		_torches.append(torch)


## Restretches the three bands, the wash, and the cornice to a breathed width.
## A morph frame calls this every tick, so it allocates no nodes.
func _apply_band_geometry(width: float) -> void:
	if _wall_band == null or not is_instance_valid(_wall_band):
		return
	var hw: float = width / 2.0
	var hh: float = _floor_height / 2.0
	var ch: float = minf(8.0, hh)
	var wall_line: float = -hh + WALL_BAND_HEIGHT
	var plane_bottom: float = wall_line + PLANE_DEPTH

	# Band A follows the slab's top chamfer so it never spills past the edge.
	var wall_pts := PackedVector2Array([
		Vector2(-hw + ch, -hh), Vector2(hw - ch, -hh),
		Vector2(hw, -hh + ch), Vector2(hw, wall_line),
		Vector2(-hw, wall_line), Vector2(-hw, -hh + ch),
	])
	_wall_band.polygon = wall_pts
	# UVs in texture pixels so the 16 px tile repeats across the wall.
	_wall_band.uv = wall_pts

	# Band B narrows at the back and reaches full slab width at the front, so
	# the flare reads as depth without breaking the silhouette.
	var plane_pts := PackedVector2Array([
		Vector2(-hw + PLANE_FLARE, wall_line), Vector2(hw - PLANE_FLARE, wall_line),
		Vector2(hw, plane_bottom), Vector2(-hw, plane_bottom),
	])
	_floor_plane.polygon = plane_pts
	_floor_plane.uv = plane_pts

	# Band C tucks inside the bottom chamfer.
	var lip_top: float = hh - FRONT_LIP_HEIGHT
	var lip_inset: float = maxf(0.0, ch - FRONT_LIP_HEIGHT)
	_front_lip.polygon = PackedVector2Array([
		Vector2(-hw + lip_inset, lip_top), Vector2(hw - lip_inset, lip_top),
		Vector2(hw - ch, hh), Vector2(-hw + ch, hh),
	])

	_wash.polygon = PackedVector2Array([
		Vector2(-hw + ch, -hh), Vector2(hw - ch, -hh),
		Vector2(hw, -hh + ch), Vector2(hw, plane_bottom),
		Vector2(-hw, plane_bottom), Vector2(-hw, -hh + ch),
	])

	_cornice.polygon = PackedVector2Array([
		Vector2(-hw - 4.0, -hh - 3.0), Vector2(hw + 4.0, -hh - 3.0),
		Vector2(hw + 4.0, -hh + 1.0), Vector2(-hw - 4.0, -hh + 1.0),
	])


## Y of an AgentCharacter's origin. Feet land at band B mid-depth, so the head
## overlaps the wall line and clears the cornice.
func _character_y() -> float:
	var feet_y: float = -_floor_height / 2.0 + WALL_BAND_HEIGHT + PLANE_DEPTH * 0.55
	return feet_y - EdgeLayout.DESK_HEIGHT / 2.0


## Y of a workstation prop. Props sit at the back edge of band B, against the
## wall line.
func _prop_y() -> float:
	return -_floor_height / 2.0 + WALL_BAND_HEIGHT + 2.0


## Structural rebuild: destroys and recreates AgentCharacter nodes for the
## active edge. Only called when the SET of agents on the active edge (or
## which edge is active, or polygon_sides) actually changes — never called
## per-frame during a morph, so desks never pop mid-animation. Frame-by-frame
## repositioning during a morph is handled by _reposition_interior() instead.
func _rebuild_interior() -> void:
	for child: Node in _agent_slots_node.get_children():
		child.queue_free()
	var edge_agents: Array[Dictionary] = []
	for slot: Dictionary in _agent_slots:
		if slot["edge_index"] == _active_edge:
			edge_agents.append(slot)
	if edge_agents.is_empty():
		return
	var edge_width: float = EdgeLayout.edge_width_for_polygon(polygon_sides, _effective_width)
	var positions: Array[Vector2] = EdgeLayout.calculate_positions(edge_agents.size(), edge_width)
	# Characters stand ON band B: feet land mid-plane so the down-angle reads,
	# and heads clear the cornice. Only x comes from EdgeLayout; y is fixed.
	var char_y: float = _character_y()
	for i: int in range(edge_agents.size()):
		var slot: Dictionary = edge_agents[i]
		var char_node: AgentCharacter = AGENT_CHARACTER_SCENE.instantiate() as AgentCharacter
		char_node.agent_id = slot["agent_id"]

		# Workstation prop behind the agent, back against the wall base.
		var prop := Sprite2D.new()
		var atlas := AtlasTexture.new()
		atlas.atlas = PROPS_TEXTURE
		var prop_idx: int = absi((slot["agent_id"] as String).hash() / 7) % PROP_COUNT
		atlas.region = Rect2(prop_idx * 16, 0, 16, 16)
		prop.texture = atlas
		prop.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
		prop.position = Vector2(
			positions[i].x + EdgeLayout.DESK_WIDTH / 2.0 - 10.0,
			_prop_y()
		)
		_agent_slots_node.add_child(prop)

		_agent_slots_node.add_child(char_node)
		char_node.position = Vector2(positions[i].x + EdgeLayout.DESK_WIDTH / 2.0, char_y)
		char_node.set_character_class(slot.get("character_class", "apprentice"))
		char_node.set_animation_state(slot.get("state", "idle"))
		char_node.set_provider(slot.get("provider", ""))
		# T17 (#127) — budget must land before power_level so particles
		# configure against the correct cap on first apply (see
		# AgentCharacter.set_particle_budget() doc-comment).
		char_node.set_particle_budget(_particle_budget)
		char_node.set_power_level(slot.get("power_level", 0.0))
		char_node.character_clicked.connect(func(agent_id: String) -> void:
			agent_clicked.emit(agent_id)
		)
		char_node.character_right_clicked.connect(func(agent_id: String) -> void:
			agent_right_clicked.emit(agent_id)
		)
		char_node.character_hovered.connect(func(agent_id: String) -> void:
			agent_hovered.emit(agent_id)
		)
		char_node.character_unhovered.connect(func(agent_id: String) -> void:
			agent_unhovered.emit(agent_id)
		)


## Frame-driven reposition of the EXISTING AgentCharacter nodes on the active
## edge to a breathed effective_width, without destroying/recreating them.
## Called every frame during a morph tween (via _apply_morph_t) — this is
## what makes desks "take more horizontal space" smoothly (criterion #124.3)
## instead of popping to a new layout once the tween completes.
func _reposition_interior(effective_width: float) -> void:
	var children: Array[Node] = _agent_slots_node.get_children()
	if children.is_empty():
		return
	# _rebuild_interior adds one prop then one character per agent, so the
	# child list is pairs. Count the characters to get the true slot count.
	var agent_count: int = 0
	for child: Node in children:
		if child is AgentCharacter:
			agent_count += 1
	if agent_count == 0:
		return
	var edge_width: float = EdgeLayout.edge_width_for_polygon(polygon_sides, effective_width)
	var positions: Array[Vector2] = EdgeLayout.calculate_positions(agent_count, edge_width)
	var char_y: float = _character_y()
	var prop_y: float = _prop_y()
	var slot: int = 0
	for child: Node in children:
		if slot >= positions.size():
			break
		var base_x: float = positions[slot].x + EdgeLayout.DESK_WIDTH / 2.0
		if child is AgentCharacter:
			(child as AgentCharacter).position = Vector2(base_x, char_y)
			slot += 1
		elif child is Node2D:
			(child as Node2D).position = Vector2(base_x - 10.0, prop_y)


## Recomputes the ActiveEdgeGlow outline segment (criterion #124.4 — glow
## emphasis specifically on the active edge). Uses polygon_sides/_active_edge
## directly (already the target/final side count — see _rehome_for_sides)
## rather than the mid-morph resampled curve, so the segment stays a clean
## single straight edge throughout the animation and only breathes in size.
func _update_active_edge_glow() -> void:
	if not _active_edge_glow:
		return
	if polygon_sides < 3:
		_active_edge_glow.points = PackedVector2Array()
		return
	var unit_pts: PackedVector2Array = FloorMorph.regular_ngon_unit(polygon_sides, ROTATION)
	var a: Vector2 = unit_pts[_active_edge % polygon_sides]
	var b: Vector2 = unit_pts[(_active_edge + 1) % polygon_sides]
	var half_width: float = _effective_width / 2.0
	var half_height: float = _floor_height / 2.0
	_active_edge_glow.points = PackedVector2Array([
		Vector2(a.x * half_width, a.y * half_height),
		Vector2(b.x * half_width, b.y * half_height),
	])
	var mat: ShaderMaterial = _active_edge_glow.material as ShaderMaterial
	if mat:
		mat.set_shader_parameter("glow", _composite_load)
