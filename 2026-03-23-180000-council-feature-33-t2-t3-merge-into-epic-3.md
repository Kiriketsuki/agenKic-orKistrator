---
## Adversarial Council -- Feature 33 T2/T3 Merge into Epic 3 (Post-Fix Re-Review)

> Convened: 2026-03-23 18:00 | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion
"Feature 33 (T2/T3: LiteLLM Client & Provider Adapters) has addressed all five prior council conditions and is ready to squash-merge into epic/3-implement-model-gateway-judge-router via PR #49."

### Advocate Positions
**ADVOCATE-1**: All five prior council conditions are resolved with verifiable code and tests. (1) Adapter wiring via `AdapterResolver` in `LiteLLMClient.Complete` at `litellm.go:125-131` delegates to `Registry.Resolve` at `provider.go:58-64`, which calls `Find` then `FormatRequest` -- satisfying Condition 1. (2) `buildRequest` at `litellm.go:188-190` serializes `SystemPrompt` as a system message -- Condition 2. (3) `OllamaAdapter.FormatRequest` at `ollama.go:23-27` strips the `ollama/` prefix -- Condition 3. (4) `Registry.Find` at `provider.go:47-54` returns `(FormatAdapter, error)` surfacing `ErrNoProvider` -- Condition 4. (5) `WithBaseURL` at `litellm.go:72-75` validates http/https schemes -- Condition 5. Both post-merge hardening items (io.LimitReader at `litellm.go:149`, Metadata aliasing documented at `provider.go:16-17`) are already addressed. Error architecture, API key redaction, context cancellation, and interface composition are all tested. Conceded that the spec needs updating and the WithBaseURL test needs strengthening -- both lightweight, non-code fixes.

### Critic Positions
**CRITIC-1**: The five conditions are technically resolved, but the branch ships a spec-implementation divergence and an interface design gap. The spec at `litellm-client-provider-adapters-spec.md:74-86` (committed after the implementation in `65e8e0e`) defines `FormatRequest` returning `(map[string]any, error)` and `ParseModelName` returning `string`, while the code uses `CompletionRequest` and `bool` respectively. The missing error return on `FormatRequest` is a deferred capability that should be documented. Conceded that the `AdapterResolver` indirection is standard dependency inversion (not obfuscation), that the functional option pattern doesn't require error returns, and that the prior council condition doesn't forbid interface-mediated wiring.

**CRITIC-2**: The validation logic at `litellm.go:72-75` is correct, but the test at `litellm_test.go:237-257` only asserts the constructor didn't panic -- it cannot verify invalid schemes were rejected. A regression removing the scheme check would pass unchanged. Conceded error message propagation (no user-facing surface before T5) and Metadata aliasing (settled as non-blocking by prior council). Narrowed to one surviving argument: the WithBaseURL test should be strengthened before merge.

### Questioner Findings
- **Spec prescriptive status**: CONFIRMED. The spec at `litellm-client-provider-adapters-spec.md:36` uses Must-Have language and lines 69-91 are titled "API Contracts" with explicit Go signatures. The spec is prescriptive, not descriptive. CITE: `litellm-client-provider-adapters-spec.md` L:36, L:74-86
- **WithBaseURL test assertion**: CONFIRMED INADEQUATE. The test at `litellm_test.go:253` asserts only `client.Provider() != "litellm"` -- a field independent of `baseURL` (`litellm.go:119` returns `c.providerName`). The test proves no-panic, nothing about URL rejection. All three debaters agreed. CITE: `litellm_test.go` L:249-256
- **`main.go` existence**: ADVOCATE-1 claimed "There is no `main.go`" -- factually incorrect. `cmd/orchestrator/main.go` exists (99 lines) with gRPC and HTTP servers. However, it does NOT import `internal/gateway`, so the broader point (no gateway consumer exists) holds. CITE: `cmd/orchestrator/main.go` L:1-99
- **Functional option pattern**: CRITIC-1 conceded after probe that `func(*T)` without error return is standard in Go (gRPC, zap). Alternative validation points exist. The claim of "structurally impossible" error reporting was overstated.
- **Condition 1 indirection**: The prior council condition specifies what to call, not the mechanism. Interface-mediated wiring satisfies the condition. CRITIC-1 conceded.

### Key Conflicts
1. **Spec-implementation divergence** -- CRITIC-1 raised, ADVOCATE-1 conceded spec must be updated. Agreed the code design is superior to the spec's design. Minor disagreement on whether the missing `FormatRequest` error return should be documented as a deferred capability -- resolved by CRITIC-1's Option 2 (spec update + TODO note).
2. **WithBaseURL test adequacy** -- All three debaters agreed the test is inadequate. ADVOCATE-1 proposed a concrete fix (~15 lines, httptest.NewServer-based). CRITIC-2 argued it should block merge. ADVOCATE-1 accepted.
3. **Silent rejection pattern** -- CRITIC-1 raised as architectural flaw, then partially conceded after QUESTIONER probe revealed the `func(*T)` pattern is standard Go. Remains a local design weakness, not systemic. Not elevated to blocking.

