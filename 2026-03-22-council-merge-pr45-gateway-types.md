---
## Adversarial Council — Merge PR #45 into Epic/3

> Convened: 2026-03-22 | Advocates: 2 | Critics: 2 | Rounds: 3/4

### Motion

"PR #45 (feature/32-feature-gateway-interface-types-t1) is ready to squash-merge into epic/3-implement-model-gateway-judge-router. The PR adds the Gateway interface, types, error sentinels, and acceptance tests for the T1 foundation task. A prior council issued CONDITIONAL with 9 conditions — all have since been addressed in commit 4f76ec8. CI is green, PR is MERGEABLE, no review comments."

---

### Advocate Positions

**ADVOCATE-1**: All 9 prior council conditions are met with direct citations. The prior council conceded the four core architectural decisions (context.Context on all interface methods `gateway.go:191-223`, RoutingDecision return type `gateway.go:62-66`, Router.Classify() naming `gateway.go:204-207`, FallbackError{[]ProviderError} `errors.go:49-62`). The two test scenarios CRITIC-2 and CRITIC-1 cite require a concrete Completer implementation, which T1's explicit out-of-scope declaration (`gateway-interface-types-spec.md:9`) prohibits. Condition 1 names `gateway-interface-types-spec.md` specifically; the epic spec was never in scope. TokenUsage satisfies its Must-Have by existing. LastNDays inversion is API misuse under a documented contract. FallbackError.Error() has no corresponding acceptance scenario requiring a string format test — the spec scenario at lines 307-310 only requires `errors.Is`, which is tested.

**ADVOCATE-2**: Three pillars: completeness (all must-have symbols enumerated at `gateway.go:1-224`, `errors.go:1-63`), risk profile (zero external imports, types-only layer, compile-time interface check at `gateway_test.go:174-196`), and delivery cost (T2-T7 blocked). Conceded four gaps over the course of debate: scenarios 5 & 6 tests absent, `model-gateway-spec.md:53-57` stale, and `FallbackError.Error()` untested. All are documentation/test omissions on a zero-external-import layer with CI green — none represent behavioral defects or API correctness failures. Withdrew the CostReport aggregation scenario concession (line 285 is outside the Condition 9 range of 214-283). Characterised the epic spec update as appropriate to include in this PR as an atomic fix.

---

### Critic Positions

**CRITIC-1**: Condition 9 is unmet: two acceptance scenarios within lines 271-283 have no tests — multi-turn CompletionRequest (lines 271-276) and FallbackUsed (lines 277-281). Conceded CostReport aggregation scenario is outside the Condition 9 range and requires T7 aggregation logic (withdrawn from objection). Scenarios 5 & 6 are pure struct-construction assertions — the "When passed to a Completer" clause in Gherkin is narrative context, not a test precondition. Both advocates conceded the tests are absent. Also objected that both copies of `model-gateway-spec.md:53-57` carry the wrong Gateway interface (no context.Context, Route returns ModelTier not RoutingDecision), which CLAUDE.md directs all T2-T9 contributors to read first. Conceded TokenUsage and FallbackError.Unwrap() design as non-blocking.

