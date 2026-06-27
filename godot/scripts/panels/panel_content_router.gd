# panel_content_router.gd — Mounts mode-specific content into a PanelBase's
# ContentRoot. Content is composed at runtime (not subclassed) so a panel that
# is live-toggled between modes (the title bar's Mode button) can swap its
# body without losing its chrome, drag/dock/fullscreen state, or identity.

class_name PanelContentRouter

const SPELL_SCROLL_SCENE: PackedScene = preload("res://scenes/spell_scroll_view.tscn")
const INJECTED_CONTENT_NAME: String = "InjectedContent"
const CONTENT_MARGIN_DEFAULT: int = 12


## Idempotent: frees any previously-injected content before mounting the
## content for `mode`. `agent_data` may be null (e.g. non-agent panels).
static func mount(panel: PanelBase, mode: String, bridge: Node, agent_data: BridgeData.AgentData) -> void:
	if panel == null:
		return
	var content_root: MarginContainer = panel.get_content_root()
	if content_root == null:
		return
	_clear_injected_content(content_root)
	var placeholder: Label = content_root.get_node_or_null("Placeholder")
	match mode:
		"scroll":
			if placeholder != null:
				placeholder.visible = false
			_set_content_margins(content_root, 0)
			var view: Control = SPELL_SCROLL_SCENE.instantiate() as Control
			view.name = INJECTED_CONTENT_NAME
			content_root.add_child(view)
			if view.has_method("setup"):
				view.call("setup", panel, agent_data, bridge)
		_:
			# Terminal mode content lands in T10 — leave the generic placeholder up.
			_set_content_margins(content_root, CONTENT_MARGIN_DEFAULT)
			if placeholder != null:
				placeholder.visible = true


static func _clear_injected_content(content_root: MarginContainer) -> void:
	var existing: Node = content_root.get_node_or_null(INJECTED_CONTENT_NAME)
	if existing != null:
		existing.queue_free()


static func _set_content_margins(content_root: MarginContainer, value: int) -> void:
	content_root.add_theme_constant_override("margin_left", value)
	content_root.add_theme_constant_override("margin_top", value)
	content_root.add_theme_constant_override("margin_right", value)
	content_root.add_theme_constant_override("margin_bottom", value)
