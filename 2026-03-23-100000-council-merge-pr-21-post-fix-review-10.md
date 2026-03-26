---
## Adversarial Council — Merge PR #21 (feat: E2E lifecycle tests) — Council 10 Post-Fix Review

> Convened: 2026-03-23T10:00:00Z | Advocates: 1 | Critics: 2 | Rounds: 2/4 | Motion type: CODE

### Motion
PR #21 (feat: E2E lifecycle tests) is ready to merge into epic/1-implement-go-orchestrator-core. This is Council 10 — a post-fix review after Council 9 addressed crashAgent pre-read, SetAgentFields re-enqueue, and assign backoff remediation (commit 49f1793). This council evaluates whether the PR is now ready to merge.

---

### Advocate Positions
**ADVOCATE-1**: Argued FOR on the basis that all Council 9 blocking conditions are implemented (crashAgent pre-read at `supervisor.go:201-205`, Scenario 16 at `lifecycle_test.go:973-1047`) and both non-blocking follow-ups were proactively addressed (SetAgentFields re-enqueue at `supervisor.go:322-329`, assign loop backoff at `supervisor.go:358-365`). Cited 18 E2E scenarios passing race-clean, per-agent mutex serialization, cooldown sentinel pre-population, and findIdleAgent double-check as structural strengths. Initially claimed Defect A/1 was "structurally unreachable" via three guards; conceded all three guards are bypassed in the critics' scenario after ARBITER CLARIFY. Pivoted to CONDITIONAL FOR with log + test as the single blocking condition, accepting CRITIC-1's consistency argument: the PR's remediation pattern (Councils 6–9) demands the same treatment for this gap.

---

### Critic Positions
**CRITIC-1**: Raised four defects. Defect 1 (stale `CurrentTaskID` in `completeAgent` after `GetAgentFields` failure at `supervisor.go:452`) — blocking, independently verified same path as CRITIC-2. Defect 2 (six "task lost" log paths untestable due to missing `SetEnqueueTaskError` mock method) — non-blocking. Defect 3 (`DequeueTask` errors at `supervisor.go:278-282` bypass backoff) — initially blocking, conceded to non-blocking after accepting ADVOCATE-1's argument that no data is at risk. Defect 4 (Scenarios 8/9 double-record crashes outside production path) — non-blocking. Final position: CONDITIONAL FOR with Defect 1 as sole blocking condition.

**CRITIC-2**: Raised four defects. Defect A (same as CRITIC-1's Defect 1) — blocking, with detailed 7-step attack path. Defect B (same as CRITIC-1's Defect 3) — partially conceded to non-blocking. Defect C (global mock error injection) — conceded to non-blocking. Defect D (no concurrency stress test) — conceded to non-blocking. Final position: CONDITIONAL FOR with log + test as the two conditions.

---

### Questioner Findings
The QUESTIONER was prompted twice but did not submit probes during this council. All claims were verified directly by the ARBITER via code inspection and by cross-examination between the debaters.

---

### Key Conflicts

**Defect A/1 reachability — "structurally unreachable" vs. "three guards bypassed" (ADVOCATE-1 vs. CRITIC-1 + CRITIC-2)**:
ADVOCATE-1 claimed three structural guards (IDLE skip at `supervisor.go:168`, TOCTOU at `supervisor.go:224`, tryAssignTask overwrite at `supervisor.go:319-321`) made the stale `CurrentTaskID` duplication path unreachable. ARBITER issued a CLARIFY noting Guard 1 (IDLE skip) is irrelevant — the agent is ASSIGNED, not IDLE, when the crash occurs. Both critics independently traced a 5-step attack path bypassing all three guards. ADVOCATE-1 conceded all three guards fail in the described scenario and withdrew the "structurally unreachable" claim. **Resolved: path is reachable. ADVOCATE-1 conceded.**

**Defect A/1 severity — blocking vs. non-blocking (CRITIC-1 + CRITIC-2 vs. ADVOCATE-1 initial)**:
ADVOCATE-1 initially argued the defect was non-blocking because: (a) at-least-once delivery is standard, (b) the trigger requires two transient `GetAgentFields` failures, (c) a log line suffices as a condition. CRITIC-1 rebutted: (a) the PR treats every other task-loss path as unacceptable, so normalizing this one is inconsistent; (b) the probability bar should be consistent — Council 9's blocking condition (`crashAgent` `GetAgentFields` failure) had equally low probability; (c) the fix is smaller than the debate about whether to apply it. ADVOCATE-1 conceded the consistency argument and pivoted to CONDITIONAL FOR. **Resolved: blocking condition. ADVOCATE-1 conceded.**

