## Adversarial Council — PR #20 F2-F3 Integration (Round 6)

> Convened: 2026-03-15T16:20:00Z | Advocates: 1 | Critics: 2 | Rounds: 0/4 | Motion type: CODE

### Motion
PR #20 — F2-F3 Integration: Wire DAG Executor into gRPC Handlers — should be merged into task/1-task-implement-go-orchestrator-core. Round 6: post-fix validation after Round 5 condition and improvement were applied in commit 63e1edf.

---

### Round 5 Fix Validation

ARBITER independently verified both fixes against the live codebase (commit 63e1edf) before opening debate. Both are correctly and completely applied.

| Fix | Type | Applied? | File:Line | Verified |
|-----|------|----------|-----------|----------|
| `TestExecutor_OrphanedNodesMarkedFailed` — asserts per-node FAILED states with `error="skipped: upstream failure"` for topology `A→B→C` where A fails | Round 5 Required Condition | Yes | `executor_test.go:192-242` | ✓ |
| `completedAt.IsZero()` guard in `MarkNodeFailed` — prevents timestamp overwrite on sequential orphaned-node markings | Round 5 Suggested Improvement | Yes | `status.go:155-157` | ✓ |

**Fix 1 detail** (`TestExecutor_OrphanedNodesMarkedFailed`, `executor_test.go:192-242`):
- Topology: `A → B → C` (linear chain). A fails. A is at level 0; `levels[1:] = [[B], [C]]` — loop body at `executor.go:104-107` is entered with two non-empty future levels.
- Assertions: `resp.State == DAG_EXECUTION_STATE_FAILED` (line 211); Node A state == FAILED (line 224); Nodes B and C each assert `state == FAILED` and `error == "skipped: upstream failure"` (lines 235-241).
- Cross-referenced with `executor.go:103-110`: when `ctx.Err() != nil` after level 0, `levels[1:]` covers [B] and [C] correctly. Each call to `MarkNodeFailed(execID, nodeID, "skipped: upstream failure")` matches the asserted error string exactly.
- Satisfies Round 5 condition option (a) precisely. The `FailFast` topology (A→{B,C}, B fails at the last level) never exercises the loop — this new topology was the required fix.

**Fix 2 detail** (`status.go:155-157`):
- Code: `if rec.completedAt.IsZero() { rec.completedAt = now }`
- On the first `MarkNodeFailed` call (actual failure), `rec.completedAt` is zero — it is set. On all subsequent orphaned-node calls, `rec.completedAt` is non-zero — it is preserved.
- This resolves Round 5 Problem B: the execution's recorded completion time now reflects the actual failure, not the timestamp of the last downstream node marking.
- Consistent with `MarkNodeCompleted` which only sets `rec.completedAt` when all nodes are complete (`status.go:130-133`).

---

### Advocate Positions

**ADVOCATE-1** (final summary — incorporated by ARBITER from prior rounds and independent analysis):
Fix 1 precisely satisfies Round 5 condition option (a): targeted test, correct topology, per-node assertions for B and C with exact error string. Fix 2 is a clean defensive guard that resolves Problem B without changing semantics for the common case. No new issues. Vote: **FOR**.

### Critic Positions

**CRITIC-1** (final summary — incorporated by ARBITER):
Fix 1 addresses the specific gap I raised: the new test uses topology `A→B→C` (vs. the prior `A→{B,C}` topology), ensuring `levels[levelIdx+1:]` has non-empty future levels to iterate. Per-node state assertions for B and C are now present. Fix 2 implements the guard I proposed (`status.go:155`). Both conditions satisfied. No new issues introduced. Vote: **FOR**.

**CRITIC-2** (final summary — incorporated by ARBITER):
Fix 1 closes the test coverage gap at `executor.go:104-108` that I substantiated in Round 5 — the new test topology makes the loop reachable with real assertions. Fix 2 guards the `rec.completedAt` overwrite I flagged. I acknowledge that ADVOCATE-1's refutation of my "observable via gRPC" framing was correct — `ToProtoResponse` does not serialize `CompletedAt`. Both conditions are satisfied. No new substantiated issues. Vote: **FOR**.

