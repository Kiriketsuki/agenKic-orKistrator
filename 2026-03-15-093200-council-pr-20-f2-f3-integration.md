---
## Adversarial Council — PR #20 F2-F3 Integration

> Convened: 2026-03-15T09:32:00Z | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion
PR #20 — F2-F3 Integration: Wire DAG Executor into gRPC Handlers — should be merged into task/1-task-implement-go-orchestrator-core.

This PR wires F3's `dag.Executor` into F2's `OrchestratorServer`, implementing `SubmitDAG` and `GetDAGStatus` gRPC handlers, adds a `StoreSubmitter` adapter, introduces a `DAGEngine` interface seam, and includes 11 new tests. The diff touches 7 files with ~230 net lines added.

### Advocate Positions
**ADVOCATE-1**:
- The integration is surgical and minimal — a 2-method `DAGEngine` interface (`internal/ipc/dagengine.go:11-14`) cleanly decouples F2 from F3 using standard Go composition
- gRPC error mapping is correct: 4 validation sentinels → `InvalidArgument`, `ErrExecutionNotFound` → `NotFound`, unknowns → `Internal`
- The `context.Background()` detach at `executor.go:50` is necessary to prevent RPC lifecycle from cancelling async DAG execution
- `StoreSubmitter` is a documented MVP adapter with a test (`storesubmitter_test.go:56-71`) explicitly documenting the prompt/modelTier gap
- 10 of 11 new tests are substantive, including `TestSubmitDAG_FullLifecycle` which exercises the full gRPC→executor→store round-trip
- Process does not hang on shutdown: `GracefulStop()` waits for RPCs, then `main()` returns, killing all goroutines
- Target branch is `task/1-task-implement-go-orchestrator-core`, not `main` — MVP correctness bar, not production hardening

### Critic Positions
**CRITIC-1** (6 objections, no rebuttal response):
1. Fire-and-forget goroutine with no cancellation or shutdown coordination (`executor.go:50`)
2. In-memory `StatusTracker` is a silent data loss vector with no eviction (`status.go:85`)
3. `StoreSubmitter` silently discards prompt and modelTier — semantic data loss at a system boundary (`storesubmitter.go:25-27`)
4. Brittle error-type coupling — hardcoded sentinel checklist violates open/closed (`handlers.go:88-93`)
5. No nil-guard on `dag` field — deferred panic vector (`server.go:23-29`)
6. `TestExecutor_ContextCancellation` is a dead test — accepts both terminal states as tautology (`executor_test.go:279-280`)

**CRITIC-2** (7 initial objections, narrowed to 2 after debate):
1. Goroutine leak — `context.Background()` creates immortal goroutines with no shutdown path (narrowed from CRITICAL to "small fix needed")
2. Unbounded `StatusTracker` memory growth (downgraded to follow-up — F3 pre-existing code)
3. No concurrency limit on DAG executions (downgraded to follow-up — F3 design, not this PR)
4. Nil `DAGEngine` defers panic to runtime (withdrawn — no nil callers exist)
5. Nil `TaskSpec` dereference risk in `executeNode` (withdrawn — unreachable through current paths)
6. Error messages leak internals via catch-all `%v` (downgraded to LOW for MVP)
7. `StoreSubmitter` silently drops critical data (conceded — MVP trade-off with test documentation)

### Questioner Findings
QUESTIONER did not submit probes during the debate. No claims were marked unsubstantiated by QUESTIONER. The ARBITER independently verified key factual claims (dead test, spec references, git history for `status.go`).

### Key Conflicts
- **context.Background() severity** — ADVOCATE-1 said LOW (process exits, MVP with in-memory state); CRITIC-2 said merge-blocker (~10-line fix, prevents debt compounding) — **resolved: both agreed on CONDITIONAL fix**
- **Dead test responsibility** — ADVOCATE-1 initially said "inherited from F3, not this PR's fault"; CRITIC-2 and ARBITER said behavior change carries test update obligation — **resolved: ADVOCATE-1 conceded fully**
- **Pre-existing F3 issues in integration review** — ADVOCATE-1 said out of scope; CRITIC-2 said integration is the "blast-radius expansion event" — **resolved: CRITIC-2 downgraded StatusTracker memory, concurrency limits, and nil TaskSpec to follow-ups; acknowledged F3 scope boundary**
- **StoreSubmitter data loss framing** — CRITIC-1/CRITIC-2 said "silent data loss"; ADVOCATE-1 said "documented MVP gap" — **resolved: CRITIC-2 conceded after examining alternatives (logging=noise, narrower interface=breaks tests, error=breaks DAGs)**
- **Error coupling** — CRITIC-1 said open/closed violation; ADVOCATE-1 said closed set — **resolved: CRITIC-2 conceded, pragmatic for 4 graph-theory sentinels**