**Defect B/3 — DequeueTask bypassing backoff blocking vs. non-blocking (CRITIC-1 initial vs. ADVOCATE-1)**:
CRITIC-1 initially classified this as blocking because it "undermines the backoff mechanism." ADVOCATE-1 argued: (a) the code predates the backoff mechanism, (b) `DequeueTask` failure means no task was removed — zero data risk, (c) the Council 9 follow-up targeted the dequeue-reenqueue spin loop, not `DequeueTask` failures. CRITIC-1 conceded to non-blocking. **Resolved: non-blocking. CRITIC-1 conceded.**

---

### Concessions

- **ADVOCATE-1** conceded the "structurally unreachable" claim — all three guards are bypassed in the critics' scenario
- **ADVOCATE-1** conceded Guard 1 (IDLE skip) is irrelevant to the attack path
- **ADVOCATE-1** conceded the "at-least-once" framing contradicts the PR's own design philosophy
- **ADVOCATE-1** conceded Defect 1 should be a blocking condition (consistency with Councils 6–9)
- **CRITIC-1** conceded Defect 3 (DequeueTask backoff) to non-blocking (no data risk)
- **CRITIC-2** conceded Defect B (DequeueTask backoff) to non-blocking (no data risk)
- **CRITIC-2** conceded Defect C (global mock error injection) to non-blocking (test stratification valid)
- **CRITIC-2** conceded Defect D (no concurrency stress test) to non-blocking (background goroutines provide some coverage)

---

### Regression Lineage

Council 10's sole finding (stale `CurrentTaskID` in `completeAgent`) is an error-handling asymmetry introduced in the same commit series as Councils 7–9's fixes. The `completeAgent` method was modified in commit 683d2ff (Council 6) to add `CurrentTaskID` clearing via read-modify-write at `supervisor.go:452-458`. The `SetAgentFields` failure path was logged (line 456) but the `GetAgentFields` failure path was left silent. Councils 7–9 systematically addressed analogous gaps in `crashAgent` and `tryAssignTask` but did not revisit `completeAgent`.

Council 9's remediation (commit 49f1793) is confirmed correctly implemented:
- Condition 1 (crashAgent pre-read): verified at `supervisor.go:201-205`
- Condition 2 (Scenario 16): verified at `lifecycle_test.go:973-1047`
- Non-blocking follow-up 1 (SetAgentFields re-enqueue): verified at `supervisor.go:322-329`
- Non-blocking follow-up 2 (assign loop backoff): verified at `supervisor.go:358-372`

No regressions from prior council fixes were identified.

---

### Evidence Verified by ARBITER

| Finding | File:Line | Verified? | Assessment |
|:--------|:----------|:----------|:-----------|
| Silent `GetAgentFields` fall-through in `completeAgent` | `supervisor.go:452` | **YES** | GAP — no log, no else branch |
| `SetAgentFields` failure logs in `completeAgent` | `supervisor.go:455-456` | YES | Asymmetry — analogous path is logged |
| `checkHeartbeats` skips IDLE agents (Guard 1) | `supervisor.go:168-170` | YES | Does NOT protect — agent is ASSIGNED in attack path |
| TOCTOU guard checks `PreviousState==IDLE` (Guard 2) | `supervisor.go:224` | YES | Does NOT protect — `PreviousState` is ASSIGNED/WORKING |
| `tryAssignTask` overwrites `CurrentTaskID` (Guard 3) | `supervisor.go:319-321` | YES | Defeated when `GetAgentFields` fails at line 319 |
| `crashAgent` pre-reads `CurrentTaskID` | `supervisor.go:201` | YES | Reads stale value if `completeAgent` failed to clear |
| `crashAgent` re-enqueues using pre-read value | `supervisor.go:234-235` | YES | Would re-enqueue completed task T1 |
| `DequeueTask` errors bypass backoff | `supervisor.go:278-282` | YES | All errors treated as `ErrQueueEmpty` |
| No `SetDequeueTaskError` in MockStore | `mock.go` (absent) | YES | `DequeueTask` failure path untestable |
| No `SetEnqueueTaskError` in MockStore | `mock.go` (absent) | YES | "task lost" paths untestable |
| Council 9 Condition 1 (crashAgent pre-read) | `supervisor.go:201-205` | YES | Implemented correctly |
| Council 9 Condition 2 (Scenario 16) | `lifecycle_test.go:973-1047` | YES | Implemented correctly |
| Council 9 non-blocking 1 (SetAgentFields re-enqueue) | `supervisor.go:322-329` | YES | Implemented correctly |
| Council 9 non-blocking 2 (assign backoff) | `supervisor.go:358-372` | YES | Implemented correctly |
| All tests pass (7 packages, 0 failures) | `go test -tags testenv -race -count=1 ./...` | YES | Clean |
| 18 E2E scenarios pass race-clean | `e2e/lifecycle_test.go` | YES | Clean (3.052s) |

