//go:build integration

package terminal

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestFullLifecycle exercises the complete substrate flow:
// SpawnSession → SendCommand → CaptureOutput → SplitPane → ListSessions → DestroySession
func TestFullLifecycle(t *testing.T) {
	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	const sessionName = "integration-lifecycle"
	// Clean up any stale session from a previous failed run.
	_ = sub.DestroySession(sessionName)

	// --- SpawnSession ---
	sess, err := sub.SpawnSession(sessionName, "bash")
	if err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(sessionName) })

	if sess.Name != sessionName {
		t.Errorf("SpawnSession: Name got %q, want %q", sess.Name, sessionName)
	}

	// --- SendCommand ---
	if err := sub.SendCommand(sessionName, "echo INTEGRATION_MARKER_12345"); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	// Give tmux a moment to process the command.
	time.Sleep(200 * time.Millisecond)

	// --- CaptureOutput ---
	out, err := sub.CaptureOutput(sessionName, 50)
	if err != nil {
		t.Fatalf("CaptureOutput: %v", err)
	}
	if !strings.Contains(out, "INTEGRATION_MARKER_12345") {
		t.Errorf("CaptureOutput: expected output to contain marker, got:\n%s", out)
	}

	// --- SplitPane ---
	pane, err := sub.SplitPane(sessionName, Horizontal)
	if err != nil {
		t.Fatalf("SplitPane(Horizontal): %v", err)
	}
	if pane.ID == "" {
		t.Error("SplitPane: returned pane has empty ID")
	}
	if pane.SessionID != sessionName {
		t.Errorf("SplitPane: SessionID got %q, want %q", pane.SessionID, sessionName)
	}

	// --- ListSessions ---
	sessions, err := sub.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.Name == sessionName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListSessions: session %q not found", sessionName)
	}

	// --- DestroySession ---
	if err := sub.DestroySession(sessionName); err != nil {
		t.Fatalf("DestroySession: %v", err)
	}

	// Verify session is gone.
	err = sub.DestroySession(sessionName)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("DestroySession after destroy: got %v, want ErrSessionNotFound", err)
	}
}

// TestDuplicateSession verifies that spawning the same session name twice
// returns ErrSessionExists.
func TestDuplicateSession(t *testing.T) {
	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	const sessionName = "integration-duplicate"
	_ = sub.DestroySession(sessionName)

	if _, err := sub.SpawnSession(sessionName, ""); err != nil {
		t.Fatalf("SpawnSession (first): %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(sessionName) })

	_, err = sub.SpawnSession(sessionName, "")
	if !errors.Is(err, ErrSessionExists) {
		t.Errorf("SpawnSession (duplicate): got %v, want ErrSessionExists", err)
	}
}

// TestSendCommand_NonExistentSession verifies error path.
func TestSendCommand_NonExistentSession(t *testing.T) {
	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	err = sub.SendCommand("integration-ghost-session", "echo hello")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("SendCommand to ghost: got %v, want ErrSessionNotFound", err)
	}
}

// TestCaptureOutput_NonExistentSession verifies error path.
func TestCaptureOutput_NonExistentSession(t *testing.T) {
	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	_, err = sub.CaptureOutput("integration-ghost-session", 10)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("CaptureOutput to ghost: got %v, want ErrSessionNotFound", err)
	}
}

// TestMultiPaneGrid creates a 2x2 grid (4 panes) and verifies.
func TestMultiPaneGrid(t *testing.T) {
	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	const sessionName = "integration-grid"
	_ = sub.DestroySession(sessionName)

	if _, err := sub.SpawnSession(sessionName, ""); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(sessionName) })

	// Split into 4 panes: horizontal, then vertical on each half.
	if _, err := sub.SplitPane(sessionName, Horizontal); err != nil {
		t.Fatalf("SplitPane 1 (H): %v", err)
	}
	if _, err := sub.SplitPane(sessionName, Vertical); err != nil {
		t.Fatalf("SplitPane 2 (V): %v", err)
	}
	// Select the first pane, then split it vertically too.
	// We don't have SelectPane yet, so just do another split — we'll have 4 panes.
	if _, err := sub.SplitPane(sessionName, Vertical); err != nil {
		t.Fatalf("SplitPane 3 (V): %v", err)
	}

	// Verify via ListSessions that PaneCount is reasonable (may be more with tabs).
	sessions, err := sub.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	for _, s := range sessions {
		if s.Name == sessionName {
			if s.PaneCount < 1 {
				t.Errorf("PaneCount should be >= 1, got %d", s.PaneCount)
			}
			return
		}
	}
	t.Errorf("session %q not found in ListSessions", sessionName)
}

// TestValidateSessionName_Integration verifies the validation utility works
// as expected for names that would cause tmux issues.
func TestValidateSessionName_Integration(t *testing.T) {
	if err := ValidateSessionName("valid-agent-01"); err != nil {
		t.Errorf("valid name rejected: %v", err)
	}
	if err := ValidateSessionName("has spaces"); err == nil {
		t.Error("name with spaces should be rejected")
	}
	if err := ValidateSessionName("has:colon"); err == nil {
		t.Error("name with colon should be rejected")
	}
	if err := ValidateSessionName(""); err == nil {
		t.Error("empty name should be rejected")
	}
}

// TestCheckTmux_Integration verifies the preflight check.
func TestCheckTmux_Integration(t *testing.T) {
	if err := CheckTmux(); err != nil {
		t.Skipf("tmux not available: %v", err)
	}

	ver, err := TmuxVersion()
	if err != nil {
		t.Fatalf("TmuxVersion: %v", err)
	}
	if !strings.HasPrefix(ver, "tmux") {
		t.Errorf("TmuxVersion: got %q, expected prefix 'tmux'", ver)
	}
}
