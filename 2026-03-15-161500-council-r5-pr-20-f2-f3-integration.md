## Adversarial Council — PR #20 F2-F3 Integration (Round 5)

> Convened: 2026-03-15T16:15:00Z | Advocates: 1 | Critics: 2 | Rounds: 1/4 | Motion type: CODE

### Motion
PR #20 — F2-F3 Integration: Wire DAG Executor into gRPC Handlers — should be merged into task/1-task-implement-go-orchestrator-core. Round 5: post-fix validation after all Round 4 conditions were applied.

### Round 4 Fix Validation

ARBITER independently verified all three fixes against the live codebase before opening debate. All tests pass (`go test ./...`, 0 failures).

| Condition | Required? | Fix Applied | File:Line | Verified |
|-----------|-----------|-------------|-----------|----------|
| Deterministic node ordering | Required | `sort.Slice(nodeStatuses, func(i, j int) bool { return nodeStatuses[i].NodeID < nodeStatuses[j].NodeID })` | `status.go:210-212` | Yes |
| Godoc immutability claim removed | Required | `// Graph is a validated DAG built from a pb.DAGSpec.` (word "immutable" removed) | `graph.go:9` | Yes |
| Orphaned PENDING marking on cancel | Recommended | Loop over `levels[levelIdx+1:]` calls `MarkNodeFailed(execID, nodeID, "skipped: upstream failure")` | `executor.go:103-110` | Yes — but see debate findings |

### Advocate Positions
**ADVOCATE-1**: Both required fixes are clean by unanimous agreement. Fix 3 is mechanically correct and precisely scoped — `levels[levelIdx+1:]` covers exactly future levels, current-level goroutines handle their own terminal states. On Problem A (test gap): conceded as "a valid observation" but argued it warrants a follow-up test rather than a merge block for a voluntary improvement. On Problem B (`rec.completedAt` overwrite): challenged CRITIC-2's gRPC impact claim as factually incorrect — `ToProtoResponse` at `status.go:181-185` constructs a `pb.GetDAGStatusResponse` containing only `DagExecutionId`, `State`, and `NodeStatuses`; `CompletedAt` is not serialized to the wire and has no current production consumer.

### Critic Positions
**CRITIC-1**: Concedes Fixes 1 and 2 are correctly applied. Objects to Fix 3 on grounds that: (a) `TestExecutor_FailFast` never reaches the orphaned-node loop (failing node B is at level 1, the last level, so `levels[1+1:] = []`); (b) `TestExecutor_ShutdownCancelsRunning` exercises the loop (A at level 0, B/C at level 1 marked FAILED) but only asserts execution-level state at `executor_test.go:281-283`, not per-node states. Proposed resolution: targeted test asserting per-node FAILED states after upstream failure, plus a guard in `MarkNodeFailed` to only update `rec.completedAt` on the first FAILED transition.

**CRITIC-2**: Concedes Fixes 1 and 2 are correctly applied. Raises the same Problem A as CRITIC-1 (test coverage gap at `executor.go:104-108` — two topologies cited: `FailFast` never enters the loop; `ShutdownCancelsRunning` enters it but asserts only `resp.State`). Raises Problem B: `MarkNodeFailed` at `status.go:154` unconditionally overwrites `rec.completedAt = now` on every call; after Fix 3, sequential orphaned-node markings push the execution's `completedAt` past the real failure time. Claims this is "observable via gRPC response." Grants ARBITER latitude to weigh conditions vs. notes.

### Questioner Findings
No QUESTIONER probes submitted in Round 5. ARBITER independently evaluated all contested claims against the codebase:

**Problem A (test coverage gap) — SUBSTANTIATED:**
- `TestExecutor_FailFast` (`executor_test.go:167-188`): topology `A → {B, C}`, B fails. B is at level 1 (`levelIdx=1`); `levels[2:]` is empty. The orphaned-marking loop body (`executor.go:105-107`) is never entered. Confirmed by code inspection.
- `TestExecutor_ShutdownCancelsRunning` (`executor_test.go:253-283`): topology same. A at level 0 fails due to shutdown; `levels[1:] = [[B, C]]` — loop IS entered, B and C are marked FAILED. But assertion at `executor_test.go:281-282` only checks `resp.State != FAILED`. Per-node states in `resp.NodeStatuses` are never inspected. The new observable behavior — downstream nodes appearing as FAILED with `error="skipped: upstream failure"` in the API response — is not asserted. ADVOCATE-1 conceded this.

**Problem B (`rec.completedAt` overwrite) — PARTIALLY SUBSTANTIATED:**
- `status.go:155`: `rec.completedAt = now` is unconditionally overwritten on every `MarkNodeFailed` call. The overwrite is real.
- `status.go:219`: `CompletedAt: rec.completedAt` flows into `ExecutionRecord`.
- **However**: CRITIC-2's claim that this "flows into `ToProtoResponse` and the gRPC `GetDAGStatusResponse`" is factually inaccurate. `ToProtoResponse` (`status.go:172-186`) constructs `pb.GetDAGStatusResponse` with only `DagExecutionId`, `State`, and `NodeStatuses` — `CompletedAt` is not included. ADVOCATE-1's challenge is correct.
- **Status**: The `rec.completedAt` regression is real and confirmed at `status.go:155`, but it is internal-only at current scope. No production consumer currently reads `Snapshot().CompletedAt` downstream of `ToProtoResponse`. The concern is latent, not active.

