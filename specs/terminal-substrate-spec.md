# Feature: Terminal Substrate Layer

## Overview

**User Story**: As an orchestrator operator, I want each AI agent to execute inside its own terminal pane with programmatic control so that I can observe agent output in real time, inject commands, and manage agent execution environments.

**Problem**: Agents need isolated execution environments with PTY support, scrollback capture, and programmatic spawn/kill. Without this, there's no way to observe, control, or isolate agent processes at the terminal level.

**Out of Scope**: Godot UI rendering of terminals (pixel UI), model selection logic (gateway), the supervisor lifecycle itself (orchestrator core — this feature provides the execution substrate the core spawns agents into).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | | | [ ] |

---

## Scope

### Must-Have
- **tmux session management** — spawn a named session per agent (`agent-{name}`), destroy on agent termination: `tmux list-sessions` shows exactly the running agents
- **Command injection** — send commands to agent panes via `send-keys`: command executes in target pane, output is captured
- **Output capture** — read last N lines from agent pane via `capture-pane`: captured text matches agent's terminal output
- **Pane layout control** — split/arrange panes programmatically for multi-agent views: 4-agent grid layout renders correctly
- **Go client library** wrapping tmux CLI: all tmux operations callable from Go with typed returns and error handling

### Should-Have
- WezTerm Lua adapter as alternative backend (same Go interface, different substrate)
- Anti-flicker output streaming (line-by-line erase, 16ms flush intervals)

### Nice-to-Have
- Inline image support detection (Kitty/iTerm2/Sixel protocols)
- Session persistence across orchestrator restarts

---

## Technical Plan

**Affected Components**:
- `internal/terminal/` — substrate interface + tmux implementation
- `internal/terminal/tmux.go` — tmux CLI wrapper
- `internal/terminal/wezterm.go` — WezTerm Lua adapter (should-have)
- `internal/terminal/types.go` — shared types (Session, Pane, CapturedOutput)

**API Contracts**:
```go
type Substrate interface {
    SpawnSession(name string, cmd string) (Session, error)
    DestroySession(name string) error
    SendCommand(session string, cmd string) error
    CaptureOutput(session string, lines int) (string, error)
    ListSessions() ([]Session, error)
    SplitPane(session string, direction Direction) (Pane, error)
}
```

**Dependencies**: tmux (system binary), `os/exec` (stdlib)

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| tmux `send-keys` Enter handling is fragile | Medium | Separate `Enter` as distinct argument; integration test validates |
| tmux not installed on target system | Low | Check at startup, fail fast with clear message |
| Platform constraint: no tmux on Windows | High | Windows is out of scope for terminal substrate; document clearly |

---

## Acceptance Scenarios

```gherkin
Feature: Terminal Substrate Layer
  As an orchestrator operator
  I want programmatic terminal pane control per agent
  So that agents execute in isolated, observable environments

  Background:
    Given tmux is installed and available on PATH

  Rule: Session lifecycle management

    Scenario: Spawn a named agent session
      Given no tmux session named "agent-codegen-01" exists
      When SpawnSession is called with name "agent-codegen-01" and cmd "bash"
      Then a detached tmux session "agent-codegen-01" is created
      And the session runs a bash shell

    Scenario: Destroy an agent session
      Given a running tmux session "agent-codegen-01"
      When DestroySession is called with name "agent-codegen-01"
      Then the session is terminated
      And tmux list-sessions no longer includes "agent-codegen-01"

  Rule: Command injection and output capture

    Scenario: Send command and capture output
      Given a running session "agent-01" with a bash shell
      When SendCommand is called with "echo hello-world"
      And CaptureOutput is called with lines=10
      Then the captured output contains "hello-world"

    Scenario: Capture output with scrollback
      Given a session that has produced 500 lines of output
      When CaptureOutput is called with lines=100
      Then exactly the last 100 lines are returned

  Rule: Multi-pane layout

    Scenario: Create a 2x2 agent grid
      Given a running session "workspace"
      When SplitPane is called 3 times (horizontal, vertical, vertical)
      Then 4 panes are visible in the session
      And each pane can receive independent commands

  Rule: Error handling

    Scenario: Spawn fails when tmux is not installed
      Given tmux is not on PATH
      When SpawnSession is called
      Then an error is returned with message containing "tmux not found"

    Scenario: Send command to non-existent session
      Given no session named "ghost-agent"
      When SendCommand is called for "ghost-agent"
      Then an error is returned indicating the session does not exist
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T1   | Define `Substrate` interface and shared types | High | None | pending |
| T2   | Implement tmux wrapper: SpawnSession, DestroySession | High | T1 | pending |
| T2.1 | Implement SendCommand with correct Enter handling | High | T2 | pending |
| T2.2 | Implement CaptureOutput with scrollback support | High | T2 | pending |
| T3   | Implement ListSessions and SplitPane | High | T2 | pending |
| T4   | Error handling: tmux-not-found, invalid session, pane limits | High | T2 | pending |
| T5   | Integration tests: full spawn -> command -> capture -> destroy cycle | High | T3 | pending |
| T6   | Wire into orchestrator core: supervisor spawns agents into tmux sessions | High | T5 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI (requires tmux in CI environment)
- [ ] No regressions on orchestrator core
- [ ] Go interface is substrate-agnostic (WezTerm can be swapped without changing callers)

---

## References

- Research MOC: `docs/research/Agentic-Orchestrator-MOC.md`
- Terminal multiplexing: `docs/research/Terminal-Multiplexing-Tmux.md`
- Repositionable terminals: `docs/research/Repositionable-Terminals.md`
- Terminal infrastructure patterns: `docs/research/patterns/Terminal-Infrastructure-for-Agents.md`

---
*Authored by: Clault KiperS 4.6*
