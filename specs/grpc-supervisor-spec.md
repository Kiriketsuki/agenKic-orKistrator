# Feature: gRPC Service + Supervisor

## Overview

**Parent Task**: [Task #1: Implement Go Orchestrator Core](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/1)

**User Story**: As an orchestrator operator, I want a gRPC server that accepts agent registrations and task submissions, backed by a supervisor that manages goroutine-based agent workers with exponential backoff, circuit breaking, and graceful shutdown, so that agents run reliably and recover automatically from failures.

**Problem**: Without a gRPC server and supervisor, agents have no way to connect to the orchestrator, receive tasks, or stream output. Without fault-tolerance mechanisms, a single agent crash halts the entire workflow.

**Out of Scope**: DAG engine (F3), health probe endpoints (F4), E2E test suite (F5). Depends on F1 (proto definitions, StateStore, agent state machine).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should circuit breaker state (open/half-open/closed) be persisted to Redis or kept in-process only? | — | [ ] |
| 2 | Unix domain socket vs TCP for local hot paths — should the gRPC server expose both at MVP? | — | [ ] |

---

## Scope

### Must-Have
- **gRPC server** (`internal/ipc/`): implements RegisterAgent, SubmitTask, StreamOutput (bidirectional), GetAgentState — all defined in `proto/orchestrator.proto` (F1)
- **Supervisor** (`internal/supervisor/`): spawns goroutine-based agent workers; monitors heartbeats; restarts crashed workers with one-for-one strategy
- **Exponential backoff**: restart delay doubles on each crash (1s → 2s → 4s … up to configured max)
- **Circuit breaker**: opens after 5 crashes in 60 seconds; auto-resets via half-open probe after configurable cooldown (default 30s)
- **Graceful shutdown**: SIGTERM → stop accepting new tasks → drain in-flight work → checkpoint state to Redis → grace period (30s default) → SIGKILL

### Should-Have
- gRPC interceptors for request logging and panic recovery
- Configurable restart policy per agent (always, on-failure, never)
- Heartbeat timeout configurable via environment variable

### Nice-to-Have
- Unix domain socket listener for local hot paths (~130µs vs ~5ms TCP)
- Prometheus metrics for restart counts and circuit breaker state

---

## Technical Plan

**Affected Components**:
- `internal/ipc/server.go` — gRPC server setup and service registration
- `internal/ipc/handlers.go` — RegisterAgent, SubmitTask, StreamOutput, GetAgentState handlers
- `internal/supervisor/supervisor.go` — supervision tree, worker spawning, heartbeat loop
- `internal/supervisor/restart.go` — exponential backoff, circuit breaker logic
- `internal/supervisor/shutdown.go` — graceful shutdown handler (SIGTERM)
- `cmd/orchestrator/main.go` — wires gRPC server + supervisor together

**Circuit Breaker State Machine**:
```
closed --[5 fails / 60s]--> open --[cooldown expires]--> half-open
half-open --[probe success]--> closed
half-open --[probe fail]--> open
```

**Heartbeat Protocol**:
- Agent sends heartbeat every N seconds (configurable, default 5s) via StreamOutput or a dedicated heartbeat RPC
- Supervisor marks agent "unhealthy" after 3 missed heartbeats
- Unhealthy agent triggers restart strategy

**Graceful Shutdown Sequence**:
1. SIGTERM received
2. Stop accepting new gRPC connections
3. Drain in-flight RPCs (up to grace period)
4. Checkpoint all agent states to Redis
5. Cancel all worker goroutines via context cancellation
6. Exit 0 (or SIGKILL after grace period)

**Dependencies**: F1 (StateStore, agent state machine, proto), `google.golang.org/grpc`

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| gRPC adds 5–20ms per call on loopback | Medium | Benchmark; add Unix socket listener if latency unacceptable |
| Circuit breaker storm: multiple agents tripping simultaneously | Low | Per-agent circuit breakers, not a global one |
| Goroutine leak if worker context not cancelled on shutdown | Medium | All workers receive context from supervisor; SIGTERM cancels root context |

---

## Acceptance Scenarios

```gherkin
Feature: gRPC Service and Supervisor
  As an orchestrator operator
  I want agents to register and receive tasks via gRPC
  And the supervisor to manage their lifecycle with fault tolerance

  Background:
    Given the orchestrator gRPC server is started on port 50051
    And a running Redis instance

  Rule: Agent registration

    Scenario: Agent registers and receives an ID
      Given a new agent worker goroutine
      When the agent calls RegisterAgent with a valid AgentInfo
      Then the server returns a unique AgentID
      And the agent state is set to "idle" in Redis

    Scenario: Duplicate registration is rejected
      Given an agent already registered as "agent-01"
      When the same agent calls RegisterAgent again with the same ID
      Then the server returns an AlreadyExists error

  Rule: Task submission and assignment

    Scenario: Task submitted to idle agent
      Given an idle agent "agent-01"
      When a TaskSpec is submitted via SubmitTask
      Then the task is enqueued in Redis sorted set
      And the supervisor assigns it to "agent-01"
      And agent state transitions to "assigned"

    Scenario: Task queued when no idle agents
      Given all agents are in "working" state
      When a TaskSpec is submitted
      Then the task remains in the Redis queue
      And is assigned when the next agent becomes idle

  Rule: Supervisor restart with exponential backoff

    Scenario: Crashed worker restarts with backoff
      Given a registered agent "agent-01"
      When the agent goroutine panics
      Then the supervisor restarts it after 1 second
      When it crashes again immediately
      Then the supervisor restarts after 2 seconds

    Scenario: Backoff resets after stable period
      Given an agent that was restarted with 4s backoff
      When the agent runs stably for 60 seconds
      Then the restart delay resets to 1 second

  Rule: Circuit breaker

    Scenario: Circuit breaker opens after repeated crashes
      Given an agent that has crashed 5 times within 60 seconds
      When the 6th crash occurs
      Then the circuit breaker opens
      And the supervisor stops restarting the agent
      And logs an alert for the operator

    Scenario: Circuit breaker auto-resets after cooldown
      Given an open circuit breaker for "agent-01"
      When 30 seconds elapse
      Then the circuit breaker enters half-open state
      And sends one probe restart
      When the probe succeeds
      Then the circuit breaker closes

  Rule: Graceful shutdown

    Scenario: SIGTERM triggers clean shutdown
      Given 2 agents processing tasks
      When SIGTERM is sent to the supervisor
      Then the supervisor stops accepting new tasks
      And waits for in-flight work to complete (up to 30s)
      And checkpoints all state to Redis
      And exits 0
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T4 | gRPC server: RegisterAgent, SubmitTask, StreamOutput, GetAgentState | High | F1 (T1.1, T3) | pending |
| T4.1 | gRPC integration test: agent registers, receives task, streams output | High | T4 | pending |
| T5 | Supervisor: spawn goroutine workers, heartbeat monitor, one-for-one restart | High | T3, T4 | pending |
| T5.1 | Exponential backoff + circuit breaker (5 crashes/60s, auto-reset 30s cooldown) | High | T5 | pending |
| T5.2 | Graceful shutdown (SIGTERM → drain → checkpoint → cancel → exit) | High | T5 | pending |
| T5.3 | Supervisor tests: restart backoff, circuit breaker trip/reset | High | T5.1 | pending |

---

## Exit Criteria

- [ ] gRPC server accepts agent registration and returns unique IDs
- [ ] Task submission enqueues in Redis and assigns to idle agent
- [ ] Crashed worker restarts with correct exponential backoff delays
- [ ] Circuit breaker opens after 5 crashes in 60s and stops restarts
- [ ] Circuit breaker auto-resets after cooldown and closes on probe success
- [ ] SIGTERM triggers graceful drain + Redis checkpoint + clean exit within 30s
- [ ] All gRPC integration tests pass against a real server

---

## References

- Parent spec: `specs/go-orchestrator-core-spec.md`
- F1 dependency: `specs/foundation-scaffold-proto-redis-agent-spec.md`
- Process supervision research: `docs/research/Process-Supervision.md`
- IPC research: `docs/research/IPC-Inter-Agent-Communication.md`
- Orchestration patterns: `docs/research/Agentic-Orchestration-Patterns.md`

---
*Sub-feature of Task #1 · Go Orchestrator Core*
