---
## Adversarial Council — Merge PR #21: E2E Lifecycle Tests (Post-Fix Review)

> Convened: 2026-03-21 | Advocates: 2 | Critics: 2 | Rounds: 3/4

### Motion

Merge PR #21 (feat: E2E lifecycle tests) into epic/1-implement-go-orchestrator-core. This is a post-fix review: Council 1 issued CONDITIONAL with 3 required conditions; commit 32251fe claims to satisfy them.

---

### Advocate Positions

**ADVOCATE-1**: All three prior conditions are correctly implemented and undisputed. `supervisor.go:237` doc comment correctly heads `getAgentMutex`; `export_e2e.go:11-13` carries its own accurate doc block. `lifecycle_test.go:95` — `cancel()` fires first. `lifecycle_test.go:158` — `t.Logf` present before retry. Conceded under direct ARBITER questioning: removing all `ApplyEventForTest` calls from Scenarios 8/9 leaves every `RecordCrash()` assertion passing. Proposed inline comments per scenario as minimum disclosure fix; narrowed from comment-only when rename scope was challenged. FOR, CONDITIONAL on two inline comments and rename.

**ADVOCATE-2**: All three conditions met, no dispute. Conceded fully under ARBITER questioning: the `RecordCrash()` assertions at `lifecycle_test.go:510, 529, 578, 595` are determined by call count alone and pass regardless of the `ApplyEventForTest` calls. Stack context contributes nothing to policy assertions because `supervisor.go` has zero calls to any policy method. Conceded `export_e2e.go:11` doc comment ("external test packages") overstates isolation. Proposed rename (drop `TestE2E_` prefix) + one NOTE comment in Scenario 5 as minimum CONDITIONAL conditions. FOR, CONDITIONAL on naming and comment fixes.

---

### Critic Positions

**CRITIC-1**: Three original conditions correctly fixed — no remaining defect there. Primary objection: `TestE2E_RestartWithBackoff` and `TestE2E_CircuitBreakerTrips` carry `TestE2E_` function names that appear in CI output, coverage reports, and PR history asserting spec coverage they do not provide. `supervisor.go` has zero policy method calls (ARBITER-verified); `taskAssignLoop` at `supervisor.go:162-174` contains no backoff delay mechanism; `RecordCrash()` return values are never read by the system. Narrowed from "remove or relocate" to "accurate naming" after conceding that crash-cycle state machine assertions have genuine E2E value. Accepts rename in-place (e.g., `TestCrashCycle_PolicyBackoff`) or relocation to `restart_test.go`; rejects inline-comment-only as insufficient because function names are the primary signal in all external artifacts.

**CRITIC-2**: Confirmed alignment with CRITIC-1 on Scenarios 8/9. `restart_test.go:19-78` already provides more thorough coverage of the same `RecordCrash()` arithmetic (fake clock, configurable threshold, crash-window expiry) — the E2E scenarios are a strict redundant subset with no incremental integration value. Conceded: build-tag scope (`Makefile:18`) is intentional (`supervisor_test.go:102` confirms), not a regression. Conceded: `time.Sleep(100ms)` + `QueueLength` assertion is not vacuous (`qLen==1` is the correct expected value in both zero-tick and multi-tick scenarios). Maintained Point 3 as a CONDITIONAL item (not standalone blocker): `TestE2E_HeartbeatStaleDetection` tests state reset but the spec clause "triggers the restart strategy" (`go-orchestrator-core-spec.md:101`) is silently untested because `checkHeartbeats` at `supervisor.go:157` never calls the policy; a one-sentence NOTE comment makes the gap visible.

---

### Key Conflicts

- **Scenarios 8/9 function naming vs. inline comment** — ADVOCATE-1 argued inline comments inside the test body are sufficient disclosure. CRITIC-1 argued function names are visible in every external artifact (CI output, coverage reports, dashboards) while body comments are not, making rename the minimum required fix. ADVOCATE-2 and CRITIC-2 both accepted rename as sufficient. **Resolved in critics' favour**: rename required; inline comment alone is insufficient.

- **Remedy scope (rename vs. relocation)** — CRITIC-1 initially proposed relocation to `restart_test.go`; ADVOCATE-2 proposed rename in-place. Both CRITIC-1 and CRITIC-2 accepted either option as satisfying the concern. **Resolved**: either rename or relocation is acceptable; the ARBITER imposes rename as the minimum condition because relocation of a full test stack setup would require test infrastructure changes beyond the scope of a naming fix.

