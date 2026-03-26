---
## Adversarial Council — Merge PR #21 (feat: E2E lifecycle tests) — Council 5 Post-Fix Review

> Convened: 2026-03-22T17:50:00Z | Advocates: 2 | Critics: 2 | Rounds: 2/4

### Motion
Merge PR #21 (feat: E2E lifecycle tests) into epic/1-implement-go-orchestrator-core. This is a post-fix review (Council 5). Council 4 issued FOR with 1 in-PR improvement (spurious-crash regression test), 1 PR description amendment, and 2 new issues (sentinel teardown coordination, crashAgent explicit error handling). Commit 427c7a9 claims to address all 3 in-PR items: (1) per-agent mutex serialization for crashAgent/completeAgent, (2) crashAgent explicit error handling, (3) spurious-crash regression test (TestE2E_SpuriousCrashGuard).

---

### Advocate Positions

**ADVOCATE-1**: All three Council 4 in-PR conditions are satisfied with cited evidence. Per-agent mutex serialization is operative at `supervisor.go:180-181` and `310-311`, preventing deadlock by bypassing `applyEvent` and enforcing consistent `agentMu -> sv.mu` lock ordering. crashAgent's explicit error handling at `supervisor.go:188-194` correctly cleans the cooldown sentinel on store failure and returns without recording a phantom crash. The spurious-crash regression test at `lifecycle_test.go:727-764` provides non-vacuous behavioral proof via `pollAgentState` at line 764. The TOCTOU guard's binary boundary condition is covered in both directions: Scenario 12 verifies the guard fires on IDLE-agent crashes, and Scenario 10 implicitly verifies it does NOT fire on WORKING-agent crashes. Both critics conceded all merge-blocking objections.

**ADVOCATE-2**: Confirms all three conditions met with matching citations. Per-agent mutex serialization gates concurrent access to `sv.machine.ApplyEvent` — not semantic dead code, not a half-implementation, symmetrically applied to both crash and complete paths. Error handling at `supervisor.go:188-194` is complete: sentinel deleted, early return prevents false policy accounting. Scenario 12's behavioral assertion distinguishes correct from incorrect behavior — if `RecordCrash` had fired, the resulting 1-second cooldown would prevent task assignment within the 500ms window. CRITIC-2's independent analysis confirmed TOCTOU bidirectional coverage via Scenarios 10 and 12.

---

### Critic Positions

**CRITIC-1**: Opened with three defects and one TOCTOU objection. Conceded all as merge blockers through the debate: Defect 1 (Scenarios 8/9 mislabeled) — naming convention is deliberately distinct (`TestCrashCycle_*` vs `TestE2E_*`), and wiring coverage exists via Scenarios 10/11. Defect 2 (findIdleAgent RLock across store I/O) — starvation model requires continuous RLock renewal, but `taskAssignLoop` releases completely between 20ms ticks; not a current blocker with in-memory MockStore. TOCTOU one-direction — CRITIC-2's analysis confirmed Scenario 10 implicitly covers the inverse direction. Sole remaining concern: Scenario 12's 500ms behavioral proxy depends on the implicit relationship that 500ms < 1s default `baseBackoff` — a documentation/maintenance concern, not a correctness defect.

**CRITIC-2**: Opened with four objections. Conceded all as merge blockers: Objection 1 (QueueLength flake) — probability corrected from ~20% to ~0.005% with in-memory MockStore; accepted as follow-up issue. Objection 2 (policy wiring untested) — Scenario 10's `CrashAgentForTest` exercises the full `crashAgent -> RecordCrash` path. Objection 3 (silent task loss at `supervisor.go:249,256`) — pre-existing pattern, not introduced by Council 4 fixes. Objection 4 (ErrSupervisorStopped misleading) — `RegisterAgent` invariant at `supervisor.go:74-93` ensures all agents in `agentMu` map are registered; unregistered-agent nil-mutex path is unreachable through the public API. Final statement: "I have no remaining merge-blocking objections."

---

### Key Conflicts

- **Scenario 3 QueueLength consistency** — CRITIC-2 cited `lifecycle_test.go:271-276` using `QueueLength` while lines 650-651 in the same file call it cycle-unstable. ADVOCATE-1 conceded the inconsistency exists but argued the assertion is redundant with the stable `GetAgentState` assertion at lines 279-284. — **Resolved: inconsistency acknowledged, not a correctness bug; in-PR improvement.**

- **Scenarios 8/9 integration depth** — Both critics argued these bypass `crashAgent` and duplicate `restart_test.go`. Advocates demonstrated the `TestCrashCycle_*` naming convention is deliberately distinct from `TestE2E_*`, and the wiring between `crashAgent` and the policy is tested through Scenarios 10/11. — **Resolved: labeling is accurate, coverage exists elsewhere.**

- **TOCTOU guard bidirectional coverage** — CRITIC-1 argued only the false-positive direction (IDLE agent → guard fires) was tested. CRITIC-2 independently analyzed and confirmed Scenario 10 implicitly covers the inverse: `CrashAgentForTest` on a WORKING agent at `lifecycle_test.go:636` would fail the 40ms negative assertion if the guard incorrectly fired (no cooldown set). — **Resolved: implicit coverage exists in both directions.**

