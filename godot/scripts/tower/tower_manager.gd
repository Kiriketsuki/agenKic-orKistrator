extends Node2D
## TowerManager — fisheye layout engine, floor ordering, scroll/zoom, signal routing.

signal agent_panel_requested(agent_id: String)
signal agent_context_menu_requested(agent_id: String, screen_position: Vector2)
signal agent_hover_requested(agent_id: String)
signal agent_unhover_requested(agent_id: String)
signal floor_focus_changed(index: int)
signal floors_changed()
## Fires when an agent's *state* changes (idle/working/reporting/etc) without
## any change to floor membership or agent_count — e.g. minimap's activity
## coloring needs this, but floor_tabs' name+count badges do not, so it is
## kept separate from floors_changed to avoid a full tab-strip rebuild on
## every state transition (see _on_agent_state_changed).
signal agent_activity_changed()

const FLOOR_SCENE: PackedScene = preload("res://scenes/floor_scene.tscn")

## Phase 4 per-floor dressing, keyed by floor name. The tile names the band B
## texture, the wash names the multiply tint over bands A and B.
const FLOOR_DRESSING: Dictionary = {
	"main": {"tile": "stone_floor", "wall": "wall_moss", "wash": Color(0.227, 0.290, 0.227, 1.0)},
	"archive": {"tile": "stone_floor", "wall": "stone_wall", "wash": Color(0.180, 0.204, 0.282, 1.0)},
	"orkistrator": {"tile": "wood_floor", "wall": "stone_wall", "wash": Color(0.290, 0.227, 0.180, 1.0)},
	"_default": {"tile": "stone_floor", "wall": "stone_wall", "wash": Color(0.227, 0.290, 0.227, 1.0)},
}
const FOCUSED_SCALE: float = 1.0
const ADJACENT_SCALE: float = 0.4
## PARITY (2026-08-04) — the tower world lives in 1x art pixels and the camera
## is the only magnifier. Zoom is always an integer from this set, so the pixel
## grid never shimmers. 6 is the 1080p base (1080 / 180 = 6).
const ZOOM_LEVELS: Array[int] = [4, 5, 6, 8]
## Free-zoom bounds for ctrl+wheel. ZOOM_LEVELS stays as the base-zoom
## ladder for viewport sizing; the user's wheel roams the full range.
const MIN_ZOOM: int = 1
const MAX_ZOOM: int = 64
const MAX_QUEUE_SIZE: int = 2
## Art-px world truth. These are absolute, never derived from viewport size.
const BASE_FLOOR_WIDTH: float = 280.0
const BASE_FLOOR_HEIGHT: float = 40.0
const BASE_TOWER_RADIUS: float = 40.0
const FLOOR_SPACING: float = 56.0
## Demo refocus timing. Position, scale and alpha all move together over this
## duration with an expo ease out. No overshoot anywhere.
const REFOCUS_DUR: float = 0.55

@export var config_path: String = "res://config/tower.json"

var _config: TowerConfig
var _floors: Array[Node2D] = []  # ordered bottom to top
var _focused_index: int = 0
var _agent_assignments: Dictionary = {}  # agent_id → {floor: String, edge: int}
## T15 (#124) — per-floor ring of recent-completion unix timestamps, used for
## the honest task_throughput_norm proxy. Keyed by floor_name. See
## FloorMorph's doc-comment for the full honest-metric rationale: this is a
## client-observed "completions per window" signal, not tokens/sec, because
## no real cost data reaches the client (or the orchestrator's store) today.
var _floor_completion_rings: Dictionary = {}
## T15 (#124) council finding — periodic sweep so composite_load (and
## therefore polygon side count) decays even when no new agent event fires.
## Only floors with a non-empty completion ring are touched each tick.
var _load_recompute_timer: Timer = null
var _scroll_tween: Tween = null
var _fisheye_tween: Tween = null
var _overscroll_tween: Tween = null
var _is_overscrolling: bool = false
var _input_queue: Array[int] = []
var _floor_spacing: float = FLOOR_SPACING
var _floor_width: float = BASE_FLOOR_WIDTH
var _floor_height: float = BASE_FLOOR_HEIGHT
var _tower_radius: float = BASE_TOWER_RADIUS
var _master_region: Rect2 = Rect2()
## True once the user ctrl-zooms away from the viewport-derived base level. A
## resize then keeps their chosen zoom instead of snapping back.
var _user_zoom_override: bool = false
## Per-floor in-flight edge-rotate tween, killed on re-click to prevent drift.
var _edge_tweens: Dictionary = {}
## State of a multi-edge walk started by rotate_focused_to_edge.
var _edge_walk_dir: int = 0
var _edge_walk_steps: int = 0
## The floor the walk belongs to. The walk finishes on the floor the user
## clicked, so a focus change part way through never redirects the remaining
## steps onto another floor.
var _edge_walk_floor: Node2D = null

@onready var _floors_container: Node2D = $FloorsContainer
@onready var _camera: Camera2D = $Camera
## Phase 6 section 4. Fades the bottom of the stair shaft into the treeline.
const BASE_FADE_SHADER: Shader = preload("res://shaders/base_fade.gdshader")
## How many pixels of the shaft bottom the fade covers.
const SHAFT_FADE_PX: float = 32.0

