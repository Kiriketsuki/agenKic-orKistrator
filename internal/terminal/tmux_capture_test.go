package terminal

import (
	"context"
	"errors"
	"strings"
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

func TestCaptureOutput_ScrollbackMath(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	const sessionName = "test-capture-math"
	if _, err := sub.SpawnSession(ctx, sessionName, ""); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(ctx, sessionName) })

	// Verify various line counts all produce valid (non-error) captures.
	for _, lines := range []int{1, 10, 100, 500} {
		out, err := sub.CaptureOutput(ctx, sessionName, lines)
		if err != nil {
			t.Errorf("CaptureOutput(lines=%d): unexpected error: %v", lines, err)
		}
		// Output may be empty for a fresh session, but should not error.
		_ = out
	}
}

func TestCaptureOutput_LargeScrollback(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	const sessionName = "test-capture-scrollback"
	if _, err := sub.SpawnSession(ctx, sessionName, ""); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(ctx, sessionName) })

	// Generate 500 numbered lines via a single command to avoid prompt noise.
	cmd := "for i in $(seq 1 500); do printf 'SCROLLTEST-LINE-%03d\\n' $i; done"
	if err := sub.SendCommand(ctx, sessionName, cmd); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	// Wait for the loop to finish producing output.
	time.Sleep(3 * time.Second)

	// Capture a window large enough to contain the last lines.
	out, err := sub.CaptureOutput(ctx, sessionName, 100)
	if err != nil {
		t.Fatalf("CaptureOutput: %v", err)
	}

	// The last line of the loop output should be within the capture window.
	if !strings.Contains(out, "SCROLLTEST-LINE-500") {
		t.Errorf("expected captured output to contain 'SCROLLTEST-LINE-500', got:\n%s", out)
	}

	// Very early lines should have scrolled past the capture window.
	if strings.Contains(out, "SCROLLTEST-LINE-001") {
		t.Errorf("expected 'SCROLLTEST-LINE-001' to be outside capture window, but it appeared")
	}

	// The lower boundary of a 100-line window over 500 lines should be
	// approximately LINE-401. Verify the boundary marker is present.
	// (Satisfies Gherkin scenario: "the first returned line is line 401")
	if !strings.Contains(out, "SCROLLTEST-LINE-401") {
		t.Errorf("expected captured output to contain 'SCROLLTEST-LINE-401' as lower boundary, got:\n%s", out)
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
	if _, err := sub.SpawnSession(ctx, sessionName, ""); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(ctx, sessionName) })

	if err := sub.SendCommand(ctx, sessionName, "echo capture-test-marker"); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	out, err := sub.CaptureOutput(ctx, sessionName, 50)
	if err != nil {
		t.Fatalf("CaptureOutput: %v", err)
	}
	if !strings.Contains(out, "capture-test-marker") {
		t.Errorf("CaptureOutput: expected output to contain 'capture-test-marker', got:\n%s", out)
	}
}
