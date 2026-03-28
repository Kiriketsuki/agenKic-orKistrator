# Feature Spec: Judge-Router (T4) — Refined

**Feature**: Judge-Router — task complexity classification and model tier routing
**Parent epic**: Model Gateway (`specs/model-gateway-spec.md`)
**GitHub issue**: #34
**PR**: #47
**Go package**: `internal/gateway`
**Primary file**: `internal/gateway/router.go`

---

## Overview

**User Story**:
As an orchestrator operator, I want tasks to be automatically classified by complexity and routed to the cheapest capable model so that I get optimal cost-performance without manually annotating every task.

**Problem**:
Without intelligent routing, every task is sent to a frontier model (Opus-class), wasting 60-90% of potential cost savings. Simple tasks — formatting, summarising, short lookups — do not require deep reasoning. A lightweight judge model can classify task complexity in a single call for a few tenths of a cent, then route the actual work to the right tier. This pattern (judge-then-work) delivers frontier-quality outcomes on complex tasks while running cheap models on everything else.

**Out of Scope**:
- LiteLLM proxy client implementation (T2, T3) — the JudgeRouter depends on the `Completer` interface, not the concrete LiteLLM client.
- Config loading for tier-to-model mapping (T5) — the router classifies into tiers; config maps tiers to concrete models.
- Fallback chain logic on provider failure (T6) — a separate concern handled by the gateway layer.
- Cost tracking (T7) — recorded by the gateway after routing decisions are made.
- Agent lifecycle or supervisor integration (T8).
- Classification failure metrics/alerting — deferred to gateway layer (T7/T8).
- Feedback loop for tuning classification — deferred to T7; `RawResponse` field enables future analysis.
- Latency-budget skip mechanism — `OverrideTier` is sufficient for callers who know the tier.

---

## Success Condition

> This feature is complete when `JudgeRouter` passes all acceptance tests with `RawResponse` populated on successful classification, structured logging via `log/slog`, and a configurable classification prompt via `WithClassificationPrompt`.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should the judge call count against the task's token budget, or be billed separately? | spec author | [x] Deferred to T7 (cost tracker) — not a router concern |
| 2 | What is the acceptable latency budget for the judge call? If judge adds >50ms, should it be skipped for latency-sensitive paths? | spec author | [x] Deferred — `OverrideTier` covers known-tier cases; no `SkipJudge` needed |
| 3 | Should we collect a feedback signal (human override rate) to tune the classification prompt over time? | spec author | [x] `RawResponse` field added to `RoutingDecision` for future analysis; full feedback loop deferred to T7 |
| 4 | Is `OverrideTier` the only exception path, or should we support per-task `SkipJudge bool` for callers that already know the tier? | spec author | [x] `OverrideTier` is sufficient — no `SkipJudge` field |
| 5 | Should classification failures increment a metric / alert, or are they silently absorbed into the default tier? | spec author | [x] Deferred to T7/T8 — gateway layer wraps `Classify` and adds observability |

---

## Scope

### Must-Have

- **`JudgeRouter` struct** implementing the `Router` interface with a `Classify(ctx, TaskSpec) (RoutingDecision, error)` method.
- **Three routing tiers**: `cheap`, `mid`, `frontier` — defined as `ModelTier` constants in `gateway.go`.
- **Judge model call**: send a structured classification prompt to a Haiku-class model via the `Completer` interface; parse the single-word response into a `ModelTier`.
- **`OverrideTier` fast path**: if `TaskSpec.OverrideTier` is set and valid, return it immediately without calling the judge model.
- **Fallback to default tier**: if the judge call fails (network error, timeout) or returns an unrecognised response, log a warning and return `defaultTier` — never surface an error to the caller.
- **Routing decision logging**: every `Classify` call must log the task ID, the resulting tier, and the human-readable reason (classification, override, or fallback) via `log/slog` structured logging.
- **Functional options**: `WithJudgeModel(string)`, `WithDefaultTier(ModelTier)`, `WithCompleter(Completer)`, `WithClassificationPrompt(string)` — allow callers to swap any field without subclassing.
- **`NewJudgeRouter(opts ...RouterOption) *JudgeRouter`** constructor with sensible defaults (`judgeModel = "claude-haiku-4-5-20251001"`, `defaultTier = TierMid`, `classificationPrompt = <built-in>`).
- **`RawResponse` field** on `RoutingDecision` — populated with the raw judge model output on successful classification; empty on override, nil-completer, or error paths.
- **Unit tests** covering: simple task -> cheap, complex task -> frontier, override bypass, classification failure fallback, garbage response fallback, nil completer fallback, case-insensitive tier parsing, empty string fallback, whitespace-padded response, override-cheap-on-complex-task, `RawResponse` population assertions.