var _shaft_fade_material: ShaderMaterial = null

@onready var _tower_exterior: Node2D = $TowerExterior
@onready var _stair_shaft: Sprite2D = $StairShaft


func _ready() -> void:
	# Grimoire Summoning (F3) — the flyout looks this node up by group instead
	# of a hardcoded scene path, so it works from any UI layer without a
	# direct node reference.
	add_to_group("tower_manager")
	_config = TowerConfig.from_file(config_path)
	get_viewport().size_changed.connect(_on_viewport_size_changed)
	_apply_base_zoom()
	_spawn_permanent_floors()
	_recalculate_layout_metrics()
	_layout_floors()
	_update_tower_frame()
	_apply_fisheye_layout()
	_sync_tower_exterior()
	_load_recompute_timer = Timer.new()
	_load_recompute_timer.wait_time = maxf(_config.load_recompute_interval_sec, 0.1)
	_load_recompute_timer.autostart = true
	_load_recompute_timer.timeout.connect(_on_load_recompute_timer_timeout)
	add_child(_load_recompute_timer)
	var bridge: Node = Engine.get_singleton("BridgeManager") if Engine.has_singleton("BridgeManager") else get_node_or_null("/root/BridgeManager")
	if bridge:
		bridge.connect("floor_created", _on_floor_created)
		bridge.connect("floor_removed", _on_floor_removed)
		bridge.connect("agent_registered", _on_agent_registered)
		bridge.connect("agent_state_changed", _on_agent_state_changed)
		bridge.connect("agent_deregistered", _on_agent_deregistered)
		bridge.connect("agent_output", _on_agent_output)
		bridge.connect("connection_status_changed", _on_connection_status_changed)


func _unhandled_input(event: InputEvent) -> void:
	if event.is_action_pressed("rotate_left"):
		_rotate_focused_edge(-1)
		get_viewport().set_input_as_handled()
	elif event.is_action_pressed("rotate_right"):
		_rotate_focused_edge(1)
		get_viewport().set_input_as_handled()
	elif event.is_action_pressed("scroll_up"):
		_scroll_focus(1)
		get_viewport().set_input_as_handled()
	elif event.is_action_pressed("scroll_down"):
		_scroll_focus(-1)
		get_viewport().set_input_as_handled()
	elif event is InputEventMouseButton:
		var mb: InputEventMouseButton = event as InputEventMouseButton
		if mb.pressed:
			if mb.button_index == MOUSE_BUTTON_WHEEL_UP:
				if mb.ctrl_pressed:
					# Ctrl+wheel up zooms IN (a larger camera zoom magnifies).
					_zoom(1)
				else:
					_scroll_focus(1)
				get_viewport().set_input_as_handled()
			elif mb.button_index == MOUSE_BUTTON_WHEEL_DOWN:
				if mb.ctrl_pressed:
					_zoom(-1)
				else:
					_scroll_focus(-1)
				get_viewport().set_input_as_handled()
	elif event is InputEventKey:
		var k: InputEventKey = event as InputEventKey
		if k.pressed and not k.echo and not k.ctrl_pressed and not k.alt_pressed and not k.meta_pressed \
				and k.keycode >= KEY_1 and k.keycode <= KEY_9:
			jump_to_floor(k.keycode - KEY_1)
			get_viewport().set_input_as_handled()


func _spawn_permanent_floors() -> void:
	for floor_def: Dictionary in _config.permanent_floors:
		var floor_scene: Node2D = _create_floor(
			floor_def.get("name", ""),
			floor_def.get("label", ""),
			true
		)
		_floors.append(floor_scene)
	if not _floors.is_empty():
		_focused_index = 0


func _create_floor(floor_name: String, label: String, permanent: bool) -> Node2D:
	var instance: Node2D = FLOOR_SCENE.instantiate()
	instance.floor_name = floor_name
	instance.floor_label = label
	instance.is_permanent = permanent
	instance.polygon_sides = _config.polygon_sides
	# Phase 4 — per-floor identity: floor tile and multiply wash tint.
	var dressing: Dictionary = FLOOR_DRESSING.get(floor_name, FLOOR_DRESSING["_default"])
	instance.floor_tile = dressing["tile"]
	instance.wall_texture = dressing.get("wall", "stone_wall")
	instance.wash_tint = dressing["wash"]
	instance.set_meta("floor_name", floor_name)
	if instance.has_method("configure_load_params"):
		instance.configure_load_params(
			_config.min_sides, _config.max_sides,
			_config.breathe_min_scale, _config.breathe_max_scale,
			_config.bucket_hysteresis
		)
	# T17 (#127) acceptance #5 — threads the global particle budget cap once
	# per floor (not per agent — see FloorScene.configure_particle_budget()
	# doc-comment).
	if instance.has_method("configure_particle_budget"):
		instance.configure_particle_budget(_config.max_particles_per_agent)
	instance.agent_clicked.connect(func(agent_id: String) -> void:
		agent_panel_requested.emit(agent_id)
	)
	instance.agent_right_clicked.connect(func(agent_id: String) -> void:
		agent_context_menu_requested.emit(agent_id, get_viewport().get_mouse_position())
	)
	instance.agent_hovered.connect(func(agent_id: String) -> void:
		agent_hover_requested.emit(agent_id)
	)
	instance.agent_unhovered.connect(func(agent_id: String) -> void:
		agent_unhover_requested.emit(agent_id)
	)
	_floors_container.add_child(instance)
	return instance


