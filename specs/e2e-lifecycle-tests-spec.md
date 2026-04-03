# Feature: E2E Lifecycle Tests

## Overview

**Parent Task**: [Task #1: Implement Go Orchestrator Core](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/1)

**User Story**: As a developer, I want a comprehensive end-to-end test suite covering the full orchestrator lifecycle — from supervisor start through task completion and DAG execution — including all failure paths, so that I can ship the orchestrator core with high confidence it behaves correctly under real conditions.

**Problem**: Unit and integration tests for individual components (state machine, gRPC server, DAG engine) do not verify the system works as a whole. Subtle integration bugs — goroutine races, Redis state divergence, incorrect context cancellation — only surface under full end-to-end load.

**Out of Scope**: Performance/load testing, model gateway integration, pixel UI integration. Depends on F3 (DAG engine), which transitively depends on F1 and F2.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should E2E tests use a real Redis instance (via testcontainers) or a Redis mock? | — | [ ] |
| 2 | How should agent workers be simulated in E2E tests — real goroutines or gRPC stubs? | — | [ ] |

---

## Scope

### Must-Have

**Happy-path lifecycle**:
- Supervisor starts → N agents register via gRPC → task submitted → assigned to idle agent → agent transitions through state machine → task completes
- DAG with parallel fork: supervisor start → agents register → SubmitDAG → fork executes in parallel → join completes

**Failure-path coverage**:
- Agent crash and restart: agent goroutine panics → supervisor restarts with backoff → agent re-registers → resumes work
- Circuit breaker trip: agent crashes 5× in 60s → circuit breaker opens → operator alert logged → cooldown → half-open probe → circuit closes
- DAG fail-fast: node failure → sibling cancellation → DAG marked failed → caller retries full DAG

### Should-Have
- Race detector enabled for all E2E tests (`go test -race`)
- Test isolation: each test gets a fresh Redis keyspace prefix
- Test teardown: all goroutines confirmed stopped, no Redis keys leaked

### Nice-to-Have
- Chaos test: random agent crashes injected during DAG execution, verifying overall system recovery

---

## Technical Plan

**Affected Components**:
- `test/e2e/` — E2E test package (separate from `internal/` unit tests)
- `test/e2e/helpers.go` — test harness: start supervisor, register N mock agents, clean up
- `test/e2e/lifecycle_test.go` — happy-path lifecycle tests
- `test/e2e/failure_test.go` — failure-path tests (crash, circuit breaker, DAG fail-fast)
- `test/e2e/dag_test.go` — DAG parallel fork and completion tests

**Test Harness Design**:
```go
type TestHarness struct {
    supervisor *supervisor.Supervisor
    grpcServer *ipc.Server
    redis      *state.RedisStore
    agents     []*MockAgent
    cancel     context.CancelFunc
}

func NewTestHarness(t *testing.T, agentCount int) *TestHarness
func (h *TestHarness) Teardown()
```

**Mock Agent**:
- Goroutine that connects via gRPC (using a real gRPC client against the test server)
- Configurable behavior: succeed after N seconds, fail after N calls, send heartbeats
- Exposes `Crash()` method to simulate panic for fault-tolerance tests

**Redis Isolation**:
- Each test suite uses a unique key prefix (e.g., `test:{uuid}:`)
- `StateStore` accepts a key prefix option
- Teardown flushes all keys with the test prefix

**Race Detector**:
- All E2E tests run with `-race` flag
- CI step: `go test -race -timeout 120s ./test/e2e/...`

**Dependencies**: F3 (DAG engine), F4 (health probes for readiness check in harness setup)

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Flaky tests due to timing sensitivity in backoff/circuit breaker | High | Use fake clock (testify/clock or manual time injection) for deterministic backoff |
| Redis test pollution across parallel test runs | Medium | Unique key prefix per test; flush on teardown |
| E2E tests too slow for CI | Medium | Parallelise test cases; set `-timeout 120s`; run E2E as separate CI step |

---

## Acceptance Scenarios

```gherkin
Feature: E2E Lifecycle Tests
  As a developer
  I want end-to-end tests covering the full orchestrator lifecycle
  So that the system behaves correctly under real conditions

  Background:
    Given a real Redis instance (testcontainers or local)
    And the test harness has started the supervisor and gRPC server
    And 3 mock agent workers are registered

  Rule: Happy-path single task lifecycle

    Scenario: Task submitted, assigned, and completed
      Given 3 idle mock agents
      When a TaskSpec is submitted via SubmitTask
      Then one agent receives the task and transitions to "assigned"
      And the agent transitions to "working"
      And the agent streams output and transitions to "reporting"
      And the agent transitions to "idle" on completion
      And the task is marked "completed" in Redis

  Rule: Happy-path DAG lifecycle

    Scenario: DAG with parallel fork completes correctly
      Given 3 idle mock agents
      When SubmitDAG is called with [A → (B, C) → D]
      Then A executes first
      And B and C execute concurrently after A
      And D executes after both B and C complete
      And the DAGExecution is marked "completed"

  Rule: Agent crash and restart

    Scenario: Agent crashes and auto-restarts
      Given a registered mock agent "agent-01"
      When agent-01 crashes via Crash()
      Then the supervisor restarts it after 1 second
      And agent-01 re-registers and transitions to "idle"
      And any in-flight task is re-queued

    Scenario: Restarted agent resumes work after crash
      Given agent-01 crashes while processing a task
      When agent-01 is restarted
      And re-registers
      Then the task is reassigned from the queue
      And completes successfully

  Rule: Circuit breaker end-to-end

    Scenario: 5 crashes trip the circuit breaker
      Given agent-01 configured to crash on every start
      When 5 crashes occur within 60 seconds
      Then the circuit breaker opens
      And the supervisor logs an operator alert
      And no further restarts occur
      When 30 seconds elapse and the next probe succeeds
      Then the circuit breaker closes
      And agent-01 resumes normal operation

  Rule: DAG fail-fast end-to-end

    Scenario: Node failure cancels siblings and marks DAG failed
      Given a DAGSpec [A → (B, C) → D] where B is configured to fail
      When the DAG executes and B fails
      Then C is cancelled
      And D is never started
      And the DAGExecution is marked "failed"
      When SubmitDAG is retried with the same spec
      Then A, B (now succeeds), C, D all execute correctly
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T8 | E2E test harness: supervisor + gRPC server start, mock agents, Redis isolation, teardown | High | F3 | pending |
| T8.1 | Happy-path lifecycle test: task submit → assign → execute → complete | High | T8 | pending |
| T8.2 | Happy-path DAG test: parallel fork → join → complete | High | T8 | pending |
| T8.3 | Failure test: crash + restart with backoff | High | T8 | pending |
| T8.4 | Failure test: circuit breaker trip (5 crashes/60s) + auto-reset | High | T8 | pending |
| T8.5 | Failure test: DAG fail-fast sibling cancellation + full retry | High | T8 | pending |
| T8.6 | Race detector CI step: `go test -race ./test/e2e/...` | High | T8.1–T8.5 | pending |

---

## Exit Criteria

- [ ] All happy-path lifecycle tests pass (single task + DAG with fork)
- [ ] Agent crash/restart test passes with correct backoff timing
- [ ] Circuit breaker trips at exactly 5 crashes in 60s and auto-resets after cooldown
- [ ] DAG fail-fast cancels all siblings on first node failure
- [ ] Full DAG retry succeeds after prior failure
- [ ] `go test -race ./test/e2e/...` passes with no data race warnings
- [ ] All goroutines confirmed stopped and no Redis keys leaked after teardown

---

## References

- Parent spec: `specs/go-orchestrator-core-spec.md`
- F3 dependency: `specs/dag-task-engine-spec.md`
- F4 dependency: `specs/health-probes-spec.md`
- Process supervision research: `docs/research/Process-Supervision.md`
- Orchestration patterns: `docs/research/Agentic-Orchestration-Patterns.md`

---
*Sub-feature of Task #1 · Go Orchestrator Core*
