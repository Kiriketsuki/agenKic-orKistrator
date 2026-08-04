# sigil_config_page_smoke_test.gd — Runtime smoke guard for SigilConfigPage.
#
# The page builds every Control in code, including the themed in-engine
# folder picker (_build_dialog_theme). A bad Theme key or a wrong node type
# there is invisible to a parse check and only bites when the page is
# actually built, so this instantiates the real page inside a SceneTree,
# opens it against a fake roster, and pops the folder dialog.
#
#   godot --headless --path godot --script tests/sigil_config_page_smoke_test.gd
#
# Exits 1 on any failure.

extends SceneTree

const TEST_PATH: String = "user://sigil_config_page_smoke.cfg"


# _initialize (not _init), and it awaits one frame first: the root Window
# is not yet inside the tree when _initialize runs, so a Control added
# before that frame never gets its _ready() and the page would be an empty
# shell. This test needs the page fully built to inspect it.
func _initialize() -> void:
	await process_frame
	var failures: Array[String] = []
	_run_build_and_open_case(failures)
	if FileAccess.file_exists(TEST_PATH):
		DirAccess.remove_absolute(TEST_PATH)
	if failures.is_empty():
		print("sigil_config_page_smoke_test: all cases passed")
		quit(0)
	else:
		for message: String in failures:
			printerr("sigil_config_page_smoke_test: FAIL — " + message)
		quit(1)


func _assert(condition: bool, message: String, failures: Array[String]) -> void:
	if not condition:
		failures.append(message)


## The page must build its rows, take a roster, and produce a fully themed
## folder dialog without erroring.
func _run_build_and_open_case(failures: Array[String]) -> void:
	var page := SigilConfigPage.new()
	root.add_child(page)

	# Loaded from (and therefore saved back to) a throwaway fixture path.
	# A bare SigilConfig.new() would carry the DEFAULT_PATH and any save
	# triggered while building the page would overwrite the real keeper
	# settings — which is exactly what this test used to do.
	var config: SigilConfig = SigilConfig.load_from_file(TEST_PATH)
	config.workdir = "/tmp"
	var providers: Array = [
		{"kind": "claude", "display": "Claude", "accent": "#d98c00"},
		{"kind": "codex", "display": "Codex", "accent": "#5999f2"},
	]
	page.open_page(providers, config)

	var dialog: FileDialog = page.get_node_or_null("FolderDialog") as FileDialog
	if dialog == null:
		# The dialog is added without an explicit name, so fall back to a
		# type scan rather than coupling this test to a node name.
		for child: Node in page.get_children():
			if child is FileDialog:
				dialog = child as FileDialog
				break
	_assert(dialog != null, "the page should own a FileDialog for the BROWSE button", failures)
	if dialog == null:
		page.queue_free()
		return

	_assert(not dialog.use_native_dialog, "the folder picker must be the in-engine dialog so it can be themed", failures)
	_assert(dialog.file_mode == FileDialog.FILE_MODE_OPEN_DIR, "the folder picker must select directories", failures)
	_assert(dialog.theme != null, "the folder picker must carry the parchment theme", failures)
	if dialog.theme != null:
		_assert(dialog.theme.has_stylebox("panel", "Window"), "the dialog theme must style the Window panel", failures)
		_assert(dialog.theme.has_stylebox("normal", "Button"), "the dialog theme must style Buttons", failures)

	# Actually popping it is the part that exercises the theme against real
	# child Controls — a bad theme entry surfaces here, not at build time.
	dialog.popup_centered_ratio(0.7)
	_assert(dialog.visible, "the folder picker should be visible after popup", failures)
	dialog.hide()

	page.queue_free()
