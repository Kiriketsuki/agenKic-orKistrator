package terminal

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCaptureScreen_InvalidSessionName(t *testing.T) {
	// Validation fires before any subprocess call, so no tmux is required.
	sub := &TmuxSubstrate{tmuxPath: "tmux"}
	ctx := context.Background()

	for _, name := range []string{"", "bad session", "bad:colon", "bad/slash"} {
		if _, err := sub.CaptureScreen(ctx, name); err == nil {
			t.Errorf("CaptureScreen(session=%q): expected validation error, got nil", name)
		}
	}
}

func TestCaptureScreen_SessionNotFound(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	if _, err := sub.CaptureScreen(context.Background(), "nonexistent-session-screen"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("CaptureScreen on missing session: got %v, want ErrSessionNotFound", err)
	}
}

func TestCaptureScreen_VisibleMarker(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	const sessionName = "test-capture-screen"
	if _, err := sub.SpawnSession(ctx, sessionName, ""); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(ctx, sessionName) })

	if err := sub.SendCommand(ctx, sessionName, "printf 'SCREENMARKER-XYZ\\n'"); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var last string
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for SCREENMARKER-XYZ; last screen:\n%s", last)
		case <-ticker.C:
			out, err := sub.CaptureScreen(ctx, sessionName)
			if err != nil {
				continue
			}
			last = out
			if strings.Contains(out, "SCREENMARKER-XYZ") {
				return
			}
		}
	}
}

// TmuxSubstrate must satisfy the optional ScreenCapturer capability.
var _ ScreenCapturer = (*TmuxSubstrate)(nil)