- **HeartbeatStaleDetection comment** — ADVOCATE-1 accepted as "a genuine improvement"; ADVOCATE-2 proposed as a CONDITIONAL item; CRITIC-2 included in the CONDITIONAL batch; CRITIC-1 treated as corroborating evidence, not a primary condition. **Resolved**: all parties accept a one-sentence NOTE comment; classified as in-PR improvement (not a standalone blocking condition) given Council 1 did not raise it.

- **Build-tag scope (`Makefile:18` passing `-tags=testenv` to `./internal/...`)** — CRITIC-2 initially argued this was a broken isolation guarantee. CRITIC-1 identified it as intentional: `supervisor_test.go:102` calls `ApplyEventForTest` directly. CRITIC-2 withdrew. **Resolved**: intentional, not a defect.

- **`time.Sleep(100ms)` vacuity** — CRITIC-2 argued zero-tick scenario could pass vacuously. ADVOCATE-2 rebutted: the task is enqueued before the sleep, so `qLen==1` is the correct expected value whether zero or five supervisor ticks occurred. CRITIC-2 conceded. **Resolved**: not vacuous.

---

### Concessions

- **ADVOCATE-1** conceded: removing all `ApplyEventForTest` calls from `TestE2E_RestartWithBackoff` and `TestE2E_CircuitBreakerTrips` leaves every `RecordCrash()` assertion passing — to CRITIC-1 / ARBITER CLARIFY
- **ADVOCATE-2** conceded: `RecordCrash()` assertions at `lifecycle_test.go:510, 529, 578, 595` are determined by call count alone, independent of the supervisor stack context — to CRITIC-1 / ARBITER CLARIFY
- **ADVOCATE-2** conceded: `export_e2e.go:11` doc comment ("external test packages") is imprecise given actual scope — to CRITIC-2
- **ADVOCATE-2** conceded: Scenarios 8 and 9 carry Pitfall 8 violations (vacuous assertions with respect to stack context) — to CRITIC-1 and CRITIC-2
- **CRITIC-1** conceded: crash-cycle state machine assertions in Scenarios 8/9 have genuine E2E value (`EventTaskAssigned` from WORKING would fail without prior `EventAgentFailed` reset) — to ADVOCATE-1
- **CRITIC-1** narrowed: from "remove or relocate entirely" to "accurate naming required"; relocation is one valid option but not the only one — to ADVOCATE-2
- **CRITIC-2** conceded: build-tag scope in `Makefile:18` is intentional (`supervisor_test.go:102` confirms) — to CRITIC-1
- **CRITIC-2** conceded: `time.Sleep(100ms)` + `QueueLength` assertion is non-vacuous — to ADVOCATE-2
- **CRITIC-2** downgraded: Point 3 (`TestE2E_HeartbeatStaleDetection`) from standalone blocking objection to CONDITIONAL in-PR improvement — to ADVOCATE-2

---

### Arbiter Recommendation

**CONDITIONAL**

The three conditions from Council 1 are correctly implemented — ARBITER-verified and confirmed as undisputed by all four agents. The remediation went beyond the minimum: `ApplyEventForTest` was moved to `export_e2e.go` with `//go:build testenv` compiler enforcement (the "New Issue" from Council 1 implemented as an in-PR change), `grpcServer.Serve` errors are now captured on a buffered channel, structural `QueueLength` assertion replaced the pure timing-based negative case, and scenario numbering was corrected. Seven of nine E2E scenarios are substantive and correctly scoped.

However, the remediation commit also added Scenarios 8 and 9 (`TestE2E_RestartWithBackoff`, `TestE2E_CircuitBreakerTrips`) before the prerequisite wiring exists. ARBITER independently verified: `supervisor.go` has zero calls to any policy method (the `policy` field at line 23 is stored but never invoked in `checkHeartbeats`, `tryAssignTask`, or `applyEvent`); `taskAssignLoop` at lines 162-174 contains no backoff delay mechanism. Under direct ARBITER questioning, ADVOCATE-2 confirmed that removing all eleven `ApplyEventForTest` calls leaves every `RecordCrash()` assertion passing, satisfying Pitfall 8 (Vacuous Test Assertions). `restart_test.go:19-78` already covers the same arithmetic more thoroughly with a configurable fake clock and crash-window expiry. The `TestE2E_` function names appear in CI output, coverage reports, and PR history implying spec-level supervisor enforcement that is not implemented. Prior Council 1 explicitly deferred these scenarios as New Issues requiring supervisor-policy wiring. The conditions below are minimal naming and disclosure fixes requiring no implementation work.

