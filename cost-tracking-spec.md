# Feature: Cost Tracking (T7)

## Overview

**User Story**: As an orchestrator operator, I want per-request cost data logged and queryable by time period so that I can see exactly how much is being spent per model tier and verify that routing decisions produce real cost savings compared to a naive all-frontier baseline.

**Problem**: Without cost tracking, the Judge-Router's claimed 60–90% cost savings are unverifiable. Operators have no way to identify runaway spend, audit tier assignments, or justify the operational complexity of multi-tier routing. Every request is currently a black box from a billing perspective.

**Out of Scope**:
- Persistent cost storage (Redis-backed tracker, database writes) — in-memory only for this task
- Budget alerts or spend thresholds
- Provider invoice reconciliation
- Cost-based routing decisions (auto-downgrade on budget exhaustion)
- Streaming cost attribution (mid-stream token counting)

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should the pricing table live in `models.yaml` (operator-configurable) or as constants in Go code? Loading from config is more flexible but adds startup coupling. | T7 spec | [ ] |
| 2 | Should `EstimateCost` be a method on `CostTracker` (interface-bound) or a free pure function? Pure function is more composable and testable; method binding ties pricing to the tracker. | T7 spec | [ ] |
| 3 | Is period filtering inclusive on both boundaries, or half-open `[Start, End)`? Inclusive is more natural for daily reports (midnight-to-midnight), but half-open avoids double-counting records that land on a shared boundary. | T7 spec | [ ] |

---

## Scope

### Must-Have
- **`TokenCost` struct** — holds `Input` and `Output` pricing in USD per 1M tokens; used as the value type in a `map[string]TokenCost` pricing table
- **`InMemoryCostTracker`** implementing the `CostTracker` interface — all state in-process, no external dependencies
- **`Record(ctx, CostRecord)`** — appends one cost record; safe for concurrent calls from multiple goroutines
- **`Report(ctx, TimePeriod)`** — filters records by time window, aggregates into `CostReport` with per-tier `TierCostSummary` entries and a combined `CostSummary` total
- **`RecordCount()`** — returns the total number of stored records; used for observability and test assertions
- **`EstimateCost(model, inputTokens, outputTokens, pricing)`** — pure function; computes estimated cost in USD from a pricing table; returns 0 for unknown models (no panic)
- **Thread safety** under concurrent `Record` calls via `sync.RWMutex` (write lock on Record, read lock on Report/RecordCount)

### Should-Have
- Named pricing constants for the standard Claude and GPT-4o model IDs used in tests (to avoid string literals scattered across callsites)
- A constructor helper that builds a `CostRecord` from a `CompletionResponse` plus a pricing table (bridges gateway completion to cost recording without boilerplate in the caller)

### Nice-to-Have
- A Redis-backed `CostTracker` implementation for persistence across restarts
- Periodic cost summary logging (hook that fires every N requests or every M minutes)
- `Snapshot()` to export all records as a slice (useful for migration to a durable store)

---

## Technical Plan

### Affected Components

| File | Change |
|:-----|:-------|
| `internal/gateway/gateway.go` | Add `TokenCost` struct definition alongside existing cost types |
| `internal/gateway/cost.go` | New file — `InMemoryCostTracker`, `EstimateCost`, `RecordCount` |
| `internal/gateway/cost_test.go` | New file — table-driven tests for all acceptance scenarios |

### Go Types and Interfaces

Types defined in `internal/gateway/gateway.go` (alongside the existing `CostRecord`, `CostReport`, `CostSummary`, `TierCostSummary`, `TimePeriod`):

```go
// TokenCost holds per-million-token pricing for a single model.
type TokenCost struct {
    Input  float64 // USD per 1,000,000 input tokens
    Output float64 // USD per 1,000,000 output tokens
}
```

The `CostTracker` interface (already defined in `gateway.go`):

```go
type CostTracker interface {
    Record(ctx context.Context, record CostRecord) error
    Report(ctx context.Context, period TimePeriod) (CostReport, error)
}
```

Implementation in `internal/gateway/cost.go`:

```go
type InMemoryCostTracker struct {
    mu      sync.RWMutex
    records []CostRecord
}

func NewInMemoryCostTracker() *InMemoryCostTracker

// Record appends a CostRecord. Safe for concurrent use.
func (t *InMemoryCostTracker) Record(_ context.Context, record CostRecord) error

// Report filters records within period (inclusive) and aggregates by tier.
func (t *InMemoryCostTracker) Report(_ context.Context, period TimePeriod) (CostReport, error)

// RecordCount returns the total number of stored records.
func (t *InMemoryCostTracker) RecordCount() int

// EstimateCost is a pure function. Returns 0 for unknown models.
func EstimateCost(model string, inputTokens, outputTokens int, pricing map[string]TokenCost) float64
```

