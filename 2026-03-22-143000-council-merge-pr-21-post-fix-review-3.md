---
## Adversarial Council — Merge PR #21: E2E Lifecycle Tests (Post-Fix Review 3)

> Convened: 2026-03-22 | Advocates: 2 | Critics: 2 | Rounds: 2/4

### Motion

Merge PR #21 (feat: E2E lifecycle tests) into epic/1-implement-go-orchestrator-core. This is a post-fix review (Council 3): Council 2 issued CONDITIONAL with 2 required conditions (rename Scenarios 8/9 to drop TestE2E_ prefix) and in-PR improvements (HeartbeatStaleDetection comment, export_e2e.go doc precision, PR description amendment). Commit 2a100bc claims to satisfy ALL prior conditions AND goes further by wiring supervisor↔policy integration (crashAgent method, cooldown/circuit-breaker enforcement in findIdleAgent) and adding two new genuine E2E tests (TestE2E_CooldownEnforcement, TestE2E_CircuitBreakerBlocksAssignment).

---

### Advocate Positions

**ADVOCATE-1 / ADVOCATE-1-2**: All Council 2 conditions met and undisputed. The supervisor↔policy integration is genuine and functional: `crashAgent` (supervisor.go:165-183) correctly calls `policy.RecordCrash()` and populates `agentCooldown`/`circuitOpen`; `findIdleAgent` (supervisor.go:249-254) enforces both. The two new E2E tests are non-vacuous — their assertions fail when the respective enforcement is absent. Conceded under ARBITER CLARIFY: the race window between supervisor.go:169 and :173 is real (`sv.mu` is not held during `policy.RecordCrash`); the TOCTOU on healthy agents is real (`transition.go:26-28` confirms EventAgentFailed from IDLE returns `StateIdle, nil`); `QueueLength` assertions are subject to the dequeue-requeue transient-zero window. Upgraded Issue A from follow-up to pre-merge condition after independently discovering that `sv.circuitOpen` has no clearance path (set-true at supervisor.go:177 with zero resets in the codebase), making spurious RecordCrash accumulation result in permanent agent exclusion. Maintained Issues B and C as New Issues on grounds of lock-order restructuring complexity and absent gRPC success-completion handler respectively. Final position: **FOR, CONDITIONAL** on Issue A (snapshot guard) and QueueLength structural fix.

**ADVOCATE-2**: All Council 2 conditions met and undisputed. The integration advances the epic branch materially — both new E2E tests are non-vacuous and verify real supervisor enforcement paths that prior councils listed as future work. Confirmed under ARBITER CLARIFY: `sv.mu` is NOT held anywhere between supervisor.go:169 (applyEvent returning) and supervisor.go:173 (sv.mu.Lock() acquiring); `AgentSnapshot.PreviousState` (machine.go:60-65) carries the pre-transition state and can gate RecordCrash correctly. Verified via `ipc/handlers.go` that no EventOutputDelivered gRPC handler exists in the current implementation — the only production path resulting in StateIdle is through `crashAgent`, which reduces Issue A's immediate severity but does not remove the need for the guard (the code is written to be extended). Conceded QueueLength transient-zero applies on every poll cycle independently of sleep duration; endorsed GetAgentState structural fix. Maintained Issue B as in-PR disclosure only; maintained Issue C as New Issue (no call site without new gRPC handler). Final position: **FOR, CONDITIONAL** on Issue A and QueueLength structural fix.

---

### Critic Positions

**CRITIC-1**: Council 2 naming conditions met — conceded immediately. Primary objections targeted new code in commit 2a100bc. Issue A confirmed via ARBITER-verified citation (`transition.go:26-28`): EventAgentFailed from any state including IDLE returns `StateIdle, nil`, making the TOCTOU path silent and undetectable without inspecting the snapshot. Strengthened Issue A by tracing `sv.circuitOpen`: set-true at supervisor.go:177, never reset anywhere in the codebase — spurious RecordCrash accumulation past crashThreshold opens the circuit permanently with no in-process recovery path. Accepted ARBITER CLARIFY on the QueueLength dequeue-requeue window — conceded the pattern applies equally to Scenario 3 and narrowed it to in-PR improvement. Accepted ARBITER CLARIFY on Issue B (cooldown atomicity) — conceded lock-order restructuring complexity makes it a New Issue rather than a pre-merge condition. Narrowed Issue C from CONDITIONAL to New Issue after ADVOCATE-2 confirmed no EventOutputDelivered handler exists; correct RecordSuccess wiring requires new production code. Final position: **FOR, CONDITIONAL** on Issue A (PreviousState guard) and QueueLength structural replacement.

