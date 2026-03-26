## Adversarial Council — PR #13: F4 Health Probes

> Convened: 2026-03-16 | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion
PR #13 (F4 Health Probes) is correct, well-designed, and ready to merge as-is.

### Advocate Positions
**ADVOCATE-1**: The PR delivers a textbook three-layer health architecture — domain aggregator (`health/aggregator.go`), HTTP transport (`ipc/health_http.go`), gRPC transport (`ipc/health_grpc.go`) — with clean separation of concerns, consumer-side interface design (`DAGStatusProvider`), immutable snapshot patterns, proven thread safety (concurrent access tests), and 25 functional tests using deterministic infrastructure (httptest/bufconn). Kubernetes-native semantics are correct: liveness is unconditional process-alive, readiness gates on Redis + agent count, progress exposes operational metrics. Functional options enable extensibility without breaking callers. Started at FOR, moved to **CONDITIONAL FOR** after conceding 7 targeted fixes — none requiring architectural changes.

### Critic Positions
**CRITIC-1**: Identified 6 defects. The strongest were: (1) gRPC health protocol violation — `RunHealthUpdater` returns on `ctx.Done()` without calling `hs.Shutdown()`, so `NOT_SERVING` is never set during graceful shutdown (`health_grpc.go:46-47`); (2) shutdown ordering bug — `tryAssignTask` at `supervisor.go:176-203` dequeues and assigns tasks after `executor.Shutdown()` but before `cancel()`, causing task loss (`main.go:83-89`); (3) misleading diagnostics — `ListAgents` error at `aggregator.go:70` is swallowed, causing "no agents registered" when the real cause is a store failure. Conceded own points on HTTP timeouts (hardening, not correctness) and string state severity (enhancement, not blocker). Final verdict: **AGAINST as-is, FOR after fixes 1-4.**

**CRITIC-2**: Identified 6 defects independently. Strongest unique contributions: (1) the `AgentsTotal != sum(buckets)` invariant violation when unrecognized states exist — `aggregator.go:78-87` has no `default` case, so agents in new states are counted in `total` but in no bucket; (2) test/production path divergence — `RegisterHealthService` at `health_grpc.go:16-20` is used only in tests while production uses `WithHealthServer` + `StartGRPC` at `server.go:57-62`, meaning the production registration path has zero test coverage; (3) dead code in both `handleHealthz` (`health_http.go:55-58`) and `handleReadyz` (`health_http.go:66-69`). Conceded N+3 store calls as non-blocking at current scale, always-alive as valid k8s pattern, and dual registration as doc-comment-level concern. Final verdict: **CONVERGED — no remaining objections beyond agreed fix list.**

### Questioner Findings
QUESTIONER probed 5 claims post-debate. Arbiter evaluation:

1. **ADVOCATE-1: "gRPC follows standard `grpc.health.v1`"** — SUBSTANTIATED. `health_grpc.go:8-9` imports the standard library; lines 16-20 use `grpc_health_v1.RegisterHealthServer`; lines 33-38 use standard `SERVING`/`NOT_SERVING` enums.
2. **ADVOCATE-1: "Functional Options for Extensibility"** — SUBSTANTIATED. Production usage confirmed at `main.go:53` (`health.WithMinAgents(minAgents)`) and `main.go:56` (`ipc.WithHealthServer(hs)`).
3. **CRITIC-1: gRPC protocol "requires" NOT_SERVING on shutdown** — RAISED BUT NUANCED. The gRPC health checking protocol doc recommends this behavior for proper drain semantics. Whether it is a strict protocol violation or a best-practice gap does not affect the recommendation — ADVOCATE-1 conceded the fix regardless.
4. **CRITIC-2: "No test covers unrecognized states"** — SUBSTANTIATED. Arbiter reviewed all test files in scope; no test sets an agent to a state outside "idle"/"working"/"assigned"/"reporting".
5. **CRITIC-2: "RegisterHealthService is used ONLY in tests"** — SUBSTANTIATED. Only call site is `health_grpc_test.go:26`. Production at `main.go:55` calls `grpchealth.NewServer()` directly.

Additionally, one claim was self-corrected during the debate: CRITIC-1 withdrew "panic or data race" phrasing on Point 2 after ADVOCATE-1 challenged it — `grpchealth.Server` is thread-safe and `SetServingStatus()` after `GracefulStop()` is a no-op, not a race.

No claims were marked unsubstantiated. All load-bearing claims in the recommendation are grounded.

