package terminal

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCaptureOutput_InvalidLines(t *testing.T) {
	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Skip("tmux not found on PATH; skipping")
	}

	ctx := context.Background()
	for _, lines := range []int{0, -1, -100} {
		_, err := sub.CaptureOutput(ctx, "any-session", lines)
		if !errors.Is(err, ErrInvalidLines) {
			t.Errorf("CaptureOutput(lines=%d): got %v, want ErrInvalidLines", lines, err)
		}
	}
}

func TestCaptureOutput_SessionNotFound(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	_, err = sub.CaptureOutput(ctx, "nonexistent-session-t2.2", 50)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("CaptureOutput to missing session: got %v, want ErrSessionNotFound", err)
	}
}

func TestCaptureOutput_ValidSession(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	const sessionName = "test-capture-valid"
	if _, err := sub.SpawnSession(ctx, sessionName, "", SessionOptions{}); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(ctx, sessionName) })

	// Send a recognisable string so we have something to capture.
	if err := sub.SendCommand(ctx, sessionName, "echo capture-test-marker"); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	// Give tmux a moment to render the output into the pane buffer.
	time.Sleep(200 * time.Millisecond)

	out, err := sub.CaptureOutput(ctx, sessionName, 50)
	if err != nil {
		t.Fatalf("CaptureOutput: %v", err)
	}
	if out == "" {
		t.Error("CaptureOutput returned empty string for a session that received a command")
	}
}