**CRITIC-2**: Council 2 naming conditions met. Four objections raised against new integration code: (1) Race window between applyEvent returning IDLE and sv.mu.Lock() writing cooldown entry — findIdleAgent can observe agent as IDLE with no cooldown during this window; (2) No RecordSuccess call anywhere in supervisor.go — agentConsecutive accumulates without bound, crash-window pruning protects circuit breaker but not backoff accuracy; (3) Scenarios 8/9 manually call RecordCrash without populating sv.agentCooldown/circuitOpen — creates supervisor state the integrated crashAgent path would never produce; (4) Scenario 10 uses 40ms sleep within 80ms cooldown — load-sensitive CI flakiness. Objection 3 rebutted: Scenarios 8/9 are explicitly scoped to policy arithmetic; each test uses its own testStack instance; TestCrashCycle_ naming correctly signals non-enforcement scope. Objection 4 accepted as in-PR improvement (structural assertion preferred). Objections 1 and 2 maintained through Round 2; consistent with convergence on Issue B as New Issue (lock-order) and Issue C as New Issue (no call site). Final position: **FOR, CONDITIONAL** (positions consistent with Issue A and QueueLength as pre-merge conditions).

---

### Key Conflicts

- **Council 2 conditions (rename Scenarios 8/9)** — all four agents: met, uncontested. `TestCrashCycle_PolicyBackoff` at e2e/lifecycle_test.go:493 and `TestCrashCycle_PolicyCircuitBreaker` at e2e/lifecycle_test.go:560 are accepted. **Resolved unanimously.**

- **Issue A: TOCTOU spurious RecordCrash** — Advocates initially argued bounded/not blocking; ARBITER CLARIFY issued; ADVOCATE-2 confirmed PreviousState gate via machine.go:60-65; ADVOCATE-1-2 upgraded to blocking after discovering sv.circuitOpen has no clearance path (permanent agent exclusion); CRITIC-1 confirmed transition.go:26-28 citation. **Resolved in critics' favour: CONDITIONAL.**

- **Issue B: crashAgent cooldown atomicity** — CRITIC-2 raised as blocking; CRITIC-1 initially maintained as CONDITIONAL; all parties then agreed the fix requires lock-order restructuring incompatible with getAgentMutex's sv.mu.RLock() acquisition; CRITIC-1 explicitly narrowed to New Issue. **Resolved: New Issue.**

- **Issue C: RecordSuccess not called** — CRITIC-1 and CRITIC-2 raised; ADVOCATE-2 verified no EventOutputDelivered gRPC handler exists; CRITIC-1 conceded that the correct call site requires new production code (new handler or supervisor completion method). **Resolved: New Issue.**

- **QueueLength transient-zero (Scenarios 10/11)** — CRITIC-1 raised; ARBITER CLARIFY confirmed the same pattern exists in Scenario 3 (accepted by Council 2); CRITIC-1 accepted same classification; ADVOCATE-2 conceded transient-zero is per-poll-cycle independent of sleep duration; structural GetAgentState fix endorsed by all. **Resolved: in-PR improvement, classified as CONDITIONAL per Fix Triage Protocol (in-PR fix, scoped and non-bug but directly improves test reliability).**

- **Scenarios 8/9 state inconsistency (CRITIC-2 Obj 3)** — ADVOCATE-1-2 rebutted: each test has its own testStack; TestCrashCycle_ naming signals non-enforcement scope; comments cross-reference enforcement tests. CRITIC-2 did not contest further. **Resolved in advocates' favour: not a defect.**

- **Scenario 10 timing fragility (CRITIC-2 Obj 4)** — ADVOCATE-1-2 partially conceded CI flakiness risk; structural GetAgentState fix also addresses this. **Resolved: captured in QueueLength condition.**

---

### Concessions