### EstimateCost Formula

```
cost (USD) = ( inputTokens × pricing[model].Input
             + outputTokens × pricing[model].Output ) / 1_000_000
```

If `model` is not present in `pricing`, return `0.0` without error or panic.

### Reference Pricing (examples; authoritative source is `config/models.yaml`)

| Model | Input $/M | Output $/M |
|:------|----------:|----------:|
| `claude-haiku-4-5-20251001` | $0.80 | $4.00 |
| `claude-sonnet-4-6` | $3.00 | $15.00 |
| `claude-opus-4-6` | $15.00 | $75.00 |
| `gpt-4o` | $2.50 | $10.00 |
| `ollama/*` | $0.00 | $0.00 |

### Dependencies

| Dependency | Source | Reason |
|:-----------|:-------|:-------|
| `context` | stdlib | Context propagation per Go convention |
| `sync` | stdlib | `sync.RWMutex` for concurrent access |
| `time` | stdlib | `CostRecord.Timestamp`, `TimePeriod` boundaries |

No external dependencies. The cost tracker must compile with only the Go standard library.

### Risks

| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| In-memory state lost on restart | Certain | Documented as known limitation; Redis-backed tracker is a future task |
| Float64 precision drift in large cumulative totals | Low | Each record stores its own `EstimatedCost`; Report sums pre-computed values rather than recomputing from raw tokens |
| Unbounded memory growth in long-running processes | Medium | RecordCount() exposes growth for monitoring; retention/eviction policy is future scope |
| Data race under heavy concurrent load | Low | `sync.RWMutex` guards all read and write paths; validated with `go test -race` |

---

## Acceptance Scenarios

```gherkin
Feature: Cost Tracking
  As an orchestrator operator
  I want per-request cost data recorded and queryable by time period
  So that I can verify routing produces real cost savings

  Background:
    Given an InMemoryCostTracker is initialised via NewInMemoryCostTracker()
    And the pricing table contains:
      | Model                      | Input $/M | Output $/M |
      | claude-haiku-4-5-20251001  | 0.80      | 4.00       |
      | claude-sonnet-4-6          | 3.00      | 15.00      |
      | claude-opus-4-6            | 15.00     | 75.00      |
      | gpt-4o                     | 2.50      | 10.00      |

  Rule: Record and Report basic requests

    Scenario: Three requests across all tiers are recorded and reported
      Given three CostRecords are recorded:
        | Tier     | Model                     | InputTokens | OutputTokens | EstimatedCost |
        | cheap    | claude-haiku-4-5-20251001 | 100         | 50           | 0.0002        |
        | mid      | claude-sonnet-4-6         | 200         | 100          | 0.002         |
        | frontier | claude-opus-4-6           | 300         | 150          | 0.016         |
      When Report is called for a period encompassing all three records
      Then Total.RequestCount equals 3
      And Total.InputTokens equals 600
      And Total.OutputTokens equals 300
      And TierCosts contains exactly three entries: cheap, mid, frontier

  Rule: Time filtering

    Scenario: Records outside the period are excluded
      Given one record timestamped at T (inside window)
      And one record timestamped at T minus 2 hours (before window)
      And one record timestamped at T plus 2 hours (after window)
      When Report is called for the window [T minus 1 hour, T plus 1 hour]
      Then Total.RequestCount equals 1
      And Total.InputTokens equals the inside-record's InputTokens only

    Scenario: Period boundaries are inclusive
      Given one record timestamped exactly at period.Start
      And one record timestamped exactly at period.End
      When Report is called for that period
      Then Total.RequestCount equals 2
      And both boundary records are counted

  Rule: Empty period

    Scenario: Report with no records returns zero-value totals
      Given no records have been recorded
      When Report is called for any period
      Then Total.RequestCount is 0
      And Total.EstimatedCost is 0.0
      And TierCosts is an empty map

    Scenario: Report for a period with no matching records returns zero-value totals
      Given records exist but all fall outside the requested period
      When Report is called for a period with no matching records
      Then Total.RequestCount is 0
      And TierCosts is empty

  Rule: Concurrent safety

    Scenario: 100 concurrent goroutines recording simultaneously
      Given 100 goroutines each call Record with a valid CostRecord simultaneously
      When all goroutines complete
      Then RecordCount() equals 100
      And no data race is detected by the Go race detector

  Rule: Per-tier breakdown

    Scenario: Tier summaries are independently accurate
      Given 2 cheap requests with InputTokens 100 and 200, OutputTokens 50 and 100
      And 1 mid request with InputTokens 500, OutputTokens 250
      And 1 frontier request with InputTokens 1000, OutputTokens 500
      When Report is called for the encompassing period
      Then TierCosts[cheap].RequestCount equals 2
      And TierCosts[cheap].InputTokens equals 300
      And TierCosts[cheap].OutputTokens equals 150
      And TierCosts[mid].RequestCount equals 1
      And TierCosts[mid].InputTokens equals 500
      And TierCosts[frontier].RequestCount equals 1
      And TierCosts[frontier].InputTokens equals 1000
      And Total.RequestCount equals 4
      And Total.InputTokens equals 1800

  Rule: Cost savings verification

    Scenario: Tiered routing costs less than all-frontier baseline
      Given 800 cheap requests each with 1000 input and 200 output tokens
        """
        Per-request cost: (1000×0.80 + 200×4.00) / 1_000_000 = $0.00168
        Tier total: 800 × $0.00168 = $1.344
        """
      And 150 mid requests each with 2000 input and 500 output tokens
        """
        Per-request cost: (2000×3.00 + 500×15.00) / 1_000_000 = $0.01350
        Tier total: 150 × $0.01350 = $2.025
        """
      And 50 frontier requests each with 5000 input and 1000 output tokens
        """
        Per-request cost: (5000×15.00 + 1000×75.00) / 1_000_000 = $0.150
        Tier total: 50 × $0.150 = $7.50
        """
      When Report is called for the period
      Then the Total.EstimatedCost is approximately $10.869
        """
        All-frontier baseline (all 1000 requests at Opus pricing, using weighted avg tokens):
          avg_input  = (800×1000 + 150×2000 + 50×5000) / 1000 = 1350
          avg_output = (800×200  + 150×500  + 50×1000) / 1000  = 260
          per-request: (1350×15.00 + 260×75.00) / 1_000_000 = $0.039975
          total: 1000 × $0.039975 = $39.975
        """
      And the total is less than the all-frontier baseline of $39.975
      And the cost savings percentage is at least 60%

  Rule: EstimateCost pure function

    Scenario: Correct cost for known model and token counts
      When EstimateCost("claude-haiku-4-5-20251001", 1000, 500, pricing) is called
      Then the result equals (1000×0.80 + 500×4.00) / 1_000_000 = 0.0028

    Scenario: Zero tokens yields zero cost
      When EstimateCost("claude-haiku-4-5-20251001", 0, 0, pricing) is called
      Then the result equals 0.0

    Scenario: Unknown model returns 0 without error or panic
      When EstimateCost("nonexistent-model", 1000, 500, pricing) is called
      Then the result is 0.0
      And no panic or error occurs

    Scenario: Sonnet pricing calculation
      When EstimateCost("claude-sonnet-4-6", 1000, 1000, pricing) is called
      Then the result equals (1000×3.00 + 1000×15.00) / 1_000_000 = 0.018
```