## Sets absolute Y positions for all floors. Call after any change to _floors.
func _layout_floors() -> void:
	for i: int in range(_floors.size()):
		_floors[i].position = Vector2(0.0, i * -_floor_spacing)
		if _floors[i].has_method("set_floor_dimensions"):
			_floors[i].set_floor_dimensions(_floor_width, _floor_height)
	_update_stair_shaft()


## Tiles stair_shaft.png vertically on the tower axis so the floor stack reads
## as one building instead of separate floating plates. The sprite spans the
## bottom slab bottom to the top slab top, and sits behind FloorsContainer in
## scene order.
func _update_stair_shaft() -> void:
	if _stair_shaft == null:
		return
	if _floors.is_empty():
		_stair_shaft.visible = false
		return
	var tex: Texture2D = _stair_shaft.texture
	if tex == null:
		_stair_shaft.visible = false
		return
	var shaft_w: float = tex.get_width()
	var span: float = (_floors.size() - 1) * _floor_spacing + _floor_height
	_stair_shaft.visible = true
	_stair_shaft.centered = false
	_stair_shaft.texture_repeat = CanvasItem.TEXTURE_REPEAT_ENABLED
	_stair_shaft.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
	_stair_shaft.region_enabled = true
	_stair_shaft.region_rect = Rect2(0.0, 0.0, shaft_w, span)
	# Top slab top edge, in Tower-local coords.
	var top_y: float = -(_floors.size() - 1) * _floor_spacing - _floor_height / 2.0
	_stair_shaft.position = Vector2(-shaft_w / 2.0, top_y)
	# Phase 5 section 4 — the shaft sits behind the rooms, so it recedes
	# instead of reading as a monolith in front of the sky.
	_stair_shaft.modulate = Color(0.8, 0.8, 0.8, 1.0)
	# Phase 6 section 4 — the last SHAFT_FADE_PX of the shaft fade to nothing, so
	# the tower has no hard bottom edge under the treeline. Local Y on this
	# sprite runs from 0 at the top to span at the bottom.
	if _shaft_fade_material == null:
		_shaft_fade_material = ShaderMaterial.new()
		_shaft_fade_material.shader = BASE_FADE_SHADER
		_stair_shaft.material = _shaft_fade_material
	_shaft_fade_material.set_shader_parameter("fade_start_y", span - SHAFT_FADE_PX)
	_shaft_fade_material.set_shader_parameter("fade_end_y", span)


## Tweens scale and opacity of all floors based on distance from _focused_index.
func _apply_fisheye_layout() -> void:
	if _fisheye_tween:
		_fisheye_tween.kill()
	_fisheye_tween = create_tween().set_parallel(true)
	for i: int in range(_floors.size()):
		var floor_node: Node2D = _floors[i]
		var distance: int = absi(i - _focused_index)
		var target_scale: Vector2
		var target_alpha: float
		var show_interior: bool
		if distance == 0:
			target_scale = Vector2(FOCUSED_SCALE, FOCUSED_SCALE)
			target_alpha = 1.0 if floor_node.get_floor_state() != floor_node.FloorState.LINGERING else 0.5
			show_interior = true
		elif distance == 1:
			target_scale = Vector2(ADJACENT_SCALE, ADJACENT_SCALE)
			target_alpha = 0.7
			show_interior = true
		else:
			target_scale = Vector2(ADJACENT_SCALE * 0.6, ADJACENT_SCALE * 0.6)
			target_alpha = 0.4
			show_interior = false
		_fisheye_tween.tween_property(floor_node, "scale", target_scale, REFOCUS_DUR).set_trans(Tween.TRANS_EXPO).set_ease(Tween.EASE_OUT)
		_fisheye_tween.tween_property(floor_node, "modulate:a", target_alpha, REFOCUS_DUR).set_trans(Tween.TRANS_EXPO).set_ease(Tween.EASE_OUT)
		floor_node.set_show_interior(show_interior)
		# Dressing follows the same distance rule. At distance 2 or more the
		# windows and torches drop below one screen pixel and read as noise.
		if floor_node.has_method("set_show_dressing"):
			floor_node.set_show_dressing(distance < 2)


func _scroll_focus(direction: int) -> void:
	if _is_overscrolling or _floors.is_empty():
		return
	var new_index: int = _focused_index + direction
	if new_index < 0 or new_index >= _floors.size():
		_input_queue.clear()
		_elastic_overscroll(direction)
		return
	if _scroll_tween != null and _scroll_tween.is_running():
		if _input_queue.size() < MAX_QUEUE_SIZE:
			_input_queue.append(direction)
		return
	_focused_index = new_index
	_do_scroll_tween()


func _do_scroll_tween() -> void:
	if _scroll_tween:
		_scroll_tween.kill()
	if _overscroll_tween:
		_overscroll_tween.kill()
		_overscroll_tween = null
	_is_overscrolling = false
	_scroll_tween = create_tween()
	var target_y: float = _focused_index * -_floor_spacing
	_scroll_tween.tween_property(_camera, "position:y", target_y, REFOCUS_DUR).set_trans(Tween.TRANS_EXPO).set_ease(Tween.EASE_OUT)
	_scroll_tween.tween_callback(_on_scroll_tween_finished)
	_apply_fisheye_layout()
	floor_focus_changed.emit(_focused_index)


