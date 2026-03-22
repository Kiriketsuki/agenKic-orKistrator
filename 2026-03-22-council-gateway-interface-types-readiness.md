---
## Adversarial Council — Gateway Interface & Types Readiness

> Convened: 2026-03-22 | Advocates: 2 | Critics: 2 | Rounds: 3/4

### Motion
"The current Gateway Interface & Types implementation is sound and ready for downstream sub-components (T2–T9) to build against."

---

### Advocate Positions

**ADVOCATE-1 (Architecture)**: The four deliberate improvements over the spec are non-trivial and correct. `context.Context` on every interface method (`gateway.go:142-149`) is a load-bearing correctness fix — every LLM call is a network operation, and cancellation cannot be retrofitted. `RoutingDecision{Tier, Model, Reason}` (`gateway.go:39-43`) is required by the epic spec's explicit logging scenario (`specs/model-gateway-spec.md:87-89`) — a bare `ModelTier` would have forced T4 to surface routing rationale via a side channel. `Router.Classify()` (`gateway.go:155`) over `Router.Route()` prevents a Go method-set ambiguity when a concrete type simultaneously satisfies `Gateway` (which also has `Route`). `FallbackError{[]ProviderError}` (`errors.go:41-53`) is structurally superior to the spec's single-`Provider` `GatewayError` for multi-provider fallback chains. However, five bounded items must be completed before any downstream task starts: spec sync (hard prerequisite for all nine), `MarshalText`/`UnmarshalText` (T5), `TimePeriod` constructors (T7), `SystemPrompt` on `CompletionRequest` (T3), and acceptance tests (exit criterion). None require changing existing interface signatures or type structures. Final verdict: **CONDITIONAL**.

**ADVOCATE-2 (General/Practical)**: The four interfaces give every downstream implementor an unambiguous, named contract (`gateway.go:139-172`). The error hierarchy is production-grade and immediately usable: `ProviderError` and `FallbackError` let T6 build its retry loop and T9 write assertions without inventing error types. `CompletionResponse.FallbackUsed` and `ProviderName` (`gateway.go:70-73`) make fallback behavior a first-class observable datum. Zero external imports (`gateway.go:3-6`, `errors.go:3`) guarantee no transitive dependency conflicts across all nine sub-packages. Conceded: `MarshalText`/`UnmarshalText`, `TimePeriod` constructors, `SystemPrompt`, `ErrCostTrackerFull`, `CompletionRequest.Stream`/`Tier`, `CostRecord.RequestID`, `TaskSpec.Payload` concern for T4 classification fidelity, spec sync as universal prerequisite, and acceptance tests. Final verdict: **CONDITIONAL** — architectural foundation is sound; seven bounded prerequisites must be satisfied.

---

### Critic Positions

**CRITIC-1 (Architecture/Spec Compliance)**: Conceded context.Context, RoutingDecision, `Classify()` naming, and FallbackError as genuine improvements. Narrowed the yaml.v3 claim (not zero-value but silent invalid tier at unmarshal). Maintained: zero tests leave exit criteria entirely unverified; `MarshalText`/`UnmarshalText`, `TimePeriod` constructors, and `SystemPrompt` are unfulfilled Must-Have commitments; the spec contradicts the implementation on method names, type shapes, and sentinel names with no deprecation notice — and CLAUDE.md designates spec files as "implementation blueprints," making the current spec an active misleading reference for T4/T5/T7 authors. Critically: ADVOCATE-1 conceded spec sync as a hard prerequisite for all downstream tasks, meaning even the supposedly unblocked T2/T4/T6 cannot safely begin until the spec is updated. `TaskSpec.Payload`'s silent removal is an unresolved architectural decision affecting T4's classification fidelity. Final verdict: **AGAINST** the motion as literally worded; CONDITIONAL is the accurate practical framing.

**CRITIC-2 (Scope/Delivery Risk)**: Conceded context.Context, `Classify()` naming, `RoutingDecision`, and `FallbackError` for multi-provider chains. Narrowed: `ErrNoRouteFound` is a spec-governance issue, not a compile hazard. Maintained: zero tests (T1-j pending, exit criteria unverified), `MarshalText`/`UnmarshalText` (T5 directly blocked), `TimePeriod` constructors (T7 directly blocked; if T7 defines them instead, it creates a circular import dependency for any other consumer of `Today()`), `TaskSpec.Payload` (T4 judge-router needs content, not just a description label, to reliably classify complexity), spec-governance gap (stale spec with no deprecation marker actively misleads contributors), `Op` field absent from all error types for single-operation failures (loses operation context in error logs). Submitted a 7-item binding prerequisite list. Final verdict: **CONDITIONAL** — architectural core is sound, but the contract is not yet frozen enough to qualify as "ready" by the motion's own scope (T2-T9, all of them).

