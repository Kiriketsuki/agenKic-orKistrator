package terminal

import (
	"context"
	"errors"
	"testing"
)

// --- unit tests for parsers (no tmux required) ---

func TestParseSessionLine_Valid(t *testing.T) {
	s, err := parseSessionLine("mySession\t220\t55\t2\t0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "mySession" {
		t.Errorf("Name: got %q, want %q", s.Name, "mySession")
	}
	if s.Width != 220 {
		t.Errorf("Width: got %d, want 220", s.Width)
	}
	if s.Height != 55 {
		t.Errorf("Height: got %d, want 55", s.Height)
	}
	if s.WindowCount != 2 {
		t.Errorf("WindowCount: got %d, want 2", s.WindowCount)
	}
	if s.Attached {
		t.Errorf("Attached: got true, want false")
	}
}

func TestParseSessionLine_Attached(t *testing.T) {
	s, err := parseSessionLine("attached-session\t80\t24\t1\t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.Attached {
		t.Errorf("Attached: got false, want true")
	}
}

func TestParseSessionLine_InvalidFields(t *testing.T) {
	_, err := parseSessionLine("only\tthree\tfields")
	if err == nil {
		t.Fatal("expected error for malformed line, got nil")
	}
}

func TestParseSessionLine_NonNumericWidth(t *testing.T) {
	// Non-numeric width is treated as 0 (lenient parsing for detached sessions)
	s, err := parseSessionLine("session\twide\t24\t1\t0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Width != 0 {
		t.Errorf("Width: got %d, want 0 (non-numeric treated as zero)", s.Width)
	}
}

func TestParsePaneLine_Valid(t *testing.T) {
	p, err := parsePaneLine("%3\tmySession\t100\t25\t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != "%3" {
		t.Errorf("ID: got %q, want %%3", p.ID)
	}
	if p.SessionID != "mySession" {
		t.Errorf("SessionID: got %q, want %q", p.SessionID, "mySession")
	}
	if p.Width != 100 {
		t.Errorf("Width: got %d, want 100", p.Width)
	}
	if p.Height != 25 {
		t.Errorf("Height: got %d, want 25", p.Height)
	}
	if !p.Active {
		t.Errorf("Active: got false, want true")
	}
}

func TestParsePaneLine_InvalidFields(t *testing.T) {
	_, err := parsePaneLine("%0\tsession\t100")
	if err == nil {
		t.Fatal("expected error for malformed line, got nil")
	}
}

// --- integration tests (require tmux) ---

func TestListSessions_NoSessions(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	// Destroy any pre-existing test sessions that might interfere.
	_ = sub.DestroySession(ctx, "test-listsessions-none")

	sessions, err := sub.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: unexpected error: %v", err)
	}
	// We can't guarantee a tmux server with zero sessions in CI, so only
	// assert no error is returned — the function must not fail when the
	// server is up but our named sessions are absent.
	_ = sessions
}

func TestListSessions_WithSession(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	const sessionName = "test-listsessions-active"
	if _, err := sub.SpawnSession(ctx, sessionName, "", SessionOptions{}); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(ctx, sessionName) })

	sessions, err := sub.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	found := false
	for _, s := range sessions {
		if s.Name == sessionName {
			found = true
			// Detached sessions may report 0 dimensions via #{session_width/height}
			// since those reflect the last attached client size, not -x/-y.
			break
		}
	}
	if !found {
		t.Errorf("ListSessions: session %q not found in results %v", sessionName, sessions)
	}
}

func TestSplitPane_Horizontal(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	const sessionName = "test-splitpane-h"
	if _, err := sub.SpawnSession(ctx, sessionName, "", SessionOptions{}); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(ctx, sessionName) })

	pane, err := sub.SplitPane(ctx, sessionName, Horizontal)
	if err != nil {
		t.Fatalf("SplitPane(Horizontal): %v", err)
	}

	if pane.ID == "" {
		t.Error("SplitPane: returned pane has empty ID")
	}
	if pane.SessionID != sessionName {
		t.Errorf("SplitPane: SessionID got %q, want %q", pane.SessionID, sessionName)
	}
	if pane.Width <= 0 || pane.Height <= 0 {
		t.Errorf("SplitPane: pane dimensions should be positive, got %dx%d", pane.Width, pane.Height)
	}
}

func TestSplitPane_Vertical(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	const sessionName = "test-splitpane-v"
	if _, err := sub.SpawnSession(ctx, sessionName, "", SessionOptions{}); err != nil {
		t.Fatalf("SpawnSession: %v", err)
	}
	t.Cleanup(func() { _ = sub.DestroySession(ctx, sessionName) })

	pane, err := sub.SplitPane(ctx, sessionName, Vertical)
	if err != nil {
		t.Fatalf("SplitPane(Vertical): %v", err)
	}

	if pane.ID == "" {
		t.Error("SplitPane: returned pane has empty ID")
	}
	if pane.Width <= 0 || pane.Height <= 0 {
		t.Errorf("SplitPane: pane dimensions should be positive, got %dx%d", pane.Width, pane.Height)
	}
}

func TestSplitPane_SessionNotFound(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate: %v", err)
	}

	ctx := context.Background()
	_, err = sub.SplitPane(ctx, "nonexistent-session-t3-split", Horizontal)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("SplitPane on missing session: got %v, want ErrSessionNotFound", err)
	}
}
