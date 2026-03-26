# Feature: Layout Management & Error Handling

## Overview

**Parent Epic**: [Terminal Substrate Layer](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/2) (Epic #2)
**Issue**: [#48 — F3: Layout Management & Error Handling](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/48)

**User Story**: As an orchestrator operator, I want to list all active agent sessions and split panes programmatically so that I can build multi-agent grid layouts, and I want robust error handling so that all tmux failure modes surface as typed Go errors I can match on.

**Problem**: Without `ListSessions`, the orchestrator cannot discover which agents are currently running. Without `SplitPane`, multi-agent grid views are impossible. Without structured error handling, callers must parse raw tmux stderr strings to distinguish recoverable from fatal failures — leading to fragile, inconsistent error paths.

**Out of Scope**: Session lifecycle (SpawnSession/DestroySession — covered by F1), command injection and output capture (F2), WezTerm Lua adapter (should-have from epic), Godot UI rendering, integration tests covering the full spawn-to-destroy cycle (F4).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | | | [ ] |

---

## Scope

### Must-Have

**ListSessions** — enumerate all active tmux sessions with metadata:
- Invoke `tmux list-sessions -F "#{session_name}\t#{session_width}\t#{session_height}\t#{session_windows}\t#{session_attached}"` with tab-delimited format string
- Parse each output line via `parseSessionLine()` into a `Session` struct (Name, Width, Height, PaneCount, Attached)
- Return an empty `[]Session{}` (not nil, not error) when tmux reports "no server running" or "no sessions"
- Use lenient dimension parsing: `strconv.Atoi` failures for width/height yield 0 instead of error (detached sessions may report empty or non-numeric dimensions when no client is attached)

**SplitPane** — split the active pane in a session to build multi-pane layouts:
- Invoke `tmux split-window` with `-h` for Horizontal (left-right) or `-v` for Vertical (top-bottom) split
- Target the session via `-t <session>`
- Use `-P -F "#{pane_id}\t#{session_name}\t#{pane_width}\t#{pane_height}\t#{pane_active}"` to capture new pane metadata
- Parse output via `parsePaneLine()` into a `Pane` struct (ID, SessionID, Width, Height, Active)
- Map tmux error strings to sentinel errors: session/window/pane not found -> `ErrSessionNotFound`; pane limit -> `ErrPaneLimit`

**Error handling utilities** — centralized tmux error classification and validation:
- `CheckTmux() error` — verifies tmux binary is on PATH via `exec.LookPath`; returns `ErrTmuxNotFound` if absent
- `TmuxVersion() (string, error)` — runs `tmux -V`, returns version string (e.g. "tmux 3.4"); returns `ErrTmuxNotFound` if absent
- `parseTmuxError(stderr string) error` — maps raw tmux stderr to sentinel errors using case-insensitive substring matching:
  - "session not found" or "can't find session" or "can't find window" or "can't find pane" -> `ErrSessionNotFound`
  - "duplicate session" -> `ErrSessionExists`
  - "create pane failed" or "no room for new pane" -> `ErrPaneLimit`
  - Anything else -> generic `fmt.Errorf("tmux: %s", stderr)`
- `ValidateSessionName(name string) error` — rejects empty names and names containing characters outside `^[a-zA-Z0-9_.\-]+$`

**Sentinel errors** — typed errors in `types.go` for `errors.Is` matching:
- `ErrTmuxNotFound` — tmux binary not on PATH
- `ErrSessionNotFound` — target session does not exist
- `ErrSessionExists` — session name already taken
- `ErrPaneLimit` — tmux cannot create another pane (terminal too small or server limit)

**Unit tests** — pure parser and utility tests (no tmux required):
- `parseSessionLine`: valid 5-field line, attached session, malformed line (wrong field count), non-numeric width (lenient → 0)
- `parsePaneLine`: valid 5-field line, malformed line (wrong field count)
- `parseTmuxError`: session-not-found variants, duplicate-session, pane-limit variants, generic/unknown error
- `ValidateSessionName`: valid patterns (alphanumeric, dash, underscore, dot), invalid patterns (empty, space, colon, slash, special chars)
- `CheckTmux`: returns nil when tmux present (skip if absent)
- `TmuxVersion`: returns "tmux " prefix when present (skip if absent)

### Should-Have

- **Integration tests for ListSessions**: spawn a real session, verify it appears in `ListSessions()` output, verify dimensions and pane count
- **Integration tests for SplitPane**: spawn session, split horizontally and vertically, verify pane ID and dimensions are positive, verify `ErrSessionNotFound` on non-existent session

### Nice-to-Have

- `parseTmuxError` extended with version-specific error message patterns for tmux < 3.0 compatibility
- Pane enumeration: `ListPanes(session string) ([]Pane, error)` for introspecting all panes in a session

---

## Technical Plan

**Affected Components**:
- `internal/terminal/tmux_layout.go` — `ListSessions()`, `SplitPane()`, `parseSessionLine()`, `parsePaneLine()`
- `internal/terminal/tmux_errors.go` — `CheckTmux()`, `TmuxVersion()`, `parseTmuxError()`, `ValidateSessionName()`
- `internal/terminal/types.go` — `Session`, `Pane`, `Direction`, sentinel error vars
- `internal/terminal/tmux_layout_test.go` — unit + integration tests for layout functions
- `internal/terminal/tmux_errors_test.go` — unit tests for error utilities

**API Contracts**:

```go
// Layout functions (on TmuxSubstrate receiver)

// ListSessions returns all sessions managed by the tmux server.
// Returns empty []Session{} when no tmux server is running.
func (t *TmuxSubstrate) ListSessions() ([]Session, error)

// SplitPane splits the active pane in session. Direction Horizontal → -h, Vertical → -v.
// Returns ErrSessionNotFound if session does not exist.
// Returns ErrPaneLimit if tmux cannot create another pane.
func (t *TmuxSubstrate) SplitPane(session string, direction Direction) (Pane, error)

// Internal parsers (unexported)
func parseSessionLine(line string) (Session, error) // tab-delimited, 5 fields
func parsePaneLine(line string) (Pane, error)       // tab-delimited, 5 fields

// Error utilities (exported)
func CheckTmux() error                   // ErrTmuxNotFound if absent
func TmuxVersion() (string, error)       // "tmux 3.4" or ErrTmuxNotFound
func parseTmuxError(stderr string) error // stderr → sentinel error
func ValidateSessionName(name string) error // regex validation
```

**tmux CLI Format Strings**:

| Function | tmux Command | Format String |
|:---------|:-------------|:--------------|
| `ListSessions` | `list-sessions -F` | `#{session_name}\t#{session_width}\t#{session_height}\t#{session_windows}\t#{session_attached}` |
| `SplitPane` | `split-window {-h\|-v} -t <session> -P -F` | `#{pane_id}\t#{session_name}\t#{pane_width}\t#{pane_height}\t#{pane_active}` |

**Error Pattern Matching Table**:

| tmux stderr pattern | Sentinel error | Match method |
|:--------------------|:---------------|:-------------|
| `session not found` | `ErrSessionNotFound` | case-insensitive `strings.Contains` |
| `can't find session` | `ErrSessionNotFound` | case-insensitive `strings.Contains` |
| `can't find window` | `ErrSessionNotFound` | case-insensitive `strings.Contains` |
| `can't find pane` | `ErrSessionNotFound` | case-insensitive `strings.Contains` |
| `duplicate session` | `ErrSessionExists` | case-insensitive `strings.Contains` |
| `create pane failed` | `ErrPaneLimit` | case-insensitive `strings.Contains` |
| `no room for new pane` | `ErrPaneLimit` | case-insensitive `strings.Contains` |

**Dependencies**: F1 (Substrate interface, `TmuxSubstrate` struct, `run()` helper, `SpawnSession`/`DestroySession`)

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Detached sessions report empty/non-numeric dimensions | High | Lenient `strconv.Atoi` — treat parse failure as 0, not error |
| tmux error messages vary across versions (2.x vs 3.x) | Medium | Match multiple known patterns; use `strings.ToLower` for case-insensitive matching |
| `split-window` fails silently if terminal is too small | Low | Detect "create pane failed" / "no room for new pane" and return `ErrPaneLimit` |
| `list-sessions` exits non-zero when no server running | High | Check stderr for "no server running" / "no sessions" and return empty slice instead of error |

---

## Acceptance Scenarios

```gherkin
Feature: Layout Management & Error Handling
  As an orchestrator operator
  I want to list sessions, split panes, and get typed errors
  So that I can build multi-agent layouts and handle failures gracefully

  Background:
    Given tmux is installed and available on PATH

  Rule: Session listing

    Scenario: List sessions returns active sessions
      Given a running tmux session "agent-01"
      When ListSessions is called
      Then the result contains a Session with Name "agent-01"
      And PaneCount is at least 1

    Scenario: List sessions returns empty slice when no server running
      Given no tmux server is running
      When ListSessions is called
      Then an empty slice is returned
      And no error is returned

    Scenario: List sessions handles detached sessions with unknown dimensions
      Given a detached tmux session "agent-detached" with no attached client
      When ListSessions is called
      Then the result contains a Session with Name "agent-detached"
      And Width and Height may be 0
      And Attached is false

  Rule: Pane splitting

    Scenario: Split pane horizontally
      Given a running session "workspace"
      When SplitPane is called with direction Horizontal
      Then a Pane is returned with a non-empty ID
      And the Pane's SessionID is "workspace"
      And Width and Height are positive

    Scenario: Split pane vertically
      Given a running session "workspace"
      When SplitPane is called with direction Vertical
      Then a Pane is returned with a non-empty ID
      And Width and Height are positive

    Scenario: Create a 2x2 agent grid
      Given a running session "workspace"
      When SplitPane is called 3 times (horizontal, vertical, vertical)
      Then 4 panes are visible in the session
      And each pane can receive independent commands

    Scenario: Split pane on non-existent session
      Given no session named "ghost-agent"
      When SplitPane is called for "ghost-agent"
      Then ErrSessionNotFound is returned

    Scenario: Split pane when terminal is too small
      Given a session with a very small pane
      When SplitPane is called and tmux reports "create pane failed"
      Then ErrPaneLimit is returned

  Rule: tmux availability checks

    Scenario: Spawn fails when tmux is not installed
      Given tmux is not on PATH
      When CheckTmux is called
      Then ErrTmuxNotFound is returned

    Scenario: tmux version is reported
      Given tmux is installed
      When TmuxVersion is called
      Then a string starting with "tmux " is returned

  Rule: Error classification

    Scenario Outline: parseTmuxError maps stderr to sentinel errors
      When parseTmuxError is called with "<stderr>"
      Then the result is <sentinel>

      Examples:
        | stderr                       | sentinel            |
        | session not found            | ErrSessionNotFound  |
        | can't find session myagent   | ErrSessionNotFound  |
        | can't find window 0          | ErrSessionNotFound  |
        | can't find pane %1           | ErrSessionNotFound  |
        | duplicate session: agent-01  | ErrSessionExists    |
        | create pane failed           | ErrPaneLimit        |
        | no room for new pane         | ErrPaneLimit        |
        | some unknown tmux error      | generic error       |

  Rule: Session name validation

    Scenario Outline: ValidateSessionName accepts valid names
      When ValidateSessionName is called with "<name>"
      Then no error is returned

      Examples:
        | name              |
        | agent01           |
        | agent-codegen-01  |
        | Agent_01          |
        | my.session        |

    Scenario Outline: ValidateSessionName rejects invalid names
      When ValidateSessionName is called with "<name>"
      Then an error is returned

      Examples:
        | name        |
        | (empty)     |
        | has space   |
        | has:colon   |
        | has/slash   |
        | has@at      |
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T3 | Implement `ListSessions` in `tmux_layout.go`: `list-sessions -F` with tab-delimited parsing, empty-slice fallback for no server | High | F1 (T2) | pending |
| T3.1 | Implement `SplitPane` in `tmux_layout.go`: `split-window -h/-v`, `-P -F` pane output parsing, error mapping | High | T3 | pending |
| T3.2 | Implement `parseSessionLine` and `parsePaneLine` internal parsers with lenient dimension handling | High | T3 | pending |
| T4 | Implement `CheckTmux` and `TmuxVersion` in `tmux_errors.go` | High | F1 (T1) | pending |
| T4.1 | Implement `parseTmuxError` in `tmux_errors.go`: case-insensitive pattern matching for 7 known error strings | High | T4 | pending |
| T4.2 | Implement `ValidateSessionName` in `tmux_errors.go`: regex-based validation against `^[a-zA-Z0-9_.\-]+$` | High | T4 | pending |
| T4.3 | Unit tests for all parsers and error utilities in `tmux_layout_test.go` and `tmux_errors_test.go` | High | T3.2, T4.2 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass (unit tests run without tmux; integration tests skip cleanly when tmux absent)
- [ ] `parseSessionLine` handles: valid 5-field line, attached session, malformed line, non-numeric dimensions
- [ ] `parsePaneLine` handles: valid 5-field line, malformed line
- [ ] `parseTmuxError` correctly maps all 7 known stderr patterns to their sentinel errors
- [ ] `ValidateSessionName` rejects empty, space, colon, slash, and special-character names
- [ ] `ListSessions` returns `[]Session{}` (not nil) when no tmux server is running
- [ ] `SplitPane` returns `ErrSessionNotFound` for missing sessions and `ErrPaneLimit` for pane creation failure
- [ ] No regressions on F1 (SpawnSession, DestroySession) or F2 (SendCommand, CaptureOutput)

---

## References

- Parent epic spec: `specs/terminal-substrate-spec.md`
- Issue: [#48 — F3: Layout Management & Error Handling](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/48)
- Implementation files: `internal/terminal/tmux_layout.go`, `internal/terminal/tmux_errors.go`
- Type definitions: `internal/terminal/types.go`
- Research: `docs/research/Terminal-Multiplexing-Tmux.md`
- Terminal infrastructure patterns: `docs/research/patterns/Terminal-Infrastructure-for-Agents.md`

---
*Sub-feature of Epic #2 -- Terminal Substrate Layer*

---
*Authored by: Clault KiperS 4.6*
