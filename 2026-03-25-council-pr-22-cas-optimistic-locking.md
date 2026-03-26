---
## Adversarial Council — Merge PR #22: CompareAndSetAgentState

> Convened: 2026-03-25 | Advocates: 1 | Critics: 2 | Rounds: 4/4 | Motion type: CODE

### Motion
Merge PR #22 — CompareAndSetAgentState for optimistic locking — into epic/1-implement-go-orchestrator-core

### Advocate Positions
**ADVOCATE-1**: The CAS mechanism is correct and closes a real atomicity gap. The Lua script at `redis.go:98-115` provides server-side atomic compare-and-swap with no TOCTOU window on the write path. The `StateConflictError` type is inspectable and follows Go idiom. Conformance tests cover all three CAS code paths including a 10-goroutine concurrency race. The `racyStore` wrapper in `machine_test.go` proves the Machine correctly surfaces conflicts. Four in-PR fixes accepted as merge conditions: three from CRITIC-1 (Lua atomic return, `conflict.Actual` assertion, realistic `racyStore` injection) plus Issue B from CRITIC-2/CRITIC-1 (supervisor must explicitly handle `*StateConflictError` with documented task-fate decision). Issues A and C from CRITIC-2 were rebutted: Issue C is factually incorrect (idle-agent guard at `supervisor.go:150-152`), Issue A is a pre-existing CI pattern (recommended as follow-on task, not blocking).

### Critic Positions
**CRITIC-1**: The CAS write atomicity is sound, but the error reporting has a defect: `StateConflictError.Actual` is populated by a non-atomic `HGet` after the Lua script returns (`redis.go:107-114`), making it TOCTOU-poisoned. The Lua-returned value would be causally correct (exact state at conflict); the post-Lua `HGet` may reflect state after N additional transitions. The `store.go:53` interface contract documents `Actual` as actionable for retry decisions — the Redis implementation does not fulfil that contract. Additionally, `racyStore` injects an impossible state (`StateWorking` from `StateIdle`) and `TestMachine_ApplyEvent_CASConflict` never asserts `conflict.Actual`. These three issues are linked: the missing assertion with the unrealistic injection masks the TOCTOU defect. Does not oppose merge if all three fixes are applied in this PR.

**CRITIC-2**: Initially raised three objections (Redis CI gap, supervisor swallowing conflicts, self-loop CAS writes). All three were conceded after verification: Issue C was factually wrong (idle-agent guard exists at `supervisor.go:150-152`), Issue A is a pre-existing CI pattern unchanged by this PR, and Issue B's re-enqueue is semantically correct at the task-assignment call site. Supports conditional merge on CRITIC-1's three amendments.

### Questioner Findings
QUESTIONER did not submit probes during the debate. All claims were substantiated with `file:line` citations by the debating agents themselves. No claims were marked unsubstantiated.

### Key Conflicts
- **`Actual` field semantics** — ADVOCATE-1 initially argued `Actual` is "diagnostic metadata"; CRITIC-1 cited `store.go:53` documenting it as actionable contract. **Resolved**: ADVOCATE-1 conceded the internal contradiction and accepted that `Actual` is contract, not diagnostic.
- **Categorical staleness** — ADVOCATE-1 argued Lua fix is "also best-effort"; CRITIC-1 distinguished causal accuracy (Lua-returned) from indeterminate staleness (post-Lua `HGet`). **Resolved**: ADVOCATE-1 conceded the categorical distinction.
- **Supervisor error handling (Issue B)** — CRITIC-2 and CRITIC-1 argued supervisor must distinguish `*StateConflictError` from hard failures; ADVOCATE-1 initially argued re-enqueue is correct regardless. Critics conceded re-enqueue is semantically correct, but CRITIC-1 argued the mechanism and its consumer should ship together with explicit handling. **Resolved**: ADVOCATE-1 conceded in Round 4 that `supervisor.go:190-194` should add an explicit `errors.As(*StateConflictError)` branch with documented task-fate decision for both conflict and hard-failure cases.
- **Self-loop CAS (Issue C)** — CRITIC-2 claimed idle agents receive `EventAgentFailed`; ARBITER verified `supervisor.go:150-152` guards against this. **Resolved**: CRITIC-2 conceded factual error.
- **Redis CI coverage (Issue A)** — CRITIC-2 argued Lua paths are untested in CI; ADVOCATE-1 showed `//go:build integration` is pre-existing. **Resolved**: ARBITER dropped via pre-existence test.

### Concessions
- **ADVOCATE-1** conceded to **CRITIC-1**: `Actual` is an actionable contract (not diagnostic), the categorical staleness distinction is valid, and all three issues (Lua TOCTOU, missing assertion, unrealistic injection) are real defects requiring in-PR fixes.
- **ADVOCATE-1** conceded to **CRITIC-1/CRITIC-2** (Round 4): Issue B — supervisor must add explicit `errors.As(*StateConflictError)` branch with documented task-fate decision. Accepted as fourth pre-merge condition.
- **CRITIC-1** initially withdrew support for Issue B after verifying re-enqueue correctness, then reinstated it as a fourth condition after ADVOCATE-1's own concession in Round 4.
- **CRITIC-2** conceded to **ADVOCATE-1**: Issue C is factually incorrect, Issue A is pre-existing pattern, Issue B's re-enqueue is semantically correct (but supported CRITIC-1's framing that explicit handling should ship with the mechanism).

### Regression Lineage
No regression lineage — no prior fix involvement.

