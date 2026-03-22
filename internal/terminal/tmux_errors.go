package terminal

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// sessionNameRe matches valid tmux session names: alphanumeric, dash, underscore, dot.
var sessionNameRe = regexp.MustCompile(`^[a-zA-Z0-9_.\-]+$`)

// CheckTmux verifies that the tmux binary is available on PATH.
// Returns ErrTmuxNotFound if tmux is not installed or not on PATH.
func CheckTmux() error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return ErrTmuxNotFound
	}
	return nil
}

// TmuxVersion runs `tmux -V` and returns the version string (e.g. "tmux 3.4").
// Returns ErrTmuxNotFound if tmux is not on PATH.
func TmuxVersion() (string, error) {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return "", ErrTmuxNotFound
	}
	out, err := exec.Command(path, "-V").Output()
	if err != nil {
		return "", fmt.Errorf("tmux -V: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// parseTmuxError maps a tmux stderr string to a typed sentinel error.
// Used internally to convert raw tmux output into structured errors.
func parseTmuxError(stderr string) error {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "session not found") || strings.Contains(s, "can't find session"):
		return ErrSessionNotFound
	case strings.Contains(s, "duplicate session"):
		return ErrSessionExists
	case strings.Contains(s, "create pane failed") || strings.Contains(s, "no room for new pane"):
		return ErrPaneLimit
	default:
		return fmt.Errorf("tmux: %s", strings.TrimSpace(stderr))
	}
}

// ValidateSessionName returns an error if name contains characters that tmux
// does not accept in session names (spaces, colons, or other special chars).
// Valid characters are: alphanumeric, dash (-), underscore (_), dot (.).
func ValidateSessionName(name string) error {
	if name == "" {
		return fmt.Errorf("terminal: session name must not be empty")
	}
	if !sessionNameRe.MatchString(name) {
		return fmt.Errorf("terminal: session name %q contains invalid characters (allowed: a-z, A-Z, 0-9, -, _, .)", name)
	}
	return nil
}