func _on_scroll_tween_finished() -> void:
	if not _input_queue.is_empty():
		var next_direction: int = _input_queue.pop_front()
		_scroll_focus(next_direction)


func _elastic_overscroll(direction: int) -> void:
	if _is_overscrolling:
		return
	_is_overscrolling = true
	var original_y: float = _focused_index * -_floor_spacing
	var overshoot_y: float = original_y + (direction * _floor_spacing * -0.5)
	if _overscroll_tween:
		_overscroll_tween.kill()
	_overscroll_tween = create_tween()
	_overscroll_tween.tween_property(_camera, "position:y", overshoot_y, 0.15).set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_OUT)
	_overscroll_tween.tween_property(_camera, "position:y", original_y, 0.2).set_trans(Tween.TRANS_QUAD).set_ease(Tween.EASE_OUT)
	_overscroll_tween.tween_callback(func() -> void: _is_overscrolling = false)


## Jumps focus directly to `index` (clamped to valid range), reusing the same
## camera-tween + fisheye path as W/S scroll so minimap/tab clicks and number
## keys stay perfectly in sync with manual scrolling.
func jump_to_floor(index: int) -> void:
	if _floors.is_empty():
		return
	var target: int = clampi(index, 0, _floors.size() - 1)
	_input_queue.clear()
	_is_overscrolling = false
	if target == _focused_index:
		return
	_focused_index = target
	_do_scroll_tween()


## Number of floors currently in the tower.
func get_floor_count() -> int:
	return _floors.size()


## Index of the currently focused floor.
func get_focus_index() -> int:
	return _focused_index


## Ordered bottom→top snapshot of floor data for the minimap/floor-tabs UI.
func get_floor_infos() -> Array[Dictionary]:
	var infos: Array[Dictionary] = []
	for i: int in range(_floors.size()):
		var floor_node: Node2D = _floors[i]
		var label: String = floor_node.floor_label if floor_node.floor_label != "" else floor_node.floor_name
		var agent_count: int = floor_node.get_agent_count() if floor_node.has_method("get_agent_count") else 0
		var active_count: int = floor_node.get_active_count() if floor_node.has_method("get_active_count") else 0
		# T15 (#124) — additive-only keys; existing consumers (minimap.gd,
		# floor_tabs.gd, quest_board_view.gd) all read via Dictionary.get with
		# defaults, so these are safe to add without touching those files.
		var composite_load: float = floor_node.get_composite_load() if floor_node.has_method("get_composite_load") else 0.0
		infos.append({
			"index": i,
			"name": floor_node.get_meta("floor_name", ""),
			"label": label,
			"agent_count": agent_count,
			"active_count": active_count,
			"is_permanent": floor_node.is_permanent,
			"composite_load": composite_load,
			"polygon_sides": floor_node.polygon_sides,
		})
	return infos


## Snaps the camera to the base zoom for the current viewport height. The base
## is floorf(viewport_h / 180) pulled down to the nearest ZOOM_LEVELS entry at
## or below it, so 1080p lands on 6 and a small window still gets 4.
func _apply_base_zoom() -> void:
	if _user_zoom_override:
		return
	# Zoom derives from the DESIGN canvas (1080 -> 6), not the OS window: the
	# canvas_items stretch already maps design px to window px, so using the
	# window height here would double-scale (a 1066px tile picked 5, not 6).
	var raw: float = floorf(get_tree().root.content_scale_size.y / 180.0)
	if raw < 1.0:
		raw = floorf(get_viewport_rect().size.y / 180.0)
	var z: int = ZOOM_LEVELS[0]
	for level: int in ZOOM_LEVELS:
		if float(level) <= raw:
			z = level
	_camera.zoom = Vector2(z, z)


## Steps the zoom multiplicatively between MIN_ZOOM and MAX_ZOOM. Zoom
## stays integer so pixel art renders crisp, but the range is wide open:
## MAX_ZOOM puts the camera right up against a single agent. Each wheel
## step scales by about 25 percent, with a guaranteed minimum step of 1.
func _zoom(direction: int) -> void:
	var current: int = maxi(1, roundi(_camera.zoom.x))
	var next: int = current
	if direction > 0:
		next = maxi(current + 1, roundi(float(current) * 1.25))
	else:
		next = mini(current - 1, roundi(float(current) / 1.25))
	next = clampi(next, MIN_ZOOM, MAX_ZOOM)
	if next == current:
		return
	_camera.zoom = Vector2(next, next)
	_user_zoom_override = true
	# The offset formula divides by zoom.x, so a new zoom invalidates the old offset.
	_aim_camera_at_region()


func _nearest_zoom_index() -> int:
	var best: int = 0
	var best_delta: float = INF
	for i: int in range(ZOOM_LEVELS.size()):
		var delta: float = absf(float(ZOOM_LEVELS[i]) - _camera.zoom.x)
		if delta < best_delta:
			best_delta = delta
			best = i
	return best