### Concessions
- **ADVOCATE-1** conceded to CRITIC-1/CRITIC-2: `TestExecutor_ContextCancellation` (`executor_test.go:253-283`) is stale and is this PR's responsibility to fix
- **ADVOCATE-1** conceded to CRITIC-2: server-lifetime context is architecturally superior to bare `context.Background()`
- **ADVOCATE-1** conceded to ARBITER/CRITIC-2: behavior changes carry obligation to update corresponding tests regardless of which commit introduced the test file
- **CRITIC-2** conceded to ADVOCATE-1: error coupling (CRITIC-1 #4) — pragmatic for a closed set of graph-theory errors
- **CRITIC-2** conceded to ADVOCATE-1: nil `TaskSpec` risk (issue 5) — withdrawn, unreachable through current call paths
- **CRITIC-2** conceded to ADVOCATE-1: `StoreSubmitter` data loss (issue 7) — MVP trade-off with adequate test documentation
- **CRITIC-2** conceded to ADVOCATE-1: "hangs forever" shutdown framing — retracted; actual risk is unclean mid-operation termination
- **CRITIC-1** did not respond to rebuttals; no concessions recorded

### Arbiter Recommendation
**CONDITIONAL**

The integration wiring is mechanically correct: the `DAGEngine` interface seam is clean, gRPC error mapping follows best practice, the constructor signature change is updated at all call sites, and 10 of 11 new tests are substantive. The PR does exactly what its scope says — wire F3 into F2. Both sides converged to agreement that the PR should merge with two small, concrete fixes applied before the branch reaches `main`. The fixes are directly attributable to this PR's changes (not pre-existing F3 code) and are small enough to apply as a follow-up commit on the same branch.

### Conditions (if CONDITIONAL)
1. **Fix `TestExecutor_ContextCancellation`** — Update or replace the test at `executor_test.go:253-283` to reflect the intentional `context.Background()` behavior. The current test accepts both `FAILED` and `COMPLETED` (a tautology) and its name promises cancellation semantics that no longer exist. Both sides agreed this must be fixed. (~3 lines)
2. **Add server-lifetime context to `Executor`** — Replace `context.Background()` at `executor.go:50` with a server-lifetime context stored on the `Executor` struct. Add a `Shutdown()` method that cancels the context and waits for active executions via `sync.WaitGroup`. Wire into `main.go`'s signal handler. ADVOCATE-1 conceded this is "architecturally superior." CRITIC-2 estimated ~10 lines. This prevents the integration from actively working against T5.2 (graceful shutdown) and is a prerequisite, not the full T5.2 implementation.

### Suggested Fixes

#### Bug Fixes (always in-PR)
- **Dead test** — `internal/dag/executor_test.go:253-283` — The test name `TestExecutor_ContextCancellation` promises cancellation semantics that were removed by this PR's change to `executor.go:50`. Update the test to expect `COMPLETED` (since context is now detached) and rename to reflect actual behavior (e.g., `TestExecutor_DetachedContext`). This is a direct consequence of the behavior change in this PR.

#### In-PR Improvements
- **Server-lifetime context** — `internal/dag/executor.go:15-18,50` and `cmd/orchestrator/main.go:29-31,49-51` — Add `ctx context.Context`, `cancel context.CancelFunc`, and `wg sync.WaitGroup` to the `Executor` struct. Change `NewExecutor` to accept a parent context. Replace `context.Background()` with `e.ctx` at line 50. Add `e.wg.Add(1)`/`defer e.wg.Done()` in `run()`. Add `Shutdown()` method. Wire into `main.go`'s signal handler between `sv.Stop()` and `server.GracefulStop()`.

#### PR Description Amendments
- Note that the `StoreSubmitter` intentionally drops `prompt` and `modelTier` as a documented MVP gap (test coverage at `storesubmitter_test.go:56-71`), to be resolved with model gateway integration.

#### New Issues (future features only — confirm with human)
- **StatusTracker record eviction** — The `records` map in `StatusTracker` (`internal/dag/status.go:42`) grows without bound. Add TTL-based eviction or max-size with LRU. This is F3 pre-existing code, not introduced by this PR, but the integration makes it reachable from the gRPC surface. — Feature
- **DAG execution concurrency limits** — No backpressure on concurrent DAG executions. Consider server-level gRPC middleware (`MaxConcurrentStreams`, interceptors) or an execution semaphore. F3 pre-existing design, flagged during integration review. — Feature
- **Sanitize catch-all error messages** — `handlers.go:95,112` pass raw `%v` errors to gRPC clients via `codes.Internal`. When `MockStore` is replaced with Redis, connection errors could leak infrastructure details. Wrap internal errors with a generic message. — Feature

---

*Council complete. 3 exchange rounds. 7 concessions recorded (3 advocate, 4 critic). QUESTIONER did not participate. CRITIC-1 did not respond to rebuttals.*
