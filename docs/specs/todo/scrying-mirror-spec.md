# Feature: Scrying Mirror

## Overview

**User Story**: As an OrKi operator, I want a Scrying Mirror panel showing live orchestrator and backend state. It lets me read supervisor health, process vitals, and agent distribution without leaving the tower UI.

**Problem**: Supervisor internals (heartbeats, restart backoff, circuit breaker, queue depth, reapers, degraded mode) are invisible to the Godot client. The UI never reads the health endpoint on :8080.

**Out of Scope**: Historical time-series or charts. Any new HTTP endpoint or port. Gateway and LLM cost metrics, because no binary imports the gateway. Windows PTY concerns. Redis state.

---

## Success Condition

> This feature is complete when the panels flyout opens a Scrying Mirror panel whose three rune tabs render live `state.snapshot` SSE data, and the mirror clouds within 6s of the stream dropping.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | None | | [x] |

---

## Scope

### Must-Have
- `state.snapshot` SSE event: bridge broadcasts an aggregated snapshot every 2s on `/events/stream`, only while at least one SSE client is connected.
- `Supervisor.Snapshot()`: read-only export returning per-agent heartbeat age, state, restart backoff remaining, crash count in the 60s window, breaker open/closed, plus queue depth, reaper stats, and degraded (no-tmux) flag.
- Vitals in payload: uptime, goroutine count, heap bytes, SSE client count, gRPC agent connection count.
- Sums in payload: agent counts by state, per floor.
- Scrying Mirror panel: `scrying_mirror_view.tscn`/`.gd` registered in `panel_content_router.gd` and the panels flyout (F5). Three rune tabs (I Wards, II Vitals, III Sums), last tab remembered per session.
- Glyph rendering: candle flicker per agent scaled by heartbeat age, wax seal (reuse `wax_seal.gd`) that cracks when the breaker trips, scroll stack for queue depth, rune dials for vitals.
- Failure handling: clouded-mirror overlay when the SSE stream drops or the last snapshot is older than 6s. A degraded-mode banner rune appears in the Wards tab.

### Should-Have
- Dimmed last-known values under the clouded overlay.

### Nice-to-Have
- Subtle candle audio cue when an agent goes stale.

---

## Technical Plan

**Affected Components**:
- `internal/supervisor/supervisor.go`, `internal/supervisor/restart.go` (new `Snapshot()`, breaker and backoff readers)
- `internal/httpbridge/` (snapshot aggregator, ticker, SSE emission beside `sse.go`)
- `cmd/orchestrator/main.go` (wire supervisor into the bridge aggregator)
- `godot/scripts/panels/scrying_mirror_view.gd` and `godot/scenes/scrying_mirror_view.tscn` (new)
- `godot/scripts/panels/panel_content_router.gd`, `godot/scripts/ui/panels_flyout.gd` (registration)
- `godot/scripts/ui/wax_seal.gd` (reuse)

**Data Model Changes**: None. The payload is transient JSON with top-level keys `wards`, `vitals`, `sums`.

**API Contracts**:
- SSE `event: state.snapshot` on `GET /events/stream`, payload `{wards: {agents: [{id, state, hb_age_ms, backoff_ms, crash_count, breaker_open}], queue_depth, reaper: {...}, degraded}, vitals: {uptime_s, goroutines, heap_bytes, sse_clients, grpc_conns}, sums: {by_state: {...}, by_floor: {...}}}`.

**Dependencies**: None external. A missing tmux only flips the `degraded` flag.

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| `Snapshot()` contends with supervisor mutexes under load | Low | Hold per-agent locks briefly, copy out, never call while holding another lock |
| Snapshot broadcast bloats SSE for many agents | Low | Send one aggregated event per tick, not one per agent |
| Panel timer drift causes false clouding | Low | Stamp snapshots client-side on receipt. Test the 6s threshold math headless |

---

## Acceptance Scenarios

```gherkin
Feature: Scrying Mirror
  As an OrKi operator
  I want a live orchestrator state panel
  So that I can see backend health inside the tower UI

  Background:
    Given the orchestrator is running and the Godot client holds the SSE stream

  Rule: Snapshot emission

    Scenario: Snapshot broadcast while a client listens
      Given at least one SSE client is connected
      When 2 seconds elapse
      Then the bridge emits one state.snapshot event with wards, vitals, and sums keys

    Scenario: No clients connected
      Given zero SSE clients are connected
      When 2 seconds elapse
      Then the bridge builds no snapshot and emits nothing

  Rule: Panel rendering

    Scenario: Wards tab shows supervisor internals
      Given an agent with a heartbeat 9 seconds old
      When the Wards tab is open
      Then that agent's candle renders in the low state
      And the wax seal shows INTACT while the breaker is closed

    Scenario: Breaker trip cracks the seal
      Given an agent crashes 5 times within 60 seconds
      When the next snapshot arrives
      Then the wax seal renders cracked

    Scenario: Degraded mode banner
      Given tmux is absent and the orchestrator runs headless
      When the Wards tab is open
      Then a degraded banner rune is visible

  Rule: Failure handling

    Scenario: Stream drops
      Given the panel is open
      When no snapshot arrives for more than 6 seconds
      Then the clouded-mirror overlay appears and the last values dim
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T1   | Add `Supervisor.Snapshot()` with backoff and breaker readers, unit tests | High | None | pending |
| T2   | Bridge aggregator, 2s client-gated ticker, `state.snapshot` emission, tests beside `sse_test.go` | High | T1 | pending |
| T3   | Wire supervisor into the bridge in `cmd/orchestrator/main.go` | High | T2 | pending |
| T4   | `scrying_mirror_view` scene and script, SSE subscription, three rune tabs | High | T2 | pending |
| T4.1 | Glyph gauges: candle flicker, wax seal reuse, scroll stack, rune dials | High | T4 | pending |
| T4.2 | Clouded overlay, 6s staleness check, degraded banner | High | T4 | pending |
| T5   | Register the panel in `panel_content_router.gd` and the panels flyout | Med | T4 | pending |
| T6   | Headless Godot math test for candle age and staleness mapping | Med | T4.1 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI
- [ ] No regressions on related features
- [ ] API contracts match implementation
- [ ] `make test` passes with -race, and the SSE emission test is green
- [ ] The headless Godot test for candle age mapping passes

---

## References

- Related specs: `specs/go-orchestrator-core-spec.md`, `specs/pixel-office-ui-spec.md`
- Tickets: N/A (create an issue per the repo workflow)

---
*Authored by: Clault KiperO 5.0*
