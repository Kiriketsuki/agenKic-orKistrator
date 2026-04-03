---
## Adversarial Council -- Feature 33 T2/T3 Final Merge Review

> Convened: 2026-03-23 19:00 | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion
"Feature 33 (T2/T3: LiteLLM Client & Provider Adapters) has resolved all prior council conditions (both rounds) and is ready to squash-merge into epic/3-implement-model-gateway-judge-router via PR #49."

### Advocate Positions
**ADVOCATE-1** (final position: CONDITIONAL FOR): The motion as stated is incorrect — C2.1 is not fully resolved. All five Council 1 code conditions and both advisory items are resolved in implementation. The Go interfaces, error taxonomy, adapter wiring, and test coverage are correct and comprehensive (23 top-level test functions, 28 distinct test cases). Council 2's C2.1 (spec sync) is resolved at the Go interface level, but three spec text divergences remain: base URL port (`spec.md:124,213` vs `litellm.go:17`), temperature semantics (`spec.md:38,180,321` vs `openai.go:30`), and Gherkin identifiers (`spec.md:276-292,370` vs `provider.go:47`). Conceded that "API contracts only" framing of C2.1 was too narrow — Gherkin acceptance scenarios, behavioral descriptions, and code-block defaults all fall within C2.1 scope. Conceded CRITIC-2's Mode A/Mode B test tautology analysis is correct and accepted the test fix as a condition (not merely advisory). Final position: CONDITIONAL FOR with 4 specific conditions (3 spec text + 1 test mechanism), achievable in one commit (~25 min).

