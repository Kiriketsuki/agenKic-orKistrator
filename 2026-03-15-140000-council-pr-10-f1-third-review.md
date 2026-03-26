---
## Adversarial Council — PR #10: F1 Third Review

> Convened: 2026-03-15 | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion
PR #10 (Adding [Feature]: F1 — Foundation: Scaffold, Proto, Redis, Agent State Machine) is complete — all 8 prior council-mandated items have been applied (6 code fixes from the first council + 2 test additions from the second council), and the branch `feature/5-adding-feature-f1-foundation-scaffold-proto-redis-agent-state-machine` is ready to squash-merge into `task/1-task-implement-go-orchestrator-core`.

### Advocate Positions
**ADVOCATE-1**: All 6 code fixes from the first council are correctly implemented and committed (`1fb8bae`), verified with line-by-line citations. Both test additions from the second council are authored, verified, and passing (all 18 subtests green) — but not yet committed. Transparently flagged the uncommitted state in opening argument. Conceded the wrapping assertion in `TestMachine_UnrecognisedState_ReturnsError` does not fully satisfy the second council's "assert a wrapped error is returned" language — accepted `strings.Contains` or similar as a condition. Confirmed CRITIC-2's MockStore/RedisStore failure trace as technically accurate but argued the prior council explicitly deferred it (`council2:58`). Rebutted CRITIC-2's `EventAgentFailed` coverage concern by citing exhaustive pure-function coverage at `transition_test.go:36-55`. Adopted CRITIC-1's operational definition of "ready to merge" and accepted CONDITIONAL as the appropriate outcome.

### Critic Positions
**CRITIC-1**: Two blocking grounds maintained: (1) the two mandated tests are uncommitted — `git diff HEAD` shows +43 lines not in any commit, and a squash-merge of HEAD would lose them; (2) `TestMachine_UnrecognisedState_ReturnsError` (`machine_test.go:138-141`) checks only `err == nil` with no wrapping verification, failing to satisfy the second council's condition at `council2:42`. Proposed `errors.Unwrap(err) != nil` as a non-fragile structural assertion testing the `%w` in `machine.go:48`, plus `strings.Contains(err.Error(), "bogus_state")` for context verification. Withdrew Ground 3 (phantom commit in PR description) as cosmetic. Conceded the `errors.As` comparison to `InvalidTransitionError` was structurally unfair since `ParseAgentState` returns a bare `fmt.Errorf`. Drew a useful distinction: unfulfilled prior mandates are blocking; newly-surfaced gaps may be conditioned or deferred.

**CRITIC-2**: Four grounds raised; two maintained as blockers, one downgraded to condition, one withdrawn. Maintained: (1) uncommitted tests are a hard blocker (aligned with CRITIC-1); (2) wrapping assertion gap is a hard blocker (aligned with CRITIC-1). Downgraded: MockStore/RedisStore behavioral divergence — provided a concrete, deterministic failure trace (`SetAgentState` at `redis.go:88-96` writes only `state` field → `GetAgentFields` at `redis.go:137` calls `ParseInt("")` on never-written `last_heartbeat` → error; MockStore succeeds because Go zero-value `int64(0)` is valid). Accepted prior council's explicit deferral but requests interface godoc documenting the divergence so F2 developers don't hit the crash blind. Withdrew: `EventAgentFailed` 4-state coverage gap — conceded after ADVOCATE-1 cited `TestValidTransition_AgentFailed_FromAnyState` at `transition_test.go:36-55` covering all 4 states at the unit level. Confirmed `StateStore` interface at `store.go:32-54` has zero method-level godoc.

