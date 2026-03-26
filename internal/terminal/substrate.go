// Package terminal provides a substrate-agnostic interface for managing
// terminal sessions used as agent execution environments.
package terminal

import "context"

// Substrate defines the interface for programmatic terminal control.
// Implementations wrap specific terminal multiplexers (tmux, WezTerm, etc.).
type Substrate interface {
	// SpawnSession creates a new detached terminal session running cmd.
	// The session name must pass ValidateSessionName.
	SpawnSession(ctx context.Context, name string, cmd string) (Session, error)

	// DestroySession terminates the named session and all its panes.
	DestroySession(ctx context.Context, name string) error

	// SendCommand sends a command string to the named session's active pane.
	// The command is followed by Enter to execute it.
	SendCommand(ctx context.Context, session string, cmd string) error

	// CaptureOutput reads the last N lines from the named session's active pane.
	CaptureOutput(ctx context.Context, session string, lines int) (string, error)

	// ListSessions returns all sessions managed by this substrate.
	ListSessions(ctx context.Context) ([]Session, error)

	// SplitPane splits the active pane in the named session.
	// Returns the newly created pane.
	// TODO: pane-targeted SendCommand/CaptureOutput overloads are planned;
	// the returned Pane.ID will be the addressing mechanism.
	SplitPane(ctx context.Context, session string, direction Direction) (Pane, error)
}
