---
## Adversarial Council — Merge PR #45 Gateway Interface & Types T1

> Convened: 2026-03-22 | Advocates: 2 | Critics: 2 | Rounds: 3/4 (called early — converged)

### Motion
Merge PR #45 (feat: Gateway Interface & Types T1) into epic/3-implement-model-gateway-judge-router.

---

### Advocate Positions

**ADVOCATE-1**: PR #45 is merge-ready. Every symbol is a type definition, constant, constructor helper, or interface — no business logic, no I/O, no side effects. ModelTier's `MarshalText`/`UnmarshalText` validates tier values at the serde boundary (`gateway.go:31-46`), catching invalid YAML/JSON config at load time rather than silently at runtime. The temperature negative sentinel (`gateway.go:79-80`) avoids `*float64` pointer boilerplate across T2–T5 without correctness risk. `FallbackError.Unwrap()` returns `ErrAllProvidersFailed` as a documented, intentional layered contract (`errors.go:49-71`). `TestInterfaceComposition` provides a compile-time guarantee that the sub-interfaces compose correctly into `Gateway` (`gateway_test.go:261-265`). Concedes TokenUsage is an orphaned type (cleanup item, not a blocker); holds APIKey masking and BaseURL validation are T2 concerns.

**ADVOCATE-2**: The four interfaces (`gateway.go:194-227`) partition responsibility along failure-domain lines — each represents a distinct failure domain, and none can be broken without the test file failing to compile. `CompletionRequest` (`gateway.go:69-85`) and `CompletionResponse` (`gateway.go:93-104`) are zero-coupling provider-agnostic types; T2 provider adapters require no type changes. By Round 3, ADVOCATE-2 shifted to an explicit conditional merge position: conceded TokenUsage deletion is warranted before merge, conceded `ProviderConfig.String()` masking is a reasonable merge condition, and conceded the `LastNDays` docstring clarification belongs in this PR. Concludes: the motion should carry conditionally on three trivial in-PR fixes.

---

### Critic Positions

**CRITIC-1**: Four findings, two of which were resolved during debate. Remains: (A) `TokenUsage` at `gateway.go:107-110` is unreferenced and should be deleted in this PR — not deferred to a follow-up that may never come, since zero downstream consumers exist to coordinate with. (B) `ProviderConfig.APIKey` at `gateway.go:183-185` establishes an unsafe default: T1 sets the type that T2–T5 inherit, and without a `String()` override, any `fmt.Sprintf("%+v", cfg)` emits the API key verbatim by Go's default formatting — that is a T1 design choice, not a T2 implementation choice. A 3-line `String()` returning a redacted representation costs nothing and makes the safe path the default.

**CRITIC-2**: Three security findings, substantially narrowed during debate. Conceded: OWASP A10:2021 SSRF classification was overstated (operator config ≠ user-controlled input); mandatory interface-level sanitization enforcement for `Router.Classify()` is not viable in Go's type system; the Metadata sink concern was withdrawn. Holds: (E) both advocates confirmed T2 Completer implementations must validate `ProviderConfig.BaseURL` against a host allowlist, but that obligation exists nowhere in the current spec or in any T2 acceptance criteria — it is an untracked security requirement. (F) `gateway.go:53-54` explicitly states "The judge-router uses this (not just Description)" — a commitment to AI model calls as the primary `Payload` use case — yet `Router.Classify()` (`gateway.go:208-211`) carries no godoc noting that LLM-backed implementations are responsible for input validation. One sentence costs nothing. (G — secondary) `gateway_test.go:213` commits `"gateway: Complete: provider anthropic: gateway: provider unavailable"` as a tested, stable string — once locked in a test assertion, an "internal detail" becomes a stable API surface.

---

### Key Conflicts

- **A (TokenUsage timing)** — CRITIC-1 says delete in-PR; ADVOCATE-1 says follow-up commit — PARTIALLY RESOLVED: ADVOCATE-2 conceded "reasonable condition for merge"
- **B (APIKey masking)** — CRITIC-1 and CRITIC-2 say T1 should establish safe default; ADVOCATE-1 says T2 serialization layer is the correct enforcement point — PARTIALLY RESOLVED: ADVOCATE-2 conceded "reasonable condition"
- **C (LastNDays docstring)** — CRITIC-1 holds 1-line fix; advocates acknowledged documentation gap — RESOLVED: ADVOCATE-2 conceded it belongs in this PR before merge
- **D (CostRecord.CacheHit)** — CRITIC-1 initially argued Should-Have omission breaks the stable-contract guarantee; advocates argued backward-compatible struct addition — RESOLVED: CRITIC-1 fully conceded in Round 2
- **E (BaseURL T2 obligation tracking)** — Both advocates confirmed the mitigation must exist; disagreement is only on whether the current PR or a T2 spec note is the right tracking location — RESOLVED as PR description amendment
- **F (Router godoc for prompt injection)** — CRITIC-2 conceded mandatory enforcement is not viable; narrows to a 1-line godoc note — PARTIALLY RESOLVED: advocates contest even the godoc; critic holds it

---

### Concessions

