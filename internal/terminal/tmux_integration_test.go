package terminal

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// TestSpawnAndDestroySession verifies the full spawn → verify → destroy → verify lifecycle.
// Skipped when tmux is not available on PATH.
func TestSpawnAndDestroySession(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available on PATH — skipping integration test")
	}

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate() error: %v", err)
	}

	ctx := context.Background()
	name := fmt.Sprintf("test-%s", strings.ReplaceAll(t.Name(), "/", "-"))

	// Cleanup: ensure the session is destroyed even on test failure.
	t.Cleanup(func() {
		_ = sub.DestroySession(ctx, name)
	})

	// Spawn a detached session with default dimensions.
	sess, err := sub.SpawnSession(ctx, name, "", SessionOptions{})
	if err != nil {
		t.Fatalf("SpawnSession(%q) error: %v", name, err)
	}
	if sess.Name != name {
		t.Errorf("SpawnSession returned Name=%q, want %q", sess.Name, name)
	}
	if sess.Width != 200 || sess.Height != 50 {
		t.Errorf("SpawnSession returned %dx%d, want 200x50", sess.Width, sess.Height)
	}

	// Verify the session exists via tmux list-sessions.
	out, err := sub.run(ctx, "list-sessions", "-F", "#{session_name}")
	if err != nil {
		t.Fatalf("list-sessions error: %v", err)
	}
	if !strings.Contains(out, name) {
		t.Fatalf("session %q not found in tmux list-sessions output:\n%s", name, out)
	}

	// Destroy the session.
	if err := sub.DestroySession(ctx, name); err != nil {
		t.Fatalf("DestroySession(%q) error: %v", name, err)
	}

	// Verify the session no longer exists.
	out, err = sub.run(ctx, "list-sessions", "-F", "#{session_name}")
	if err != nil {
		// list-sessions errors when no sessions exist — that's fine.
		return
	}
	if strings.Contains(out, name) {
		t.Errorf("session %q still present after DestroySession:\n%s", name, out)
	}
}

// TestSpawnSessionCustomDimensions verifies that SessionOptions configures terminal geometry.
func TestSpawnSessionCustomDimensions(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available on PATH — skipping integration test")
	}

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate() error: %v", err)
	}

	ctx := context.Background()
	name := fmt.Sprintf("test-%s", strings.ReplaceAll(t.Name(), "/", "-"))

	t.Cleanup(func() {
		_ = sub.DestroySession(ctx, name)
	})

	sess, err := sub.SpawnSession(ctx, name, "", SessionOptions{Width: 120, Height: 40})
	if err != nil {
		t.Fatalf("SpawnSession(%q) error: %v", name, err)
	}
	if sess.Width != 120 || sess.Height != 40 {
		t.Errorf("SpawnSession returned %dx%d, want 120x40", sess.Width, sess.Height)
	}

	if err := sub.DestroySession(ctx, name); err != nil {
		t.Fatalf("DestroySession(%q) error: %v", name, err)
	}
}

// TestSpawnDuplicateSession verifies ErrSessionExists is returned for a duplicate name.
func TestSpawnDuplicateSession(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available on PATH — skipping integration test")
	}

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate() error: %v", err)
	}

	ctx := context.Background()
	name := fmt.Sprintf("test-%s", strings.ReplaceAll(t.Name(), "/", "-"))

	t.Cleanup(func() {
		_ = sub.DestroySession(ctx, name)
	})

	if _, err := sub.SpawnSession(ctx, name, "", SessionOptions{}); err != nil {
		t.Fatalf("first SpawnSession(%q) error: %v", name, err)
	}

	_, err = sub.SpawnSession(ctx, name, "", SessionOptions{})
	if !errors.Is(err, ErrSessionExists) {
		t.Errorf("duplicate SpawnSession(%q) = %v, want ErrSessionExists", name, err)
	}
}

// TestDestroyNonExistentSession verifies ErrSessionNotFound is returned.
func TestDestroyNonExistentSession(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available on PATH — skipping integration test")
	}

	sub, err := NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate() error: %v", err)
	}

	ctx := context.Background()
	err = sub.DestroySession(ctx, "nonexistent-session-12345")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("DestroySession(nonexistent) = %v, want ErrSessionNotFound", err)
	}
}

// TestDestroySessionRejectsInvalidName verifies DestroySession validates the name.
func TestDestroySessionRejectsInvalidName(t *testing.T) {
	sub := &TmuxSubstrate{tmuxPath: "/usr/bin/tmux"}

	ctx := context.Background()
	for _, name := range []string{"", "has:colon", "has space", "has/slash"} {
		if err := sub.DestroySession(ctx, name); err == nil {
			t.Errorf("DestroySession(%q) = nil, want validation error", name)
		}
	}
}
