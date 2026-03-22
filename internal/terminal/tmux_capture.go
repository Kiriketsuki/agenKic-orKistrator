package terminal

import (
	"fmt"
	"strings"
)

// CaptureOutput reads the last N lines from the named session's active pane.
// It uses tmux capture-pane with the -S flag to request scrollback history.
// Returns ErrInvalidLines if lines <= 0, ErrSessionNotFound if the session
// does not exist.
func (t *TmuxSubstrate) CaptureOutput(session string, lines int) (string, error) {
	if lines <= 0 {
		return "", ErrInvalidLines
	}

	// -p  prints to stdout
	// -S  start-line: negative offset counts back from the visible pane top
	out, err := t.run("capture-pane", "-t", session, "-p", "-S", fmt.Sprintf("-%d", lines))
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "session not found") ||
			strings.Contains(errStr, "can't find session") {
			return "", ErrSessionNotFound
		}
		return "", fmt.Errorf("capture output from session %q: %w", session, err)
	}

	return strings.TrimRight(out, "\n"), nil
}
