class_name EdgeLayout
## Calculates desk positions for agents along a floor edge.

const DESK_WIDTH: float = 16.0
const DESK_HEIGHT: float = 20.0
const DESK_SPACING: float = 4.0


## Returns an array of Vector2 positions for agent desks along the edge.
static func calculate_positions(agent_count: int, edge_width: float) -> Array[Vector2]:
	var positions: Array[Vector2] = []
	if agent_count == 0:
		return positions
	var total_width: float = agent_count * (DESK_WIDTH + DESK_SPACING) - DESK_SPACING
	var start_x: float = -total_width / 2.0
	if total_width > edge_width:
		var clamped_spacing: float = maxf((edge_width - agent_count * DESK_WIDTH) / maxf(agent_count - 1, 1), 1.0)
		total_width = agent_count * DESK_WIDTH + (agent_count - 1) * clamped_spacing
		start_x = -total_width / 2.0
		for i: int in range(agent_count):
			positions.append(Vector2(start_x + i * (DESK_WIDTH + clamped_spacing), -DESK_HEIGHT / 2.0))
		return positions
	for i: int in range(agent_count):
		positions.append(Vector2(start_x + i * (DESK_WIDTH + DESK_SPACING), -DESK_HEIGHT / 2.0))
	return positions


## Returns the edge width for a regular polygon given polygon_sides and floor width.
static func edge_width_for_polygon(polygon_sides: int, floor_width: float) -> float:
	if polygon_sides <= 2:
		return floor_width
	var r: float = floor_width / 2.0
	return 2.0 * r * sin(PI / polygon_sides)
