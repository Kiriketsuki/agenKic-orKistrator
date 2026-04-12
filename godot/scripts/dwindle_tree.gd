extends RefCounted
## DwindleTree — Hyprland-inspired master-dwindle BSP for one dock side.

class_name DwindleTree

const SPLIT_VERTICAL: String = "vertical"
const SPLIT_HORIZONTAL: String = "horizontal"

var side: String = ""
var root: DwindleNode = null
var _insertion_order: Array[String] = []


func _init(tree_side: String = "") -> void:
	side = tree_side


func is_empty() -> bool:
	return root == null


func panel_count() -> int:
	return _insertion_order.size()


func panel_ids() -> Array[String]:
	return _insertion_order.duplicate()


func insert_panel(panel: PanelBase) -> DwindleNode:
	var new_leaf: DwindleNode = DwindleNode.leaf(panel)
	if root == null:
		root = new_leaf
		_insertion_order = [panel.panel_id]
		return new_leaf
	var target_leaf: DwindleNode = _find_insertion_leaf()
	var branch: DwindleNode = DwindleNode.branch(_split_for_depth(target_leaf.depth()), 0.5, target_leaf, new_leaf)
	_replace_node(target_leaf, branch)
	_insertion_order.append(panel.panel_id)
	return new_leaf


func remove_panel(panel_id: String) -> PanelBase:
	var leaf: DwindleNode = find_leaf(panel_id)
	if leaf == null:
		return null
	var panel: PanelBase = leaf.panel
	_insertion_order.erase(panel_id)
	if leaf == root:
		root = null
		return panel
	var parent: DwindleNode = leaf.parent
	var sibling: DwindleNode = parent.sibling_of(leaf)
	_promote_sibling(parent, sibling)
	return panel


func find_leaf(panel_id: String) -> DwindleNode:
	if root == null:
		return null
	var leaves: Array[DwindleNode] = []
	root.collect_leaves(leaves)
	for leaf: DwindleNode in leaves:
		if leaf.panel_id == panel_id:
			return leaf
	return null


func layout(area: Rect2) -> Dictionary:
	var solved: Dictionary = {}
	if root == null:
		return solved
	_layout_node(root, area, solved)
	return solved


func serialize() -> Variant:
	if root == null:
		return null
	return root.serialize()


func restore(serialized: Variant, panels_by_id: Dictionary) -> void:
	root = _restore_node(serialized, panels_by_id)
	_insertion_order.clear()
	if root == null:
		return
	var leaves: Array[DwindleNode] = []
	root.collect_leaves(leaves)
	for leaf: DwindleNode in leaves:
		_insertion_order.append(leaf.panel_id)


func set_ratio_for_panel(panel_id: String, split_ratio: float) -> bool:
	var leaf: DwindleNode = find_leaf(panel_id)
	if leaf == null or leaf.parent == null:
		return false
	leaf.parent.ratio = clampf(split_ratio, 0.15, 0.85)
	return true


func _find_insertion_leaf() -> DwindleNode:
	if _insertion_order.is_empty():
		return root
	var last_id: String = _insertion_order[_insertion_order.size() - 1]
	var existing: DwindleNode = find_leaf(last_id)
	return existing if existing != null else _first_leaf()


func _first_leaf() -> DwindleNode:
	var current: DwindleNode = root
	while current != null and not current.is_leaf():
		current = current.first
	return current


func _replace_node(target: DwindleNode, replacement: DwindleNode) -> void:
	var parent: DwindleNode = target.parent
	replacement.parent = parent
	if parent == null:
		root = replacement
	else:
		if parent.first == target:
			parent.first = replacement
		else:
			parent.second = replacement
	target.parent = replacement


func _promote_sibling(branch: DwindleNode, sibling: DwindleNode) -> void:
	var grandparent: DwindleNode = branch.parent
	if grandparent == null:
		root = sibling
		sibling.parent = null
		return
	if grandparent.first == branch:
		grandparent.first = sibling
	else:
		grandparent.second = sibling
	sibling.parent = grandparent


func _split_for_depth(depth: int) -> String:
	return SPLIT_VERTICAL if depth % 2 == 0 else SPLIT_HORIZONTAL


func _layout_node(node: DwindleNode, area: Rect2, solved: Dictionary) -> void:
	if node == null:
		return
	if node.is_leaf():
		solved[node.panel_id] = area
		return
	var first_area: Rect2
	var second_area: Rect2
	if node.split == SPLIT_VERTICAL:
		var split_y: float = area.size.y * node.ratio
		first_area = Rect2(area.position, Vector2(area.size.x, split_y))
		second_area = Rect2(Vector2(area.position.x, area.position.y + split_y), Vector2(area.size.x, area.size.y - split_y))
	else:
		var split_x: float = area.size.x * node.ratio
		first_area = Rect2(area.position, Vector2(split_x, area.size.y))
		second_area = Rect2(Vector2(area.position.x + split_x, area.position.y), Vector2(area.size.x - split_x, area.size.y))
	_layout_node(node.first, first_area, solved)
	_layout_node(node.second, second_area, solved)


func _restore_node(serialized: Variant, panels_by_id: Dictionary) -> DwindleNode:
	if not serialized is Dictionary:
		return null
	var data: Dictionary = serialized as Dictionary
	var node_type: String = data.get("type", "")
	if node_type == DwindleNode.TYPE_LEAF:
		var panel_id: String = data.get("panel_id", "")
		if panel_id.is_empty() or not panels_by_id.has(panel_id):
			return null
		var panel: PanelBase = panels_by_id[panel_id]
		return DwindleNode.leaf(panel)
	if node_type != DwindleNode.TYPE_BRANCH:
		return null
	var children: Array = data.get("children", [])
	if children.size() != 2:
		return null
	var first_child: DwindleNode = _restore_node(children[0], panels_by_id)
	var second_child: DwindleNode = _restore_node(children[1], panels_by_id)
	if first_child == null or second_child == null:
		return null
	return DwindleNode.branch(data.get("split", SPLIT_VERTICAL), data.get("ratio", 0.5), first_child, second_child)
