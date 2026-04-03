# Feature: Command Injection & Output Capture

## Overview

**Parent Epic**: [Epic #2: Implement Terminal Substrate Layer](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/2)
**GitHub Issue**: [#44 — F2: Command Injection & Output Capture](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/44)

**User Story**: As an orchestrator operator, I want to programmatically send commands to agent terminal sessions and capture their output so that I can control agent execution and observe results without manual terminal interaction.

**Problem**: Once an agent session is spawned (F1), there is no way to inject commands into it or read back what the agent has produced. Without `SendCommand` and `CaptureOutput`, the orchestrator cannot drive agent behaviour or verify agent output — sessions are opaque black boxes.

**Out of Scope**: Session lifecycle management (spawn/destroy — covered by F1), pane layout and splitting (F3), WezTerm Lua adapter (should-have in parent epic, not this feature), streaming/real-time output subscription (future feature), Godot UI rendering of captured output (pixel UI epic).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should `SendCommand` support sending to a specific pane ID, or always target the active pane? | — | [x] Active pane only for F2 MVP. Pane-targeted overloads (`SendCommandToPane`, `CaptureOutputFromPane`) planned for F3 -- see `substrate.go:29` TODO. |
| 2 | Should `CaptureOutput` strip trailing empty lines from the captured buffer? | — | [x] Yes. `strings.TrimRight(out, "\n")` strips trailing newlines to normalize output. |

---

## Scope

### Must-Have

- **SendCommand via `tmux send-keys`** — send an arbitrary command string to a named session's active pane; the command executes in the target shell: verified by capturing output afterwards
  - Enter key sent as a **separate argument** (`"Enter"` as its own `send-keys` arg), not appended to the command string or sent as `\n` — this prevents tmux from interpreting special characters in the command text
- **CaptureOutput via `tmux capture-pane`** — read the last N lines from a named session's active pane using `capture-pane -p -S -{lines}`: captured text matches the agent's terminal output
  - Scrollback math: `-S -{lines}` means "start capture N lines before the current cursor position"; the flag value is negative-lines (e.g., `lines=100` becomes `-S -100`)
- **Error handling: session not found** — `SendCommand` and `CaptureOutput` return `ErrSessionNotFound` when the target session does not exist
- **Error handling: invalid lines** — `CaptureOutput` returns `ErrInvalidLines` when `lines <= 0`
- **Unit tests for SendCommand** — success path (command sent, no error), session-not-found error path
- **Unit tests for CaptureOutput** — success path (output captured), session-not-found error path, invalid-lines error path

### Should-Have

- Command sanitisation: strip or escape characters that could confuse `send-keys` (e.g., bare `;` which tmux interprets as a command separator)
- Configurable pane target: allow specifying a pane ID instead of always targeting the active pane

### Nice-to-Have

- Timestamped capture: return captured output with a timestamp indicating when the capture was taken
- Output diffing: capture only new lines since the last capture call (requires tracking cursor position)

---

## Technical Plan

**Affected Components**:
- `internal/terminal/tmux_command.go` — `SendCommand` implementation
- `internal/terminal/tmux_command_test.go` — unit tests for `SendCommand`
- `internal/terminal/tmux_capture.go` — `CaptureOutput` implementation
- `internal/terminal/tmux_capture_test.go` — unit tests for `CaptureOutput`
- `internal/terminal/tmux.go` — `TmuxSubstrate` struct (already defined in F1; methods added here)
- `internal/terminal/types.go` — sentinel errors `ErrSessionNotFound`, `ErrInvalidLines` (already defined)
- `internal/terminal/tmux_errors.go` — error classification utilities (from F1)

**Data Model Changes**: None. This feature adds methods to the existing `TmuxSubstrate` type.

**API Contracts**:

```go
// SendCommand sends a command to the named session's active pane.
// Internally executes: tmux send-keys -t {session} {cmd} Enter
// Enter is passed as a separate argument to avoid special-char interpretation.
func (t *TmuxSubstrate) SendCommand(ctx context.Context, session string, cmd string) error

// CaptureOutput captures the last N lines from the named session's active pane.
// Internally executes: tmux capture-pane -t {session} -p -S -{lines}
// Returns the captured text as a single string with embedded newlines.
// Trailing newlines are stripped from the output.
func (t *TmuxSubstrate) CaptureOutput(ctx context.Context, session string, lines int) (string, error)
```

**tmux CLI Details**:

| Operation | Command | Key flags | Notes |
|:----------|:--------|:----------|:------|
| Send command | `tmux send-keys -t {session} {cmd} Enter` | `-t` targets session | `Enter` is a **separate argument** after `{cmd}`; tmux resolves it as a keypress. Do NOT use `C-m` or `\n` appended to cmd. |
| Capture output | `tmux capture-pane -t {session} -p -S -{lines}` | `-p` prints to stdout; `-S` sets start line | `-S -{lines}` is relative to cursor: `-S -100` captures from 100 lines above cursor to cursor. `-p` sends to stdout instead of a paste buffer. |

**Error Flow**:

```
SendCommand(ctx, "ghost", "echo hi")
  → tmux send-keys -t ghost "echo hi" Enter
  → tmux exits non-zero: "can't find session: ghost" (or similar)
  → parseTmuxError(stderr) matches session-not-found pattern
  → return ErrSessionNotFound

CaptureOutput(ctx, "agent-01", 0)
  → lines <= 0 → return "", ErrInvalidLines (no tmux call made)

CaptureOutput(ctx, "ghost", 10)
  → tmux capture-pane -t ghost -p -S -10
  → tmux exits non-zero: session not found
  → return "", ErrSessionNotFound
```

**Dependencies**:
- F1 (Substrate Interface & Core Implementation) — provides `TmuxSubstrate` struct, `execTmux` helper, `classifyTmuxError`, sentinel errors
- tmux system binary (checked at substrate construction time per F1)
- `os/exec` (stdlib)

**Risks**:

| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| `send-keys` interprets special characters in command text (`;`, `#`, prefix keys) | Medium | Pass Enter as separate arg; integration tests with special-char commands; consider should-have sanitisation |
| `capture-pane -S` off-by-one on scrollback boundary | Low | Test with exact line counts; verify captured line count matches requested |
| Race between `SendCommand` and `CaptureOutput` — output not yet flushed | Medium | Integration tests use `time.Sleep` or poll-with-retry; document that capture is point-in-time, not synchronous with command completion |
| tmux stderr message format varies across versions | Low | Use substring matching in `classifyTmuxError`, not exact string comparison; test against tmux 3.x+ |

---

## Acceptance Scenarios

```gherkin
Feature: Command Injection & Output Capture
  As an orchestrator operator
  I want to send commands to agent sessions and capture their output
  So that I can drive agent behaviour and observe results programmatically

  Background:
    Given tmux is installed and available on PATH
    And a TmuxSubstrate instance is created

  Rule: Command injection via send-keys

    Scenario: Send a simple command to a running session
      Given a running session "agent-01" with a bash shell
      When SendCommand is called with session "agent-01" and cmd "echo hello-world"
      Then tmux send-keys is invoked with args ["-t", "agent-01", "echo hello-world", "Enter"]
      And no error is returned

    Scenario: Send command with special characters
      Given a running session "agent-01" with a bash shell
      When SendCommand is called with cmd "echo 'foo; bar' && echo done"
      Then the command is passed verbatim to send-keys
      And Enter is sent as a separate argument
      And no error is returned

    Scenario: Send command to non-existent session
      Given no session named "ghost-agent"
      When SendCommand is called for "ghost-agent"
      Then ErrSessionNotFound is returned

  Rule: Output capture via capture-pane

    Scenario: Capture output after command execution
      Given a running session "agent-01" with a bash shell
      When SendCommand is called with "echo hello-world"
      And CaptureOutput is called with session "agent-01" and lines=10
      Then the captured output contains "hello-world"

    Scenario: Capture output with scrollback
      Given a session "agent-01" that has produced 500 lines of output
      When CaptureOutput is called with session "agent-01" and lines=100
      Then exactly the last 100 lines are returned
      And the first returned line is line 401 of the output

    Scenario: Capture output from non-existent session
      Given no session named "ghost-agent"
      When CaptureOutput is called for "ghost-agent" with lines=10
      Then ErrSessionNotFound is returned

    Scenario: Capture output with invalid line count
      Given a running session "agent-01"
      When CaptureOutput is called with lines=0
      Then ErrInvalidLines is returned
      And no tmux command is executed

    Scenario: Capture output with negative line count
      Given a running session "agent-01"
      When CaptureOutput is called with lines=-5
      Then ErrInvalidLines is returned
      And no tmux command is executed

  Rule: Scrollback math correctness

    Scenario Outline: Capture-pane -S flag computed correctly
      Given a running session "agent-01"
      When CaptureOutput is called with lines=<requested>
      Then tmux capture-pane is invoked with -S -<expected_flag>

      Examples:
        | requested | expected_flag |
        | 1         | 1             |
        | 10        | 10            |
        | 100       | 100           |
        | 500       | 500           |
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T2.1 | Implement `SendCommand` in `internal/terminal/tmux_command.go` — invoke `tmux send-keys -t {session} {cmd} Enter` with Enter as separate arg; classify tmux errors via `classifyTmuxError` | High | F1 (TmuxSubstrate, execTmux, classifyTmuxError) | pending |
| T2.1.1 | Unit tests for `SendCommand` in `tmux_command_test.go` — success path, session-not-found error | High | T2.1 | pending |
| T2.2 | Implement `CaptureOutput` in `internal/terminal/tmux_capture.go` — validate `lines > 0` (return `ErrInvalidLines`), invoke `tmux capture-pane -t {session} -p -S -{lines}`, return stdout as string | High | F1 (TmuxSubstrate, execTmux, classifyTmuxError) | pending |
| T2.2.1 | Unit tests for `CaptureOutput` in `tmux_capture_test.go` — success path, scrollback correctness, session-not-found error, invalid-lines error (0 and negative) | High | T2.2 | pending |

---

## Exit Criteria

- [ ] `SendCommand` sends Enter as a separate `send-keys` argument (not `\n`, not `C-m`, not appended)
- [ ] `CaptureOutput` correctly computes `-S -{lines}` flag for scrollback
- [ ] `ErrSessionNotFound` returned for both methods when session does not exist
- [ ] `ErrInvalidLines` returned when `lines <= 0` (without invoking tmux)
- [ ] All unit tests pass: `go test ./internal/terminal/... -run "TestSendCommand|TestCaptureOutput"`
- [ ] No regressions on F1 (SpawnSession, DestroySession) tests
- [ ] `SendCommand` and `CaptureOutput` satisfy their `Substrate` interface contracts defined in `substrate.go`

---

## References

- Parent epic spec: `specs/terminal-substrate-spec.md`
- Substrate interface: `internal/terminal/substrate.go`
- Shared types and sentinel errors: `internal/terminal/types.go`
- Error classification: `internal/terminal/tmux_errors.go`
- GitHub issue: [#44 — F2: Command Injection & Output Capture](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/44)
- Research MOC: `docs/research/Agentic-Orchestrator-MOC.md`
- Terminal multiplexing research: `docs/research/Terminal-Multiplexing-Tmux.md`

---
*Sub-feature of Epic #2 -- Terminal Substrate Layer*
*Authored by: Clault KiperS 4.6*