### Critic Positions
**CRITIC-1** (final position: CONDITIONAL): Identified five findings. Findings 1-3 are confirmed spec-code divergences that constitute incomplete C2.1 resolution: (1) base URL `localhost:4000` in spec vs `localhost:8000` in code (`litellm-client-provider-adapters-spec.md:124,213` vs `litellm.go:17`), (2) temperature "zero/set to 0.0" in spec vs `-1` (omit) in code (`litellm-client-provider-adapters-spec.md:38,180,321` vs `openai.go:30`), (3) `AdapterRegistry.Lookup` in spec Gherkin vs `Registry.Find` in code (`litellm-client-provider-adapters-spec.md:276-292,370` vs `provider.go:47`). Findings 4-5 conceded as advisory (test port-occupancy assumption deferred to CRITIC-2's stronger formulation, copy semantics inconsistency has no current impact). Recommends CONDITIONAL: merge after three spec text fixes committed; endorses CRITIC-2's test mechanism fix but defers BLOCKING vs ADVISORY classification to ARBITER.

**CRITIC-2** (final position: CONDITIONAL): Demonstrated the WithBaseURL test at `litellm_test.go:259-272` is a structural tautology for the security guard. Traced two paths for all three bad URLs: Path A (guard present) produces `connection refused` at localhost:8000; Path B (guard removed) produces `unsupported protocol scheme` from `http.Transport`. Both yield `err != nil`, both pass the test assertion at line 269. Initially classified as BLOCKING (AGAINST), then **downgraded to CONDITIONAL** after accepting ADVOCATE-1's argument that the scheme check is defense-in-depth (not the primary security boundary) and Go's `http.Transport` independently enforces scheme rejection. The guard's value is fail-fast configuration validation, not SSRF prevention. Proposed fix: inject a recording `RoundTripper` (~15 lines). Also raised silent rejection (advisory, deferred to T5) and HTTPS branch untested (advisory). Endorses CRITIC-1's three spec fixes. Final: CONDITIONAL on all 4 conditions.

### Questioner Findings

| Probe | Target | Finding | Status |
|:------|:-------|:--------|:-------|
| 1 | ADVOCATE-1 (spec-code alignment) | Spec documents current signature (no error return) plus deferred evolution path. No contradiction. | SATISFIED |
| 2 | ADVOCATE-1 (test count) | Claimed 11+15=26; actual 7+16=23 top-level functions (28 cases with subtests). Corrected count doesn't weaken coverage. | SATISFIED (with correction) |
| 3 | ADVOCATE-1 (shallow copy safety) | Shared reference types: `Messages []Message`, `Metadata map[string]string`. No current adapter mutates these. Defensive doc exists. | SATISFIED |
| 4 | ADVOCATE-1 (base URL) | Commit 8f23f7a did not modify `litellm.go`. Spec still says 4000, code says 8000. C2.1 claim incomplete. | UNSUBSTANTIATED |
| 5 | ADVOCATE-1 (temperature) | Test at `provider_test.go:192` explicitly asserts `temperature should be negative (omit)`. Spec says 0.0. Code, test, and spec diverge. | UNSUBSTANTIATED |
| 6 | CRITIC-2 (test brittleness) | Path A (guard present) and Path B (guard removed) both produce `err != nil`. Test survives guard removal. | GROUNDED |
| 7 | ADVOCATE-1 (C2.1 scope) | Asked whether Gherkin scenarios are "API contracts" per C2.1. ADVOCATE-1 conceded the narrow framing — Gherkin, behavioral descriptions, and code-block defaults all fall within C2.1 scope. | SATISFIED (conceded) |

### Key Conflicts

1. **C2.1 completeness**: ADVOCATE-1 initially claimed full resolution; CRITIC-1 demonstrated three residual spec divergences; ARBITER confirmed all three against source files. ADVOCATE-1 conceded all three and conceded the narrow "API contracts only" C2.1 framing was too narrow. **Conflict resolved** — all parties agree divergences exist, fall within C2.1 scope, and need fixing.

2. **WithBaseURL test adequacy**: CRITIC-2 demonstrated structural tautology (test passes with or without guard). QUESTIONER grounded the claim. ADVOCATE-1 initially argued advisory, then conceded to accepting as a condition. CRITIC-2 initially argued BLOCKING/AGAINST, then downgraded to CONDITIONAL after accepting the guard is defense-in-depth, not the primary security boundary. **Conflict resolved** — all parties agree the test should be fixed as a condition; the severity dispute resolved to CONDITIONAL.

3. **Spec vs code authority**: All parties agree the code's behavior is correct (especially temperature omission for reasoning models — confirmed by OpenAI docs referenced at spec line 386: "temperature must be 1 or omitted for o1/o3"). The spec is the stale artifact. No code changes needed.

### Concessions

**ADVOCATE-1 conceded:**
- C2.1 is not fully resolved (opening claim was overstated)
- "API contracts only" framing of C2.1 was too narrow — Gherkin, behavioral specs, and code-block defaults are in scope
- Three spec text divergences exist and need fixing (base URL, temperature, Gherkin)
- WithBaseURL test is a structural tautology for the scheme guard (Mode A/Mode B analysis is "logically sound")
- Accepted the test fix as a condition (not merely advisory)
- Silent rejection at `litellm.go:74` is a latent operational risk
- HTTPS branch at `litellm.go:73` is untested
- Test count was wrong (claimed 26, actual 23 functions / 28 cases)
- Did not adequately rebut CRITIC-2's structural argument (acknowledged QUESTIONER's note)

**CRITIC-1 conceded:**
- Finding 4 (test port-occupancy assumption) — advisory, deferred to CRITIC-2's stronger formulation
- Finding 5 (copy semantics inconsistency) — advisory, no current impact
- Code quality is high — no code defects in any file

**CRITIC-2 conceded:**
- Issue 1 severity downgrade: BLOCKING → CONDITIONAL. The scheme check is defense-in-depth, not the primary security boundary. Go's `http.Transport` independently rejects non-http/https schemes. The guard's value is fail-fast configuration validation, not SSRF prevention.
- Issue 2 (silent rejection) — advisory, config loading (T5) is the correct validation point
- Issue 3 (HTTPS untested) — advisory, minor coverage gap
- Error propagation (from Council 2) — `ProviderError` wrapping adequate
- Metadata aliasing (from Council 2) — documented, not mutated by current adapters

### Regression Lineage

