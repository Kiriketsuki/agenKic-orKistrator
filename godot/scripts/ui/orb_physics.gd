class_name OrbPhysics
## F2 (#175) — Pure, node-free math for the orb flock HUD: spring-to-edge
## docking, momentum with edge ricochet, nearest-edge selection, and dock
## stack offsets. Mirrors the T15-T18 idiom (FloorMorph, PaletteMath,
## ParticleMath, PanelFloatMath): every function here is static and takes
## only plain values, so tests/orb_physics_test.gd exercises it headlessly
## without a running engine. No Node, Control, Tween, or Viewport reference
## anywhere in this file. OrbFlock owns the per-frame call sites and the
## Vector2 state these functions read and return.

## Screen edges the flock can dock against. TOP and BOTTOM only matter for
## momentum ricochet (acceptance: "a top-edge bounce"); the dock itself
## always resolves to LEFT or RIGHT, matching the mockup's side-rail dock.
enum Edge { LEFT, RIGHT, TOP, BOTTOM }

## Spring stiffness/damping for the release-to-edge snap. Critically damped
## (damping^2 ~= 4*stiffness) so the flock settles without overshoot.
const SPRING_STIFFNESS: float = 220.0
const SPRING_DAMPING: float = 28.0
const SPRING_SETTLE_DISTANCE: float = 0.5
const SPRING_SETTLE_SPEED: float = 4.0

## Fraction of speed kept after a top/bottom bounce during a flick. 1.0
## keeps all speed (perfectly elastic), 0.0 stops the flock dead at the
## edge.
const DEFAULT_RICOCHET_FRACTION: float = 0.62

## Momentum decays over time so a flick eventually settles instead of
## bouncing forever. Expressed as speed lost per second, multiplicative.
## Raised sharply (keeper feedback, item 4b) from the original 0.35/6.0 so a
## flick bleeds off and hands control to the spring-to-edge phase in about a
## second, instead of drifting freely around the screen.
const MOMENTUM_DRAG_PER_SEC: float = 4.0
const MOMENTUM_STOP_SPEED: float = 60.0

## Dock stack: each trailing orb sits this many pixels behind the lead orb,
## both toward screen center and slightly down, so the lead orb reads on
## top of the stack.
const STACK_STEP_INWARD_PX: float = 10.0
const STACK_STEP_DOWN_PX: float = 14.0

## Margin between an orb's edge and the viewport edge while docked.
const DOCK_MARGIN_PX: float = 18.0


## Picks the closest screen edge to `position` (an orb center) given the
## viewport size. Ties resolve toward LEFT/RIGHT over TOP/BOTTOM, matching
## the dock's side-rail behavior: a flock released near a corner docks to
## the side, not the top or bottom.
static func nearest_edge(position: Vector2, viewport_size: Vector2) -> Edge:
	var dist_left: float = position.x
	var dist_right: float = viewport_size.x - position.x
	var dist_top: float = position.y
	var dist_bottom: float = viewport_size.y - position.y
	var side_min: float = minf(dist_left, dist_right)
	var vertical_min: float = minf(dist_top, dist_bottom)
	if side_min <= vertical_min:
		return Edge.LEFT if dist_left <= dist_right else Edge.RIGHT
	return Edge.TOP if dist_top <= dist_bottom else Edge.BOTTOM


## The dock anchor point on `edge` for an orb of `radius`, at the given
## `height` along the edge (the release height, per the acceptance
## scenario "the orbs stack at the release height"). Clamps the height so
## the whole stack stays on-screen.
static func edge_dock_anchor(edge: Edge, viewport_size: Vector2, radius: float, height: float) -> Vector2:
	var clamped_height: float = clampf(height, radius + DOCK_MARGIN_PX, viewport_size.y - radius - DOCK_MARGIN_PX)
	match edge:
		Edge.RIGHT:
			return Vector2(viewport_size.x - radius - DOCK_MARGIN_PX, clamped_height)
		Edge.TOP:
			return Vector2(clampf(height, radius + DOCK_MARGIN_PX, viewport_size.x - radius - DOCK_MARGIN_PX), radius + DOCK_MARGIN_PX)
		Edge.BOTTOM:
			return Vector2(clampf(height, radius + DOCK_MARGIN_PX, viewport_size.x - radius - DOCK_MARGIN_PX), viewport_size.y - radius - DOCK_MARGIN_PX)
		_:
			return Vector2(radius + DOCK_MARGIN_PX, clamped_height)