---

### Scope Audit

| Finding | Relevance | Pre-existence | Disposition |
|:--------|:----------|:-------------|:------------|
| `completeAgent` `GetAgentFields` silent failure | PASSES — `supervisor.go` in PR scope | PASSES — introduced in 683d2ff (Council 6) | IN-PR GAP (blocking condition) |
| `DequeueTask` errors bypass backoff | PASSES — `supervisor.go` in PR scope | PARTIAL — early return predates backoff, but gap exists only because backoff was added | Non-blocking follow-up |
| Six "task lost" paths untestable | PASSES — `supervisor.go` in PR scope | PASSES — present since original implementation | Non-blocking follow-up |
| Scenarios 8/9 description accuracy | PASSES — `lifecycle_test.go` in PR scope | PASSES — introduced in 26accdf | Non-blocking (documentation nit) |
| Global mock error injection | PASSES — `mock.go` in PR scope | PASSES — design decision in original mock | Non-blocking follow-up |
| Concurrency stress test | PASSES — `e2e/` in PR scope | N/A — aspirational coverage | Non-blocking follow-up |

---

### Arbiter Recommendation

**CONDITIONAL**

Council 10 confirms that all Council 9 blocking conditions and non-blocking follow-ups are correctly implemented (commit 49f1793). The E2E test suite (18 scenarios, in-process gRPC via bufconn, race-clean, build-tag-isolated) provides comprehensive lifecycle coverage. The 9-council remediation history has systematically closed every identified error-handling gap — crashAgent pre-read (C9), tryAssignTask GetAgentFields re-enqueue (C8), tryAssignTask SetAgentFields re-enqueue (C9), assign loop backoff (C9), per-agent mutex serialization (C7), cooldown sentinel pre-population (C6), findIdleAgent double-check (C6).

One remaining gap was unanimously identified: `completeAgent` silently discards `GetAgentFields` failures at `supervisor.go:452`, leaving a stale `CurrentTaskID` in the store. Under a double `GetAgentFields` failure (in `completeAgent` and the subsequent `tryAssignTask`), a completed task can be re-enqueued by `crashAgent`. All three debaters independently verified the attack path and agreed it bypasses the three guards ADVOCATE-1 initially cited. ADVOCATE-1 conceded both the reachability and the blocking severity, accepting CRITIC-1's consistency argument: Councils 6–9 all resolved analogous gaps as merge conditions with small, targeted fixes. This gap warrants the same treatment.

The fix is bounded (~3 lines for the log, ~40 lines for the test) and follows the exact pattern established by `SetAgentFields` failure handling at `supervisor.go:455-456` and the error-injection test structure of Scenarios 14–18.

---

### Conditions (if CONDITIONAL)

All conditions are blocking. PR must not merge until all are addressed.

#### Condition 1: Add log warning for `GetAgentFields` failure in `completeAgent`

Add an `else` branch at `supervisor.go:452` that logs the failure, matching the `SetAgentFields` failure log at `supervisor.go:455-456`:

```go
// Clear the assigned task so crashAgent doesn't re-enqueue a completed task.
if cur, fErr := sv.store.GetAgentFields(ctx, agentID); fErr == nil {
    cur.CurrentTaskID = ""
    cur.CurrentTaskPriority = 0
    if err := sv.store.SetAgentFields(ctx, agentID, cur); err != nil {
        log.Printf("supervisor: CurrentTaskID not cleared for agent %s — duplicate re-enqueue possible on next crash: %v", agentID, err)
    }
} else {
    log.Printf("supervisor: GetAgentFields failed for agent %s — CurrentTaskID not cleared, duplicate re-enqueue possible on next crash: %v", agentID, fErr)
}
```

This closes the observability gap. The structural fix (preventing duplication entirely via a dedicated `ClearCurrentTask` store method or blind write) is a non-blocking follow-up.

#### Condition 2: Add Scenario 19 — `GetAgentFields` failure in `completeAgent`

Add an E2E scenario to `lifecycle_test.go` that:
1. Registers an agent, submits a task, waits for assignment
2. Advances the agent to REPORTING
3. Injects `GetAgentFields` error via `store.SetGetAgentFieldsError`
4. Calls `CompleteAgentForTest` — verifies it returns nil (agent transitions to IDLE)
5. Clears the error
6. Verifies the agent is IDLE and can receive a subsequent task assignment

