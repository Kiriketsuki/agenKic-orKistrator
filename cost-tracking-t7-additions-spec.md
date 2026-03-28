# Feature: Cost Tracking T7 Additions

## Overview

**User Story**: As an orchestrator operator, I want the cost tracking module to have named model constants, a response-to-record bridge, and full spec-scenario test coverage so that callers avoid string literals, cost recording from completions is one call, and issue #55's acceptance scenario is satisfied.

**Problem**: PR #51 delivers the core cost tracker but has three gaps: (1) issue #55 requires an exact 5-record aggregation test that the existing `PerTierBreakdown` test does not cover, (2) three open questions in the spec are resolved but not marked, (3) should-have items (named constants, constructor helper) are missing — forcing callers to duplicate string literals and manually construct `CostRecord` from `CompletionResponse`.

**Out of Scope**:
- Redis-backed CostTracker (future task)
- Config loader integration (T5 territory)
- Budget alerts or spend thresholds
- Any changes to the `CostTracker` interface

---

## Success Condition

> This feature is complete when `go test -race ./internal/gateway/...` passes with the spec-scenario aggregation test (5 records, 3 cheap / 2 mid), `NewCostRecordFromResponse` constructor test, all existing tests green, and the spec's 3 open questions are marked resolved.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| — | None — all questions resolved during brainstorming | — | [x] |

---

## Scope

