package terminal

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ListSessions returns all sessions currently managed by the tmux server.
// If no tmux server is running (no sessions exist), an empty slice is returned.
func (t *TmuxSubstrate) ListSessions(ctx context.Context) ([]Session, error) {
	out, err := t.run(ctx,
		"list-sessions",
		"-F", "#{session_name}\t#{session_width}\t#{session_height}\t#{session_windows}\t#{session_attached}",
	)
	if err != nil {
		// tmux exits non-zero when no server is running or no sessions exist.
		// parseTmuxError maps these to a generic error; we treat them as empty.
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
// Expected format: name\twidth\theight\twindows\tattached (tab-delimited)
func parseSessionLine(line string) (Session, error) {
	parts := strings.SplitN(line, "\t", 5)
	if len(parts) != 5 {
		return Session{}, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	width, _ := strconv.Atoi(parts[1])  // 0 if empty (detached, no client)
	height, _ := strconv.Atoi(parts[2]) // 0 if empty
	windowCount, _ := strconv.Atoi(parts[3])

	return Session{
		Name:        parts[0],
		Width:       width,
		Height:      height,
		WindowCount: windowCount,
		Attached:    parts[4] == "1",
	}, nil
}

// SplitPane splits the active pane in the named session and returns the new pane.
// Direction Horizontal splits left-right (-h); Vertical splits top-bottom (-v).
func (t *TmuxSubstrate) SplitPane(ctx context.Context, session string, direction Direction) (Pane, error) {
	flag := "-v"
	if direction == Horizontal {
		flag = "-h"
	}

	out, err := t.run(ctx,
		"split-window", flag,
		"-t", session,
		"-P", "-F", "#{pane_id}\t#{session_name}\t#{pane_width}\t#{pane_height}\t#{pane_active}",
	)
	if err != nil {
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
// Expected format: pane_id\tsession_name\twidth\theight\tactive (tab-delimited)
func parsePaneLine(line string) (Pane, error) {
	parts := strings.SplitN(line, "\t", 5)
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