## Turns the focused floor to an absolute edge index, the short way round.
## Phase 6 section 3 — the edge dots call this on a click. The turn walks one
## edge per step, so it reuses the same rotate path the keyboard uses and the
## prism sweeps continuously instead of jumping.
func rotate_focused_to_edge(edge: int) -> void:
	if _floors.is_empty() or _focused_index >= _floors.size():
		return
	var floor_node: Node2D = _floors[_focused_index]
	var sides: int = maxi(int(floor_node.polygon_sides), 3)
	var target: int = posmod(edge, sides)
	var delta: int = target - int(floor_node.get_active_edge())
	# Wrap the step into [-sides/2, sides/2], so a 5 to 0 move turns one step
	# forward instead of five steps back.
	var half: int = sides / 2
	while delta > half:
		delta -= sides
	while delta < -half:
		delta += sides
	if delta == 0:
		return
	_edge_walk_dir = signi(delta)
	_edge_walk_steps = absi(delta)
	_edge_walk_floor = floor_node
	_advance_edge_walk()


## Runs one step of a multi-edge walk, then queues the next step when the turn
## settles.
func _advance_edge_walk() -> void:
	if _edge_walk_steps <= 0:
		return
	if _edge_walk_floor == null or not is_instance_valid(_edge_walk_floor):
		_edge_walk_steps = 0
		_edge_walk_floor = null
		return
	_edge_walk_steps -= 1
	var floor_node: Node2D = _edge_walk_floor
	_rotate_floor_edge(floor_node, _edge_walk_dir)
	var tween: Tween = _edge_tweens.get(floor_node)
	if tween == null or not tween.is_valid():
		_edge_walk_steps = 0
		_edge_walk_floor = null
		return
	tween.finished.connect(_advance_edge_walk, CONNECT_ONE_SHOT)


func _rotate_focused_edge(direction: int, from_walk: bool = false) -> void:
	if not from_walk:
		# A direct turn cancels any walk still in flight, so the two inputs
		# never fight over the same floor.
		_edge_walk_steps = 0
		_edge_walk_floor = null
	if _floors.is_empty() or _focused_index >= _floors.size():
		return
	_rotate_floor_edge(_floors[_focused_index], direction)


## Turns one named floor by one edge. The walk and the keyboard both come here,
## so the per-floor kill discipline stays in one place.
func _rotate_floor_edge(floor_node: Node2D, direction: int) -> void:
	if floor_node == null or not is_instance_valid(floor_node):
		return
	var current_edge: int = floor_node.get_active_edge()
	# T15 (#124) council finding — floor_node.polygon_sides is now dynamic
	# (6..12, driven by composite_load) rather than the static _config value,
	# so rotation must wrap against the focused floor's own current side
	# count. Wrapping against _config.polygon_sides (always 6) would make
	# edges above index 5 unreachable once this floor has morphed larger.
	var sides: int = floor_node.polygon_sides
	var new_edge: int = (current_edge + direction) % sides
	if new_edge < 0:
		new_edge += sides
	# Phase 4: the floor node itself never moves. FloorScene.rotate_to_edge
	# turns the interior carousel and scrolls the wall band. The manager keeps
	# only the per-floor kill discipline, so spam clicks cannot stack tweens.
	var prev: Tween = _edge_tweens.get(floor_node)
	if prev != null and prev.is_valid():
		prev.kill()
	var tween: Tween = floor_node.rotate_to_edge(new_edge, direction)
	_edge_tweens[floor_node] = tween
	tween.finished.connect(func() -> void: _edge_tweens.erase(floor_node))


# --- Signal Handlers ---

# TODO(dynamic-floor): Two known cases where the idempotency guard at _on_agent_registered:169
# blocks recovery, to be resolved together in the dynamic floor lifecycle task:
# 1. Floor removal: non-permanent floors leave stale _agent_assignments entries when removed —
#    _on_floor_removed does not clean _agent_assignments, so reconnect re-registration is silently
#    dropped for agents that were on the removed floor.
# 2. Rapid deregister→re-register: _on_agent_deregistered defers _agent_assignments.erase() by
#    0.45s (exit animation window). If agent.registered fires for the same agent within that window
#    — e.g., an agent crash-restart under supervision — the guard at line 169 returns early and the
#    re-registration is permanently lost. The dropped SSE event is not re-emitted by the orchestrator.
func _on_floor_created(floor_data: BridgeData.FloorData) -> void:
	for existing: Node2D in _floors:
		if existing.get_meta("floor_name", "") == floor_data.name:
			return
	var floor_node: Node2D = _create_floor(floor_data.name, floor_data.name, floor_data.is_permanent)
	_floors.append(floor_node)
	_layout_floors()
	_apply_fisheye_layout()
	_update_tower_frame()
	_sync_tower_exterior()
	floors_changed.emit()


