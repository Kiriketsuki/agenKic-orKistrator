# Adversarial Council -- T2/T3 LiteLLM Proxy Client & Provider Format Adapters Merge

> Convened: 2026-03-23 14:30 | Advocates: 1 | Critics: 2 | Rounds: 2/4 | Motion type: CODE

---

## Motion

"The T2/T3 implementation (LiteLLM proxy client + provider format adapters) is ready to squash-merge into epic/3."

---

## Advocate Positions

**ADVOCATE-1**: The core design is sound and individually well-tested. `LiteLLMClient` correctly implements the `gateway.Completer` interface (`litellm.go:101,105`). Error normalisation maps HTTP 429 to `ErrRateLimited` and 5xx to `ErrProviderUnavailable`, both wrapped in `*ProviderError` with `Unwrap()` support for `errors.Is` (`litellm.go:119-136`, `errors.go:31-51`). Context cancellation is correctly propagated via `http.NewRequestWithContext` (`litellm.go:111`). All three `FormatAdapter` implementations are individually correct, immutable (shallow copy before mutation), and tested with table-driven tests covering temperature constraints and field preservation. The feature is stdlib-only with no external dependencies. ADVOCATE-1 revised to CONDITIONAL after conceding four defects in Round 2 and agreed the five pre-merge conditions are surgical and do not require architectural redesign.

---

## Critic Positions

**CRITIC-1 (Architecture)**: The providers package ships as inert dead code. T3.7 -- "Integrate adapter lookup into `LiteLLMClient.Complete`" -- is explicitly in the T2/T3 spec task table (`litellm-client-provider-adapters-spec.md:337`) with dependencies T2.3 and T3.5, both present in this branch. `LiteLLMClient.Complete` (`litellm.go:105-155`) builds its HTTP request via `buildRequest` (`litellm.go:158`) without ever consulting the adapter registry, making the entire `providers/` package unreachable from the client. Additionally: the Ollama acceptance scenario (`litellm-client-provider-adapters-spec.md:208`) requires `"model set to 'llama3' (prefix stripped)"` but `OllamaAdapter.FormatRequest` (`ollama.go:23`) returns `req` unchanged, and `TestOllamaAdapter_FormatRequest_Passthrough` (`provider_test.go:271-273`) asserts the prefix is preserved -- directly contradicting the acceptance criterion. `Registry.Find` (`provider.go:45-52`) returns `(nil, false)` instead of an error, making `ErrNoProvider` (`errors.go:7`) unreachable despite being an explicit exit criterion (`litellm-client-provider-adapters-spec.md:349`). CRITIC-1 also identified that `buildRequest` (`litellm.go:158-177`) silently drops `CompletionRequest.SystemPrompt` (`gateway.go:75`), though this was later reclassified from merge-blocking to "pre-merge fix or explicit TODO" after QUESTIONER established it has no corresponding Gherkin scenario.

**CRITIC-2 (Security)**: `WithBaseURL` (`litellm.go:65`) accepts any URL string without validation, which is then concatenated directly into an outbound HTTP call (`litellm.go:111`). `gateway.go:183` contains an explicit `TODO(T2)` noting that Completer implementations must "validate the host against a scheme+host allowlist and block private CIDRs." `LiteLLMClient` is the only such implementation. A caller supplying `http://169.254.169.254/` or `http://10.0.0.1:6379` will have the request faithfully executed -- a textbook SSRF vector. ADVOCATE-1 agreed this is "a legitimate pre-merge fix." CRITIC-2 also identified that `litellm.go:131` and `litellm.go:140` read `resp.Body` directly without `io.LimitReader`, creating an OOM vector from a compromised or misconfigured proxy, though this was classified below the Critical Discovery threshold and is not independently merge-blocking.

---

## Questioner Findings

QUESTIONER probed five claims during Round 1:

1. **CRITIC-2's TODO authorship framing** -- "wrote this note specifically for the code being merged today": UNSUBSTANTIATED. CRITIC-2 could not establish authorship timing without git blame. Claim withdrawn.

2. **CRITIC-2's LiteLLM auth docs claim** -- "LiteLLM's own documentation gates all proxy requests behind a master key in production deployments": UNSUBSTANTIATED. Drawn from training data, not a citable URL from files under review. Claim withdrawn. Finding 2 (no auth header) reclassified as a deployment posture concern and withdrawn as a client-level defect.

3. **ADVOCATE-1's immutability claim for `Metadata`**: Partially unsubstantiated. `out := req` performs a shallow copy; `Metadata map[string]string` is a reference type and is shared between original and copy. No current adapter mutates Metadata, so the functional claim holds today, but the interface contract relies on author discipline rather than structural enforcement. ADVOCATE-1 conceded the structural fragility.

