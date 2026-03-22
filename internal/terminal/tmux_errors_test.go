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
	ver, err := TmuxVersion(context.Background())
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

// TestParseTmuxError_SessionNotFound verifies "session not found" variants.
func TestParseTmuxError_SessionNotFound(t *testing.T) {
	for _, msg := range []string{"session not found", "can't find session myagent"} {
		err := parseTmuxError(msg)
		if !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("parseTmuxError(%q) = %v, want ErrSessionNotFound", msg, err)
		}
	}
}

// TestParseTmuxError_PaneNotFound verifies window/pane not-found variants.
func TestParseTmuxError_PaneNotFound(t *testing.T) {
	for _, msg := range []string{"can't find window 3", "can't find pane %5"} {
		err := parseTmuxError(msg)
		if !errors.Is(err, ErrPaneNotFound) {
			t.Errorf("parseTmuxError(%q) = %v, want ErrPaneNotFound", msg, err)
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

// TestParseTmuxError_Generic verifies unknown stderr is wrapped as a plain error.
func TestParseTmuxError_Generic(t *testing.T) {
	msg := "some unknown tmux error"
	err := parseTmuxError(msg)
	if err == nil {
		t.Fatal("parseTmuxError(unknown) returned nil")
	}
	if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionExists) || errors.Is(err, ErrPaneLimit) {
		t.Errorf("parseTmuxError(unknown) matched a sentinel: %v", err)
	}
	if !strings.Contains(err.Error(), msg) {
		t.Errorf("parseTmuxError(unknown) = %q, want it to contain the original message", err.Error())
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
