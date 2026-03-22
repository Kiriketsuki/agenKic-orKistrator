# Feature: Gateway Interface & Types (T1)

## Overview

**User Story**: As a gateway package developer, I want a complete set of shared interfaces and types for the model gateway so that all gateway sub-components (router, completer, cost tracker, provider adapters) can be built against stable contracts without circular dependencies.

**Problem**: The model gateway requires four distinct sub-systems (routing, completion, cost tracking, provider adapters) that must interoperate. Without upfront interface and type definitions, each sub-system makes incompatible assumptions about request/response shapes, error handling, and tier semantics, leading to integration breakage later.

**Out of Scope**: Implementation of any sub-system (LiteLLM client, judge-router logic, provider adapters, cost tracking storage). This task produces only the type file(s) — no business logic, no I/O.

---

## Open Questions

| # | Question | Raised By | Resolved | Resolution |
|:--|:---------|:----------|:---------|:-----------|
| 1 | Should `ModelTier` be a typed string or an int enum? Typed string is more readable in logs and YAML config; int is faster to compare. | T1 author | [x] | Typed string (`ModelTier string`). Readable in logs and YAML; `MarshalText`/`UnmarshalText` provide validation at serde boundary. |
| 2 | Does `CompletionRequest` need to carry a raw `[]Message` (OpenAI-style) or a single `Prompt string`? Both are needed for different provider adapters. | T1 author | [x] | `[]Message` chosen for multi-turn support. `SystemPrompt string` field added separately for Anthropic-style system prompt separation. |
| 3 | Should `CostRecord` store actual cost in USD (float64) or integer micro-cents to avoid floating-point drift? | T1 author | [x] | `float64` chosen for simplicity. Known drift risk over large volumes; T7 must apply `math.Round(x*10000)/10000` in aggregation. |
| 4 | Should the `Router` sub-interface return `(ModelTier, string, error)` where the string is a human-readable routing rationale, or is that out of scope for T1? | T1 author | [x] | `RoutingDecision{Tier, Model, Reason}` struct returned. Richer than a bare string; satisfies the epic spec's logging requirement (`model-gateway-spec.md:87`). |

---

## Scope

### Must-Have

> **Note**: The authoritative type reference is `internal/gateway/gateway.go` and `internal/gateway/errors.go`. This spec documents the design intent and acceptance criteria. Where the implementation deliberately improved on the original spec, the implementation is authoritative.

