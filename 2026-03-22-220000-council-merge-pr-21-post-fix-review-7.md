---
## Adversarial Council — Merge PR #21 (feat: E2E lifecycle tests) — Council 7 Post-Fix Review

> Convened: 2026-03-22T22:00:00Z | Advocates: 2 | Critics: 2 | Rounds: 2+/4

### Motion

PR #21 is ready to merge into epic/1-implement-go-orchestrator-core.

This is Council 7 — a post-fix review. Council 6 issued CONDITIONAL FOR on one condition: add a fresh RLock re-check in `findIdleAgent` to close the snapshot race introduced by the Council 5 fix. Commit `683d2ff` claims to address that condition plus three others (snapshot race, priority preservation, crash recovery, CompleteAgent RPC). This council evaluates whether the condition is met and whether new defects have been introduced.

---

### Council History

Condition from Council 6:
> Add a fresh RLock re-check after the snapshot-based filter passes and GetAgentState confirms IDLE, covering the window where crashAgent could complete between RUnlock and GetAgentState. 8-line block specified verbatim.

Status verified by ARBITER: the fix is applied at supervisor.go:333-341 and matches the specified block exactly. Council 6's sole blocking condition is **satisfied**.

---

### Advocate Positions

**ADVOCATE-1**: All Council 6 conditions are satisfied. The double-check revalidation at supervisor.go:333-341 closes the snapshot race exactly as specified. The E2E suite exercises 13 scenarios via an in-process gRPC stack (bufconn, real serialization), test exports are gated behind `//go:build testenv`, and six prior councils have been remediated. Conceded CompleteAgent RPC silent failure (handlers.go:107 returns success unconditionally) as a genuine issue with bounded impact — no state corruption, but misleading to the caller.

**ADVOCATE-2**: StateStore interface extensions are minimal and correctly isolated. Layered concurrency design (per-agent mutex, pre-population sentinel, TOCTOU guard, RLock/IO decoupling, double-check revalidation) is structurally sound. Test infrastructure is idiomatic Go with no production footprint.

---

### Critic Positions

**CRITIC-1 (strongest new finding)**:
1. **CRITICAL RACE**: `tryAssignTask` calls `sv.applyEvent(ctx, agentID, EventTaskAssigned)` at supervisor.go:272. `applyEvent` (supervisor.go:387-398) acquires the per-agent mutex internally and releases it before returning. The subsequent `GetAgentFields`/`SetAgentFields` at supervisor.go:283-287 execute **outside any per-agent mutex**. In the window between applyEvent's mutex release and the SetAgentFields call, `crashAgent` can:
   - Acquire the per-agent mutex
   - Call `machine.ApplyEvent(EventAgentFailed)` — valid from ASSIGNED state, transitions to StateIdle
   - Call `GetAgentFields` — reads `CurrentTaskID=""` (not yet written)
   - No task to re-enqueue → **task permanently lost**
   - Release mutex
   Then `tryAssignTask` continues, sets `CurrentTaskID=taskID` with stale `State` — writing inconsistent state to the store.
   This is an asymmetry with `completeAgent`: that function acquires the per-agent mutex at the top of the function body (supervisor.go:364-365) and holds it through both `machine.ApplyEvent` (line 367) and `SetAgentFields` (line 378). `tryAssignTask` delegates to `sv.applyEvent` which manages its own mutex, leaving the SetAgentFields write unprotected.
2. **Silent error discards**: `_ = sv.store.SetAgentFields(ctx, agentID, cur)` at supervisor.go:286 (task ID never persisted if write fails → silent crash-recovery failure) and supervisor.go:378 (CurrentTaskID not cleared → duplicate task execution on next crash).
3. Scenarios 8-9 bypass `crashAgent` entirely and do not test the integrated supervisor-policy path.

**CRITIC-2**:
1. Scope creep: production changes (supervisor, handlers, state) bundled into a PR labeled as E2E tests.
2. 11 of 13 scenarios use `ApplyEventForTest` bypassing the real gRPC codepath.
3. Scenarios 8-9 call `policy.RecordCrash` directly — unit tests in E2E costume.
4. **CompleteAgent RPC silently discards state transition failures** (handlers.go:107): `s.supervisor.CompleteAgent(ctx, req.AgentId)` followed unconditionally by `return &pb.CompleteAgentResponse{}, nil`. Conceded by ADVOCATE-1.
5. Timing-dependent assignment detection in Scenario 13 (lifecycle_test.go:803): `time.Sleep(100ms)` determines which agent to check.

---

### Evidence Verified by ARBITER

All claims below were verified by direct code inspection of the feature branch.