### Key Conflicts
- **Shutdown ordering** — ADVOCATE-1 initially dismissed as "benign no-ops"; CRITIC-1 strengthened with `tryAssignTask` data loss evidence at `supervisor.go:176-203`. **Resolved**: ADVOCATE-1 conceded task loss is real.
- **Error swallowing in aggregator** — ADVOCATE-1 argued "fails closed is safe"; CRITIC-1 argued "misleading diagnostics undermine observability contract." **Resolved**: ADVOCATE-1 conceded the health system should not report misleading reasons.
- **String state matching** — ADVOCATE-1 argued coupling trade-off (importing `agent` from `health`); both critics argued silent invariant violation on state addition. **Resolved**: ADVOCATE-1 conceded `AgentsUnknown` counter; broader constant migration deferred to follow-up.
- **Dual gRPC registration** — CRITIC-2 argued test/production path divergence; ADVOCATE-1 argued trivial wiring. **Resolved**: CRITIC-2 reduced to doc comment recommendation, not merge-blocking.
- **Dead code branches** — Both critics identified unreachable code in `handleHealthz` and `handleReadyz`. **Resolved**: ADVOCATE-1 conceded removal.

### Concessions
- **ADVOCATE-1** conceded to CRITIC-1: gRPC NOT_SERVING (#1), shutdown task loss (#2), misleading diagnostics (#3), dead readyz branch (#4)
- **ADVOCATE-1** conceded to CRITIC-2: dead healthz branch (#3), missing /progress fields (#6), AgentsUnknown counter (#1)
- **CRITIC-1** conceded to ADVOCATE-1: architecture points 1-4 and 8 (layered design, interfaces, immutability, thread safety, functional options); HTTP timeouts (#6, hardening not correctness); string state severity (#5, enhancement not blocker); withdrew "panic or data race" phrasing (#2)
- **CRITIC-2** conceded to ADVOCATE-1: always-alive is valid k8s pattern (#3 partial); N+3 not performance bottleneck at current scale (#5); dual registration reduced to doc comment (#2)

### Arbiter Recommendation
**CONDITIONAL**

The motion "ready to merge as-is" fails — all three council members agree the PR cannot merge without fixes. However, the architecture and design are sound, as acknowledged by both critics. The defects are in shutdown wiring (`main.go:83-89`), protocol compliance (`health_grpc.go:46-47`), error propagation (`aggregator.go:70,90`), and dead code (`health_http.go:55-58,66-69`) — not in the health probe design itself. The three-layer architecture, consumer-side interfaces, immutable snapshots, and comprehensive test suite (25 tests) represent solid engineering. With the 7 fixes below applied, the PR is merge-ready.

### Conditions
- All 7 items under "Bug Fixes" and "In-PR Improvements" must be applied before merge
- Documentation items (8-9) are strongly recommended but not blocking

### Suggested Fixes

#### Bug Fixes (always in-PR)
- Add `hs.Shutdown()` before `return` on `ctx.Done()` — `ipc/health_grpc.go:47` — gRPC health protocol recommends NOT_SERVING on graceful shutdown so load balancers drain traffic; without it, k8s gRPC health checks see stale SERVING during pod termination
- Move `cancel()` before `server.GracefulStop()` in shutdown sequence — `cmd/orchestrator/main.go:83-89` — supervisor's `tryAssignTask` dequeues and assigns tasks after executor is dead, causing task loss in the window between `executor.Shutdown()` and `cancel()`
- Check and propagate `ListAgents` and `QueueLength` errors into readiness reason string — `internal/health/aggregator.go:70,90` — discarded errors cause misleading "no agents registered" diagnostic when the real cause is a store failure

#### In-PR Improvements
- Remove dead `else` branch returning `{"status": "dead"}` — `ipc/health_http.go:55-58` — unreachable because `Alive` is unconditionally `true`
- Remove dead `redisStatus` branch inside ready path — `ipc/health_http.go:66-69` — unreachable because `Ready=true` implies `RedisOK=true`
- Add `agents_assigned`, `agents_reporting`, and `agents_unknown` to `/progress` JSON response — `ipc/health_http.go:90-97` — HealthSnapshot computes these values but the HTTP endpoint silently drops them
- Add `default` case in aggregator switch incrementing `AgentsUnknown` counter — `internal/health/aggregator.go:78-87` — without it, agents in unrecognized states count toward `AgentsTotal` but no bucket, breaking the sum invariant

#### PR Description Amendments
- Note the 7 fixes applied and their rationale
- Document the shutdown ordering fix as addressing a task-loss scenario, not just cleanup

#### New Issues (future only — confirm with human before creating)
- Migrate agent state constants to `state` package — eliminates raw string matching between `health` and `agent` packages, provides compile-time safety — Feature
- Add `GetAllAgentStates` batch method to `StateStore` interface — reduces O(N) per-probe round trips against real Redis — Feature
- Add HTTP server timeouts (`ReadHeaderTimeout`, `WriteTimeout`) to `HealthHTTPServer` — defense-in-depth hardening for pod-network exposure — Task
