class_name FloorPrism
extends Node2D
## Phase 6 section 1 — draws an N-face wall prism in fake perspective.
##
## The HTML demo builds the room from CSS 3D faces: 560x160 px faces pushed out
## by translateZ(485px) under a 1600px perspective, with the camera turning by
## one edge step per rotation. Divided by 4 for art px that becomes a 140x40
## face, an apothem of 121 and a focal length of 400. The face width equals
## EdgeLayout.edge_width_for_polygon(sides, floor_width), so the prism face IS
## the floor edge the desks already lay out along.
##
## This draws the same thing with one _draw() call and no 3D engine. Each face
## projects its two corner posts through a perspective divide, the back faces
## drop out, and the rest paint back to front so the near wall covers the far
## wall. The node itself carries no state machine. FloorScene owns the rotation
## tween and pushes a new rot value per frame.

## Demo perspective 1600 px, divided by 4 for art px.
const FOCAL: float = 400.0
## Dim factor at a 90 degree face angle. The demo uses opacity 0.45 for every
## face that is not the focused one.
const SIDE_DIM: float = 0.45

## Three-band anatomy, in art px, inside a face that is _face_h tall. Every face
## carries the same anatomy, so a turn sweeps fully dressed walls past the camera
## instead of moving one dressed panel. FloorScene mirrors these names.
const WALL_BAND_HEIGHT: float = 16.0
const PLANE_DEPTH: float = 20.0
const FRONT_LIP_HEIGHT: float = 3.0
const FRONT_LIP_COLOR: Color = Color(0.059, 0.067, 0.090, 1.0)
## 1 px step shadow at the wall and plane junction. It reads as a room corner.
const SEAM_COLOR: Color = Color(0.106, 0.118, 0.153, 1.0)
const SEAM_HEIGHT: float = 1.0
## The wall catches the torchlight, the ground recedes.
const WALL_VALUE: float = 0.82
const PLANE_VALUE: float = 0.78
## Per-face dressing bounds. Each face shows 1 to 3 windows, each jittered inside
## its cell, plus one ambient prop. The counts come from a hash of the floor name
## and the face index, so a face always looks the same and two faces rarely match.
const WINDOW_START: float = 24.0
const EDGE_WINDOW_MIN: int = 1
const EDGE_WINDOW_MAX: int = 3
const EDGE_WINDOW_JITTER: float = 9.0
## Half-width of the central door area. No window sits inside it.
const EDGE_WINDOW_CENTER_GAP: float = 28.0
## Ambient props come from a strip of 16 px cells.
const PROP_COUNT: int = 8
const PROP_CELL: float = 16.0

var _sides: int = 6
var _face_w: float = 140.0
var _face_h: float = 40.0
var _rot: float = 0.0
var _wall_tex: Texture2D = null
var _tint: Color = Color.WHITE
## Per-face dressing skin. FloorScene pushes these once per rebuild.
var _floor_tex: Texture2D = null
var _window_tex: Texture2D = null
var _prop_tex: Texture2D = null
## Hash seed for the per-face dressing. FloorScene passes its floor_name, so a
## floor keeps the same windows and props across every rebuild.
var _seed: String = ""


func _init() -> void:
	texture_repeat = CanvasItem.TEXTURE_REPEAT_ENABLED
	texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST


## Sets the static shape and skin. FloorScene calls this on every rebuild and
## on every morph frame, so it never allocates.
func configure(sides: int, face_w: float, face_h: float, wall_tex: Texture2D, tint: Color) -> void:
	_sides = maxi(sides, 3)
	_face_w = face_w
	_face_h = face_h
	_wall_tex = wall_tex
	_tint = tint
	queue_redraw()


## Sets the per-face dressing skin and the hash seed. FloorScene calls this from
## the same rebuild that calls configure().
func configure_dressing(seed_name: String, floor_tex: Texture2D, window_tex: Texture2D, prop_tex: Texture2D) -> void:
	_seed = seed_name
	_floor_tex = floor_tex
	_window_tex = window_tex
	_prop_tex = prop_tex
	queue_redraw()


## Sets the camera rotation in radians. One edge step per turn.
func set_rotation_value(value: float) -> void:
	_rot = value
	queue_redraw()


func get_rotation_value() -> float:
	return _rot