## Picks the flock's dock edge for a release at `position` (keeper feedback,
## item 4a): LEFT, RIGHT, or BOTTOM only — never TOP, so a flick toward the
## top of the screen still docks somewhere reachable. BOTTOM is the default
## unless the nearer of LEFT/RIGHT sits under half the distance to the
## bottom edge, matching the keeper's rule that a release has to land
## clearly closer to a side before the flock cedes the bottom-center home.
static func choose_dock_edge(position: Vector2, viewport_size: Vector2) -> Edge:
	var dist_left: float = position.x
	var dist_right: float = viewport_size.x - position.x
	var dist_bottom: float = viewport_size.y - position.y
	var side_min: float = minf(dist_left, dist_right)
	if side_min < dist_bottom * 0.5:
		return Edge.LEFT if dist_left <= dist_right else Edge.RIGHT
	return Edge.BOTTOM


## The along-edge release coordinate to pass into edge_dock_anchor() for
## `edge`: the release y for a side dock (LEFT/RIGHT), the release x for a
## bottom dock. Pure companion to choose_dock_edge() so OrbFlock never has
## to duplicate the "which axis matters for this edge" branch itself.
static func dock_release_coordinate(edge: Edge, position: Vector2) -> float:
	return position.y if (edge == Edge.LEFT or edge == Edge.RIGHT) else position.x


## The flock's resting dock: centered horizontally, resting on the bottom
## edge with the standard margin. The keeper's rule: released orbs
## gravitate to the middle bottom of the screen.
static func bottom_center_anchor(viewport_size: Vector2, radius: float) -> Vector2:
	return Vector2(viewport_size.x * 0.5, viewport_size.y - radius - DOCK_MARGIN_PX)


## One spring-damper integration step moving `current` toward `target`.
## Returns {"position": Vector2, "velocity": Vector2, "settled": bool}.
## `settled` is true once the orb is within SPRING_SETTLE_DISTANCE of the
## target and moving slower than SPRING_SETTLE_SPEED, so the caller can
## snap exactly onto the dock and stop ticking the spring.
static func spring_step(current: Vector2, target: Vector2, velocity: Vector2, delta: float) -> Dictionary:
	var displacement: Vector2 = target - current
	var accel: Vector2 = (displacement * SPRING_STIFFNESS) - (velocity * SPRING_DAMPING)
	var next_velocity: Vector2 = velocity + accel * delta
	var next_position: Vector2 = current + next_velocity * delta
	var settled: bool = displacement.length() <= SPRING_SETTLE_DISTANCE and next_velocity.length() <= SPRING_SETTLE_SPEED
	if settled:
		return {"position": target, "velocity": Vector2.ZERO, "settled": true}
	return {"position": next_position, "velocity": next_velocity, "settled": false}


## Flick velocity from a short window of recent drag positions. Callers
## sample the pointer every frame during a drag and feed the last couple of
## samples in on release; this stays a pure function of two positions and a
## time delta so it needs no history buffer of its own. `max_speed` caps
## the result so a stray zero-delta sample cannot produce an infinite
## velocity.
static func flick_velocity(previous_position: Vector2, released_position: Vector2, delta: float, max_speed: float) -> Vector2:
	if delta <= 0.0:
		return Vector2.ZERO
	var raw: Vector2 = (released_position - previous_position) / delta
	if raw.length() > max_speed:
		raw = raw.normalized() * max_speed
	return raw


