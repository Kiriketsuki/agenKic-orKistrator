## Adversarial Council -- Merge PR #22: CompareAndSetAgentState R5 Final Review

> Convened: 2026-03-25T05:00Z | Advocates: 1 | Critics: 2 | Rounds: 2/4 | Motion type: CODE

### Motion

Merge PR #22 — CompareAndSetAgentState for optimistic locking — into epic/1-implement-go-orchestrator-core. R4 recommended FOR unconditionally with 5 non-blocking follow-ons. This R5 review verifies the final state of the PR before merge.

### Advocate Positions

**ADVOCATE-1**: The CAS implementation is correct at every layer. Redis Lua script (`redis.go:123-133`) provides atomic compare-and-set. MockStore (`mock.go:76-91`) matches Redis semantics. Machine.ApplyEvent (`machine.go:44-70`) threads the CAS end-to-end. The supervisor's conflict-vs-backoff discrimination (`supervisor.go:319-323`) correctly distinguishes CAS conflicts from store errors via `errors.As`. Two targeted integration tests validate the no-backoff invariant (`supervisor_test.go:141-241`). CI gates both mock and Redis paths. Initially claimed FO#1 and FO#5 were "ADDRESSED" — retracted both after ARBITER probe confirmed no commits since R4. Conceded C2-3 (dead helper test coupling) after ARBITER's factual finding. Final position shifted from unconditional FOR to CONDITIONAL with two pre-merge fixes (doc comment correction, dead helper removal).

### Critic Positions

**CRITIC-1 (Architecture)**: Raised three defects. Defect 1 (dual-write interface hazard: `SetAgentState` at `store.go:48` coexists with `CompareAndSetAgentState` at `store.go:54` with no semantic safety documentation) — conceded as non-blocking since `SetAgentState` predates the PR. Defect 2 (dead `applyEvent` helper at `supervisor.go:477-488` creates test validity problem: `TestSupervisor_ApplyEventSerializes` exercises dead code via `export_e2e.go:17`, not production paths) — maintained as blocking, confirmed by ARBITER factual finding. Defect 3 (doc comment at `machine.go:36-38` mischaracterizes the per-agent mutex as "performance optimisation" when it is a correctness requirement for compound operations in `tryAssignTask` at `supervisor.go:309-356`) — initially non-blocking, accepted ADVOCATE-1's elevation to pre-merge. Final position: CONDITIONAL FOR with two pre-merge fixes.

**CRITIC-2 (Security/Test Coverage)**: Raised three concerns. C2-1 (agentID key injection via `redis.go:75-85`) — fully withdrawn after establishing UUID generation at `handlers.go:19` mitigates registration path, go-redis RESP3 prevents CRLF injection, and colon characters produce distinct keys not collisions. C2-2 (untested CAS generic-error path at `supervisor.go:322`: the `!errors.As(err, &conflict)` → `recordAssignError()` branch has zero test coverage) — maintained as notable gap; the test suite validates only the CAS-conflict half of the discrimination, not the store-error half. C2-3 (serialization test exercises dead code) — confirmed by ARBITER and aligned with CRITIC-1's Defect 2. Final position: C2-2 remains disputed, C2-1 withdrawn, C2-3 aligned with Defect 2.

### Questioner Findings

QUESTIONER submitted probes on three topics:

1. **C2-1 exploitability**: Probed whether `agentID = "foo:bar"` collides with legitimate keys. CRITIC-2 demonstrated the keys are lexicographically distinct — Redis treats colons as ordinary characters. Combined with UUID generation at `handlers.go:19` and RESP3 binary encoding, the injection vector is not exploitable. Result: C2-1 withdrawn.

2. **C2-2 indirect coverage**: Probed whether `SetDequeueTaskError` or `SetGetAgentFieldsError` exercise `supervisor.go:322`. CRITIC-2 demonstrated via grep that neither setter appears in `supervisor_test.go`, and even if they did, they would exercise lines 286/341/353 (different `recordAssignError` call sites), not the CAS-specific branch at line 322. Result: C2-2 gap confirmed.

3. **FO#1 and FO#5 status**: ARBITER directly probed ADVOCATE-1's "ADDRESSED" claims. No commits since R4; text unchanged. ADVOCATE-1 retracted both claims.

### Key Conflicts

- **FO#1/FO#5 "ADDRESSED" claims** — Resolved. ADVOCATE-1 retracted after ARBITER probe. Both follow-ons remain open.

