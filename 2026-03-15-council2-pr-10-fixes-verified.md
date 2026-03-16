---
## Adversarial Council — PR #10 Fix Verification

> Convened: 2026-03-15 | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion
All 6 council-mandated fixes have been correctly applied to PR #10 and the branch is ready to squash-merge into task/1-task-implement-go-orchestrator-core.

### Advocate Positions
**ADVOCATE-1**: All six fixes are textually present and semantically correct, verified with line-by-line citations. Fix 1 (ParseInt errors) is handled at `redis.go:137-143`. Fix 2 (TxPipeline) is applied at `redis.go:89, 113, 155`. Fix 3 (type assertion) uses `.(string)` with `ok` check at `redis.go:220-223`. Fix 4 (concurrency comment) documents the single-writer contract at `machine.go:32-34`. Fix 5 (event-publishing comment) documents caller responsibility at `machine.go:36-39`. Fix 6 (snapshot enrichment) adds `PreviousState` and `Event` fields at `snapshot.go:5-10`, populated at `machine.go:60-64`, and tested at `machine_test.go:47-52, 83-88`. The prior council's conditions specified code changes and documentation — none mandated new tests. The motion's scope is fix verification, not comprehensive test coverage expansion. Conceded late in debate that `ParseAgentState` error path test is a legitimate merge-readiness gap.

### Critic Positions
**CRITIC-1**: Four objections raised. Conceded Objections A (TxPipeline testing) and B (concurrency comment) to the letter of the prior council's conditions. Maintained Objection C: fixes 5, 6, and `transition.go:26-28` interact to produce a self-loop snapshot (`{PreviousState: Idle, State: Idle, Event: AgentFailed}`) that Fix 5's documentation unconditionally instructs callers to publish — creating semantically questionable domain events. Argued Fix 5's comment is defective because it assigns publishing responsibility without guard language for self-loops. Maintained Objection D: `ParseAgentState` error path at `machine.go:46-48` is new code (verified via git history: `new file mode 100644` in commit `de7237f`) with zero test coverage, violating the project's 80% coverage mandate and the "ready to squash-merge" clause of the motion.

**CRITIC-2**: Six test coverage gaps identified across all fixes. Conceded TxPipeline atomicity testing, concurrency comment, and Fix 1's correctness (Fix 1 made RedisStore more correct by surfacing silent errors). Converged with CRITIC-1 on three points: (1) Fix 5's comment is defective re: self-loops; (2) ParseAgentState error path untested; (3) MockStore/RedisStore fidelity gap where `SetAgentState` then `GetAgentFields` succeeds on MockStore but would fail on RedisStore due to `ParseInt("")` — a divergence widened by Fix 1's error surfacing. Proposed 3-4 targeted tests (~40-60 lines total) to close all gaps. Later conceded CRITIC-1's sharper framing that Fix 5's primary issue is the comment itself, not a missing test.

### Questioner Findings
QUESTIONER did not submit probes during the debate despite two prompts from the arbiter. All claim substantiation was driven by direct advocate-critic exchanges and arbiter file verification.

### Key Conflicts
- **Fix 5 comment completeness** — CRITIC-1 said the comment unconditionally instructs callers to publish (creating spurious events for self-loop transitions); ADVOCATE-1 said "responsible for" assigns ownership, not a mandate, and self-loop events are useful operational telemetry — **unresolved, arbiter rules below**
- **ParseAgentState test coverage** — CRITIC-1/CRITIC-2 said untested new code violates merge-readiness; ADVOCATE-1 initially said "pre-existing, scope creep" but conceded after git history disproved the "pre-existing" claim — **resolved: all agree test is needed**
- **MockStore/RedisStore fidelity gap** — CRITIC-2 said `SetAgentState` → `GetAgentFields` diverges between implementations; ADVOCATE-1 said the gap is real but latent (no current code path exercises the cross-API sequence) and is an F2 concern — **partially resolved: gap acknowledged, merge-blocking status disputed**
- **Motion scope ("fixes applied" vs "ready to merge")** — ADVOCATE-1 said the motion is about fix verification per the prior council's literal conditions; both critics said the motion's "ready to squash-merge" clause imposes merge-readiness standards beyond textual presence — **resolved: the motion does contain both claims, both must hold**

