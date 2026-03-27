## Adversarial Council — PR #13 F4 Health Probes: Follow-Up Review

> Convened: 2026-03-16 | Advocates: 2 | Critics: 1 | Rounds: 3/4 | Motion type: CODE

### Motion
PR #13 (F4 Health Probes) is correct, complete, and ready to merge.

### Advocate Positions
**ADVOCATE-1**: All 7 prior-council conditions are satisfied in code with file:line citations. The architecture is a clean three-layer design (domain aggregator → HTTP transport → gRPC transport) with immutable snapshots, interface-based coupling, and functional options. 18 tests across 3 files cover all critical paths including shutdown transitions, error propagation, unknown-state bucketing, and the bucket-sum invariant. The QueueLength error path at `aggregator.go:97-101` uses the identical `storeReasons` mechanism proven by the `GetAllAgentStates` test, and is simpler (no nil-map guard needed). The prior council's 7 conditions were the explicit merge gate — all are met. The test gap is real but appropriate for a fast-follow in a research-phase project.

**ADVOCATE-2**: Independent verification of all 7 fixes with citations. gRPC protocol compliance is textbook: standard `grpc_health_v1` package (`health_grpc.go:8-9`), dual service naming for `""` and `"orchestrator.OrchestratorService"` (`health_grpc.go:37-38`), proper `hs.Shutdown()` on context cancellation (`health_grpc.go:47`). Kubernetes probe semantics are correct: `/healthz` is unconditional liveness, `/readyz` gates on Redis + agent count, `/progress` is informational. The implementation improved on the prior council's Fix #3 by using `GetAllAgentStates` (a single batch operation at `aggregator.go:74`) instead of `ListAgents` + per-agent `GetAgentState`, reducing per-probe round trips. HTTP server timeouts are set at `health_http.go:32-34`, addressing a prior council "future" recommendation proactively.

### Critic Positions
**CRITIC-1**: Raised 4 objections, withdrew 3 after debate. Sole remaining objection: the `QueueLength` error path at `aggregator.go:97-101` has zero test coverage — no `SetQueueLengthError` exists in `MockStore` (`mock.go`), and no test exercises this branch. Of the 3 store methods called by `Aggregator.Check()` (`Ping`, `GetAllAgentStates`, `QueueLength`), two have mock error injection and dedicated tests; one does not. This asymmetry is unexplained. Both advocates concede the gap and agree the fix is trivial (~15 lines). CRITIC-1 argues "implemented and verified" is a higher bar than "implemented and plausibly correct by analogy" and requests CONDITIONAL merge on adding the test.

### Questioner Findings
QUESTIONER did not file independent probes during this council session. All load-bearing claims were substantiated through direct code citations and mutual agreement between debate participants. No claims are marked unsubstantiated. The Arbiter independently verified CRITIC-1's Objection 1: confirmed that `MockStore.QueueLength` at `mock.go:168-173` unconditionally returns `nil` error, no `SetQueueLengthError` method exists, and no aggregator test exercises the `QueueLength` error path.

### Key Conflicts
- **QueueLength test gap severity** — CRITIC-1 argued merge-blocking (project testing standards, 2-of-3 asymmetry); Advocates argued fast-follow (mechanism proven by analogy, prior council conditions met, research-phase project). **Unresolved — Arbiter judgment below.**
- **Coarse storeErrors guard** — CRITIC-1 argued `ReadyReason` is incomplete in partial-failure scenarios (`aggregator.go:138`); ADVOCATE-1 showed `/progress` endpoint at `health_http.go:83-99` provides full diagnostics regardless. **Resolved: CRITIC-1 withdrew.**
- **Shutdown ordering guarantee** — CRITIC-1 argued no happens-before between `cancel()` and `hs.Shutdown()` (`main.go:86-87`); ADVOCATE-2 showed `GracefulStop()` blocks, giving scheduler time, and SERVING during active drain is semantically correct. **Resolved: CRITIC-1 withdrew (standard Go pattern, no practical impact).**
- **Hardcoded `"redis": "ok"`** — CRITIC-1 argued brittle coupling at `health_http.go:72`; ADVOCATE-1 showed adding a conditional would reintroduce the dead code Fix #5 removed; ADVOCATE-2 showed it would change the JSON type (API contract). **Resolved: CRITIC-1 withdrew (YAGNI, API contract valid).**

