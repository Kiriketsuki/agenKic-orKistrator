## Adversarial Council — PR #20 F2-F3 Integration (Round 7)

> Convened: 2026-03-15T16:35:00Z | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion
PR #20 — F2-F3 Integration: Wire DAG Executor into gRPC Handlers — should be merged into task/1-task-implement-go-orchestrator-core. Round 7: post-commit 6fa22a0 validation (stabilized orphaned-node test by adding Shutdown() before Status() to eliminate goroutine race, plus GitNexus stats update in CLAUDE.md).

---

### Advocate Positions

**ADVOCATE-1**: The race fix in `executor_test.go` is mechanically correct and the only claim requiring extraordinary proof. `MarkNodeFailed()` at `status.go:154` sets DAG state to FAILED when node A fails — before the orphaned-node loop at `executor.go:103-110` marks B and C. `waitForCompletion` returns on FAILED state, creating a window. The fix (`waitForCompletion` → `e.Shutdown()` → `e.Status()`) is correct: `Shutdown()` at `executor.go:37-40` calls `e.wg.Wait()`, which blocks until the `run()` goroutine (and its orphaned-node loop) has fully returned. `Status()` then reads a settled tracker. The `server_test.go` goroutine leak is pre-existing and benign in practice (StoreSubmitter completes in microseconds). The gRPC coverage gap was deferred in prior rounds. `handler.GetDAGStatus` is a three-line pass-through with no logic to corrupt state.

---

### Critic Positions

