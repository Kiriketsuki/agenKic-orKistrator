# Feature: Power Controls

## Overview

**User Story**: As the tower keeper, I want to banish agents and restart the orchestrator from the GUI so that I never touch the shell for lifecycle control.

**Problem**: No endpoint deletes a single agent, so dead agents linger as tmux sessions and tower floors. Restarting the orchestrator requires the shell, and the GUI keeps stale agents afterward because the server never emits `agent.deregistered`.

**Out of Scope**: Task requeue on banish (the task drops, by decision). Auth tiers for the admin endpoint (the existing single API key covers it). External process supervisors.

---

## Success Condition

> This feature is complete when the GUI can banish an agent with full cleanup on both sides, and the GUI can restart the orchestrator with the tower reconnecting on its own.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Requeue or drop the in-flight task | brainstorm | [x] Drop |

---

## Scope

### Must-Have
- `POST /api/agents/{id}/despawn`: send Ctrl-C to the pane, drop the current task, destroy the tmux session `agent-<id>`, remove the agent from the supervisor, delete it from the store, delete the bridge name and provider (and floor) entries, publish `agent_deregistered`.
- New `mapStoreEvent` case turns `agent_deregistered` into the `agent.deregistered` SSE event. The Godot side already consumes it and cleans the tower, overlays, and panels.
- `POST /api/admin/restart`: run the graceful shutdown sequence, then re-exec the same binary with the same arguments. The SSE client reconnects on its own.
- Godot: "Banish" item in the agent context menu. Despawn drops the task, so the item carries a confirm when the agent is WORKING.
- Power flyout: BANISH ALL AGENTS and RESTART ORCHESTRATOR behind confirm dialogs, SETTINGS (opens the sigil config page), QUIT TO TITLE.
- BridgeManager gains `despawn_agent(id)` and `restart_orchestrator()` through the existing command queue.

### Should-Have
- Toast feedback on banish and restart, matching the cancel and reassign toasts.

### Nice-to-Have
- A dissolve animation on the banished agent's sprite before the floor updates.

---

## Technical Plan

**Affected Components**:
- Go: `internal/httpbridge/handlers.go` (new despawn and restart handlers, cancel logic at :340 reused for the Ctrl-C and task-drop step), `internal/httpbridge/bridge.go` (registry deletes at :160-190, routes at :124), `internal/httpbridge/sse.go` (`mapStoreEvent` at :180), `internal/supervisor/supervisor.go` (new per-agent `RemoveAgent`. `Stop()` at :161 is the only session destroyer today), `internal/state/store.go` (`DeleteAgent` at :108, currently unused), `cmd/orchestrator/main.go` (shutdown goroutine at :158-175 extracted into a reusable func, re-exec via `syscall.Exec(os.Args[0], os.Args, os.Environ())`)
- Godot: `godot/scripts/overlays/agent_context_menu.gd` (item ids at :16-25, dispatch at :163, toasts at :268), `godot/scripts/ui/power_flyout.gd` (new), `godot/scripts/autoload/bridge_manager.gd` (`_enqueue_command` at :573, POST only, no verb change needed)

**Data Model Changes**: New store event type `agent_deregistered` with `{agent_id}`.

**API Contracts**:
- `POST /api/agents/{id}/despawn`: no body. 200 on success, 404 for an unknown agent. Idempotent: a second call on a gone agent returns 404 and changes nothing.
- `POST /api/admin/restart`: no body. Responds 202, then restarts. Behind the same API key middleware as every route.

**Dependencies**: F2 Orb Flock for the Power flyout. The endpoints and the context-menu Banish item have no F2 dependency and can land first.

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Re-exec loses the listening sockets mid-request | Medium | Respond 202 first, then shut down and re-exec from a goroutine after a short delay |
| Partial despawn on a mid-step failure leaves a half-removed agent | Medium | Order the steps so the tmux kill and store delete run before registry deletes. Log and continue on each step, then always publish the event |
| Banish-all racing new spawns | Low | Banish-all iterates a snapshot of the agent list. New spawns after the snapshot survive |

---

## Acceptance Scenarios

```gherkin
Feature: Power Controls
  As the tower keeper
  I want GUI lifecycle control
  So that I never touch the shell

  Background:
    Given the tower runs with two live agents

  Rule: Banish removes one agent everywhere

    Scenario: Banish an idle agent
      When the keeper picks Banish on agent A's context menu
      Then the bridge kills tmux session "agent-A"
      And the store and supervisor forget agent A
      And the SSE stream carries agent.deregistered for A
      And the tower, overlays, and panels drop agent A

    Scenario: Banish a working agent
      Given agent A is WORKING on a task
      When the keeper picks Banish and confirms
      Then the task is dropped, not requeued
      And agent A is removed as above

    Scenario: Banish an unknown agent
      When despawn is called for an id that does not exist
      Then the bridge returns 404 and nothing changes

  Rule: Restart survives from the GUI

    Scenario: Restart the orchestrator
      When the keeper picks RESTART ORCHESTRATOR and confirms
      Then the bridge responds 202
      And the orchestrator shuts down gracefully and re-execs
      And the GUI reconnects its SSE stream without a manual step

    Scenario: Banish all
      When the keeper picks BANISH ALL AGENTS and confirms
      Then every live agent is despawned
      And the tower shows no agents
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T1 | Go: `Supervisor.RemoveAgent` with per-agent session destroy and map cleanup | High | None | pending |
| T2 | Go: despawn handler (cancel step, T1, store delete, registry deletes, event publish) plus httptest coverage | High | T1 | pending |
| T3 | Go: `mapStoreEvent` case for `agent_deregistered` | High | T2 | pending |
| T4 | Go: extract the shutdown func, restart handler with 202-then-re-exec, httptest for the 202 | High | None | pending |
| T5 | Godot: Banish item in the context menu with WORKING confirm plus toasts | High | T2 | pending |
| T6 | Godot: `despawn_agent` and `restart_orchestrator` in BridgeManager | High | None | pending |
| T7 | Godot: Power flyout with Banish All, Restart, Settings, Quit to title, confirm dialogs | High | F2, T6 | pending |
| T8 | Live verification: banish and restart from the GUI, reconnect observed | High | T5, T7 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI
- [ ] No regressions on related features
- [ ] API contracts match implementation
- [ ] Five Godot headless suites and `go test -race ./...` stay green
- [ ] Live GUI verification by the keeper

---

## References

- Related specs: `orb-flock-spec.md`, `grimoire-summoning-spec.md` (shared floor registry)
- Mockups: `.brainstorm/1776334-1785821252/content/feature-mockups.html` (F4 section)

---
*Authored by: Clault KiperF 5.0*