### Concessions
- **CRITIC-1** conceded to ADVOCATE-2: Objection 3 (shutdown race — standard Go pattern), Objection 4 (hardcoded redis:ok — YAGNI + API contract), QueueLength path is genuinely simpler (no nil-map guard)
- **CRITIC-1** conceded to ADVOCATE-1: Objection 2 (storeErrors guard — `/progress` provides full diagnostics), 80% coverage is package-level not per-branch
- **ADVOCATE-1** conceded to CRITIC-1: straw man on `SetEnqueueTaskError` analogy — only the 3 store methods called by `Aggregator.Check()` are relevant, and 2 of 3 have tests
- **ADVOCATE-2** conceded to CRITIC-1: invoking research-phase informality while arguing the council's formal conditions were rigorously met is inconsistent — the stronger argument is that the mechanism is proven and the path is trivially simple

### Prior Council Cross-Reference

All 7 fixes from the prior council (2026-03-16-120000) were evaluated:

| Fix | Prior Requirement | Status | Notes |
|-----|-------------------|--------|-------|
| 1 | `hs.Shutdown()` on `ctx.Done()` | Applied and tested | `health_grpc.go:47`, test at `health_grpc_test.go:102-118` |
| 2 | `cancel()` before `GracefulStop()` | Applied | `main.go:86-87`, documented comment at lines 78-80. Standard Go pattern (CRITIC-1 conceded). |
| 3 | Propagate store errors to readiness | Code applied; partially tested | `GetAllAgentStates` error: tested (`aggregator_test.go:194-217`). `QueueLength` error: code at `aggregator.go:97-101` is correct but untested. Implementation improved over prior suggestion by using batch `GetAllAgentStates`. |
| 4 | Remove dead `else` `{"status":"dead"}` | Applied | `health_http.go:56-60` — clean unconditional handler |
| 5 | Remove dead `redisStatus` branch | Applied | `health_http.go:62-81` — documented comment at line 67 |
| 6 | Add assigned/reporting/unknown to `/progress` | Applied and tested | `health_http.go:92-94`, tests at `health_http_test.go:147-152, 172` |
| 7 | Add `default` case for `AgentsUnknown` | Applied and tested | `aggregator.go:92-93`, test at `aggregator_test.go:168-192` with bucket-sum invariant assertion |

The current debate **confirms** all 7 prior findings. It **adds nuance** to Fix #3: the code implementation is correct but the `QueueLength` half lacks test verification. No prior findings are **contradicted**.

### Arbiter Recommendation
**CONDITIONAL**

All 7 prior-council conditions are satisfied in code — the implementation is correct and the architecture is sound. CRITIC-1's sole remaining objection is well-founded on the facts: of the 3 store-error paths in `Aggregator.Check()`, two (`Ping` and `GetAllAgentStates`) have mock error injection and dedicated tests, while the third (`QueueLength`) has neither. Both advocates concede the gap and agree the fix is trivial. The principle that "implemented and verified" is a higher standard than "implemented and plausibly correct by analogy" is sound engineering practice. Given the near-zero cost of closing this gap (~15 lines of code) versus the nonzero cost of tracking it as tech debt in a research-phase project with no enforced follow-up SLA, the balance favours testing before merge.

### Conditions
- Add `SetQueueLengthError` to `MockStore` and a `TestAggregator_QueueLengthError` test covering the error branch at `aggregator.go:97-101` (analogous to the existing `SetGetAllAgentStatesError` + `TestAggregator_GetAllAgentStatesError` pattern)

### Suggested Fixes

#### Bug Fixes (always in-PR)
No bug fixes required. All prior-council bug fixes (1, 2, 3) are correctly implemented.

#### In-PR Improvements
- Add `SetQueueLengthError(err error)` method to `MockStore` — `internal/state/mock.go` — enables error injection for the `QueueLength` path, completing the error-injection surface for all store methods called by the health aggregator
- Add `TestAggregator_QueueLengthError` test — `internal/health/aggregator_test.go` — should inject `QueueLength` failure, assert `Ready=false`, `RedisOK=false`, reason contains `"queue length unavailable"`, and verify the reason does NOT misleadingly report agent-count issues (following the pattern at `aggregator_test.go:194-217`)

#### PR Description Amendments
- Note that all 7 prior-council fixes were applied and verified by a follow-up adversarial council
- Document the `GetAllAgentStates` batch improvement over the prior council's `ListAgents` suggestion

#### New Issues (future only — confirm with human before creating)
- Refine `storeErrors` guard in `readiness()` to distinguish `GetAllAgentStates` failure from other store failures — improves `ReadyReason` diagnostic completeness in partial-failure scenarios — Feature (low priority; `/progress` already provides full diagnostics)
- Migrate agent state constants to a shared package — eliminates string matching between `health` and `agent` packages — Feature (carried forward from prior council)
