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
	case strings.Contains(s, "session not found") ||
		strings.Contains(s, "can't find session") ||
		strings.Contains(s, "can't find window") ||
		strings.Contains(s, "can't find pane"):
		return ErrSessionNotFound
	case strings.Contains(s, "duplicate session"):
		return ErrSessionExists
	case strings.Contains(s, "create pane failed") || strings.Contains(s, "no room for new pane"):
		return ErrPaneLimit
	case strings.Contains(s, "no server running") ||
		strings.Contains(s, "no sessions") ||
		strings.Contains(s, "error connecting to"):
		return ErrNoServer
	default:
		return fmt.Errorf("tmux: %s", strings.TrimSpace(stderr))
	}
}

// MaxCommandLen is the maximum allowed length for a command string passed to
// SendCommand or SpawnSession. Commands exceeding this length are rejected.
const MaxCommandLen = 8192

// ValidateCommand checks that cmd does not contain structurally unsafe content.
// It rejects null bytes, dangerous control characters (\x01-\x08, \x0b, \x0c,
// \x0e-\x1f), and commands exceeding MaxCommandLen. Tab (\x09), newline (\x0a),
// and carriage return (\x0d) are permitted as legitimate whitespace.
// An empty command is valid (SpawnSession uses "" for default shell).
func ValidateCommand(cmd string) error {
	if len(cmd) > MaxCommandLen {
		return fmt.Errorf("%w: exceeds maximum length %d (got %d)", ErrInvalidCommand, MaxCommandLen, len(cmd))
	}
	for i := 0; i < len(cmd); i++ {
		b := cmd[i]
		if b == 0 {
			return fmt.Errorf("%w: contains null byte at position %d", ErrInvalidCommand, i)
		}
		// Reject control characters \x01-\x08, \x0b, \x0c, \x0e-\x1f
		// Allow: \x09 (tab), \x0a (newline), \x0d (carriage return)
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			return fmt.Errorf("%w: contains control character 0x%02x at position %d", ErrInvalidCommand, b, i)
		}
	}
	return nil
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
