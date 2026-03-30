# Feature: BridgeManager (Godot side)

## Overview

**User Story**: As an orchestrator operator, I want the Godot UI to automatically connect to the Go orchestrator, sync current state, and receive live updates via SSE so that the pixel office reflects real-time agent activity without manual refresh.

**Problem**: The Godot UI has no way to communicate with the Go orchestrator. Without a bridge, the pixel office is a static scene with no awareness of agent states, task assignments, or terminal output.

**Out of Scope**: Consumer nodes (FloorManager, AgentManager, OutputBuffer) — they will wire themselves to BridgeManager signals in later tasks. Authentication/API key support on the Godot side (orchestrator runs locally without auth for now). Floor-specific SSE events (the Go side only exposes agent events; floor data is sync-only).

---

## Success Condition

> This feature is complete when BridgeManager can connect to a running Go orchestrator, sync agents and floors via REST, stream live SSE events as typed Godot signals, automatically reconnect on connection loss without data loss, and send write commands (task submission, DAG submission, terminal input) back to the orchestrator.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | | | [ ] |

---

## Scope

### Must-Have
- **Typed data model**: `AgentData`, `FloorData`, `AgentOutputChunk` as `RefCounted` classes with `from_dict()` factory methods — signals carry typed objects, not raw Dictionaries
- **Initial sync via REST**: `HTTPRequest` GETs to `/api/agents` and `/api/floors` on startup; parse responses into typed arrays; emit `agent_registered` for each agent
- **SSE stream parser**: Low-level `HTTPClient` polled in `_process()`; buffer accumulation; `\n\n` delimiter splitting; extract `event:` type and `data:` JSON payload; handle `:ping` keepalive comments
- **Signal dispatch**: Map SSE `agent.registered` -> `agent_registered(AgentData)`, `agent.state_changed` -> `agent_state_changed(agent_id, old_state, new_state)`, `agent.output` -> `agent_output(AgentOutputChunk)`
- **Automatic reconnection**: 20s keepalive timeout (Go sends `:ping` every 15s); exponential backoff 1s/2s/4s/8s capped at 30s; reset on successful reconnect; cursor-based resumption via `?since=<last_cursor>` to prevent data loss
- **Connection status signals**: State enum (disconnected, connecting, connected, reconnecting); `connection_status_changed(status)` emitted at every transition
- **Write commands**: `submit_task(task_id, priority)` -> POST `/api/tasks`, `submit_dag(nodes, edges)` -> POST `/api/dags`, `send_input(agent_id, keys)` -> POST `/api/agents/{id}/input` via `HTTPRequest`

### Should-Have
- Configurable `base_url` via `@export` (already in skeleton)
- `get_agent_output(agent_id, lines)` -> GET `/api/agents/{id}/output` for on-demand terminal history

### Nice-to-Have
- Connection health metrics (latency, reconnect count) exposed as properties
- Batch initial sync (single round-trip if Go side adds a combined endpoint)

---

## Technical Plan

**Affected Components**:
- `godot/scripts/models/bridge_data.gd` — new file: `AgentData`, `FloorData`, `AgentOutputChunk` classes
- `godot/scripts/autoload/bridge_manager.gd` — existing skeleton: fill out REST sync, SSE parser, reconnect, write commands

**Data Model** (new `RefCounted` classes):

```
AgentData:
  id: String
  state: String          # idle, assigned, working, reporting, crashed
  current_task_id: String
  last_heartbeat: int
  registered_at: int
  + static from_dict(d: Dictionary) -> AgentData

FloorData:
  name: String
  agent_count: int
  + static from_dict(d: Dictionary) -> FloorData

AgentOutputChunk:
  agent_id: String
  payload: String
  timestamp: int
  + static from_dict(d: Dictionary) -> AgentOutputChunk
```

**API Contracts** (consuming Go T2 bridge):