- **`ModelTier` enum**: Three values — `TierCheap`, `TierMid`, `TierFrontier` — with `String()`, `Valid()`, `MarshalText`/`UnmarshalText` for YAML/JSON serde
- **`TaskSpec` type**: Describes the task to be classified and routed; includes `ID string`, `Description string`, `Payload string`, `Metadata map[string]string`, `OverrideTier ModelTier`
- **`RoutingDecision` type**: Router output carrying `Tier ModelTier`, `Model string`, `Reason string` — richer than a bare `ModelTier` return (see Open Question #4)
- **`Message` type**: A single conversation turn with `Role string` and `Content string` fields (compatible with OpenAI and Anthropic message formats)
- **`CompletionRequest` type**: Encapsulates a full LLM call — `Model string`, `Messages []Message`, `SystemPrompt string`, `MaxTokens int`, `Temperature float64`, `Stream bool`, `Tier ModelTier`, `Metadata map[string]string`
- **`CompletionResponse` type**: Unified response regardless of provider — `Content string`, `Model string`, `InputTokens int`, `OutputTokens int`, `FallbackUsed bool`, `ProviderName string`
- **`TokenUsage` type**: Token consumption summary — `InputTokens int`, `OutputTokens int`
- **`CostRecord` type**: Per-request cost entry — `RequestID string`, `Timestamp time.Time`, `Model string`, `Tier ModelTier`, `Provider string`, `InputTokens int`, `OutputTokens int`, `EstimatedCost float64`, `Metadata map[string]string`
- **`CostReport` type**: Aggregated report — `Period TimePeriod`, `TierCosts map[ModelTier]TierCostSummary`, `Total CostSummary`
- **`TierCostSummary` type**: Per-tier aggregate — `Tier ModelTier`, `RequestCount int`, `InputTokens int`, `OutputTokens int`, `EstimatedCost float64`
- **`CostSummary` type**: Cross-tier aggregate — `RequestCount int`, `InputTokens int`, `OutputTokens int`, `EstimatedCost float64`
- **`TimePeriod` type**: Half-open interval `[Start, End)` — `Start time.Time`, `End time.Time`; with constructors `Today()`, `LastNDays(n int)`, `Since(t time.Time)`
- **`ProviderConfig` type**: Runtime config for a single provider — `Name string`, `BaseURL string`, `APIKey string`, `Models []string`
- **`TierConfig` type**: Maps tier to models — `Tier ModelTier`, `PrimaryModel string`, `FallbackChain []string`
- **`Gateway` interface**: Top-level interface — all methods accept `context.Context`:
  - `Route(ctx, TaskSpec) (RoutingDecision, error)`
  - `Complete(ctx, CompletionRequest) (CompletionResponse, error)`
  - `GetCostReport(ctx, TimePeriod) (CostReport, error)`
- **`Router` sub-interface**: `Classify(ctx, TaskSpec) (RoutingDecision, error)` — named `Classify` (not `Route`) to avoid Go method-set ambiguity when composing with `Gateway`
- **`Completer` sub-interface**: `Complete(ctx, CompletionRequest) (CompletionResponse, error)`, `Provider() string` — implemented by per-provider adapters
- **`CostTracker` sub-interface**: `Record(ctx, CostRecord) error`, `Report(ctx, TimePeriod) (CostReport, error)`
- **Error sentinels**: `ErrNoProvider`, `ErrAllProvidersFailed`, `ErrInvalidTier`, `ErrClassificationFailed`, `ErrProviderUnavailable`, `ErrRateLimited`, `ErrConfigInvalid`, `ErrCostTrackerFull`
- **`ProviderError` structured type**: Wraps errors with `Op string`, `Provider string`, `Err error`; implements `error` and `Unwrap()`
- **`FallbackError` structured type**: Aggregates `[]ProviderError` from a fallback chain; `Unwrap()` returns `ErrAllProvidersFailed`

### Should-Have

- `CostRecord.CacheHit bool` field to distinguish cached vs uncached token pricing (relevant for DeepSeek V3 and Claude prompt caching)

### Nice-to-Have

- `TaskSpec.Priority int` field for future priority-based routing
- `CompletionResponse.FinishReason string` for surfacing stop/length/content-filter signals from providers
- `TimePeriod.String() string` for human-readable log output

---

## Technical Plan

**Affected Components**:
- `internal/gateway/gateway.go` — all interfaces, types, config types
- `internal/gateway/errors.go` — sentinel errors and structured error types
- `internal/gateway/gateway_test.go` — acceptance test suite

**API Contracts** (reflects actual implementation — see `gateway.go` for authoritative source):

```go
// internal/gateway/gateway.go
package gateway

import (
    "context"
    "time"
)

type ModelTier string

const (
    TierCheap    ModelTier = "cheap"
    TierMid      ModelTier = "mid"
    TierFrontier ModelTier = "frontier"
)

func (t ModelTier) String() string
func (t ModelTier) Valid() bool
func (t ModelTier) MarshalText() ([]byte, error)
func (t *ModelTier) UnmarshalText(b []byte) error

type TaskSpec struct {
    ID           string
    Description  string
    Payload      string
    Metadata     map[string]string
    OverrideTier ModelTier
}

type RoutingDecision struct {
    Tier   ModelTier
    Model  string
    Reason string
}

type Message struct {
    Role    string
    Content string
}

type CompletionRequest struct {
    Model        string
    Messages     []Message
    SystemPrompt string
    MaxTokens    int
    Temperature  float64
    Stream       bool
    Tier         ModelTier
    Metadata     map[string]string
}

type CompletionResponse struct {
    Content      string
    Model        string
    InputTokens  int
    OutputTokens int
    FallbackUsed bool
    ProviderName string
}

type CostRecord struct {
    RequestID     string
    Timestamp     time.Time
    Model         string
    Tier          ModelTier
    Provider      string
    InputTokens   int
    OutputTokens  int
    EstimatedCost float64 // USD
    Metadata      map[string]string
}

type TimePeriod struct {
    Start time.Time
    End   time.Time
}

func Today() TimePeriod
func LastNDays(n int) TimePeriod
func Since(t time.Time) TimePeriod

type TierCostSummary struct {
    Tier          ModelTier
    RequestCount  int
    InputTokens   int
    OutputTokens  int
    EstimatedCost float64
}

type CostSummary struct {
    RequestCount  int
    InputTokens   int
    OutputTokens  int
    EstimatedCost float64
}

type CostReport struct {
    Period    TimePeriod
    TierCosts map[ModelTier]TierCostSummary
    Total     CostSummary
}

type Gateway interface {
    Route(ctx context.Context, task TaskSpec) (RoutingDecision, error)
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    GetCostReport(ctx context.Context, period TimePeriod) (CostReport, error)
}

type Router interface {
    Classify(ctx context.Context, task TaskSpec) (RoutingDecision, error)
}

type Completer interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    Provider() string
}

type CostTracker interface {
    Record(ctx context.Context, record CostRecord) error
    Report(ctx context.Context, period TimePeriod) (CostReport, error)
}

// internal/gateway/errors.go
var (
    ErrNoProvider         = errors.New("gateway: no provider configured for model")
    ErrAllProvidersFailed = errors.New("gateway: all providers in fallback chain failed")
    ErrInvalidTier        = errors.New("gateway: invalid model tier")
    ErrClassificationFailed = errors.New("gateway: task classification failed")
    ErrProviderUnavailable = errors.New("gateway: provider unavailable")
    ErrRateLimited        = errors.New("gateway: provider rate limited")
    ErrConfigInvalid      = errors.New("gateway: invalid configuration")
    ErrCostTrackerFull    = errors.New("gateway: cost tracker capacity exceeded")
)

type ProviderError struct {
    Op       string
    Provider string
    Err      error
}

type FallbackError struct {
    Errors []ProviderError
}
```

**Deliberate improvements over original spec**:
- `context.Context` on all interface methods — required for cancellation and deadline propagation in network operations
- `RoutingDecision` return type — richer than bare `ModelTier`; carries model name and routing rationale
- `Router.Classify()` instead of `Router.Route()` — prevents Go method-set ambiguity when composing with `Gateway.Route()`
- `ProviderError`/`FallbackError` instead of `GatewayError` — `FallbackError{[]ProviderError}` represents multi-provider chain failures; `ProviderError.Op` carries operation context
- `Valid()` instead of `IsValid()` — follows Go stdlib convention (e.g. `net.IP.IsValid()` is actually the exception; `Valid()` is more idiomatic)

**Dependencies**: `context` (stdlib), `errors` (stdlib), `time` (stdlib) — zero external imports. Verified via `go list -deps`.

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| `CompletionRequest.Messages` is too OpenAI-centric; Anthropic separates `system` from the message list | Mitigated | `SystemPrompt string` field added; adapters handle translation |
| `float64` for cost introduces rounding drift across aggregated reports | Low | T7 must apply consistent rounding (`math.Round(x*10000)/10000`) in aggregation; flagged as known limitation |
| Type proliferation: adding too many types in T1 makes later refactors expensive | Low | Types constrained to those needed by T2–T7; config types (`ProviderConfig`, `TierConfig`) included to prevent T5 from defining parallel type hierarchies |

---

## Acceptance Scenarios

```gherkin
Feature: Gateway Interface & Types
  As a gateway package developer
  I want stable Go interfaces and types for the model gateway
  So that all sub-components can be built against shared contracts

  Rule: ModelTier must be a safe, serializable enum

    Scenario: Valid tier values are recognised
      Given a Go program that imports "internal/gateway"
      When ModelTier("cheap") is evaluated
      Then TierCheap.Valid() returns true
      And TierCheap.String() returns "cheap"

    Scenario: Invalid tier value is detected
      Given a ModelTier with value "unknown"
      When Valid() is called
      Then it returns false

    Scenario: ModelTier round-trips through YAML
      Given a ModelTier value TierFrontier
      When it is marshalled via MarshalText and then unmarshalled via UnmarshalText
      Then the result equals TierFrontier

  Rule: Gateway interface is compositionally satisfied by sub-interfaces

    Scenario: Struct that delegates to Router, Completer, CostTracker satisfies Gateway
      Given a concrete type that delegates Route to Router.Classify, Complete to Completer.Complete, and GetCostReport to CostTracker.Report
      When the compiler checks that the type implements Gateway
      Then it compiles without error

  Rule: Completion types carry all data needed for provider adapters

    Scenario: CompletionRequest holds a multi-turn conversation
      Given a CompletionRequest with three Messages (system, user, assistant)
      When the request is passed to a Completer implementation
      Then no data fields are lost between construction and consumption

    Scenario: CompletionResponse marks fallback usage
      Given a CompletionResponse where FallbackUsed is true
      When the caller inspects the response
      Then it can log that a fallback provider was used

  Rule: Cost types support per-tier aggregation

    Scenario: CostReport aggregates multiple CostRecords by tier
      Given five CostRecords — three with TierCheap and two with TierMid
      When a CostReport is built from those records
      Then ByTier[TierCheap].Requests equals 3
      And ByTier[TierMid].Requests equals 2
      And TotalCostUSD equals the sum of all EstimatedCostUSD values

    Scenario: TimePeriod Today() covers the current calendar day
      Given the current time is 2026-03-22T14:30:00Z
      When Today() is called
      Then Period.Start equals 2026-03-22T00:00:00Z
      And Period.End equals 2026-03-23T00:00:00Z

  Rule: Error types convey actionable context

    Scenario: ProviderError wraps a sentinel and is unwrappable
      Given a ProviderError{Op: "Complete", Provider: "anthropic", Err: ErrProviderUnavailable}
      When errors.Is(err, ErrProviderUnavailable) is evaluated
      Then it returns true

    Scenario: FallbackError wraps ErrAllProvidersFailed
      Given a FallbackError with two ProviderErrors
      When errors.Is(err, ErrAllProvidersFailed) is evaluated
      Then it returns true

    Scenario: ErrInvalidTier is returned for unrecognised tier strings
      Given a ModelTier("bogus")
      When MarshalText() or UnmarshalText() is called
      Then the error is ErrInvalidTier
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T1-a | Create `internal/gateway/gateway.go` with package declaration and import block | High | None | done |
| T1-b | Define `ModelTier` typed string, constants, and `String()` / `Valid()` methods | High | T1-a | done |
| T1-c | Implement `MarshalText` / `UnmarshalText` on `ModelTier` for YAML/JSON round-trip | High | T1-b | done |
| T1-d | Define `Message`, `TaskSpec`, `RoutingDecision`, `CompletionRequest`, `CompletionResponse` types | High | T1-a | done |
| T1-e | Define `CostRecord`, `TierCostSummary`, `CostSummary`, `TimePeriod`, `CostReport` types | High | T1-a | done |
| T1-f | Implement `Today()`, `LastNDays(n int)`, `Since(t time.Time)` constructors on `TimePeriod` | High | T1-e | done |
| T1-g | Define `Gateway`, `Router`, `Completer`, `CostTracker` interfaces (with `context.Context`) | High | T1-d, T1-e | done |
| T1-h | Define error sentinels (`ErrNoProvider`, `ErrAllProvidersFailed`, `ErrInvalidTier`, `ErrClassificationFailed`, `ErrProviderUnavailable`, `ErrRateLimited`, `ErrConfigInvalid`, `ErrCostTrackerFull`) | High | T1-a | done |
| T1-i | Define `ProviderError` and `FallbackError` structs with `Error()` and `Unwrap()` methods | High | T1-h | done |
| T1-j | Write unit tests: `ModelTier` serde round-trip, `TimePeriod` constructors, error unwrap, interface composition | High | T1-c, T1-f, T1-i | done |
| T1-k | Verify zero external imports with `go list -deps ./internal/gateway/` | High | T1-j | done |

---

## Exit Criteria

- [x] `internal/gateway/gateway.go` compiles with `go build ./internal/gateway/`
- [x] All acceptance scenarios pass as unit tests (`gateway_test.go`)
- [x] `ModelTier` marshals to/from `"cheap"`, `"mid"`, `"frontier"` via `MarshalText`/`UnmarshalText` without loss
- [x] `ProviderError` is unwrappable via `errors.Is` against sentinel errors; `FallbackError` unwraps to `ErrAllProvidersFailed`
- [x] `TimePeriod.Today()` returns a range covering only the current calendar day (UTC)
- [x] `internal/gateway/` imports only stdlib packages (`context`, `errors`, `time`) — verified via `go list -deps`
- [x] No other gateway sub-package files are modified or created in this task

---

## References

- Epic spec: `model-gateway-spec.md`
- GitHub issue: #32
- Module path: `github.com/Kiriketsuki/agenKic-orKistrator`
- Gateway package path: `internal/gateway/`

---
*Authored by: Clault KiperS 4.6*
