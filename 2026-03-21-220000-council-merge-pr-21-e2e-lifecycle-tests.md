## Adversarial Council — Merge PR #21: E2E Lifecycle Tests

> Convened: 2026-03-21 | Advocates: 2 | Critics: 2 | Rounds: 4/4

### Motion
Merge PR #21 (feat: E2E lifecycle tests) — adding 448 lines of E2E lifecycle tests, a test-e2e Makefile target, and an ApplyEventForTest exported method on Supervisor.

---

### Advocate Positions

**ADVOCATE-1**: The PR delivers 7 spec-compliant test scenarios covering the full in-process component stack under race detection. All confirmed defects are targeted fixes totalling fewer than 10 lines. A CONDITIONAL verdict preserves 448 lines of integration coverage the epic branch currently lacks entirely while enforcing every correction the critics demand. The build-tag alternative for `ApplyEventForTest` is a valid architectural improvement but belongs as a follow-up refactor, not a pre-merge blocker. Spec scenarios 7 and 8 (restart/backoff, circuit breaker) require supervisor restart-policy wiring to real process failures — correctly deferred with explicit documentation.

**ADVOCATE-2**: The debate produced complete consensus on what is wrong. Every confirmed fix is ≤2 lines. The core contribution is undisputed: 7 scenarios exercising Store → Machine → Supervisor → DAG Executor → gRPC under the race detector. Spec scenarios at `go-orchestrator-core-spec.md:130` and `:137` require future implementation work (OTP restart policy, circuit breaker) that is out of scope for this test-addition PR. A CONDITIONAL verdict delivers the same quality outcome as AGAINST without the cost of a rejected-and-resubmitted PR.

---

### Critic Positions

**CRITIC-1**: Five pre-merge corrections were confirmed by the advocates themselves. The build-tag alternative (`//go:build testenv`) was raised as providing compiler-enforced exclusion of `ApplyEventForTest` from production binaries — neither advocate argued this approach was technically infeasible, only that it was a follow-up preference. "Fix before merge" and "not yet mergeable in submitted state" are operationally equivalent: the PR as committed contains none of the five required corrections. CONDITIONAL AGAINST: the work has value, but the PR in its current form carries confirmed defects in a production source file and a test harness with a known race window.

**CRITIC-2**: Four defects confirmed in production file and test harness. The doc comment corruption at `internal/supervisor/supervisor.go:237` is a production-file bug shipping incorrect godoc for an exported method. Cleanup ordering at `e2e/lifecycle_test.go:79–84` creates a race window that `-race` cannot reliably detect. The `time.Sleep(100ms)` negative assertion has a trivial structural alternative using `s.store` directly. The build-tag approach was ultimately withdrawn as a blocking argument (legitimate discovery tradeoffs acknowledged) but the method's blast radius within the module is confirmed non-zero. The PR should not merge with confirmed bugs in a production source file.

---

### Key Conflicts

- **`ApplyEventForTest` on production type vs. build-tag alternative** — Advocates argued the method is acceptable in an `internal/` package on an epic branch; CRITIC-1 argued the build-tag alternative is uncontested as infeasible; CRITIC-2 ultimately withdrew the build-tag approach as a merger blocker, conceding its discovery tradeoffs. ADVOCATE-1 fully conceded: (a) `ApplyEventForTest` IS exported (capital letter), (b) blast radius within the module is NOT zero, (c) the `testcert` analogy was false equivalence. **Resolved**: build-tag approach is a valid future improvement; current method is accepted for this PR with conditions.

- **Pre-merge vs. post-merge fix timing** — Advocates framed confirmed fixes as "request changes" on the existing PR; critics framed the same fixes as "not yet mergeable." CRITIC-1 correctly observed these are operationally equivalent. **Resolved**: all parties agreed on the required corrections; the ARBITER finds the distinction immaterial and renders CONDITIONAL.

- **E2E label scope** — ADVOCATE-1 proposed a package-level doc comment clarifying the in-process scope and noting gRPC-bypassed scenarios; CRITIC-2 accepted this as satisfactory. **Resolved**: add package-level annotation (in-PR improvement).