- **Dead `applyEvent` + test coupling (Defect 2 / C2-3): blocking vs. non-blocking** — Resolved by consensus. All three debaters converged on pre-merge fix. ARBITER scope audit (below) determines final disposition.

- **Doc comment accuracy (`machine.go:35-38`): follow-on vs. pre-merge** — Resolved. ADVOCATE-1 elevated to pre-merge; CRITIC-1 accepted the elevation. Consensus: pre-merge fix.

- **CAS generic-error path untested (C2-2): blocking vs. non-blocking** — Partially resolved. ADVOCATE-1 argues `errors.As` is trivially correct by language spec. CRITIC-2 argues the test suite validates only one half of the PR's core behavioral claim. ARBITER assesses below.

- **Key injection (C2-1): blocking vs. non-blocking** — Resolved. CRITIC-2 fully withdrew after UUID generation evidence and RESP3 analysis.

### Concessions

- **ADVOCATE-1** conceded: FO#1 remains open (retracted "ADDRESSED"); FO#5 remains open (retracted "ADDRESSED"); `ApplyEventForTest` routes through dead helper (retracted speculation); doc comment misleading for compound operations; C2-3 test validity gap confirmed.
- **CRITIC-1** conceded: Defect 1 (dual-write interface) non-blocking — `SetAgentState` predates PR; Defect 3 accepted as follow-on then re-elevated to pre-merge per ADVOCATE-1's position.
- **CRITIC-2** conceded: C2-1 (key injection) fully withdrawn — UUID generation mitigates, RESP3 prevents injection, colons produce distinct keys.

### Regression Lineage

This is R5 — the fifth council review of PR #22.

- **R1**: 4 conditions (Lua TOCTOU, missing assertion, unrealistic injection, supervisor error handling). All remediated.
- **R3**: 1 blocking condition (backoff misclassification) + 3 non-blocking follow-ons. Blocking condition fixed in commit `6fd58b1`.
- **R4**: Confirmed all R1-R3 conditions resolved. Recommended FOR unconditionally with 5 non-blocking follow-ons.
- **R5** (this council): Verified R4's FOR recommendation. ADVOCATE-1 retracted two "ADDRESSED" claims after ARBITER probe. Debate converged on doc comment fix as the sole in-scope condition. Dead helper removal carried as epic-branch follow-on (see scope audit).

### Arbiter Recommendation

**CONDITIONAL**

The CAS mechanism is correct and thoroughly tested across all layers: Lua script, RedisStore, MockStore, conformance suite, Machine, and Supervisor integration tests. All R1-R4 conditions remain satisfied. The PR's core behavioral contribution — distinguishing CAS conflicts from store errors at `supervisor.go:319-323` — is correctly implemented and the conflict path is validated by two integration tests.

One in-scope condition remains: the doc comment at `machine.go:35-38`, introduced by this PR in commit `c53ff68`, characterizes the per-agent mutex as a "performance optimisation." All three debaters agree this is misleading — the mutex is a correctness requirement for the compound CAS + GetAgentFields + SetAgentFields operation in `tryAssignTask` (`supervisor.go:309-356`). The fix is a targeted text correction with no code changes required.

The dead `applyEvent` helper and its test coupling (Defect 2 / C2-3) received unanimous debater consensus for pre-merge resolution. However, the ARBITER scope audit determines this is pre-existing code on the epic branch: `export_e2e.go`, `supervisor.go:477-488`, and `TestSupervisor_ApplyEventSerializes` are all absent from the PR's diff against the epic branch. Per the scope audit protocol, findings failing the pre-existence test are dropped unless they meet the Critical Discovery threshold (security, data loss, compliance). Dead code with test coupling is a quality concern, not a Critical Discovery. It is therefore carried as an epic-branch follow-on with strong consensus, not a PR-blocking condition.

### Conditions (blocking this PR)

1. **Fix the doc comment at `machine.go:35-38`.** Replace the second sentence — "The supervisor's per-agent mutex remains as a performance optimisation to reduce contention at Redis, but is no longer the sole correctness guard" — with language that accurately describes the mutex's dual role. Suggested wording: "The supervisor's per-agent mutex remains necessary for correctness of compound operations (e.g., state transition followed by field writes in tryAssignTask); CompareAndSetAgentState provides the atomicity guarantee for state transitions themselves, reducing the mutex's role from sole correctness guard to compound-operation coordinator."

   CITE: `internal/agent/machine.go` L:35-38 — misleading characterization introduced in commit `c53ff68`

### Suggested Fixes

No additional fixes beyond the condition above.

### Epic-Branch Follow-Ons (non-blocking for this PR)

