## Adversarial Council — Merge PR #21 (feat: E2E lifecycle tests) — Council 11 Post-Fix Review

> Convened: 2026-03-23T19:15:00Z | Advocates: 1 | Critics: 1 | Rounds: 2/4 | Motion type: CODE

### Motion
PR #21 (feat: E2E lifecycle tests) is ready to merge into epic/1-implement-go-orchestrator-core. This is Council 11 — a post-fix review after Council 10 addressed completeAgent GetAgentFields log warning and Scenario 19 (commit 0862bc1). This council evaluates whether the PR is now ready to merge.

---

### Advocate Positions
**ADVOCATE-1**: Both Council 10 blocking conditions are fully satisfied.

- **Condition 1 — log warning**: `internal/supervisor/supervisor.go:452-460` adds an `else` branch that logs `"GetAgentFields failed for agent %s — CurrentTaskID not cleared, duplicate re-enqueue possible on next crash"`, mirroring the SetAgentFields failure log pattern at line 456. ✓
- **Condition 2 — Scenario 19**: `e2e/lifecycle_test.go:973-1042` implements `TestE2E_GetAgentFieldsFailureInCompleteAgent` with the required five-step structure: inject error → call CompleteAgent → assert returns nil → verify IDLE → verify recovery.
- **Primary behavioral proof** is at lines 1018-1020: `CompleteAgentForTest` returns `nil` while `GetAgentFields` error is active (injected at line 1014). This is the operative assertion.
- **IDLE check independence**: `state/mock.go:58-67` confirms `GetAgentState` reads `rec.fields.State` directly with no dependence on `getAgentFieldsErr`. Whether the error is active or cleared when `GetAgentState` is called is behaviorally irrelevant.
- Comment defect at lines 1022-1024 and ordering difference vs Scenario 15 are documentation issues, not behavioral gaps. No evidence chain is missing.

---

### Critic Positions
**CRITIC-1**: Both C10 conditions are present in substance; raised three defects, all ultimately downgraded to non-blocking.

- **Defect 1 (comment/ordering)**: The comment at `lifecycle_test.go:1022-1024` claims "GetAgentFields error is still injected" when it is not — the error is cleared at line 1025 before `GetAgentState` at line 1026. This inverts Scenario 15's order (which clears at line 962 after checking IDLE at line 953). Opened as blocking; downgraded to documentation defect after ADVOCATE-1 cited `mock.go:58-67` proving `GetAgentState` is independent of `getAgentFieldsErr`. No behavioral gap introduced.
- **Defect 2 (file ordering)**: Scenario 19 appears at line 973, between Scenario 15 (line 905) and Scenario 16 (line 1044), breaking sequential numbering. Non-blocking.
- **Defect 3 (no intermediate stale-state assertion)**: The test does not assert that `CurrentTaskID` remains set after `CompleteAgentForTest` fails to clear it. Removing the `else` branch at `supervisor.go:458-460` would not cause Scenario 19 to fail. Non-blocking; same pattern in Scenario 15.
- **Final statement**: "All Council 10 conditions are satisfied in substance. If the ARBITER concludes FOR, I do not have a remaining blocking objection to assert."

---

### Questioner Findings
QUESTIONER did not respond during this council. ARBITER assumed probing responsibilities directly, independently verifying the following:

- **Defect 1 core claim** — verified: `lifecycle_test.go:1025` clears error before `GetAgentState` at line 1026; `lifecycle_test.go:953` in Scenario 15 calls `GetAgentState` before clearing at line 962. Ordering difference is real.
- **ADVOCATE-1 counter-claim** — verified: `state/mock.go:58-67` confirms `GetAgentState` reads `rec.fields.State` with no check of `getAgentFieldsErr`. IDLE check is functionally equivalent in both error states.
- **Defect 3 symmetry** — noted: Scenario 15 also lacks an intermediate assertion about stale store state. The pattern is pre-existing.
- No claims were marked unsubstantiated.

---

### Key Conflicts
- **Defect 1 severity** — CRITIC said blocking (inverted order, wrong comment makes test structurally weaker than Scenario 15); ADVOCATE said non-blocking (primary proof at lines 1018-1020 is complete, GetAgentState is state-machine-only). **Resolved**: CRITIC conceded after ARBITER verified `mock.go:58-67`. No behavioral gap. Defect downgraded to documentation defect by CRITIC.

---

### Concessions
- **CRITIC-1** conceded "structurally weaker" framing after ADVOCATE-1 cited `state/mock.go:58-67`; downgraded Defect 1 from blocking → documentation defect.
- **ADVOCATE-1** conceded the comment at `lifecycle_test.go:1022-1024` is factually wrong.
- **ADVOCATE-1** conceded Defect 2 (file ordering) is unconventional.
- Both sides agreed Defects 2 and 3 are non-blocking throughout.

---

### Regression Lineage
No regression lineage — no prior fix involvement. Commit 0862bc1 introduced the Scenario 19 test and the `else` log branch; neither reopens a previously fixed issue.

---

### Evidence Verified by ARBITER