---

### Key Conflicts

- **yaml.v3 zero-value claim** — CRITIC-1 initially claimed yaml.v3 silently zero-values `ModelTier` without `TextUnmarshaler`. ADVOCATE-1 challenged: yaml.v3 uses reflection, named string types populate correctly; what's missing is validation at unmarshal time. CRITIC-1 conceded. — **Resolved**: failure mode is "silent invalid tier passes unmarshal," not silent zero-value. Lower severity than initially stated; gap still real.

- **CONDITIONAL vs AGAINST binary** — CRITIC-1 argued the motion is binary and CONDITIONAL equals AGAINST; CRITIC-2 accepted CONDITIONAL as the accurate label while agreeing the motion as literally worded fails. — **Resolved**: CONDITIONAL is the operative verdict. CRITIC-1's binary observation is noted — if the motion required a simple pass/fail, it would fail.

- **TaskSpec.Payload functional necessity** — CRITIC-2 argued a judge-router classifying by `Description` label alone cannot reliably distinguish task complexity. ADVOCATE-2 partially conceded the functional distinction (label vs. full content). ADVOCATE-1 framed it as a T4 design question. — **Unresolved**: whether `Description string` suffices for T4's classification logic depends on T4's design intent, which does not yet exist. Flagged as requiring explicit resolution before T4 starts.

- **ErrCostTrackerFull design intent** — CRITIC-2 cited it as a missing sentinel; ADVOCATE-1 questioned whether the Redis Streams design eliminates the capacity constraint the sentinel was predicated on. — **Unresolved**: debated design intention. Flagged for explicit documentation in T1's spec update.

- **"Additive changes don't destabilize" vs. "incomplete contracts produce incomplete work"** — ADVOCATE-1 argued MarshalText, TimePeriod constructors, and SystemPrompt are additive and T3/T5/T7 can absorb them without rewriting core logic. CRITIC-2 countered that T3 embedding the assumption "all turns fit in Messages[]" will produce behaviorally wrong Anthropic adapter code until SystemPrompt lands — bounded rework, not zero cost. — **Partially resolved**: advocates accepted the rework is nonzero; critics accepted the rework is bounded. The disagreement is scope framing, not technical substance.

---

### Concessions

**Critics conceded to Advocates:**
- CRITIC-1, CRITIC-2 conceded: `context.Context` on all interface methods is a spec defect fixed correctly — every LLM call is a network operation requiring cancellation propagation
- CRITIC-1, CRITIC-2 conceded: `RoutingDecision` over bare `ModelTier` is architecturally richer and serves the epic spec's explicit logging requirement
- CRITIC-1, CRITIC-2 conceded: `Router.Classify()` naming prevents Go method-set ambiguity when embedding Router in a Gateway implementation
- CRITIC-1, CRITIC-2 conceded: `FallbackError{[]ProviderError}` is structurally superior to the spec's single-`Provider` `GatewayError` for multi-provider fallback chains
- CRITIC-1 conceded: yaml.v3 zero-value claim was overstated; narrowed to "silent invalid tier passes unmarshal"
- CRITIC-2 conceded (narrow): `ErrNoRouteFound` absence is a spec-governance issue, not a compile-time hazard for unwritten downstream code

**Advocates conceded to Critics:**
- ADVOCATE-1, ADVOCATE-2 conceded: zero tests — T1-j and T1-k are pending; exit criteria are unmet
- ADVOCATE-1 conceded: `MarshalText`/`UnmarshalText` are absent; failure mode is silent invalid tier at unmarshal (not zero-value); Must-Have gap, T5 prerequisite
- ADVOCATE-1 conceded: `TimePeriod` constructors belong in `internal/gateway`; if T7 defines `Today()` instead, any other package needing it must import T7, creating a circular dependency
- ADVOCATE-1 conceded: `SystemPrompt` requires editing T1's file, not a T3 workaround
- ADVOCATE-1, ADVOCATE-2 conceded: spec sync is a hard prerequisite before any T2-T9 author opens the spec; CLAUDE.md designates spec files as implementation blueprints
- ADVOCATE-2 conceded: `ErrCostTrackerFull` is absent from `errors.go`
- ADVOCATE-2 conceded: `CompletionRequest.Stream bool` and `Tier ModelTier` are missing Must-Have fields
- ADVOCATE-2 conceded: `TaskSpec.Payload` removal raises a real concern for T4's classification fidelity requiring explicit resolution
- ADVOCATE-1 conceded: `Op` context is absent from all error types — neither `ProviderError` (`errors.go:29-38`) nor `FallbackError` (`errors.go:41-53`) carries an `Op string` field; single-operation failures lose operation context in error logs

