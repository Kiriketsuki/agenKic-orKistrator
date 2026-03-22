package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestModelTier_Valid(t *testing.T) {
	tests := []struct {
		tier ModelTier
		want bool
	}{
		{TierCheap, true},
		{TierMid, true},
		{TierFrontier, true},
		{ModelTier("unknown"), false},
		{ModelTier(""), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := tt.tier.Valid(); got != tt.want {
				t.Fatalf("ModelTier(%q).Valid() = %v, want %v", tt.tier, got, tt.want)
			}
		})
	}
}

func TestModelTier_String(t *testing.T) {
	tests := []struct {
		tier ModelTier
		want string
	}{
		{TierCheap, "cheap"},
		{TierMid, "mid"},
		{TierFrontier, "frontier"},
	}
	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.want {
			t.Fatalf("ModelTier.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestModelTier_MarshalText_RoundTrip(t *testing.T) {
	tiers := []ModelTier{TierCheap, TierMid, TierFrontier}
	for _, tier := range tiers {
		t.Run(string(tier), func(t *testing.T) {
			b, err := tier.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() error: %v", err)
			}

			var got ModelTier
			if err := got.UnmarshalText(b); err != nil {
				t.Fatalf("UnmarshalText() error: %v", err)
			}
			if got != tier {
				t.Fatalf("round-trip failed: got %q, want %q", got, tier)
			}
		})
	}
}

func TestModelTier_MarshalText_Invalid(t *testing.T) {
	invalid := ModelTier("bogus")
	_, err := invalid.MarshalText()
	if !errors.Is(err, ErrInvalidTier) {
		t.Fatalf("MarshalText(bogus) error = %v, want ErrInvalidTier", err)
	}
}

func TestModelTier_UnmarshalText_Invalid(t *testing.T) {
	var tier ModelTier
	err := tier.UnmarshalText([]byte("bogus"))
	if !errors.Is(err, ErrInvalidTier) {
		t.Fatalf("UnmarshalText(bogus) error = %v, want ErrInvalidTier", err)
	}
}

func TestTimePeriod_Today(t *testing.T) {
	now := time.Now().UTC()
	p := Today()

	wantStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	wantEnd := wantStart.AddDate(0, 0, 1)

	if !p.Start.Equal(wantStart) {
		t.Fatalf("Today().Start = %v, want %v", p.Start, wantStart)
	}
	if !p.End.Equal(wantEnd) {
		t.Fatalf("Today().End = %v, want %v", p.End, wantEnd)
	}
}

func TestTimePeriod_LastNDays(t *testing.T) {
	now := time.Now().UTC()
	p := LastNDays(7)

	wantStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -7)

	if !p.Start.Equal(wantStart) {
		t.Fatalf("LastNDays(7).Start = %v, want %v", p.Start, wantStart)
	}
	if p.End.Before(now.Add(-time.Second)) {
		t.Fatalf("LastNDays(7).End = %v, expected close to now %v", p.End, now)
	}
}

func TestTimePeriod_Since(t *testing.T) {
	ref := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	before := time.Now().UTC()
	p := Since(ref)

	if !p.Start.Equal(ref) {
		t.Fatalf("Since().Start = %v, want %v", p.Start, ref)
	}
	if p.End.Before(before) {
		t.Fatalf("Since().End = %v, expected >= %v", p.End, before)
	}
}

func TestTimePeriod_LastNDays_Negative(t *testing.T) {
	now := time.Now().UTC()
	p := LastNDays(-5)

	// Negative n should be clamped to 0 (today only).
	wantStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if !p.Start.Equal(wantStart) {
		t.Fatalf("LastNDays(-5).Start = %v, want %v (today midnight)", p.Start, wantStart)
	}
	if p.End.Before(p.Start) {
		t.Fatalf("LastNDays(-5) produced inverted interval: Start=%v, End=%v", p.Start, p.End)
	}
}