### Must-Have
- **Spec-compliant aggregation test (issue #55)** — 5 CostRecords (3 TierCheap, 2 TierMid); assert `TierCosts[TierCheap].RequestCount == 3`, `TierCosts[TierMid].RequestCount == 2`, `Total.EstimatedCost == sum of all 5 EstimatedCost values`
- **Resolve open questions in `cost-tracking-spec.md`** — mark all 3 as `[x]` with resolution text

### Should-Have
- **Named model constants** — `ModelHaiku`, `ModelSonnet`, `ModelOpus` exported constants replacing string literals in tests and future callers
- **`DefaultPricing` variable** — `map[string]TokenCost` fallback until config loader (T5) loads from `config/models.yaml`
- **`NewCostRecordFromResponse` constructor** — bridges `CompletionResponse` + tier + pricing to a `CostRecord` in one call; uses `time.Now()` for timestamp, delegates to `EstimateCost` for cost calculation

### Nice-to-Have
- Migrate existing test file to use named constants instead of inline strings (cleanup, no behaviour change)

---

## Technical Plan

### Affected Components

| File | Change |
|:-----|:-------|
| `internal/gateway/cost.go` | Add `ModelHaiku`/`ModelSonnet`/`ModelOpus` constants, `DefaultPricing` var, `NewCostRecordFromResponse` func, `"time"` import |
| `internal/gateway/cost_test.go` | Add spec-scenario aggregation test, constructor test, migrate `testPricing` to use `DefaultPricing` |
| `cost-tracking-spec.md` | Mark 3 open questions as resolved, note should-have items as delivered |

### Data Model Changes

None — all existing types unchanged. New constants and helper only.

### API Contracts

```go
// Named model ID constants
const (
    ModelHaiku  = "claude-haiku-4-5-20251001"
    ModelSonnet = "claude-sonnet-4-6"
    ModelOpus   = "claude-opus-4-6"
)

// DefaultPricing is the fallback pricing table used until config/models.yaml
// is loaded by the config loader (T5).
var DefaultPricing = map[string]TokenCost{
    ModelHaiku:  {Input: 0.80, Output: 4.00},
    ModelSonnet: {Input: 3.00, Output: 15.00},
    ModelOpus:   {Input: 15.00, Output: 75.00},
}

// NewCostRecordFromResponse constructs a CostRecord from a CompletionResponse.
func NewCostRecordFromResponse(resp CompletionResponse, tier ModelTier, pricing map[string]TokenCost) CostRecord
```

### Dependencies

No new dependencies. `"time"` import added to `cost.go` (stdlib only).

### Risks

| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| `DefaultPricing` diverges from real provider pricing over time | Medium | Authoritative source is `config/models.yaml` once T5 lands; `DefaultPricing` is documented as fallback only |
| `time.Now()` in constructor makes test assertions on Timestamp non-deterministic | Low | Tests assert on all fields except Timestamp, or use approximate time window |

---

## Acceptance Scenarios

```gherkin
Feature: Cost Tracking T7 Additions
  As an orchestrator operator
  I want spec-compliant tests and convenience helpers
  So that cost tracking is fully validated and easy to use from completion code

  Background:
    Given an InMemoryCostTracker is initialised via NewInMemoryCostTracker()
    And DefaultPricing is available as the pricing table

  Rule: Spec-scenario aggregation (issue #55)

    Scenario: CostReport aggregates 5 records by tier (3 cheap, 2 mid)
      Given five CostRecords are recorded:
        | # | Tier  | Model       | InputTokens | OutputTokens | EstimatedCost |
        | 1 | cheap | ModelHaiku  | 100         | 50           | 0.00028       |
        | 2 | cheap | ModelHaiku  | 200         | 100          | 0.00056       |
        | 3 | cheap | ModelHaiku  | 150         | 75           | 0.00042       |
        | 4 | mid   | ModelSonnet | 300         | 150          | 0.00315       |
        | 5 | mid   | ModelSonnet | 400         | 200          | 0.00420       |
      When Report is called for a period encompassing all five records
      Then TierCosts[TierCheap].RequestCount equals 3
      And TierCosts[TierMid].RequestCount equals 2
      And Total.EstimatedCost equals the sum of all five EstimatedCost values
      And Total.RequestCount equals 5

  Rule: Constructor helper

    Scenario: NewCostRecordFromResponse bridges completion to cost record
      Given a CompletionResponse with Model "claude-sonnet-4-6", InputTokens 1000, OutputTokens 500, ProviderName "anthropic"
      When NewCostRecordFromResponse is called with tier TierMid and DefaultPricing
      Then the returned CostRecord has:
        | Field         | Value                             |
        | Model         | claude-sonnet-4-6                 |
        | Tier          | mid                               |
        | Provider      | anthropic                         |
        | InputTokens   | 1000                              |
        | OutputTokens  | 500                               |
        | EstimatedCost | (1000*3.00 + 500*15.00)/1_000_000 |
      And Timestamp is approximately time.Now()

    Scenario: NewCostRecordFromResponse with unknown model yields zero cost
      Given a CompletionResponse with Model "unknown-model"
      When NewCostRecordFromResponse is called with DefaultPricing
      Then EstimatedCost equals 0.0
      And no panic occurs

  Rule: Named constants

    Scenario: Model constants match expected string values
      Then ModelHaiku equals "claude-haiku-4-5-20251001"
      And ModelSonnet equals "claude-sonnet-4-6"
      And ModelOpus equals "claude-opus-4-6"
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T7a.1 | Add `ModelHaiku`, `ModelSonnet`, `ModelOpus` constants and `DefaultPricing` var to `cost.go` | High | None | pending |
| T7a.2 | Add `NewCostRecordFromResponse` constructor to `cost.go` | High | T7a.1 | pending |
| T7a.3 | Add spec-scenario aggregation test (5 records: 3 cheap, 2 mid) to `cost_test.go` | High | T7a.1 | pending |
| T7a.4 | Add `TestNewCostRecordFromResponse` to `cost_test.go` | High | T7a.2 | pending |
| T7a.5 | Migrate `testPricing` and string literals in existing tests to use named constants | Low | T7a.1 | pending |
| T7a.6 | Mark 3 open questions as resolved in `cost-tracking-spec.md` | High | None | pending |
| T7a.7 | Run `go test -race -v ./internal/gateway/...` — all tests pass, zero races | High | T7a.3, T7a.4 | pending |

---

## Exit Criteria

- [ ] Spec-scenario test passes: 5 records (3 cheap, 2 mid), correct per-tier counts, correct total sum
- [ ] `NewCostRecordFromResponse` test passes with correct field mapping
- [ ] All 7 existing tests continue to pass
- [ ] `go test -race ./internal/gateway/...` — zero data races
- [ ] `go vet ./internal/gateway/...` — clean
- [ ] `cost-tracking-spec.md` open questions all marked `[x]`
- [ ] No external dependencies introduced — stdlib only

---

## References

- Issue: [#55](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/55) — CostReport aggregation test must cover spec scenario
- Issue: [#38](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/38) — Cost Tracking (T7)
- PR: [#51](https://github.com/Kiriketsuki/agenKic-orKistrator/pull/51) — feat: Cost Tracking (T7)
- Epic spec: `model-gateway-spec.md`
- Existing spec: `cost-tracking-spec.md`

---

*Authored by: Clault KiperS 4.6*
