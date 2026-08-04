class_name TitleFocusMath
extends RefCounted
## TitleFocusMath — pure focus-order math for the title screen menu (F1),
## extracted out of title_screen.gd so it carries no autoload dependency.
## title_screen.gd preloads BridgeManager (an autoload) at parse time, so a
## headless test that preloads title_screen.gd to reach this math fails to
## compile outside a full Godot boot. This script has no such dependency, so
## tests/title_screen_focus_test.gd preloads it directly instead.


## The menu entry order, mirrored here (not just in TitleScreen) so a
## headless test can check the ordering without preloading title_screen.gd
## (see the module doc-comment above for why that preload fails headless).
## TitleScreen._activate_entry's match statement relies on this same order.
enum TitleMenuEntry { ENTER, GRIMOIRE, DEPART }


## Given the current focused index, the menu entry count, and a direction
## (+1/-1), returns the next focused index, wrapping at both ends. Mirrors
## TitleScreen._move_focus/_set_focus without touching any live Control.
static func next_focus_index(current: int, count: int, direction: int) -> int:
	return (current + direction + count) % count
