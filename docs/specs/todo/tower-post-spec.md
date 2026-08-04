# Feature: Tower Post (agent-to-agent messaging)

## Overview

**User Story**: As a tower operator, I want agents to pass results and messages to each other through the orchestrator so that multi-agent pipelines run without me relaying output by hand.

**Problem**: Agents coordinate only by DAG ordering today. Task B waits for task A but never sees A's output. Agents have no way to address each other, so every cross-agent handoff routes through the user.

**Out of Scope**:
- Peer-to-peer gRPC between agents. All traffic stays hub-mediated.
- Broadcast or group channels. Messages are point to point.
- Message persistence across orchestrator restarts on the in-memory store. Durability arrives with the Redis should-have.
- Any change to the settle-heuristic completion signal.

---

## Success Condition

> This feature is complete when task B's prompt can embed task A's captured output, one agent can send a message that arrives in another agent's tmux session, and the GUI shows that message as a speech bubble between floors.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Does an interactive CLI agent send mail via a sentinel line in the pane or via an `orkctl send` helper binary on PATH? Spec assumes `orkctl` (cleaner, no pane parsing). | Claude | [ ] |
| 2 | Maximum stored output size per task before truncation (default 64 KiB, head+tail)? | Claude | [ ] |

---

## Scope

### Must-Have
- **Task output capture**: when any task completes, the orchestrator stores its final output (headless: streamed output. interactive: last settled pane snapshot). Acceptance: `GET /api/tasks/{id}/output` returns it.
- **DAG result piping**: a task description may contain `{{output:<taskID>}}`. The orchestrator substitutes the stored output before assignment. Acceptance: a two-node DAG where B's prompt contains A's answer verbatim.
- **Mailbox send**: `SendAgentMessage(from, to, body)` gRPC plus `POST /api/agents/{id}/message` on the bridge. The store keeps a per-agent inbox. Acceptance: message lands in the store with sender, body, timestamp.
- **Mailbox delivery**: the orchestrator delivers inbox messages to an interactive agent by typing `[post from <name>]: <body>` into its tmux session via the existing send-keys path. Headless agents receive pending mail prepended to their next prompt. Acceptance: text visible in the target pane.
- **`orkctl send` helper**: a small CLI (`cmd/orkctl`) an agent can call to send mail: `orkctl send --to <name-or-id> "body"`. It resolves fantasy names to IDs via the bridge. Acceptance: a claude agent instructed to run it delivers mail.
- **SSE + speech bubbles**: every delivered message emits an `agent.message` SSE event. The Godot UI renders a temporary speech bubble near the sender's character with the body (truncated ~80 chars), and the quest board logs it. Acceptance: bubble appears on send in the GUI.

### Should-Have
- **Redis-backed inboxes**: when the orchestrator runs with `RedisStore` (`internal/state/redis.go` already implements Streams via `PublishEvent`/`ReadEvents`), inboxes become per-agent streams with consumer-group delivery, surviving restarts.
- **Message history endpoint**: `GET /api/agents/{id}/messages?limit=N` for the GUI to backfill bubbles/logs.

### Nice-to-Have
- Bubble reply affordance in the GUI (click bubble, prefill chat input with `@sender`).
- Per-floor postbox prop that animates when a floor's agent receives mail.

---

## Technical Plan

**Affected Components**:
- `proto/orchestrator.proto` — `SendAgentMessage` RPC, message payload type.
- `internal/state/store.go` — `StateStore` gains task-output and inbox methods. `internal/state/memory*.go` and `redis.go` implement them.
- `internal/ipc/` — gRPC handler for the new RPC.
- `internal/dag/` — template substitution `{{output:<taskID>}}` at assignment time.
- `internal/cliagent/cliagent.go` — store settled pane snapshot as task output. prepend pending mail for headless runs.
- `internal/httpbridge/` — `POST /api/agents/{id}/message`, `GET /api/tasks/{id}/output`, `agent.message` SSE event (broker.go pattern), name-to-ID resolution reuse (`bridge.go agentName`).
- `cmd/orkctl/` — new tiny CLI, HTTP client of the bridge.
- Godot: `bridge_client` event wiring, new `speech_bubble.gd` on the UILayer (follow `world_labels.gd` projection pattern), quest board log line.

