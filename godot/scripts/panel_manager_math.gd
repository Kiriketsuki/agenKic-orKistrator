class_name PanelManagerMath
extends RefCounted
## PanelManagerMath — pure arrangement-filtering math for PanelManager,
## extracted out of panel_manager.gd so it carries no autoload dependency.
## panel_manager.gd sits in the OrbFlock -> GrimoireFlyout -> BridgeManager
## preload chain (grimoire_flyout.gd touches the BridgeManager autoload at
## parse time), so a headless test that preloads panel_manager.gd to reach
## this math fails to compile outside a full Godot boot. This script has no
## such dependency, so tests/layout_presets_test.gd preloads it directly
## instead. See tests/title_screen_focus_test.gd's doc-comment for the full
## explanation of why the preload chain fails headless.


## Drops any arrangement entry whose agent_id names an agent absent from
## `known_agent_ids`. An empty agent_id (the quest board, say) always
## survives, since it names no agent. PanelManager.filter_arrangement_for_agents
## delegates here so both the production path and the headless test share
## one implementation.
static func filter_arrangement_for_agents(arrangement: Dictionary, known_agent_ids: Array) -> Dictionary:
	var kept: Array[Dictionary] = []
	for entry: Variant in arrangement.get("panels", []):
		if not (entry is Dictionary):
			continue
		var agent_id: String = (entry as Dictionary).get("agent_id", "")
		if agent_id.is_empty() or known_agent_ids.has(agent_id):
			kept.append(entry as Dictionary)
	return {"panels": kept}


## Computes the floating rect for the `index`-th (0-based) simultaneously
## open scroll panel (#169, up to SigilConfig.MAX_SCROLL_PANELS), so multiple
## panels cascade toward the upper-left instead of stacking exactly on top of
## one another. index 0 reproduces the original single-scroll placement
## exactly (full height, right-anchored at `panel_width`); each subsequent
## index shifts the panel left by `cascade_offset.x` and down by
## `cascade_offset.y`, shrinking its height to stay within the viewport.
## Negative `index` is clamped to 0 so a caller with a stale index (e.g. a
## panel not yet found in its own focus-order list) degrades to the base
## placement rather than producing a rect off-viewport.
## `panel_height` is the keeper's configured open height (SigilConfig's
## scroll_height_ratio applied to the viewport). It is clamped to what is
## left below the cascade offset, so a full-height panel at cascade index 2
## still ends at the viewport bottom instead of overhanging it. Passing the
## full viewport height reproduces the original full-height placement.
static func tile_scroll_rect(
	viewport_size: Vector2, panel_width: float, index: int, cascade_offset: Vector2, panel_height: float = -1.0
) -> Rect2:
	var clamped_index: int = maxi(index, 0)
	var origin_y: float = cascade_offset.y * float(clamped_index)
	var origin_x: float = maxf(viewport_size.x - panel_width - cascade_offset.x * float(clamped_index), 0.0)
	var available_height: float = maxf(viewport_size.y - origin_y, 0.0)
	# A negative panel_height means "as tall as there is room for" — the
	# pre-configurable-height behavior, kept as the default so existing
	# callers and tests need no change.
	var height: float = available_height if panel_height < 0.0 else minf(panel_height, available_height)
	return Rect2(Vector2(origin_x, origin_y), Vector2(panel_width, height))