### Concessions
- **CRITIC-1** conceded Objections A (TxPipeline testing) and B (concurrency comment) to the letter of the prior council's conditions to **ADVOCATE-1**
- **CRITIC-2** conceded TxPipeline atomicity testing, concurrency comment sufficiency, and Fix 1 correctness to **ADVOCATE-1**
- **ADVOCATE-1** conceded that `ParseAgentState` error path (`machine.go:46-48`) is new code requiring a test, and that the MockStore/RedisStore fidelity gap is "worth documenting" to **CRITIC-1** and **CRITIC-2**
- **CRITIC-2** conceded to **CRITIC-1** that the EventAgentFailed self-loop issue is primarily a comment defect (Fix 5), not a missing test

### Arbiter Recommendation
**CONDITIONAL**

All six mandated fixes are present and substantively correct. The code changes (fixes 1-3) are idiomatic, the documentation (fixes 4-5) is precise, and the API enrichment (fix 6) is implemented and tested. The debate surfaced one universally-agreed merge-readiness gap (untested `ParseAgentState` error path in new code) and one substantive but narrower concern (the interaction of Fix 5's documentation with self-loop transitions).

On the Fix 5 comment dispute: the arbiter finds the comment is **not defective**. "Callers are responsible for publishing domain events" is standard Go documentation assigning ownership of a concern, not an unconditional directive. ADVOCATE-1's operational telemetry argument — that a self-loop `AgentFailed` event for an idle agent is useful diagnostic information, not a semantic contradiction — has merit. However, both critics independently identified this edge case, and a test documenting the self-loop as intentional behavior is warranted to prevent future developers from treating it as a bug.

This confirms the prior council's findings: the F1 foundation is architecturally sound. The prior council's CONDITIONAL recommendation has been substantially met. The two conditions below are small, focused additions (~10-15 lines total) that close the remaining gap between "fixes present" and "branch merge-ready."

### Conditions (if CONDITIONAL)
1. Add a test in `machine_test.go` for the `ParseAgentState` error path at `machine.go:46-48` — seed an invalid state string (e.g., `"bogus_state"`) via `MockStore.SetAgentState` and assert a wrapped error is returned. (~5 lines)
2. Add a test in `machine_test.go` for `EventAgentFailed` from `StateIdle` — verify the self-loop produces `AgentSnapshot{PreviousState: StateIdle, State: StateIdle, Event: EventAgentFailed}`. This documents the behavior as intentional and prevents future misinterpretation. (~5-10 lines)

### Suggested Fixes

#### Bug Fixes (always in-PR)
No bugs identified. All six mandated fixes are correctly implemented.

#### In-PR Improvements
- **Add `ParseAgentState` error path test** — `internal/agent/machine_test.go` (new test function) — New code in `machine.go:46-48` has an explicit error branch with zero coverage. Universally agreed by all debate participants as a merge-readiness requirement.
- **Add `EventAgentFailed` from `StateIdle` test** — `internal/agent/machine_test.go` (new test function) — Documents that the self-loop transition `Idle → AgentFailed → Idle` is intentional design, not accidental. Both critics independently identified this edge case; the test serves as living documentation.

#### PR Description Amendments
- Note that `EventAgentFailed` from `StateIdle` produces a self-loop snapshot (`PreviousState == State`). This is intentional — callers may use this for operational telemetry or choose to filter no-op transitions before publishing domain events.

#### New Issues (future only — confirm with human before creating)
- **Document MockStore/RedisStore fidelity gap for cross-API sequences** — `SetAgentState` followed by `GetAgentFields` succeeds on MockStore but would produce a `ParseInt("")` error on RedisStore. The gap is currently latent (no F1 code path exercises this sequence), but F2 supervisor code should be aware. Add a conformance test or interface contract documentation when F2 introduces `GetAgentFields` usage. — Feature
---