- **findIdleAgent RLock/IO coupling** — CRITIC-1 cited `supervisor.go:276-296` holding `sv.mu.RLock()` across per-agent store I/O. CRITIC-2 correctly identified that starvation requires continuous RLock renewal, which the 20ms tick interval does not produce. — **Resolved: architectural concern for future Redis migration, not a current blocker.**

- **Silent task loss** — CRITIC-2 cited `supervisor.go:249,256` discarding `EnqueueTask` errors. Advocates demonstrated this is pre-existing code not introduced by this PR. — **Resolved: pre-existing tech debt, not in scope.**

---

### Concessions

- **ADVOCATE-1** conceded Scenario 3's QueueLength assertion at `lifecycle_test.go:271-276` is inconsistent with the PR's own documentation of QueueLength instability.
- **ADVOCATE-1** conceded line 751's state assertion in Scenario 12 is vacuous (IDLE self-loop always passes) — the behavioral proof at line 764 is the real regression detector.
- **CRITIC-1** conceded Defect 1 (Scenarios 8/9 mislabeled) — naming convention is accurate.
- **CRITIC-1** conceded Defect 2 (RLock/IO) — not a current merge blocker.
- **CRITIC-1** conceded TOCTOU one-direction objection — Scenario 10 provides implicit inverse coverage.
- **CRITIC-2** conceded Objection 1 (QueueLength flake) as merge blocker — practical risk is ~0.005%.
- **CRITIC-2** conceded Objection 2 (policy wiring untested) — Scenario 10 covers it.
- **CRITIC-2** conceded Objection 3 (silent task loss) — pre-existing.
- **CRITIC-2** conceded Objection 4 (ErrSupervisorStopped) — invariant prevents the problematic case.

---

### Arbiter Recommendation

**FOR**

All three Council 4 in-PR conditions are substantively satisfied. Per-agent mutex serialization at `supervisor.go:176-181` and `305-311` is operative and symmetrically applied with consistent lock ordering. crashAgent's explicit error handling at `supervisor.go:188-194` provides complete detection-and-response (sentinel cleanup + early return), closing the pre-existing tech debt identified by Council 4. The spurious-crash regression test at `lifecycle_test.go:727-764` provides non-vacuous behavioral proof that the TOCTOU guard correctly fires on IDLE-agent crashes, while Scenario 10 implicitly validates the inverse direction. Both critics entered with substantive, well-cited objections and conceded all merge blockers through rigorous exchange — CRITIC-2 explicitly stated "I have no remaining merge-blocking objections" and CRITIC-1 reduced all defects to "should-improve" documentation concerns. The PR's 12 E2E scenarios pass under `go test -race` in 1.6 seconds.

---

### Conditions
None.

---

### Suggested Fixes

#### Bug Fixes (always in-PR)
None — all correctness concerns either satisfied or pre-existing, pre-dating this PR.

#### In-PR Improvements (scoped, non-bug)

- **Replace Scenario 3's QueueLength assertion with GetAgentState** — `e2e/lifecycle_test.go:271-276` — The `QueueLength` assertion is inconsistent with the PR's own documentation at lines 650-651 which calls it cycle-unstable. Replace with `GetAgentState != ASSIGNED` check (matching the pattern at lines 279-284 which is already present). The existing `GetAgentState == WORKING` assertion at lines 279-284 already validates the critical invariant; the QueueLength check is redundant. Either remove it or replace with `GetAgentState` for consistency. Both advocates acknowledged this inconsistency.

- **Add comment to Scenario 12 documenting the 500ms/1s baseBackoff dependency** — `e2e/lifecycle_test.go:755-764` — The behavioral proof relies on the implicit relationship that `pollAgentState`'s 500ms timeout < the default 1s `baseBackoff` (from `restart.go:73`). Add a comment explaining this dependency so future maintainers know to adjust the timeout if `baseBackoff` is changed. Raised by CRITIC-1, accepted by advocates as documentation improvement.

#### PR Description Amendments
- Update the PR description's council history section to note that Council 5 issued FOR with no conditions, and that the per-agent mutex serialization, explicit error handling, and spurious-crash test from Council 4 are all verified.

#### New Issues (future features/enhancements only — confirm with human before creating)

- **findIdleAgent RLock/IO decoupling for Redis migration** — When `MockStore` is replaced with Redis (network I/O), `findIdleAgent`'s pattern of holding `sv.mu.RLock()` across per-agent `GetAgentState` calls (`supervisor.go:276-296`) should be revisited. Currently benign with nanosecond in-memory operations, but network round-trips could introduce lock contention with `crashAgent`'s `sv.mu.Lock()` calls. Architectural concern only — no current bug. — Feature

- **Silent task re-enqueue error handling** — `tryAssignTask` at `supervisor.go:249` and `256` discards `EnqueueTask` errors with `_ =`. If the store fails to re-enqueue, the dequeued task is silently lost. Pre-existing pattern, not introduced by this PR. Should be addressed when the store moves from in-memory to durable (Redis). — Feature
---
