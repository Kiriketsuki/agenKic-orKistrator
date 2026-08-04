package terminal

import (
	"context"
	"fmt"
	"unicode"
	"unicode/utf8"
)

// allowedKeyNames lists every tmux key name that SendKey accepts.
//
// tmux resolves a bare send-keys argument against its key table, so an
// unchecked value lets a caller send any key, including a tmux command prefix
// such as C-b. The whitelist keeps the endpoint to the navigation keys the
// terminal panel needs.
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
}

// ValidateKeyName reports whether name is a key SendKey may forward.
//
// A name passes when the whitelist contains it, or when it is a single
// printable character such as a digit or a letter. Everything else fails with
// ErrInvalidCommand so the HTTP layer answers 400.
func ValidateKeyName(name string) error {
	if allowedKeyNames[name] {
		return nil
	}
	if r, size := utf8.DecodeRuneInString(name); size == len(name) && size > 0 {
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