func get_sides() -> int:
	return _sides


func get_face_width() -> float:
	return _face_w


# --- Pure geometry, static so the headless tests can reach it ---


## Distance from the prism axis to the middle of a face.
static func apothem_for(sides: int, face_w: float) -> float:
	if sides < 3:
		return 0.0
	return face_w / (2.0 * tan(PI / float(sides)))


## Angle of face `index` relative to the camera. Zero means the face looks
## straight at the viewer.
static func face_angle(sides: int, rot: float, index: int) -> float:
	var step: float = TAU / float(maxi(sides, 3))
	return wrapf(float(index) * step - rot, -PI, PI)


## True while the face turns its painted side toward the camera.
static func face_is_front(sides: int, rot: float, index: int) -> bool:
	return cos(face_angle(sides, rot, index)) > 0.0


## Faces sorted back to front by the depth of their centre. Painting in this
## order lets the near wall cover the far wall without a depth buffer.
static func draw_order(sides: int, rot: float) -> Array[int]:
	var order: Array[int] = []
	for k: int in range(maxi(sides, 3)):
		order.append(k)
	order.sort_custom(func(a: int, b: int) -> bool:
		return cos(face_angle(sides, rot, a)) < cos(face_angle(sides, rot, b))
	)
	return order


## Projects one point on the prism ring. `angle` is measured from the camera
## axis. Returns the screen x and the perspective scale at that point.
static func project_ring_point(sides: int, face_w: float, angle: float, radius: float) -> Vector2:
	var apothem: float = apothem_for(sides, face_w)
	var depth: float = apothem - cos(angle) * radius
	var s: float = FOCAL / maxf(FOCAL + depth, 1.0)
	return Vector2(sin(angle) * radius * s, s)


## Screen x and perspective scale of the middle of face `index`. The ghost
## agents on the neighbour faces read their position from this.
static func face_center_projection(sides: int, face_w: float, rot: float, index: int) -> Vector2:
	var apothem: float = apothem_for(sides, face_w)
	return project_ring_point(sides, face_w, face_angle(sides, rot, index), apothem)


## Distance from the prism axis to a corner of the ring.
static func corner_radius_for(sides: int, face_w: float) -> float:
	var step: float = TAU / float(maxi(sides, 3))
	return apothem_for(sides, face_w) / cos(step / 2.0)


## A point on face `index`, in camera space, at face-local fraction `u`. Zero is
## the left corner and one is the right corner. Returns x and z, both unprojected.
## A face is flat, so the chord between its two corners is a straight line and any
## point on it is a plain lerp. That is what lets a window land on the same spot
## of the wall no matter how far the prism has turned.
static func face_point_3d(sides: int, face_w: float, rot: float, index: int, u: float) -> Vector2:
	var step: float = TAU / float(maxi(sides, 3))
	var apothem: float = apothem_for(sides, face_w)
	var corner_r: float = corner_radius_for(sides, face_w)
	var mid: float = face_angle(sides, rot, index)
	var a0: float = mid - step / 2.0
	var a1: float = mid + step / 2.0
	var p0 := Vector2(sin(a0) * corner_r, apothem - cos(a0) * corner_r)
	var p1 := Vector2(sin(a1) * corner_r, apothem - cos(a1) * corner_r)
	return p0.lerp(p1, u)


## Perspective scale at a camera-space depth.
static func scale_for_depth(depth: float) -> float:
	return FOCAL / maxf(FOCAL + depth, 1.0)


## Screen position of a face-local point. `u` runs corner to corner and `y_local`
## is the art-px height measured from the middle of the face.
static func face_screen_point(sides: int, face_w: float, rot: float, index: int, u: float, y_local: float) -> Vector2:
	var p: Vector2 = face_point_3d(sides, face_w, rot, index, u)
	var s: float = scale_for_depth(p.y)
	return Vector2(p.x * s, y_local * s)


# --- Deterministic per-face dressing, static so FloorScene and the tests agree ---


## The same seed, face index and salt always return the same value, so a face
## keeps its dressing across rebuilds and across a turn.
static func edge_hash(seed_name: String, index: int, salt: String) -> int:
	return absi(("%s|%d|%s" % [seed_name, index, salt]).hash())


