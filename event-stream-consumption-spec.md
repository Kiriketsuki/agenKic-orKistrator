# Feature: Event Stream Consumption

## Overview

**User Story**: As an orchestrator component (supervisor, DAG engine, UI), I want to read and subscribe to the event stream so that I can react to agent state changes and task lifecycle events in real time.

**Problem**: `StateStore` can publish events via `PublishEvent` (XADD) but has no read-side methods. F2 (supervisor), F3 (DAG engine), and F4 (UI) are all blocked on having a consumption path for the event stream.

**Out of Scope**: Dead-letter queue handling, stream trimming/retention policies, consumer-specific filtering logic (consumers filter in application code).

---

## Success Condition

> This feature is complete when any StateStore consumer can perform one-shot reads (XREAD), subscribe via consumer groups (XREADGROUP), acknowledge processed events (XACK), create consumer groups idempotently, and all operations pass conformance tests against both the Redis and mock implementations -- including a Redis integration test demonstrating two competing consumers processing events concurrently with distinct delivery.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should consumer groups be auto-created on first SubscribeEvents call, or require explicit CreateConsumerGroup? | Spec author | [x] Explicit — CreateConsumerGroup is a separate interface method |
| 2 | Does the Event struct need a stream ID field for ack? | Spec author | [x] Yes — new `StreamEvent` wrapper: `{ ID string; Event Event }` |

---

## Scope

### Must-Have
- **M1 — ReadEvents (one-shot XREAD)**: Given published events, `ReadEvents(ctx, lastID, count)` returns up to `count` `StreamEvent`s after `lastID`; returns empty slice when no new events exist
- **M2 — CreateConsumerGroup**: `CreateConsumerGroup(ctx, group, startID)` creates a Redis consumer group on the event stream; idempotent (no error if group already exists)
- **M3 — SubscribeEvents (XREADGROUP)**: `SubscribeEvents(ctx, group, consumer, count, block)` returns events assigned to that consumer; competing consumers each get distinct events
- **M4 — AckEvent**: `AckEvent(ctx, group, ids...)` acknowledges processed events; acked events are not re-delivered
- **M5 — StreamEvent wrapper**: A `StreamEvent` struct wrapping `Event` + the Redis-assigned stream entry ID; all read methods return `[]StreamEvent`
- **M6 — Mock implementation**: MockStore implements all new methods with in-memory consumer group semantics (round-robin delivery, pending event tracking)
- **M7 — Conformance tests**: Shared test suite covers all must-haves; passes on both Mock and Redis implementations
- **M8 — Integration test**: Redis integration test demonstrating two competing consumers processing events concurrently, each receiving distinct events

### Should-Have
- **Pending event list (XPENDING)**: Inspect unacked messages per consumer for health monitoring

### Nice-to-Have
- None

---

## Technical Plan

**Affected Components**:
- `internal/state/store.go` — `StreamEvent` type + 4 new methods on `StateStore` interface
- `internal/state/redis.go` — Redis implementations (XREAD, XGROUP CREATE, XREADGROUP, XACK)
- `internal/state/mock.go` — in-memory implementations with simulated consumer groups
- `internal/state/store_test.go` — conformance tests for new methods
- `internal/state/redis_test.go` — competing-consumer integration test
- `internal/state/errors.go` — potential new sentinel errors

**Data Model Changes**:
- No new Redis keys — uses existing `{prefix}events` stream
- Consumer groups are created on the existing stream via XGROUP CREATE

**API Contracts** (Go interface additions):

```go
// StreamEvent wraps an Event with its Redis-assigned stream entry ID.
type StreamEvent struct {
    ID    string // Redis stream entry ID (e.g., "1234567890-0")
    Event Event
}

// New methods on StateStore:
ReadEvents(ctx context.Context, lastID string, count int64) ([]StreamEvent, error)
CreateConsumerGroup(ctx context.Context, group string, startID string) error
SubscribeEvents(ctx context.Context, group, consumer string, count int64, block time.Duration) ([]StreamEvent, error)
AckEvent(ctx context.Context, group string, ids ...string) error
```

**Design decisions**:
- Stream key is hidden — callers don't pass it (single event stream, matches `PublishEvent` pattern)
- `SubscribeEvents` is synchronous (returns `[]StreamEvent`) — callers wrap in goroutine loops; simpler to test than channel-based
- `CreateConsumerGroup` uses `MKSTREAM` flag to avoid errors if the stream doesn't exist yet
- Mock consumer groups: track assigned-but-unacked events per consumer, round-robin delivery across consumers in the same group