- **ADVOCATE-2** conceded to **CRITIC-1**: TokenUsage deletion is warranted before merge (not a blocker, but a reasonable condition)
- **ADVOCATE-2** conceded to **CRITIC-1**: `ProviderConfig.String()` masking is a reasonable merge condition ("safe by default before T2 inherits the type")
- **ADVOCATE-2** conceded to **CRITIC-1**: `LastNDays` docstring clarification belongs in this PR
- **CRITIC-1** conceded to **ADVOCATE-1**: The "documented as equivalent constructors" framing of `LastNDays(0)` and `Today()` was overstated; spec does not assert equivalence
- **CRITIC-1** conceded to **ADVOCATE-2**: `CostRecord.CacheHit bool` deferral is valid — Should-Have classification supports it, struct field addition is backward-compatible in Go
- **CRITIC-2** conceded to **ADVOCATE-2**: OWASP A10:2021 SSRF classification was overstated; `ProviderConfig.BaseURL` is operator config, not user-controlled input
- **CRITIC-2** conceded to **ADVOCATE-2**: Mandatory interface-level enforcement of sanitization on `Router` is not viable in Go's type system
- **CRITIC-2** conceded to **ADVOCATE-2**: Metadata sink concern withdrawn — enforcement belongs at the `CostTracker.Record()` write path in T4, not the type definition
- **CRITIC-2** partially conceded to **ADVOCATE-1**: `ProviderError` topology disclosure is T5's responsibility to wrap; holds only that the test assertion at `gateway_test.go:213` locks the format as quasi-stable API

---

### Arbiter Recommendation

**CONDITIONAL**

The core architecture of PR #45 is sound: four interfaces with distinct failure domains, provider-agnostic request/response types with zero coupling to T2–T5, validated serde at the ModelTier boundary, a well-documented layered error contract, and substantive tests including a compile-time interface composition guarantee. The debate did not surface any structural defect or design error that would require reworking the PR's architecture. However, the debate converged on four in-PR improvements that became uncontested or near-uncontested by Round 3: ADVOCATE-2 explicitly conceded three of them as reasonable merge conditions, and CRITIC-2's narrowed finding F (Router godoc) is a one-line addition with no architectural implications. All four are cheaper to apply now than after T2–T5 build against this type layer.

---

### Conditions

1. Remove `TokenUsage` struct at `gateway.go:107-110` — both advocates acknowledged it is unreferenced; 4-line deletion
2. Add `func (p ProviderConfig) String() string` returning a redacted representation of `APIKey` near `gateway.go:183-185` — establishes safe logging default before T2 inherits the type; 3 lines, no external imports required
3. Add one-line clarifying note to `LastNDays` docstring at `gateway.go:163-165` stating that `n=0` returns `[midnight, now)`, not `[midnight, midnight+24h)` — use `Today()` for a fixed calendar-day window
4. Add one-line godoc to `Router.Classify()` at `gateway.go:208-211` noting that implementations forwarding `task.Payload` to hosted language models bear responsibility for input validation and length limits

---

### Suggested Fixes

#### Bug Fixes (always in-PR, regardless of original scope)
*(none — no bugs, regressions, or incorrect behaviour identified)*

#### In-PR Improvements (scoped, non-bug)

- **Remove `TokenUsage`** — `gateway.go:107-110` — dead type with no interface, method, return value, or embedding reference; 4-line deletion; both sides agreed on removal, disagreed only on timing
- **Add `ProviderConfig.String()` masking** — `gateway.go:183-185` vicinity — `APIKey string` with no `String()` override causes verbatim key emission under Go's default `%+v` formatting; 3-line redacting `String()` method establishes safe default before T2 config-loading and T5 integration code inherit the type; no external imports required
- **Clarify `LastNDays` docstring** — `gateway.go:163-165` — add one sentence: `n=0` returns `[midnight, now)`, not `[midnight, midnight+24h)`; `Today()` is the correct choice for a fixed calendar-day audit window
- **Add `Router.Classify()` godoc note** — `gateway.go:208-211` — one sentence noting that implementations passing `task.Payload` to hosted language models are responsible for input validation and length limiting; enforces no contract (Go cannot express this at the type level) but creates the documentation obligation at the interface definition site where T3 implementors read first

#### PR Description Amendments

- **Track T2 BaseURL allowlist requirement**: Both advocates confirmed in Round 2/3 that T2 Completer implementations "must validate the host against an allowlist" before constructing HTTP clients from `ProviderConfig.BaseURL` (`gateway.go:182`). This obligation exists nowhere in the current PR description, spec, or T2 acceptance criteria. Add a note to the PR body (or a TODO comment at `gateway.go:182`) stating that T2 HTTP client construction must validate `BaseURL` against a scheme+host allowlist and block private CIDRs.
- **Clarify internal error format scope**: `gateway_test.go:213` asserts `ProviderError.Error()` as a stable string. Add a note to the PR description clarifying this is an internal contract — T5 HTTP handlers must wrap internal errors with external-facing messages and must not return `err.Error()` directly in response bodies.

#### New Issues (future features/enhancements only — confirm with human before creating)
*(none — all findings were either resolved in-PR or narrowed to PR description notes)*