### Arbiter Recommendation
**CONDITIONAL**
The CAS mechanism is architecturally sound and closes a real distributed atomicity gap. The Lua script, typed error, conformance test suite, Machine integration, and `racyStore` conflict simulation all demonstrate correct design. Four issues require in-PR fixes before merge: three from CRITIC-1 (Lua TOCTOU on `Actual`, missing assertion, unrealistic injection) and one from CRITIC-2/CRITIC-1 (supervisor explicit error handling). All four were substantiated with citations and conceded by ADVOCATE-1. CRITIC-2's remaining objections were resolved — Issue C factually incorrect, Issue A pre-existing pattern. Issue A (Redis CI wiring) is recommended as a tracked follow-on task.

### Conditions (if CONDITIONAL)
All four must be applied in this PR before merge — not deferred to follow-ups:

1. **Lua script must return the conflict state atomically.** Modify `casScript` at `redis.go:98-115` to return `{0, current}` when the CAS fails (instead of bare `0`), and update the Go caller at `redis.go:117-134` to parse the conflict state from the Lua result table. This eliminates the TOCTOU on `StateConflictError.Actual` and fulfils the `store.go:53` interface contract.

2. **Add `conflict.Actual` assertion to `TestMachine_ApplyEvent_CASConflict`.** At `machine_test.go:152-165`, assert that `conflict.Actual` matches the state injected by the `racyStore`. This closes the test gap that currently masks the TOCTOU defect.

3. **Update `racyStore` injection to use `StateAssigned`.** At `machine_test.go:132`, change `raceTo: string(agent.StateWorking)` to `raceTo: string(agent.StateAssigned)`. `StateAssigned` is reachable from `StateIdle` via `EventTaskAssigned` and represents a realistic concurrent race scenario.

4. **Supervisor must explicitly handle `*StateConflictError`.** At `supervisor.go:190-194`, add an `errors.As(err, &conflict)` branch that distinguishes CAS conflicts from hard failures (Redis connectivity, context cancellation). Both branches must document the task-fate decision (re-enqueue for conflicts; re-enqueue for hard failures since `DequeueTask`/`ZPOPMIN` atomically removes). This ensures the typed error introduced by this PR has an explicit, documented consumer in the same changeset.

Additionally recommended as a tracked follow-on (not blocking):

5. **Wire Redis integration tests into CI.** File a GitHub issue to add `docker-compose` Redis to the CI test matrix with `-tags integration`. The Lua CAS script has no unit-test analogue in MockStore and is currently only exercised when a live Redis instance is available.

### Suggested Fixes

#### Bug Fixes
1. **Lua script TOCTOU on `Actual` field.**
   CITE: `internal/state/redis.go` L:98-115 (Lua script) and L:107-114 (Go caller)
   The Lua script returns bare `0` on conflict. The Go caller then issues a separate `HGet` to populate `StateConflictError.Actual`, which is non-atomic and may return a state from after additional transitions. Fix: return `{0, current}` from Lua; parse in Go.

#### In-PR Improvements
2. **Missing `conflict.Actual` assertion.**
   CITE: `internal/agent/machine_test.go` L:152-165
   `TestMachine_ApplyEvent_CASConflict` asserts `conflict.Expected` but not `conflict.Actual`. Add assertion: `conflict.Actual` must equal the `racyStore`'s injected state.

3. **Unrealistic `racyStore` state injection.**
   CITE: `internal/agent/machine_test.go` L:132
   `raceTo` is set to `StateWorking`, which is not reachable from `StateIdle` in one transition. Change to `StateAssigned` for a realistic concurrent race simulation.

4. **Supervisor undifferentiated error handling.**
   CITE: `internal/supervisor/supervisor.go` L:190-194
   The supervisor treats `*StateConflictError` identically to hard failures (Redis connectivity, context cancellation). Add an `errors.As(err, &conflict)` branch that documents the deliberate re-enqueue decision for CAS conflicts separately from the re-enqueue for hard failures (necessary because `ZPOPMIN` atomically removes tasks). The typed error and its consumer ship in the same PR — the consumer should demonstrate explicit handling.

#### PR Description Amendments
None required.

#### Critical Discoveries
None identified.

### Verification Results
| # | Finding | Citations | Verdict | Action |
|---|---------|-----------|---------|--------|
| 1 | Lua script TOCTOU on `Actual` field | `internal/state/redis.go` L:98-115, L:107-114 | UNVERIFIED | Tagged, retained |
| 2 | Missing `conflict.Actual` assertion | `internal/agent/machine_test.go` L:152-165 | UNVERIFIED | Tagged, retained |
| 3 | Unrealistic `racyStore` state injection | `internal/agent/machine_test.go` L:132 | UNVERIFIED | Tagged, retained |
| 4 | Supervisor undifferentiated error handling | `internal/supervisor/supervisor.go` L:190-194 | Not verified (added post-debate) | Retained |

Verification: 0 verified, 0 phantom, 3 unverified (retained for review), 1 not verified

**Note**: All 3 findings describe real behaviors confirmed by the verifier, but line citations from the debate are incorrect (agents cited line numbers from the base branch, not the feature branch). Correct locations:
- Finding 1: Lua script is at `redis.go:122-132`, case-0 branch with HGet at `redis.go:142-148`
- Finding 2: `TestMachine_ApplyEvent_CASConflict` starts at `machine_test.go:172`, `conflict.Expected` check at L:194
- Finding 3: `raceTo: string(agent.StateWorking)` is at `machine_test.go:182`
---