## Deterministic float in [0, 1) for one face and one salt.
static func edge_unit(seed_name: String, index: int, salt: String) -> float:
	return float(edge_hash(seed_name, index, salt) % 10007) / 10007.0


## Window count on one face.
static func window_count(seed_name: String, index: int) -> int:
	return EDGE_WINDOW_MIN + edge_hash(seed_name, index, "count") \
		% (EDGE_WINDOW_MAX - EDGE_WINDOW_MIN + 1)


## Face-local x of every window on one face, in art px from the face middle.
static func window_positions(seed_name: String, index: int, face_w: float) -> Array[float]:
	var out: Array[float] = []
	var half_w: float = face_w / 2.0
	var span_left: float = -half_w + WINDOW_START
	var span_right: float = half_w - WINDOW_START
	var span: float = maxf(1.0, span_right - span_left)
	var count: int = window_count(seed_name, index)
	var cell: float = span / float(count)
	for i: int in range(count):
		var jitter: float = (edge_unit(seed_name, index, "jitter%d" % i) - 0.5) \
			* 2.0 * EDGE_WINDOW_JITTER
		var x: float = span_left + cell * (float(i) + 0.5) + jitter
		# Push the window clear of the central door area.
		if absf(x) < EDGE_WINDOW_CENTER_GAP:
			x = EDGE_WINDOW_CENTER_GAP * signf(x if x != 0.0 else 1.0)
		out.append(clampf(x, span_left, span_right))
	return out


## Atlas cell index of the ambient prop on one face.
static func prop_index(seed_name: String, index: int) -> int:
	return edge_hash(seed_name, index, "prop") % PROP_COUNT


## Face-local x of the ambient prop on one face.
static func prop_x(seed_name: String, index: int, face_w: float) -> float:
	var half_w: float = face_w / 2.0
	var side: float = -1.0 if edge_hash(seed_name, index, "side") % 2 == 0 else 1.0
	return side * lerpf(half_w * 0.45, half_w * 0.8, edge_unit(seed_name, index, "propx"))


## Brightness multiplier for a face. The face that looks at the camera keeps
## full value. A face at 90 degrees falls to SIDE_DIM.
static func face_dim(sides: int, rot: float, index: int) -> float:
	var a: float = absf(face_angle(sides, rot, index))
	return lerpf(1.0, SIDE_DIM, clampf(a / (PI / 2.0), 0.0, 1.0))


## Screen quad for a face-local rect. `u0` and `u1` run corner to corner, `y0`
## and `y1` are art px from the middle of the face.
func _face_quad(index: int, u0: float, u1: float, y0: float, y1: float) -> PackedVector2Array:
	return PackedVector2Array([
		face_screen_point(_sides, _face_w, _rot, index, u0, y0),
		face_screen_point(_sides, _face_w, _rot, index, u1, y0),
		face_screen_point(_sides, _face_w, _rot, index, u1, y1),
		face_screen_point(_sides, _face_w, _rot, index, u0, y1),
	])


## Applies a face dim to a fill colour without touching its alpha.
static func _dimmed(color: Color, dim: float) -> Color:
	return Color(color.r * dim, color.g * dim, color.b * dim, color.a)


## Draws one sprite flat against a face. `cx` is the face-local x of the sprite
## middle, in art px, and `cy` is the same for y. The two vertical edges project
## separately, so the sprite shears with the wall it sits on.
func _draw_face_sprite(index: int, cx: float, cy: float, w: float, h: float, tex: Texture2D, uvs: PackedVector2Array, dim: float) -> void:
	if tex == null:
		return
	var half_w: float = _face_w / 2.0
	var u0: float = (cx - w / 2.0 + half_w) / _face_w
	var u1: float = (cx + w / 2.0 + half_w) / _face_w
	var quad: PackedVector2Array = _face_quad(index, u0, u1, cy - h / 2.0, cy + h / 2.0)
	draw_colored_polygon(quad, Color(dim, dim, dim, 1.0), uvs, tex)


