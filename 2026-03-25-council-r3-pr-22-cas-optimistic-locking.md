## Adversarial Council — Merge PR #22: CompareAndSetAgentState for Optimistic Locking (Post-Remediation Round 3)

> Convened: 2026-03-25T03:17Z | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion

Merge PR #22 — CompareAndSetAgentState for optimistic locking (post-remediation round 3) — into epic/1-implement-go-orchestrator-core. This is a post-remediation review verifying the final condition from the second council (supervisor CAS test) plus the epic-branch follow-ons (priority-preserving re-enqueue, batched GetAllAgentStates, heartbeat observability) that were implemented in the latest commits.

### Advocate Positions

**ADVOCATE-1**: The blocking condition from the second council is satisfied: `TestSupervisor_TryAssignTask_CASConflict_ReenqueuesTask` at `supervisor_test.go:140-191` exercises the full `supervisor.Run` loop under injected CAS conflict, asserting both task identity and priority preservation. All four prior-council remediation conditions are code-complete. Priority-preserving re-enqueue is fixed across all `tryAssignTask` paths — `DequeueTask` now returns `(string, float64, error)` and the original priority flows through every re-enqueue call site. `GetAllAgentStates` is implemented and called by the health aggregator at `aggregator.go:82`. CI is in place. Conceded two real gaps: (1) backoff misclassification at `supervisor.go:316` is semantically wrong per `machine.go:35-38`, and (2) silent `ApplyEvent` failure in `crashAgent` at `supervisor.go:213-219` is an asymmetric observability gap. Argued neither is blocking in the current single-supervisor topology.

### Critic Positions

**CRITIC-1**: The backoff misclassification is not merely a semantic imprecision — it is a defect against the PR's own stated design intent. `machine.go:35-38` explicitly declares CAS as the correctness guard and the mutex as a performance optimization, anticipating multi-supervisor operation. `recordAssignError()` at `supervisor.go:362-369` applies supervisor-global exponential backoff (capping at 6.4 seconds) — not per-agent backoff. A CAS conflict on one agent freezes all task assignments across the entire supervisor instance. In the multi-supervisor topology the PR explicitly targets, this inverts throughput: healthy concurrency detection triggers maximal assignment suppression. Conceded Objections 2 (crashAgent silent logging — non-blocking per retry argument), 3 (agent state assertion — Machine-layer concern), and 4 (time-coupled test — theoretical).

**CRITIC-2**: Independently identified the same backoff misclassification and converged with CRITIC-1. Highlighted the self-contradiction: ADVOCATE-1 cannot simultaneously argue the CAS test proves robustness against a real scenario AND that the scenario is remote enough for the wrong behavior to be non-blocking. Conceded Point 2 (dead interface surface — `GetAllAgentStates` is called at `aggregator.go:82`), Point 3 (agent state assertion — Machine-layer coverage sufficient), and narrowed Point 4 from `checkHeartbeats` to `crashAgent` asymmetric logging, adopting CRITIC-1's position.

### Questioner Findings

QUESTIONER was solicited three times but did not submit findings. The ARBITER independently verified all cited code references.

**ARBITER factual findings:**
- `machine.go:35-38` does explicitly state: "The supervisor's per-agent mutex remains as a performance optimisation to reduce contention at Redis, but is no longer the sole correctness guard." This confirms the PR's design intent includes multi-supervisor operation.
- `consecutiveAssignErrors` and `nextAssignAttempt` at `supervisor.go:39-40` are fields on the `Supervisor` struct — confirmed supervisor-global, not per-agent.
- `GetAllAgentStates` is called at `internal/health/aggregator.go:82` — CRITIC-2's "dead interface" claim was factually incorrect.
- The supervisor CAS test at `supervisor_test.go:140-191` does exercise the full `supervisor.Run` loop and asserts both task ID and priority preservation.

### Key Conflicts

- **Backoff misclassification: blocking vs follow-on** — Unresolved. Both critics maintain it is blocking; ADVOCATE-1 acknowledges the defect but argues it only activates in a multi-supervisor topology that does not yet exist. CRITIC-1 counters that the code's own documentation and test suite establish multi-supervisor as the designed-for topology, not a hypothetical future. All three parties agree the backoff is semantically wrong.
- **"Structurally impossible" CAS conflicts** — Resolved. ADVOCATE-1 conceded this framing was wrong after CRITIC-1 cited `machine.go:35-38`.
- **Silent `ApplyEvent` in `crashAgent`** — Resolved as non-blocking. ADVOCATE-1 conceded the asymmetry. CRITIC-1 conceded non-blocking per retry argument (no data loss). Both critics accept as follow-on.

### Concessions

- **ADVOCATE-1** conceded: "structurally impossible" framing was wrong per `machine.go:35-38`; backoff misclassification is a real semantic defect; silent `ApplyEvent` in `crashAgent` is a real observability gap; `checkHeartbeats` citation error (line 204 is in `crashAgent`, not `checkHeartbeats`); test assertion `QueueLength >= 1` is weak.
- **CRITIC-1** conceded: Objection 2 (crashAgent silent logging — non-blocking per retry); Objection 3 (agent state assertion — Machine-layer concern); Objection 4 (time-coupled test — theoretical).
- **CRITIC-2** conceded: Point 2 (dead interface surface — factually wrong, `aggregator.go:82` calls it); Point 3 (agent state assertion — Machine-layer coverage sufficient); narrowed Point 4 to align with CRITIC-1's crashAgent position.

