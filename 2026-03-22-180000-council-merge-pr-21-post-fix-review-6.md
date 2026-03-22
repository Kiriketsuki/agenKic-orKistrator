---
## Adversarial Council — Merge PR #21 (feat: E2E lifecycle tests) — Council 6 Post-Fix Review

> Convened: 2026-03-22T18:00:00Z | Advocates: 2 | Critics: 2 | Rounds: 3/4

### Motion

PR #21 is ready to merge into epic/1-implement-go-orchestrator-core.

This is Council 6 — a post-fix review. Council 5 issued FOR with no conditions but recommended 4 in-PR improvements. Commit d1ad995 claims to address all 4:
1. Remove Scenario 3's QueueLength assertion (lifecycle_test.go:271-276) — QueueLength is cycle-unstable.
2. Add baseBackoff dependency comment to Scenario 12 — the 500ms pollAgentState timeout depends on the default 1s baseBackoff.
3. findIdleAgent RLock/IO decoupling (supervisor.go) — snapshot cooldown/circuitOpen maps under RLock, release before per-agent store I/O.
4. Silent task re-enqueue error handling (supervisor.go:249,256) — replace `_ = EnqueueTask` with `log.Printf` on failure.

---

### Advocate Positions

**ADVOCATE-1**: All four fixes are delivered correctly. Fix 1 replaces the cycle-unstable QueueLength assertion with a structural state-machine invariant (lifecycle_test.go:270-277). Fix 2 documents the baseBackoff dependency accurately (lifecycle_test.go:749-750, citing restart.go:73). Fix 3 snapshots both maps under RLock and releases before I/O (supervisor.go:283-292). Fix 4 covers both re-enqueue paths with actionable log messages (supervisor.go:250-252, 259-261). The snapshot race introduced by Fix 3 is real and acknowledged, but is bounded (one task per race occurrence, conservative circuit timing), and the "no clear in-PR fix" argument supports filing it as a tracked issue rather than blocking this PR.

**ADVOCATE-2**: All four fixes are delivered. Fix 3 is the most operationally significant: before the fix, findIdleAgent held RLock across all store I/O, blocking crashAgent's sv.mu.Lock acquisition. The snapshot approach resolves that contention. ADVOCATE-2 initially argued the snapshot race was impossible because EventAgentFailed transitions to a FAILED state — this premise was factually wrong (transition.go:26-27 confirms EventAgentFailed always returns StateIdle from any source state). ADVOCATE-2 conceded the race in Round 2. In Round 3, ADVOCATE-2 confirmed the proposed two-phase re-check closes the primary race window and accepted CONDITIONAL FOR as the appropriate disposition. log.Printf is adequate for re-enqueue failure given no retry infrastructure exists in the current architecture.

---

### Critic Positions

**CRITIC-1**: All four Council 5 fixes are correctly implemented on the surface. Fix 3 introduces a circuit-breaker bypass race: findIdleAgent snapshots circuitOpen/agentCooldown under RLock, releases, then calls store.GetAgentState. In the window between RUnlock (supervisor.go:292) and GetAgentState (supervisor.go:296), crashAgent can complete fully — applying EventAgentFailed (supervisor.go:189, which always transitions to StateIdle per transition.go:26-27), setting circuitOpen[agentID] = true (supervisor.go:214) — yet findIdleAgent's stale snapshot passes the agent through, resulting in a task assignment to a circuit-open agent. This race was not present before Fix 3 (old code held RLock through all I/O, fully serializing against crashAgent's sv.mu.Lock at supervisor.go:185). CRITIC-1 conceded Defects B (priority-0 re-enqueue predates d1ad995), C (Scenarios 8/9 intentional layering is documented at lifecycle_test.go:546-547), and D (comment was the exact ask; programmatic guard requires a new exported method). Sole blocking objection: the snapshot race, for which CRITIC-1 proposed the exact 8-line fix. CRITIC-1 accepts CONDITIONAL FOR once the fix is applied.