func _on_floor_removed(floor_name: String) -> void:
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			if floor_node.is_permanent:
				return
			floor_node.begin_linger(_config.linger_duration_sec)
			floor_node.tree_exiting.connect(func() -> void:
				# Preserve which floor was actually focused across the erase —
				# clamping the raw index would silently reassign focus to
				# whatever floor now sits at that numeric slot once a floor
				# below the focus is removed and the array shifts.
				var focused_floor: Node2D = _floors[_focused_index] if _focused_index < _floors.size() else null
				_floors.erase(floor_node)
				_floor_completion_rings.erase(floor_name)
				if focused_floor != null and focused_floor != floor_node:
					var new_index: int = _floors.find(focused_floor)
					_focused_index = new_index if new_index != -1 else clampi(_focused_index, 0, maxi(_floors.size() - 1, 0))
				else:
					_focused_index = clampi(_focused_index, 0, maxi(_floors.size() - 1, 0))
				_layout_floors()
				_update_tower_frame()
				_apply_fisheye_layout()
				floors_changed.emit()
				floor_focus_changed.emit(_focused_index)
			)
			return


func _on_agent_registered(agent_data: BridgeData.AgentData) -> void:
	if _agent_assignments.has(agent_data.id):
		return
	var floor_name: String = agent_data.floor_name
	# Grimoire Summoning (F3) — agent_data.floor is the numeric tower-index
	# the keeper dropped the sigil on. It names a slot in _floors by array
	# index rather than by floor_name, and it may point above the top of the
	# stack today (the "new floor" drop zone), so the stack grows to meet it.
	if agent_data.floor > 0:
		_ensure_floor_count(agent_data.floor + 1)
		if agent_data.floor < _floors.size():
			floor_name = _floors[agent_data.floor].get_meta("floor_name", "")
	if floor_name.is_empty() or not _has_floor(floor_name):
		floor_name = _floors[0].get_meta("floor_name", "main") if not _floors.is_empty() else "main"
	var edge: int = _find_best_edge_for_agent(floor_name)
	# The orchestrator does not persist character_class yet, so every agent
	# arrives as the default "apprentice". Derive a stable class from the
	# agent ID so the tower shows visual variety until the server carries it.
	var char_class: String = agent_data.character_class
	if char_class == "apprentice":
		var classes: Array[String] = [
			"alchemist", "scribe", "archmage", "wardkeeper",
			"librarian", "enchanter", "apprentice",
		]
		char_class = classes[absi(agent_data.id.hash()) % classes.size()]
	# T16 (#125) HONEST-MINIMAL power level — see palette_math.gd doc-comment.
	# No real per-agent tier signal reaches the client yet; class_power_levels
	# is an optional config demo scaffold, falling back to default_power_level.
	var power_level: float = _config.class_power_levels.get(
		char_class, _config.default_power_level
	)
	assign_agent_to_edge(agent_data.id, floor_name, edge, char_class, agent_data.provider, power_level)


func _on_agent_state_changed(agent_id: String, _old_state: String, new_state: String, _task_id: String) -> void:
	var assignment: Dictionary = _agent_assignments.get(agent_id, {})
	if assignment.is_empty():
		return
	var floor_name: String = assignment.get("floor", "")
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			floor_node.update_agent_state(agent_id, new_state)
			# T15 (#124) honest throughput proxy — a transition into "idle" is
			# treated as an observed completion. This is a client-observed
			# state-machine signal, not a real cost/throughput metric; see
			# FloorMorph's doc-comment for why token_cost_norm is dropped
			# entirely rather than faked from this.
			#
			# KNOWN LIMITATION (council finding, #124) — a task_cancelled SSE
			# event maps onto this exact same {state:"idle", no TaskID} shape
			# (see internal/httpbridge/sse.go case "task_cancelled": it reuses
			# SSEAgentStateChanged with State "idle" precisely so no new
			# frontend handler is needed). The client has no signal that lets
			# it tell a genuine completion apart from a cancellation here, so
			# a cancelled task is counted as a completion and can inflate
			# task_throughput_norm. Fixing this honestly would require the
			# orchestrator to emit a distinct event type — out of scope for a
			# client-only fix, and not worth inventing partial plumbing (e.g.
			# subtracting only client-initiated /cancel calls via
			# BridgeManager.command_succeeded would race the SSE event and
			# only cover cancels issued from this client, not other clients
			# or reassigns). Documented and accepted as-is.
			if new_state == "idle":
				_record_floor_completion(floor_name)
			_recompute_floor_load(floor_name)
			agent_activity_changed.emit()
			return


func _on_agent_deregistered(agent_id: String) -> void:
	var assignment: Dictionary = _agent_assignments.get(agent_id, {})
	if assignment.is_empty():
		_agent_assignments.erase(agent_id)
		return
	var floor_name: String = assignment.get("floor", "")
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			var char_node: AgentCharacter = floor_node.get_agent_character(agent_id)
			if char_node:
				char_node.play_exit_animation()
				# Remove slot after exit animation (0.4 s) so rebuild doesn't cull the fading node.
				var timer: SceneTreeTimer = get_tree().create_timer(0.45)
				timer.timeout.connect(func() -> void:
					if is_instance_valid(floor_node):
						floor_node.remove_agent_slot(agent_id)
						_recompute_floor_load(floor_name)
					_agent_assignments.erase(agent_id)
					floors_changed.emit()
				)
			else:
				# Agent is on a non-active edge — remove immediately.
				floor_node.remove_agent_slot(agent_id)
				_agent_assignments.erase(agent_id)
				_recompute_floor_load(floor_name)
				floors_changed.emit()
			return
	_agent_assignments.erase(agent_id)