## One momentum integration step: moves `position` by `velocity * delta`,
## bounces off the top and bottom viewport edges by reflecting the vertical
## velocity component and keeping `ricochet_fraction` of its speed, then
## applies exponential drag. Left/right edges are not bounced here — the
## flock docks sideways instead of ricocheting off the side rails, per the
## acceptance scenario's "top-edge bounce".
## Returns {"position": Vector2, "velocity": Vector2}.
static func momentum_step(position: Vector2, velocity: Vector2, delta: float, viewport_size: Vector2, radius: float, ricochet_fraction: float) -> Dictionary:
	var next_position: Vector2 = position + velocity * delta
	var next_velocity: Vector2 = velocity
	var top_bound: float = radius
	var bottom_bound: float = viewport_size.y - radius
	if next_position.y < top_bound:
		next_position.y = top_bound
		next_velocity.y = absf(next_velocity.y) * ricochet_fraction
	elif next_position.y > bottom_bound:
		next_position.y = bottom_bound
		next_velocity.y = -absf(next_velocity.y) * ricochet_fraction
	next_position.x = clampf(next_position.x, radius, viewport_size.x - radius)
	var drag: float = clampf(1.0 - MOMENTUM_DRAG_PER_SEC * delta, 0.0, 1.0)
	next_velocity *= drag
	if next_velocity.length() < MOMENTUM_STOP_SPEED:
		next_velocity = Vector2.ZERO
	return {"position": next_position, "velocity": next_velocity}


## True once a flying flock has bled off enough speed to hand control back
## to the spring-to-edge phase.
static func momentum_settled(velocity: Vector2) -> bool:
	return velocity.length() <= 0.0


## Continuous glide: stiffness and damping for the always-on edge pull that
## replaced the two-phase momentum-then-spring flight (keeper feedback,
## round 6). The ratio is underdamped (7 / (2 * sqrt(90)) ~= 0.37) so a
## flick keeps visible momentum and lands with a small physical bounce
## instead of stopping dead mid-air before the pull starts. The ratio
## (4.7 / (2 * sqrt(40)) ~= 0.37) is preserved from the 90/7 original;
## only the pull strength was reduced — gravity felt too strong in play.
const GLIDE_STIFFNESS: float = 40.0
const GLIDE_DAMPING: float = 4.7
const GLIDE_SETTLE_DISTANCE: float = 1.0
const GLIDE_SETTLE_SPEED: float = 8.0


## One always-on gravity step. Re-picks the dock edge from the current
## position, pulls toward that edge's anchor while the carried velocity
## keeps moving the flock along it, ricochets off the top bound, and clamps
## the rest. The along-edge anchor coordinate tracks the flock, so the pull
## acts mostly perpendicular to the edge and momentum along the edge glides
## until the damping bleeds it off. Runs from the instant of release: there
## is no separate momentum phase and no wait before gravity engages.
## Returns {"position", "velocity", "edge", "settled"}.
static func glide_step(position: Vector2, velocity: Vector2, delta: float, viewport_size: Vector2, radius: float, ricochet_fraction: float) -> Dictionary:
	var edge: Edge = choose_dock_edge(position, viewport_size)
	var along: float = dock_release_coordinate(edge, position)
	var anchor: Vector2 = edge_dock_anchor(edge, viewport_size, radius, along)
	var displacement: Vector2 = anchor - position
	var accel: Vector2 = (displacement * GLIDE_STIFFNESS) - (velocity * GLIDE_DAMPING)
	var next_velocity: Vector2 = velocity + accel * delta
	var next_position: Vector2 = position + next_velocity * delta
	if next_position.y < radius:
		next_position.y = radius
		next_velocity.y = absf(next_velocity.y) * ricochet_fraction
	next_position.x = clampf(next_position.x, radius, viewport_size.x - radius)
	next_position.y = minf(next_position.y, viewport_size.y - radius)
	if displacement.length() <= GLIDE_SETTLE_DISTANCE and next_velocity.length() <= GLIDE_SETTLE_SPEED:
		return {"position": anchor, "velocity": Vector2.ZERO, "edge": edge, "settled": true}
	return {"position": next_position, "velocity": next_velocity, "edge": edge, "settled": false}


## Offset of the orb at `index` (0 = lead orb, drawn on top) within a
## docked stack, relative to the lead orb's anchor position. Trailing orbs
## step inward (away from the edge) and down, so the stack reads as a
## fanned deck with the lead orb fully visible on top.
static func stack_offset(index: int, edge: Edge) -> Vector2:
	if index <= 0:
		return Vector2.ZERO
	var inward: float = STACK_STEP_INWARD_PX * float(index)
	var down: float = STACK_STEP_DOWN_PX * float(index)
	match edge:
		Edge.RIGHT:
			return Vector2(-inward, down)
		Edge.TOP:
			return Vector2(down, inward)
		Edge.BOTTOM:
			return Vector2(down, -inward)
		_:
			return Vector2(inward, down)