- **Spec divergences (F1-F3)**: These are NOT regressions from Council 2 fixes. Commit `8f23f7a` updated the API contract section (lines 68-114) correctly but missed the default URL comment (line 124), Gherkin Background (line 213), temperature constraint table (line 180), Gherkin temperature assertions (lines 321, 327), Gherkin method names (lines 276-292), and exit criteria (line 370). These are gaps in the C2.1 fix scope, not regressions in previously-correct content.

- **WithBaseURL test tautology**: This IS a regression relative to Council 2's C2.2 intent. C2.2 asked for the test to make "the scheme-rejection security property regression-proof." The test in `8f23f7a` improved significantly over the prior no-op `Provider()` assertion but does not achieve regression-proof-ness: removing the guard at `litellm.go:73` would not cause the test to fail (demonstrated by CRITIC-2, validated by QUESTIONER).

### Arbiter Recommendation
**CONDITIONAL**

The Go implementation is correct, well-tested, and architecturally sound. All five Council 1 code conditions and both advisory items are genuinely resolved in the implementation code. No code changes to the gateway are needed. All three council participants converged on CONDITIONAL — a unanimous disposition.

The motion claims "has resolved all prior council conditions." Two conditions remain incompletely resolved:

**C2.1 (spec sync)**: The API contract section was correctly updated, but three material divergences persist in the spec's defaults, behavioral descriptions, and acceptance scenarios. All parties agree these need fixing. The fixes are spec-text-only (~10 min).

**C2.2 (WithBaseURL test)**: The test was significantly improved but does not achieve the "regression-proof" standard C2.2 specified. CRITIC-2 demonstrated the test is a structural tautology — it passes whether or not the guard exists because Go's `http.Transport` independently rejects non-HTTP schemes. ADVOCATE-1 conceded the logical gap and accepted the test fix as a condition. CRITIC-2 downgraded from AGAINST to CONDITIONAL after recognizing the guard is defense-in-depth (not the primary security boundary). I weigh this as a condition for two reasons: (a) C2.2 explicitly asked for regression-proof-ness, which is not achieved; (b) the fix is small (~15 lines of `RoundTripper` injection) and can land in the same commit as the spec fixes.

Both conditions are achievable in a single commit with no changes to the gateway implementation code. Estimated effort: ~25 minutes total.

### Conditions (blocking)

1. **Complete C2.1 spec sync** — fix three residual spec-code divergences:
   - `litellm-client-provider-adapters-spec.md:124` and `:213`: change `http://localhost:4000` to `http://localhost:8000` (matching `litellm.go:17`)
   - `litellm-client-provider-adapters-spec.md:38,180,315,321,327,371`: change "zero/set to 0.0" language to "omit the temperature field" (negative sentinel convention, matching `openai.go:30` and `litellm.go:202-204`). Note: the spec's own reference at line 386 says "temperature must be 1 or omitted for o1/o3" — the code's behavior (omit) is correct.
   - `litellm-client-provider-adapters-spec.md:276,280,284,288,292,370`: change `AdapterRegistry.Lookup` to `Registry.Find` (matching `provider.go:47`)

   CITE: `litellm-client-provider-adapters-spec.md` L:124, L:180, L:213, L:276-292, L:315-327, L:370-371
   CITE: `litellm.go` L:17
   CITE: `openai.go` L:30
   CITE: `provider.go` L:47

2. **Complete C2.2 — make WithBaseURL test regression-proof** — replace the nil-only assertion at `litellm_test.go:269` with a mechanism that proves the guard rejected the bad scheme. Recommended approach: inject a recording `RoundTripper` into the HTTP client for bad-URL test cases that captures the request URL, then assert the URL targets `http://localhost:8000` (the default fallback), not the rejected scheme. This distinguishes "guard rejected, client fell back" from "guard accepted, transport failed."

   CITE: `litellm_test.go` L:262-272
   CITE: `litellm.go` L:70-78

### Suggested Fixes

#### Bug Fixes
None. All Go implementation code is correct.

#### In-PR Improvements