- **ADVOCATE-1-2** conceded: "No race condition" claim was incorrect — sv.mu is NOT held between supervisor.go:169 and :173 — to ARBITER CLARIFY / CRITIC-2
- **ADVOCATE-1-2** conceded: Issue A (TOCTOU spurious RecordCrash) is a pre-merge condition — upgraded when independently verifying sv.circuitOpen has zero clearance paths — to CRITIC-1
- **ADVOCATE-1-2** conceded: QueueLength transient-zero applies per-poll-cycle; GetAgentState structural fix endorsed — to CRITIC-1 / CRITIC-2
- **ADVOCATE-2** conceded: sv.mu not held between supervisor.go:169 and :173 — confirmed to ARBITER CLARIFY
- **ADVOCATE-2** conceded: Issue A gate exists (PreviousState in AgentSnapshot) but is unused — confirmed via machine.go:60-65 — to CRITIC-1 / ARBITER
- **ADVOCATE-2** conceded: QueueLength transient-zero is per-poll-cycle, not sleep-duration-dependent — to CRITIC-1 / CRITIC-2
- **CRITIC-1** conceded: Issue B (cooldown atomicity) is a New Issue due to lock-order restructuring complexity — to ADVOCATE-1-2 / ADVOCATE-2
- **CRITIC-1** conceded: Issue C (RecordSuccess) is a New Issue — no EventOutputDelivered call site exists without new gRPC handler — to ADVOCATE-2
- **CRITIC-1** conceded: QueueLength transient-zero applies equally to Scenario 3 (accepted by Council 2); narrowed to in-PR improvement — to ARBITER CLARIFY
- **CRITIC-1** confirmed: ADVOCATE-1-2's sv.circuitOpen no-clearance-path discovery strengthens Issue A beyond CRITIC-1's own Round 1 framing — on record

---

### Arbiter Recommendation

**CONDITIONAL**

The two Council 2 conditions are met without dispute: `TestCrashCycle_PolicyBackoff` and `TestCrashCycle_PolicyCircuitBreaker` appear at `e2e/lifecycle_test.go:493` and `:560` respectively; all Council 2 in-PR improvements are applied. The supervisor↔policy integration added by commit 2a100bc is substantive — `crashAgent` correctly invokes `policy.RecordCrash()` and populates enforcement state; `findIdleAgent` correctly enforces cooldown and circuit-breaker gates; the two new E2E tests verify real behavioral constraints that would fail without the enforcement code. However, two targeted defects in the new integration code require correction before merge. First, `crashAgent` at `supervisor.go:169` discards the `applyEvent` return value, silently allowing `policy.RecordCrash()` to fire when the agent was already IDLE — a TOCTOU that can permanently open `sv.circuitOpen` (which has no clearance path in the codebase) for a healthy agent that completed work concurrently. ARBITER independently verified via `transition.go:26-28` that `EventAgentFailed` from `StateIdle` returns `(StateIdle, nil)`, making the no-op undetectable without the snapshot. Second, the negative-case assertions in Scenarios 10 and 11 use `QueueLength` checks (lifecycle_test.go:657 and :722) that are subject to the per-poll-cycle dequeue-requeue transient-zero window in `tryAssignTask` — a structural flaw the same council that mandated structural assertions for Scenario 3 cannot leave unaddressed in new tests. Both fixes are fewer than 5 lines total and do not alter the architecture.

---

### Conditions (CONDITIONAL — all must be applied before merge)

1. At `internal/supervisor/supervisor.go:169`: capture the return value of `sv.applyEvent(ctx, agentID, agent.EventAgentFailed)` into a named variable. Before calling `sv.policy.RecordCrash(agentID)` at line 171, add the guard:
   ```go
   if snap.PreviousState == agent.StateIdle {
       return
   }
   ```
   Rationale: `transition.go:26-28` defines EventAgentFailed as always returning `StateIdle, nil` from any prior state, including IDLE itself. Without this guard, a healthy agent that legitimately completes work (transitioning to IDLE) between `checkHeartbeats` reading a stale non-IDLE state and `crashAgent` firing receives a spurious RecordCrash call. Spurious crash accumulation can open `sv.circuitOpen[agentID]`, which has no clearance path in the codebase, permanently excluding a healthy agent from task assignment for the supervisor process lifetime.

2. At `e2e/lifecycle_test.go:653-659` (TestE2E_CooldownEnforcement) and `e2e/lifecycle_test.go:718-724` (TestE2E_CircuitBreakerBlocksAssignment): replace the `QueueLength` negative-case assertion with a `GetAgentState` check. Specifically, replace the `qLen == 1` assertions with:
   ```go
   stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
   if err != nil {
       t.Fatalf("GetAgentState: %v", err)
   }
   if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
       t.Fatalf("expected agent to remain IDLE (cooldown/circuit-open), got %v", stateResp.State)
   }
   ```
   Rationale: `tryAssignTask` (supervisor.go:201-210) dequeues the task before deciding whether to re-enqueue, creating a transient window on every 10ms poll cycle where `QueueLength` returns 0. The `GetAgentState == IDLE` assertion is cycle-stable: an agent blocked by cooldown or open circuit cannot receive `EventTaskAssigned`, so its state cannot advance to ASSIGNED regardless of when the check fires.