- **Missing spec scenarios 6 and 7** — ARBITER independently verified `go-orchestrator-core-spec.md:130` ("Agent crashes and restarts with backoff") and `:137` ("Circuit breaker trips after repeated failures") are absent from the test file. ADVOCATE-1's citation challenge was resolved by ARBITER verification. Advocates argued deferral is appropriate because the supervisor restart policy is not yet wired to real agent process failures. **Resolved**: deferral is appropriate; must be made explicit in PR description (not invisible).

- **`ForTest` suffix as Go convention** — ADVOCATE-1 initially cited `net/http/internal/testcert` as standard library precedent; conceded the analogy was false equivalence; withdrew the claim. **Resolved in critics' favour**: `ForTest` is not an established Go spec idiom; naming convention carries zero compile-time enforcement.

---

### Concessions

- **ADVOCATE-1** conceded: `ApplyEventForTest` is an exported symbol, not unexported — ARBITER CLARIFY issued
- **ADVOCATE-1** conceded: blast radius within the module is not zero — to CRITIC-1 / CRITIC-2
- **ADVOCATE-1** conceded: `testcert` analogy was false equivalence — to CRITIC-1
- **ADVOCATE-1** conceded: `-race` flag does not mitigate the cleanup ordering race window — to CRITIC-2
- **ADVOCATE-1** conceded: cleanup ordering at `e2e/lifecycle_test.go:79–84` is a pre-merge fix — to CRITIC-1
- **ADVOCATE-1** conceded: doc comment corruption at `internal/supervisor/supervisor.go:237` is a real production-file bug — to CRITIC-2
- **ADVOCATE-1** conceded: spec scenarios at `go-orchestrator-core-spec.md:130` and `:137` are confirmed absent (ARBITER-verified) — citation challenge resolved
- **ADVOCATE-1** conceded: `pollDAGComplete` error swallowing at `e2e/lifecycle_test.go:138` is a pre-merge fix
- **ADVOCATE-2** conceded: scenario numbering gap at `e2e/lifecycle_test.go:407` is a documentation defect — to CRITIC-1
- **ADVOCATE-2** conceded: doc comment corruption at `internal/supervisor/supervisor.go:237` is confirmed — to CRITIC-2
- **ADVOCATE-2** conceded: build-tag alternative is a valid architectural improvement worth pursuing — to CRITIC-2
- **CRITIC-2** conceded: `sv.Run(ctx)` return value discard at `e2e/lifecycle_test.go:66` is benign (`Run` always returns `nil` on context cancel) — to ADVOCATE-2
- **CRITIC-2** conceded: build-tag approach has legitimate discovery tradeoffs; withdrew it as a blocking argument — to ADVOCATE-2

---

### Arbiter Recommendation

**CONDITIONAL**

The PR's core contribution — 7 test scenarios exercising the full in-process component stack (Store → Machine → Supervisor → DAG Executor → gRPC) under the race detector — is substantive and undisputed by all parties. However, three confirmed defects require correction before the PR proceeds: a displaced doc comment in a production file (`internal/supervisor/supervisor.go:237`) that causes `godoc` to misattribute the `getAgentMutex` description to `ApplyEventForTest`; a structural test teardown defect (`e2e/lifecycle_test.go:79–84`) where `cancel()` fires after `grpcServer.GracefulStop()`, creating a supervisor-tick window against a draining server that `-race` cannot reliably detect; and a silent error discard in `pollDAGComplete` (`e2e/lifecycle_test.go:138–140`) that degrades CI diagnostic quality when the DAG gRPC endpoint is broken. Both advocates agreed to all three corrections, making the practical outcome unanimous. The `ApplyEventForTest` method is accepted as a documented internal-package test seam on this epic branch; a build-tag hardening path is valid and should be tracked as a follow-up task.

---

### Conditions (CONDITIONAL — all must be applied to branch before merge)

1. Fix displaced doc comment at `internal/supervisor/supervisor.go:237`: move the `getAgentMutex` sentence to precede `getAgentMutex` at line 246; write a clean doc block for `ApplyEventForTest` that accurately describes its purpose.
2. Fix cleanup ordering at `e2e/lifecycle_test.go:79–84`: call `cancel()` before `grpcServer.GracefulStop()` so the supervisor's goroutines stop before the gRPC server drains.
3. Fix `pollDAGComplete` at `e2e/lifecycle_test.go:138–140`: surface `GetDAGStatus` errors (at minimum via `t.Logf`) rather than silently continuing; a broken DAG gRPC endpoint should produce a clear failure message, not an opaque timeout.

