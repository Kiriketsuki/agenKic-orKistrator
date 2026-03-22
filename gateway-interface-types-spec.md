# Feature: Gateway Interface & Types (T1)

## Overview

**User Story**: As a gateway package developer, I want a complete set of shared interfaces and types for the model gateway so that all gateway sub-components (router, completer, cost tracker, provider adapters) can be built against stable contracts without circular dependencies.

**Problem**: The model gateway requires four distinct sub-systems (routing, completion, cost tracking, provider adapters) that must interoperate. Without upfront interface and type definitions, each sub-system makes incompatible assumptions about request/response shapes, error handling, and tier semantics, leading to integration breakage later.

**Out of Scope**: Implementation of any sub-system (LiteLLM client, judge-router logic, provider adapters, cost tracking storage). This task produces only the type file(s) — no business logic, no I/O.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should `ModelTier` be a typed string or an int enum? Typed string is more readable in logs and YAML config; int is faster to compare. | T1 author | [ ] |
| 2 | Does `CompletionRequest` need to carry a raw `[]Message` (OpenAI-style) or a single `Prompt string`? Both are needed for different provider adapters. | T1 author | [ ] |
| 3 | Should `CostRecord` store actual cost in USD (float64) or integer micro-cents to avoid floating-point drift? | T1 author | [ ] |
| 4 | Should the `Router` sub-interface return `(ModelTier, string, error)` where the string is a human-readable routing rationale, or is that out of scope for T1? | T1 author | [ ] |

---

## Scope

### Must-Have

- **`ModelTier` enum**: Three values — `TierCheap`, `TierMid`, `TierFrontier` — with `String()` and `MarshalText`/`UnmarshalText` for YAML/JSON serde
- **`TaskSpec` type**: Describes the task to be classified and routed; includes `Description string`, `Payload string`, and `Metadata map[string]string`
- **`Message` type**: A single conversation turn with `Role string` and `Content string` fields (compatible with OpenAI and Anthropic message formats)
- **`CompletionRequest` type**: Encapsulates a full LLM call — `Model string`, `Messages []Message`, `MaxTokens int`, `Temperature float64`, `Stream bool`, and `Tier ModelTier`
- **`CompletionResponse` type**: Unified response regardless of provider — `Content string`, `Model string`, `InputTokens int`, `OutputTokens int`, `FallbackUsed bool`, `ProviderMeta map[string]any`
- **`CostRecord` type**: Per-request cost entry — `RequestID string`, `Model string`, `Tier ModelTier`, `InputTokens int`, `OutputTokens int`, `EstimatedCostUSD float64`, `Timestamp time.Time`
- **`CostReport` type**: Aggregated report — `Period TimePeriod`, `ByTier map[ModelTier]TierStats`, `TotalCostUSD float64`
- **`TierStats` type**: Per-tier aggregate — `Requests int`, `InputTokens int`, `OutputTokens int`, `CostUSD float64`
- **`TimePeriod` type**: Named time range — `Start time.Time`, `End time.Time`; with constructors `Today()`, `LastNDays(n int)`, `Since(t time.Time)`
- **`Gateway` interface**: Top-level interface — `Route(TaskSpec) (ModelTier, error)`, `Complete(CompletionRequest) (CompletionResponse, error)`, `GetCostReport(TimePeriod) (CostReport, error)`
- **`Router` sub-interface**: `Route(TaskSpec) (ModelTier, error)` — implemented by JudgeRouter
- **`Completer` sub-interface**: `Complete(CompletionRequest) (CompletionResponse, error)` — implemented by LiteLLMClient
- **`CostTracker` sub-interface**: `Record(CostRecord) error`, `Report(TimePeriod) (CostReport, error)` — implemented by InMemoryCostTracker (and future Redis-backed variant)
- **Error sentinels**: `ErrNoRouteFound`, `ErrProviderUnavailable`, `ErrAllFallbacksFailed`, `ErrInvalidTier`, `ErrCostTrackerFull`
- **`GatewayError` structured type**: Wraps underlying errors with `Op string` (operation name), `Tier ModelTier`, `Provider string`, `Err error`; implements `error` and `Unwrap()`

### Should-Have