**CRITIC-2**: Same snapshot race finding as CRITIC-1, with the most detailed trace: per-agent mutex provides temporal ordering but not re-validation — applyEvent (supervisor.go:342-352) and machine.ApplyEvent contain no check against sv.circuitOpen or sv.agentCooldown (confirmed by tracing supervisor.go:342-351 and machine.go). CRITIC-2 raised the RegisterAgent lock-during-I/O inconsistency as an OBJECTION (supervisor.go:76-77 holds write lock across store.SetAgentFields at supervisor.go:84-89); this was ruled out of scope (pre-existing pattern, below Critical Discovery threshold). CRITIC-2 conceded Defect 3 (completeAgent no production callsite) when challenged: lifecycle_test.go:5-8 explicitly acknowledges gRPC-bypassed scenarios, and CLAUDE.md documents the research/spike phase. CRITIC-2 accepts CONDITIONAL as the appropriate recommendation.

---

### Key Conflicts

- **Snapshot race disposition** — Advocates said "bounded tradeoff, follow-up issue is sufficient"; Critics said "in-scope correctness regression introduced by d1ad995, must fix in-PR per Fix Triage Protocol." Resolved: the race passes both scope tests (relevance: directly about Fix 3's correctness; pre-existence: would not exist without d1ad995). Fix Triage Protocol applies — in-scope bugs are fixed in-PR. Additionally, a concrete minimal fix was proposed (CRITIC-1, 8 lines) that does not require architectural changes or revert Fix 3's latency improvement. ADVOCATE-1's "no clear in-PR fix" claim did not hold against the concrete proposal.

- **EventAgentFailed state machine fact** — ADVOCATE-2 initially argued the race was impossible because EventAgentFailed transitions to FAILED state. CRITIC-1 cited transition.go:26-27 (unambiguous: `if event == EventAgentFailed { return StateIdle, nil }`) proving the agent is IDLE post-crash. ADVOCATE-2 conceded. Resolved: circuitOpen=true + state=IDLE is the confirmed normal post-crash steady state; findIdleAgent's IDLE filter does NOT exclude circuit-open agents.

- **Priority loss on re-enqueue (CRITIC-1 Defect B)** — CRITIC-1 raised EnqueueTask(ctx, taskID, 0) hardcoded priority at supervisor.go:250,259. ARBITER and ADVOCATE-1 challenged: priority=0 predates d1ad995; Fix 4 was scoped to error handling only. Resolved: CRITIC-1 conceded, filed as new GitHub issue.

- **Scenarios 8/9 test coverage (CRITIC-1 Gap C / CRITIC-2 Defect 2)** — Critics argued Scenarios 8/9 bypass the integrated crash path (ApplyEventForTest + manual policy.RecordCrash). ADVOCATE-1 and ADVOCATE-2 countered with Scenarios 10 (lifecycle_test.go:628), 11 (lifecycle_test.go:687), and 12 (lifecycle_test.go:736), all of which use CrashAgentForTest and cover the full integrated supervisor-policy path. Resolved: CRITIC-1 conceded; the design is intentional (lifecycle_test.go:546-547 documents the layering).

- **completeAgent no production callsite (CRITIC-2 Defect 3)** — Resolved: ARBITER scope challenge accepted by CRITIC-2. lifecycle_test.go:5-8 explicitly acknowledges the gap; CLAUDE.md documents research/spike phase. Below Critical Discovery threshold. Filed as new GitHub issue.

- **RegisterAgent inconsistency (CRITIC-2 OBJECTION)** — ARBITER ruling: out of scope. Pre-existing pattern, not introduced by d1ad995, below Critical Discovery threshold. Dropped as blocking concern; eligible for new GitHub issue.

---

### Concessions

- **ADVOCATE-2** conceded that EventAgentFailed always transitions to StateIdle (transition.go:26-27), not a FAILED state; the snapshot race is real and introduced by Fix 3.
- **CRITIC-1** conceded Defect B (priority-0 pre-existing), Gap C (Scenarios 8/9 intentional layering), and Gap D (programmatic guard was not in scope for Council 5's ask).
- **CRITIC-2** conceded Defect 3 (completeAgent no production callsite is an acknowledged architectural gap, not a d1ad995 regression).
- **ADVOCATE-1** conceded (Round 3) that crash-count compounding is real: when the race fires, RecordCrash increments for the spurious post-race crash, causing the circuit to open at N-1 real failures instead of N.

---

### Arbiter Recommendation

**CONDITIONAL**

All four Council 5 improvements are correctly implemented by d1ad995. Fixes 1, 2, and 4 are unconditionally correct. Fix 3 solves the sv.mu lock-contention problem as Council 5 specified, but introduces a snapshot race: because findIdleAgent releases sv.mu.RLock before calling store.GetAgentState, a concurrent crashAgent can complete between the snapshot and the live state query, causing a circuit-open or cooling-down agent to pass findIdleAgent's filters and receive a task assignment. All four debate agents agreed this race is real, in-scope (introduced by d1ad995, passes both relevance and pre-existence tests), and that a minimal fix exists that preserves Fix 3's core benefit. Under the Fix Triage Protocol, this in-scope correctness regression must be addressed in-PR. The E2E test suite itself is valid, and the CONDITIONAL targets production code only.

---

### Conditions

Add the following block to `findIdleAgent` in `internal/supervisor/supervisor.go`, after line 308 (the closing `}` of the cooldownSnap check) and before `return agentID, true`:

```go
// Re-verify under fresh RLock: crashAgent may have updated circuit/cooldown
// after the snapshot was taken (outer race window before sentinel pre-population).
sv.mu.RLock()
recheckCircuit := sv.circuitOpen[agentID]
recheckExp, recheckCooling := sv.agentCooldown[agentID]
sv.mu.RUnlock()
if recheckCircuit || (recheckCooling && now.Before(recheckExp)) {
    continue
}
```

This fix:
- Takes a fresh RLock only for agents that passed all snapshot-based filters AND were confirmed IDLE by GetAgentState. No lock is held during store I/O (Fix 3's core benefit is preserved).
- Closes the outer race window — the span of per-agent store I/O during which crashAgent can complete and commit circuitOpen=true or a final cooldown.
- Requires no new exported API, no architectural changes, no revert of Fix 3.
- Uses the existing `now` variable (supervisor.go:294) for cooldown expiry to match the semantics of the existing snapshot check at supervisor.go:306-308. (Note: CRITIC-1's proposed form used `_, inCooldown` which checks map key existence only, not expiry — the above corrects this to `recheckExp, recheckCooling` with an explicit `now.Before(recheckExp)` guard.)

---

### Suggested Fixes

#### Bug Fixes (always in-PR)

- **supervisor.go — findIdleAgent fresh RLock re-check**: insert the 8-line block above after supervisor.go:308 and before `return agentID, true`. This is the sole blocking condition.

#### In-PR Improvements (scoped, non-bug)

- **lifecycle_test.go:749** — Fix off-by-one citation: change `restart.go:73` to `restart.go:72`. `baseBackoff: 1 * time.Second` is at restart.go:72; line 73 is `maxBackoff: 30 * time.Second`. Both advocates independently identified this. One character change.

#### PR Description Amendments

None beyond the line citation correction above.

#### New Issues (future features/enhancements only — confirm with human before creating)

- **Priority loss on re-enqueue** (`supervisor.go:250,259`): `EnqueueTask(ctx, taskID, 0)` hardcodes priority 0 on both re-enqueue paths. A task submitted with priority 1.0 that is temporarily dequeued is silently demoted. This is pre-existing behavior predating d1ad995 and outside Fix 4's mandate. Should be tracked for when a real priority queue backend (Redis Sorted Sets per spec) is wired in.

- **completeAgent no production callsite**: `completeAgent` (supervisor.go:320) is only reachable via `CompleteAgentForTest`. The gRPC handler for the REPORTING → IDLE transition does not exist yet. Acknowledged in lifecycle_test.go:5-8 ("full gRPC lifecycle coverage for those paths will follow once agent-side gRPC clients are implemented"). Track as a future implementation task.

- **Task-in-ASSIGNED-limbo on agent crash**: `crashAgent` (supervisor.go:176-223) applies EventAgentFailed and updates maps, but does not call EnqueueTask for any task that was ASSIGNED to the crashing agent. A task assigned to an agent that then crashes (before completing work) enters ASSIGNED state with no active agent and is never re-enqueued. This pre-existing mechanism is not introduced by d1ad995 but CRITIC-1 identified it as the worst-case consequence of the snapshot race. Track for future crash-recovery work.

- **RegisterAgent sv.mu held during store I/O** (`supervisor.go:76-77`, `84-89`): RegisterAgent holds the write lock across SetAgentFields, inconsistent with Fix 3's snapshot-then-release pattern. Pre-existing, below Critical Discovery threshold. Track for pattern consistency cleanup.