func TestCompletionRequest_MultiTurn(t *testing.T) {
	req := CompletionRequest{
		Model: "claude-sonnet-4-6",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there"},
			{Role: "user", Content: "What's the weather?"},
		},
		SystemPrompt: "Be helpful",
		MaxTokens:    1024,
		Temperature:  0.7,
		Stream:       true,
		Tier:         TierMid,
	}

	if len(req.Messages) != 3 {
		t.Fatalf("len(Messages) = %d, want 3", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content != "Be helpful" {
		t.Fatalf("Messages[0] = %+v, want {system, Be helpful}", req.Messages[0])
	}
	if req.Messages[1].Role != "user" || req.Messages[1].Content != "Hello" {
		t.Fatalf("Messages[1] = %+v, want {user, Hello}", req.Messages[1])
	}
	if req.Messages[2].Role != "assistant" || req.Messages[2].Content != "Hi there" {
		t.Fatalf("Messages[2] = %+v, want {assistant, Hi there}", req.Messages[2])
	}
	if req.SystemPrompt != "Be helpful" {
		t.Fatalf("SystemPrompt = %q, want %q", req.SystemPrompt, "Be helpful")
	}
	if req.Stream != true {
		t.Fatalf("Stream = %v, want true", req.Stream)
	}
	if req.Tier != TierMid {
		t.Fatalf("Tier = %q, want %q", req.Tier, TierMid)
	}
}

func TestCompletionResponse_FallbackUsed(t *testing.T) {
	resp := CompletionResponse{
		Content:      "Hello!",
		Model:        "gpt-4o",
		InputTokens:  10,
		OutputTokens: 5,
		FallbackUsed: true,
		ProviderName: "openai",
	}

	if !resp.FallbackUsed {
		t.Fatalf("FallbackUsed = false, want true")
	}
	if resp.ProviderName != "openai" {
		t.Fatalf("ProviderName = %q, want %q", resp.ProviderName, "openai")
	}
}

func TestProviderError_Unwrap(t *testing.T) {
	err := &ProviderError{
		Op:       "Complete",
		Provider: "anthropic",
		Err:      ErrProviderUnavailable,
	}
	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("errors.Is(ProviderError, ErrProviderUnavailable) = false, want true")
	}
}

func TestProviderError_ErrorString(t *testing.T) {
	err := &ProviderError{
		Op:       "Complete",
		Provider: "anthropic",
		Err:      ErrProviderUnavailable,
	}
	got := err.Error()
	want := "gateway: Complete: provider anthropic: gateway: provider unavailable"
	if got != want {
		t.Fatalf("ProviderError.Error() = %q, want %q", got, want)
	}
}

func TestProviderError_NoOp(t *testing.T) {
	err := &ProviderError{
		Provider: "openai",
		Err:      ErrRateLimited,
	}
	got := err.Error()
	want := "gateway: provider openai: gateway: provider rate limited"
	if got != want {
		t.Fatalf("ProviderError.Error() = %q, want %q", got, want)
	}
}

func TestFallbackError_Unwrap(t *testing.T) {
	err := &FallbackError{
		Errors: []ProviderError{
			{Provider: "anthropic", Err: ErrProviderUnavailable},
			{Provider: "openai", Err: ErrRateLimited},
		},
	}
	if !errors.Is(err, ErrAllProvidersFailed) {
		t.Fatalf("errors.Is(FallbackError, ErrAllProvidersFailed) = false, want true")
	}
}

func TestFallbackError_ErrorString(t *testing.T) {
	err := &FallbackError{
		Errors: []ProviderError{
			{Provider: "anthropic", Err: ErrProviderUnavailable},
			{Op: "Complete", Provider: "openai", Err: ErrRateLimited},
		},
	}
	got := err.Error()
	want := "gateway: all providers failed:\n" +
		"  gateway: provider anthropic: gateway: provider unavailable\n" +
		"  gateway: Complete: provider openai: gateway: provider rate limited"
	if got != want {
		t.Fatalf("FallbackError.Error() =\n%q\nwant:\n%q", got, want)
	}
}

func TestProviderConfig_String_RedactsAPIKey(t *testing.T) {
	cfg := ProviderConfig{
		Name:    "anthropic",
		BaseURL: "https://api.anthropic.com",
		APIKey:  "sk-ant-secret-key-12345",
		Models:  []string{"claude-sonnet-4-6"},
	}
	got := cfg.String()
	if got != "ProviderConfig{Name: anthropic, BaseURL: https://api.anthropic.com, APIKey: [REDACTED]}" {
		t.Fatalf("String() = %q, want redacted APIKey", got)
	}
	if strings.Contains(got, "sk-ant-secret-key-12345") {
		t.Fatalf("String() leaked APIKey: %q", got)
	}
}

// TestInterfaceComposition verifies at compile time that a struct delegating
// to Router, Completer, and CostTracker can satisfy the Gateway interface.
func TestInterfaceComposition(t *testing.T) {
	// This is a compile-time check. If the interfaces are incompatible,
	// this file will not compile.
	var _ Gateway = (*compositeGateway)(nil)
}

type compositeGateway struct {
	router  Router
	comp    Completer
	tracker CostTracker
}

func (g *compositeGateway) Route(ctx context.Context, task TaskSpec) (RoutingDecision, error) {
	return g.router.Classify(ctx, task)
}

func (g *compositeGateway) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	return g.comp.Complete(ctx, req)
}

func (g *compositeGateway) GetCostReport(ctx context.Context, period TimePeriod) (CostReport, error) {
	return g.tracker.Report(ctx, period)
}
