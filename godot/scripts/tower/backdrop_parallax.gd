extends Node2D
## BackdropParallax — the layered mountain sky behind the tower, matching the
## HTML demo: sky, drifting clouds, peaks, canopy, plus a moon.
##
## This lives inside the tower world (not on a CanvasLayer) so the camera zoom
## magnifies it exactly like every other art pixel. Parallax2D proved unreliable
## here (children never rendered under our camera setup), so each layer is a
## plain Node2D that this script repositions from the camera every frame:
## layer.position = camera_position * (1 - scroll_scale). scroll_scale 0 pins
## the layer to the screen, 1 keeps it fixed in the world.

const SKY_TEXTURE: Texture2D = preload("res://assets/backdrops/backdrop_sky.png")
const CLOUDS_TEXTURE: Texture2D = preload("res://assets/backdrops/backdrop_clouds.png")
const PEAKS_TEXTURE: Texture2D = preload("res://assets/backdrops/backdrop_peaks.png")
const CANOPY_TEXTURE: Texture2D = preload("res://assets/backdrops/backdrop_canopy.png")

## The art is 1x and the world is 1x. One backdrop tile is exactly the demo's
## 320x180 canvas, so the repeat width is 320.
const REPEAT_X: float = 320.0
## Horizontal copies per layer, centered: -1, 0, +1.
const REPEAT_COPIES: int = 3
## Slow horizontal cloud loop, matching the demo's drift keyframe.
const CLOUD_DRIFT_PX_PER_SEC: float = 2.0

## Phase 6 section 4. The canopy tile is 320x48. Its first tree tips start 10 px
## down and it turns solid 34 px down. The bottom of the tower sits at y = 28
## (plinth bottom). Centring the tile at y = 18 puts the tips at y = 4 and the
## solid mass at y = 28, so the treeline overlaps the tower base by 24 px.
const CANOPY_CENTER_Y: float = 18.0
## Draws the canopy in front of the shaft, the plinth and the floors.
const CANOPY_Z_INDEX: int = 5
## The canopy tile's own bottom row colour, extended down off screen.
const CANOPY_FLOOR_COLOR: Color = Color8(13, 28, 21)


## Entries: {node, scroll_scale (Vector2)}.
var _layers: Array[Dictionary] = []
var _clouds: Node2D = null
var _cloud_drift: float = 0.0


func _ready() -> void:
	var sky: Node2D = _make_layer("SkyLayer", Vector2(0.0, 0.02))
	# Fills above and below the 180px sky band, so a non-16:9 viewport never
	# shows the clear color. Colors sampled from the sky art's top edge and
	# the canopy's forest dark.
	_add_fill(sky, Color("#2a2545"), -2000.0, -88.0)
	_add_fill(sky, Color("#0e1a14"), 88.0, 2000.0)
	_add_sprite(sky, SKY_TEXTURE, Vector2.ZERO)
	# No generated moon: backdrop_sky.png carries the demo's moon already.

	_clouds = _make_layer("CloudsLayer", Vector2(0.1, 0.05))
	_add_sprite(_clouds, CLOUDS_TEXTURE, Vector2(0.0, -40.0))

	var peaks: Node2D = _make_layer("PeaksLayer", Vector2(0.25, 0.12))
	_add_sprite(peaks, PEAKS_TEXTURE, Vector2(0.0, 60.0))

	# Phase 6 section 4. The canopy is the only foreground layer, so it draws in
	# front of the tower and swallows the base. Its vertical scroll sits near 1.0
	# to keep it locked to the tower base. A screen-locked treeline would ride up
	# the tower and cover a mid stack floor when the camera pans to it.
	var canopy: Node2D = _make_layer("CanopyLayer", Vector2(0.45, 0.9))
	canopy.z_index = CANOPY_Z_INDEX
	# Forest floor under the canopy tile, so no sky shows below the treeline.
	_add_fill(canopy, CANOPY_FLOOR_COLOR, CANOPY_CENTER_Y + 23.0, 2000.0)
	_add_sprite(canopy, CANOPY_TEXTURE, Vector2(0.0, CANOPY_CENTER_Y))


func _process(delta: float) -> void:
	var cam: Camera2D = get_viewport().get_camera_2d()
	if cam == null:
		return
	var cam_pos: Vector2 = cam.get_screen_center_position()
	_cloud_drift = fmod(_cloud_drift + CLOUD_DRIFT_PX_PER_SEC * delta, REPEAT_X)
	for entry: Dictionary in _layers:
		var node: Node2D = entry["node"]
		var scroll: Vector2 = entry["scroll_scale"]
		var pos: Vector2 = Vector2(
			cam_pos.x * (1.0 - scroll.x),
			cam_pos.y * (1.0 - scroll.y)
		)
		if node == _clouds:
			pos.x -= _cloud_drift
		# Wrap horizontally so the 3 tiled copies always straddle the camera.
		pos.x -= REPEAT_X * roundf((pos.x - cam_pos.x) / REPEAT_X)
		node.position = pos


func _make_layer(layer_name: String, scroll_scale: Vector2) -> Node2D:
	var layer := Node2D.new()
	layer.name = layer_name
	add_child(layer)
	_layers.append({"node": layer, "scroll_scale": scroll_scale})
	return layer


func _add_sprite(parent: Node2D, texture: Texture2D, pos: Vector2) -> void:
	# Three horizontal copies so parallax and cloud drift never expose a seam.
	for i: int in range(REPEAT_COPIES):
		var sprite := Sprite2D.new()
		sprite.texture = texture
		sprite.texture_filter = CanvasItem.TEXTURE_FILTER_NEAREST
		sprite.position = pos + Vector2((i - REPEAT_COPIES / 2) * REPEAT_X, 0.0)
		parent.add_child(sprite)


## Solid horizontal band from y_top to y_bottom, wide enough for any parallax
## drift. Drawn before the sprites in the same layer, so it sits behind them.
func _add_fill(parent: Node2D, color: Color, y_top: float, y_bottom: float) -> void:
	var rect := ColorRect.new()
	rect.color = color
	rect.position = Vector2(-2000.0, y_top)
	rect.size = Vector2(4000.0, y_bottom - y_top)
	parent.add_child(rect)
