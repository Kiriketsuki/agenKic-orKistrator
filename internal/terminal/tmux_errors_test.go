package terminal

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestCheckTmux verifies CheckTmux returns nil when tmux is available on PATH.
func TestCheckTmux(t *testing.T) {
	if err := CheckTmux(); err != nil {
		if errors.Is(err, ErrTmuxNotFound) {
			t.Skip("tmux not available on PATH — skipping")
		}
		t.Fatalf("CheckTmux() unexpected error: %v", err)
	}
}

// TestTmuxVersion verifies TmuxVersion returns a non-empty version string.
func TestTmuxVersion(t *testing.T) {
	ver, err := TmuxVersion()
	if err != nil {
		if errors.Is(err, ErrTmuxNotFound) {
			t.Skip("tmux not available on PATH — skipping")
		}
		t.Fatalf("TmuxVersion() unexpected error: %v", err)
	}
	if ver == "" {
		t.Fatal("TmuxVersion() returned empty string")
	}
	if !strings.HasPrefix(ver, "tmux ") {
		t.Errorf("TmuxVersion() = %q, want prefix \"tmux \"", ver)
	}
}

// TestParseTmuxError_SessionNotFound verifies both "session not found" variants.
func TestParseTmuxError_SessionNotFound(t *testing.T) {
	for _, msg := range []string{"session not found", "can't find session myagent"} {
		err := parseTmuxError(msg)
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("parseTmuxError(%q) = %v, want ErrSessionNotFound", msg, err)
		}
	}
}

// TestParseTmuxError_SessionExists verifies the duplicate-session pattern.
func TestParseTmuxError_SessionExists(t *testing.T) {
	err := parseTmuxError("duplicate session: agent-01")
	if !errors.Is(err, ErrSessionExists) {
		t.Errorf("parseTmuxError(duplicate session) = %v, want ErrSessionExists", err)
	}
}

// TestParseTmuxError_PaneLimit verifies both pane-limit patterns.
func TestParseTmuxError_PaneLimit(t *testing.T) {
	for _, msg := range []string{"create pane failed", "no room for new pane"} {
		err := parseTmuxError(msg)
		if !errors.Is(err, ErrPaneLimit) {
			t.Errorf("parseTmuxError(%q) = %v, want ErrPaneLimit", msg, err)
		}
	}
}

// TestParseTmuxError_NoServer verifies "no server running" and related patterns.
func TestParseTmuxError_NoServer(t *testing.T) {
	for _, msg := range []string{
		"no server running on /tmp/tmux-1000/default",
		"no sessions",
		"error connecting to /tmp/tmux-1000/default: No such file or directory",
	} {
		err := parseTmuxError(msg)
		if !errors.Is(err, ErrNoServer) {
			t.Errorf("parseTmuxError(%q) = %v, want ErrNoServer", msg, err)
		}
	}
}

// TestParseTmuxError_Generic verifies unknown stderr is wrapped as a plain error.
func TestParseTmuxError_Generic(t *testing.T) {
	msg := "some unknown tmux error"
	err := parseTmuxError(msg)
	if err == nil {
		t.Fatal("parseTmuxError(unknown) returned nil")
	}
	if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionExists) || errors.Is(err, ErrPaneLimit) || errors.Is(err, ErrNoServer) {
		t.Errorf("parseTmuxError(unknown) matched a sentinel: %v", err)
	}
	if !strings.Contains(err.Error(), msg) {
		t.Errorf("parseTmuxError(unknown) = %q, want it to contain the original message", err.Error())
	}
}

// TestDestroySession_InvalidSessionName verifies validation fires before subprocess call.
func TestDestroySession_InvalidSessionName(t *testing.T) {
	sub := &TmuxSubstrate{tmuxPath: "tmux"}
	ctx := context.Background()

	for _, name := range []string{"", "bad session", "bad:colon", "bad/slash"} {
		err := sub.DestroySession(ctx, name)
		if err == nil {
			t.Errorf("DestroySession(session=%q): expected validation error, got nil", name)
		}
	}
}

// TestValidateCommand_Valid verifies accepted command patterns.
func TestValidateCommand_Valid(t *testing.T) {
	valid := []string{
		"",                                 // empty command (default shell)
		"echo hello",                       // simple command
		"ls -la /tmp",                      // flags and paths
		"echo 'hello\tworld'",              // tab is allowed
		"echo 'line1\nline2'",              // newline is allowed
		"echo 'line1\r\nline2'",            // carriage return is allowed
		strings.Repeat("a", MaxCommandLen), // at max length
	}
	for _, cmd := range valid {
		if err := ValidateCommand(cmd); err != nil {
			t.Errorf("ValidateCommand(%q) = %v, want nil", cmd, err)
		}
	}
}

// TestValidateCommand_Invalid verifies rejected command patterns.
func TestValidateCommand_Invalid(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"null byte", "echo\x00hello"},
		{"ctrl-c", "echo\x03hello"},
		{"ctrl-d", "echo\x04hello"},
		{"ctrl-z", "echo\x1ahello"},
		{"escape", "echo\x1bhello"},
		{"ctrl-a", "\x01cmd"},
		{"file separator", "echo\x1chello"},
		{"unit separator", "echo\x1fhello"},
		{"over max length", strings.Repeat("a", MaxCommandLen+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommand(tt.cmd)
			if err == nil {
				t.Errorf("ValidateCommand(%q) = nil, want error", tt.name)
			}
			if !errors.Is(err, ErrInvalidCommand) {
				t.Errorf("ValidateCommand(%q) = %v, want ErrInvalidCommand", tt.name, err)
			}
		})
	}
}

// TestSendCommand_InvalidCommand verifies cmd validation fires before subprocess call.
func TestSendCommand_InvalidCommand(t *testing.T) {
	sub := &TmuxSubstrate{tmuxPath: "tmux"}
	ctx := context.Background()

	err := sub.SendCommand(ctx, "valid-session", "echo\x00bad")
	if err == nil {
		t.Fatal("SendCommand with null byte: expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("SendCommand with null byte = %v, want ErrInvalidCommand", err)
	}
}

// TestSpawnSession_InvalidCommand verifies cmd validation fires before subprocess call.
func TestSpawnSession_InvalidCommand(t *testing.T) {
	sub := &TmuxSubstrate{tmuxPath: "tmux"}
	ctx := context.Background()

	_, err := sub.SpawnSession(ctx, "valid-session", "echo\x03bad")
	if err == nil {
		t.Fatal("SpawnSession with ctrl-c: expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidCommand) {
		t.Errorf("SpawnSession with ctrl-c = %v, want ErrInvalidCommand", err)
	}
}

// TestValidateSessionName_Valid verifies accepted name patterns.
func TestValidateSessionName_Valid(t *testing.T) {
	valid := []string{
		"agent01",
		"agent-codegen-01",
		"Agent_01",
		"my.session",
		"a",
		"abc123",
		"A-b_c.1",
	}
	for _, name := range valid {
		if err := ValidateSessionName(name); err != nil {
			t.Errorf("ValidateSessionName(%q) = %v, want nil", name, err)
		}
	}
}

// TestValidateSessionName_Invalid verifies rejected name patterns.
func TestValidateSessionName_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"has space",
		"has:colon",
		"has/slash",
		"has@at",
		"has#hash",
		"has!bang",
		"tab\there",
		"new\nline",
	}
	for _, name := range invalid {
		if err := ValidateSessionName(name); err == nil {
			t.Errorf("ValidateSessionName(%q) = nil, want error", name)
		}
	}
}