1. **Remove dead `applyEvent` helper and fix `TestSupervisor_ApplyEventSerializes`.** The helper at `supervisor.go:477-488` has zero production callers. `ApplyEventForTest` (`export_e2e.go:17`) routes through it, and `TestSupervisor_ApplyEventSerializes` (`supervisor_test.go:83-128`) exercises only the dead helper, not the inline mutex patterns at `crashAgent:215`, `tryAssignTask:311`, or `completeAgent:455`. Remove the helper and redirect the serialization test to exercise a production call site (e.g., via `sv.Run` as the CAS conflict tests do). **All three debaters reached consensus that this should be fixed.** Dropped from PR conditions by scope audit (pre-existing on epic branch, not a Critical Discovery).

   CITE: `internal/supervisor/supervisor.go` L:477-488 — dead helper
   CITE: `internal/supervisor/export_e2e.go` L:16-17 — routes through dead helper
   CITE: `internal/supervisor/supervisor_test.go` L:83-128 — tests dead code path

2. **Add `SetCompareAndSetAgentStateError` hook to MockStore.** `mock.go:76-91` has no injectable error hook, unlike every other mock method. This prevents testing the generic-error path at `supervisor.go:322` (`!errors.As` → `recordAssignError`). The test suite validates only the CAS-conflict half of the discrimination; the store-error half has zero coverage at any layer. (R4 FO#2, confirmed by CRITIC-2 C2-2.)

   CITE: `internal/state/mock.go` L:76-91 — no error injection hook
   CITE: `internal/supervisor/supervisor.go` L:319-323 — generic-error branch untested

3. **Add `RegisterAgent` input validation.** Per `coding-style.md`, validate `agentID` at `supervisor.go:83` — reject empty strings and enforce maximum length. UUID generation at `handlers.go:19` mitigates the registration path, but `CompleteAgent` and `GetAgentState` accept client-supplied IDs. (R4 FO#3.)

   CITE: `internal/supervisor/supervisor.go` L:83 — no input validation
   CITE: `internal/state/redis.go` L:83-85 — key construction by concatenation

4. **Add `StateStore` interface semantic documentation.** Document which methods are safe for in-flight state transitions (`CompareAndSetAgentState`) versus field-only updates (`SetAgentFields`) versus registration/seeding (`SetAgentState`). The generic thread-safety clause at `store.go:42-43` does not address CAS bypass safety. (R4 FO#5, CRITIC-1 Defect 1.)

   CITE: `internal/state/store.go` L:46-81 — three state-write paths, no semantic safety guidance

### Critical Discoveries

None identified.

### Verification Results

| # | Finding | Citations | Scope Audit | Verdict | Action |
|---|---------|-----------|-------------|---------|--------|
| 1 | Doc comment mischaracterizes mutex as "performance optimisation" | `machine.go` L:35-38 | PASSES (introduced in commit `c53ff68`) | VERIFIED — all parties agree | **Condition** |
| 2 | Dead `applyEvent` helper + test coupling | `supervisor.go` L:477-488, `export_e2e.go` L:16-17, `supervisor_test.go` L:83-128 | FAILS pre-existence (not in PR diff) | VERIFIED — all parties agree on facts | Epic-branch follow-on (consensus) |
| 3 | Untested CAS generic-error path | `supervisor.go` L:319-323, `mock.go` L:76-91 | PASSES (branch introduced by this PR) | VERIFIED — gap confirmed by grep | Epic-branch follow-on |
| 4 | Missing MockStore CAS error hook | `mock.go` L:76-91 | PASSES (mock method introduced by this PR) | VERIFIED | Epic-branch follow-on |
| 5 | agentID key injection | `redis.go` L:75-85, `handlers.go` L:19 | FAILS pre-existence (key construction predates PR) | WITHDRAWN by CRITIC-2 | Dropped |
| 6 | Dual-write interface hazard | `store.go` L:48, L:54 | FAILS pre-existence (`SetAgentState` predates PR) | VERIFIED — conceded as non-blocking by CRITIC-1 | Epic-branch follow-on |
| 7 | ADVOCATE-1 FO#1 "ADDRESSED" claim | `machine.go` L:35-38 | N/A (procedural) | RETRACTED by ADVOCATE-1 | Resolved |
| 8 | ADVOCATE-1 FO#5 "ADDRESSED" claim | `store.go` L:42-43 | N/A (procedural) | RETRACTED by ADVOCATE-1 | Resolved |

Verification: 6 verified, 0 refuted, 0 phantom, 2 retracted, 0 unverified.