### Should-Have

- Structured log output via `log/slog` (key=value) instead of plain `log.Printf` to make routing decisions queryable in production.
- Classification prompt tunable via `WithClassificationPrompt(string)` so operators can refine tier descriptions without recompiling. Prompt must include `%s` for task description injection via `fmt.Sprintf`.

### Nice-to-Have

- Confidence score returned alongside the tier (e.g. from a logprobs call) to flag borderline classifications for human review.
- Batch classification: accept `[]TaskSpec` and issue a single judge call for multiple tasks, reducing per-task overhead.

---

## Technical Plan

### Affected Components

| File | Change |
|:-----|:-------|
| `internal/gateway/gateway.go` | Add `RawResponse string` field to `RoutingDecision` struct |
| `internal/gateway/router.go` | Add `classificationPrompt` field to `JudgeRouter`; add `WithClassificationPrompt` option; move prompt from const to field default; populate `RawResponse` in `Classify`; replace `log.Printf` with `log/slog` |
| `internal/gateway/router_test.go` | Add test cases: empty string, whitespace-padded, override-cheap-on-complex; add `RawResponse` assertions to all existing tests |

### Data Model Changes

```go
// RoutingDecision — one new field
type RoutingDecision struct {
    Tier        ModelTier
    Model       string
    Reason      string
    RawResponse string // raw judge model output; empty on override/fallback
}
```

### API Contracts

```go
// JudgeRouter — one new field (classificationPrompt)
type JudgeRouter struct {
    completer            Completer
    judgeModel           string
    defaultTier          ModelTier
    classificationPrompt string // fmt.Sprintf pattern with %s for task.Description
}

// New option
func WithClassificationPrompt(prompt string) RouterOption

// NewJudgeRouter defaults:
//   judgeModel:           "claude-haiku-4-5-20251001"
//   defaultTier:          TierMid
//   classificationPrompt: <built-in prompt>
```

### Classification Prompt Contract

The judge model receives a structured prompt that constrains output to a single word. This makes parsing trivial and avoids hallucinated explanations leaking into the routing path.

```
You are a task complexity classifier. Classify the following task into exactly one of three tiers.

Respond with ONLY one word — no punctuation, no explanation:
- "cheap"    — simple lookups, formatting, summarizing short text, straightforward code fixes
- "mid"      — moderate analysis, code generation, multi-step reasoning
- "frontier" — complex architecture, novel problem-solving, long-form creative work

Task: %s
```

Operators can replace this via `WithClassificationPrompt(string)`. The replacement must include `%s` for task description injection.

Completion parameters for the judge call:
- `MaxTokens: 10` — a single word needs at most 2 tokens; cap tightly to prevent verbose output.
- `Temperature: 0` — deterministic classification; we want the same task to land in the same tier on every call.

### Key Data Flow

```
Classify(ctx, task)
  |-- task.OverrideTier.Valid()? -> slog.Info + return RoutingDecision{Tier: override, Reason: "override: ...", RawResponse: ""}
  |-- r.completer == nil? -> slog.Warn + return RoutingDecision{Tier: defaultTier, Reason: "no completer ...", RawResponse: ""}
  |-- completer.Complete(ctx, judgeRequest)
  |    |-- err != nil -> slog.Warn + return RoutingDecision{Tier: defaultTier, Reason: "judge call failed ...", RawResponse: ""}
  |    +-- ok -> parseTier(resp.Content)
  |         |-- valid tier -> slog.Info + return RoutingDecision{Tier: tier, Model: resp.Model, Reason: "judge classified as X", RawResponse: resp.Content}
  |         +-- invalid -> slog.Warn + return RoutingDecision{Tier: defaultTier, Reason: "unrecognised ...", RawResponse: resp.Content}
  +-- (never returns non-nil error)
```

**Note**: `RawResponse` is populated even on unrecognised-response fallback (the raw garbage is useful for debugging). It is only empty on override, nil-completer, and completer-error paths where no response exists.

### Dependencies

| Dependency | Why | Already in repo |
|:-----------|:----|:----------------|
| `Completer` interface (`gateway.go`) | Judge model calls go through the same abstraction as real completions | Yes (T1) |
| `ModelTier`, `TaskSpec`, `RoutingDecision` types (`gateway.go`) | Core vocabulary types | Yes (T1) |
| `context` (stdlib) | Cancellation/deadline propagation | Yes |
| `fmt`, `strings` (stdlib) | Prompt formatting, response parsing | Yes |
| `log/slog` (stdlib) | Structured logging — replaces `log` | New import |