| Method | Endpoint | Direction | Description |
|:-------|:---------|:----------|:------------|
| GET | `/api/agents` | Read | Returns `{"agents": [AgentJSON...]}` |
| GET | `/api/floors` | Read | Returns `{"floors": [FloorJSON...]}` |
| GET | `/api/agents/{id}/output?lines=N` | Read | Returns `{"output": "string"}` |
| GET | `/events/stream?since=CURSOR` | Read (SSE) | Long-lived stream; events: `agent.registered`, `agent.state_changed`, `agent.output`; keepalive `:ping` every 15s. All event payloads include a `"cursor"` field (Redis stream entry ID) for client-side `?since=` resumption. `agent.registered` carries full agent state (`id`, `state`, `registered_at`, etc.) matching `AgentJSON` schema. |
| POST | `/api/tasks` | Write | Body: `{"task_id": "...", "priority": 0.0}` |
| POST | `/api/dags` | Write | Body: `{"nodes": [...], "edges": [...]}` |
| POST | `/api/agents/{id}/input` | Write | Body: `{"keys": "..."}` |

**Internal Architecture**:

```
BridgeManager (autoload singleton)
  |
  |-- HTTPRequest nodes (child nodes, one per concurrent request type)
  |     |-- _sync_agents_request   (GET /api/agents)
  |     |-- _sync_floors_request   (GET /api/floors)
  |     |-- _command_request       (POST write commands, sequential queue)
  |
  |-- HTTPClient (_sse_client)
  |     |-- polled in _process()
  |     |-- _sse_buffer: String accumulator
  |     |-- _last_cursor: String (for ?since= resumption)
  |     |-- _last_data_time: float (for 20s keepalive timeout)
  |
  |-- State tracking
        |-- _connection_state: enum {DISCONNECTED, CONNECTING, CONNECTED, RECONNECTING}
        |-- _agent_states: Dictionary[String, String] (for old_state tracking in state_changed)
        |-- _backoff_seconds: float (current backoff delay)
        |-- _backoff_timer: float (countdown to next reconnect attempt)
```

**Lifecycle**:
1. `_ready()`: create `HTTPRequest` child nodes, start initial sync
2. Initial sync: GET agents + floors in parallel; on success -> open SSE; on failure -> retry with backoff
3. `_process(delta)`: poll `HTTPClient`, accumulate buffer, parse complete events, check keepalive timeout, manage reconnect timer

**Dependencies**: Godot 4.2, Go orchestrator running (T2 HTTP/SSE bridge)

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Godot `HTTPClient` chunked transfer handling is inconsistent across platforms | Medium | Test on Linux first (primary platform); buffer parser handles partial chunks |
| SSE reconnect during high event throughput could miss events if cursor window expires | Low | Redis stream retention is configurable server-side; default retention is generous |
| `_process()` poll loop adds per-frame overhead even when idle | Low | Early return when disconnected; SSE poll is cheap (no syscall if no data) |

---

## Acceptance Scenarios

