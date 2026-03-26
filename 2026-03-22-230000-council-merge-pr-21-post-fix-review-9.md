---
## Adversarial Council — Merge PR #21 (feat: E2E lifecycle tests) — Council 9 Post-Fix Review

> Convened: 2026-03-22T23:00:00Z | Advocates: 2 | Critics: 1 | Rounds: 3/4 | Motion type: CODE

### Motion

PR #21 (feat: E2E lifecycle tests) is ready to merge into epic/1-implement-go-orchestrator-core. This is Council 9 — a post-fix review after Council 8 addressed GetAgentFields failure re-enqueue and error injection tests. This council evaluates whether the PR is now ready to merge.

---

### Advocate Positions

**ADVOCATE-1**: Argued FOR based on three pillars: (1) all nine prior council defects verified fixed with cited evidence across councils 5–8, (2) 15 E2E scenarios passing race-clean under `go test -tags testenv -race -count=1`, and (3) a principled three-tier error handling hierarchy (state transitions critical → task preservation high-priority → metadata best-effort). Conceded the error tier classification of `SetAgentFields` failure at `supervisor.go:300-301` was incorrect (tier 2 consequence, not tier 3), that Pillar 1 omitted the `crashAgent` GetAgentFields failure path, and that CRITIC-1's mutex-scope analysis invalidated the stale-data counter-argument. Final position: CONDITIONAL FOR with the pre-read fix.

**ADVOCATE-2**: Argued FOR on five structural pillars: clean StateStore interface design, correct three-layer concurrency (sv.mu RWMutex → per-agent Mutex → store-level Mutex) with no lock-during-IO, build-tag-isolated test infrastructure, disciplined gRPC system boundaries, and consistent extensibility via functional options. Conceded two test coverage gaps (no scenarios for `crashAgent` GetAgentFields failure or `tryAssignTask` SetAgentFields failure), acknowledged both issues are in-scope (introduced by council remediation commits 707ce7e and efea61a), and accepted that the pre-read + early-return fix is strictly superior. Also raised the "zombie-agent problem" counterargument against a naive re-enqueue fix for Objection 2. Final position: CONDITIONAL FOR with the pre-read fix.

---

### Critic Positions

**CRITIC-1**: Raised three objections: (1) silent task loss in `crashAgent` when `GetAgentFields` fails after `ApplyEvent` at `supervisor.go:217-222`, (2) unrecoverable task when `SetAgentFields` fails in `tryAssignTask` at `supervisor.go:300-301` followed by agent crash, and (3) unbounded dequeue-fail-reenqueue spin loop under persistent errors. Conceded Objection 2 as non-blocking after the advocates demonstrated the zombie-agent problem (re-enqueue leaves agent ASSIGNED with no task → dual ownership or stuck agent). Held Objection 1 as blocking with a concrete ~5-line pre-read + early-return fix that all parties verified as safe under the per-agent mutex scope. Final position: CONDITIONAL FOR with Objection 1 fix.

---

### Questioner Findings

The QUESTIONER did not submit probes during this council. All claims were verified directly by the ARBITER via code inspection and by cross-examination between the debaters.

---

### Key Conflicts

**Objection 1 severity — blocking vs. non-blocking (CRITIC-1 vs. Advocates)**:
CRITIC-1 identified silent task loss in `crashAgent` at `supervisor.go:217-222` — if `GetAgentFields` fails after `ApplyEvent` succeeds, the task is permanently lost with no log. Advocates initially argued: (a) the failure window is narrow (same goroutine, store read just succeeded at `checkHeartbeats:157`), (b) stale-data risk if fields are pre-read, and (c) the fix changes the function signature. CRITIC-1 rebutted: (a) probability does not override correctness for task preservation, (b) the per-agent mutex held from `supervisor.go:189` eliminates stale-data risk — no concurrent modification of `CurrentTaskID` is possible, and (c) the fix is ~5 lines with no signature change. ARBITER independently verified the mutex scope. Both advocates conceded the mutex analysis and accepted the fix as a blocking condition. **Resolved: blocking, fix in-PR.**