**CRITIC-2**: Four initial findings. Conceded Finding 2 (TokenUsage — spec requires existence, not wiring) and Finding 3 (LastNDays inversion — valid quality concern but outside the 9 prior conditions; not a merge blocker under the motion's scope). Maintained Finding 1 (scenarios 5 & 6 untested; both advocates factually conceded absence; struct-only test requires no Completer). Narrowed Finding 4 from a design defect to a documentation gap — `FallbackError.Unwrap()` design is intentional and correct; the gap is that no test or comment demonstrates the `errors.As(err, &fe) → range fe.Errors` pattern that T6 will need. Independently verified and pressed the epic spec staleness finding (`model-gateway-spec.md:53-57` and `specs/model-gateway-spec.md:53-57` both confirmed stale).

---

### Key Conflicts

- **Scenarios 5 & 6 require a Completer?** — ADVOCATE-1 argued the Gherkin "When passed to a Completer" clause requires a concrete Completer, prohibited by T1's out-of-scope declaration. CRITIC-1 and CRITIC-2 argued the "Then no data fields are lost" clause is the testable assertion — pure struct construction + field read, no Completer needed. CRITIC-1 provided a three-line conforming test implementation. — **RESOLVED**: Critics' reading is correct. Gherkin "When" clauses describe context; "Then" clauses are assertions. Both advocates conceded the tests are absent. The struct-assertion interpretation is correct.

- **Epic spec staleness is Condition 1 violation or new condition?** — ADVOCATE-1 argued Condition 1 names `gateway-interface-types-spec.md` only; the prior council had access to `model-gateway-spec.md` and chose not to name it. CRITIC-1 argued the condition's rationale ("gates all downstream work / active misleading reference") applies equally to the epic spec. ADVOCATE-2 conceded the staleness and proposed including the fix in-PR. — **RESOLVED**: Condition 1 as literally written is satisfied (feature spec was synced). However, the epic spec staleness is independently confirmed as a real risk: CLAUDE.md directs T2-T9 authors to read this file first; the stale interface would produce non-compiling downstream code. This is an independent objection requiring an in-PR fix regardless of Condition 1 scope.

- **LastNDays(n<0) silent inversion** — CRITIC-2 raised as a data-correctness hazard; both advocates argued it is API misuse under a documented contract, outside the 9 prior conditions. CRITIC-2 withdrew it as a merge blocker under the motion's scope. — **NOT RESOLVED as motion-scope issue**, but identified as a real behavioral defect independently (see Bug Fixes below).

- **FallbackError.Error() untested** — ADVOCATE-1 argued the spec scenario at lines 307-310 only requires `errors.Is` (tested at `gateway_test.go:160-170`), not a string format test. CRITIC-1 argued the multi-line error string is entirely unverified. — **PARTIALLY RESOLVED**: ADVOCATE-1's reading of the spec scenario is technically correct; it is not a Condition 9 failure. However, it is a genuine test gap in a PR that explicitly tests `ProviderError.Error()` format (two tests at `gateway_test.go:135-158`) while leaving the sibling `FallbackError.Error()` unverified.

---

### Concessions

- **CRITIC-1 conceded**: CostReport aggregation scenario (line 285-294) is outside Condition 9 range and requires T7 logic — withdrawn.
- **CRITIC-1 conceded**: TokenUsage — spec requirement is "type exists with two fields," which `gateway.go:106-110` satisfies. Withdrawn as blocking.
- **CRITIC-2 conceded**: Finding 2 (TokenUsage) — same as CRITIC-1.
- **CRITIC-2 conceded**: Finding 3 (LastNDays inversion) — valid quality concern but outside the 9 prior conditions; not a merge blocker under this motion.
- **CRITIC-2 narrowed**: Finding 4 (FallbackError.Unwrap) from design defect to documentation gap only — design is intentional and correct.
- **ADVOCATE-2 conceded**: Scenarios 5 & 6 tests absent from `gateway_test.go`.
- **ADVOCATE-2 conceded**: `model-gateway-spec.md:53-57` and `specs/model-gateway-spec.md:53-57` are factually stale and proposed in-PR fix.
- **ADVOCATE-2 conceded**: `FallbackError.Error()` format untested.
- **ADVOCATE-1 conceded** (factual observation): Scenarios 5 & 6 tests absent (disputed T1-scope interpretation only).
- **ADVOCATE-1 confirmed** (factual observation): `model-gateway-spec.md:53-57` is stale.
- **ADVOCATE-2 withdrew**: CostReport aggregation scenario concession — outside line range 214-283 and requires T7 logic.

---

### Arbiter Recommendation

**CONDITIONAL FOR**

The code in `internal/gateway/gateway.go` and `internal/gateway/errors.go` is architecturally sound, correctly implements all must-have contracts, carries zero external imports, and passes CI. The prior council's core decisions — context.Context threading, RoutingDecision return type, Router.Classify() naming, FallbackError composition — are all correctly implemented and were not seriously challenged. The debate converged on a finite set of documentation and test gaps, none of which represent behavioral defects or API design errors. However, four fixable items must be addressed before the squash-merge proceeds: (1) two acceptance scenario tests within Condition 9's explicit scope are absent from the test file, which both advocates acknowledged; (2) both epic spec files that CLAUDE.md directs all T2-T9 authors to read first carry a stale Gateway interface that would produce non-compiling downstream code on two axes; (3) `LastNDays(n<0)` silently produces an inverted `TimePeriod` with no guard or error, which is a behavioral defect regardless of whether it was in the original 9 conditions; and (4) `FallbackError.Error()` format is completely unverified despite the PR explicitly testing the sibling `ProviderError.Error()` format.

---

### Conditions (if CONDITIONAL)

1. **Add `TestCompletionRequest_MultiTurn`**: Construct `CompletionRequest{Messages: []Message{{...},{...},{...}}}` with three entries and assert `len(req.Messages) == 3` and all field values are readable. Covers `gateway-interface-types-spec.md:271-276`.

2. **Add `TestCompletionResponse_FallbackUsed`**: Construct `CompletionResponse{FallbackUsed: true, ProviderName: "openai"}` and assert both fields are readable. Covers `gateway-interface-types-spec.md:277-281`.

3. **Update `model-gateway-spec.md:53-57`** and **`specs/model-gateway-spec.md:53-57`** to match `gateway.go:191-201`:
   ```go
   type Gateway interface {
       Route(ctx context.Context, task TaskSpec) (RoutingDecision, error)
       Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
       GetCostReport(ctx context.Context, period TimePeriod) (CostReport, error)
   }
   ```

4. **Add `TestFallbackError_ErrorString`**: Construct a `FallbackError` with two `ProviderError` values and assert the exact output of `.Error()` against the format produced by `errors.go:54-60`.

---

### Suggested Fixes

#### Bug Fixes (always in-PR, regardless of original scope)

- **`LastNDays(n<0)` silent inversion** (`gateway.go:164-168`): `LastNDays(-3)` computes `AddDate(0, 0, 3)` → `TimePeriod{Start: future, End: now}` with `Start > End`. No error, no guard, no panic. A consumer relying on the interval being well-formed (e.g., T7 building a Redis time-range query) will receive silently malformed output. Fix: add an input guard. Minimum: `if n < 0 { n = 0 }`. Preferred: `if n < 0 { return TimePeriod{} /* or return error */ }`. The doc comment at `gateway.go:163` ("covering the last n days") makes positive-n the only documented semantic; the implementation should enforce it.

#### In-PR Improvements (scoped, non-bug)

- **`TestCompletionRequest_MultiTurn`** and **`TestCompletionResponse_FallbackUsed`**: See Conditions 1 and 2 above.
- **`TestFallbackError_ErrorString`**: See Condition 4 above.
- **`FallbackError` access pattern documentation** (`errors.go:49-51`): Add a doc comment on `FallbackError` explaining that `errors.Is(err, ErrAllProvidersFailed)` is the sentinel check, while `errors.As(err, &fe)` targeting `*FallbackError` is the correct path to inspect individual provider failures via `fe.Errors`. Prevents T6 authors from first attempting `errors.As(err, &pe)` targeting `*ProviderError`, finding it returns false, and having no documentation explaining why.

#### PR Description Amendments

- Update the PR description to note that `model-gateway-spec.md` and `specs/model-gateway-spec.md` were also updated as part of this commit, so reviewers reviewing future PRs know the epic spec is now authoritative for the interface shape.

#### New Issues (future features/enhancements only — confirm with human before creating)

- None. All findings are addressable within this PR scope. No genuine future feature or enhancement was identified.

---

*Authored by: ARBITER (claude-sonnet-4-6) | Council convened 2026-03-22*
