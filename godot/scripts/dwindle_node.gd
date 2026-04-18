extends RefCounted
## DwindleNode — branch/leaf node for a BSP dwindle tree.

class_name DwindleNode

const TYPE_LEAF: String = "leaf"
const TYPE_BRANCH: String = "branch"

var node_type: String = TYPE_LEAF
var split: String = "vertical"
var ratio: float = 0.5
var parent: DwindleNode = null
var first: DwindleNode = null
var second: DwindleNode = null
var panel_id: String = ""
var panel: PanelBase = null


static func leaf(panel_ref: PanelBase) -> DwindleNode:
	var node: DwindleNode = DwindleNode.new()
	node.node_type = TYPE_LEAF
	node.panel = panel_ref
	node.panel_id = panel_ref.panel_id
	return node


static func branch(split_direction: String, split_ratio: float, first_child: DwindleNode, second_child: DwindleNode) -> DwindleNode:
	var node: DwindleNode = DwindleNode.new()
	node.node_type = TYPE_BRANCH
	node.split = split_direction
	node.ratio = split_ratio
	node.set_children(first_child, second_child)
	return node


func is_leaf() -> bool:
	return node_type == TYPE_LEAF


func set_children(first_child: DwindleNode, second_child: DwindleNode) -> void:
	first = first_child
	second = second_child
	if first != null:
		first.parent = self
	if second != null:
		second.parent = self


func sibling_of(child: DwindleNode) -> DwindleNode:
	if first == child:
		return second
	if second == child:
		return first
	return null


func depth() -> int:
	var current: DwindleNode = parent
	var result: int = 0
	while current != null:
		result += 1
		current = current.parent
	return result


func collect_leaves(out: Array[DwindleNode]) -> void:
	if is_leaf():
		out.append(self)
		return
	if first != null:
		first.collect_leaves(out)
	if second != null:
		second.collect_leaves(out)


func serialize() -> Variant:
	if is_leaf():
		return {
			"type": TYPE_LEAF,
			"panel_id": panel_id,
			"agent_id": panel.agent_id if panel != null else "",
			"title": panel.panel_title if panel != null else panel_id,
			"mode": panel.mode if panel != null else "scroll",
		}
	return {
		"type": TYPE_BRANCH,
		"split": split,
		"ratio": ratio,
		"children": [
			first.serialize() if first != null else null,
			second.serialize() if second != null else null,
		],
	}
