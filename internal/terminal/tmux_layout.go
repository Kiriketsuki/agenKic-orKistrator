package terminal

import (
	"fmt"
	"strconv"
	"strings"
)

// ListSessions returns all sessions currently managed by the tmux server.
// If no tmux server is running (no sessions exist), an empty slice is returned.
func (t *TmuxSubstrate) ListSessions() ([]Session, error) {
	out, err := t.run(
		"list-sessions",
		"-F", "#{session_name}:#{session_width}:#{session_height}:#{session_windows}:#{session_attached}",
	)
	if err != nil {
		// tmux exits non-zero when no server is running or no sessions exist.
		errStr := err.Error()
		if strings.Contains(errStr, "no server running") ||
			strings.Contains(errStr, "no sessions") {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var sessions []Session
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		s, err := parseSessionLine(line)
		if err != nil {
			return nil, fmt.Errorf("list sessions: parse %q: %w", line, err)
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// parseSessionLine parses a single line from tmux list-sessions -F output.
// Expected format: name:width:height:windows:attached
func parseSessionLine(line string) (Session, error) {
	parts := strings.SplitN(line, ":", 5)
	if len(parts) != 5 {
		return Session{}, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	width, err := strconv.Atoi(parts[1])
	if err != nil {
		return Session{}, fmt.Errorf("width: %w", err)
	}
	height, err := strconv.Atoi(parts[2])
	if err != nil {
		return Session{}, fmt.Errorf("height: %w", err)
	}
	paneCount, err := strconv.Atoi(parts[3])
	if err != nil {
		return Session{}, fmt.Errorf("pane count: %w", err)
	}

	return Session{
		Name:      parts[0],
		Width:     width,
		Height:    height,
		PaneCount: paneCount,
		Attached:  parts[4] == "1",
	}, nil
}

// SplitPane splits the active pane in the named session and returns the new pane.
// Direction Horizontal splits left-right (-h); Vertical splits top-bottom (-v).
func (t *TmuxSubstrate) SplitPane(session string, direction Direction) (Pane, error) {
	flag := "-v"
	if direction == Horizontal {
		flag = "-h"
	}

	out, err := t.run(
		"split-window", flag,
		"-t", session,
		"-P", "-F", "#{pane_id}:#{session_name}:#{pane_width}:#{pane_height}:#{pane_active}",
	)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "session not found") ||
			strings.Contains(errStr, "can't find session") {
			return Pane{}, ErrSessionNotFound
		}
		if strings.Contains(errStr, "create pane failed") {
			return Pane{}, ErrPaneLimit
		}
		return Pane{}, fmt.Errorf("split pane in session %q: %w", session, err)
	}

	line := strings.TrimSpace(out)
	pane, err := parsePaneLine(line)
	if err != nil {
		return Pane{}, fmt.Errorf("split pane: parse %q: %w", line, err)
	}
	return pane, nil
}

// parsePaneLine parses a single line from tmux split-window -P -F output.
// Expected format: pane_id:session_name:width:height:active
func parsePaneLine(line string) (Pane, error) {
	parts := strings.SplitN(line, ":", 5)
	if len(parts) != 5 {
		return Pane{}, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	width, err := strconv.Atoi(parts[2])
	if err != nil {
		return Pane{}, fmt.Errorf("width: %w", err)
	}
	height, err := strconv.Atoi(parts[3])
	if err != nil {
		return Pane{}, fmt.Errorf("height: %w", err)
	}

	return Pane{
		ID:        parts[0],
		SessionID: parts[1],
		Width:     width,
		Height:    height,
		Active:    parts[4] == "1",
	}, nil
}