### Regression Lineage

This council is the third review of PR #22. The first council identified 4 conditions, all remediated in commit `f5d8e63`. The second council added one new condition (supervisor-level CAS test), remediated in commit `5b8e37d`. This third council confirms all prior conditions are satisfied. The new finding (backoff misclassification) was not identified by prior councils because the prior code had a separate `errors.As` branch for `*StateConflictError` — the remediation commits simplified the error handling by removing it, inadvertently introducing the uniform-backoff defect.

### Arbiter Recommendation

**CONDITIONAL**

All prior council conditions are satisfied and code-complete. The CAS mechanism is correct across all layers. However, a new defect emerged from the debate that all three parties agree is real: `recordAssignError()` is called indiscriminately on `*StateConflictError` at `supervisor.go:316`, applying supervisor-global exponential backoff to a healthy-store concurrency event. The code's own documentation at `machine.go:35-38` explicitly positions CAS as the correctness guard for multi-supervisor operation — making CAS conflicts a designed-for operating condition, not an edge case. The fix is small and well-scoped: add an `errors.As` check in `tryAssignTask` to skip `recordAssignError()` when the error is `*StateConflictError`. Landing a known defect against the PR's own stated design intent on the epic branch — which subsequent features will build upon — sets a poor foundation.

### Conditions (blocking this PR)

1. **Distinguish `*StateConflictError` from store errors in `tryAssignTask`.** Add an `errors.As` check at `supervisor.go:310-317` so that `*StateConflictError` triggers re-enqueue but skips `recordAssignError()`. CAS conflicts should not apply exponential backoff because they indicate healthy concurrency, not store degradation. Include a unit test verifying that a CAS conflict does not increment `consecutiveAssignErrors`.

   CITE: `internal/supervisor/supervisor.go` L:309-317 — uniform error handling applies backoff to CAS conflicts
   CITE: `internal/supervisor/supervisor.go` L:362-369 — `recordAssignError()` exponential backoff
   CITE: `internal/agent/machine.go` L:35-38 — CAS is the correctness guard, mutex is performance optimization

### Suggested Fixes

#### Bug Fixes

1. **Backoff misclassification** (blocking condition above). `tryAssignTask` calls `recordAssignError()` on all `ApplyEvent` errors including `*StateConflictError`, which is a healthy-concurrency signal, not a store failure. Under multi-supervisor operation (the topology `machine.go:35-38` explicitly designs for), this causes supervisor-global assignment starvation proportional to healthy concurrency.

   CITE: `internal/supervisor/supervisor.go` L:309-317
   CITE: `internal/supervisor/supervisor.go` L:362-369
   CITE: `internal/agent/machine.go` L:35-38

#### In-PR Improvements

None identified beyond the blocking condition.

#### PR Description Amendments

None required.

#### Epic-Branch Follow-Ons (non-blocking for this PR)

1. **Add log for `ApplyEvent` failure in `crashAgent`.** `supervisor.go:213-219` silently returns when `ApplyEvent` fails, asymmetric with the `GetAgentFields` log two lines above at line 204. Add a `log.Printf` consistent with the existing pattern. Delayed recovery (no data loss) makes this non-blocking.

   CITE: `internal/supervisor/supervisor.go` L:213-219 — silent return on ApplyEvent failure
   CITE: `internal/supervisor/supervisor.go` L:203-206 — logged GetAgentFields failure (asymmetric)

2. **Add logging for `checkHeartbeats` store errors.** `supervisor.go:155-157` and `163-165` silently swallow `ListAgents` and `GetAgentFields` errors. Add structured logging so operators can detect degraded heartbeat monitoring.

   CITE: `internal/supervisor/supervisor.go` L:155-157 — silent ListAgents failure
   CITE: `internal/supervisor/supervisor.go` L:163-165 — silent GetAgentFields failure

3. **Strengthen CAS integration test assertion.** `TestSupervisor_TryAssignTask_CASConflict_ReenqueuesTask` asserts `QueueLength >= 1` (`supervisor_test.go:175`), which passes regardless of how many task copies are re-enqueued. Consider tightening to `QueueLength == 1` or documenting why multiple copies are acceptable under the always-conflict wrapper.

   CITE: `internal/supervisor/supervisor_test.go` L:172-178 — weak QueueLength assertion

#### Critical Discoveries

None identified.

### Verification Results

| # | Finding | Citations | Verdict | Action |
|---|---------|-----------|---------|--------|
| 1 | Backoff misclassification — CAS conflicts trigger supervisor-global exponential backoff | `supervisor.go` L:309-317, L:362-369, `machine.go` L:35-38 | VERIFIED — all parties agree | Blocking condition |
| 2 | Silent `ApplyEvent` failure in `crashAgent` | `supervisor.go` L:213-219 vs L:203-206 | VERIFIED — asymmetry confirmed | Non-blocking follow-on |
| 3 | Silent `checkHeartbeats` store errors | `supervisor.go` L:155-157, L:163-165 | VERIFIED | Non-blocking follow-on |
| 4 | Weak test assertion (`QueueLength >= 1`) | `supervisor_test.go` L:172-178 | VERIFIED — ADVOCATE-1 conceded | Non-blocking follow-on |
| 5 | `GetAllAgentStates` dead interface | `aggregator.go` L:82 | REFUTED — has caller | Dropped |

Verification: 4 verified, 1 refuted, 0 phantom, 0 unverified.