### Questioner Findings
QUESTIONER probed effectively throughout all rounds:
1. Distinguished "applied" (working tree) from "committed" (git history) — forced precision on the motion's claims. ADVOCATE-1 refined to "addressed — 6 committed, 2 authored and verified but awaiting commit."
2. Asked whether the full test body of `TestMachine_UnrecognisedState_ReturnsError` contains any wrapping assertion — ARBITER confirmed it does not (function ends at `machine_test.go:141` after the `err == nil` check).
3. Probed whether `StateStore` interface documents cross-method contracts — CRITIC-2 confirmed `store.go:32-54` has zero method-level godoc, no documentation on `SetAgentState`/`GetAgentFields` interaction.
4. Asked whether tests should verify wrapping rather than relying on code review to catch `%w` removal — catalyzed ADVOCATE-1's concession that the test doesn't fully satisfy the council's language.
5. Confirmed ADVOCATE-1 accepts the technical accuracy of CRITIC-2's MockStore/RedisStore failure trace (`redis.go:88-96` → `redis.go:137`).
6. Clarified the philosophical disagreement: ADVOCATE-1 and CRITIC-2 disagree on timing (defer to F2 vs. fix now), not on facts.

All claims raised in debate were substantiated with file:line citations. No claims marked unsubstantiated.

### Key Conflicts
- **Wrapping assertion adequacy** — CRITIC-1/CRITIC-2 said test doesn't satisfy "assert a wrapped error is returned" (`council2:42`); ADVOCATE-1 initially defended `err != nil` as sufficient, then conceded — **resolved: all agree wrapping assertion must be added**
- **MockStore/RedisStore fidelity gap timing** — CRITIC-2 argued blocking with concrete failure trace (`redis.go:137` `ParseInt("")`); ADVOCATE-1 cited prior council's explicit deferral (`council2:58`) — **resolved: CRITIC-2 downgraded to condition, accepts deferral with documentation request**
- **EventAgentFailed state coverage** — CRITIC-2 said only 2/4 states tested at integration level; ADVOCATE-1 cited `transition_test.go:36-55` exhaustive unit coverage of all 4 states — **resolved: CRITIC-2 withdrew objection**
- **"Ready to merge" semantics** — ADVOCATE-1 initially argued decision-readiness; CRITIC-1 argued operation-readiness citing the motion's literal "squash-merge" language; ADVOCATE-1 adopted CRITIC-1's definition and accepted CONDITIONAL — **resolved: all agree CONDITIONAL is the appropriate outcome**

### Concessions
- **ADVOCATE-1** conceded wrapping assertion gap to **CRITIC-1** — test needs structural wrapping check
- **ADVOCATE-1** conceded uncommitted tests need committing (flagged transparently in opening)
- **ADVOCATE-1** adopted **CRITIC-1**'s operational definition of "ready to merge"
- **ADVOCATE-1** confirmed **CRITIC-2**'s MockStore/RedisStore failure trace as technically accurate
- **CRITIC-1** withdrew Ground 3 (phantom commit in PR description) as cosmetic to **ADVOCATE-1**
- **CRITIC-1** conceded `errors.As` comparison to `InvalidTransitionError` was structurally unfair to **ADVOCATE-1**
- **CRITIC-2** withdrew EventAgentFailed 4-state coverage objection after **ADVOCATE-1** cited `transition_test.go:36-55`
- **CRITIC-2** downgraded MockStore/RedisStore gap from "blocking" to "strongly recommended condition"
- **CRITIC-2** conceded `errors.As` point (echoed from **CRITIC-1**)

### Arbiter Recommendation
**CONDITIONAL**

All 6 first-council code fixes are correctly implemented and committed. Both second-council test additions are correctly authored, verified passing (18/18 subtests, 0 failures), and match the council's specifications in substance — but are not yet committed and the wrapping assertion in `TestMachine_UnrecognisedState_ReturnsError` does not fully satisfy the second council's exact condition ("assert a wrapped error is returned" at `council2:42`). The debate converged across all three sides to CONDITIONAL: the implementation is architecturally sound, the tests are correct in substance, and the path to merge-ready is small and well-defined.