---

### Conditions (CONDITIONAL — all must be applied before merge)

1. Rename `TestE2E_RestartWithBackoff` at `e2e/lifecycle_test.go:486` to remove the `TestE2E_` prefix. The renamed function must not imply supervisor backoff enforcement (acceptable names: `TestCrashCycle_PolicyBackoff`, `TestRestartPolicy_BackoffArithmetic_ViaStack`; alternatively, move the function to `internal/supervisor/restart_test.go` with a `TestRestartPolicy_*` prefix). Rationale: the `TestE2E_` prefix asserts in CI output and coverage reports that supervisor restart-with-backoff enforcement is E2E tested; `supervisor.go` never reads the `Backoff` value returned by `RecordCrash()`, so this spec behavior is untested.

2. Rename `TestE2E_CircuitBreakerTrips` at `e2e/lifecycle_test.go:553` with the same approach as Condition 1. Acceptable names: `TestCrashCycle_PolicyCircuitBreaker`, `TestRestartPolicy_CircuitBreakerTrips_ViaStack`, or relocation to `restart_test.go`.

---

### Suggested Fixes

#### Bug Fixes (always in-PR)

_(None — the three required conditions are correctly implemented. The naming defect above is classified as an in-PR improvement per Fix Triage Protocol; however, given the false spec-coverage signal it produces in all external artifacts, the ARBITER elevates it to a CONDITION.)_

#### In-PR Improvements (scoped, non-bug)

- **HeartbeatStaleDetection deferred-wiring disclosure** — `e2e/lifecycle_test.go` inside `TestE2E_HeartbeatStaleDetection` — Add one comment: `// NOTE: supervisor.go checkHeartbeats calls applyEvent(EventAgentFailed) but does not call policy.RecordCrash(). The "triggers the restart strategy" clause (go-orchestrator-core-spec.md:101) is deferred pending supervisor↔policy wiring.` ADVOCATE-1 accepted this as "a genuine improvement"; ADVOCATE-2 included it as a CONDITIONAL item; CRITIC-2 included it in the CONDITIONAL batch. All four agents agree on its value; classified as in-PR improvement (not a standalone blocking condition) because Council 1 did not raise it and the cost is one sentence.

- **`export_e2e.go:11` doc precision** — Change "external test packages (e.g. e2e/)" to "test packages; prefer the public API in unit tests" to match the actual scope (both `e2e/` and `internal/supervisor/` under `-tags=testenv`). ADVOCATE-2 conceded the imprecision; zero implementation cost.

#### PR Description Amendments

- Add a **Supervisor↔Policy Integration Gap** note: `supervisor.go checkHeartbeats and taskAssignLoop do not yet call policy.RecordCrash() or enforce the computed Backoff delay. Scenarios 8 and 9 (renamed per council conditions) verify RestartPolicy arithmetic and crash-cycle state machine behaviour in the full stack context; they do not test supervisor-enforced backoff timing or operator alerting.`

#### New Issues (future features only)

- **Wire `checkHeartbeats` to call `sv.policy.RecordCrash()`** — `internal/supervisor/supervisor.go:157` — After `EventAgentFailed` is applied in the stale-heartbeat path, the supervisor should call `sv.policy.RecordCrash()` and use the returned `RestartDecision` to decide whether to schedule a delayed re-assignment or open the circuit. This is the prerequisite for Scenarios 8 and 9 to become genuine E2E integration tests. — Task

- **Implement backoff delay enforcement in `taskAssignLoop`** — `internal/supervisor/supervisor.go:162-174` — `taskAssignLoop` currently re-assigns immediately on the next tick after an agent returns to IDLE. A per-agent "ready after" timestamp (populated from `RestartDecision.Backoff`) is needed before the spec's backoff enforcement can be tested. — Task

- **Upgrade Scenarios 8 and 9 to full E2E integration tests** — Once the supervisor-policy wiring and backoff enforcement are implemented, upgrade the renamed scenarios to verify actual timing: confirm that the supervisor does not re-assign a task to a recovered agent until the backoff window expires, and that a circuit-open result prevents all re-assignment. — Feature
---