**CRITIC-1**:
1. `internal/ipc/dagengine.go:10` — `// F3's Executor will satisfy this interface at integration time.` is now factually false. Integration has occurred in this PR. Stale forward-looking comments are documentation debt.
2. `server_test.go:52-55` — cleanup calls `conn.Close()` and `grpcServer.GracefulStop()` but never `executor.Shutdown()`. The executor at line 30 uses `context.Background()` — no cancellation ever arrives. The commit adds goroutine discipline in `executor_test.go:215` explicitly because "goroutines outliving their test cause race conditions" — the same reasoning applies to `server_test.go`. `main.go:49-50` already enforces `GracefulStop()` → `executor.Shutdown()` in production; the test does not mirror it.
3. No gRPC-layer integration test verifies orphaned-node FAILED states (`"skipped: upstream failure"`) via `GetDAGStatus` after a failing DAG submission. (Joined CRITIC-2's position.) **Conceded**: Shutdown() semantic conflation is not a hard blocker for the single-execution test.

**CRITIC-2**:
1. Same goroutine leak — frames it as a consistency violation, not merely hygiene: the commit cannot apply goroutine discipline in one test and waive it for the directly adjacent integration test. Under `-race` with a 1ms submitter latency, `go test -count=100` would expose this.
2. `Shutdown()` calls `e.cancel()` (executor.go:38), which cancels the executor-lifetime context shared by ALL executions. Using `Shutdown()` as a per-execution barrier is only safe in single-execution isolation, which the test does not document or enforce.
3. No `server_test.go` test submits a failing DAG via gRPC and calls `GetDAGStatus` to verify orphaned nodes carry `"skipped: upstream failure"`. The specific regression path this commit closes has zero gRPC-layer coverage. The `convert_test.go` coverage cited by ADVOCATE covers proto serialization, not live-executor-through-live-handler data flow.

---

### Questioner Findings

QUESTIONER did not submit findings (pending state). ARBITER applied probe standard independently, including verification of late-debate factual claims:

| Claim | Agent | Result |
|---|---|---|
| Race fix mechanics correct | ADVOCATE-1 | Substantiated: `executor.go:37-40`, `executor.go:64-68`, empirical `-race` results |
| Stale comment at `dagengine.go:10` | CRITIC-1 | Substantiated: `internal/ipc/dagengine.go:10` reads `// F3's Executor will satisfy this interface at integration time.` — ADVOCATE-1 explicitly conceded this in final summary |
| Missing `executor.Shutdown()` in test cleanup | CRITIC-1/2 | Substantiated: `server_test.go:52-55` confirmed — only `conn.Close()` + `grpcServer.GracefulStop()` |
| Production shutdown pattern shows correct ordering | CRITIC-1 | Substantiated: `main.go:49-50` shows `server.GracefulStop()` → `executor.Shutdown()` |
| Handler is thin pass-through | ADVOCATE-1 | Substantiated: `handlers.go:102-117` — validate → `s.dag.Status()` → return |
| gRPC gap "explicitly deferred in prior rounds" | ADVOCATE-1 | **Unsubstantiated**: Round 6 recommendation contains no such deferral — dag-level test (`executor_test.go:192-252`) was accepted as sufficient for Round 6's orphaned-node condition, but no explicit deferral of a gRPC-level test was recorded |
| "`convert_test.go` covers `ToProtoResponse`" | ADVOCATE-1 | **False**: `convert_test.go` tests only `AgentStateToProto`/`AgentStateFromProto` (lines 11-63) — no reference to `ToProtoResponse`. Correctly falsified by CRITIC-2. `ToProtoResponse` IS unit-tested at `status_test.go:170-194` (dag package, happy path only), but `convert_test.go` is not the file. |
| `ToProtoResponse` contains FAILED-path-specific logic | CRITIC-2 (implied) | Partially unsubstantiated: `status.go:174-188` is a pure field mapper with no conditional logic — FAILED states flow through identically to COMPLETED. The gap is real at the gRPC integration boundary but not a correctness risk in the current code. |
| Shutdown() conflation unsafe in multi-execution scenario | CRITIC-2 | Substantiated conceptually (`executor.go:38` calls `e.cancel()`), but harmless in this test (single-execution per test, terminal state pre-confirmed by `waitForCompletion`) — CRITIC-2 conceded this in debate |

---

### Key Conflicts

- **Goroutine leak scope (pre-existing vs. consistency violation)** — partially resolved. ADVOCATE correctly states the leak was not introduced by 6fa22a0. CRITIC argument holds: the commit's own stated reasoning for adding `Shutdown()` in `executor_test.go:212-214` applies equally to `setupTestServer` — the same executor type, same goroutine lifecycle, same missing join point. The consistency argument is stronger than the scoping argument because (a) the fix is 1 line, (b) it mirrors the already-correct production pattern, and (c) the Honest Findings Protocol requires fixing bugs unconditionally regardless of when introduced.
- **Stale comment** — unresolved by ADVOCATE (no rebuttal offered). Claim stands.
- **gRPC-layer orphaned-node coverage** — unresolved: both critics substantiated the gap; ADVOCATE's "thin pass-through" rebuttal is technically accurate but does not refute the existence of the coverage hole at the integration boundary. ADVOCATE's "deferred in prior rounds" claim is unsubstantiated.

---

### Concessions

- CRITIC-1 conceded to ADVOCATE-1: the Shutdown() semantic conflation (per-execution vs. executor-lifetime barrier) is not a hard blocker for the current single-execution test.
- ADVOCATE-1 acknowledged the Shutdown() conflation concern is "valid as a design note for future multi-execution tests."
- ADVOCATE-1 explicitly conceded the stale comment at `dagengine.go:10` in their final summary: "I concede this is a real doc-debt issue."
- CRITIC-2 conceded the Shutdown() semantic conflation objection: "I accept that... calling `Shutdown()` is safe and the race is correctly eliminated."

---

### Arbiter Recommendation

**CONDITIONAL**

The race fix in commit 6fa22a0 is mechanically correct and confirmed by the race detector (`go test -race -count=3` clean). However, two genuine in-PR defects were raised and substantiated by both critics: (1) `setupTestServer` in `server_test.go:52-55` omits `executor.Shutdown()` from its cleanup, creating a goroutine leak that is inconsistent with both the production shutdown ordering at `main.go:49-50` and the goroutine discipline this commit explicitly introduces in `executor_test.go:212-215`; (2) `internal/ipc/dagengine.go:10` carries a forward-looking comment that is now false. Both fixes are trivial. The gRPC-layer orphaned-node coverage gap is real but the handler is a confirmed thin pass-through, and the gap is appropriate for a follow-up issue rather than a merge blocker.

---

### Conditions

1. **`server_test.go:52-55`** — add `executor.Shutdown()` to the cleanup closure to match `main.go:49-50` production ordering and be consistent with the goroutine discipline this commit introduces.
2. **`internal/ipc/dagengine.go:10`** — update stale comment: `// F3's Executor will satisfy this interface at integration time.` is now false; the interface is satisfied. Update to reflect the current state.

---

### Suggested Fixes

#### Bug Fixes

**`internal/ipc/server_test.go:52-55`** — add `executor.Shutdown()` to test cleanup:
```go
cleanup := func() {
    conn.Close()
    grpcServer.GracefulStop()
    executor.Shutdown()  // join background run() goroutines; mirrors main.go:49-50
}
```
Rationale: executor goroutines launched by `Execute()` (`executor.go:64`) are not joined without this call. With `context.Background()` (line 30), no cancellation arrives from outside. The production binary already enforces this ordering at `main.go:49-50`.

#### In-PR Improvements

**`internal/ipc/dagengine.go:10`** — update stale forward-looking comment:
```go
// DAGEngine is the interface the ipc service delegates DAG operations to.
// Implemented by dag.Executor (wired in F2-F3 integration).
```
(Companion stale comment at `internal/dag/submitter.go:6` — `// F2's supervisor will satisfy this interface at integration time.` — was not raised in debate and cannot be made a condition, but is noted for companion cleanup.)

#### PR Description Amendments

None new. (Round 6 amendment — note that StoreSubmitter MVP intentionally drops prompt and modelTier fields — remains valid and unaffected by this commit.)

#### New Issues (confirm with human before creating)

**gRPC-layer orphaned-node failure integration test** — `TestSubmitDAG_FullLifecycle` (server_test.go:377-414) covers only the happy path. A test that submits a failing DAG via gRPC and calls `GetDAGStatus` to verify downstream nodes carry `DAG_EXECUTION_STATE_FAILED` with `error = "skipped: upstream failure"` would close the integration boundary coverage gap. The handler (`handlers.go:102-117`) is a confirmed thin pass-through and `ToProtoResponse` (`status.go:174-188`) is a pure field mapper with no conditional logic — this is a regression safety net, not a correctness gap. Note: ADVOCATE-1's claim that `convert_test.go` covers `ToProtoResponse` was provably false (`convert_test.go` tests only agent-state conversion; `ToProtoResponse` has a happy-path unit test at `status_test.go:170-194` but no FAILED-path test). Confirm with human before creating.

---

### Cross-Round Continuity

Round 7 partially confirms Round 6's FOR finding (race fix is correct) but introduces two new conditions not previously evaluated. The ADVOCATE's "deferred in prior rounds" claim for the gRPC coverage gap is unsubstantiated — Round 6 accepted the dag-level test as sufficient but recorded no explicit deferral of gRPC-level coverage. Round 7 escalates to CONDITIONAL rather than overturning FOR — the core implementation remains sound; only test/comment hygiene requires resolution.