---

## Task Breakdown

| ID    | Task | Priority | Dependencies | Estimate |
|:------|:-----|:---------|:-------------|:---------|
| T7.1  | Add `TokenCost` struct to `internal/gateway/gateway.go` | High | T1 | 30 min |
| T7.2  | Implement `NewInMemoryCostTracker` and `Record` with `sync.RWMutex` write-lock | High | T7.1 | 1 h |
| T7.3  | Implement `Report` with inclusive time filtering and per-tier aggregation | High | T7.2 | 2 h |
| T7.4  | Implement `RecordCount()` observability helper (read-lock) | High | T7.2 | 30 min |
| T7.5  | Implement `EstimateCost` pure function; handle missing-model case | High | T7.1 | 30 min |
| T7.6  | Write table-driven unit tests covering all Gherkin scenarios above | High | T7.5 | 3 h |
| T7.7  | Run `go test -race ./internal/gateway/...` and confirm zero races | High | T7.6 | 30 min |

---

## Exit Criteria

- [ ] `InMemoryCostTracker` satisfies the `CostTracker` interface (compile-time check: `var _ CostTracker = (*InMemoryCostTracker)(nil)`)
- [ ] All Gherkin scenarios above pass as Go unit tests in `cost_test.go`
- [ ] `go test -race ./internal/gateway/...` passes with zero data races
- [ ] `EstimateCost` formula verified: `(input × pricing.Input + output × pricing.Output) / 1_000_000`
- [ ] Cost savings scenario: 800 cheap + 150 mid + 50 frontier total < all-frontier baseline ($39.975), with ≥60% savings
- [ ] `RecordCount()` returns the exact number of records after 100 concurrent `Record` calls
- [ ] Period boundary records (timestamps equal to `Start` or `End`) are included in report totals
- [ ] No external dependencies introduced — stdlib only (`context`, `sync`, `time`)

---

## References

- Epic spec: `model-gateway-spec.md`
- GitHub issue: [#38](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/38)

---

*Authored by: Clault KiperS 4.6*