This mirrors Scenario 15 (`SetAgentFields` failure in `completeAgent`) and closes the last asymmetry in `completeAgent` error-path test coverage.

---

### Suggested Fixes

#### Bug Fixes (blocking, always in-PR)

None. The finding is an observability gap (missing log + missing test), not a behavioral bug. The stale `CurrentTaskID` duplication path requires a deeper structural fix (non-blocking follow-up).

#### In-PR Improvements (scoped, non-bug)

- **`supervisor.go:452` — add else branch with log warning**: See Condition 1 above.
  CITE: `internal/supervisor/supervisor.go` L:452

- **`lifecycle_test.go` — add Scenario 19 for `GetAgentFields` failure in `completeAgent`**: See Condition 2 above.
  CITE: `e2e/lifecycle_test.go` L:971 (insert after Scenario 15)

#### PR Description Amendments

None required.

#### Critical Discoveries (informational)

None identified. All findings are directly about the motion.

---

### Follow-Up Issues (non-blocking, track on epic branch)

- **Structural `CurrentTaskID` duplication prevention** (`supervisor.go:452`): The log line makes the failure observable but does not prevent the duplication. A dedicated `ClearCurrentTask` store method or a blind `SetAgentFields` write (without requiring a prior read) would eliminate the stale `CurrentTaskID` risk entirely. Requires store interface design work.
  CITE: `internal/supervisor/supervisor.go` L:452

- **`DequeueTask` errors bypassing backoff** (`supervisor.go:278-282`): All `DequeueTask` errors are treated identically to `ErrQueueEmpty`. Real store failures should trigger `recordAssignError()` to engage the backoff mechanism. No data risk (failed dequeue = nothing removed), but performance concern under sustained store degradation. Requires adding `SetDequeueTaskError` to MockStore.
  CITE: `internal/supervisor/supervisor.go` L:278

- **`SetEnqueueTaskError` mock injection** (`mock.go`): Six "task lost" log paths in supervisor.go are structurally untestable because MockStore lacks `SetEnqueueTaskError`. Add the injection method and test at least one representative path.
  CITE: `internal/state/mock.go` L:169

- **Per-agent error injection in MockStore**: Current error injection (`SetGetAgentFieldsError`, `SetSetAgentFieldsError`) is globally scoped. Per-agent error injection would enable multi-agent error isolation tests. Non-blocking — current tests correctly verify code-path logic.
  CITE: `internal/state/mock.go` L:73

- **Concurrency stress test for crash+complete race**: No test exercises `crashAgent` and `completeAgent` concurrently on the same agent. The per-agent mutex (`supervisor.go:190-195`, `supervisor.go:441-442`) is verified by code review and the `-race` detector, but a targeted stress test would increase confidence.
  CITE: `internal/supervisor/supervisor.go` L:190

- **Scenarios 8/9 comment accuracy**: Per-scenario descriptions say "E2E stack context" but use fully manual crash paths via `ApplyEventForTest` + `policy.RecordCrash`. The scenarios are correctly marked `[gRPC-bypassed]` in the package header but per-scenario comments could be more precise.
  CITE: `e2e/lifecycle_test.go` L:482

---

### Summary Table

| Finding | Source | Severity | Triage |
|:--------|:-------|:---------|:-------|
| `completeAgent` `GetAgentFields` silent failure — stale `CurrentTaskID` | CRITIC-1, CRITIC-2 | GAP (observability + coverage) | Fix in-PR (Conditions 1 + 2) |
| `DequeueTask` errors bypass backoff | CRITIC-1, CRITIC-2 | PERFORMANCE | Non-blocking follow-up |
| Six "task lost" paths untestable | CRITIC-1 | TEST GAP | Non-blocking follow-up |
| Scenarios 8/9 description accuracy | CRITIC-1 | DOCUMENTATION | Non-blocking follow-up |
| Global mock error injection | CRITIC-2 | TEST DESIGN | Non-blocking follow-up |
| Concurrency stress test | CRITIC-2 | TEST COVERAGE | Non-blocking follow-up |
| Council 9 Condition 1 (crashAgent pre-read) | ARBITER verified | CONDITION MET | No action needed |
| Council 9 Condition 2 (Scenario 16) | ARBITER verified | CONDITION MET | No action needed |
| Council 9 non-blocking 1 (SetAgentFields re-enqueue) | ARBITER verified | IMPLEMENTED | No action needed |
| Council 9 non-blocking 2 (assign backoff) | ARBITER verified | IMPLEMENTED | No action needed |