No external libraries required.

### Risks

| Risk | Likelihood | Impact | Mitigation |
|:-----|:-----------|:-------|:-----------|
| Judge misclassifies task complexity | Medium | Medium — wrong tier selected, cost/quality impact | Log every routing decision with reason; `OverrideTier` provides escape hatch; prompt can be tuned via `WithClassificationPrompt` without code changes |
| Judge call adds latency to every task | Medium | Low — Haiku is fast (~200ms p50); total overhead is small relative to the work model | Document expected overhead; `OverrideTier` provides bypass for latency-critical paths |
| Judge returns multi-word or verbose response despite prompt constraints | Low | Low — `parseTier` handles by falling back to default | `parseTier` normalises with `strings.TrimSpace` + `strings.ToLower` before matching |
| `nil` completer causes panic | Low | High — unrecoverable | `Classify` guards `r.completer == nil` before any call; returns default tier, never panics |
| Operator supplies `WithClassificationPrompt` without `%s` placeholder | Low | Medium — task description never injected | Documented in option godoc; runtime: `fmt.Sprintf` safely ignores missing verb |

---

## Acceptance Scenarios

```gherkin
Feature: Judge-Router task complexity classification
  As an orchestrator operator
  I want tasks automatically classified and routed to the cheapest capable model
  So that I achieve cost savings without manual per-task annotations

  Background:
    Given a JudgeRouter configured with:
      | option       | value                       |
      | judgeModel   | claude-haiku-4-5-20251001   |
      | defaultTier  | mid                         |

  Rule: Simple tasks route to cheap tier

    Scenario: Summarise short error message routes to cheap
      Given the judge model will respond "cheap"
      And a task with description "Summarize this 3-line error message"
      When Classify is called
      Then the routing decision tier is "cheap"
      And the routing decision reason contains "classified as cheap"
      And the routing decision RawResponse is "cheap"
      And the completer was called exactly once

    Scenario: Format JSON blob routes to cheap
      Given the judge model will respond "cheap"
      And a task with description "Format this JSON blob to be pretty-printed"
      When Classify is called
      Then the routing decision tier is "cheap"
      And the routing decision RawResponse is "cheap"

  Rule: Complex tasks route to frontier tier

    Scenario: Design distributed caching architecture routes to frontier
      Given the judge model will respond "frontier"
      And a task with description "Design a distributed caching architecture for a globally distributed system"
      When Classify is called
      Then the routing decision tier is "frontier"
      And the routing decision reason contains "classified as frontier"
      And the routing decision RawResponse is "frontier"
      And the completer was called exactly once

  Rule: Mid-complexity tasks route to mid tier

    Scenario: Code generation task routes to mid
      Given the judge model will respond "mid"
      And a task with description "Generate a Go HTTP handler that validates a JSON payload against a schema"
      When Classify is called
      Then the routing decision tier is "mid"
      And the routing decision reason contains "classified as mid"
      And the routing decision RawResponse is "mid"

  Rule: OverrideTier bypasses classification entirely

    Scenario: OverrideTier set to frontier skips judge call
      Given a task with description "Summarize this short message" and OverrideTier "frontier"
      When Classify is called
      Then the routing decision tier is "frontier"
      And the routing decision reason contains "override"
      And the routing decision RawResponse is ""
      And the completer was called exactly zero times

    Scenario: OverrideTier set to cheap skips judge call regardless of complexity
      Given a task with description "Design a distributed system" and OverrideTier "cheap"
      When Classify is called
      Then the routing decision tier is "cheap"
      And the routing decision reason contains "override"
      And the routing decision RawResponse is ""
      And the completer was called exactly zero times

  Rule: Classification failure falls back to default tier without surfacing an error

    Scenario: Judge call returns network error falls back to default
      Given the completer will return an error "network timeout"
      And a task with description "do something"
      When Classify is called
      Then Classify returns no error
      And the routing decision tier is "mid"
      And the routing decision reason contains "falling back to default tier"
      And the routing decision reason contains "network timeout"
      And the routing decision RawResponse is ""

    Scenario: Judge call returns HTTP 500 falls back to default
      Given the completer will return an error "HTTP 500 Internal Server Error"
      And a task with description "do something"
      When Classify is called
      Then Classify returns no error
      And the routing decision tier is "mid"
      And the routing decision reason contains "falling back to default tier"
      And the routing decision RawResponse is ""

  Rule: Unrecognised judge response falls back to default tier

    Scenario: Judge returns garbage text falls back to default
      Given the judge model will respond "bananas"
      And a task with description "do something"
      When Classify is called
      Then Classify returns no error
      And the routing decision tier is "mid"
      And the routing decision reason contains "unrecognised response"
      And the routing decision reason contains "bananas"
      And the routing decision RawResponse is "bananas"

    Scenario: Judge returns empty string falls back to default
      Given the judge model will respond ""
      And a task with description "do something"
      When Classify is called
      Then Classify returns no error
      And the routing decision tier is "mid"
      And the routing decision reason contains "falling back to default tier"
      And the routing decision RawResponse is ""

    Scenario: Judge response is case-insensitive — "CHEAP" parses as cheap
      Given the judge model will respond "CHEAP"
      And a task with description "format a file"
      When Classify is called
      Then the routing decision tier is "cheap"
      And the routing decision RawResponse is "CHEAP"

    Scenario: Judge response with surrounding whitespace — "  frontier  " parses as frontier
      Given the judge model will respond "  frontier  "
      And a task with description "design a system"
      When Classify is called
      Then the routing decision tier is "frontier"
      And the routing decision RawResponse is "  frontier  "

  Rule: Nil completer uses default tier without panicking

    Scenario: No completer configured returns default tier
      Given a JudgeRouter with no completer configured
      And defaultTier is "cheap"
      And a task with description "anything"
      When Classify is called
      Then Classify returns no error
      And the routing decision tier is "cheap"
      And the routing decision reason contains "no completer configured"
      And the routing decision RawResponse is ""

  Rule: Custom classification prompt

    Scenario: Custom prompt is used for classification
      Given a JudgeRouter configured with WithClassificationPrompt "Rate this: %s\nAnswer: cheap/mid/frontier"
      And the judge model will respond "frontier"
      And a task with description "build a spaceship"
      When Classify is called
      Then the completer received a prompt containing "Rate this: build a spaceship"
      And the routing decision tier is "frontier"
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status |
|:-----|:-----|:---------|:-------------|:-------|
| T4.A | Add `RawResponse string` field to `RoutingDecision` in `gateway.go` | High | None | pending |
| T4.B | Add `classificationPrompt` field to `JudgeRouter` struct; move const to field default in `NewJudgeRouter`; add `WithClassificationPrompt` option | High | None | pending |
| T4.C | Populate `RawResponse` in all `Classify` paths (non-empty on success and unrecognised; empty on override/nil/error) | High | T4.A | pending |
| T4.D | Replace `log.Printf` with `log/slog` structured logging in all `Classify` paths | High | None | pending |
| T4.E | Add test cases: empty string response, whitespace-padded, override-cheap-on-complex, custom prompt, `RawResponse` assertions on all existing tests | High | T4.A, T4.B, T4.C, T4.D | pending |
| T4.F | Update `judge-router-spec.md` with resolved open questions and refined scope (this document) | High | None | done |

---

## Exit Criteria

- [ ] `JudgeRouter` implements `Router` interface — confirmed by `var _ Router = (*JudgeRouter)(nil)` compile-time assertion.
- [ ] `go test ./internal/gateway/...` passes with zero failures.
- [ ] All Gherkin scenarios above have a corresponding test case in `router_test.go`.
- [ ] `Classify` never returns a non-nil error — all failure modes result in `defaultTier` with a logged reason.
- [ ] `OverrideTier` set to any valid `ModelTier` results in zero calls to the `Completer`.
- [ ] `parseTier` correctly normalises uppercase, mixed-case, and whitespace-padded judge responses.
- [ ] `NewJudgeRouter()` with no options produces a router with `judgeModel = "claude-haiku-4-5-20251001"`, `defaultTier = TierMid`, and built-in classification prompt.
- [ ] Code review: no direct import of concrete LiteLLM client — only `Completer` interface used.
- [ ] `RawResponse` is populated with raw judge output on successful classification and unrecognised response; empty on override, nil-completer, and error paths.
- [ ] All log output uses `log/slog` structured logging (no `log.Printf` calls remain in `router.go`).
- [ ] `WithClassificationPrompt` option successfully overrides the default prompt.

---

## References

- Epic spec: `specs/model-gateway-spec.md`
- GitHub issue: #34
- GitHub PR: #47
- Gateway interface and type definitions: `internal/gateway/gateway.go`
- Multi-model coordination research: `docs/research/patterns/Multi-Model-Coordination.md`

---

*Authored by: Clault KiperS 4.6*
