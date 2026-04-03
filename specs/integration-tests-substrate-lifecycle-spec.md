# Feature: Integration Tests — Full Substrate Lifecycle

## Overview

**Parent Epic**: [Epic #2: Implement Terminal Substrate Layer](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/2)

**User Story**: As a developer working on the terminal substrate, I want a comprehensive integration test suite covering the full tmux lifecycle — spawn, command, capture, layout, and destroy — so that I can ship the substrate layer with high confidence it behaves correctly against a real tmux process.

**Problem**: Unit tests for individual tmux wrapper functions mock `os/exec` and cannot verify that the substrate works end-to-end against a real tmux binary. Subtle issues — incorrect `send-keys` Enter handling, `capture-pane` scrollback off-by-one, session name collisions, pane split failures — only surface when running against real tmux.

**Out of Scope**: WezTerm adapter tests (separate feature), performance/load testing, orchestrator-level integration (T6 wires substrate into supervisor — that is a separate task), UI rendering of terminal output.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | | | [ ] |

---

## Scope

### Must-Have

- **Full lifecycle test** — SpawnSession, SendCommand, CaptureOutput, SplitPane, ListSessions, DestroySession exercised in sequence with marker-based output verification: echoed marker string appears in captured output
- **Duplicate session error path** — spawning the same session name twice returns `ErrSessionExists`
- **SendCommand to non-existent session** — returns `ErrSessionNotFound`
- **CaptureOutput from non-existent session** — returns `ErrSessionNotFound`
- **Multi-pane grid** — 4-pane grid (3 splits) created without error; session's `PaneCount >= 1` confirmed via `ListSessions`
- **Session name validation** — `ValidateSessionName` rejects empty, space-containing, and colon-containing names; accepts valid names
- **tmux preflight check** — `CheckTmux` succeeds when tmux is on PATH; `TmuxVersion` returns a string prefixed with `"tmux"`
- **Build tag isolation** — all tests gated behind `//go:build integration` so `go test ./...` skips them by default
- **Test cleanup** — every test that spawns a session uses `t.Cleanup` with `DestroySession` and pre-cleans stale sessions from previous failed runs

### Should-Have

- **Unique session names per test** — each test function uses a distinct session name prefix (`integration-lifecycle`, `integration-duplicate`, `integration-grid`, etc.) to prevent cross-test interference when running in parallel
- **CI documentation** — clear instructions for enabling tmux in CI (e.g., `apt-get install tmux` in GitHub Actions)

### Nice-to-Have

- **Concurrent test safety** — `t.Parallel()` on tests that use distinct session names, with verification that no session leaks occur
- **Scrollback stress test** — generate 500+ lines, capture last 100, verify exact count

---

## Technical Plan

**Affected Components**:
- `internal/terminal/integration_test.go` — all integration tests (single file, `//go:build integration` tag)

**Test Infrastructure**:
```go
//go:build integration

package terminal

// Tests run inside the terminal package (white-box) so they can access
// unexported helpers like ValidateSessionName, CheckTmux, TmuxVersion.
// Each test creates its own TmuxSubstrate via NewTmuxSubstrate().
```

**Marker-Based Verification Pattern**:
```go
// 1. Send a unique echo command with a marker string
sub.SendCommand(session, "echo UNIQUE_MARKER_STRING")
// 2. Wait briefly for tmux to process (200ms)
time.Sleep(200 * time.Millisecond)
// 3. Capture output and assert marker is present
out, _ := sub.CaptureOutput(session, 50)
assert: strings.Contains(out, "UNIQUE_MARKER_STRING")
```

**Cleanup Strategy**:
```go
// Pre-clean stale sessions from previous failed runs
_ = sub.DestroySession(sessionName)
// Register cleanup to run even if test panics
t.Cleanup(func() { _ = sub.DestroySession(sessionName) })
```

**Dependencies**: tmux (system binary), Go stdlib (`testing`, `errors`, `strings`, `time`)

**CI Requirement**: tmux must be installed in the CI runner. GitHub Actions example:
```yaml
- name: Install tmux
  run: sudo apt-get update && sudo apt-get install -y tmux
- name: Run integration tests
  run: go test -tags=integration -timeout 60s ./internal/terminal/...
```

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| tmux timing sensitivity (command not yet processed when capture runs) | Medium | 200ms sleep after `SendCommand`; increase if CI is slow |
| Stale sessions from crashed test runs pollute next run | Medium | Pre-clean with `_ = sub.DestroySession(name)` before each test |
| tmux not installed in CI | Low | Fail fast with clear skip message via `t.Skip` or hard fail |
| Parallel test runs collide on session names | Low | Unique session name per test function; no shared state |

---

## Acceptance Scenarios

```gherkin
Feature: Integration Tests — Full Substrate Lifecycle
  As a developer
  I want integration tests that exercise the substrate against real tmux
  So that I can verify correctness before shipping

  Background:
    Given tmux is installed and available on PATH
    And a TmuxSubstrate is created via NewTmuxSubstrate()

  Rule: Full lifecycle coverage

    Scenario: TestFullLifecycle — spawn, command, capture, split, list, destroy
      Given no tmux session named "integration-lifecycle" exists
      When SpawnSession is called with name "integration-lifecycle" and cmd "bash"
      Then a Session is returned with Name "integration-lifecycle"
      When SendCommand is called with "echo INTEGRATION_MARKER_12345"
      And 200ms elapses for tmux processing
      And CaptureOutput is called with lines=50
      Then the captured output contains "INTEGRATION_MARKER_12345"
      When SplitPane is called with direction Horizontal
      Then a Pane is returned with non-empty ID and SessionID "integration-lifecycle"
      When ListSessions is called
      Then the result includes a session named "integration-lifecycle"
      When DestroySession is called with name "integration-lifecycle"
      Then the session is terminated
      And a second DestroySession call returns ErrSessionNotFound

  Rule: Duplicate session detection

    Scenario: TestDuplicateSession — ErrSessionExists on double-spawn
      Given no tmux session named "integration-duplicate" exists
      When SpawnSession is called with name "integration-duplicate"
      Then the session is created successfully
      When SpawnSession is called again with name "integration-duplicate"
      Then ErrSessionExists is returned

  Rule: Error paths for non-existent sessions

    Scenario: TestSendCommand_NonExistentSession
      Given no tmux session named "integration-ghost-session" exists
      When SendCommand is called for "integration-ghost-session"
      Then ErrSessionNotFound is returned

    Scenario: TestCaptureOutput_NonExistentSession
      Given no tmux session named "integration-ghost-session" exists
      When CaptureOutput is called for "integration-ghost-session" with lines=10
      Then ErrSessionNotFound is returned

  Rule: Multi-pane layout

    Scenario: TestMultiPaneGrid — 4-pane grid creation
      Given a session "integration-grid" is spawned
      When SplitPane is called 3 times (Horizontal, Vertical, Vertical)
      Then no errors are returned
      And ListSessions shows the session with PaneCount >= 1

  Rule: Utility verification

    Scenario: TestValidateSessionName_Integration
      When ValidateSessionName is called with "valid-agent-01"
      Then no error is returned
      When ValidateSessionName is called with "has spaces"
      Then an error is returned
      When ValidateSessionName is called with "has:colon"
      Then an error is returned
      When ValidateSessionName is called with ""
      Then an error is returned

    Scenario: TestCheckTmux_Integration
      When CheckTmux is called
      Then no error is returned (or the test is skipped if tmux is unavailable)
      When TmuxVersion is called
      Then the result starts with "tmux"
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T5.1 | Create `integration_test.go` with `//go:build integration` tag and `TestFullLifecycle` (spawn, send, capture, split, list, destroy with marker verification) | High | F1, F2, F3 | pending |
| T5.2 | Add `TestDuplicateSession` — verify `ErrSessionExists` on double-spawn | High | T5.1 | pending |
| T5.3 | Add `TestSendCommand_NonExistentSession` and `TestCaptureOutput_NonExistentSession` — error path coverage | High | T5.1 | pending |
| T5.4 | Add `TestMultiPaneGrid` — 4-pane grid creation and `PaneCount` verification | High | T5.1 | pending |
| T5.5 | Add `TestValidateSessionName_Integration` and `TestCheckTmux_Integration` — utility function verification | High | None | pending |
| T5.6 | CI setup: document tmux installation step and `go test -tags=integration` invocation | Medium | T5.1-T5.5 | pending |

---

## Exit Criteria

- [ ] All 7 test functions pass locally with `go test -tags=integration ./internal/terminal/...`
- [ ] All tests use `t.Cleanup` for session teardown — no sessions leaked after test run
- [ ] Each test uses a unique session name — no cross-test interference
- [ ] Build tag `//go:build integration` prevents tests from running in default `go test ./...`
- [ ] Marker-based verification confirms SendCommand + CaptureOutput round-trip works against real tmux
- [ ] Error sentinel matching (`errors.Is`) used for all error path assertions
- [ ] CI pipeline can run tests with tmux installed (documented in spec)

---

## References

- Parent epic spec: `terminal-substrate-spec.md`
- Substrate interface: `internal/terminal/substrate.go`
- Types and sentinels: `internal/terminal/types.go`
- Error utilities: `internal/terminal/tmux_errors.go`
- GitHub issue: [#53 — F4: Integration Tests — Full Substrate Lifecycle](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/53)

---
*Sub-feature of Epic #2 -- Terminal Substrate Layer*

---
*Authored by: Clault KiperS 4.6*
