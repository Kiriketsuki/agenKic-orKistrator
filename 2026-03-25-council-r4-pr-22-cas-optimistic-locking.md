## Adversarial Council -- Merge PR #22: CompareAndSetAgentState for Optimistic Locking (R4 Post-Remediation Review)

> Convened: 2026-03-25T04:05Z | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion

Merge PR #22 -- CompareAndSetAgentState for optimistic locking (R4 post-remediation review) -- into epic/1-implement-go-orchestrator-core. All R1-R3 council conditions are claimed resolved. This review verifies the R3 fixes (backoff misclassification, silent error logging, tightened test assertion) and assesses overall merge readiness.

### Advocate Positions

**ADVOCATE-1**: All four R3 conditions -- one blocking, three non-blocking -- are verifiably resolved. The backoff misclassification fix at `supervisor.go:319-323` correctly distinguishes `*StateConflictError` from store errors via `errors.As`, preventing CAS conflicts from triggering supervisor-global exponential backoff. Two new tests cover this: `TestSupervisor_TryAssignTask_CASConflict_ReenqueuesTask` (exact queue assertion, priority preservation) and `TestSupervisor_TryAssignTask_CASConflict_NoBackoff` (verifies >= 5 dequeue attempts in 100ms, proving no backoff). Non-blocking fixes (crashAgent logging at `supervisor.go:216`, checkHeartbeats logging at `supervisor.go:156,166`, tightened test assertion at `supervisor_test.go:177`) are all in place. The CAS chain is end-to-end correct across `machine.go:60`, `mock.go:76-91`, and `redis.go:123-162`. CI gates all tests. Conceded CRITIC-2 Gap 3 (input validation) as valid per coding standards but non-blocking given no external callers exist.

### Critic Positions

**CRITIC-1**: The R3 fixes are mechanically correct, but the PR introduces a misleading doc comment at `machine.go:36-38` characterizing the per-agent mutex as "a performance optimisation" when it remains a correctness guard for the `GetAgentFields` -> `SetAgentFields` sequence at `supervisor.go:330-333` (which writes `fieldState` via plain `HSet` at `redis.go:168-174`, bypassing CAS). The comment was introduced by this PR (commit `c53ff68`) and is prescriptive guidance that teaches future contributors the mutex is removable -- which would produce a real data race. Additionally flagged: `SetAgentState` and `SetAgentFields` on the `StateStore` interface (`store.go:48,57`) provide non-CAS state-write paths alongside `CompareAndSetAgentState` (`store.go:54`) with no interface-level documentation distinguishing safe from unsafe usage. Conceded Argument 3 (dead `applyEvent` helper at `supervisor.go:477-488`).

**CRITIC-2**: Withdrew Gap 1 (testenv tag misclassification -- factually incorrect; `Makefile:18` passes `-tags=testenv` in `make test`). Reclassified Gap 2 (missing `SetCompareAndSetAgentStateError` hook on `MockStore` at `mock.go:76-91`) as a follow-on: the `errors.As` guard at `supervisor.go:320-323` has two branches but only the `StateConflictError` branch is tested; the genuine-store-error branch has no direct test. Reclassified Gap 3 (no input validation at `supervisor.go:83` / `redis.go:83-85`) as a follow-on per ADVOCATE-1's argument that no external callers exist yet.

### Questioner Findings

QUESTIONER submitted four probes:

1. **Idempotency vs interleaving** (probe to ADVOCATE-1): Asked whether the per-agent mutex prevents concurrent *readers* from observing intermediate state. ADVOCATE-1 responded: concurrent readers CAN observe the window but the intermediate state equals the CAS-committed state ("assigned"), so no reader observes a contradictory value. ARBITER verified: `findIdleAgent` at `supervisor.go:408` reads `GetAgentState` without the per-agent mutex, by design (snapshot pattern at `supervisor.go:389`). The intermediate state is consistent.

2. **Doc comment provenance** (probe to CRITIC-1): Asked whether `machine.go:36-38` is new to this PR. ARBITER verified via `git diff`: the comment IS new in this PR (commit `c53ff68`). The previous comment said "callers must serialize ApplyEvent calls per agentID."