| Claim | File | Line(s) | Verdict |
|---|---|---|---|
| GetAgentFields log warning exists in completeAgent | `internal/supervisor/supervisor.go` | 458-460 | CONFIRMED |
| Scenario 19 present in lifecycle_test.go | `e2e/lifecycle_test.go` | 973-1042 | CONFIRMED |
| Error cleared at line 1025 before GetAgentState at 1026 | `e2e/lifecycle_test.go` | 1025-1026 | CONFIRMED |
| Scenario 15 clears error after GetAgentState (opposite order) | `e2e/lifecycle_test.go` | 953, 962 | CONFIRMED |
| GetAgentState reads rec.fields.State, no check of getAgentFieldsErr | `internal/state/mock.go` | 58-67 | CONFIRMED |
| Primary behavioral proof: CompleteAgent returns nil while error active | `e2e/lifecycle_test.go` | 1018-1020 | CONFIRMED |
| SetGetAgentFieldsError injection method exists in MockStore | `internal/state/mock.go` | 90-96 | CONFIRMED |
| CompleteAgentForTest export exists | `internal/supervisor/export_e2e.go` | 27-32 | CONFIRMED |
| Scenario 19 file position (between S15 and S16) | `e2e/lifecycle_test.go` | 973, 905, 1044 | CONFIRMED |

---

### Scope Audit

| Finding | Relevance test | Pre-existence test | Verdict |
|---|---|---|---|
| Log warning at supervisor.go:458-460 satisfies C10 Condition 1 | PASS — directly about motion | PASS — introduced by motion | INCLUDE |
| Scenario 19 behavioral proof at lifecycle_test.go:1018-1020 satisfies C10 Condition 2 | PASS — directly about motion | PASS — introduced by motion | INCLUDE |
| Comment defect at lifecycle_test.go:1022-1024 (wrong) | PASS — directly about motion's test | PASS — introduced in this PR | INCLUDE as in-PR improvement |
| Scenario 19 file ordering (S15→S19→S16→S17→S18) | PASS — about this PR's test | PASS — introduced by this PR | INCLUDE as in-PR improvement |
| Defect 3: no stale-state intermediate assertion | PASS — about this PR's test | FAIL — same pattern exists in Scenario 15 (line 952-962) pre-dates this PR | DROP (pre-existing pattern) |

---

### Arbiter Recommendation

**FOR**

Both Council 10 blocking conditions are satisfied in substance. The log warning at `supervisor.go:458-460` exactly mirrors the SetAgentFields failure log pattern. Scenario 19 at `lifecycle_test.go:1018-1020` correctly proves that `CompleteAgent` returns `nil` while `GetAgentFields` returns an error — the operative behavioral assertion. CRITIC-1's Defect 1 concern was substantiated as a documentation defect (wrong comment) but not a behavioral gap: `mock.go:58-67` confirms `GetAgentState` is entirely independent of `getAgentFieldsErr`, making the IDLE check at line 1030 correct regardless of error state. CRITIC-1 withdrew all blocking objections after this was established. No outstanding blocking conditions remain.

Prior council (Council 10) CONDITIONAL is satisfied. The six non-blocking follow-up issues from Council 10 (#57, #58, #60, #61, #64, #65) remain open and are not affected by this review.

---

### Conditions
None. Recommendation is unconditional FOR.

---

### Suggested Fixes

#### Bug Fixes (always in-PR)
None.

#### In-PR Improvements (scoped, non-bug)

**Fix wrong comment and optionally reorder lines in Scenario 19**

The comment at `e2e/lifecycle_test.go:1022-1024` states "GetAgentFields error is still injected" but the error is cleared at line 1025 before `GetAgentState` is called. The comment should be removed or corrected. Optionally, swap lines 1025 and 1026 (move error clear to after the IDLE assertion at lines 1026-1032) to match Scenario 15's cleanup ordering and make the comment accurate.

CITE: `e2e/lifecycle_test.go` L:1022-1026

**Consider reordering Scenario 19 in file to restore sequential numbering**

Scenario 19 appears at line 973, between Scenario 15 (line 905) and Scenario 16 (line 1044). Moving it after Scenario 18 (currently ending ~line 1240) would restore the 1→19 sequential order.

CITE: `e2e/lifecycle_test.go` L:973

#### PR Description Amendments
None.

#### Critical Discoveries (informational)
None.

---

### Verification Results

| # | Finding | Citations | Verdict | Action |
|---|---------|-----------|---------|--------|
| 1 | Wrong comment at lifecycle_test.go:1022-1024 | `e2e/lifecycle_test.go` L:1022-1026 | VERIFIED | Retained |
| 2 | Scenario 19 out of sequential order | `e2e/lifecycle_test.go` L:973 | VERIFIED | Retained |

Verification: 2 verified, 0 phantom (purged), 0 unverified (retained for review)

All findings verified against codebase.

---

### Council 10 vs Council 11 Comparison

| Council 10 Finding | Council 11 Status |
|---|---|
| CONDITIONAL: add log warning in completeAgent | SATISFIED — supervisor.go:458-460 ✓ |
| CONDITIONAL: add Scenario 19 mirroring Scenario 15 | SATISFIED IN SUBSTANCE — behavioral proof present at lines 1018-1020; comment defect is non-blocking ✓ |
| 6 non-blocking follow-up issues (#57, #58, #60, #61, #64, #65) | Open, unchanged — not in scope of this review |