4. **CRITIC-1's Defect 3 (SystemPrompt) as merge-blocking**: Probed for a supporting Gherkin scenario. None exists in the spec. CRITIC-1 revised classification from "merge-blocking" to "pre-merge fix recommended, or explicit in-code TODO with tracking issue." The field is documented at `gateway.go:75` with Anthropic as the motivating case; the silent drop is a real functional gap, but it lacks an exit criterion.

5. **CRITIC-1's T3.7 scope and Ollama test**: Both fully confirmed by verbatim quotation from `litellm-client-provider-adapters-spec.md:337` (T3.7 in T2/T3 task table with T2.3/T3.5 dependencies) and `provider_test.go:271-273` vs `spec:208` (test directly contradicts acceptance scenario). Substantiated.

---

## Key Conflicts

- **T3.7 scope (T2/T3 vs T5/T6)** -- ADVOCATE-1 claimed T3.7 belongs to T5/T6; CRITIC-1 cited verbatim spec table showing T3.7 under the T2/T3 feature breakdown with T2.3 and T3.5 dependencies -- **resolved: CRITIC-1 correct, ADVOCATE-1 conceded**

- **Ollama prefix stripping** -- ADVOCATE-1 claimed non-stripping resolves Open Question Q3 correctly, citing `spec:25`; ARBITER CLARIFY challenge identified that `spec:39` (must-have) and `spec:208` (Gherkin) both require stripping; Q3 is marked unresolved (`[ ]`) but the spec body is unambiguous -- **resolved: ADVOCATE-1 conceded, point #6 fully withdrawn**

- **SSRF as spec exit criterion violation** -- CRITIC-2 initially framed the `gateway.go:183` TODO as a binding T2 exit criterion; QUESTIONER challenged this; exit criteria at `spec:344-351` do not include SSRF validation -- **resolved: CRITIC-2 conceded the framing, recharacterised as acknowledged open security item**

- **SystemPrompt as merge-blocking** -- CRITIC-1 and ADVOCATE-1 initially agreed it was merge-blocking; QUESTIONER probed for a supporting Gherkin scenario and found none; CRITIC-1 revised to "pre-merge fix or explicit TODO" -- **resolved: reclassified as conditional pre-merge requirement, not hard exit criterion failure**

---

## Concessions

- **ADVOCATE-1** conceded: T3.7 is in-scope and absent (Defect 1); Ollama test asserts wrong behaviour against acceptance scenario (Defect 2 + ARBITER CLARIFY); `buildRequest` silently drops `SystemPrompt` (Defect 3); `ErrNoProvider` is unreachable and exit criterion is unmet (Defect 4); SSRF scheme validation is a "legitimate pre-merge fix" (CRITIC-2 Finding 1); overall position shifted from FOR to CONDITIONAL.

- **CRITIC-2** conceded: SSRF TODO authorship framing unsubstantiated; LiteLLM auth docs claim unsubstantiated; Finding 2 (no auth header) withdrawn as client-level defect; Finding 4 (error message propagation) withdrawn as a merge condition; Finding 1 reframed from exit criterion violation to acknowledged open security item; Finding 3 classified below Critical Discovery threshold.

- **CRITIC-1** conceded: Partial -- acknowledged the spec contains a genuine internal contradiction on Ollama prefix stripping (Q3 unresolved vs. must-have body + acceptance scenario); maintained acceptance scenario is the controlling test criterion. Conceded Defect 3 (SystemPrompt) does not map to a failing exit criterion.

---

## Regression Lineage

No regression lineage -- no prior fix involvement. This is a new feature branch.

---

## Arbiter Recommendation

**CONDITIONAL**

The core architecture -- `Completer` interface, error sentinel chain, functional options pattern, context propagation -- is sound and correctly implemented. All parties converged on this. However, five substantiated defects prevent unconditional merge: T3.7 was not wired (the providers package is inert dead code relative to the client), one acceptance scenario is tested backwards (Ollama prefix), one exit criterion is demonstrably unmet (`ErrNoProvider` unreachable), `buildRequest` silently drops `SystemPrompt`, and an in-code `TODO(T2)` for SSRF mitigation at `gateway.go:183` was not addressed. These are all surgical fixes -- no redesign is required -- but merging without them would signal false completion of T3 and leave the branch with tests that contradict their own acceptance criteria.

---

## Conditions (CONDITIONAL)

All five must be addressed before squash-merge:

1. **Wire T3.7** -- Call `registry.Find(req.Model)` and, if found, `adapter.FormatRequest(req)` inside `LiteLLMClient.Complete` before passing to `buildRequest`. Resolves inert dead code in the providers package.
   CITE: `litellm.go` L:105 (insertion point: before buildRequest call at L:106)

