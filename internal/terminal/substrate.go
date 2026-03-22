// Package terminal provides a substrate-agnostic interface for managing
// terminal sessions used as agent execution environments.
package terminal

// Substrate defines the interface for programmatic terminal control.
// Implementations wrap specific terminal multiplexers (tmux, WezTerm, etc.).
type Substrate interface {
	// SpawnSession creates a new detached terminal session running cmd.
	// The session is identified by name for subsequent operations.
	SpawnSession(name string, cmd string) (Session, error)

	// DestroySession terminates the named session and all its panes.
	DestroySession(name string) error

	// SendCommand sends a command string to the named session's active pane.
	// The command is followed by Enter to execute it.
	SendCommand(session string, cmd string) error

	// CaptureOutput reads the last N lines from the named session's active pane.
	CaptureOutput(session string, lines int) (string, error)

	// ListSessions returns all sessions managed by this substrate.
	ListSessions() ([]Session, error)

	// SplitPane splits the active pane in the named session.
	// Returns the newly created pane.
	SplitPane(session string, direction Direction) (Pane, error)
}
