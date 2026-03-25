## Adversarial Council — Merge PR #22: CompareAndSetAgentState for Optimistic Locking (Post-Remediation)

> Convened: 2026-03-25T02:22Z | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion

Merge PR #22 — CompareAndSetAgentState for optimistic locking (post-remediation) — into epic/1-implement-go-orchestrator-core. This is a post-remediation review verifying fixes for 4 conditions identified by the prior council, plus CI (Condition 5).

### Prior Council Context

The prior council identified 4 pre-merge conditions, all remediated in commit f5d8e63, plus a CI condition addressed in commit 8f6afdd:

1. Lua script must return conflict state atomically (eliminate TOCTOU)
2. Add `conflict.Actual` assertion to `TestMachine_ApplyEvent_CASConflict`
3. `racyStore` injection must use `StateAssigned` (realistic race)
4. Supervisor must handle `*StateConflictError` via `errors.As`
5. CI with Redis integration tests

**This council confirms all 5 conditions are code-complete.** However, Condition 4's verification is incomplete — see below.

### Advocate Positions

**ADVOCATE-1**: All four prior council conditions are remediated with cited evidence. The CAS mechanism is correct across all layers: Lua script (`redis.go:122-132`) returns `{0, current}` atomically; Go caller (`redis.go:149-157`) populates `StateConflictError.Actual`; MockStore (`mock.go:67-82`) is lock-correct; conformance suite (`store_test.go:231-318`) covers four CAS scenarios including a 10-goroutine race; machine-level test (`machine_test.go:172-206`) validates full round-trip with `racyStore`; supervisor (`supervisor.go:193-199`) uses `errors.As` for CAS conflicts. Defense in depth: per-agent mutex reduces contention, CAS provides correctness. Revised to CONDITIONAL after conceding the supervisor test gap.

### Critic Positions

**CRITIC-1**: The supervisor's `errors.As` branch at `supervisor.go:193-198` — the Condition 4 remediation — has zero test coverage at the supervisor layer. `supervisor_test.go` contains no reference to `StateConflictError`. The serialization test (`supervisor_test.go:80-125`) produces `*InvalidTransitionError`, not `*StateConflictError`. Lower-layer tests (machine, store) do not exercise the supervisor's `tryAssignTask` control flow. Also identified priority-loss on re-enqueue at `supervisor.go:197` (hardcoded priority 0), later conceded as a tracked follow-on. Final position: ONE blocker — pre-merge supervisor CAS test.

**CRITIC-2**: Raised four issues: (1) priority loss on re-enqueue, (2) N+1 queries in `findIdleAgent`, (3) silent error discard in `checkHeartbeats`, (4) `ApplyEventForTest` export. Progressively conceded Issues 2-4 as non-blocking follow-ons. Aligned with CRITIC-1 on the supervisor CAS test gap as the primary blocker. Contributed the argument that "trivial and structurally identical" proves too much — if the branch has no distinct behavior it shouldn't exist; if it exists because CAS conflicts are semantically distinct, it warrants a test. Final position: ONE blocker — pre-merge supervisor CAS test.

### Questioner Findings

QUESTIONER was solicited but did not submit findings. The ARBITER independently verified all cited code references and performed git history analysis.

**ARBITER factual finding**: `supervisor.go` is `new file mode 100644` in the PR's diff against the merge base (`863f079`). The entire file was introduced on this branch (commit `8b0524f`, modified in `f5d8e63`). ADVOCATE-1's repeated claim that priority-0 re-enqueue, `findIdleAgent` N+1 queries, and `checkHeartbeats` error discard are "pre-existing patterns" is factually incorrect — all supervisor code is part of this PR's changeset. ADVOCATE-1 subsequently conceded this framing.

### Key Conflicts

- **Supervisor CAS test: pre-merge vs follow-on** — Resolved. ADVOCATE-1 initially argued the branch was "trivial" and lower-layer coverage was sufficient. CRITIC-1 countered that untested remediation undermines the council process. CRITIC-2 argued "trivial" proves too much. ADVOCATE-1 conceded and accepted pre-merge test as proportionate.
- **Priority loss: blocker vs follow-on** — Resolved. Both critics initially held this as blocking. CRITIC-1 conceded after acknowledging the `DequeueTask` interface change required. CRITIC-2 followed. All parties agree: tracked as epic-branch blocker.
- **"Pre-existing" scope framing** — Resolved. ARBITER's git analysis disproved the claim. ADVOCATE-1 conceded.
- **"Mutex makes CAS unreachable" defense** — Resolved. CRITIC-1 demonstrated this argument is self-defeating: if the mutex makes CAS conflicts unreachable, the CAS PR's own rationale is undermined. ADVOCATE-1 did not rebut.