- `CompletionRequest.SystemPrompt string` field for providers that handle system prompts separately from the message list
- `CostRecord.CacheHit bool` field to distinguish cached vs uncached token pricing (relevant for DeepSeek V3 and Claude prompt caching)
- `ModelTier.IsValid() bool` helper to guard against zero-value misuse

### Nice-to-Have

- `TaskSpec.Priority int` field for future priority-based routing
- `CompletionResponse.FinishReason string` for surfacing stop/length/content-filter signals from providers
- `TimePeriod.String() string` for human-readable log output

---

## Technical Plan

**Affected Components**:
- `internal/gateway/gateway.go` — all interfaces, types, error sentinels (new file)
- No other files touched in T1

**API Contracts**:

```go
// internal/gateway/gateway.go

package gateway

import "time"

// ModelTier classifies task complexity and maps to a cost bracket.
type ModelTier string

const (
    TierCheap    ModelTier = "cheap"
    TierMid      ModelTier = "mid"
    TierFrontier ModelTier = "frontier"
)

func (t ModelTier) String() string
func (t ModelTier) IsValid() bool
func (t ModelTier) MarshalText() ([]byte, error)
func (t *ModelTier) UnmarshalText(b []byte) error

// Message is a single conversation turn.
type Message struct {
    Role    string // "user" | "assistant" | "system"
    Content string
}

// TaskSpec describes an incoming task for routing classification.
type TaskSpec struct {
    Description string
    Payload     string
    Metadata    map[string]string
}

// CompletionRequest is a provider-agnostic LLM call.
type CompletionRequest struct {
    Model        string
    Messages     []Message
    SystemPrompt string
    MaxTokens    int
    Temperature  float64
    Stream       bool
    Tier         ModelTier
}

// CompletionResponse is a unified response from any provider.
type CompletionResponse struct {
    Content      string
    Model        string
    InputTokens  int
    OutputTokens int
    FallbackUsed bool
    ProviderMeta map[string]any
}

// CostRecord captures cost data for a single completion request.
type CostRecord struct {
    RequestID        string
    Model            string
    Tier             ModelTier
    InputTokens      int
    OutputTokens     int
    EstimatedCostUSD float64
    CacheHit         bool
    Timestamp        time.Time
}

// TierStats aggregates cost data for one ModelTier in a report period.
type TierStats struct {
    Requests     int
    InputTokens  int
    OutputTokens int
    CostUSD      float64
}

// TimePeriod is a half-open interval [Start, End).
type TimePeriod struct {
    Start time.Time
    End   time.Time
}

func Today() TimePeriod
func LastNDays(n int) TimePeriod
func Since(t time.Time) TimePeriod

// CostReport aggregates cost data for a TimePeriod.
type CostReport struct {
    Period       TimePeriod
    ByTier       map[ModelTier]TierStats
    TotalCostUSD float64
}

// Gateway is the top-level model gateway interface.
type Gateway interface {
    Route(task TaskSpec) (ModelTier, error)
    Complete(req CompletionRequest) (CompletionResponse, error)
    GetCostReport(period TimePeriod) (CostReport, error)
}

// Router classifies a TaskSpec into a ModelTier.
type Router interface {
    Route(task TaskSpec) (ModelTier, error)
}

// Completer executes a CompletionRequest against a provider.
type Completer interface {
    Complete(req CompletionRequest) (CompletionResponse, error)
}

// CostTracker records and reports per-request cost data.
type CostTracker interface {
    Record(record CostRecord) error
    Report(period TimePeriod) (CostReport, error)
}

// Error sentinels
var (
    ErrNoRouteFound       = errors.New("gateway: no route found for task")
    ErrProviderUnavailable = errors.New("gateway: provider unavailable")
    ErrAllFallbacksFailed = errors.New("gateway: all fallback providers failed")
    ErrInvalidTier        = errors.New("gateway: invalid model tier")
    ErrCostTrackerFull    = errors.New("gateway: cost tracker capacity exceeded")
)

// GatewayError is a structured error with operation context.
type GatewayError struct {
    Op       string    // e.g. "Route", "Complete", "GetCostReport"
    Tier     ModelTier
    Provider string
    Err      error
}

func (e *GatewayError) Error() string
func (e *GatewayError) Unwrap() error
```

