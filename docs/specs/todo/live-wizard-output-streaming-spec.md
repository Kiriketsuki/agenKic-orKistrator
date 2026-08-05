# Feature: Live Wizard Output Streaming

## Overview

**User Story**: As an OrKi operator, I want each wizard's terminal output to stream live while a task runs. Floating runes and the spell scroll then reflect agent activity in real time, not only at task completion.

**Problem**: For real CLI agents, `cliagent` sends a single output chunk after the task settles (`internal/cliagent/cliagent.go:283-285`). During work, `settleWatch` captures the pane every 5 s and discards the snapshots, so runes and the live scroll stay empty until the end.

**Out of Scope**: Any Godot change (FloatingRune, RuneFilter, TerminalView, SpellScrollView already consume the SSE path). Setting `significant=true` server side. Changes to settle detection semantics. The simagent path (already streams).

---

## Success Condition

> This feature is complete when a running CLI agent's new pane lines arrive as per-line SSE `agent.output` events within one settle tick (5 s), and runes float above the wizard during the task, verified in the running app.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Task assignment wiring is incomplete. Agents may stay IDLE and suppress runes (`agent_character.gd:339-340`). Streaming still works, and runes appear once StartWork fires. | user | [x] noted as known limit |

---

## Scope

### Must-Have
- Line-set tail diff: each 5 s settle tick, the runner emits lines present in the new 50-line pane snapshot and absent from the previous one, in pane order. The first tick (empty previous snapshot) emits nothing.
- Per-line chunks: one `OutputChunk` per new non-blank line, `OUTPUT_TYPE_STDOUT`, `significant=false`.
- Per-agent stream: one `StreamOutput` gRPC stream opened at agent registration, reused across tasks, closed at despawn. Monotonic per-agent sequence numbers.
- Reconnect-once: on send failure, reopen the stream once and continue. On a second failure, log and disable streaming until the next task.
- Drop the settled full-pane chunk at task end (`cliagent.go:283-285`). Keep `streamOne` for send-failure notices. State transitions stay on the `ReportOutput` RPC. Never send a FINAL-type chunk.

### Should-Have
- None.

### Nice-to-Have
- Tighter cadence constant (2 s) once proven stable.

---

## Technical Plan

**Affected Components**: `internal/cliagent/cliagent.go` only (plus a new `_test.go` table set). Reuses: `settleWatch` capture loop, `StreamOutput` client, `pb.OutputChunk`, existing SSE mapping (`internal/httpbridge/sse.go:284`).

**Data Model Changes**: None.

**API Contracts**: None changed. Chunks flow through existing `StreamOutput` gRPC and SSE `agent.output`.

**Dependencies**: None. Works regardless of incomplete task wiring (chunks keyed by agent).

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| TUI spinner/redraw churn re-emits lines | Medium | RuneFilter insignificance check + 2 s client rate limit absorb it |
| Line-set diff misses a repeated identical line | Low | Acceptable; scroll backfill reads pane via /output |
| Long-lived stream dies silently | Low | Reconnect-once path; send-failure logged |

---

## Acceptance Scenarios

```gherkin
Feature: Live Wizard Output Streaming

  Background:
    Given a CLI agent registered with a per-agent StreamOutput stream

  Rule: Deltas stream live per line

    Scenario: New pane lines stream during a task
      Given the previous snapshot holds lines A and B
      When the next capture holds lines A, B, C and D
      Then chunks for C and D are sent, in order, with increasing sequence numbers

    Scenario: First tick emits nothing
      Given no previous snapshot
      When the first capture returns 50 lines
      Then no chunks are sent

    Scenario: Blank lines are skipped
      Given a delta containing a blank line between two new lines
      Then only the two non-blank lines are sent

    Scenario: In-place redraw does not flood
      Given a snapshot where one spinner line changed in place
      Then only the changed line is emitted

  Rule: Stream lifecycle survives tasks and failures

    Scenario: Sequence numbers stay monotonic across tasks
      Given an agent finishes task 1 with sequence N
      When task 2 emits its first delta
      Then its sequence is greater than N

    Scenario: Send failure triggers one reconnect
      Given a stream send returns an error
      When the runner reopens the stream and the retry succeeds
      Then streaming continues on the new stream
      And a second consecutive failure disables streaming until the next task

    Scenario: No final full-pane chunk
      Given a task settles
      Then no full-snapshot chunk is sent and ReportOutput fires as today
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T1   | Extract pure tail-diff function (prev, next -> new lines) | High | None | pending |
| T1.1 | Table-driven tests: scroll, in-place redraw, empty first snapshot, blank lines, repeated lines | High | T1 | pending |
| T2   | Per-agent stream holder: open at registration, close at despawn, monotonic sequence, reconnect-once | High | None | pending |
| T2.1 | Tests: sequence monotonicity across two tasks, reconnect-once, disable-on-second-failure | High | T2 | pending |
| T3   | Wire diff emission into settleWatch tick; per-line chunk send | High | T1, T2 | pending |
| T4   | Remove settled full-pane chunk; keep streamOne failure notice | Med | T3 | pending |
| T5   | Manual e2e: run ./run.sh, spawn a claude agent, verify runes float during the task | Med | T3, T4 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI (`make test`, race on)
- [ ] No regressions on related features (settleWatch tests stay green)
- [ ] API contracts match implementation (no proto change)
- [ ] Manual check: runes appear during a live task, spell scroll fills line by line

---

## References

- Related specs: `specs/terminal-substrate-spec.md`, `specs/pixel-office-ui-spec.md`
- Code: `internal/cliagent/cliagent.go:283-366`, `internal/ipc/handlers.go:92-118`, `internal/httpbridge/sse.go:284`, `godot/scripts/agents/floating_rune.gd`, `godot/scripts/agents/rune_filter.gd`, `godot/scripts/agents/agent_character.gd:338-360`

---
*Authored by: Clault KiperO 4.8*
