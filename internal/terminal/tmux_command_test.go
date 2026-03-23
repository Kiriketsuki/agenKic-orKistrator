package terminal

import (
	"context"
	"errors"
	"testing"
)

func TestSendCommand_ValidSession(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	const sessionName = "test-sendcmd-valid"
	if _, err := sub.SpawnSession(ctx, sessionName, ""); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(ctx, sessionName) })

	if err := sub.SendCommand(ctx, sessionName, "echo hello"); err != nil {
		t.Errorf("SendCommand to valid session: unexpected error: %v", err)
	}
}

// TestSendCommand_SpecialCharacters verifies that tmux's send-keys API accepts
// keystroke sequences containing special characters without returning an error.
// It does NOT verify shell-side interpretation of these commands. End-to-end
// command execution (SendCommand + CaptureOutput) is covered by TestFullLifecycle
// in integration_test.go.
func TestSendCommand_SpecialCharacters(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	const sessionName = "test-sendcmd-special"
	if _, err := sub.SpawnSession(ctx, sessionName, ""); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(ctx, sessionName) })

	// Commands with special characters that tmux might misinterpret.
	cmds := []string{
		"echo 'foo; bar' && echo done",
		`echo "hello world"`,
		"echo $HOME",
		"echo test#comment",
	}
	for _, cmd := range cmds {
		if err := sub.SendCommand(ctx, sessionName, cmd); err != nil {
			t.Errorf("SendCommand(%q): unexpected error: %v", cmd, err)
		}
	}
}

func TestSendCommand_SessionNotFound(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	err = sub.SendCommand(ctx, "nonexistent-session-t2.1", "echo hi")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("SendCommand to missing session: got %v, want ErrSessionNotFound", err)
	}
}