## Paints the bands, the windows and the ambient prop of one face. Every face
## gets the same treatment, so a turn reveals a room that is already dressed.
func _draw_face_dressing(index: int, dim: float) -> void:
	var half_h: float = _face_h / 2.0
	var wall_line: float = -half_h + WALL_BAND_HEIGHT
	var lip_top: float = half_h - FRONT_LIP_HEIGHT

	# Band B. The walkable plane, from the wall line down to the lip.
	if _floor_tex != null:
		var ft: Vector2 = _floor_tex.get_size()
		var uvx: float = _face_w / maxf(ft.x, 1.0)
		var uvy: float = (lip_top - wall_line) / maxf(ft.y, 1.0)
		var plane_uvs := PackedVector2Array([
			Vector2(0.0, 0.0), Vector2(uvx, 0.0), Vector2(uvx, uvy), Vector2(0.0, uvy),
		])
		var plane_color := Color(
			_tint.r * PLANE_VALUE * dim, _tint.g * PLANE_VALUE * dim,
			_tint.b * PLANE_VALUE * dim, 1.0
		)
		draw_colored_polygon(_face_quad(index, 0.0, 1.0, wall_line, lip_top), plane_color, plane_uvs, _floor_tex)

	# 1 px step shadow where the wall meets the plane.
	draw_colored_polygon(
		_face_quad(index, 0.0, 1.0, wall_line, wall_line + SEAM_HEIGHT),
		_dimmed(SEAM_COLOR, dim)
	)
	# Band C. Front lip, a plinth shadow at the face bottom.
	draw_colored_polygon(
		_face_quad(index, 0.0, 1.0, lip_top, half_h),
		_dimmed(FRONT_LIP_COLOR, dim)
	)

	var unit_uvs := PackedVector2Array([
		Vector2(0.0, 0.0), Vector2(1.0, 0.0), Vector2(1.0, 1.0), Vector2(0.0, 1.0),
	])
	if _window_tex != null:
		var wy: float = -half_h + WALL_BAND_HEIGHT / 2.0
		var ww: float = _window_tex.get_width()
		var wh: float = _window_tex.get_height()
		for wx: float in window_positions(_seed, index, _face_w):
			_draw_face_sprite(index, wx, wy, ww, wh, _window_tex, unit_uvs, dim)

	if _prop_tex != null:
		# One cell out of the props strip, addressed by UV so no AtlasTexture is
		# needed. draw_colored_polygon takes a plain texture and its own UVs.
		var aw: float = maxf(_prop_tex.get_width(), 1.0)
		var ah: float = maxf(_prop_tex.get_height(), 1.0)
		var cell: float = PROP_CELL / aw
		var left: float = float(prop_index(_seed, index)) * cell
		var prop_uvs := PackedVector2Array([
			Vector2(left, 0.0), Vector2(left + cell, 0.0),
			Vector2(left + cell, PROP_CELL / ah), Vector2(left, PROP_CELL / ah),
		])
		# The prop stands at the back of the plane, against the wall line.
		var py: float = -half_h + WALL_BAND_HEIGHT + 2.0
		_draw_face_sprite(
			index, prop_x(_seed, index, _face_w), py,
			PROP_CELL, PROP_CELL, _prop_tex, prop_uvs, dim
		)


func _draw() -> void:
	if _sides < 3 or _face_w <= 0.0 or _face_h <= 0.0:
		return
	var tex_size: Vector2 = Vector2(16.0, 16.0)
	if _wall_tex != null:
		tex_size = _wall_tex.get_size()
	var uv_scale: Vector2 = Vector2(_face_w / maxf(tex_size.x, 1.0), _face_h / maxf(tex_size.y, 1.0))
	var uvs := PackedVector2Array([
		Vector2(0.0, 0.0), Vector2(uv_scale.x, 0.0),
		Vector2(uv_scale.x, uv_scale.y), Vector2(0.0, uv_scale.y),
	])
	var half_h: float = _face_h / 2.0
	for k: int in draw_order(_sides, _rot):
		if not face_is_front(_sides, _rot, k):
			continue
		var dim: float = face_dim(_sides, _rot, k)
		# Band A. The wall covers the whole face and the plane paints over its
		# lower part, so the brick still shows behind a window.
		var color := Color(
			_tint.r * WALL_VALUE * dim, _tint.g * WALL_VALUE * dim,
			_tint.b * WALL_VALUE * dim, 1.0
		)
		draw_colored_polygon(_face_quad(k, 0.0, 1.0, -half_h, half_h), color, uvs, _wall_tex)
		_draw_face_dressing(k, dim)