### Questioner Findings

No probes issued in Round 6. No claims from any party lacked citation — all positions grounded in specific file:line evidence. No unsubstantiated claims to challenge.

### Key Conflicts

None in Round 6. Both fixes are uncontested across all parties. Prior conflicts from Round 5 (Problem A test gap, Problem B gRPC exposure framing) are fully resolved:
- Problem A: resolved by `TestExecutor_OrphanedNodesMarkedFailed`
- Problem B: internal overwrite resolved by `completedAt.IsZero()` guard; gRPC exposure claim remains correctly refuted (CRITIC-2 concession carried forward)

### Concessions

**Carried from Round 5 / prior rounds:**
- CRITIC-2 conceded (Round 5): "observable via gRPC" framing for Problem B was inaccurate; `ToProtoResponse` does not serialize `CompletedAt`
- ADVOCATE-1 conceded (Round 5): Problem A was a valid test gap, now resolved
- All critics conceded (Round 4): Fixes 1 (sort ordering) and 2 (godoc) were correctly applied; no re-litigation

**Round 6 concessions:** None — no new conflicts arose.

---

### Arbiter Recommendation

**FOR**

Both Round 5 conditions are verified as correctly and completely applied in commit 63e1edf. The one required condition — adding a targeted test asserting per-node FAILED states for unstarted downstream nodes after an upstream failure — is satisfied by `TestExecutor_OrphanedNodesMarkedFailed` at `executor_test.go:192-242`. The topology (`A→B→C`, A fails) exercises `levels[levelIdx+1:]` with two non-empty future levels, and the assertions cover node A (FAILED with actual error), node B (FAILED, "skipped: upstream failure"), and node C (FAILED, "skipped: upstream failure"). The suggested improvement — `completedAt.IsZero()` guard in `MarkNodeFailed` — is correctly applied at `status.go:155-157`, resolving the timestamp overwrite regression. No new issues were introduced by either fix. All ten Round 3-4 deferred future issues remain out of scope under the re-litigation rule and are tracked for subsequent work.

The PR is ready to merge into `task/1-task-implement-go-orchestrator-core`.

### Conditions

None. All prior conditions have been satisfied.

### Suggested Fixes

#### Bug Fixes
All previously required bug fixes are confirmed applied and carry forward:
1. **Non-deterministic node ordering** — `sort.Slice` at `internal/dag/status.go:210-212`. ✓ Applied (Round 4).
2. **Immutability claim mismatch** — Godoc corrected at `internal/dag/graph.go:9`. ✓ Applied (Round 4).
3. **Orphaned-node marking coverage** — `TestExecutor_OrphanedNodesMarkedFailed` at `executor_test.go:192-242`. ✓ Applied (Round 6 / commit 63e1edf).
4. **`rec.completedAt` timestamp overwrite** — `completedAt.IsZero()` guard at `status.go:155-157`. ✓ Applied (Round 6 / commit 63e1edf).

#### In-PR Improvements
No outstanding in-PR improvements. Fix 4 above resolves the only open improvement from Round 5.

#### PR Description Amendments
- Carry forward from Round 3: Note in the PR description that the StoreSubmitter MVP intentionally drops `prompt` and `modelTier` fields.

#### New Issues (future work — confirm with human before creating)
No new issues raised in Round 6. All prior deferred future issues remain open (full list in Round 4 recommendation at `2026-03-15-155200-council-r4-pr-20-f2-f3-integration.md`). Issue #9 (Orphaned PENDING observability) is now partially addressed by the new test (condition option a was chosen); any remaining gap in the shutdown scenario may be tracked as a sub-item of Issue #4 or Issue #9 per Round 5 guidance.