### Concessions

- **ADVOCATE-1** conceded: supervisor CAS test gap exists; "trivial" argument proves too much; priority-loss framing (line 187 vs 197 asymmetry); accepted pre-merge test condition.
- **CRITIC-1** conceded: priority loss (follow-on, not blocker); heartbeat error logging (follow-on); Lua table decoding edge cases (withdrawn).
- **CRITIC-2** conceded: N+1 queries (follow-on); `ApplyEventForTest` (follow-on); heartbeat error discard (follow-on); priority loss (follow-on, aligning with CRITIC-1).

### Regression Lineage

This council is a direct follow-up to the prior council on PR #22 (recommendation file: `2026-03-25-council-pr-22-cas-optimistic-locking.md`). All 5 prior conditions are code-complete. This council adds one new condition (supervisor-level CAS test) that the prior council's Condition 4 implied but did not explicitly require.

### Arbiter Recommendation

**CONDITIONAL**

The CAS mechanism is correct and thoroughly tested across the Lua script, RedisStore, MockStore, conformance suite, and Machine layers. All four prior council conditions are code-complete. However, the Condition 4 remediation (`errors.As` branch at `supervisor.go:193-198`) has no test coverage at the supervisor layer — the only layer where the remediation lives. All three debaters converged on this as the single remaining gap. The test is small and well-defined: inject a `racyStore`-style wrapper at the supervisor level, trigger `tryAssignTask`, verify the task is re-enqueued. This should be completed before merge.

### Conditions (blocking this PR)

1. **Add a supervisor-level CAS integration test.** Write a test in `supervisor_test.go` that injects a store wrapper causing `CompareAndSetAgentState` to return `*StateConflictError` during `tryAssignTask`, then asserts the task is re-enqueued. This completes Condition 4's verification chain at the supervisor layer.

   CITE: `internal/supervisor/supervisor.go` L:193-198 — untested `errors.As` branch
   CITE: `internal/supervisor/supervisor_test.go` L:1-125 — zero references to `StateConflictError`

### Suggested Fixes

#### Bug Fixes

None identified. The CAS mechanism is correct across all layers.

#### In-PR Improvements

1. **Supervisor CAS test** (blocking condition above).

#### PR Description Amendments

None required.

#### Epic-Branch Follow-Ons (non-blocking for this PR)

These are defects or improvements in code introduced by this PR that should be tracked as issues on the epic branch, to be resolved before `epic/1-implement-go-orchestrator-core` merges to `main`:

1. **Priority-preserving re-enqueue.** `tryAssignTask` re-enqueues tasks at hardcoded priority 0 (`supervisor.go:187,197,203`), losing the original priority from `DequeueTask`. Requires `DequeueTask` to return `(string, float64, error)` or an equivalent mechanism. Under CAS contention, this causes priority inversion — low-priority tasks queue-jump on re-enqueue.

   CITE: `internal/supervisor/supervisor.go` L:187,197,203 — hardcoded priority 0
   CITE: `internal/state/store.go` L:69 — `DequeueTask` returns `(string, error)`, no priority

2. **`findIdleAgent` N+1 optimization.** Replace per-agent `GetAgentState` loop (`supervisor.go:222-229`) with `GetAllAgentStates` (`store.go:60-62`), which already exists and batches into a single Redis pipeline.

   CITE: `internal/supervisor/supervisor.go` L:222-229 — N+1 queries
   CITE: `internal/state/store.go` L:60-62 — batched alternative exists

3. **Heartbeat error observability.** Add structured logging for failed `applyEvent` calls in `checkHeartbeats` (`supervisor.go:158`), distinguishing CAS conflicts (benign) from hard failures (operational).

   CITE: `internal/supervisor/supervisor.go` L:158 — `_, _ =` discard

#### Critical Discoveries

None.

### Verification Results

| # | Finding | Citations | Verdict | Action |
|---|---------|-----------|---------|--------|
| 1 | Untested `errors.As` branch | `supervisor.go` L:193-198, `supervisor_test.go` L:1-125 | VERIFIED | Retained |
| 2 | Priority-loss re-enqueue | `supervisor.go` L:187,197,203 | VERIFIED | Retained |
| 3 | findIdleAgent N+1 | `supervisor.go` L:222-229, `store.go` L:60-62 | VERIFIED | Retained |
| 4 | Heartbeat error discard | `supervisor.go` L:158 | VERIFIED | Retained |

Verification: 4 verified, 0 phantom, 0 unverified. All findings verified against codebase.