```gherkin
Feature: BridgeManager (Godot side)
  As an orchestrator operator
  I want the Godot UI to connect to the Go orchestrator automatically
  So that the pixel office reflects real-time agent activity

  Background:
    Given the Go orchestrator is running on localhost:8080
    And BridgeManager.base_url is set to "http://localhost:8080"

  Rule: Initial sync populates current state

    Scenario: Successful initial sync
      Given the orchestrator has 2 registered agents and 1 floor
      When BridgeManager starts
      Then it GETs /api/agents and /api/floors
      And emits agent_registered for each agent with typed AgentData
      And stores floor data as Array[FloorData]
      And emits connection_status_changed("connected")

    Scenario: Initial sync fails then retries
      Given the orchestrator is unreachable
      When BridgeManager starts
      Then it emits connection_status_changed("reconnecting")
      And retries with exponential backoff (1s, 2s, 4s, 8s, cap 30s)
      And when the orchestrator becomes reachable, sync completes normally

  Rule: SSE stream delivers live updates

    Scenario: Agent state change via SSE
      Given BridgeManager is connected and SSE stream is open
      When the orchestrator emits an agent.state_changed event
      Then BridgeManager emits agent_state_changed(agent_id, old_state, new_state)

    Scenario: Agent output via SSE
      Given BridgeManager is connected and SSE stream is open
      When the orchestrator emits an agent.output event
      Then BridgeManager emits agent_output with a typed AgentOutputChunk

    Scenario: New agent registered via SSE
      Given BridgeManager is connected and SSE stream is open
      When the orchestrator emits an agent.registered event
      Then BridgeManager emits agent_registered with a typed AgentData

  Rule: Automatic reconnection on connection loss

    Scenario: SSE connection drops
      Given BridgeManager has an active SSE connection
      When no data or keepalive is received for 20 seconds
      Then BridgeManager closes the connection
      And emits connection_status_changed("disconnected")
      And reconnects with exponential backoff
      And passes ?since=<last_cursor> to resume without data loss

    Scenario: Reconnect succeeds
      Given BridgeManager is in reconnecting state
      When the SSE connection is re-established
      Then backoff resets to 1s
      And emits connection_status_changed("connected")

  Rule: Write commands reach the orchestrator

    Scenario: Submit a task
      When submit_task(task_id, priority) is called
      Then BridgeManager POSTs to /api/tasks with the correct JSON body
      And returns the response or error via callback

    Scenario: Send input to agent terminal
      When send_input(agent_id, keys) is called
      Then BridgeManager POSTs to /api/agents/{id}/input
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T3.1 | Define `AgentData`, `FloorData`, `AgentOutputChunk` RefCounted classes in `scripts/models/bridge_data.gd` with `from_dict()` factories | High | None | pending |
| T3.2 | Implement initial sync -- `HTTPRequest` GETs to `/api/agents` and `/api/floors`, parse responses into typed arrays, emit signals | High | T3.1 | pending |
| T3.3 | Implement SSE parser -- `HTTPClient` poll loop in `_process()`, buffer accumulation, `\n\n` splitting, event type + JSON data extraction | High | T3.1 | pending |
| T3.4 | Wire SSE events to signals -- map `agent.registered`, `agent.state_changed`, `agent.output` to typed signal emissions; track `old_state` for state changes | High | T3.3 | pending |
| T3.5 | Reconnect logic -- 20s keepalive timeout, exponential backoff (1/2/4/8/30s cap), cursor-based resumption via `?since=` | High | T3.3 | pending |
| T3.6 | Write commands -- `submit_task()`, `submit_dag()`, `send_input()` methods via `HTTPRequest` POSTs | Med | T3.2 | pending |
| T3.7 | Connection status management -- state enum (disconnected/connecting/connected/reconnecting), `connection_status_changed` signal emissions at all transitions | High | T3.5 | pending |

---

## Exit Criteria

- [ ] All Must-Have acceptance scenarios pass manually against a running Go orchestrator
- [ ] SSE parser handles all 3 event types (`agent.registered`, `agent.state_changed`, `agent.output`) plus keepalive comments
- [ ] Reconnect recovers within backoff schedule after orchestrator restart
- [ ] No data loss on reconnect (cursor-based resumption verified)
- [ ] Typed data objects (`AgentData`, `FloorData`, `AgentOutputChunk`) parse all fields from Go JSON responses
- [ ] Write commands (`submit_task`, `submit_dag`, `send_input`) reach Go endpoints and return responses
- [ ] No regressions on existing Godot project scaffold (T1)

---

## References

- Go HTTP/SSE bridge (T2): `internal/httpbridge/` — types.go, handlers.go, sse.go, bridge.go
- Parent feature: #86 (Phase 1 -- Skeleton Tower)
- Issue: #98
- PR: #101
- Depends on: T1 (#94, merged), T2 (#95, merged)
- Pixel Office UI spec: `specs/pixel-office-ui-spec.md`

---
*Authored by: Clault KiperS 4.6*
