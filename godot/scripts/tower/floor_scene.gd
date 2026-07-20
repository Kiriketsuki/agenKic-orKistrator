extends Node2D
## FloorScene — a single floor in the tower. Manages edge rotation,
## AgentCharacter spawning, the ephemeral lifecycle state machine, and
## (T15/#124) composite-load-driven polygon morphing.

signal agent_clicked(agent_id: String)
signal agent_right_clicked(agent_id: String)
signal agent_hovered(agent_id: String)
signal agent_unhovered(agent_id: String)

const AGENT_CHARACTER_SCENE: PackedScene = preload("res://scenes/agent_character.tscn")

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

var _state: FloorState = FloorState.ACTIVE
var _active_edge: int = 0
var _floor_width: float = 280.0
var _floor_height: float = 40.0
## Each entry: {agent_id, edge_index, character_class, state}
var _agent_slots: Array[Dictionary] = []
var _linger_timer: float = 0.0
var _linger_duration: float = 30.0

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

@onready var _background: Polygon2D = $Background
@onready var _edge_glow: Line2D = $EdgeGlow
@onready var _active_edge_glow: Line2D = $ActiveEdgeGlow
@onready var _interior: Node2D = $Interior
@onready var _agent_slots_node: Node2D = $AgentSlots
@onready var _name_label: Label = $NameLabel


func _ready() -> void:
	_name_label.text = floor_label if floor_label != "" else floor_name
	_current_sides = polygon_sides
	_target_sides = polygon_sides
	_effective_width = _floor_width
	_rebuild_background()
	_rebuild_interior()
	_update_active_edge_glow()


func _process(delta: float) -> void:
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
	var scaled: PackedVector2Array = FloorMorph.scale_unit_array(unit_pts, width / 2.0, _floor_height / 2.0)
	_background.polygon = scaled
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


func add_agent_slot(agent_id: String, edge_index: int, character_class: String = "apprentice", provider: String = "") -> void:
	for slot: Dictionary in _agent_slots:
		if slot["agent_id"] == agent_id:
			return
	_agent_slots.append({
		"agent_id": agent_id,
		"edge_index": edge_index,
		"character_class": character_class,
		"state": "idle",
		"provider": provider,
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


func set_show_interior(visible_flag: bool) -> void:
	_interior.visible = visible_flag
	_agent_slots_node.visible = visible_flag
	_name_label.visible = visible_flag


func _rebuild_background() -> void:
	var unit_pts: PackedVector2Array = FloorMorph.resample_ngon(_current_sides, RESAMPLE_K, ROTATION)
	var scaled: PackedVector2Array = FloorMorph.scale_unit_array(unit_pts, _effective_width / 2.0, _floor_height / 2.0)
	_background.polygon = scaled
	_background.color = Color(0.18, 0.22, 0.18, 1.0)  # dark stone green
	if _edge_glow:
		_edge_glow.points = scaled
		var mat: ShaderMaterial = _edge_glow.material as ShaderMaterial
		if mat:
			mat.set_shader_parameter("glow", _composite_load)
	_morph_last_unit = unit_pts
	_morph_last_width = _effective_width
	_morph_last_glow = _composite_load


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
	# Offset converts EdgeLayout's top-left corner to AgentCharacter's center origin.
	var center_offset: Vector2 = Vector2(EdgeLayout.DESK_WIDTH / 2.0, EdgeLayout.DESK_HEIGHT / 2.0)
	for i: int in range(edge_agents.size()):
		var slot: Dictionary = edge_agents[i]
		var char_node: AgentCharacter = AGENT_CHARACTER_SCENE.instantiate() as AgentCharacter
		char_node.agent_id = slot["agent_id"]
		_agent_slots_node.add_child(char_node)
		char_node.position = positions[i] + center_offset
		char_node.set_character_class(slot.get("character_class", "apprentice"))
		char_node.set_animation_state(slot.get("state", "idle"))
		char_node.set_provider(slot.get("provider", ""))
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
	var edge_width: float = EdgeLayout.edge_width_for_polygon(polygon_sides, effective_width)
	var positions: Array[Vector2] = EdgeLayout.calculate_positions(children.size(), edge_width)
	var center_offset: Vector2 = Vector2(EdgeLayout.DESK_WIDTH / 2.0, EdgeLayout.DESK_HEIGHT / 2.0)
	for i: int in range(mini(children.size(), positions.size())):
		var child: Node = children[i]
		if child is Node2D:
			(child as Node2D).position = positions[i] + center_offset


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
