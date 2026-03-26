package terminal

import (
	"context"
	"fmt"
)

// SendCommand sends a command string to the named session's active pane
// followed by Enter to execute it.
//
// Enter is sent as a separate argument to send-keys to avoid tmux
// interpreting special characters in the command string.
func (t *TmuxSubstrate) SendCommand(ctx context.Context, session string, cmd string) error {
	if err := ValidateSessionName(session); err != nil {
		return err
	}
	if _, err := t.run(ctx, "send-keys", "-t", session, cmd, "Enter"); err != nil {
		return fmt.Errorf("send command to session %q: %w", session, err)
	}
	return nil
}
