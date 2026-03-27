## Adversarial Council — PR #13 F4 Health Probes: R3 Review

> Convened: 2026-03-16 | Advocates: 1 | Critics: 2 | Rounds: 2/4 | Motion type: CODE

### Motion
PR #13 (F4 Health Probes, feature/8-adding-feature-f4-health-probes) — all R1 and R2 council conditions have been applied in code. The implementation is correct, complete, and ready to merge.

### Advocate Positions
**ADVOCATE-1**: All six R2 conditions have been mechanically and semantically applied. The three store-error paths (Ping, GetAllAgentStates, QueueLength) now have symmetrical mock error injection. The `agentDataValid` flag correctly decouples agent-count checking from unrelated store failures, fixing the semantic bug identified in R2. All 11 tests pass across 6 packages. Code quality is sound: immutable value types, pure functions, files under 800 lines. The `readiness()` signature change is purely internal with no exported API impact.

### Critic Positions
**CRITIC-1**: Three objections raised. (1) No test covers `GetAllAgentStates` + `QueueLength` both failing simultaneously — this is a novel interaction path created by this PR's QueueLength error injection and the `agentDataValid` refactor. (2) `RedisOK` is semantically overloaded — now means "all store ops succeeded" not "Redis is reachable." (3) `TasksQueued = 0` on QueueLength error is indistinguishable from an empty queue, creating ambiguity for dashboard consumers. Asks: dual-failure test (non-negotiable), tracking issues for RedisOK and TasksQueued.

**CRITIC-2**: Four objections raised. (1) Ping failure is silently suppressed when any store method also fails — the `len(storeErrors) == 0` guard at `aggregator.go:136` prevents "redis unreachable" from appearing alongside specific store errors. (2) `RedisOK` semantically overloaded (convergent with CRITIC-1). (3) No test for simultaneous Ping + GetAllAgentStates failure. (4) `TestAggregator_BothDown` misnomer masks coverage gap. Additionally raised: actual `err` values from store calls are discarded at `aggregator.go:77` and `aggregator.go:101` — operators get hardcoded generic strings with no underlying error detail. Proposed `fmt.Sprintf` wrapping, later conceded security concern and dropped to tracking issue.

### Questioner Findings
QUESTIONER was inactive throughout the debate. No substantiation probes were filed. The ARBITER performed independent fact-checking in lieu of QUESTIONER findings:

- **ADVOCATE-1's claim** that "the store-level error message will already contain the Redis error string": **UNSUBSTANTIATED.** Code at `aggregator.go:77` and `aggregator.go:101` uses hardcoded generic strings; `err` is discarded. ADVOCATE-1 conceded this claim in their final summary.
- **ADVOCATE-1's claim** that `redisOK = false` at `aggregator.go:102` was not changed by R2: **SUBSTANTIATED.** Verified via `git diff ccbeff0^ ccbeff0 -- internal/health/aggregator.go`. The R2 commit did not touch lines 99-103.
- **CRITIC-1's claim** that "this PR made RedisOK worse by adding `aggregator.go:102` in R2": **UNSUBSTANTIATED** as stated. The line was added in R1. However, CRITIC-2's refined version — that R2 *activated and cemented* the behavior by adding mock error injection and a test asserting `RedisOK=false` — is substantiated.

### Key Conflicts
- **Dual-failure test necessity** — ADVOCATE-1 argued the two mechanisms (`agentDataValid` and `storeReasons`) are independent with no conditional interaction, making a dual-failure test "structurally redundant." CRITIC-1 countered that independence is today's implementation detail, not a permanent guarantee, and that testing compositions is standard practice. **Resolved**: ADVOCATE-1 conceded the test is a reasonable merge condition.
- **Error wrapping in ReadyReason** — CRITIC-2 proposed `fmt.Sprintf("...unavailable: %v", err)` to include underlying Redis errors. ADVOCATE-1 raised a security concern: raw Redis error strings can contain internal IPs, connection URIs, and ACL details that would leak through an HTTP health endpoint. **Resolved**: CRITIC-2 conceded the security concern and dropped error wrapping as a merge condition, replacing it with a tracking issue for the design question (error classification vs. structured logging).
- **"Pre-existing vs R2 regression" framing** — ADVOCATE-1 argued RedisOK overloading and error discarding pre-date R2. Critics argued R2 worsened/cemented these via new tests. **Resolved**: ARBITER verified via git that `aggregator.go:102` was R1, not R2. Both sides agreed to track as design debt rather than block.