| Finding | File:Line | Verified? | Assessment |
|:--------|:----------|:----------|:-----------|
| CRITIC-1 race: SetAgentFields outside per-agent mutex | supervisor.go:272-287 | **YES** | CRITICAL BUG |
| completeAgent correctly holds mutex through SetAgentFields | supervisor.go:364-379 | YES — confirms asymmetry | Correct pattern |
| crashAgent correctly holds mutex through entire operation | supervisor.go:184-239 | YES | Correct pattern |
| Silent error at tryAssignTask SetAgentFields | supervisor.go:286 | **YES** | BUG |
| Silent error at completeAgent SetAgentFields | supervisor.go:378 | **YES** | BUG |
| CompleteAgent RPC returns success unconditionally | handlers.go:107-108 | **YES** | BUG (conceded) |
| Council 6 re-check fix applied | supervisor.go:333-341 | YES — matches specified block | Condition satisfied |
| TOCTOU guard | supervisor.go:209-214 | YES — correctly placed | Valid |
| Scenario 13 sleep | lifecycle_test.go:803 | YES — determines assignment, not assertion | Minor quality issue |
| Real assertion via pollAgentState | lifecycle_test.go:822 | YES — polling-based | Valid |
| Test exports behind testenv build tag | export_e2e.go:1, lifecycle_test.go:1 | YES | No production footprint |
| circuit breaker logic | restart.go:97-113 | YES — threshold > check, correct | Valid |

**Key asymmetry confirmed**: `tryAssignTask` calls `sv.applyEvent` which manages its own mutex (acquires+releases before returning), then does `GetAgentFields`/`SetAgentFields` OUTSIDE that mutex. `completeAgent` takes the opposite approach: acquires the mutex at the function top, calls `sv.machine.ApplyEvent` directly, and holds the mutex through the `SetAgentFields` write. This is not a stylistic difference — it is the structural cause of the race.

---

### Key Conflicts

**tryAssignTask race disposition (CRITIC-1 vs. Advocates)**:
CRITIC-1 identified a race in tryAssignTask — the per-agent mutex is released by `applyEvent` before the `SetAgentFields` write, creating a window where `crashAgent` can interleave. Advocates argued the concurrency design is structurally sound. ARBITER verdict: race is confirmed by code inspection. The asymmetry between `completeAgent` (mutex held top-to-bottom) and `tryAssignTask` (mutex released by `applyEvent` before SetAgentFields) is the root cause. Under the Fix Triage Protocol, in-scope bugs are fixed in-PR unconditionally.

**Scope creep (CRITIC-2 vs. ADVOCATE-1)**:
CRITIC-2 argued production changes are bundled into a test PR. ADVOCATE-1 countered that the production changes are mechanisms the tests exercise and cannot be decoupled without orphaned PRs. ARBITER scope audit: the production changes are load-bearing for the E2E scenarios; splitting would create an unlanded test suite. This is a process concern, not a blocking defect. Does not meet Critical Discovery threshold. Dropped as a blocking concern.

**CompleteAgent RPC silent failure (CRITIC-2, conceded by ADVOCATE-1)**:
Both sides agree this is a genuine issue. Severity is bounded — no state corruption, but the caller cannot distinguish success from failure. Fix Triage: confirmed bug → fix in-PR.

**Scenarios 8-9 test placement (CRITIC-2)**:
ADVOCATE-1 conceded the placement concern but noted the design is explicitly documented in lifecycle_test.go:5-8. Scenarios 10-12 exercise the full integrated `CrashAgentForTest` path. This is a test quality observation, not a blocking defect.

**Scenario 13 sleep (CRITIC-2)**:
`time.Sleep(100ms)` at lifecycle_test.go:803 determines which agent was assigned (test logistics). The actual correctness assertion is `pollAgentState` at line 822 (polling-based). Minor quality issue — add an explanatory comment, not a bug.

---

### Concessions

- **ADVOCATE-1** conceded CompleteAgent RPC silent failure (handlers.go:107) as a genuine issue.
- **ADVOCATE-1** conceded Scenarios 8-9 placement concern; acknowledged it's documented in comments.
- **CRITIC-2** did not substantiate scope-creep as a blocking defect (acknowledged the coupling argument).

---

### QUESTIONER Findings