func _on_agent_output(chunk: BridgeData.AgentOutputChunk) -> void:
	var assignment: Dictionary = _agent_assignments.get(chunk.agent_id, {})
	if assignment.is_empty():
		return
	var floor_name: String = assignment.get("floor", "")
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			var char_node: AgentCharacter = floor_node.get_agent_character(chunk.agent_id)
			if char_node:
				char_node.receive_output(chunk)
			return


func _on_connection_status_changed(status: String) -> void:
	match status:
		"disconnected", "reconnecting":
			modulate = Color(0.6, 0.6, 0.7, 1.0)
		"connected":
			modulate = Color(1.0, 1.0, 1.0, 1.0)
			RuneFilter.reset_rate_limits()


# --- Agent Assignment ---

func assign_agent_to_edge(agent_id: String, floor_name: String, edge_index: int, character_class: String = "apprentice", provider: String = "", power_level: float = 0.0) -> void:
	_agent_assignments[agent_id] = {"floor": floor_name, "edge": edge_index}
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			floor_node.add_agent_slot(agent_id, edge_index, character_class, provider, power_level)
			if floor_node.get_floor_state() == floor_node.FloorState.LINGERING:
				floor_node.reactivate()
			_recompute_floor_load(floor_name)
			floors_changed.emit()
			return


## Agents per edge before spilling onto the next edge. Keeps new spawns on
## the currently visible edge so they appear on screen immediately.
const PREFERRED_EDGE_CAPACITY: int = 5


# --- T15 (#124) — composite_load aggregation ---
#
# HONEST-MINIMAL, per the T14 precedent: composite_load = w_active *
# active_agents_norm + w_thru * task_throughput_norm, with token_cost_norm
# dropped (no data source anywhere in the orchestrator — see
# FloorMorph's doc-comment) and the remaining weights renormalized from the
# spec's 0.4/0.3/0.3 to 0.4/(0.4+0.3) and 0.3/(0.4+0.3).
const _LOAD_WEIGHT_ACTIVE: float = 0.4 / 0.7
const _LOAD_WEIGHT_THROUGHPUT: float = 0.3 / 0.7


## Appends a completion timestamp to floor_name's rolling ring and prunes
## anything older than the configured throughput window.
func _record_floor_completion(floor_name: String) -> void:
	var now: float = Time.get_unix_time_from_system()
	var ring: Array = _floor_completion_rings.get(floor_name, [])
	ring.append(now)
	_floor_completion_rings[floor_name] = _prune_completion_ring(ring, now)


func _prune_completion_ring(ring: Array, now: float) -> Array:
	var window: float = _config.throughput_window_sec
	return ring.filter(func(ts: float) -> bool: return now - ts <= window)


## T15 (#124) council finding — composite_load is otherwise only recomputed
## from _recompute_floor_load() calls triggered by agent register/state-change
## /deregister events, so a floor's throughput_norm (and therefore its
## polygon side count) never decays on its own once activity stops: the ring
## just sits there, still non-empty, with nothing left to prune it. This
## periodic sweep re-prunes and recomputes only floors with a non-empty ring
## — idle floors that have already fully decayed (empty ring) are skipped so
## this stays cheap even with many floors.
func _on_load_recompute_timer_timeout() -> void:
	for floor_name: String in _floor_completion_rings.keys():
		var ring: Array = _floor_completion_rings.get(floor_name, [])
		if not ring.is_empty():
			_recompute_floor_load(floor_name)


## Recomputes and pushes composite_load for a single floor from data that is
## actually available client-side today (see the honest-minimal note above).
## Only the named floor is touched — cheap enough to call on every agent
## register/state-change/deregister/reassign without batching.
func _recompute_floor_load(floor_name: String) -> void:
	var floor_node: Node2D = null
	for candidate: Node2D in _floors:
		if candidate.get_meta("floor_name", "") == floor_name:
			floor_node = candidate
			break
	if floor_node == null or not floor_node.has_method("set_composite_load"):
		return
	var now: float = Time.get_unix_time_from_system()
	var ring: Array = _prune_completion_ring(_floor_completion_rings.get(floor_name, []), now)
	_floor_completion_rings[floor_name] = ring

	var active_count: int = floor_node.get_active_count() if floor_node.has_method("get_active_count") else 0
	var active_norm: float = clampf(float(active_count) / maxf(float(_config.load_capacity), 1.0), 0.0, 1.0)
	var throughput_norm: float = clampf(float(ring.size()) / maxf(float(_config.throughput_cap), 1.0), 0.0, 1.0)

	var composite_load: float = _LOAD_WEIGHT_ACTIVE * active_norm + _LOAD_WEIGHT_THROUGHPUT * throughput_norm
	floor_node.set_composite_load(composite_load)


func _find_best_edge_for_agent(floor_name: String) -> int:
	var edge_counts: Dictionary = {}
	for i: int in range(_config.polygon_sides):
		edge_counts[i] = 0
	for existing_id: String in _agent_assignments:
		var assignment: Dictionary = _agent_assignments[existing_id]
		if assignment.get("floor", "") == floor_name:
			var e: int = assignment.get("edge", 0)
			edge_counts[e] = edge_counts.get(e, 0) + 1
	# Prefer the edge the player is looking at until it fills up.
	var active_edge: int = 0
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			active_edge = floor_node.get_active_edge()
			break
	if edge_counts.get(active_edge, 0) < PREFERRED_EDGE_CAPACITY:
		return active_edge
	var min_edge: int = active_edge
	var min_count: int = edge_counts.get(active_edge, 0)
	for e: int in edge_counts:
		if edge_counts[e] < min_count:
			min_count = edge_counts[e]
			min_edge = e
	return min_edge


