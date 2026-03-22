package terminal

import (
	"errors"
	"os/exec"
	"testing"
)

// skipIfNoTmux skips the test if tmux is not available on PATH.
func skipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found on PATH; skipping integration test")
	}
}

func TestSendCommand_ValidSession(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	const sessionName = "test-sendcmd-valid"
	if _, err := sub.SpawnSession(sessionName, ""); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(sessionName) })

	if err := sub.SendCommand(sessionName, "echo hello"); err != nil {
		t.Errorf("SendCommand to valid session: unexpected error: %v", err)
	}
}

func TestSendCommand_SessionNotFound(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	err = sub.SendCommand("nonexistent-session-t2.1", "echo hi")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("SendCommand to missing session: got %v, want ErrSessionNotFound", err)
	}
}
