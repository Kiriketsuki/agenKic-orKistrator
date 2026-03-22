# Feature: Substrate Interface & Core Implementation

## Overview

**Parent Epic**: [Epic #2: Implement Terminal Substrate Layer](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/2)
**Issue**: [#42 — F1: Substrate Interface & Core Implementation](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/42)

**User Story**: As an orchestrator operator, I want a Go interface that abstracts terminal substrate operations and a tmux implementation that can spawn and destroy named sessions so that agents have isolated execution environments with programmatic lifecycle control.

**Problem**: Without a defined `Substrate` interface, higher-level orchestrator code cannot manage agent execution environments in a backend-agnostic way. Without a tmux implementation of spawn/destroy, agents have no isolated terminal sessions to run in.

**Out of Scope**:
- Command injection (`SendCommand`) — covered by F2
- Output capture (`CaptureOutput`) — covered by F2
- Layout management (`ListSessions`, `SplitPane`) — covered by F3
- Error handling utilities (classification, retry logic) — covered by F3
- Integration tests (full lifecycle) — covered by F4
- WezTerm adapter — epic should-have, not this feature
- Godot UI rendering of terminals
- Windows platform support

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should session names be validated against tmux naming rules (no dots, no colons) at the Go layer, or rely on tmux to reject invalid names? | spec | [ ] |

---

## Scope

### Must-Have
- **`Substrate` interface** (`internal/terminal/substrate.go`): defines 6 methods (`SpawnSession`, `DestroySession`, `SendCommand`, `CaptureOutput`, `ListSessions`, `SplitPane`) with typed Go signatures — callers compile against the interface, not the tmux implementation
- **Shared types** (`internal/terminal/types.go`): `Session` struct (Name, CreatedAt), `Pane` struct (ID, SessionName), `Direction` type (`Horizontal`, `Vertical`), and 5 sentinel errors (`ErrTmuxNotFound`, `ErrSessionNotFound`, `ErrSessionExists`, `ErrPaneLimit`, `ErrInvalidLines`) — all exported and usable by callers without importing tmux-specific code
- **`TmuxSubstrate` constructor** (`internal/terminal/tmux.go`): `NewTmuxSubstrate()` verifies tmux binary is on PATH via `exec.LookPath("tmux")`; returns `ErrTmuxNotFound` if absent — fail-fast at startup, not at first use
- **`SpawnSession`**: creates a detached tmux session via `tmux new-session -d -s <name> <cmd>`; returns `ErrSessionExists` if a session with that name already exists; returns a populated `Session` struct on success
- **`DestroySession`**: kills a tmux session via `tmux kill-session -t <name>`; returns `ErrSessionNotFound` if the session does not exist
- **Exec helper** (`internal/terminal/tmux.go`): private method `execTmux(args ...string) (string, error)` that wraps `os/exec.Command("tmux", args...)`, captures combined stdout/stderr, and returns trimmed output — single point for all tmux CLI calls

### Should-Have
- Session name validation (reject names containing `.`, `:`, or whitespace) before calling tmux
- Configurable tmux binary path (default: `exec.LookPath` result) for non-standard installations

### Nice-to-Have
- `TmuxSubstrate` implements `io.Closer` for cleanup of all sessions on shutdown

---

## Technical Plan

**Affected Components**:
- `internal/terminal/substrate.go` — `Substrate` interface definition (new file)
- `internal/terminal/types.go` — `Session`, `Pane`, `Direction`, sentinel errors (new file)
- `internal/terminal/tmux.go` — `TmuxSubstrate` struct, constructor, `SpawnSession`, `DestroySession`, `execTmux` helper (new file)

**Data Model Changes**: None — no database or persistent state. All state lives in tmux server process.

**API Contracts**:
```go
// substrate.go
package terminal

// Substrate defines programmatic control over terminal sessions.
// Implementations wrap a specific terminal multiplexer (tmux, WezTerm, etc.).
type Substrate interface {
    SpawnSession(name string, cmd string) (Session, error)
    DestroySession(name string) error
    SendCommand(session string, cmd string) error
    CaptureOutput(session string, lines int) (string, error)
    ListSessions() ([]Session, error)
    SplitPane(session string, direction Direction) (Pane, error)
}
```

```go
// types.go
package terminal

import (
    "errors"
    "time"
)

type Session struct {
    Name      string
    CreatedAt time.Time
}

type Pane struct {
    ID          string
    SessionName string
}

type Direction int

const (
    Horizontal Direction = iota
    Vertical
)

var (
    ErrTmuxNotFound    = errors.New("tmux not found on PATH")
    ErrSessionNotFound = errors.New("session not found")
    ErrSessionExists   = errors.New("session already exists")
    ErrPaneLimit       = errors.New("pane limit reached")
    ErrInvalidLines    = errors.New("lines must be positive")
)
```

```go
// tmux.go — constructor and core methods
package terminal

import "os/exec"

type TmuxSubstrate struct {
    tmuxPath string
}

// NewTmuxSubstrate verifies tmux is available and returns a ready substrate.
func NewTmuxSubstrate() (*TmuxSubstrate, error) {
    path, err := exec.LookPath("tmux")
    if err != nil {
        return nil, ErrTmuxNotFound
    }
    return &TmuxSubstrate{tmuxPath: path}, nil
}
```

**tmux CLI commands used**:
| Operation | Command | Notes |
|:----------|:--------|:------|
| Spawn | `tmux new-session -d -s <name> <cmd>` | `-d` = detached |
| Destroy | `tmux kill-session -t <name>` | |
| Check exists | `tmux has-session -t <name>` | Exit code 0 = exists |

**Dependencies**:
- `tmux` — system binary, must be on PATH
- `os/exec` — Go stdlib, for subprocess execution
- `errors` — Go stdlib, for sentinel errors
- `time` — Go stdlib, for `Session.CreatedAt`

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| tmux not installed on target system | Low | `NewTmuxSubstrate()` checks at construction time, returns `ErrTmuxNotFound` |
| Race condition: session created between has-session check and new-session | Low | Parse tmux stderr for "duplicate session" and return `ErrSessionExists` instead of double-checking |
| tmux binary path changes between constructor and method calls | Very Low | Store resolved path at construction; document that the substrate is not safe to use across tmux reinstalls |
| Session name conflicts with tmux internal naming | Medium | Should-have: validate names before passing to tmux CLI |

---

## Acceptance Scenarios

```gherkin
Feature: Substrate Interface & Core Implementation
  As an orchestrator operator
  I want a substrate interface with tmux-backed session lifecycle
  So that agents can be spawned into and destroyed from isolated terminal sessions

  Background:
    Given tmux is installed and available on PATH

  Rule: Interface definition

    Scenario: Substrate interface is implementable
      Given the Substrate interface is defined in internal/terminal/substrate.go
      When a new struct implements all 6 methods with correct signatures
      Then it compiles without errors
      And callers can use the interface type without importing the implementation

  Rule: Type definitions

    Scenario: Shared types are usable without implementation imports
      Given types are defined in internal/terminal/types.go
      When a caller imports only the terminal package
      Then Session, Pane, Direction, and all sentinel errors are accessible
      And Direction has Horizontal and Vertical constants

  Rule: Constructor — tmux discovery

    Scenario: Constructor succeeds when tmux is on PATH
      Given tmux is installed and available on PATH
      When NewTmuxSubstrate is called
      Then a non-nil TmuxSubstrate is returned
      And no error is returned

    Scenario: Constructor fails when tmux is not on PATH
      Given tmux is not on PATH
      When NewTmuxSubstrate is called
      Then a nil TmuxSubstrate is returned
      And the error is ErrTmuxNotFound

  Rule: Session lifecycle — spawn

    Scenario: Spawn a named agent session
      Given no tmux session named "agent-codegen-01" exists
      When SpawnSession is called with name "agent-codegen-01" and cmd "bash"
      Then a detached tmux session "agent-codegen-01" is created
      And the returned Session has Name "agent-codegen-01"
      And the returned Session has a non-zero CreatedAt

    Scenario: Spawn fails for duplicate session name
      Given a running tmux session "agent-codegen-01"
      When SpawnSession is called with name "agent-codegen-01" and cmd "bash"
      Then ErrSessionExists is returned
      And only one session named "agent-codegen-01" exists

  Rule: Session lifecycle — destroy

    Scenario: Destroy an existing agent session
      Given a running tmux session "agent-codegen-01"
      When DestroySession is called with name "agent-codegen-01"
      Then the session is terminated
      And tmux list-sessions no longer includes "agent-codegen-01"

    Scenario: Destroy fails for non-existent session
      Given no tmux session named "ghost-agent" exists
      When DestroySession is called with name "ghost-agent"
      Then ErrSessionNotFound is returned
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T1   | Define `Substrate` interface in `internal/terminal/substrate.go` | High | None | pending |
| T1.1 | Define shared types (`Session`, `Pane`, `Direction`, sentinel errors) in `internal/terminal/types.go` | High | None | pending |
| T2   | Implement `TmuxSubstrate` constructor with `exec.LookPath` validation in `internal/terminal/tmux.go` | High | T1, T1.1 | pending |
| T2.1 | Implement `execTmux` private helper method | High | T2 | pending |
| T2.2 | Implement `SpawnSession` (detached session, duplicate detection) | High | T2.1 | pending |
| T2.3 | Implement `DestroySession` (kill session, not-found detection) | High | T2.1 | pending |
| T3   | Unit tests for constructor (tmux present/absent) | High | T2 | pending |
| T3.1 | Unit tests for SpawnSession (success, duplicate) | High | T2.2 | pending |
| T3.2 | Unit tests for DestroySession (success, not-found) | High | T2.3 | pending |

---

## Exit Criteria

- [ ] `Substrate` interface compiles and is importable without implementation dependency
- [ ] All 5 sentinel errors are defined and exported
- [ ] `NewTmuxSubstrate()` returns `ErrTmuxNotFound` when tmux is absent
- [ ] `SpawnSession` creates a detached tmux session verifiable via `tmux has-session`
- [ ] `SpawnSession` returns `ErrSessionExists` for duplicate names
- [ ] `DestroySession` removes a session verifiable via `tmux has-session` failing
- [ ] `DestroySession` returns `ErrSessionNotFound` for non-existent sessions
- [ ] All Must-Have acceptance scenarios pass
- [ ] No regressions on existing code (CI green)
- [ ] Interface is substrate-agnostic — a mock implementation compiles against it

---

## References

- Parent epic spec: `specs/terminal-substrate-spec.md`
- Research MOC: `docs/research/Agentic-Orchestrator-MOC.md`
- Terminal multiplexing research: `docs/research/Terminal-Multiplexing-Tmux.md`
- Terminal infrastructure patterns: `docs/research/patterns/Terminal-Infrastructure-for-Agents.md`
- GitHub issue: [#42 — F1: Substrate Interface & Core Implementation](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/42)
- Epic PR: [#25 — feat: implement terminal substrate layer](https://github.com/Kiriketsuki/agenKic-orKistrator/pull/25)

---
*Authored by: Clault KiperS 4.6*