On the MockStore/RedisStore fidelity gap: the prior council explicitly deferred this to F2 (`council2:58`), and CRITIC-2 accepted that deferral after presenting a concrete failure trace. The trace is technically accurate and deterministic — `redis.go:137` will `ParseInt("")` on a field never written by `SetAgentState` — but no F1 code exercises this cross-method path. The appropriate resolution is in-PR documentation (interface godoc on `StateStore`) so F2 developers are aware. This prevents the "forgotten footnote" problem CRITIC-2 identified without expanding the PR's scope.

On `EventAgentFailed` state coverage: `TestValidTransition_AgentFailed_FromAnyState` at `transition_test.go:36-55` exhaustively tests all 4 states at the pure function level. Machine integration tests cover 2 representative states, and the Machine delegates unconditionally to `ValidTransition` without branching on source state (`machine.go:51`). CRITIC-2 withdrew this objection. No condition needed.

CRITIC-1's distinction between unfulfilled prior mandates (blocking) and newly-surfaced gaps (condition/defer) is well-taken and guided the weighting of conditions below.

### Conditions (if CONDITIONAL)
1. **Commit both tests** — `TestMachine_UnrecognisedState_ReturnsError` and `TestMachine_AgentFailed_FromIdle` must exist in committed history on the branch before squash-merge. Uncommitted working-tree code is not part of a branch.
2. **Add wrapping assertion to `TestMachine_UnrecognisedState_ReturnsError`** — The test at `machine_test.go:138-141` currently checks only `err == nil`. Add both: (a) `errors.Unwrap(err) != nil` to assert structural wrapping (tests the `%w` at `machine.go:48`), and (b) `strings.Contains(err.Error(), "bogus_state")` to assert the seeded invalid state surfaces in the error context. This satisfies the second council's condition at `council2:42`. (~4 lines added)
3. **Add `StateStore` interface godoc documenting the `SetAgentState`/`GetAgentFields` cross-method gap** — At minimum, add a comment to `store.go:33-35` noting that `SetAgentState` creates a partial agent record (state field only) and that `GetAgentFields` requires a prior `SetAgentFields` call to populate numeric fields. This prevents F2 developers from hitting the `ParseInt("")` crash at `redis.go:137` without warning. (~3-5 lines of comment)

### Suggested Fixes

#### Bug Fixes (always in-PR)
No bugs identified. All six first-council fixes are correctly implemented.

#### In-PR Improvements
- **Strengthen wrapping assertion** — `internal/agent/machine_test.go:138-141` — Add `errors.Unwrap(err) != nil` and `strings.Contains(err.Error(), "bogus_state")` after the existing `err == nil` check. (~4 lines)
- **Commit the two tests** — `internal/agent/machine_test.go` — Stage and commit the 43 lines of uncommitted test code (with the wrapping assertion added).
- **Add StateStore interface godoc** — `internal/state/store.go:33-35` — Document that `SetAgentState` creates a partial record (state field only) and that calling `GetAgentFields` on an agent created only via `SetAgentState` will fail on RedisStore due to missing numeric fields. (~3-5 lines of comment)

#### PR Description Amendments
- Note that `EventAgentFailed` from `StateIdle` produces a self-loop snapshot (`PreviousState == State`). This is intentional — callers may use this for operational telemetry or choose to filter no-op transitions before publishing domain events. (Carried forward from second council.)
- Remove or correct any reference to a commit message that does not exist in git history.

#### New Issues (future features/enhancements only — confirm with human before creating)
- **MockStore/RedisStore conformance test for cross-method interactions** — When F2 introduces `GetAgentFields` usage, add a conformance test verifying `SetAgentState` → `GetAgentFields` behavior is consistent across implementations. Consider either making `RedisStore.GetAgentFields` tolerate missing numeric fields (default to 0) or making `MockStore.SetAgentState` not populate `AgentFields`. — Feature
---