2. **Serialize `SystemPrompt`** -- `buildRequest` must include `req.SystemPrompt` in the outbound wire body. Either prepend as a `{role: "system"}` message in the messages array, or add a top-level `system` field (pending Open Question Q2 resolution). If intentionally deferred, add an explicit in-code `TODO` with a tracking issue reference -- silent omission of a documented field is not acceptable.
   CITE: `litellm.go` L:158-177

3. **Fix Ollama prefix stripping** -- Resolve Open Question Q3 explicitly. Given that `litellm-client-provider-adapters-spec.md:39` and `:208` both require stripping, implement prefix stripping in `OllamaAdapter.FormatRequest` and correct `TestOllamaAdapter_FormatRequest_Passthrough` to assert the stripped model name, not the original.
   CITE: `ollama.go` L:23, `provider_test.go` L:262-281

4. **Surface `ErrNoProvider`** -- Change `Registry.Find` signature to `(FormatAdapter, error)`, returning `ErrNoProvider` when no adapter matches. Update all callers. This satisfies the exit criterion at `litellm-client-provider-adapters-spec.md:349`.
   CITE: `provider.go` L:45-52, `errors.go` L:7

5. **URL scheme validation in `WithBaseURL`** -- Validate that the supplied URL uses `http` or `https` and optionally enforce a host allowlist or block private CIDRs, closing the `gateway.go:183` TODO. Minimum viable fix: reject non-http/https schemes and document the SSRF risk acceptance for localhost deployments.
   CITE: `litellm.go` L:65, `gateway.go` L:183

---

## Suggested Fixes

### Bug Fixes

- **`buildRequest` silently drops `SystemPrompt`** -- the field is documented at `gateway.go:75` as serving the Anthropic use case; every `Complete` call with a system prompt silently loses it with no error or log.
  CITE: `litellm.go` L:158-177

- **`OllamaAdapter.FormatRequest` does not strip prefix; test asserts preserved prefix** -- `TestOllamaAdapter_FormatRequest_Passthrough` at `provider_test.go:271-273` asserts `got.Model == req.Model` where `req.Model == "ollama/llama3"`, which directly contradicts the acceptance scenario at `litellm-client-provider-adapters-spec.md:208`.
  CITE: `ollama.go` L:23, `provider_test.go` L:271-273

### In-PR Improvements

- **Wire T3.7: adapter lookup inside `Complete`** -- Without this, `FormatAdapter` implementations are never called from the client path.
  CITE: `litellm.go` L:105

- **`Registry.Find` → `(FormatAdapter, error)` returning `ErrNoProvider`** -- Current `(FormatAdapter, bool)` signature makes the sentinel at `errors.go:7` unreachable, violating the exit criterion.
  CITE: `provider.go` L:45-52

- **URL scheme validation in `WithBaseURL`** -- Close the acknowledged `TODO(T2)` security gap.
  CITE: `litellm.go` L:65

### PR Description Amendments

- Note that Open Question Q2 (`litellm-client-provider-adapters-spec.md:24`) -- "Does the Anthropic adapter need to translate `system` messages into the top-level `system` field, or can we rely on LiteLLM?" -- must be explicitly resolved and documented alongside the `SystemPrompt` fix.
- Note that Open Question Q3 (`litellm-client-provider-adapters-spec.md:25`) -- Ollama prefix stripping -- has been resolved in favour of stripping (acceptance scenario is controlling), closing the open question.
- Note that `FormatAdapter` interface diverges from spec's API contract (`map[string]any, error` → `CompletionRequest`); document the rationale (type safety, no meaningful error path) so future contributors understand the intentional deviation.

### Post-Merge Hardening (not merge-blocking)

- **`io.LimitReader` on response body reads** -- `resp.Body` is decoded without a size cap at `litellm.go:131` (error path) and `litellm.go:140` (success path). A compromised or misconfigured LiteLLM proxy returning a multi-GB body will exhaust process memory. Recommended as an in-PR addition alongside the five conditions above, but classified below Critical Discovery threshold (DoS, not OWASP Top 10) and therefore not independently blocking.
  CITE: `litellm.go` L:131, `litellm.go` L:140

- **`Metadata` map structural protection in `FormatAdapter`** -- `out := req` shallow copy shares the underlying `Metadata` map between original and copy (`gateway.go:85`). No current adapter mutates it, but the interface contract at `provider.go:15` ("must not mutate the original request") is not structurally enforced. Add a deep-copy helper or document the constraint explicitly for future adapter authors.
  CITE: `provider.go` L:15, `gateway.go` L:85
