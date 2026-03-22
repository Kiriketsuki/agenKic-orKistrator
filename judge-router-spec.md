# Feature Spec: Judge-Router (T4)

**Feature**: Judge-Router — task complexity classification and model tier routing
**Parent epic**: Model Gateway (`model-gateway-spec.md`)
**GitHub issue**: #34
**Go package**: `internal/gateway`
**Primary file**: `internal/gateway/router.go`

---

## Overview

**User Story**:
As an orchestrator operator, I want tasks to be automatically classified by complexity and routed to the cheapest capable model so that I get optimal cost-performance without manually annotating every task.

**Problem**:
Without intelligent routing, every task is sent to a frontier model (Opus-class), wasting 60–90% of potential cost savings. Simple tasks — formatting, summarising, short lookups — do not require deep reasoning. A lightweight judge model can classify task complexity in a single call for a few tenths of a cent, then route the actual work to the right tier. This pattern (judge-then-work) delivers frontier-quality outcomes on complex tasks while running cheap models on everything else.

**Out of Scope**:
- LiteLLM proxy client implementation (T2, T3) — the JudgeRouter depends on the `Completer` interface, not the concrete LiteLLM client.
- Config loading for tier-to-model mapping (T5) — the router classifies into tiers; config maps tiers to concrete models.
- Fallback chain logic on provider failure (T6) — a separate concern handled by the gateway layer.
- Cost tracking (T7) — recorded by the gateway after routing decisions are made.
- Agent lifecycle or supervisor integration (T8).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should the judge call count against the task's token budget, or be billed separately? | spec author | [ ] |
| 2 | What is the acceptable latency budget for the judge call? If judge adds >50ms, should it be skipped for latency-sensitive paths? | spec author | [ ] |
| 3 | Should we collect a feedback signal (human override rate) to tune the classification prompt over time? | spec author | [ ] |
| 4 | Is `OverrideTier` the only exception path, or should we support per-task `SkipJudge bool` for callers that already know the tier? | spec author | [ ] |
| 5 | Should classification failures increment a metric / alert, or are they silently absorbed into the default tier? | spec author | [ ] |

---

## Scope

### Must-Have
- **`JudgeRouter` struct** implementing the `Router` interface with a `Classify(ctx, TaskSpec) (RoutingDecision, error)` method.
- **Three routing tiers**: `cheap`, `mid`, `frontier` — defined as `ModelTier` constants in `gateway.go`.
- **Judge model call**: send a structured classification prompt to a Haiku-class model via the `Completer` interface; parse the single-word response into a `ModelTier`.
- **`OverrideTier` fast path**: if `TaskSpec.OverrideTier` is set and valid, return it immediately without calling the judge model.
- **Fallback to default tier**: if the judge call fails (network error, timeout) or returns an unrecognised response, log a warning and return `defaultTier` — never surface an error to the caller.
- **Routing decision logging**: every `Classify` call must log the task ID, the resulting tier, and the human-readable reason (classification, override, or fallback).
- **Functional options**: `WithJudgeModel(string)`, `WithDefaultTier(ModelTier)`, `WithCompleter(Completer)` — allow callers to swap any field without subclassing.
- **`NewJudgeRouter(opts ...RouterOption) *JudgeRouter`** constructor with sensible defaults (`judgeModel = "claude-haiku-4-5-20251001"`, `defaultTier = TierMid`).
- **Unit tests** covering: simple task → cheap, complex task → frontier, override bypass, classification failure fallback, garbage response fallback, nil completer fallback, case-insensitive tier parsing.

### Should-Have
- Classification prompt tunable via an option (`WithClassificationPrompt(string)`) so operators can refine tier descriptions without recompiling.
- Structured log output (key=value or JSON) instead of plain `log.Printf` to make routing decisions queryable in production.

### Nice-to-Have
- Confidence score returned alongside the tier (e.g. from a logprobs call) to flag borderline classifications for human review.
- Batch classification: accept `[]TaskSpec` and issue a single judge call for multiple tasks, reducing per-task overhead.

---

## Technical Plan

### Affected Components

| File | Change |
|:-----|:-------|
| `internal/gateway/router.go` | New file — `JudgeRouter`, `RouterOption`, `NewJudgeRouter`, `Classify`, `parseTier` |
| `internal/gateway/gateway.go` | Already defines `Router`, `Completer`, `TaskSpec`, `RoutingDecision`, `ModelTier` — no changes needed |
| `internal/gateway/router_test.go` | New file — unit tests for `JudgeRouter` |

### API Contracts