---

### Arbiter Recommendation

**CONDITIONAL**

The architectural foundation is non-trivially sound: four improvements over the spec were conceded by both critics and represent correct engineering decisions that would be costly to undo — context threading, richer routing return, composition-safe naming, and aggregated fallback diagnostics. The zero-dependency guarantee is verified. These decisions should not be reverted.

However, the implementation fails five stated exit criteria. The spec contradicts the implementation on method names, type field names, sentinel names, and interface shapes with no deprecation notice, making it an active misleading reference in a codebase where CLAUDE.md designates spec files as implementation blueprints. Three downstream tasks (T3, T5, T7) are directly blocked by missing Must-Have items; and both advocates conceded that spec sync is a prerequisite before any of the nine downstream tasks formally begins. The conditions are bounded, additive, and require no changes to existing interface method signatures (`gateway.go:140-172`).

---

### Conditions

The following must be satisfied — in the order listed — before T1 is declared done and before any T2-T9 task formally begins:

1. **Spec sync** *(gates all downstream work)*: Update `gateway-interface-types-spec.md` to reflect the actual implementation — `Router.Classify()` method name, `RoutingDecision` return type, `Valid()` instead of `IsValid()`, actual sentinel names (`ErrNoProvider`, `ErrAllProvidersFailed`, `ErrClassificationFailed`, `ErrRateLimited`, `ErrConfigInvalid`), actual type field names (`TierCosts`, `TierCostSummary`, `EstimatedCost`, `Total CostSummary`), and all four open questions marked `[x]` resolved with implementation choices documented.

2. **`MarshalText`/`UnmarshalText` on `ModelTier`** *(T5 prerequisite; hard exit criterion)*: Without these, an invalid tier string in `models.yaml` passes unmarshal silently and is only caught — if at all — when `Valid()` is called downstream, potentially after the gateway has started serving requests.

3. **`TimePeriod.Today()`, `LastNDays(n int)`, `Since(t time.Time)`** *(T7 prerequisite; hard exit criterion)*: Constructors must live in `internal/gateway`. If T7 defines them, any other consumer (T9 integration tests) importing `Today()` must import T7 — creating a circular dependency.

4. **`CompletionRequest.SystemPrompt string`** *(T3 prerequisite)*: Anthropic's API separates system prompts from the message array. Without this field, T3's Anthropic adapter embeds the incorrect assumption that all turns fit in `Messages []Message`; correcting it after T3 ships is bounded but nonzero rework.

5. **`CompletionRequest.Stream bool` and `Tier ModelTier`** *(T3/T4 prerequisite; Must-Have spec fields)*: `Stream bool` gates LiteLLM streaming dispatch; `Tier ModelTier` allows the cost tracker to receive tier context at the point of the call rather than out-of-band.

6. **`CostRecord.RequestID string`** *(T7 prerequisite)*: T7 requires a stable key for deduplication when Redis Streams may deliver duplicate events.

7. **`ErrCostTrackerFull` sentinel or explicit design decision** *(T7 prerequisite)*: Either add the sentinel, or document in the spec update that the Redis Streams design eliminates the fixed-capacity constraint and the sentinel was intentionally omitted.

8. **`TaskSpec.Payload` resolution** *(T4 prerequisite)*: Either restore `Payload string` (and optionally `Metadata map[string]string`) to `TaskSpec` at `gateway.go:31-36`, or explicitly document in T4's spec why `Description string` alone is sufficient for the judge-router to reliably classify task complexity. This is a functional concern — a classifier receiving only a label cannot distinguish "complex task with long content" from "simple task with long content."

9. **Acceptance test suite (T1-j, T1-k)** *(hard exit criterion; verifies all above are correct)*: All acceptance scenarios from `gateway-interface-types-spec.md:214-283` must pass as unit tests. T1-k (`go list -deps ./internal/gateway/`) must confirm zero external imports.

