package terminal

import (
	"fmt"
	"os/exec"
	"strings"
)

// TmuxSubstrate implements Substrate using the tmux terminal multiplexer.
type TmuxSubstrate struct {
	tmuxPath string // absolute path to the tmux binary
}

// NewTmuxSubstrate creates a TmuxSubstrate after verifying tmux is available.
// Returns ErrTmuxNotFound if the tmux binary is not on PATH.
func NewTmuxSubstrate() (*TmuxSubstrate, error) {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return nil, ErrTmuxNotFound
	}
	return &TmuxSubstrate{tmuxPath: path}, nil
}

// run executes a tmux command with the given arguments and returns stdout.
// If tmux exits non-zero, the stderr content is returned as the error message.
func (t *TmuxSubstrate) run(args ...string) (string, error) {
	cmd := exec.Command(t.tmuxPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tmux %s: %s", args[0], strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// SpawnSession creates a new detached tmux session with the given name
// running the specified command.
func (t *TmuxSubstrate) SpawnSession(name string, cmd string) (Session, error) {
	args := []string{"new-session", "-d", "-s", name, "-x", "200", "-y", "50"}
	if cmd != "" {
		args = append(args, cmd)
	}

	if _, err := t.run(args...); err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "duplicate session") {
			return Session{}, ErrSessionExists
		}
		return Session{}, fmt.Errorf("spawn session %q: %w", name, err)
	}

	return Session{
		Name:      name,
		Width:     200,
		Height:    50,
		PaneCount: 1,
		Attached:  false,
	}, nil
}

// DestroySession kills the named tmux session.
func (t *TmuxSubstrate) DestroySession(name string) error {
	if _, err := t.run("kill-session", "-t", name); err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "session not found") ||
			strings.Contains(errStr, "can't find session") {
			return ErrSessionNotFound
		}
		return fmt.Errorf("destroy session %q: %w", name, err)
	}
	return nil
}
