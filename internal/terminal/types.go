package terminal

import "errors"

// Direction specifies how a pane split is oriented.
type Direction int

const (
	Horizontal Direction = iota // split left-right
	Vertical                    // split top-bottom
)

// Session represents a terminal multiplexer session.
type Session struct {
	Name        string // unique session identifier (e.g. "agent-codegen-01")
	Width       int    // columns; 0 for detached sessions returned by ListSessions
	Height      int    // rows; 0 for detached sessions returned by ListSessions
	WindowCount int    // number of windows in the session
	Attached    bool   // whether a client is attached
}

// Pane represents a single pane within a session.
type Pane struct {
	ID        string // pane identifier (e.g. "%0")
	SessionID string // owning session name
	Width     int
	Height    int
	Active    bool // whether this pane has focus
}

// Sentinel errors returned by Substrate implementations.
var (
	// ErrTmuxNotFound is returned when the tmux binary is not on PATH.
	ErrTmuxNotFound = errors.New("terminal: tmux not found on PATH")

	// ErrSessionNotFound is returned when the target session does not exist.
	ErrSessionNotFound = errors.New("terminal: session not found")

	// ErrSessionExists is returned when creating a session that already exists.
	ErrSessionExists = errors.New("terminal: session already exists")

	// ErrPaneLimit is returned when a split would exceed the substrate's pane limit.
	ErrPaneLimit = errors.New("terminal: pane limit reached")

	// ErrInvalidLines is returned when CaptureOutput receives a non-positive line count.
	ErrInvalidLines = errors.New("terminal: lines must be positive")

	// ErrNoServer is returned when no tmux server is running (no sessions exist).
	ErrNoServer = errors.New("terminal: no tmux server running")
)