```go
// Router is defined in gateway.go — reproduced here for clarity.
type Router interface {
    Classify(ctx context.Context, task TaskSpec) (RoutingDecision, error)
}

// Completer is defined in gateway.go — the judge router depends on this,
// not on any concrete LiteLLM client.
type Completer interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    Provider() string
}

// RouterOption is a functional option for JudgeRouter configuration.
type RouterOption func(*JudgeRouter)

// Constructors and options.
func NewJudgeRouter(opts ...RouterOption) *JudgeRouter
func WithJudgeModel(model string) RouterOption
func WithDefaultTier(tier ModelTier) RouterOption
func WithCompleter(c Completer) RouterOption

// JudgeRouter — exported fields are intentionally unexported; configured via options.
type JudgeRouter struct {
    completer   Completer
    judgeModel  string
    defaultTier ModelTier
}

// Classify implements Router.
// Fast path: if task.OverrideTier is valid, return it immediately.
// Normal path: send classificationPrompt to judgeModel via completer,
//              parse single-word response into ModelTier.
// Failure path: any error or unrecognised response → return defaultTier, never error.
func (r *JudgeRouter) Classify(ctx context.Context, task TaskSpec) (RoutingDecision, error)
```

### Classification Prompt Contract

The judge model receives a structured prompt that constrains output to a single word. This
makes parsing trivial and avoids hallucinated explanations leaking into the routing path.

```
You are a task complexity classifier. Classify the following task into exactly one of three tiers.

Respond with ONLY one word — no punctuation, no explanation:
- "cheap"    — simple lookups, formatting, summarizing short text, straightforward code fixes
- "mid"      — moderate analysis, code generation, multi-step reasoning
- "frontier" — complex architecture, novel problem-solving, long-form creative work

Task: <task.Description>
```

Completion parameters for the judge call:
- `MaxTokens: 10` — a single word needs at most 2 tokens; cap tightly to prevent verbose output.
- `Temperature: 0` — deterministic classification; we want the same task to land in the same tier on every call.

### Key Data Flow

```
Classify(ctx, task)
  ├─ task.OverrideTier.Valid()? → return RoutingDecision{Tier: override, Reason: "override: tier forced to X"}
  ├─ r.completer == nil? → log + return RoutingDecision{Tier: defaultTier, Reason: "no completer configured; using default tier"}
  ├─ completer.Complete(ctx, judgeRequest)
  │    ├─ err != nil → log + return RoutingDecision{Tier: defaultTier, Reason: "judge call failed (...); falling back to default tier X"}
  │    └─ ok → parseTier(resp.Content)
  │         ├─ valid tier → return RoutingDecision{Tier: tier, Model: resp.Model, Reason: "judge classified as X"}
  │         └─ invalid → log + return RoutingDecision{Tier: defaultTier, Reason: "judge returned unrecognised response \"...\"; falling back to default tier X"}
  └─ (never returns non-nil error)
```

**Note**: `Classify` is intentionally error-free from the caller's perspective. The routing system must not block task execution because a judge call flaked — degraded routing (default tier) is always better than no routing.

### Dependencies

| Dependency | Why | Already in repo |
|:-----------|:----|:----------------|
| `Completer` interface (`gateway.go`) | Judge model calls go through the same abstraction as real completions — no concrete LiteLLM import needed | Yes (T1) |
| `ModelTier`, `TaskSpec`, `RoutingDecision` types (`gateway.go`) | Core vocabulary types | Yes (T1) |
| `context` (stdlib) | Cancellation/deadline propagation | Yes |
| `fmt`, `log`, `strings` (stdlib) | Prompt formatting, logging, response parsing | Yes |

No external libraries required.

### Risks

