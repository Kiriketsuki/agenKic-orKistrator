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

var _sides: int = 6
var _face_w: float = 140.0
var _face_h: float = 40.0
var _rot: float = 0.0
var _wall_tex: Texture2D = null
var _tint: Color = Color.WHITE


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


## Brightness multiplier for a face. The face that looks at the camera keeps
## full value. A face at 90 degrees falls to SIDE_DIM.
static func face_dim(sides: int, rot: float, index: int) -> float:
	var a: float = absf(face_angle(sides, rot, index))
	return lerpf(1.0, SIDE_DIM, clampf(a / (PI / 2.0), 0.0, 1.0))


func _draw() -> void:
	if _sides < 3 or _face_w <= 0.0 or _face_h <= 0.0:
		return
	var step: float = TAU / float(_sides)
	var apothem: float = apothem_for(_sides, _face_w)
	# Radius of the corner ring. A corner sits half a step off the face middle.
	var corner_r: float = apothem / cos(step / 2.0)
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
		var mid: float = face_angle(_sides, _rot, k)
		var p0: Vector2 = project_ring_point(_sides, _face_w, mid - step / 2.0, corner_r)
		var p1: Vector2 = project_ring_point(_sides, _face_w, mid + step / 2.0, corner_r)
		var quad := PackedVector2Array([
			Vector2(p0.x, -half_h * p0.y), Vector2(p1.x, -half_h * p1.y),
			Vector2(p1.x, half_h * p1.y), Vector2(p0.x, half_h * p0.y),
		])
		var dim: float = face_dim(_sides, _rot, k)
		var color := Color(_tint.r * dim, _tint.g * dim, _tint.b * dim, 1.0)
		draw_colored_polygon(quad, color, uvs, _wall_tex)