**Data Model Changes**:
- Task record gains `Output string` (truncated to cap) and `CompletedAt`.
- New `Message{ID, From, To, Body, SentAt, Delivered}` per-agent inbox list (memory) or stream (Redis).

**API Contracts**:
- `POST /api/agents/{id}/message` — body `{from, body}`. 202 on enqueue. 404 unknown agent.
- `GET /api/tasks/{id}/output` — `{output, truncated}`.
- SSE `agent.message` — `{from_id, from_name, to_id, to_name, body, timestamp, cursor}`.
- gRPC `SendAgentMessage(SendAgentMessageRequest) returns (SendAgentMessageResponse)`.

**Dependencies**: none new for must-have. Redis server only for the should-have.

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Typed-in mail corrupts an agent's in-flight TUI prompt | Medium | Deliver only when the agent state is idle. Queue otherwise, flush on state change |
| `{{output:X}}` cycles or references an incomplete task | Low | DAG validation rejects references outside declared dependencies |
| Huge outputs bloat prompts | Medium | Truncation cap with head+tail retention (open question 2) |
| Sentinel/injection: mail body containing tmux-hostile text | Medium | Reuse `send-keys -l --` literal path, strip control chars |

---

## Acceptance Scenarios

```gherkin
Feature: Tower Post
  As a tower operator
  I want agents to pass results and messages through the orchestrator
  So that multi-agent pipelines run without manual relaying

  Background:
    Given the orchestrator runs with the tmux substrate
    And two interactive agents "Emberwick" and "Thornquill" are registered

  Rule: Task outputs flow along DAG edges

    Scenario: Dependent task receives upstream output
      Given a DAG where task B depends on task A
      And task B's description contains "{{output:A}}"
      When task A completes with output "42"
      Then task B is assigned with "42" substituted into its prompt

    Scenario: Reference to a non-dependency is rejected
      Given a DAG where task B does not depend on task C
      And task B's description contains "{{output:C}}"
      When the DAG is submitted
      Then the submission fails with a validation error

  Rule: Agents exchange point-to-point mail through the hub

    Scenario: Mail reaches an idle interactive agent's session
      Given "Thornquill" is idle
      When "Emberwick" sends "review my diff" to "Thornquill"
      Then the text "[post from Emberwick]: review my diff" appears in Thornquill's tmux pane
      And an "agent.message" SSE event is emitted

    Scenario: Mail to a busy agent queues until idle
      Given "Thornquill" is working
      When "Emberwick" sends mail to "Thornquill"
      Then the message is stored undelivered
      And it is typed into the pane when "Thornquill" returns to idle

    Scenario: Mail to an unknown agent fails
      When a message is posted to agent "nobody"
      Then the bridge returns 404

  Rule: The GUI shows the conversation

    Scenario: Speech bubble on message delivery
      Given the Godot client is connected to the SSE stream
      When "Emberwick" sends mail to "Thornquill"
      Then a speech bubble with the message body appears near Emberwick's character
      And the quest board logs the exchange
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T1   | Store: task Output field + inbox methods on StateStore (memory impl + tests) | High | None | pending |
| T2   | Capture output on completion (cliagent settled snapshot, sim/headless stream tail) | High | T1 | pending |
| T3   | DAG `{{output:id}}` substitution + dependency validation | High | T2 | pending |
| T4   | Proto + gRPC `SendAgentMessage`, ipc handler | High | T1 | pending |
| T5   | Bridge: message POST, task output GET, `agent.message` SSE | High | T4 | pending |
| T6   | Delivery loop: idle-gated tmux typing, headless prompt prepend | High | T5 | pending |
| T7   | `cmd/orkctl` send helper + name resolution | Med | T5 | pending |
| T8   | Godot: speech bubbles + quest board log | High | T5 | pending |
| T9   | Redis inbox streams (should-have) | Med | T4 | pending |
| T10  | Message history endpoint + GUI backfill (should-have) | Med | T5 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI
- [ ] No regressions on related features (chat panel, summon flow, DAG execution)
- [ ] API contracts match implementation
- [ ] `go test -race ./...` green, Godot headless suites green
- [ ] Manual: two live claude agents exchange mail end to end in the GUI

---

## References

- Related specs: `go-orchestrator-core-spec.md`, `terminal-substrate-spec.md`
- Research: `docs/research/Agentic-Orchestrator-MOC.md` (IPC decision table)

---
*Authored by: Clault KiperF 5.0*