---

### Suggested Fixes

Categorised by triage priority. Bugs are always fixed in-PR — never punted.

#### Bug Fixes (always in-PR, regardless of original scope)

- **Spurious RecordCrash via TOCTOU — permanent circuitOpen risk** — `internal/supervisor/supervisor.go:169` — `crashAgent` discards the snapshot from `sv.applyEvent` (`_, _ = sv.applyEvent(...)`). `EventAgentFailed` from StateIdle is a valid no-op transition (transition.go:26-28), so there is no error signal when the agent was already IDLE. `policy.RecordCrash` fires unconditionally, and if enough spurious crashes accumulate within crashWindow, `sv.circuitOpen[agentID] = true` is written (supervisor.go:177) with no reset path anywhere in the codebase. Fix: capture the snapshot and gate RecordCrash on `snap.PreviousState != agent.StateIdle`. This is Condition 1 above. _(Elevated to Condition per Fix Triage Protocol: bug in new production code.)_

#### In-PR Improvements (scoped, non-bug)

- **QueueLength transient-zero in Scenarios 10 and 11** — `e2e/lifecycle_test.go:657` and `:722` — `tryAssignTask` (supervisor.go:201-210) calls DequeueTask before conditionally re-enqueueing, creating a transient window where `QueueLength` returns 0 on every poll cycle. Replace with `GetAgentState == IDLE` structural assertion. This is Condition 2 above. _(Elevated to Condition: same structural flaw Council 1 required fixing in the analogous Scenario 3 negative assertion; consistency demands the same standard for new scenarios.)_

- **crashAgent cooldown atomicity disclosure** — `internal/supervisor/supervisor.go:165` — The window between `applyEvent` returning at line 169 (agent IDLE in store, per-agent mutex released) and `sv.mu.Lock()` acquiring at line 173 (cooldown written) allows `findIdleAgent` to observe an IDLE agent with no cooldown entry and assign it a task before cooldown is enforced. Add a doc comment to `crashAgent` acknowledging this window: the fix (pre-populating agentCooldown before applyEvent) requires lock-order restructuring incompatible with getAgentMutex's sv.mu.RLock() acquisition and is tracked as a follow-up task. _(Classified as in-PR improvement rather than Condition because: consequence is one leaked task assignment per occurrence; fix requires non-trivial lock-order restructuring; CRITIC-1 explicitly narrowed to New Issue.)_

#### PR Description Amendments

- Remove or update the "Supervisor↔Policy Integration Gap" note added by Council 2: the gap described (supervisor.go never calling policy.RecordCrash) is now resolved. Add a new note: "Supervisor↔Policy integration is implemented via crashAgent (supervisor.go:165-183) and findIdleAgent enforcement (supervisor.go:249-254). Two follow-up tasks track known limitations: cooldown atomicity (crashAgent race window) and RecordSuccess wiring (success-path integration)."

#### New Issues (future features/enhancements only — confirm with human before creating)

- **crashAgent cooldown atomicity** — `internal/supervisor/supervisor.go:169-173` — The window between `applyEvent` returning (agent IDLE in store) and `sv.mu.Lock()` writing `agentCooldown` allows `findIdleAgent` to return the crashed agent before cooldown is enforced. Consequence: one task assignment per occurrence bypasses the cooldown (the cooldown then applies on the next IDLE cycle). Fix requires pre-populating `agentCooldown` before calling `applyEvent`, which conflicts with `getAgentMutex`'s `sv.mu.RLock()` acquisition at supervisor.go:274 — a lock-order restructuring. — Task

- **RecordSuccess wiring** — `internal/supervisor/supervisor.go` — `sv.policy.RecordSuccess(agentID)` (restart.go:127) is never called. `agentConsecutive` (restart.go:66) accumulates without bound, degrading backoff accuracy for agents with historical crashes. The circuit-breaker path is correctly protected by crash-window pruning (restart.go:96-105). Correct wiring requires a supervisor method called when EventOutputDelivered transitions an agent to IDLE — a call site that does not currently exist in the gRPC handler (ipc/handlers.go has no EventOutputDelivered handler). Tracked invariant: `TestRestartPolicy_SuccessResetsConsecutive` at `restart_test.go:81-108` verifies the policy contract; the supervisor integration must fulfill it when the completion handler is added. — Task

---