| Risk | Likelihood | Impact | Mitigation |
|:-----|:-----------|:-------|:-----------|
| Judge misclassifies task complexity | Medium | Medium — wrong tier selected, cost/quality impact | Log every routing decision with reason; `OverrideTier` provides escape hatch; prompt can be tuned without code changes |
| Judge call adds latency to every task | Medium | Low — Haiku is fast (~200ms p50); total overhead is small relative to the work model | Document expected overhead; expose `SkipJudge` option if latency SLA is tight |
| Judge returns multi-word or verbose response despite prompt constraints | Low | Low — `parseTier` handles by falling back to default | `parseTier` normalises with `strings.TrimSpace` + `strings.ToLower` before matching |
| `nil` completer causes panic | Low | High — unrecoverable | `Classify` guards `r.completer == nil` before any call; returns default tier, never panics |
| Temperature=0 causes identical misclassification on retries | Low | Low — default tier fallback is safe | Accept this; the caller can set `OverrideTier` for tasks that repeatedly misclassify |

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
      And the completer was called exactly once

    Scenario: Format JSON blob routes to cheap
      Given the judge model will respond "cheap"
      And a task with description "Format this JSON blob to be pretty-printed"
      When Classify is called
      Then the routing decision tier is "cheap"
      And the routing decision reason contains "classified as cheap"

  Rule: Complex tasks route to frontier tier

    Scenario: Design distributed caching architecture routes to frontier
      Given the judge model will respond "frontier"
      And a task with description "Design a distributed caching architecture for a globally distributed system"
      When Classify is called
      Then the routing decision tier is "frontier"
      And the routing decision reason contains "classified as frontier"
      And the completer was called exactly once

    Scenario: Novel algorithm design routes to frontier
      Given the judge model will respond "frontier"
      And a task with description "Design a novel consensus algorithm for Byzantine fault tolerance"
      When Classify is called
      Then the routing decision tier is "frontier"
      And the routing decision reason contains "classified as frontier"

  Rule: Mid-complexity tasks route to mid tier

    Scenario: Code generation task routes to mid
      Given the judge model will respond "mid"
      And a task with description "Generate a Go HTTP handler that validates a JSON payload against a schema"
      When Classify is called
      Then the routing decision tier is "mid"
      And the routing decision reason contains "classified as mid"

  Rule: OverrideTier bypasses classification entirely

    Scenario: OverrideTier set to frontier skips judge call
      Given a task with description "Summarize this short message" and OverrideTier "frontier"
      When Classify is called
      Then the routing decision tier is "frontier"
      And the routing decision reason contains "override"
      And the completer was called exactly zero times

    Scenario: OverrideTier set to cheap skips judge call regardless of complexity
      Given a task with description "Design a distributed system" and OverrideTier "cheap"
      When Classify is called
      Then the routing decision tier is "cheap"
      And the routing decision reason contains "override"
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

    Scenario: Judge call returns HTTP 500 falls back to default
      Given the completer will return an error "HTTP 500 Internal Server Error"
      And a task with description "do something"
      When Classify is called
      Then Classify returns no error
      And the routing decision tier is "mid"
      And the routing decision reason contains "falling back to default tier"

  Rule: Unrecognised judge response falls back to default tier

    Scenario: Judge returns garbage text falls back to default
      Given the judge model will respond "bananas"
      And a task with description "do something"
      When Classify is called
      Then Classify returns no error
      And the routing decision tier is "mid"
      And the routing decision reason contains "unrecognised response"
      And the routing decision reason contains "bananas"
      And the routing decision reason contains "falling back to default tier"

    Scenario: Judge returns empty string falls back to default
      Given the judge model will respond ""
      And a task with description "do something"
      When Classify is called
      Then Classify returns no error
      And the routing decision tier is "mid"
      And the routing decision reason contains "falling back to default tier"

    Scenario: Judge response is case-insensitive — "CHEAP" parses as cheap
      Given the judge model will respond "CHEAP"
      And a task with description "format a file"
      When Classify is called
      Then the routing decision tier is "cheap"

    Scenario: Judge response with surrounding whitespace — "  frontier  " parses as frontier
      Given the judge model will respond "  frontier  "
      And a task with description "design a system"
      When Classify is called
      Then the routing decision tier is "frontier"

  Rule: Nil completer uses default tier without panicking

    Scenario: No completer configured returns default tier
      Given a JudgeRouter with no completer configured
      And defaultTier is "cheap"
      And a task with description "anything"
      When Classify is called
      Then Classify returns no error
      And the routing decision tier is "cheap"
      And the routing decision reason contains "no completer configured"
```

---

## Task Breakdown

| ID  | Task | Priority | Dependencies | Estimate |
|:----|:-----|:---------|:-------------|:---------|
| T4.1 | Define `RouterOption` functional option type and `JudgeRouter` struct with unexported fields | High | T1 (types in `gateway.go`) | XS |
| T4.2 | Implement `NewJudgeRouter` constructor with default `judgeModel` and `defaultTier = TierMid` | High | T4.1 | XS |
| T4.3 | Implement `WithJudgeModel`, `WithDefaultTier`, `WithCompleter` option functions | High | T4.1 | XS |
| T4.4 | Write `classificationPrompt` constant and `parseTier` helper | High | T1 | XS |
| T4.5 | Implement `Classify` method: override fast path, nil completer guard, judge call, parse, fallback | High | T4.2, T4.3, T4.4 | S |
| T4.6 | Add structured logging to all routing decision paths in `Classify` | High | T4.5 | XS |
| T4.7 | Write `router_test.go` with `mockCompleter` and table-driven tests for all Gherkin scenarios | High | T4.5 | M |
| T4.8 | Run `go test ./internal/gateway/...` and confirm all tests pass | High | T4.7 | XS |

---

## Exit Criteria

- [ ] `JudgeRouter` implements `Router` interface — confirmed by `var _ Router = (*JudgeRouter)(nil)` compile-time assertion.
- [ ] `go test ./internal/gateway/...` passes with zero failures.
- [ ] All Gherkin scenarios above have a corresponding test case in `router_test.go`.
- [ ] `Classify` never returns a non-nil error — all failure modes result in `defaultTier` with a logged reason.
- [ ] `OverrideTier` set to any valid `ModelTier` results in zero calls to the `Completer`.
- [ ] `parseTier` correctly normalises uppercase, mixed-case, and whitespace-padded judge responses.
- [ ] `NewJudgeRouter()` with no options produces a router with `judgeModel = "claude-haiku-4-5-20251001"` and `defaultTier = TierMid`.
- [ ] Code review: no direct import of concrete LiteLLM client — only `Completer` interface used.

---

## References

- Epic spec: `model-gateway-spec.md` (repo root)
- GitHub issue: #34
- Gateway interface and type definitions: `internal/gateway/gateway.go`
- Multi-model coordination research: `docs/research/patterns/Multi-Model-Coordination.md`

---

*Authored by: Clault KiperS 4.6*