### Key Conflicts

- **Problem A: Fix 3 test coverage** — **Unresolved**. CRITIC-1 and CRITIC-2 identified the gap; ADVOCATE-1 conceded it is "a valid observation" but argued it doesn't block merge. The observable behavior change (per-node FAILED states in API response) has no regression guard.

- **Problem B: `rec.completedAt` overwrite** — **Partially resolved**. The overwrite is confirmed real (`status.go:155`). ADVOCATE-1 correctly refuted CRITIC-2's gRPC impact claim — `CompletedAt` is not in the proto response. CRITIC-2's "observable via gRPC" framing was inaccurate; the issue is internal-only at current scope. CRITIC-1's guard proposal is sound defensively but not essential at current scope.

### Concessions

**Round 5 concessions:**
- **ADVOCATE-1** conceded Problem A is "a valid observation" to **CRITIC-1** and **CRITIC-2** — the new code path has no per-node assertion
- **ADVOCATE-1** did not rebut Problem B's internal `rec.completedAt` overwrite (only disputed its gRPC exposure)

**Carried from Round 4** (full log in `2026-03-15-155200-council-r4-pr-20-f2-f3-integration.md`):
- ADVOCATE-1 conceded C1-2 (sort fix) and C1-1 (godoc fix) — both now applied
- All critics downgraded or withdrew other blockers

### Arbiter Recommendation
**CONDITIONAL FOR**

Both required conditions from Round 4 are verified correctly applied — all parties agree. Fix 3 (recommended improvement, now applied) introduces one substantiated new issue: the orphaned-node marking at `executor.go:103-110` changes observable API behavior (downstream node states in `GetDAGStatusResponse.NodeStatuses` now transition PENDING→FAILED with a descriptive error) but this state transition is never asserted by any test. `TestExecutor_FailFast` cannot reach the loop (`levels[2:]` is empty for the given topology); `TestExecutor_ShutdownCancelsRunning` exercises the loop but asserts only execution-level state, not per-node states. ADVOCATE-1 conceded the gap. This warrants a condition. Problem B (`rec.completedAt` timestamp overwrite) is real at `status.go:155` but has no current production consumer — `ToProtoResponse` does not serialize `CompletedAt` to the gRPC response; CRITIC-2's impact claim was overstated and ADVOCATE-1's refutation is correct. Problem B is noted as a recommended in-PR improvement.

**Relation to prior councils**: Round 5 confirms both Round 4 REQUIRED conditions (Fixes 1 and 2) as correctly and unanimously accepted. Round 5 adds one new condition against Fix 3 (test coverage), which was not imposed in Round 4 because Fix 3 did not exist as applied code at that time. Round 5 notes one minor internal-only regression (Problem B) as a recommended improvement, partially rebutting CRITIC-2's impact framing. All ten Round 3-4 future issue deferrals remain in scope for subsequent work.

### Conditions (before merge)

1. **Fix 3 test coverage**: Either:
   - **(a)** Add a targeted test that asserts per-node FAILED state with `error="skipped: upstream failure"` for all unstarted downstream nodes after an upstream node failure — topology `A → B → C` where A fails is the minimal case that exercises `levels[levelIdx+1:]` with non-empty future levels; OR
   - **(b)** Revert Fix 3 (`executor.go:103-110` and the `levelIdx` variable introduction at `executor.go:90`) and defer the behavior to Issue #9 (Orphaned PENDING observability), since Fix 3 was a recommended improvement, not a required condition.

   Either resolution satisfies this condition. The implementer may choose.

### Suggested Fixes

#### Bug Fixes (always in-PR)
Both Round 4 bug fixes are confirmed applied. No new bugs.
1. **Non-deterministic node ordering** — `sort.Slice` at `internal/dag/status.go:210-212`. ✓ Applied.
2. **Immutability claim mismatch** — Godoc corrected at `internal/dag/graph.go:9`. ✓ Applied.

#### In-PR Improvements
1. **`rec.completedAt` timestamp precision** — In `MarkNodeFailed` at `internal/dag/status.go:154`, guard the `rec.completedAt = now` assignment so it only updates on the first FAILED transition (e.g., `if rec.completedAt.IsZero()`). This ensures the execution's recorded completion time reflects the actual failure, not the timestamp of the last orphaned-node marking. The issue is internal-only at current scope (CompletedAt is not in the gRPC response) but will matter when `CompletedAt` is surfaced. (CRITIC-1 proposal; Problem B acknowledged by all parties.)

#### PR Description Amendments
- Carry forward from Round 3: Note in the PR description that the StoreSubmitter MVP intentionally drops `prompt` and `modelTier` fields.

#### New Issues (future features only — confirm with human)
No new issues. All prior future issues remain open (full list in Round 4 recommendation). Fix 3 condition resolution affects Issue #9:
> If condition option (b) is chosen (revert Fix 3): Issue #9 (Orphaned PENDING observability) remains fully open, no partial resolution.
> If condition option (a) is chosen (add test): Issue #9 is partially addressed. The remaining gap — shutdown scenario where "skipped: upstream failure" is slightly imprecise (see ARBITER pre-debate note) — may be tracked as a sub-item of Issue #4 (Executor shutdown guard) or Issue #9.
