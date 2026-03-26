---
## Adversarial Council — Merge PR #21 (feat: E2E lifecycle tests) — Council 4 Post-Fix Review

> Convened: 2026-03-22T17:21:39Z | Advocates: 2 | Critics: 2 | Rounds: 2/4

### Motion
Merge PR #21 (feat: E2E lifecycle tests) into epic/1-implement-go-orchestrator-core. This is a post-fix review (Council 4): Council 3 issued CONDITIONAL with 2 required conditions and 2 new issues. Commit 87aa58f claims to satisfy ALL 4 items:
- Condition 1: TOCTOU guard — crashAgent must capture applyEvent snapshot and check snap.PreviousState == StateIdle before calling RecordCrash
- Condition 2: QueueLength → GetAgentState — Scenarios 10 and 11 must replace QueueLength negative assertions with GetAgentState == IDLE checks
- Issue A: Cooldown atomicity — crashAgent race window between applyEvent returning and sv.mu.Lock() writing cooldown
- Issue B: RecordSuccess wiring — sv.policy.RecordSuccess never called; agentConsecutive accumulates without bound

---

### Advocate Positions

**ADVOCATE-1**: All four Council 3 items are structurally satisfied in commit 87aa58f. Condition 1's TOCTOU guard at `supervisor.go:184` is mechanically correct — `machine.go:62` populates `PreviousState` as the pre-transition state, and `transition.go:26-28` confirms EventAgentFailed from StateIdle yields PreviousState=StateIdle, causing the guard to fire and return before RecordCrash. Condition 2's `GetAgentState` assertions at `lifecycle_test.go:652-658` and `lifecycle_test.go:712-718` are cycle-stable, non-vacuous, and correctly replace the QueueLength pattern. Issue A's sentinel at `supervisor.go:175-177` is load-bearing (read by `findIdleAgent:supervisor.go:275` under sv.mu.RLock()) and closes the applyEvent-return→cooldown-write window. Issue B's `RecordSuccess` at `supervisor.go:292` has direct operative effects on `agentConsecutive` (drives computeBackoff, `restart.go:116-117`) and `agentCrashes` (drives circuit breaker, `restart.go:108`), and its behavioral consequence is verified by `TestRestartPolicy_SuccessResetsConsecutive` at `restart_test.go:81-109`. The guard not-firing on `applyEvent` error is the correct conservative outcome — when the store fails, pre-transition state is unknown, and RecordCrash is the safe default. The completeAgent sentinel-deletion race is architecturally real but pre-existing and unreachable in production (no production caller; `CompleteAgentForTest` is gated by `//go:build testenv` at `export_e2e.go:1`).

**ADVOCATE-2**: All four conditions are satisfied with no correctness failures in the current codebase. Error-path behavior at `supervisor.go:179` (`snap, _`) is unchanged from pre-fix commit `2a100bc:supervisor.go:169` where `_, _` explicitly discarded both values — confirmed by git history. The guard not-firing on error is the correct conservative outcome. Confirmed concession: withdrew the per-agent mutex argument as protection against the completeAgent sentinel-deletion race — the mutex serializes individual `applyEvent` calls but does not prevent the three-step interleaving (complete→IDLE, assign→ASSIGNED, crash→IDLE with PreviousState=StateAssigned). The valid defense for the race remains: no production caller for `completeAgent`.

---

### Critic Positions