**Dependencies**: None new — `go-redis/v9` already supports XREAD, XREADGROUP, XACK, XGROUP

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Mock consumer group semantics diverge from Redis behavior | Medium | Conformance tests run against both impls; keep mock simple (round-robin, no redelivery timeout) |
| XREADGROUP blocking in tests causes flaky timeouts | Medium | Use short block durations (10-50ms) in tests; always publish before subscribing in happy path |
| Stream ID format differences between Redis and mock | Low | Mock generates monotonic `"mock-{n}"` IDs; tests compare by count/content, not by ID format |

---

## Acceptance Scenarios

```gherkin
Feature: Event Stream Consumption
  As an orchestrator component
  I want to read and subscribe to the event stream
  So that I can react to agent state changes and task lifecycle events

  Background:
    Given a StateStore with no prior events

  Rule: One-shot reads return events after a cursor

    Scenario: ReadEvents returns published events
      Given 3 events have been published
      When I call ReadEvents with lastID "0" and count 10
      Then I receive 3 StreamEvents with matching Event payloads
      And each StreamEvent has a non-empty ID

    Scenario: ReadEvents with cursor returns only newer events
      Given 3 events have been published
      And I record the ID of the 2nd event
      When I call ReadEvents with that ID and count 10
      Then I receive 1 StreamEvent (the 3rd event only)

    Scenario: ReadEvents on empty stream returns empty slice
      When I call ReadEvents with lastID "0" and count 10
      Then I receive an empty slice and no error

  Rule: Consumer groups enable competing consumption

    Scenario: CreateConsumerGroup is idempotent
      When I call CreateConsumerGroup with group "workers" and startID "0"
      And I call CreateConsumerGroup with group "workers" and startID "0" again
      Then both calls succeed without error

    Scenario: SubscribeEvents delivers events to a consumer
      Given a consumer group "workers" starting from "0"
      And 3 events have been published
      When consumer "w1" calls SubscribeEvents with count 10
      Then consumer "w1" receives 3 StreamEvents

    Scenario: Competing consumers receive distinct events
      Given a consumer group "workers" starting from "0"
      And 6 events have been published
      When consumer "w1" and "w2" each call SubscribeEvents with count 10
      Then the union of their results is all 6 events
      And no event appears in both results

  Rule: Acknowledgement prevents redelivery

    Scenario: Acked events are not re-delivered
      Given a consumer group "workers" starting from "0"
      And 2 events have been published
      And consumer "w1" receives both events via SubscribeEvents
      When consumer "w1" acks both event IDs
      And consumer "w1" calls SubscribeEvents again
      Then consumer "w1" receives an empty slice

    # Future / Out of Scope — requires XCLAIM/XAUTOCLAIM interface additions
    # Scenario: Unacked events can be reclaimed
    #   Given a consumer group "workers" starting from "0"
    #   And 1 event has been published
    #   And consumer "w1" receives the event but does not ack
    #   When consumer "w2" reads with startID "0" (claiming pending)
    #   Then consumer "w2" can receive the unacked event
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T1 | Add `StreamEvent` type and 4 new methods to `StateStore` interface in `store.go` | High | None | pending |
| T2 | Implement `ReadEvents` in `redis.go` (XREAD) and `mock.go` | High | T1 | pending |
| T3 | Implement `CreateConsumerGroup` in `redis.go` (XGROUP CREATE) and `mock.go` | High | T1 | pending |
| T4 | Implement `SubscribeEvents` in `redis.go` (XREADGROUP) and `mock.go` | High | T1, T3 | pending |
| T5 | Implement `AckEvent` in `redis.go` (XACK) and `mock.go` | High | T1, T4 | pending |
| T6 | Add conformance tests to `store_test.go` for all new methods | High | T2-T5 | pending |
| T7 | Add competing-consumer integration test in `redis_test.go` | High | T6 | pending |
| T8 | Add new sentinel errors to `errors.go` if needed | Med | T1 | pending |

Notes: T2 and T3 can be parallelized. T4 depends on T3 (needs consumer groups). T5 depends on T4 (needs stream IDs from subscribe).

---

## Exit Criteria

- [ ] All Must-Have acceptance scenarios pass in conformance tests (Mock + Redis)
- [ ] Competing-consumer integration test passes against real Redis
- [ ] No regressions on existing conformance tests (`RunStateStoreConformance`)
- [ ] `StateStore` interface additions compile with existing implementations
- [ ] API contracts match implementation signatures

---

## References

- Issue: [#17 — Event Stream Consumption Methods for StateStore](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/17)
- PR: [#23 — feat: event stream consumption methods](https://github.com/Kiriketsuki/agenKic-orKistrator/pull/23)
- Parent spec: `specs/foundation-scaffold-proto-redis-agent-spec.md`
- Parent spec: `specs/go-orchestrator-core-spec.md`

---
*Authored by: Clault KiperS 4.6*
