# Feature: Foundation — Scaffold, Proto, Redis, Agent State Machine

## Overview

**Parent Task**: [Task #1: Implement Go Orchestrator Core](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/1)

**User Story**: As a developer, I want a fully scaffolded Go module with proto definitions, a Redis state layer, and an event-driven agent state machine so that every subsequent feature (gRPC, supervisor, DAG) has a working foundation to build on.

**Problem**: No application code exists yet. Without a Go module, proto contracts, or a Redis state layer, none of the orchestrator features can be implemented. This feature establishes the complete structural skeleton — all later features depend on it.

**Out of Scope**: gRPC server runtime (F2), supervisor/heartbeat (F2), DAG engine (F3), health probes (F4), E2E tests (F5).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should proto definitions be committed as generated `.go` files or only as `.proto` source with codegen in CI? | — | [ ] |
| 2 | Which Redis client version — `go-redis/v9` or `rueidis`? | — | [ ] |

---

## Scope

### Must-Have
- **Go module scaffold**: `go mod init`, full directory structure per `CLAUDE.md` module layout, `.gitignore`, `cmd/orchestrator/main.go` stub
- **Full proto definitions** (`proto/orchestrator.proto`): all services and messages for F1–F4 in a single file to avoid proto churn (RegisterAgent, SubmitTask, StreamOutput, GetAgentState, SubmitDAG, health service)
- **Redis state layer** (`internal/state/`): `StateStore` interface + Redis implementation; agent hashes, task stream (Redis Streams), sorted-set task queue
- **In-memory mock** (`internal/state/mock.go`): implements `StateStore` for unit tests with zero external dependencies
- **Agent state machine** (`internal/agent/`): idle → assigned → working → reporting → idle; transitions are validated, logged, and persisted to Redis via `StateStore`; invalid transitions return typed errors

### Should-Have
- Makefile targets: `generate` (protoc codegen), `test`, `lint`
- `config/models.yaml` stub with tier definitions

### Nice-to-Have
- `docker-compose.yml` for local Redis dev environment

---

## Technical Plan

**Affected Components**:
- `go.mod` / `go.sum` — module and dependency lock
- `cmd/orchestrator/main.go` — empty entrypoint stub
- `proto/orchestrator.proto` — canonical gRPC contract for all features
- `internal/state/store.go` — `StateStore` interface
- `internal/state/redis.go` — Redis implementation (go-redis/v9)
- `internal/state/mock.go` — in-memory mock for tests
- `internal/agent/state.go` — state constants and transition table
- `internal/agent/machine.go` — state machine logic + Redis persistence

**Data Model**:
```
agent:{id}         (Redis Hash)  — state, last_heartbeat, current_task_id, registered_at
tasks              (Redis Stream) — events: {type, agent_id, task_id, timestamp, payload}
task_queue         (Redis Sorted Set) — score=priority, member=task_id
```

**StateStore Interface**:
```go
type StateStore interface {
    SetAgentState(ctx, agentID, state string) error
    GetAgentState(ctx, agentID string) (string, error)
    PublishEvent(ctx, event Event) error
    EnqueueTask(ctx, taskID string, priority float64) error
    DequeueTask(ctx) (string, error)
}
```

**State Transition Table**:
| From | Event | To |
|------|-------|-----|
| idle | TaskAssigned | assigned |
| assigned | WorkStarted | working |
| working | OutputReady | reporting |
| reporting | OutputDelivered | idle |
| any | AgentFailed | idle (reset) |

**Dependencies**: `google.golang.org/grpc`, `google.golang.org/protobuf`, `github.com/redis/go-redis/v9`

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Proto churn if messages are under-specified | Medium | Define all F1–F4 messages upfront; review against all feature specs before generating |
| Redis Streams consumer group complexity at MVP | Low | Start with simple XADD/XREAD; add consumer groups in should-have scope |

---

## Acceptance Scenarios

```gherkin
Feature: Foundation — Scaffold, Proto, Redis, Agent State Machine
  As a developer
  I want a scaffolded module with state persistence
  So that all orchestrator features have a stable foundation

  Background:
    Given the Go module is initialised with all dependencies
    And a Redis instance is running

  Rule: Go module scaffold is complete

    Scenario: Module compiles cleanly
      Given the scaffolded directory structure
      When `go build ./...` is run
      Then it exits 0 with no errors

  Rule: Proto definitions cover all orchestrator features

    Scenario: Proto generates valid Go code
      Given `proto/orchestrator.proto` with all services and messages
      When `protoc` codegen is run
      Then Go stub files are generated without errors
      And the generated code compiles cleanly

  Rule: Redis state layer persists agent state

    Scenario: Agent state is stored in Redis Hash
      Given a StateStore backed by Redis
      When SetAgentState("agent-01", "idle") is called
      Then a Redis HSET on key "agent:agent-01" stores state="idle"

    Scenario: In-memory mock implements StateStore
      Given a StateStore backed by the in-memory mock
      When SetAgentState("agent-01", "idle") is called
      Then GetAgentState("agent-01") returns "idle"

  Rule: Agent state machine validates transitions

    Scenario: Valid transition succeeds
      Given an agent in state "idle"
      When the TaskAssigned event is applied
      Then the state transitions to "assigned"
      And the new state is persisted via StateStore

    Scenario: Invalid transition returns error
      Given an agent in state "idle"
      When the OutputDelivered event is applied
      Then an InvalidTransition error is returned
      And the agent state remains "idle"

    Scenario: Full lifecycle round-trip
      Given an agent in state "idle"
      When events TaskAssigned → WorkStarted → OutputReady → OutputDelivered are applied in sequence
      Then the agent passes through assigned → working → reporting → idle
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T1 | `go mod init`, full directory skeleton, Makefile stubs | High | None | pending |
| T1.1 | Define `proto/orchestrator.proto` (all F1–F4 services + messages) | High | T1 | pending |
| T2 | `internal/state/`: StateStore interface, Redis impl, in-memory mock | High | T1 | pending |
| T3 | `internal/agent/`: state constants, transition table, machine with Redis persistence | High | T2 | pending |
| T3.1 | Unit tests: all valid transitions pass, all invalid transitions error | High | T3 | pending |
| T3.2 | Redis integration test: state persists across mock restart | High | T2, T3 | pending |

---

## Exit Criteria

- [ ] `go build ./...` exits 0
- [ ] `protoc` codegen succeeds with no errors
- [ ] All valid state transitions pass unit tests
- [ ] All invalid transitions return typed errors
- [ ] `StateStore.SetAgentState` / `GetAgentState` round-trip verified against real Redis
- [ ] In-memory mock passes the same test suite as the Redis implementation

---

## References

- Parent spec: `specs/go-orchestrator-core-spec.md`
- State management research: `docs/research/Agent-State-Management.md`
- IPC research: `docs/research/IPC-Inter-Agent-Communication.md`
- Process supervision patterns: `docs/research/patterns/Process-Supervision.md`

---
*Sub-feature of Task #1 · Go Orchestrator Core*