**Objection 2 severity — blocking vs. non-blocking (CRITIC-1 initial vs. Advocates)**:
CRITIC-1 initially classified `SetAgentFields` failure in `tryAssignTask` at `supervisor.go:300-301` as blocking. ADVOCATE-2 raised the zombie-agent problem: re-enqueueing the task leaves the agent ASSIGNED with no task, potentially causing dual-ownership if another agent picks up the re-enqueued task. ADVOCATE-1 added: the failure requires a double-fault (write failure + subsequent crash). CRITIC-1 conceded the fix requires design work and downgraded to non-blocking. **Resolved: non-blocking, track as follow-up.**

**Error tier classification (CRITIC-1 vs. ADVOCATE-1)**:
ADVOCATE-1 classified `SetAgentFields` failure at `supervisor.go:300` as tier 3 (best-effort metadata). CRITIC-1 challenged: the *consequence* is tier 2 (task loss), because a subsequent crash reads `CurrentTaskID=""` and skips re-enqueue. ADVOCATE-1 conceded the misclassification. **Resolved: ADVOCATE-1 conceded.**

---

### Concessions

- **ADVOCATE-1** conceded the error tier misclassification (`SetAgentFields` failure is tier 2, not tier 3)
- **ADVOCATE-1** conceded Pillar 1 omitted the `crashAgent` GetAgentFields failure path
- **ADVOCATE-1** conceded the stale-data argument does not apply within the per-agent mutex scope
- **ADVOCATE-1** conceded the pre-read + early-return fix is strictly superior to the current post-read approach
- **ADVOCATE-2** conceded two test coverage gaps (crashAgent GetAgentFields, tryAssignTask SetAgentFields)
- **ADVOCATE-2** conceded both issues are in-scope (introduced by council remediation commits)
- **ADVOCATE-2** conceded the stale-data argument does not apply within the per-agent mutex scope
- **CRITIC-1** conceded Objection 2 to non-blocking (zombie-agent problem makes naive fix unsafe)
- **CRITIC-1** conceded Objection 3 was non-blocking from the start

---

### Regression Lineage

Council 9's Objection 1 is a direct descendant of Council 8's Defect A (GetAgentFields failure re-enqueue in `tryAssignTask`). Council 8 fixed the `tryAssignTask` path but the analogous path in `crashAgent` was not addressed. Council 7 introduced the `crashAgent` re-enqueue logic (commit 707ce7e) as part of the mutex race fix — the error handling gap was present from that commit. The fix identified by Council 9 completes the error-handling pattern across both code paths.

Councils 7 and 8's fixes (commits 707ce7e and efea61a) remain verified and intact. No regressions from those fixes were identified.

---

### Evidence Verified by ARBITER

| Finding | File:Line | Verified? | Assessment |
|:--------|:----------|:----------|:-----------|
| Silent GetAgentFields fall-through in crashAgent | supervisor.go:217-222 | **YES** | BUG (blocking) |
| Per-agent mutex held across entire crashAgent body | supervisor.go:189-190 | **YES** | Confirms pre-read is safe |
| SetAgentState does not modify CurrentTaskID | mock.go:52-54 | YES | Confirms pre-read value remains valid after ApplyEvent |
| tryAssignTask acquires same per-agent mutex before SetAgentFields | supervisor.go:282 | YES | Confirms no concurrent modification |
| SetAgentFields failure in tryAssignTask logs but doesn't recover | supervisor.go:300-301 | YES | Non-blocking (double-fault + zombie-agent) |
| checkHeartbeats reads GetAgentFields before crashAgent | supervisor.go:157 | YES | Bounds failure window probability |
| Council 7 tryAssignTask mutex fix | supervisor.go:272-313 | YES | Condition met |
| Council 8 GetAgentFields re-enqueue fix | supervisor.go:303-311 | YES | Condition met |
| Council 6 findIdleAgent RLock re-check | supervisor.go:359-367 | YES | Condition met |
| CompleteAgent error propagation | handlers.go:107-109 | YES | Condition met |
| SetAgentFields error logging in completeAgent | supervisor.go:404-406 | YES | Condition met |
| All tests pass (6 packages, 0 failures) | `go test ./...` | YES | Clean |
| 15 E2E scenarios pass race-clean | `go test -tags testenv -race` | YES | Clean |