1. **Spec text alignment** (Condition 1): Find-and-replace operations in `litellm-client-provider-adapters-spec.md`:
   - `http://localhost:4000` -> `http://localhost:8000` (2 occurrences: lines 124, 213)
   - Temperature language: "Always zero the temperature field" -> "Always omit the temperature field (set to negative sentinel)" (line 180); "temperature set to 0.0" -> "temperature field is omitted (negative value)" (lines 321, 327); "zeros temperature" -> "omits temperature" (lines 38, 371); Rule title "zeroes" -> "omits" (line 315)
   - `AdapterRegistry.Lookup` -> `Registry.Find` (5 occurrences: lines 276, 280, 284, 288, 292) + exit criteria (line 370)

2. **WithBaseURL test mechanism** (Condition 2): Add a recording `RoundTripper` to the bad-URL test cases at `litellm_test.go:262-272`. Approximately 15 lines. Example:
   ```go
   type recordingTransport struct {
       requestedURL string
   }
   func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
       t.requestedURL = req.URL.String()
       return nil, errors.New("intentional test error")
   }
   ```
   Inject via `WithHTTPClient(&http.Client{Transport: &recordingTransport{}})`, then assert `transport.requestedURL` starts with `http://localhost:8000`, not the rejected scheme URL.

#### PR Description Amendments
- Note that commit `8f23f7a` resolved the API contract sections of C2.1 and C2.2 but left residual divergences in spec defaults, Gherkin scenarios, and test assertion specificity. This commit completes both conditions.

#### Critical Discoveries
None. No OWASP Top 10, data loss, or compliance issues were identified. The WithBaseURL scheme validation at `litellm.go:70-78` is defense-in-depth alongside Go's `http.Transport` built-in scheme rejection; neither is a standalone SSRF mitigation (full host allowlist deferred to T5 config loading per `gateway.go:182-184`).

### Prior Council Comparison

| Council | Date | Outcome | Conditions | Status |
|:--------|:-----|:--------|:-----------|:-------|
| Council 1 | 2026-03-23 14:30 | CONDITIONAL | 5 code conditions + 2 advisory | All 7 resolved in implementation |
| Council 2 | 2026-03-23 18:00 | CONDITIONAL | C2.1 (spec sync) + C2.2 (test fix) | Partially resolved: API contracts aligned, but defaults/Gherkin/temperature text and test mechanism incomplete |
| Council 3 (this) | 2026-03-23 19:00 | CONDITIONAL | 2 conditions (complete C2.1 + complete C2.2) | Pending |

**Trajectory**: Each council has narrowed the gap. Council 1 found 5 code issues; Council 2 reduced to 2 documentation/test issues; this council confirms those 2 are partially resolved and specifies the remaining work precisely. The implementation code has been stable since Council 1 — all remaining work is spec text and test refinement. All three council participants converged from divergent positions (FOR, CONDITIONAL, AGAINST) to unanimous CONDITIONAL.

### Advisory Items (non-blocking, tracked for future work)

| # | Item | Source | Recommended Timing |
|:--|:-----|:-------|:-------------------|
| 1 | Add `slog.Warn` for rejected schemes in `WithBaseURL` (`litellm.go:74`) | CRITIC-2 Issue #2 | T5 (config loading) |
| 2 | Test HTTPS acceptance via `httptest.NewTLSServer` (`litellm.go:73`) | CRITIC-2 Issue #3 | Next PR touching this file |
| 3 | Consider `maps.Clone` for Metadata in `Registry.Resolve` to prevent future aliasing | Council 1 advisory | When a future adapter needs to mutate Metadata |

### Verification Results (populated by verifier)
- [ ] Condition 1a: Spec base URL updated to `http://localhost:8000` (lines 124, 213)
- [ ] Condition 1b: Spec temperature language updated to "omit" (lines 38, 180, 315, 321, 327, 371)
- [ ] Condition 1c: Spec Gherkin updated from `AdapterRegistry.Lookup` to `Registry.Find` (lines 276-292, 370)
- [ ] Condition 2: WithBaseURL test asserts actual URL targeted via `RoundTripper` (not just `err != nil`)
- [ ] All tests pass: `go test ./internal/gateway/...`
---
