package terminal

import (
	"context"
	"fmt"
)

// SendCommand sends a command string to the named session's active pane
// followed by Enter to execute it.
//
// The text goes in its own send-keys call with the -l flag, so tmux types it
// literally. Without -l, tmux resolves a text that matches a key name, such as
// Enter, Space or C-c, into that key, and it reads a leading hyphen as a flag.
// Enter follows in a second call, without -l, so it stays a key press.
func (t *TmuxSubstrate) SendCommand(ctx context.Context, session string, cmd string) error {
	if err := ValidateSessionName(session); err != nil {
		return err
	}
	if err := ValidateCommand(cmd); err != nil {
		return err
	}
	if _, err := t.run(ctx, "send-keys", "-t", session, "-l", "--", cmd); err != nil {
		return fmt.Errorf("send command to session %q: %w", session, err)
	}
	if _, err := t.run(ctx, "send-keys", "-t", session, "Enter"); err != nil {
		return fmt.Errorf("send enter to session %q: %w", session, err)
	}
	return nil
}
