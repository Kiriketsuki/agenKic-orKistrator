package terminal

import (
	"context"
	"fmt"
	"unicode"
	"unicode/utf8"
)

// allowedKeyNames lists every tmux key name that SendKey accepts.
//
// tmux resolves a bare send-keys argument against its key table. The
// whitelist names the multi-character keys the panels forward. Single
// printable runes and C-/M- chords pass through ValidateKeyName's rune
// check instead. The agent sessions run with a detached tmux server, so a
// forwarded C-b lands in the pane, not in a client prefix table.
var allowedKeyNames = map[string]bool{
	"Up":     true,
	"Down":   true,
	"Left":   true,
	"Right":  true,
	"Enter":  true,
	"Escape": true,
	"Tab":    true,
	"BTab":   true,
	"Space":  true,
	"PPage":  true,
	"NPage":  true,
	"Home":   true,
	"End":    true,
	"BSpace": true,
	"DC":     true,
	"IC":     true,
	"F1":     true,
	"F2":     true,
	"F3":     true,
	"F4":     true,
	"F5":     true,
	"F6":     true,
	"F7":     true,
	"F8":     true,
	"F9":     true,
	"F10":    true,
	"F11":    true,
	"F12":    true,
}

// ValidateKeyName reports whether name is a key SendKey may forward.
//
// A name passes in three cases. The whitelist contains it. It is one
// printable character such as a digit or a letter. It is a C- or M-
// prefixed printable character such as C-c, which the full-passthrough
// panels need for control and alt chords. Everything else fails with
// ErrInvalidCommand so the HTTP layer answers 400.
func ValidateKeyName(name string) error {
	if allowedKeyNames[name] {
		return nil
	}
	body := name
	if len(name) > 2 && (name[:2] == "C-" || name[:2] == "M-") {
		body = name[2:]
	}
	if r, size := utf8.DecodeRuneInString(body); size == len(body) && size > 0 {
		if r != utf8.RuneError && unicode.IsPrint(r) {
			return nil
		}
	}
	return fmt.Errorf("%w: key %q is not allowed", ErrInvalidCommand, name)
}

// SendKey sends one key press to the named session's active pane.
//
// The key goes through send-keys WITHOUT the -l flag, so tmux resolves the
// name into a key press. That is the opposite of SendCommand, which types its
// argument literally. A curses application such as a Claude Code trust prompt
// only reacts to a real key press.
func (t *TmuxSubstrate) SendKey(ctx context.Context, session string, key string) error {
	if err := ValidateSessionName(session); err != nil {
		return err
	}
	if err := ValidateKeyName(key); err != nil {
		return err
	}
	if _, err := t.run(ctx, "send-keys", "-t", session, "--", key); err != nil {
		return fmt.Errorf("send key %q to session %q: %w", key, session, err)
	}
	return nil
}