---

### Suggested Fixes

#### Bug Fixes (always in-PR)

- **Displaced `getAgentMutex` doc comment** — `internal/supervisor/supervisor.go:237` — The line `// getAgentMutex returns the per-agent mutex, or nil if not registered.` precedes `ApplyEventForTest` (line 242), causing `godoc` to render it as that method's documentation. `getAgentMutex` at line 246 is left without a doc comment. Move the sentence to immediately precede `getAgentMutex`; add an accurate standalone doc block to `ApplyEventForTest`.

- **Cleanup ordering race window** — `e2e/lifecycle_test.go:79–84` — Current order: `conn.Close()` → `grpcServer.GracefulStop()` → `cancel()` → `executor.Shutdown()`. Correct order: `cancel()` first (stops supervisor goroutines), then wait for goroutines if possible, then `grpcServer.GracefulStop()`, then `conn.Close()`, then `executor.Shutdown()`.

- **`pollDAGComplete` silent error discard** — `e2e/lifecycle_test.go:138–140` — `GetDAGStatus` errors are caught and silently continued. At minimum add `t.Logf("GetDAGStatus error (will retry): %v", err)` before the `continue`; alternatively fail fast on non-transient errors.

#### In-PR Improvements (scoped, non-bug)

- **Scenario comment mislabeling** — `e2e/lifecycle_test.go:407` — The comment `// ── Scenario 9: Parallel Fork DAG` is the 7th test function in the file. Renumber to match actual sequence, or align all comment labels with spec scenario numbers and document the skipped OTP scenarios explicitly.

- **Package-level scope annotation** — `e2e/lifecycle_test.go` (package declaration) — Add a `// Package e2e_test exercises the in-process orchestrator stack (MockStore → Machine → Supervisor → DAG Executor → gRPC via bufconn). Scenarios that use ApplyEventForTest to inject state transitions are marked [gRPC-bypassed]; full gRPC lifecycle coverage for those paths will follow once agent-side gRPC clients are implemented. Spec scenarios for OTP restart (spec:130) and circuit breaker (spec:137) are deferred pending supervisor restart-policy integration.`

- **`time.Sleep(100ms)` negative assertion** — `e2e/lifecycle_test.go:246` — Replace wall-clock sleep with a structural store invariant check: verify via `s.store` that the task remains in the queue (e.g., confirm queue depth or that the task ID is still present). The `testStack.store` field at line 29 provides direct access. CRITIC-2 conceded this has near-zero implementation cost; ADVOCATE-2 conceded the structural approach is more robust.

- **Goroutine setup error surfacing** — `e2e/lifecycle_test.go:63` (`grpcServer.Serve`) — The `_ = grpcServer.Serve(lis)` discard loses diagnostics if the bufconn setup fails before any test method runs. Consider capturing the error on a channel and checking it in `newTestStack` or failing the test on non-`ErrServerStopped` returns.

#### PR Description Amendments

- Add an explicit **Deferred Scenarios** section to the PR description noting that `go-orchestrator-core-spec.md:130` (Agent crashes and restarts with backoff) and `go-orchestrator-core-spec.md:137` (Circuit breaker trips after repeated failures) are intentionally absent — both require supervisor restart-policy wiring to real agent process failures, which is future implementation work. Without this note the gap is invisible in the epic branch history.

#### New Issues (future features only — confirm with human before creating)

- **Harden `ApplyEventForTest` with build-tag constraint** — Create a separate file `internal/supervisor/testing_helpers.go` guarded with `//go:build testenv` containing `ApplyEventForTest`; update `Makefile:24` test-e2e target to pass `-tags=testenv`. This moves the method from naming-convention exclusion to build-system-enforced exclusion and removes it from production binaries. Neither advocate argued this is technically infeasible; both accepted it as a valid follow-up improvement. — Task

- **E2E test: supervisor restart with exponential backoff** — Implement the acceptance scenario at `go-orchestrator-core-spec.md:130` ("Agent crashes and restarts with backoff") once the supervisor's `RestartPolicy` is wired to real agent process lifecycle events. — Feature

- **E2E test: circuit breaker after repeated failures** — Implement the acceptance scenario at `go-orchestrator-core-spec.md:137` ("Circuit breaker trips after repeated failures") once the circuit-breaker state machine is implemented. — Feature