---

### Suggested Fixes

#### Bug Fixes (always in-PR, regardless of original scope)
*None identified.* All findings are missing features or spec-compliance gaps. No behavioral bugs, incorrect computations, security flaws, or data-corruption defects were established.

#### In-PR Improvements (scoped, non-bug)

- **Add `MarshalText()`/`UnmarshalText()` to `ModelTier`** — `internal/gateway/gateway.go` (after line 28) — implements `encoding.TextMarshaler`/`encoding.TextUnmarshaler`; enables validation-at-decode for YAML and JSON; required exit criterion.

- **Add `Today()`, `LastNDays(n int)`, `Since(t time.Time)` package-level functions** — `internal/gateway/gateway.go` (after line 122) — required exit criterion; prevents circular import if defined in T7 instead.

- **Add `SystemPrompt string` to `CompletionRequest`** — `internal/gateway/gateway.go:46-55` — T3 prerequisite; Anthropic adapter requires system-prompt separation from message list.

- **Add `Stream bool` and `Tier ModelTier` to `CompletionRequest`** — `internal/gateway/gateway.go:46-55` — Must-Have spec fields; `Stream` gates LiteLLM streaming; `Tier` passes routing context to cost tracker at call site.

- **Add `RequestID string` to `CostRecord`** — `internal/gateway/gateway.go:83-92` — T7 deduplication key for Redis Streams replay scenarios.

- **Add `ErrCostTrackerFull` sentinel** — `internal/gateway/errors.go` — T7 prerequisite; or document in spec update that the Redis design makes this sentinel inapplicable.

- **Add `Op` context to error types** — `internal/gateway/errors.go` — Either add `Op string` to `ProviderError` or introduce a minimal `OpError` wrapper (`Op string`, `Err error`). Without it, single-operation failures (`Route`, `GetCostReport`) lose operation context in error logs; callers wanting typed operation context must parse error strings.

- **Resolve `TaskSpec.Payload`** — `internal/gateway/gateway.go:31-36` — Either restore `Payload string` (and `Metadata map[string]string`) from the spec, or explicitly document in T4's spec why `Description string` alone is sufficient for classifier fidelity.

- **Write acceptance test suite** — `internal/gateway/gateway_test.go` (new file) — Tests for: `ModelTier` serde round-trip (YAML + JSON), `Valid()` on valid and invalid tiers, `TimePeriod` constructor boundary conditions (UTC), `GatewayError`-equivalent unwrap via `errors.Is`/`errors.As`, and `FallbackError` aggregation. Required exit criterion T1-j.

#### PR Description Amendments

- **Document deliberate spec divergences**: Enumerate each implementation decision that consciously improved on the spec: context.Context on all interface methods (spec defect corrected), `RoutingDecision` return type (richer than `ModelTier`, required for routing-decision logging per epic spec), `Router.Classify()` method name (Go composition safety), `ProviderError`+`FallbackError` replacing `GatewayError` (superior for multi-provider chain diagnostics), `Valid()` instead of `IsValid()` (stdlib convention), actual sentinel names replacing spec names.

- **Mark all four open questions resolved**: Q1 (typed string chosen over int enum — readable in logs and YAML), Q2 (`[]Message` chosen — OpenAI/Anthropic compatible; `SystemPrompt` field handles Anthropic separation), Q3 (float64 chosen — note drift risk over large volumes; T7 must apply `math.Round(x*10000)/10000` in aggregation), Q4 (`RoutingDecision.Reason string` satisfies the routing rationale requirement).

- **Declare `gateway.go` authoritative**: Add a note that `gateway.go` is the authoritative type reference for all downstream tasks; `gateway-interface-types-spec.md` sections superseded by implementation decisions are enumerated in the spec sync (Condition 1).

#### New Issues (future features/enhancements only — confirm with human before creating)
*None.* All identified gaps are in-scope for the current T1 PR under the conditions above.

---

*Council convened: 2026-03-22*
*Arbiter: Claude (claude-sonnet-4-6)*
*Rounds conducted: 3 of 4*
*Convergence basis: Both advocates endorsed CONDITIONAL; both critics accepted CONDITIONAL as the practical verdict (CRITIC-1 noting it equals AGAINST for the motion as literally worded). No new ground covered in final exchanges.*