3. **StateStore interface provenance** (probe to CRITIC-1): Asked whether the three state-write paths are pre-existing or introduced by this PR. ARBITER verified: `SetAgentState` and `SetAgentFields` are pre-existing. Only `CompareAndSetAgentState` is new. The interface-width concern is pre-existing.

4. **MockStore CAS error hook provenance** (probe to CRITIC-2): Asked whether `CompareAndSetAgentState` on MockStore is new. ARBITER verified via diff: yes, `mock.go:76-91` is entirely new in this PR. The missing error injection hook is therefore a gap introduced by this PR.

### Key Conflicts

- **Doc comment accuracy (`machine.go:36-38`)** -- Partially resolved. CRITIC-1 maintains the "performance optimisation" label is factually incorrect given `SetAgentFields`'s non-CAS state write. ADVOCATE-1 counters that "no longer the sole correctness guard" accurately describes the dual-layer model. QUESTIONER probed whether the concern is a current race or hypothetical-if-removed; ADVOCATE-1 confirmed no race exists with the mutex in place. The ARBITER finds the comment is imprecise but the runtime is correct.

- **MockStore CAS error injection hook** -- Resolved as follow-on. CRITIC-2 reclassified after accepting ADVOCATE-1's argument that the R3 fix verification claim is narrow ("CAS conflicts do not trigger backoff") and the tested branch covers that claim.

- **Input validation** -- Resolved as follow-on. Both parties agree the gap is real per `coding-style.md` but non-blocking given no external callers exist.

### Concessions

- **ADVOCATE-1** conceded: CRITIC-2 Gap 3 (input validation) is valid per `coding-style.md` but non-blocking.
- **CRITIC-1** conceded: Argument 3 (dead `applyEvent` helper at `supervisor.go:477-488`) -- code quality, not a correctness defect or merge blocker.
- **CRITIC-2** conceded: Gap 1 (testenv tag) -- factually incorrect, withdrawn. Gap 2 reclassified as follow-on. Gap 3 reclassified as follow-on.

### Regression Lineage

This is the fourth council review of PR #22.

- **R1**: 4 conditions (Lua TOCTOU, missing assertion, unrealistic injection, supervisor error handling). All remediated.
- **R3**: 1 blocking condition (backoff misclassification) + 3 non-blocking follow-ons. The blocking condition was introduced by the R1 remediation (removing the separate `errors.As` branch inadvertently created uniform backoff). Fixed in commit `6fd58b1`.
- **R4** (this council): Confirms R3 fix is correct. No new blocking conditions. The doc comment imprecision at `machine.go:36-38` was introduced in commit `c53ff68` (part of the original CAS implementation, not the R3 remediation).

### Arbiter Recommendation

**FOR**

All R1-R3 council conditions are satisfied and verified in code. The R3 blocking fix (backoff misclassification) at `supervisor.go:319-323` correctly distinguishes CAS conflicts from store errors, is covered by two targeted tests, and all parties agree it is correct. The three non-blocking R3 follow-ons (logging, test assertion) are all implemented. No new runtime defects were identified by this council.

CRITIC-1's strongest remaining argument -- that the doc comment at `machine.go:36-38` mislabels the per-agent mutex as a "performance optimisation" -- is a documentation imprecision, not a runtime defect. The runtime path is correct: the mutex IS held across the `SetAgentFields` sequence at `supervisor.go:309-356`. The comment could be more precise (e.g., "the per-agent mutex guards both the CAS retry window and the post-CAS field write; CAS provides the atomicity guarantee for state transitions"). However, a comment wording improvement does not warrant a fifth council round. It is included as a non-blocking suggestion below.

CRITIC-2's remaining findings (MockStore CAS error injection hook, input validation) were both reclassified as follow-ons by CRITIC-2 themselves. The ARBITER concurs: the MockStore hook gap is real and introduced by this PR, but it does not undermine the R3 fix verification. Input validation is pre-existing.

After four council rounds, the CAS mechanism, its error handling, its test coverage, and its integration with the supervisor are all sound. The PR should merge.

### Conditions (if CONDITIONAL)

None. This is an unconditional FOR recommendation.

### Suggested Fixes

None required for merge.

### Epic-Branch Follow-Ons (non-blocking for this PR)

