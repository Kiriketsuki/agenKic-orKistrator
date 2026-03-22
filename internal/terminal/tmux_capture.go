package terminal

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// CaptureOutput reads the last N lines from the named session's active pane.
// It uses tmux capture-pane with the -S flag to request scrollback history.
// Returns ErrInvalidLines if lines <= 0, ErrSessionNotFound if the session
// does not exist.
func (t *TmuxSubstrate) CaptureOutput(ctx context.Context, session string, lines int) (string, error) {
	if lines <= 0 {
		return "", ErrInvalidLines
	}

	// -p  prints to stdout
	// -S  start-line: negative offset counts back from the visible pane top
	out, err := t.run(ctx, "capture-pane", "-t", session, "-p", "-S", fmt.Sprintf("-%d", lines))
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrPaneNotFound) {
			return "", ErrSessionNotFound
		}
		return "", fmt.Errorf("capture output from session %q: %w", session, err)
	}

	return strings.TrimRight(out, "\n"), nil
}