## Return the live AgentCharacter node for an agent across all floors, or null
## if it isn't currently visible (non-active edge, or unknown agent).
func get_agent_character(agent_id: String) -> AgentCharacter:
	for floor_node: Node2D in _floors:
		if floor_node.has_method("get_agent_character"):
			var char_node: AgentCharacter = floor_node.get_agent_character(agent_id)
			if char_node != null:
				return char_node
	return null


## Grimoire Summoning (F3) — grows the floor stack with synthetic non-permanent
## floors ("floor-N") until _floors.size() >= target_count, so a drop on the
## "new floor" gap above the top always has a real floor node waiting for it.
func _ensure_floor_count(target_count: int) -> void:
	while _floors.size() < target_count:
		var index: int = _floors.size()
		var synthetic_name: String = "floor-%d" % index
		var floor_node: Node2D = _create_floor(synthetic_name, "Floor %d" % index, false)
		_floors.append(floor_node)
	_layout_floors()
	_apply_fisheye_layout()
	_update_tower_frame()
	_sync_tower_exterior()
	floors_changed.emit()


## Grimoire Summoning (F3) — converts a screen-space position (e.g. the
## mouse) into Tower-local world space, through the same camera the tower
## renders with, so placement-mode hit-testing never fights zoom or the
## panel-docking offset (see set_master_region/_aim_camera_at_region).
func world_position_from_screen(screen_pos: Vector2) -> Vector2:
	return _camera.get_screen_transform().affine_inverse() * screen_pos


## The inverse of world_position_from_screen — used to draw the placement-mode
## highlight overlay at the right screen Y for a given world-space Y.
func screen_position_from_world(world_pos: Vector2) -> Vector2:
	return _camera.get_screen_transform() * world_pos


## Floor geometry the flyout needs to draw its highlight overlay, without it
## having to know about _floor_spacing/_floor_height directly.
func get_floor_spacing() -> float:
	return _floor_spacing


func get_floor_height() -> float:
	return _floor_height


## Grimoire Summoning (F3) — classifies a world-space position against the
## floor stack (see FloorHitTest.classify) and enriches a FLOOR result with
## live desk-count/full-floor data so the flyout can show "floor N, x/4" and
## the reject state under the cursor. Floor 0 always reports RESERVED, even
## before any floor node occupies it.
func hit_test_floor(world_pos: Vector2) -> Dictionary:
	var result: Dictionary = FloorHitTest.classify(
		world_pos.y, maxi(_floors.size(), 1), _floor_spacing, _floor_height
	)
	if result["type"] == FloorHitTest.ZoneType.FLOOR:
		var index: int = result["index"]
		var floor_node: Node2D = _floors[index]
		result["agent_count"] = floor_node.get_agent_count() if floor_node.has_method("get_agent_count") else 0
		result["capacity"] = floor_node.get_desk_capacity() if floor_node.has_method("get_desk_capacity") else 4
		result["is_full"] = floor_node.is_floor_full() if floor_node.has_method("is_floor_full") else false
	return result


func _has_floor(floor_name: String) -> bool:
	for floor_node: Node2D in _floors:
		if floor_node.get_meta("floor_name", "") == floor_name:
			return true
	return false


## Panel docking calls this (see panel_manager.gd) with the screen rectangle the
## tower still owns. Floor geometry never changes — only the camera aims, so
## world x=0 keeps centering inside that region.
func set_master_region(region: Rect2) -> void:
	_master_region = region
	_aim_camera_at_region()


func _aim_camera_at_region() -> void:
	if _camera == null:
		return
	if _master_region.size == Vector2.ZERO:
		_camera.offset = Vector2.ZERO
		return
	var viewport_center: Vector2 = get_viewport_rect().size * 0.5
	var region_center: Vector2 = _master_region.position + (_master_region.size * 0.5)
	var zoom_x: float = maxf(_camera.zoom.x, 0.001)
	_camera.offset = Vector2((viewport_center.x - region_center.x) / zoom_x, 0.0)


func _on_viewport_size_changed() -> void:
	_apply_base_zoom()
	set_master_region(Rect2(Vector2.ZERO, get_viewport_rect().size))


## The world is measured in art pixels, so the metrics are constants. Kept as a
## function because several call sites still run it after structural changes.
func _recalculate_layout_metrics() -> void:
	_floor_width = BASE_FLOOR_WIDTH
	_floor_height = BASE_FLOOR_HEIGHT
	_floor_spacing = FLOOR_SPACING
	_tower_radius = BASE_TOWER_RADIUS


func _update_tower_frame() -> void:
	# The Tower root stays at world origin. Only the camera moves.
	_camera.position = Vector2(0.0, _focused_index * -_floor_spacing)
	_aim_camera_at_region()


func _sync_tower_exterior() -> void:
	_tower_exterior.configure(_config.polygon_sides, _floors.size() * _floor_spacing, _tower_radius)