### Concessions
**ADVOCATE-1 conceded:**
- Spec must be updated before merge (blocking)
- WithBaseURL test should be strengthened before merge (blocking)
- Silent rejection in WithBaseURL is a weaker design than error-returning options
- Test at `litellm_test.go:249-256` only verifies no-panic, not the security property

**CRITIC-1 conceded:**
- Point 3 (AdapterResolver indirection) fully withdrawn -- standard dependency inversion
- Point 2 (silent rejection) partially withdrawn -- `func(*T)` pattern is standard Go; concern overstated
- The prior council condition does not forbid interface-mediated wiring

**CRITIC-2 conceded:**
- Point 3 (Metadata aliasing) fully withdrawn -- settled as non-blocking by prior council
- Point 2 (error propagation) fully withdrawn -- no user-facing surface before T5
- Point 1 narrowed to test quality, then accepted ADVOCATE-1's two-condition framing

### Regression Lineage
- **Spec divergence**: NOT a regression from prior council fixes. The spec was new documentation committed alongside the implementation (`65e8e0e`). It was never reviewed against the code it documents.
- **WithBaseURL test weakness**: RELATED to prior council Condition 5 (URL scheme validation). The code fix at `litellm.go:72-75` was correctly implemented per the condition, but the test added to verify it (`litellm_test.go:237-257`) does not actually assert the security property. This is a gap in the verification of a prior council fix.

### Arbiter Recommendation
**CONDITIONAL**

All five prior council conditions are implemented correctly in code and verified by the ARBITER against the source files. The implementation design (typed `CompletionRequest` returns, `bool` predicate for `ParseModelName`, `AdapterResolver` dependency inversion) is sound and arguably superior to the spec's original design. Two lightweight issues -- a spec that contradicts the implementation it documents, and a security test that proves less than it claims -- should be resolved before merge. Both are achievable in a single commit with no changes to the gateway implementation code.

### Conditions (blocking)
1. **Update `litellm-client-provider-adapters-spec.md`** to match the implemented API contracts. Specifically: update the `FormatAdapter` interface signatures at lines 74-86, the Must-Have scope at line 36, and add a documented note that the `FormatRequest` error return was intentionally deferred (to be added when a future adapter needs request rejection).
   CITE: `litellm-client-provider-adapters-spec.md` L:36, L:74-86
   CITE: `internal/gateway/providers/provider.go` L:9-22

2. **Strengthen `TestLiteLLMClient_WithBaseURL_RejectsInvalidSchemes`** at `litellm_test.go:237-257`. Replace the current `Provider()` assertion with an `httptest.NewServer`-based test that: (a) constructs a client with the test server's URL and verifies it connects successfully, (b) constructs a client with `WithBaseURL("ftp://evil.com")` and verifies it does NOT reach the test server (falls back to default, gets connection refused). This makes the scheme-rejection security property regression-proof.
   CITE: `internal/gateway/litellm_test.go` L:237-257
   CITE: `internal/gateway/litellm.go` L:70-78

### Suggested Fixes

#### Bug Fixes
None. All prior council conditions are correctly implemented in code.

#### In-PR Improvements
1. **Spec contract alignment** (Condition 1 above): Update `litellm-client-provider-adapters-spec.md` sections at lines 36, 74-86, and the task breakdown to reflect `FormatRequest(req CompletionRequest) CompletionRequest`, `ParseModelName(model string) bool`, and `Registry.Find` method signature. Add a note: "Error return on FormatRequest intentionally deferred -- current adapters only perform infallible transformations (temperature clamping, prefix stripping). When a future adapter needs to reject requests, evolve the interface to `(CompletionRequest, error)` and update `Registry.Resolve` accordingly. Blast radius at time of writing: 3 adapters, 1 call site."
   CITE: `litellm-client-provider-adapters-spec.md` L:36, L:74-86

2. **WithBaseURL test hardening** (Condition 2 above): Replace the no-op assertion with an httptest.NewServer-based negative test (~15 lines). Example approach: start a test server, verify `WithBaseURL(srv.URL)` routes to it, then verify `WithBaseURL("ftp://evil.com")` does not.
   CITE: `internal/gateway/litellm_test.go` L:237-257

#### PR Description Amendments
- Note in the PR description that the spec was updated post-implementation to align with the actual API contracts and document the deferred `FormatRequest` error return.

#### Critical Discoveries (informational)
None. No OWASP Top 10, data loss, or compliance issues were identified. The Metadata shallow-copy concern was confirmed as non-blocking (documented contract, correct implementations, trivial fix path via `maps.Clone` in `Registry.Resolve`).

### Prior Council Comparison
The prior council (2026-03-23 14:30) was CONDITIONAL with 5 code conditions + 2 post-merge items. This council **confirms** all 5 code conditions are resolved and both post-merge items are addressed ahead of schedule. This council **adds nuance**: the spec committed alongside the fixes was never reconciled with the implementation, and the test for Condition 5 (URL scheme validation) doesn't assert the property it covers. Two new lightweight conditions replace the prior five.

### Verification Results (placeholder -- populated by verifier)
- [ ] Condition 1: Spec updated to match implementation
- [ ] Condition 2: WithBaseURL test strengthened with httptest.NewServer assertion
- [ ] All tests pass: `go test ./internal/gateway/...`
---