**Dependencies**: `errors` (stdlib), `time` (stdlib) — no external imports. T1 has zero external dependencies by design so it can be imported by all other gateway sub-packages without creating import cycles.

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| `CompletionRequest.Messages` is too OpenAI-centric; Anthropic separates `system` from the message list | Medium | Add `SystemPrompt string` field to `CompletionRequest` as a Should-Have; adapters handle translation |
| `float64` for cost introduces rounding drift across aggregated reports | Low | Use consistent rounding (`math.Round(x*10000)/10000`) in cost estimation; flag as a known limitation |
| Type proliferation: adding too many types in T1 makes later refactors expensive | Medium | Constrain T1 strictly to types needed by T2–T7; no speculative fields |

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
      Then TierCheap.IsValid() returns true
      And TierCheap.String() returns "cheap"

    Scenario: Invalid tier value is detected
      Given a ModelTier with value "unknown"
      When IsValid() is called
      Then it returns false

    Scenario: ModelTier round-trips through YAML
      Given a ModelTier value TierFrontier
      When it is marshalled via MarshalText and then unmarshalled via UnmarshalText
      Then the result equals TierFrontier

  Rule: Gateway interface is compositionally satisfied by sub-interfaces

    Scenario: Struct that implements Router, Completer, CostTracker satisfies Gateway
      Given a concrete type "GatewayImpl" that embeds a Router, Completer, and CostTracker
      When the compiler checks that GatewayImpl implements Gateway
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

    Scenario: GatewayError wraps a sentinel and is unwrappable
      Given a GatewayError{Op: "Complete", Err: ErrProviderUnavailable}
      When errors.Is(err, ErrProviderUnavailable) is evaluated
      Then it returns true

    Scenario: ErrInvalidTier is returned for unrecognised tier strings
      Given a CompletionRequest with Tier set to ModelTier("bogus")
      When a Completer validates the request
      Then the error wraps ErrInvalidTier
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T1-a | Create `internal/gateway/gateway.go` with package declaration and import block | High | None | pending |
| T1-b | Define `ModelTier` typed string, constants, and `String()` / `IsValid()` methods | High | T1-a | pending |
| T1-c | Implement `MarshalText` / `UnmarshalText` on `ModelTier` for YAML/JSON round-trip | High | T1-b | pending |
| T1-d | Define `Message`, `TaskSpec`, `CompletionRequest`, `CompletionResponse` types | High | T1-a | pending |
| T1-e | Define `CostRecord`, `TierStats`, `TimePeriod`, `CostReport` types | High | T1-a | pending |
| T1-f | Implement `Today()`, `LastNDays(n int)`, `Since(t time.Time)` constructors on `TimePeriod` | High | T1-e | pending |
| T1-g | Define `Gateway`, `Router`, `Completer`, `CostTracker` interfaces | High | T1-d, T1-e | pending |
| T1-h | Define error sentinels (`ErrNoRouteFound`, `ErrProviderUnavailable`, `ErrAllFallbacksFailed`, `ErrInvalidTier`, `ErrCostTrackerFull`) | High | T1-a | pending |
| T1-i | Define `GatewayError` struct with `Error()` and `Unwrap()` methods | High | T1-h | pending |
| T1-j | Write unit tests: `ModelTier` serde round-trip, `TimePeriod` constructors, `GatewayError` unwrap | High | T1-c, T1-f, T1-i | pending |
| T1-k | Verify zero external imports with `go list -deps ./internal/gateway/` | High | T1-j | pending |

---

## Exit Criteria

- [ ] `internal/gateway/gateway.go` compiles with `go build ./internal/gateway/`
- [ ] All acceptance scenarios pass as unit tests
- [ ] `ModelTier` marshals to/from `"cheap"`, `"mid"`, `"frontier"` in YAML and JSON without loss
- [ ] `GatewayError` is unwrappable via `errors.Is` and `errors.As` against all sentinel errors
- [ ] `TimePeriod.Today()` returns a range covering only the current calendar day (UTC)
- [ ] `internal/gateway/gateway.go` imports only stdlib packages (`errors`, `time`)
- [ ] No other gateway sub-package files are modified or created in this task

---

## References

- Epic spec: `model-gateway-spec.md`
- GitHub issue: #32
- Module path: `github.com/Kiriketsuki/agenKic-orKistrator`
- Gateway package path: `internal/gateway/`

---
*Authored by: Clault KiperS 4.6*