| Claim | Agent | Status |
|:------|:------|:-------|
| "The race cannot occur because the per-agent mutex prevents interleaving" | Implicit advocate position | **Unsubstantiated** — `applyEvent` releases the mutex before returning; SetAgentFields is demonstrably outside any mutex at supervisor.go:283-287 |
| "CompleteAgent failure is bounded — no state corruption" | ADVOCATE-1 | Substantiated — `completeAgent` returns without state writes on `ApplyEvent` error (supervisor.go:367-370); the corrupt path is the missed clear, not a corrupt transition |
| "Scenario 13's real assertion is pollAgentState, not the sleep" | ADVOCATE-1 | Substantiated — lifecycle_test.go:822 uses `pollAgentState` |
| "Scenarios 10-12 cover the full integrated crash path" | ADVOCATE-1 | Substantiated — lifecycle_test.go:628, 687, 736 use `CrashAgentForTest` |

---

### Scope Audit

All findings assessed against relevance test (directly about the motion) and pre-existence test (would not exist without this PR or its prior remediations):

| Finding | Relevance | Pre-existence | Disposition |
|:--------|:----------|:-------------|:------------|
| Race in tryAssignTask | PASSES — supervisor.go is within PR scope | PASSES — pattern present in current branch code | IN-PR BUG (blocking) |
| Silent SetAgentFields errors | PASSES — same file, same PR scope | PASSES — present in current code | IN-PR BUG (blocking) |
| CompleteAgent RPC silent failure | PASSES — handlers.go is in PR scope | PASSES — present in current code | IN-PR BUG (blocking) |
| Scenario 13 sleep | PASSES — lifecycle_test.go is in PR | PASSES | IN-PR IMPROVEMENT (non-blocking) |
| Scope creep process concern | FAILS relevance (process, not correctness) | — | DROPPED |
| Scenarios 8-9 unit-in-E2E placement | FAILS pre-existence (would exist without this PR's bugs) | — | INFORMATIONAL |

---

### Arbiter Recommendation

**CONDITIONAL**

Council 6's sole blocking condition — the fresh RLock re-check in `findIdleAgent` — is satisfied at supervisor.go:333-341. The E2E test suite provides genuine value: 13 scenarios, in-process gRPC stack via bufconn, polling-based assertions, and test exports correctly gated behind `//go:build testenv`.

However, Council 7 has identified three confirmed bugs that must be fixed in-PR per the Fix Triage Protocol:

1. **Critical race in `tryAssignTask`**: The per-agent mutex is released by `applyEvent` before the `GetAgentFields`/`SetAgentFields` write at supervisor.go:283-287. `crashAgent` can interleave in this window, read `CurrentTaskID=""`, and permanently lose the task. The fix is to inline mutex acquisition in `tryAssignTask` and hold it through the `SetAgentFields` write — the same pattern `completeAgent` correctly uses.

2. **Silent error discards on `SetAgentFields`** at supervisor.go:286 and supervisor.go:378: if either write fails silently, crash-recovery data is lost or stale task IDs survive. Log the errors at minimum.

3. **`CompleteAgent` RPC always returns success** (handlers.go:107): caller cannot detect failure. Change `CompleteAgent`/`completeAgent` to return an error and propagate it in the handler.

The CONDITIONAL targets production code only. The test infrastructure itself is sound.

---

### Conditions

All three are blocking. PR must not merge until all are addressed.

#### Condition 1: Fix race in `tryAssignTask` (supervisor.go)

Replace the current `tryAssignTask` assignment block (supervisor.go:272-294) so the per-agent mutex is held across both `machine.ApplyEvent` and `SetAgentFields`. Pattern mirrors `completeAgent`:

```go
// Acquire per-agent mutex for the entire assign+persist operation, preventing
// crashAgent from interleaving between the state transition and CurrentTaskID write.
mu := sv.getAgentMutex(agentID)
if mu == nil {
    if err := sv.store.EnqueueTask(ctx, taskID, priority); err != nil {
        log.Printf("supervisor: task %s lost — re-enqueue failed (agent unregistered): %v", taskID, err)
    }
    return
}
mu.Lock()
snap, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventTaskAssigned)
if err != nil {
    mu.Unlock()
    if err := sv.store.EnqueueTask(ctx, taskID, priority); err != nil {
        log.Printf("supervisor: task %s lost — re-enqueue failed (assign error): %v", taskID, err)
    }
    return
}
// Record the assigned task INSIDE the mutex so crashAgent reads a consistent CurrentTaskID.
if cur, fErr := sv.store.GetAgentFields(ctx, agentID); fErr == nil {
    cur.CurrentTaskID = taskID
    cur.CurrentTaskPriority = priority
    if err := sv.store.SetAgentFields(ctx, agentID, cur); err != nil {
        log.Printf("supervisor: task %s — CurrentTaskID not persisted (agent %s): %v", taskID, agentID, err)
    }
}
mu.Unlock()
```

Remove the now-redundant `sv.applyEvent` call and the separate SetAgentFields block at supervisor.go:283-287. The `applyEvent` helper remains available for other callers.

#### Condition 2: Log `SetAgentFields` errors (supervisor.go)

In `completeAgent` at supervisor.go:375-379, replace:
```go
_ = sv.store.SetAgentFields(ctx, agentID, cur)
```
with:
```go
if err := sv.store.SetAgentFields(ctx, agentID, cur); err != nil {
    log.Printf("supervisor: CurrentTaskID not cleared for agent %s — duplicate re-enqueue possible on next crash: %v", agentID, err)
}
```

#### Condition 3: Propagate error from `CompleteAgent` RPC (handlers.go, supervisor.go)

Change `completeAgent` and `CompleteAgent` signatures to return `error`:

```go
// supervisor.go
func (sv *Supervisor) CompleteAgent(ctx context.Context, agentID string) error {
    return sv.completeAgent(ctx, agentID)
}

func (sv *Supervisor) completeAgent(ctx context.Context, agentID string) error {
    mu := sv.getAgentMutex(agentID)
    if mu == nil {
        return ErrSupervisorStopped
    }
    mu.Lock()
    defer mu.Unlock()

    if _, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventOutputDelivered); err != nil {
        return err
    }
    // ... rest of function unchanged ...
    return nil
}
```

```go
// handlers.go
func (s *OrchestratorServer) CompleteAgent(ctx context.Context, req *pb.CompleteAgentRequest) (*pb.CompleteAgentResponse, error) {
    if req.AgentId == "" {
        return nil, status.Error(codes.InvalidArgument, "agent_id is required")
    }
    if err := s.supervisor.CompleteAgent(ctx, req.AgentId); err != nil {
        return nil, status.Errorf(codes.Internal, "complete agent %s: %v", req.AgentId, err)
    }
    return &pb.CompleteAgentResponse{}, nil
}
```

Update the `supervisor.Supervisor` interface used by the server (if applicable) and update `CompleteAgentForTest` in export_e2e.go accordingly.

---

### Suggested Fixes

#### Bug Fixes (blocking, always in-PR)

- **supervisor.go — tryAssignTask mutex extension**: See Condition 1 above. Inline per-agent mutex acquisition and hold through SetAgentFields write.
- **supervisor.go:378 — completeAgent silent error**: See Condition 2 above. Log SetAgentFields failure with context.
- **handlers.go:107 + supervisor.go:349 — CompleteAgent error propagation**: See Condition 3 above.

#### In-PR Improvements (non-blocking)

- **lifecycle_test.go:803** — Add a comment explaining why `time.Sleep(100ms)` is acceptable here: it is for test logistics (determining which agent received the task), not a correctness assertion. The correctness assertion is the `pollAgentState` call at line 822. Example:
  ```go
  // Give the assign loop time to route the task to one of the two agents.
  // This sleep is for test logistics only — the real assertion is pollAgentState below.
  time.Sleep(100 * time.Millisecond)
  ```

#### PR Description Amendments

None required beyond the fixes above.

#### New Issues (future features/enhancements only)

- **Priority loss on re-enqueue** (pre-existing, predates this PR): `EnqueueTask(ctx, taskID, 0)` hardcodes priority 0 on the no-idle-agent re-enqueue path at supervisor.go:266. A task submitted with priority 1.0 is silently demoted. Track for when a real priority queue backend is wired in.

- **completeAgent no production gRPC callsite**: `CompleteAgent` is only exercised via `CompleteAgentForTest`. The gRPC → REPORTING→IDLE path is acknowledged in lifecycle_test.go:5-8 as pending agent-side gRPC client implementation. Condition 3 above fixes the RPC error propagation; the agent-side caller is a future task.

---

### Summary Table

| Finding | Source | Severity | Triage |
|:--------|:-------|:---------|:-------|
| Race: tryAssignTask SetAgentFields outside per-agent mutex | CRITIC-1 | CRITICAL | Fix in-PR (Condition 1) |
| Silent error: completeAgent SetAgentFields | CRITIC-1 | BUG | Fix in-PR (Condition 2) |
| Silent error: tryAssignTask SetAgentFields | CRITIC-1 | BUG | Fix in-PR (Condition 1) |
| CompleteAgent RPC always returns success | CRITIC-2 / ADVOCATE-1 conceded | BUG | Fix in-PR (Condition 3) |
| Scenario 13 sleep comment | CRITIC-2 | MINOR | In-PR improvement |
| Scenarios 8-9 placement | CRITIC-2 | INFORMATIONAL | Acknowledged in comments |
| Scope creep process concern | CRITIC-2 | OUT OF SCOPE | Dropped |
| Council 6 snapshot race fix | ARBITER verified | CONDITION MET | No action needed |
