---
## Adversarial Council — PR #13 F4 Health Probes: R4 Review

> Convened: 2026-03-16-180000 | Advocates: 2 | Critics: 1 | Rounds: 2/4 | Motion type: CODE

### Motion
PR #13 (F4 Health Probes) — all R3 council conditions have been applied: `TestAggregator_DualStoreFailure` added, `AgentDataValid`/`QueueDataValid` sentinel fields added to HealthSnapshot and exposed in /progress JSON, `RedisOK` decoupled to Ping-only (store method failures no longer set `RedisOK=false`), `slog.ErrorContext` logging added for all three store error paths. The implementation is correct, complete, and ready to merge.

### Advocate Positions
**ADVOCATE-1**: All R3 conditions met with evidence. The sole merge condition (`TestAggregator_DualStoreFailure`) is implemented at `aggregator_test.go:281-317` with all required assertions. All three R3 deferred debts (RedisOK semantic overload, TasksQueued zero-value ambiguity, error diagnostic completeness) are resolved in code. Test suite is comprehensive (12 aggregator tests, 6 HTTP tests), code follows immutability patterns, and HTTP server has proper timeout hardening. Conceded Objection 2 (missing `TasksQueued` assertion) as a pre-merge one-line addition.

**ADVOCATE-2**: Grounded position on three pillars — test coverage completeness, observability quality, and API surface correctness. Verified all six assertions in `DualStoreFailure` test match R3 requirements. Demonstrated that `slog.ErrorContext` logging with structured `"error"` keys gives operators actual Redis error messages. Showed sentinel fields resolve zero-value ambiguity programmatically. Conceded Objection 2 after CRITIC-1's consistency argument — the test's own documentation standard demands the `TasksQueued` assertion.

### Critic Positions
**CRITIC-1**: Raised four objections. Withdrew three after advocate rebuttals (Objections 1 partial, 3, 4). Pressed Objection 2: `TestAggregator_DualStoreFailure` omits `TasksQueued == 0` assertion, creating an inconsistency — the test re-asserts `RedisOK`, `AgentDataValid`, and `QueueDataValid` (all individually tested elsewhere) but omits `TasksQueued` despite R3 explicitly flagging its zero-value ambiguity. Argued this one-line addition should be pre-merge since this test is the R3 council's sole merge condition.

### Questioner Findings
QUESTIONER did not submit probes during the debate. No claims were flagged as unsubstantiated by the QUESTIONER. All file:line citations from advocates and critic were independently verified by the ARBITER during the pre-debate scan.

### Key Conflicts
- **Objection 1** (HTTP-level failure test for sentinel fields) — CRITIC-1 argued zero confidence in the `false` path; ADVOCATE-2 demonstrated existing tests catch key-typo and boolean-inversion mutations. **Resolved**: CRITIC-1 partially withdrew; acknowledged as defense-in-depth but not a merge blocker.
- **Objection 2** (`TasksQueued` assertion in `DualStoreFailure`) — ADVOCATE-2 initially argued "redundant"; CRITIC-1 showed internal inconsistency in the test's own documentation standard. **Resolved**: Both advocates conceded, agreeing it should be pre-merge.
- **Objection 3** (`/progress` 200 with invalid data) — CRITIC-1 argued self-contradictory HTTP contract; advocates demonstrated `/progress` is a data endpoint (not health), and non-2xx would cause Prometheus to discard the response. **Resolved**: CRITIC-1 fully withdrew.
- **Objection 4** (`RedisOK` naming and gRPC surface) — CRITIC-1 argued deferred debts unresolved; advocates showed naming is cosmetic (behavior fixed) and gRPC health protocol is binary by specification. **Resolved**: CRITIC-1 fully withdrew as merge blocker.

### Concessions
- CRITIC-1 conceded Objection 3 to ADVOCATE-1 and ADVOCATE-2 (Prometheus scraper argument decisive)
- CRITIC-1 conceded Objection 4 to ADVOCATE-2 (gRPC health protocol is binary by spec; naming is cosmetic)
- CRITIC-1 partially conceded Objection 1 to ADVOCATE-2 (mutation analysis sound; not a merge blocker)
- ADVOCATE-1 conceded Objection 2 to CRITIC-1 (test consistency argument; upgraded partial to full concession)
- ADVOCATE-2 conceded Objection 2 to CRITIC-1 (withdrew "redundant" framing; consistency argument decisive)

### Prior Council Cross-Reference
**R3 Condition #1** (`TestAggregator_DualStoreFailure`): **Confirmed implemented** at `aggregator_test.go:281-317`. R4 adds one refinement — the `TasksQueued` assertion omitted from the test.

**R3 Deferred Issue #1** (RedisOK semantic overload): **Confirmed resolved in behavior** — `redisOK` set from `pingErr == nil` only at `aggregator.go:72-73`. R4 consensus: naming (`RedisOK` vs `RedisPingOK`) is cosmetic follow-up, not a merge concern.

**R3 Deferred Issue #2** (TasksQueued zero-value ambiguity): **Confirmed resolved** via `QueueDataValid` sentinel field at `aggregator.go:34, 106-111, 134` and exposed in `/progress` JSON at `health_http.go:99`. R4 adds nuance: the test that closes R3 should explicitly assert `TasksQueued == 0` to document the zero-value fallback.

**R3 Deferred Issue #3** (Error diagnostic completeness): **Confirmed resolved** — `slog.ErrorContext` at `aggregator.go:75, 84, 109` with structured `"error"` key carrying the original error value.

**R3 Key Concession** (ADVOCATE-1 conceded err discarded at aggregator.go:77, 101): **No longer applicable** — those lines no longer exist in the current code; `slog.ErrorContext` logging replaces the silent discard.

### Arbiter Recommendation
**CONDITIONAL** — The PR satisfies the R3 merge condition and resolves all three R3 deferred design debts in code. The implementation is correct, well-tested, and architecturally sound. One minor condition: add a `TasksQueued == 0` assertion to `TestAggregator_DualStoreFailure` for internal consistency with the test's own documentation standard. All three positional agents (both advocates and the critic) agree this is the right resolution.

### Conditions
1. Add `if snap.TasksQueued != 0 { t.Errorf("TasksQueued = %d, want 0 (zero-value fallback on error)", snap.TasksQueued) }` to `TestAggregator_DualStoreFailure` at approximately `aggregator_test.go:310` (before the `AgentDataValid` check). One line. All parties agreed this should be pre-merge.

### Suggested Fixes

#### Bug Fixes (always in-PR)
None identified. No bugs were raised or substantiated during the debate.

#### In-PR Improvements (scoped, non-bug)
1. **Add `TasksQueued` assertion to `TestAggregator_DualStoreFailure`** — one line at `aggregator_test.go:~310`. Addresses R3's `TasksQueued` zero-value ambiguity concern and maintains the test's internal consistency standard. (Objection 2 — all parties agreed.)

#### PR Description Amendments
None needed.

#### New Issues (future features/enhancements only — confirm with human before creating)
> NEVER list bugs here. Confirm with team lead before filing.
1. **Rename `RedisOK` to `RedisPingOK`** — cosmetic clarity improvement. Behavior is correct; naming could better communicate Ping-only semantics. (R3 deferred, R4 confirmed as cosmetic follow-up.)
2. **Add HTTP-level failure-path test for `/progress` sentinel fields** — defense-in-depth for serialization boundary. Not a gap in coverage (existing tests catch mutations), but adds explicit verification that `agent_data_valid=false` and `queue_data_valid=false` serialize correctly over HTTP. (Objection 1 — withdrawn as merge blocker but acknowledged as valuable.)
