# Feature: Go Orchestrator Core

## Overview

**User Story**: As an orchestrator operator, I want a supervisor process that spawns, monitors, and coordinates AI agent workers so that I can run multi-agent workflows with fault tolerance and observable state.

**Problem**: No runtime exists to manage agent lifecycles, distribute tasks, or maintain shared state. Without this, agents are isolated processes with no coordination, supervision, or recoverability.

**Out of Scope**: Model-specific routing logic (model gateway), terminal pane management (terminal substrate), pixel-art UI rendering (Godot UI).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | | | [ ] |

---

## Scope

### Must-Have
- **Supervisor process** that spawns and manages agent worker processes: supervisor starts, spawns N agents, all agents register and report healthy
- **Agent state machine** (idle -> assigned -> working -> reporting -> idle): state transitions are logged, queryable via gRPC, persisted in Redis Hashes
- **gRPC service layer** with agent registration, task submission, and bidirectional streaming: agents connect, receive tasks, and stream output back to supervisor
- **Redis state store** (Streams for event log + Hashes for agent state + Sorted Sets for task queue): all state survives supervisor restart, events are replayable
- **OTP-style process supervision** with exponential backoff, circuit breaker, graceful shutdown: killed agent restarts automatically; 5 crashes in 60s triggers circuit breaker
- **Task DAG engine** that decomposes multi-step tasks into ordered, parallelizable subtasks: a 3-step DAG with one parallel fork executes correctly

### Should-Have
- Health probe endpoints (liveness, readiness, progress)
- Consumer group support for horizontal scaling of workers

### Nice-to-Have
- Checkpointing/snapshotting for crash recovery mid-task
- Metrics export (Prometheus-compatible)

---

## Technical Plan

**Affected Components**:
- `cmd/orchestrator/main.go` — supervisor entrypoint
- `internal/supervisor/` — supervision tree, restart strategies, health probes
- `internal/agent/` — agent state machine, worker lifecycle
- `internal/dag/` — task DAG engine, topological execution
- `internal/ipc/` — gRPC server, service definitions
- `internal/state/` — Redis client, streams, hashes, sorted sets
- `proto/orchestrator.proto` — gRPC service + message definitions
- `go.mod` — module definition

**Data Model Changes**:
- `agent:{id}` (Redis Hash) — state, last_heartbeat, current_task
- `tasks` (Redis Stream) — task events (submitted, assigned, completed, failed)
- `task_queue` (Redis Sorted Set) — priority-ordered pending tasks

**API Contracts** (gRPC):
- `RegisterAgent(AgentInfo) -> AgentID` — agent joins the pool
- `SubmitTask(TaskSpec) -> TaskID` — submit work to the queue
- `StreamOutput(AgentID) -> stream OutputChunk` — bidirectional agent output
- `GetAgentState(AgentID) -> AgentState` — query current state
- `SubmitDAG(DAGSpec) -> DAGExecutionID` — submit multi-step workflow

**Dependencies**: `google.golang.org/grpc`, `github.com/redis/go-redis/v9`, `google.golang.org/protobuf`

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| gRPC adds 5-20ms per call; too slow for control channel | Medium | Use Unix domain sockets (~130us) for local hot paths |
| Redis single point of failure | Low | Redis Sentinel for HA; event sourcing enables replay from any checkpoint |
| DAG engine complexity grows beyond MVP | Medium | Start with linear + single-fork DAGs only; defer complex graph scheduling |

---

## Acceptance Scenarios