---

### Scope Audit

| Finding | Relevance | Pre-existence | Disposition |
|:--------|:----------|:-------------|:------------|
| crashAgent GetAgentFields silent loss | PASSES — supervisor.go in PR scope | PASSES — introduced in 707ce7e | IN-PR BUG (blocking) |
| tryAssignTask SetAgentFields task loss | PASSES — supervisor.go in PR scope | PASSES — introduced in 707ce7e | Non-blocking (fix requires design work) |
| Dequeue-reenqueue spin loop | PASSES — supervisor.go in PR scope | PASSES — pattern present in current code | Non-blocking (production hardening) |

---

### Arbiter Recommendation

**CONDITIONAL**

Council 9 confirms that all blocking conditions from Councils 5–8 are satisfied. The E2E test suite (15 scenarios, in-process gRPC via bufconn, race-clean, build-tag-isolated) provides comprehensive lifecycle coverage. The concurrency design — three-layer locking with snapshot-then-release in `findIdleAgent` — is confirmed correct by all parties.

One new blocking defect was identified and unanimously agreed upon: `crashAgent` at `supervisor.go:217-222` silently loses tasks when `GetAgentFields` fails after `ApplyEvent` succeeds. The fix — pre-reading `GetAgentFields` before `ApplyEvent` and using the cached `CurrentTaskID` for re-enqueue — is safe because the per-agent mutex (held from line 189) prevents concurrent modification. This fix also changes the failure mode from "task permanently lost, no retry" to "crash deferred, heartbeat loop retries next tick," which is strictly superior. All three debaters independently verified the mutex analysis and accepted the fix.

Two non-blocking concerns (tryAssignTask SetAgentFields failure causing task loss under double-fault conditions, and dequeue-reenqueue spin loop under persistent errors) are trackable as follow-up issues on the epic branch.

---

### Conditions

All conditions are blocking. PR must not merge until all are addressed.

#### Condition 1: Pre-read `GetAgentFields` in `crashAgent` (supervisor.go)

Replace the current post-`ApplyEvent` `GetAgentFields` read at `supervisor.go:216-222` with a pre-read + early-return pattern. The per-agent mutex is held from line 189 — no concurrent modification of `CurrentTaskID` is possible.

```go
func (sv *Supervisor) crashAgent(ctx context.Context, agentID string) {
    mu := sv.getAgentMutex(agentID)
    if mu == nil {
        return
    }
    mu.Lock()
    defer mu.Unlock()

    // Pre-read task binding before transitioning — if store is degraded,
    // skip the crash so the heartbeat loop can retry next tick.
    preFields, preErr := sv.store.GetAgentFields(ctx, agentID)
    if preErr != nil {
        log.Printf("supervisor: crashAgent %s — GetAgentFields failed, deferring crash: %v", agentID, preErr)
        return
    }

    // Pre-populate cooldown sentinel so findIdleAgent skips this agent.
    sv.mu.Lock()
    sv.agentCooldown[agentID] = time.Now().Add(24 * time.Hour)
    sv.mu.Unlock()

    snap, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventAgentFailed)
    if err != nil {
        sv.mu.Lock()
        delete(sv.agentCooldown, agentID)
        sv.mu.Unlock()
        return
    }

    if snap.PreviousState == agent.StateIdle {
        sv.mu.Lock()
        delete(sv.agentCooldown, agentID)
        sv.mu.Unlock()
        return
    }

    // Re-enqueue using pre-read CurrentTaskID (per-agent mutex held —
    // no concurrent modification possible since pre-read).
    if preFields.CurrentTaskID != "" {
        if err := sv.store.EnqueueTask(ctx, preFields.CurrentTaskID, preFields.CurrentTaskPriority); err != nil {
            log.Printf("supervisor: task %s lost — re-enqueue failed (agent %s crashed): %v", preFields.CurrentTaskID, agentID, err)
        }
    }

    decision := sv.policy.RecordCrash(agentID)
    // ... rest unchanged ...
}
```

