extends RefCounted
## KeyPassthrough — one keystroke-to-tmux-key-name mapper shared by
## TerminalView and SpellScrollView.
##
## The rule is mouse-based: while the pointer sits over a panel's output
## body, EVERY keystroke forwards to the agent's tmux session as a real key
## press. While the pointer sits over the input line, keys keep their normal
## editing behavior. The views own the hover tracking. This class only turns
## an InputEventKey into the tmux key name the bridge whitelist accepts, or
## an empty string when the event carries nothing forwardable.

class_name KeyPassthrough

## Multi-character tmux key names by Godot keycode. Everything else goes
## through the printable-character path.
const NAMED_KEYS: Dictionary = {
	KEY_UP: "Up",
	KEY_DOWN: "Down",
	KEY_LEFT: "Left",
	KEY_RIGHT: "Right",
	KEY_ENTER: "Enter",
	KEY_KP_ENTER: "Enter",
	KEY_ESCAPE: "Escape",
	KEY_TAB: "Tab",
	KEY_SPACE: "Space",
	KEY_BACKSPACE: "BSpace",
	KEY_DELETE: "DC",
	KEY_INSERT: "IC",
	KEY_PAGEUP: "PPage",
	KEY_PAGEDOWN: "NPage",
	KEY_HOME: "Home",
	KEY_END: "End",
	KEY_F1: "F1",
	KEY_F2: "F2",
	KEY_F3: "F3",
	KEY_F4: "F4",
	KEY_F5: "F5",
	KEY_F6: "F6",
	KEY_F7: "F7",
	KEY_F8: "F8",
	KEY_F9: "F9",
	KEY_F10: "F10",
	KEY_F11: "F11",
	KEY_F12: "F12",
}


## Returns the tmux key name for a key press, or "" when nothing should
## forward (bare modifier, unmappable chord, key release, echo repeat).
static func key_name_for(key_event: InputEventKey) -> String:
	if key_event == null or not key_event.pressed or key_event.echo:
		return ""
	# Meta chords stay with the desktop environment.
	if key_event.meta_pressed:
		return ""
	if key_event.ctrl_pressed or key_event.alt_pressed:
		return _chord_name(key_event)
	if key_event.keycode == KEY_TAB and key_event.shift_pressed:
		return "BTab"
	if NAMED_KEYS.has(key_event.keycode):
		return NAMED_KEYS[key_event.keycode]
	return _printable(key_event)


## Ctrl and Alt chords. The bridge whitelist accepts C-<char> and M-<char>
## with exactly one printable character, so a chord on a named key such as
## Ctrl+Left cannot forward and returns "".
static func _chord_name(key_event: InputEventKey) -> String:
	var body: String = ""
	if key_event.keycode >= KEY_A and key_event.keycode <= KEY_Z:
		body = String.chr(key_event.keycode).to_lower()
	elif key_event.keycode >= KEY_0 and key_event.keycode <= KEY_9:
		body = String.chr(key_event.keycode)
	elif key_event.keycode == KEY_SPACE:
		body = " "
	if body.is_empty():
		return ""
	# Ctrl wins when both modifiers are down, because tmux has no combined
	# C-M- name the bridge accepts.
	if key_event.ctrl_pressed:
		return "C-" + body
	return "M-" + body


## Unmodified printable character, straight from the event's unicode field.
static func _printable(key_event: InputEventKey) -> String:
	if key_event.unicode <= 0:
		return ""
	var text: String = String.chr(key_event.unicode)
	# Control characters never forward as literals. tmux resolves them from
	# the named forms above instead.
	if text.strip_edges().is_empty() and key_event.keycode != KEY_SPACE:
		return ""
	return text