```gherkin
Feature: Go Orchestrator Core
  As an orchestrator operator
  I want a supervisor that coordinates AI agent workers
  So that I can run multi-agent workflows with fault tolerance

  Background:
    Given a running Redis instance
    And the orchestrator supervisor is started

  Rule: Supervisor spawns and manages agent workers

    Scenario: Agent registers with supervisor
      Given an agent worker process is started
      When the agent sends a RegisterAgent request via gRPC
      Then the supervisor assigns an AgentID
      And the agent state is set to "idle" in Redis

    Scenario: Agent fails to heartbeat
      Given a registered agent with ID "agent-01"
      When the agent misses 3 consecutive heartbeats
      Then the supervisor marks the agent as "unhealthy"
      And triggers the restart strategy

  Rule: Task submission and assignment

    Scenario: Task is submitted and assigned to idle agent
      Given an idle agent "agent-01"
      When a TaskSpec is submitted via SubmitTask
      Then the task is added to the task queue
      And the supervisor assigns it to "agent-01"
      And the agent state transitions to "assigned"

    Scenario: No idle agents available
      Given all registered agents are in "working" state
      When a TaskSpec is submitted
      Then the task remains in the queue as "pending"
      And is assigned when the next agent becomes idle

  Rule: Agent state machine transitions

    Scenario: Full lifecycle from idle to reporting
      Given an idle agent "agent-01" assigned a task
      When the agent begins execution
      Then the state transitions to "working"
      When the agent completes and streams results
      Then the state transitions to "reporting"
      When the output is fully received
      Then the state transitions to "idle"

  Rule: OTP-style supervision with circuit breaker

    Scenario: Agent crashes and restarts with backoff
      Given a registered agent "agent-01"
      When the agent process crashes
      Then the supervisor restarts it after 1 second
      When it crashes again
      Then the supervisor restarts after 2 seconds (exponential backoff)

    Scenario: Circuit breaker trips after repeated failures
      Given an agent that has crashed 5 times in 60 seconds
      When the 6th crash occurs
      Then the circuit breaker opens
      And the supervisor stops restarting and alerts the operator

  Rule: DAG task execution

    Scenario: Linear DAG executes in order
      Given a DAG with tasks [A -> B -> C]
      When the DAG is submitted via SubmitDAG
      Then task A executes first
      And task B executes after A completes
      And task C executes after B completes

    Scenario: Parallel fork in DAG
      Given a DAG with tasks [A -> (B, C) -> D]
      When task A completes
      Then tasks B and C execute in parallel
      And task D executes after both B and C complete
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T1   | Project scaffold: `go mod init`, directory structure, proto definitions | High | None | pending |
| T1.1 | Define `orchestrator.proto` with gRPC services and messages | High | T1 | pending |
| T2   | Redis state layer: agent hashes, task stream, sorted set queue | High | T1 | pending |
| T3   | Agent state machine (idle/assigned/working/reporting) with Redis persistence | High | T2 | pending |
| T4   | gRPC server: RegisterAgent, SubmitTask, StreamOutput, GetAgentState | High | T1.1, T3 | pending |
| T5   | Supervisor: spawn workers, heartbeat monitoring, one-for-one restart | High | T3, T4 | pending |
| T5.1 | Exponential backoff + circuit breaker restart policy | High | T5 | pending |
| T5.2 | Graceful shutdown (SIGTERM -> checkpoint -> grace period -> SIGKILL) | High | T5 | pending |
| T6   | DAG engine: define DAG spec, topological sort, parallel fork execution | High | T4 | pending |
| T7   | Health probes (liveness, readiness, progress) | Med  | T5 | pending |
| T8   | Integration tests: full supervisor -> agent -> task -> DAG lifecycle | High | T6 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI
- [ ] No regressions on related features
- [ ] gRPC contracts match proto definitions
- [ ] Agent state transitions are fully logged and queryable
- [ ] Circuit breaker behavior verified under load (5 crashes / 60s)
- [ ] DAG with parallel fork executes correctly

---

## References

- Research MOC: `docs/research/Agentic-Orchestrator-MOC.md`
- Orchestration patterns: `docs/research/Agentic-Orchestration-Patterns.md`
- IPC research: `docs/research/IPC-Inter-Agent-Communication.md`
- State management: `docs/research/Agent-State-Management.md`
- Process supervision: `docs/research/Process-Supervision.md`

---
*Authored by: Clault KiperS 4.6*