This fix:
- Pre-reads `GetAgentFields` before `ApplyEvent`, caching `CurrentTaskID` locally
- Returns early if the pre-read fails — the agent is NOT transitioned, so the heartbeat loop retries next tick
- Uses cached `CurrentTaskID` for re-enqueue instead of a post-read
- Same store op count (4), same lock duration, no signature change
- Failure mode changes from "task permanently lost" to "crash deferred, automatic retry"

#### Condition 2: Add Scenario 16 — GetAgentFields failure during crash recovery

Add an E2E scenario to `lifecycle_test.go` that:
1. Registers an agent, submits a task, waits for assignment
2. Advances the agent to WORKING
3. Injects `GetAgentFields` error via `store.SetGetAgentFieldsError`
4. Triggers stale heartbeat (or uses `CrashAgentForTest`)
5. Verifies the crash is deferred (agent stays WORKING, not transitioned to IDLE)
6. Clears the error
7. Verifies the next heartbeat tick successfully crashes the agent and re-enqueues the task

---

### Suggested Fixes

#### Bug Fixes (blocking, always in-PR)

- **supervisor.go:216-222 — crashAgent pre-read + early-return**: See Condition 1 above. Replace post-ApplyEvent GetAgentFields with pre-read pattern.
  CITE: `internal/supervisor/supervisor.go` L:217

#### In-PR Improvements (scoped, non-bug)

None beyond the conditions above.

#### PR Description Amendments

None required.

#### Critical Discoveries (informational)

None identified. All findings are directly about the motion.

---

### Follow-Up Issues (non-blocking, track on epic branch)

- **tryAssignTask SetAgentFields failure task loss** (`supervisor.go:300-301`): If `SetAgentFields` fails after `ApplyEvent` succeeds, the agent is ASSIGNED with empty `CurrentTaskID`. A subsequent crash silently loses the task. The naive fix (re-enqueue) creates a zombie-agent problem. Requires design work: either revert the state transition on write failure or store the task-agent binding outside `AgentFields`. Add accompanying E2E scenario.
  CITE: `internal/supervisor/supervisor.go` L:300

- **Dequeue-reenqueue spin loop** (`supervisor.go:256-270`): Under persistent store errors, `tryAssignTask` enters a tight dequeue-fail-reenqueue loop bounded only by `taskPollInterval`. Add exponential backoff on consecutive re-enqueue cycles.
  CITE: `internal/supervisor/supervisor.go` L:256

---

### Summary Table

| Finding | Source | Severity | Triage |
|:--------|:-------|:---------|:-------|
| crashAgent GetAgentFields silent task loss | CRITIC-1 | BUG | Fix in-PR (Condition 1) |
| crashAgent GetAgentFields test coverage | CRITIC-1 | TEST GAP | Fix in-PR (Condition 2) |
| tryAssignTask SetAgentFields task loss | CRITIC-1 | BUG (double-fault) | Non-blocking follow-up |
| Dequeue-reenqueue spin loop | CRITIC-1 | RESOURCE | Non-blocking follow-up |
| Error tier misclassification | CRITIC-1 | CONCEDED | Acknowledged by ADVOCATE-1 |
| Council 7 tryAssignTask mutex fix | ARBITER verified | CONDITION MET | No action needed |
| Council 8 GetAgentFields re-enqueue fix | ARBITER verified | CONDITION MET | No action needed |
| Council 6 findIdleAgent RLock re-check | ARBITER verified | CONDITION MET | No action needed |
| CompleteAgent error propagation | ARBITER verified | CONDITION MET | No action needed |