### Concessions
- **ADVOCATE-1** conceded to **CRITIC-1** and **CRITIC-2**: the claim that store-level error messages contain Redis error strings is factually wrong — `err` is discarded at both `aggregator.go:77` and `aggregator.go:101`.
- **ADVOCATE-1** conceded to **CRITIC-1**: the dual-failure test is a reasonable merge condition (~15 lines, locks a novel interaction path).
- **ADVOCATE-1** conceded to **CRITIC-1** and **CRITIC-2**: tracking issues for `RedisOK` semantics and `TasksQueued` ambiguity are accepted.
- **CRITIC-1** conceded to **ADVOCATE-1**: `redisOK = false` at `aggregator.go:102` was added in R1, not R2. The specific claim that R2 added that line was factually wrong.
- **CRITIC-2** conceded to **ADVOCATE-1**: raw `err` wrapping via `fmt.Sprintf` is unsafe for an exported health endpoint (internal IP/topology leakage). Dropped as merge condition, replaced with tracking issue.
- **CRITIC-2** conceded **Finding 4** (TestAggregator_BothDown misnomer) as not merge-blocking. Folded into the broader test coverage argument.

### Prior Council Cross-Reference
**R1** (2026-03-16-120000): Returned CONDITIONAL with 7 fixes. All applied. R3 does not contradict any R1 findings.

**R2** (2026-03-16-165000): Returned CONDITIONAL with 1 condition (`SetQueueLengthError` + test) and 1 in-PR fix (`storeErrors` guard refinement via `agentDataValid`). R3 confirms:
- The `agentDataValid` flag correctly addresses R2's core condition (decouple agent-count check from unrelated store failures). Both critics and the advocate agree the semantic fix is correct.
- The `SetQueueLengthError` mock injection and `QueueLength` error handling are properly implemented.
- All 11 tests pass.

**R3 adds nuance to R2**: R2's condition was narrowly scoped to the guard logic. R3 reveals that the QueueLength error path introduces a novel interaction (dual-failure) that R2 did not explicitly require testing for but which the council unanimously agrees warrants a test. R3 also surfaces three design debts (RedisOK semantics, TasksQueued ambiguity, error diagnostic completeness) that R2 did not address — these are deferred to tracking issues, not merge blockers.

### Arbiter Recommendation
**CONDITIONAL**

The R2 conditions have been correctly applied in code. The `agentDataValid` flag is semantically sound, the mock error injection is symmetrical, and all tests pass. However, the QueueLength error injection introduced in R2 creates a novel dual-failure interaction path (GetAllAgentStates + QueueLength both failing) that has no test coverage. All three debaters — including the advocate — agree this test should exist. The test is ~15 lines and locks a regression-prone code path. Three design debts were identified and should be tracked explicitly alongside the merge.

### Conditions (if CONDITIONAL)
1. **Add `TestAggregator_DualStoreFailure`** — test that both `GetAllAgentStates` and `QueueLength` fail simultaneously. Verify: `Ready=false`, `RedisOK=false`, `ReadyReason` contains both "agent states unavailable" and "queue length unavailable", `ReadyReason` does NOT contain "no agents registered" (agent count check skipped via `agentDataValid=false`), and Ping's "redis unreachable" is correctly suppressed. (~15 lines, in-PR)

### Suggested Fixes

#### Bug Fixes (always in-PR)
No bugs identified.

#### In-PR Improvements
1. **Dual-failure test** (condition above) — `internal/health/aggregator_test.go`. Registers zero agents, injects errors on both `GetAllAgentStates` and `QueueLength`, asserts the combined ReadyReason and that the agent-count check is correctly skipped.

#### PR Description Amendments
No amendments needed.

#### New Issues (future only — confirm with human before creating)
1. **RedisOK semantic overload** — `HealthSnapshot.RedisOK` (`aggregator.go:31`) means "all store operations succeeded," not "Redis is reachable." Consider splitting into `RedisPingOK` (connectivity) and `StoreOK` (operation health), or renaming to `StoreHealthy`. Affects exported API — needs design review.
2. **TasksQueued zero-value ambiguity** — `TasksQueued = 0` on `QueueLength` error is indistinguishable from an empty queue. Consider `*int64` (nil on error), a sentinel value, or a companion `TasksQueuedValid bool`. Same pattern applies to `AgentsTotal` — fix should be consistent across all numeric fields.
3. **Error diagnostic completeness** — actual `err` values from `GetAllAgentStates` (`aggregator.go:77`) and `QueueLength` (`aggregator.go:101`) are silently discarded. Operators get generic strings with no underlying error detail. Design question: should `ReadyReason` carry safe error classifications (e.g., "connection error" / "command error" / "timeout") or should detail go to structured logging (`slog.Error`) alongside the generic reason? Raw `err` wrapping is unsafe for HTTP-exposed health endpoints (internal IP/topology leakage).