1. **Clarify doc comment at `machine.go:36-38`.** Replace "remains as a performance optimisation" with language that accurately describes the mutex's dual role: it guards both the CAS retry window (reducing contention) and the post-CAS `GetAgentFields` -> `SetAgentFields` choreography (correctness for `CurrentTaskID` writes). Suggested wording: "The supervisor's per-agent mutex guards the post-transition field writes and reduces CAS contention at Redis; CompareAndSetAgentState provides the atomicity guarantee for state transitions themselves."

   CITE: `internal/agent/machine.go` L:36-38 -- imprecise characterization of mutex role

2. **Add `SetCompareAndSetAgentStateError` hook to MockStore.** `mock.go:76-91` implements `CompareAndSetAgentState` but has no injectable error hook, unlike every other store method. This prevents unit-testing the genuine-store-error path through `supervisor.go:320-323` (the branch that calls `recordAssignError`).

   CITE: `internal/state/mock.go` L:76-91 -- no error injection hook
   CITE: `internal/supervisor/supervisor.go` L:320-323 -- genuine-error branch untested at unit level

3. **Add `RegisterAgent` input validation.** Per `coding-style.md`, validate `agentID` at `supervisor.go:83` before passing to `SetAgentFields`. At minimum: reject empty strings and enforce a maximum length. Wire validation when the gRPC handler is added.

   CITE: `internal/supervisor/supervisor.go` L:83 -- no input validation
   CITE: `internal/state/redis.go` L:83-85 -- `agentKey` constructs Redis keys by concatenation

4. **Dead code: `applyEvent` helper at `supervisor.go:477-488`.** Only reachable via `ApplyEventForTest`. All production callers use `sv.machine.ApplyEvent` directly with inline mutex management. Consider removing the helper or migrating production callers to use it consistently.

   CITE: `internal/supervisor/supervisor.go` L:477-488 -- unused in production paths

5. **Add `StateStore` interface documentation for state-write safety.** Document which methods are safe for concurrent state transitions (`CompareAndSetAgentState`) versus field-only updates (`SetAgentFields`) versus registration/seeding (`SetAgentState`).

   CITE: `internal/state/store.go` L:46-81 -- three state-write paths, no safety guidance

### Critical Discoveries

None identified.

### Verification Results

| # | Finding | Citations | Verdict | Action |
|---|---------|-----------|---------|--------|
| 1 | R3 backoff fix correct -- CAS conflicts skip `recordAssignError()` | `supervisor.go` L:319-323 | VERIFIED -- all parties agree | Resolved (R3 condition satisfied) |
| 2 | R3 crashAgent logging added | `supervisor.go` L:216 | VERIFIED | Resolved |
| 3 | R3 checkHeartbeats logging added | `supervisor.go` L:156, L:166 | VERIFIED | Resolved |
| 4 | R3 test assertion tightened to exact equality | `supervisor_test.go` L:177 | VERIFIED | Resolved |
| 5 | R3 no-backoff test added | `supervisor_test.go` L:206-241 | VERIFIED | Resolved |
| 6 | Doc comment imprecision ("performance optimisation") | `machine.go` L:36-38 | VERIFIED -- new in this PR, imprecise but not a runtime defect | Non-blocking follow-on |
| 7 | Missing MockStore CAS error injection hook | `mock.go` L:76-91 | VERIFIED -- new in this PR, gap in branch coverage | Non-blocking follow-on |
| 8 | Input validation absent at RegisterAgent | `supervisor.go` L:83, `redis.go` L:83-85 | VERIFIED -- pre-existing, no external callers | Non-blocking follow-on |
| 9 | Dead `applyEvent` helper | `supervisor.go` L:477-488 | VERIFIED -- conceded by CRITIC-1 as non-blocking | Non-blocking follow-on |
| 10 | StateStore interface lacks safety documentation | `store.go` L:46-81 | VERIFIED -- pre-existing interface, new method added | Non-blocking follow-on |
| 11 | testenv tag misclassification | `supervisor_test.go` L:1, `Makefile` L:18 | REFUTED -- `make test` includes `-tags=testenv` | Dropped (CRITIC-2 withdrew) |

Verification: 10 verified, 1 refuted, 0 phantom, 0 unverified.
