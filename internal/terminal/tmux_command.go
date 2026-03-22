package terminal

import (
	"fmt"
	"strings"
)

// SendCommand sends a command string to the named session's active pane
// followed by Enter to execute it.
//
// Enter is sent as a separate argument to send-keys to avoid tmux
// interpreting special characters in the command string.
func (t *TmuxSubstrate) SendCommand(session string, cmd string) error {
	if _, err := t.run("send-keys", "-t", session, cmd, "Enter"); err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "session not found") ||
			strings.Contains(errStr, "can't find session") ||
			strings.Contains(errStr, "can't find window") ||
			strings.Contains(errStr, "can't find pane") {
			return ErrSessionNotFound
		}
		return fmt.Errorf("send command to session %q: %w", session, err)
	}
	return nil
}
