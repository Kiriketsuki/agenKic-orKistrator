package gateway

import (
	"context"
	"sync"
	"testing"
	"time"
)

var testPricing = map[string]TokenCost{
	"claude-haiku-4-5-20251001": {Input: 0.80, Output: 4.00},
	"claude-sonnet-4-6":         {Input: 3.00, Output: 15.00},
	"claude-opus-4-6":           {Input: 15.00, Output: 75.00},
}

func baseTime() time.Time {
	return time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
}

func period(start, end time.Time) TimePeriod {
	return TimePeriod{Start: start, End: end}
}

// TestInMemoryCostTracker_RecordAndReport verifies that records are stored and
// aggregated correctly across a basic set of requests.
func TestInMemoryCostTracker_RecordAndReport(t *testing.T) {
	ctx := context.Background()
	tracker := NewInMemoryCostTracker()

	now := baseTime()
	records := []CostRecord{
		{Timestamp: now, Model: "claude-haiku-4-5-20251001", Tier: TierCheap, Provider: "anthropic", InputTokens: 100, OutputTokens: 50, EstimatedCost: 0.0002},
		{Timestamp: now, Model: "claude-sonnet-4-6", Tier: TierMid, Provider: "anthropic", InputTokens: 200, OutputTokens: 100, EstimatedCost: 0.002},
		{Timestamp: now, Model: "claude-opus-4-6", Tier: TierFrontier, Provider: "anthropic", InputTokens: 300, OutputTokens: 150, EstimatedCost: 0.016},
	}

	for _, r := range records {
		if err := tracker.Record(ctx, r); err != nil {
			t.Fatalf("Record() error = %v", err)
		}
	}

	p := period(now.Add(-time.Hour), now.Add(time.Hour))
	report, err := tracker.Report(ctx, p)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	if got := report.Total.RequestCount; got != 3 {
		t.Errorf("Total.RequestCount = %d, want 3", got)
	}
	if got := report.Total.InputTokens; got != 600 {
		t.Errorf("Total.InputTokens = %d, want 600", got)
	}
	if got := report.Total.OutputTokens; got != 300 {
		t.Errorf("Total.OutputTokens = %d, want 300", got)
	}
	if len(report.TierCosts) != 3 {
		t.Errorf("len(TierCosts) = %d, want 3", len(report.TierCosts))
	}
}

// TestInMemoryCostTracker_TimeFiltering verifies that records outside the
// reporting period are excluded.
func TestInMemoryCostTracker_TimeFiltering(t *testing.T) {
	ctx := context.Background()
	tracker := NewInMemoryCostTracker()

	now := baseTime()
	inside := CostRecord{Timestamp: now, Tier: TierCheap, InputTokens: 100, OutputTokens: 50, EstimatedCost: 0.001}
	before := CostRecord{Timestamp: now.Add(-2 * time.Hour), Tier: TierCheap, InputTokens: 999, OutputTokens: 999, EstimatedCost: 999}
	after := CostRecord{Timestamp: now.Add(2 * time.Hour), Tier: TierCheap, InputTokens: 999, OutputTokens: 999, EstimatedCost: 999}

	for _, r := range []CostRecord{before, inside, after} {
		_ = tracker.Record(ctx, r)
	}

	p := period(now.Add(-time.Hour), now.Add(time.Hour))
	report, err := tracker.Report(ctx, p)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	if got := report.Total.RequestCount; got != 1 {
		t.Errorf("Total.RequestCount = %d, want 1 (only record inside window)", got)
	}
	if got := report.Total.InputTokens; got != 100 {
		t.Errorf("Total.InputTokens = %d, want 100", got)
	}
}

// TestInMemoryCostTracker_PeriodBoundariesInclusive verifies that records
// exactly on the Start and End boundaries are included.
func TestInMemoryCostTracker_PeriodBoundariesInclusive(t *testing.T) {
	ctx := context.Background()
	tracker := NewInMemoryCostTracker()

	start := baseTime()
	end := start.Add(time.Hour)

	for _, ts := range []time.Time{start, end} {
		_ = tracker.Record(ctx, CostRecord{Timestamp: ts, Tier: TierCheap, InputTokens: 1, EstimatedCost: 0.001})
	}

	report, err := tracker.Report(ctx, period(start, end))
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if got := report.Total.RequestCount; got != 2 {
		t.Errorf("Total.RequestCount = %d, want 2 (boundary records must be included)", got)
	}
}

// TestInMemoryCostTracker_EmptyReport verifies that a report with no matching
// records returns a zero-value CostReport.
func TestInMemoryCostTracker_EmptyReport(t *testing.T) {
	ctx := context.Background()
	tracker := NewInMemoryCostTracker()

	now := baseTime()
	p := period(now, now.Add(time.Hour))
	report, err := tracker.Report(ctx, p)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	if report.Total.RequestCount != 0 {
		t.Errorf("Total.RequestCount = %d, want 0", report.Total.RequestCount)
	}
	if report.Total.EstimatedCost != 0 {
		t.Errorf("Total.EstimatedCost = %f, want 0", report.Total.EstimatedCost)
	}
	if len(report.TierCosts) != 0 {
		t.Errorf("len(TierCosts) = %d, want 0", len(report.TierCosts))
	}
}

