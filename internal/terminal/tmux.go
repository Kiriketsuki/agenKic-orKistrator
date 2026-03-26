package terminal

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Compile-time check: TmuxSubstrate must implement Substrate.
var _ Substrate = (*TmuxSubstrate)(nil)

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
// Errors are mapped to typed sentinels via parseTmuxError (case-insensitive).
func (t *TmuxSubstrate) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, t.tmuxPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", parseTmuxError(strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// SpawnSession creates a new detached tmux session with the given name
// running the specified command. Name must be valid per ValidateSessionName.
func (t *TmuxSubstrate) SpawnSession(ctx context.Context, name string, cmd string) (Session, error) {
	if err := ValidateSessionName(name); err != nil {
		return Session{}, err
	}
	if err := ValidateCommand(cmd); err != nil {
		return Session{}, err
	}

	args := []string{"new-session", "-d", "-s", name, "-x", "200", "-y", "50"}
	if cmd != "" {
		args = append(args, cmd)
	}

	if _, err := t.run(ctx, args...); err != nil {
		return Session{}, fmt.Errorf("spawn session %q: %w", name, err)
	}

	return Session{
		Name:        name,
		Width:       200,
		Height:      50,
		WindowCount: 1,
		Attached:    false,
	}, nil
}

// DestroySession kills the named tmux session.
func (t *TmuxSubstrate) DestroySession(ctx context.Context, name string) error {
	if err := ValidateSessionName(name); err != nil {
		return err
	}
	if _, err := t.run(ctx, "kill-session", "-t", name); err != nil {
		return fmt.Errorf("destroy session %q: %w", name, err)
	}
	return nil
}
