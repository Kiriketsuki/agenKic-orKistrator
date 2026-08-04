package terminal

import (
	"context"
	"fmt"
	"strings"
)

// ScreenCapturer is an optional Substrate capability: capture the visible
// pane content of a session, with SGR styling preserved. A redrawing TUI
// overwrites the same screen, so history capture returns duplicated frames.
// The interface stays separate from Substrate so existing mocks compile.
type ScreenCapturer interface {
	CaptureScreen(ctx context.Context, session string) (string, error)
}

// CaptureScreen reads the visible pane of the named session. It omits the -S
// flag, so tmux returns only the current screen. The -e flag keeps SGR color
// sequences, which the Godot ANSI parser renders.
func (t *TmuxSubstrate) CaptureScreen(ctx context.Context, session string) (string, error) {
	if err := ValidateSessionName(session); err != nil {
		return "", err
	}

	out, err := t.run(ctx, "capture-pane", "-t", session, "-p", "-e")
	if err != nil {
		return "", fmt.Errorf("capture screen from session %q: %w", session, err)
	}

	return strings.TrimRight(out, "\n"), nil
}
