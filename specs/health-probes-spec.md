# Feature: Health Probes

## Overview

**Parent Task**: [Task #1: Implement Go Orchestrator Core](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/1)

**User Story**: As an operator, I want the orchestrator to expose liveness, readiness, and progress probe endpoints over both gRPC and HTTP so that container orchestrators (Kubernetes), load balancers, and monitoring systems can reliably detect degraded or unavailable instances.

**Problem**: Without health probes, there is no standard way for external systems to determine whether the orchestrator is alive, ready to serve traffic, or making progress. Failed instances go undetected, and rolling restarts cannot safely verify the new instance is ready.

**Out of Scope**: Prometheus metrics scraping (nice-to-have), E2E lifecycle tests (F5). Depends on F2 (gRPC server, supervisor).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should the HTTP server share the gRPC port (via `cmux`) or run on a dedicated port (e.g. 8080)? | — | [ ] |
| 2 | What constitutes "not ready"? Zero registered agents? Redis unavailable? Both? | — | [ ] |

---

## Scope

### Must-Have
- **gRPC health service** (`grpc.health.v1`): standard gRPC health protocol; reports SERVING / NOT_SERVING / UNKNOWN per service name
- **HTTP /healthz** (liveness): returns 200 if the process is alive; returns 503 if in a fatal state (e.g. supervisor crashed)
- **HTTP /readyz** (readiness): returns 200 if at least one agent is registered and Redis is reachable; returns 503 otherwise
- **HTTP /progress** (progress): returns JSON with current agent count, tasks in queue, tasks in flight, DAGs in progress

### Should-Have
- Unified health aggregator that checks: Redis ping, gRPC server listening, ≥1 idle agent
- Configurable readiness thresholds (min agent count) via environment variable
- Structured JSON response body for all HTTP endpoints

### Nice-to-Have
- `/metrics` endpoint (Prometheus format) for agent count, restart count, task throughput
- gRPC reflection service for tooling discoverability

---

## Technical Plan

**Affected Components**:
- `internal/supervisor/health.go` — health aggregator: checks Redis, gRPC server, agent count
- `internal/ipc/health_grpc.go` — gRPC health v1 service implementation
- `internal/ipc/health_http.go` — HTTP mux with /healthz, /readyz, /progress handlers
- `cmd/orchestrator/main.go` — starts HTTP health server alongside gRPC server

**gRPC Health Protocol** (`grpc.health.v1`):
```proto
// Standard — uses google.golang.org/grpc/health/grpc_health_v1
service Health {
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
  rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse);
}
```

**HTTP Response Formats**:
```
GET /healthz → 200 OK  {"status": "alive"}
             → 503     {"status": "dead", "reason": "supervisor panic"}

GET /readyz  → 200 OK  {"status": "ready", "agents": 3, "redis": "ok"}
             → 503     {"status": "not_ready", "reason": "no agents registered"}

GET /progress → 200 OK {
  "agents_total": 3,
  "agents_idle": 1,
  "agents_working": 2,
  "tasks_queued": 5,
  "tasks_in_flight": 2,
  "dags_in_progress": 1
}
```

**Health State Transitions**:
- Process starts: liveness=ALIVE, readiness=NOT_READY
- First agent registers: readiness=READY
- All agents unhealthy: readiness=NOT_READY
- Redis ping fails: readiness=NOT_READY
- Supervisor fatal error: liveness=DEAD

**Dependencies**: F2 (supervisor, gRPC server), `google.golang.org/grpc/health/grpc_health_v1`, `net/http`

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| HTTP health server blocks on shutdown | Low | Use separate goroutine with context cancellation |
| False-ready during rolling restart before agents connect | Medium | Default min-agent threshold = 1; make configurable |
| gRPC health Watch leaks goroutine if client disconnects | Low | Cancel server-side context on stream close |

---

## Acceptance Scenarios

```gherkin
Feature: Health Probes
  As an operator
  I want health endpoints over gRPC and HTTP
  So that container orchestrators can manage the orchestrator reliably

  Background:
    Given the orchestrator is started with gRPC on port 50051 and HTTP on port 8080
    And Redis is running

  Rule: Liveness probe

    Scenario: Process alive returns 200
      Given the supervisor is running normally
      When GET /healthz is called
      Then the response is 200 with body {"status": "alive"}

    Scenario: gRPC health returns SERVING
      Given the orchestrator is healthy
      When gRPC Health.Check is called for service ""
      Then the response is SERVING

  Rule: Readiness probe

    Scenario: Not ready before agents register
      Given no agents have registered
      When GET /readyz is called
      Then the response is 503 with reason "no agents registered"

    Scenario: Ready after first agent registers
      Given one agent has registered successfully
      When GET /readyz is called
      Then the response is 200 with agents ≥ 1 and redis "ok"

    Scenario: Not ready when Redis is unavailable
      Given Redis becomes unreachable
      When GET /readyz is called
      Then the response is 503 with reason containing "redis"

  Rule: Progress probe

    Scenario: Progress reflects current state
      Given 2 agents: 1 idle and 1 working
      And 3 tasks queued, 1 in flight
      When GET /progress is called
      Then the response contains agents_idle=1, agents_working=1, tasks_queued=3, tasks_in_flight=1

  Rule: Degraded state reporting

    Scenario: gRPC health returns NOT_SERVING when no agents
      Given all agents are unhealthy
      When gRPC Health.Check is called
      Then the response is NOT_SERVING
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T7 | Health aggregator: Redis ping + gRPC listening + agent count check | Med | F2 | pending |
| T7.1 | gRPC health v1 service (Check + Watch) wired into ipc server | Med | T7 | pending |
| T7.2 | HTTP server: /healthz, /readyz, /progress handlers with JSON responses | Med | T7 | pending |
| T7.3 | Health probe tests: healthy, degraded (no agents), degraded (no Redis) | Med | T7.1, T7.2 | pending |

---

## Exit Criteria

- [ ] GET /healthz returns 200 when process is alive
- [ ] GET /readyz returns 503 before any agent registers
- [ ] GET /readyz returns 200 after first agent registers with Redis reachable
- [ ] GET /readyz returns 503 when Redis is unreachable
- [ ] GET /progress returns correct agent and task counts
- [ ] gRPC Health.Check returns SERVING / NOT_SERVING correctly
- [ ] All health probe tests pass

---

## References

- Parent spec: `specs/go-orchestrator-core-spec.md`
- F2 dependency: `specs/grpc-supervisor-spec.md`
- gRPC health protocol: `google.golang.org/grpc/health/grpc_health_v1`
- Process supervision research: `docs/research/Process-Supervision.md`

---
*Sub-feature of Task #1 · Go Orchestrator Core*