// TestEstimateCost_KnownPricing verifies cost calculation with known pricing.
func TestEstimateCost_KnownPricing(t *testing.T) {
	// claude-haiku-4-5-20251001: input $0.80/M, output $4.00/M
	// 1000 input + 500 output = (1000*0.80 + 500*4.00) / 1_000_000 = 0.0028
	tests := []struct {
		name         string
		model        string
		input        int
		output       int
		wantCost     float64
	}{
		{
			name:     "haiku 1000 input 500 output",
			model:    "claude-haiku-4-5-20251001",
			input:    1000,
			output:   500,
			wantCost: 0.0028,
		},
		{
			name:     "zero tokens",
			model:    "claude-haiku-4-5-20251001",
			input:    0,
			output:   0,
			wantCost: 0,
		},
		{
			name:     "unknown model no panic",
			model:    "nonexistent-model",
			input:    1000,
			output:   500,
			wantCost: 0,
		},
		{
			name:     "sonnet 1000 input 1000 output",
			model:    "claude-sonnet-4-6",
			input:    1000,
			output:   1000,
			wantCost: (1000*3.00 + 1000*15.00) / 1_000_000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EstimateCost(tc.model, tc.input, tc.output, testPricing)
			const epsilon = 1e-10
			diff := got - tc.wantCost
			if diff < -epsilon || diff > epsilon {
				t.Errorf("EstimateCost(%q, %d, %d) = %v, want %v", tc.model, tc.input, tc.output, got, tc.wantCost)
			}
		})
	}
}

// TestInMemoryCostTracker_ConcurrentSafety launches 100 goroutines recording
// simultaneously and verifies the count matches.
func TestInMemoryCostTracker_ConcurrentSafety(t *testing.T) {
	ctx := context.Background()
	tracker := NewInMemoryCostTracker()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	now := baseTime()
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = tracker.Record(ctx, CostRecord{
				Timestamp:     now,
				Tier:          TierCheap,
				InputTokens:   1,
				OutputTokens:  1,
				EstimatedCost: 0.001,
			})
		}()
	}
	wg.Wait()

	if got := tracker.RecordCount(); got != goroutines {
		t.Errorf("RecordCount() = %d, want %d", got, goroutines)
	}
}

// TestInMemoryCostTracker_PerTierBreakdown verifies that each tier's summary
// is correct when requests span multiple tiers.
func TestInMemoryCostTracker_PerTierBreakdown(t *testing.T) {
	ctx := context.Background()
	tracker := NewInMemoryCostTracker()

	now := baseTime()
	records := []CostRecord{
		// Two cheap requests
		{Timestamp: now, Tier: TierCheap, InputTokens: 100, OutputTokens: 50, EstimatedCost: 0.001},
		{Timestamp: now, Tier: TierCheap, InputTokens: 200, OutputTokens: 100, EstimatedCost: 0.002},
		// One mid request
		{Timestamp: now, Tier: TierMid, InputTokens: 500, OutputTokens: 250, EstimatedCost: 0.005},
		// One frontier request
		{Timestamp: now, Tier: TierFrontier, InputTokens: 1000, OutputTokens: 500, EstimatedCost: 0.05},
	}

	for _, r := range records {
		_ = tracker.Record(ctx, r)
	}

	p := period(now.Add(-time.Hour), now.Add(time.Hour))
	report, err := tracker.Report(ctx, p)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	tests := []struct {
		tier          ModelTier
		wantRequests  int
		wantInput     int
		wantOutput    int
		wantCost      float64
	}{
		{TierCheap, 2, 300, 150, 0.003},
		{TierMid, 1, 500, 250, 0.005},
		{TierFrontier, 1, 1000, 500, 0.05},
	}

	for _, tc := range tests {
		t.Run(tc.tier.String(), func(t *testing.T) {
			s, ok := report.TierCosts[tc.tier]
			if !ok {
				t.Fatalf("TierCosts missing tier %q", tc.tier)
			}
			if s.RequestCount != tc.wantRequests {
				t.Errorf("RequestCount = %d, want %d", s.RequestCount, tc.wantRequests)
			}
			if s.InputTokens != tc.wantInput {
				t.Errorf("InputTokens = %d, want %d", s.InputTokens, tc.wantInput)
			}
			if s.OutputTokens != tc.wantOutput {
				t.Errorf("OutputTokens = %d, want %d", s.OutputTokens, tc.wantOutput)
			}
			const epsilon = 1e-10
			diff := s.EstimatedCost - tc.wantCost
			if diff < -epsilon || diff > epsilon {
				t.Errorf("EstimatedCost = %v, want %v", s.EstimatedCost, tc.wantCost)
			}
		})
	}

	// Verify total
	if got := report.Total.RequestCount; got != 4 {
		t.Errorf("Total.RequestCount = %d, want 4", got)
	}
	if got := report.Total.InputTokens; got != 1800 {
		t.Errorf("Total.InputTokens = %d, want 1800", got)
	}
}
