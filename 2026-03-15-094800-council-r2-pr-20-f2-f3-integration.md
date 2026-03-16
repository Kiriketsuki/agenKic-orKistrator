---
## Adversarial Council — PR #20 F2-F3 Integration (Round 2 Validation)

> Convened: 2026-03-15T09:48:00Z | Advocates: 1 | Critics: 2 | Rounds: 1/2 (called early — full convergence) | Motion type: CODE

### Motion
PR #20 — F2-F3 Integration: Wire DAG Executor into gRPC Handlers — should be merged into task/1-task-implement-go-orchestrator-core, following application of Round 1 fixes.

### Round 1 Context
Round 1 (2026-03-15T09:32:00Z) issued **CONDITIONAL** with 2 required fixes:
1. Fix dead `TestExecutor_ContextCancellation` test
2. Add server-lifetime context to `Executor`

Both fixes were committed in `5443d42` and pushed. This round validates their adequacy.

### Pre-Debate Scan
- **git diff 10e5ca3...HEAD**: Fix commit adds `ctx`, `cancel`, `wg` to Executor struct; replaces dead test with `TestExecutor_ShutdownCancelsRunning`; wires `executor.Shutdown()` into `main.go` signal handler
- **Test results**: All 37 tests pass (20 in `internal/dag` @ 0.049s, 17 in `internal/ipc` @ 0.018s, 0 failures)

### Advocate Positions
**ADVOCATE-1**:
- Fix 1 (server-lifetime context) is verified correct: `Executor` struct gains `ctx`, `cancel`, `wg` fields (`executor.go:18-20`); `NewExecutor` accepts parent context (`executor.go:26-33`); `Shutdown()` cancels and waits (`executor.go:37-40`); goroutines tracked with `wg.Add(1)` / `defer wg.Done()` (`executor.go:64-68`); `main.go:29-33` threads server-lifetime context; signal handler calls `executor.Shutdown()` at `main.go:50`
- Fix 2 (dead test) is verified correct: renamed to `TestExecutor_ShutdownCancelsRunning` (`executor_test.go:253`); exercises `Shutdown()` directly (`executor_test.go:275`); asserts strictly `FAILED` not the old `FAILED or COMPLETED` tautology (`executor_test.go:281`)
- No new issues introduced by fix commit
- Conceded CRITIC-1's shutdown ordering follow-up as a valid improvement

### Critic Positions
**CRITIC-1** (0 blockers, 1 follow-up):
- Both conditions verified satisfied with file:line citations
- Systematic check: WaitGroup correctness, double-cancel safety, concurrent tracker access, error wrapping, handler error mapping, interface satisfaction, StoreSubmitter adapter, test coverage — all clear
- Raised architectural observation (NOT a blocker): `executor.Shutdown()` before `server.GracefulStop()` in `main.go:47-53` creates a theoretical window where a late-arriving RPC calls `Execute()` with `wg.Add(1)` while `Shutdown()` is in `wg.Wait()`, technically violating `sync.WaitGroup` reuse contract. Recommended follow-up: swap to `server.GracefulStop()` -> `executor.Shutdown()` -> `cancel()`

**CRITIC-2** (0 blockers):
- Both conditions verified satisfied with file:line citations
- Proactive edge-case analysis: race conditions (none — `e.ctx` immutable after construction), WaitGroup reuse after Shutdown (safe — new execution fails immediately on cancelled context), shutdown ordering in main.go (benign failure mode), test cleanup in server_test.go (goroutines complete naturally), nil-pointer on TaskSpec (consistent with prior concession)
- Concurred with CRITIC-1's shutdown ordering follow-up recommendation

### Questioner Findings
QUESTIONER did not submit probes during the debate (consistent with Round 1). No claims were marked unsubstantiated. All code claims from debaters were verified by ARBITER pre-debate scan (git diff, test run, file reads).

### Key Conflicts
- **Shutdown ordering WaitGroup semantics** — CRITIC-1 raised the theoretical `wg.Add(1)` during `wg.Wait()` contract violation; ADVOCATE-1 argued the WaitGroup is not "reused" (Wait never returns while Add is pending); CRITIC-2 confirmed the behavioral outcome is safe (FAILED execution, no panic/leak). **Resolved**: all three agreed this is a follow-up improvement, not a merge blocker. The race window is microsecond-scale during SIGTERM handling, and the failure mode is benign.

No other conflicts arose. All positions converged in Round 1.

### Concessions
- **ADVOCATE-1** conceded to CRITIC-1: shutdown ordering swap (`GracefulStop()` -> `Shutdown()` -> `cancel()`) is a valid hardening follow-up
- **CRITIC-1** conceded to ADVOCATE-1/CRITIC-2: the WaitGroup concern is not a merge blocker — behavioral outcome is correct regardless
- **CRITIC-2** conceded to CRITIC-1: the during-Shutdown `wg.Add(1)` case is technically a spec violation (distinct from the post-Shutdown case CRITIC-2 originally analyzed)

### Arbiter Recommendation
**FOR**

Both Round 1 conditions are fully met. The server-lifetime context implementation (`executor.go:15-40, 62-68`, `main.go:29-33, 50`) follows textbook Go shutdown patterns: `context.WithCancel` for cancellation propagation, `sync.WaitGroup` for goroutine tracking, `Shutdown()` for clean drain. The dead test has been replaced with a deterministic test (`executor_test.go:253-284`) that exercises the actual `Shutdown()` path and asserts a single expected state. All 37 tests pass. No new defects were introduced by the fix commit. All three debaters independently verified the fixes and converged to the same conclusion without contested claims.

### Conditions (if CONDITIONAL)
None. Both prior conditions have been satisfied.

### Suggested Fixes

#### Bug Fixes (always in-PR)
No issues identified.

#### In-PR Improvements
No issues identified.

#### PR Description Amendments
- Note that the `StoreSubmitter` intentionally drops `prompt` and `modelTier` as a documented MVP gap (test coverage at `storesubmitter_test.go:56-71`), to be resolved with model gateway integration. *(Carried from Round 1 — still applicable.)*

#### New Issues (future features only — confirm with human)
- **Shutdown ordering hardening** — `cmd/orchestrator/main.go:47-53`: swap to `server.GracefulStop()` -> `executor.Shutdown()` -> `cancel()` so the gRPC server stops accepting new RPCs before draining the executor. Eliminates the theoretical WaitGroup reuse window during SIGTERM. All three debaters agreed this is a valid follow-up. — Feature (hardening)
- **StatusTracker record eviction** — `internal/dag/status.go:42`: unbounded `records` map. Add TTL-based eviction or max-size with LRU. F3 pre-existing, carried from Round 1. — Feature
- **DAG execution concurrency limits** — No backpressure on concurrent DAG executions. F3 pre-existing, carried from Round 1. — Feature
- **Sanitize catch-all error messages** — `handlers.go:95,112`: raw `%v` errors to gRPC clients via `codes.Internal`. Carried from Round 1. — Feature

---

*Council complete (Round 2). 1 exchange round (called early — full convergence). 3 concessions recorded (1 advocate, 2 critic). QUESTIONER did not participate. All debaters aligned: FOR merge with no conditions.*
