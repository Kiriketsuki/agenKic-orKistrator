package gateway

import (
	"context"
	"sync"
	"time"
)

// Named model ID constants for standard Claude models.
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

// InMemoryCostTracker is a thread-safe in-memory implementation of CostTracker.
type InMemoryCostTracker struct {
	mu      sync.RWMutex
	records []CostRecord
}

// NewInMemoryCostTracker returns a ready-to-use in-memory cost tracker.
func NewInMemoryCostTracker() *InMemoryCostTracker {
	return &InMemoryCostTracker{}
}

// Record appends a CostRecord to the tracker. Safe for concurrent use.
func (t *InMemoryCostTracker) Record(_ context.Context, record CostRecord) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, record)
	return nil
}

// Report returns aggregated cost data for all records within period (inclusive).
func (t *InMemoryCostTracker) Report(_ context.Context, period TimePeriod) (CostReport, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	tierCosts := make(map[ModelTier]TierCostSummary)
	var total CostSummary

	for _, r := range t.records {
		if r.Timestamp.Before(period.Start) || !r.Timestamp.Before(period.End) {
			continue
		}

		s := tierCosts[r.Tier]
		s.Tier = r.Tier
		s.RequestCount++
		s.InputTokens += r.InputTokens
		s.OutputTokens += r.OutputTokens
		s.EstimatedCost += r.EstimatedCost
		tierCosts[r.Tier] = s

		total.RequestCount++
		total.InputTokens += r.InputTokens
		total.OutputTokens += r.OutputTokens
		total.EstimatedCost += r.EstimatedCost
	}

	return CostReport{
		Period:    period,
		TierCosts: tierCosts,
		Total:     total,
	}, nil
}

// RecordCount returns the total number of recorded entries.
func (t *InMemoryCostTracker) RecordCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.records)
}

// EstimateCost calculates the estimated cost in USD for a given model and token
// counts using the provided pricing table. Returns 0 if the model is not found.
func EstimateCost(model string, inputTokens, outputTokens int, pricing map[string]TokenCost) float64 {
	tc, ok := pricing[model]
	if !ok {
		return 0
	}
	return (float64(inputTokens)*tc.Input + float64(outputTokens)*tc.Output) / 1_000_000
}

// NewCostRecordFromResponse constructs a CostRecord from a CompletionResponse,
// bridging gateway completion output to cost tracking. Note: RequestID and
// Metadata are not populated by this constructor; callers must set RequestID
// from TaskSpec.ID (or generate one) and Metadata from CompletionRequest.Metadata
// when propagation is required.
func NewCostRecordFromResponse(resp CompletionResponse, tier ModelTier, pricing map[string]TokenCost) CostRecord {
	return CostRecord{
		Timestamp:     time.Now(),
		Model:         resp.Model,
		Tier:          tier,
		Provider:      resp.ProviderName,
		InputTokens:   resp.InputTokens,
		OutputTokens:  resp.OutputTokens,
		EstimatedCost: EstimateCost(resp.Model, resp.InputTokens, resp.OutputTokens, pricing),
	}
}