**CRITIC-1**: The sentinel-deletion race (completeAgent at `supervisor.go:295` deleting crashAgent's sentinel) is architecturally reachable — the per-agent mutex does not block the three-step interleaving, as sentinel reads/writes happen outside `agentMu` scope, under `sv.mu` only. However, this race is pre-existing (existed before the sentinel in `2a100bc`) and is narrowed by the fix, not introduced by it. The TOCTOU guard's PreviousState==StateIdle branch at `supervisor.go:184-188` has no dedicated E2E test — if the condition were inverted in a future commit, no test would catch it. Both are quality observations; neither is a Council 3 condition failure. Surviving position: in-PR improvement for a spurious-crash regression test; pre-existing race tracked as future design constraint. Conceded: Defect 3 (Scenarios 8/9 deliberately scoped with integrated path exercised by Scenarios 10/11), Defect 4 (RecordSuccess behavioral test — unit test `TestRestartPolicy_SuccessResetsConsecutive` exists at `restart_test.go:81-109`), and reclassified Defect 2 (guard test) from "coverage requirement" to "in-PR improvement."

> Note: CRITIC-1's final summary was not received before recommendation was written. Round 2 positions are used as the record of their final stance.

**CRITIC-2**: Opened with three failures; formally conceded all three in the final summary. Failure 1 (error-path as hard merge blocker): pre-existing behavior unchanged by commit — "the guard's operative scope is the success path, which is the exact path where the TOCTOU scenario occurs." Failure 2 (sentinel teardown race): unreachable in production — completeAgent has no production caller. Failure 3 (RecordSuccess behavioral test): incorrect claim — ARBITER verified the test exists at `restart_test.go:81-109`; misapplied Pitfall #1. Final statement: "No hard merge blockers remain in commit 87aa58f." Surviving observations: two future-PR design constraints when `completeAgent` gains a production caller — explicit error handling at `supervisor.go:179` and sentinel teardown coordination at `supervisor.go:295`.

---

### Key Conflicts

- **Error path (`snap, _ :=` at `supervisor.go:179`)** — CRITIC-2 initially called this a "hard merge blocker" (Pitfall 2, Half-Implementation). ADVOCATES produced git-verified evidence: `2a100bc:supervisor.go:169` used `_, _` (both values discarded), identical pre-transition behavior. ADVOCATE-2 further argued not-firing on error is the _correct_ conservative outcome (pre-transition state unknown on store failure). CRITIC-2 conceded: "pre-existing behavior unchanged by this commit." — **Resolved: pre-existing, not a blocker.**

- **completeAgent sentinel-deletion race** — CRITIC-2 Failure 2 described a three-step interleaving (complete→IDLE, assign→ASSIGNED, crash→IDLE with PreviousState=StateAssigned) where TOCTOU guard misses. ADVOCATE-2 initially (incorrectly) cited per-agent mutex as protection; CRITIC-1 correctly demonstrated the mutex does not block the three-step interleaving (sentinel operations happen outside `agentMu`). ADVOCATE-2 conceded the mutex argument. All parties agreed: race is architecturally real, confirmed reachable in theory by CRITIC-1's locking analysis, but pre-existing (same race class existed before sentinel in `2a100bc`) and unreachable in production (no production caller for `completeAgent`). — **Resolved: pre-existing race, future-scoped design gap.**

- **RecordSuccess behavioral test** — CRITIC-1 (Defect 4) and CRITIC-2 (Failure 3) claimed "no test verifies the crash-success-crash backoff reset." ADVOCATE-1 cited `TestRestartPolicy_SuccessResetsConsecutive` at `restart_test.go:81-109`. ARBITER independently verified: test records 3 crashes, calls `RecordSuccess`, records another crash, asserts `Backoff == 1s` — exactly the sequence critics claimed was absent. Both critics conceded; CRITIC-2 acknowledged misapplication of Pitfall #1. — **Resolved: critics' claim was factually incorrect; test exists.**

- **TOCTOU guard test coverage** — CRITIC-1 Defect 2 / CRITIC-2 Defect 2: PreviousState==StateIdle branch at `supervisor.go:184-188` has no dedicated E2E test. Both critics reclassified as in-PR improvement after acknowledging Condition 1 specified a structural requirement (add the guard before RecordCrash), not a coverage requirement. — **Resolved: quality gap, not a condition failure. In-PR improvement.**

---

### Concessions

- **ADVOCATE-2** conceded the per-agent mutex argument on the completeAgent race: mutex serializes individual `applyEvent` calls but does not prevent the three-step interleaving. Valid defense is no-production-caller, not the mutex.
- **CRITIC-1** withdrew Defect 3 (Scenarios 8/9 by design, integrated path in Scenarios 10/11), withdrew Defect 4 (unit test `TestRestartPolicy_SuccessResetsConsecutive` exists), reclassified Defect 2 from "condition failure" to "in-PR improvement."
- **CRITIC-2** conceded all three failures: Failure 1 (pre-existing behavior), Failure 2 (no production caller), Failure 3 (test exists). Withdrew Pitfall #1 (Semantic Dead Code) application to RecordSuccess — "untested ≠ dead code."

---

### Arbiter Recommendation

**FOR**

Commit 87aa58f satisfies all four Council 3 items. Condition 1's TOCTOU guard is mechanically correct and structurally ordered — `supervisor.go:184` checks `snap.PreviousState == agent.StateIdle` before `RecordCrash` at line 191, grounded by `machine.go:62`'s `PreviousState: current` assignment and `transition.go:26-28`'s unconditional StateIdle return for EventAgentFailed. Condition 2's `GetAgentState` assertions at `lifecycle_test.go:652-658` and `712-718` are non-vacuous and cycle-stable. Issue A's 24h sentinel at `supervisor.go:175-177` is load-bearing (read by `findIdleAgent:supervisor.go:275`) and eliminates the specific applyEvent-return→cooldown-write race window that Issue A identified. Issue B's `RecordSuccess` at `supervisor.go:292` is wired with operative effects confirmed, and its behavioral consequence (counter reset to base backoff) is proven by the existing unit test `TestRestartPolicy_SuccessResetsConsecutive` at `restart_test.go:81-109`.

The two main critic objections that survived Round 1 did not survive scrutiny. The error-path behavior is pre-existing and git-verified (both values were `_, _` in `2a100bc`); ADVOCATE-2's framing that not-firing the guard on error is correct conservative behavior (state unknown on store failure) is sound. The completeAgent sentinel-deletion race is architecturally real — CRITIC-1's locking analysis correctly demonstrated the per-agent mutex provides no cross-call protection — but it is pre-existing, narrowed by the sentinel fix, and currently unreachable in production because `completeAgent` has no production call site. Both critics ultimately found no hard merge blockers.

---

### Conditions
None.

---

### Suggested Fixes

#### Bug Fixes (always in-PR)
None — all correctness concerns either satisfied or pre-existing, pre-dating this commit.

#### In-PR Improvements (scoped, non-bug)
- **Add spurious-crash regression test for TOCTOU guard** — `e2e/lifecycle_test.go` — Register an agent (IDLE state), immediately call `sv.CrashAgentForTest(ctx, agentID)` without first advancing to WORKING, then assert: (a) agent state remains IDLE, (b) no cooldown entry set (`agentCooldown` map should not contain agentID), (c) policy crash count not incremented. This exercises the `supervisor.go:184-188` early-return branch (PreviousState==StateIdle) and creates a regression guard if the guard condition is ever inverted. Agreed by both critics as the appropriate response to their coverage gap observation (CRITIC-1 Defect 2, CRITIC-2 Defect 2). — `supervisor.go:184-188`

#### PR Description Amendments
- Document that `completeAgent` currently has no production call site (awaiting agent-side gRPC implementation, as noted at `lifecycle_test.go:7-8`). When the agent-side gRPC completion handler is implemented, the sentinel mechanism at `supervisor.go:175-177` must be reviewed against `completeAgent:supervisor.go:295`'s cooldown deletion to ensure the pre-populated sentinel cannot be torn down mid-flight. Reference the sentinel teardown race analysis from this council.

#### New Issues (future features only — confirm with human)
- **Agent-side gRPC wiring: sentinel teardown coordination** — When `completeAgent` gains a production caller, the race between `crashAgent`'s sentinel write (`supervisor.go:175-177`) and `completeAgent`'s cooldown deletion (`supervisor.go:295`) must be addressed before `completeAgent` is wired to a live gRPC handler. Fix options: (a) hold `agentMu` across the full `crashAgent` body (not just the `applyEvent` call) to block concurrent completeAgent + assign-loop interleaving, or (b) add a per-agent "crash-in-progress" flag that `completeAgent` checks before deleting the sentinel. This was architecturally confirmed as reachable by CRITIC-1's locking analysis in Council 4. — Task

- **crashAgent explicit error handling (pre-existing tech debt)** — `supervisor.go:179`: `snap, _ := sv.applyEvent(...)` discards the error. On store failure, `snap.PreviousState = ""`, the TOCTOU guard cannot fire, and `RecordCrash` is called for a phantom crash. Fix: check `err`, clean the sentinel (`delete(sv.agentCooldown, agentID)`), and return early without calling `RecordCrash`. Consistent with `completeAgent:supervisor.go:287-290` which already guards on `applyEvent` error. This behavior is pre-existing from `2a100bc`; it is not introduced by this PR. Low priority. — Task
