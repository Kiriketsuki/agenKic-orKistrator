package gateway

import (
	"context"
	"errors"
	"testing"
)

// stubCompleter is a test double for the Completer interface used in fallback tests.
type stubCompleter struct {
	provider string
	resp     CompletionResponse
	err      error
}

func (s *stubCompleter) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	if s.err != nil {
		return CompletionResponse{}, s.err
	}
	resp := s.resp
	resp.Model = req.Model
	resp.ProviderName = s.provider
	return resp, nil
}

func (s *stubCompleter) Provider() string { return s.provider }

// testConfig builds a minimal GatewayConfig for the fallback tests.
func testConfig() GatewayConfig {
	return GatewayConfig{
		Gateway: GatewaySettings{
			LiteLLMBaseURL: "http://localhost:4000",
			TimeoutSeconds: 30,
		},
		Tiers: map[ModelTier]TierConfig{
			TierCheap: {
				Tier:          TierCheap,
				PrimaryModel:  "model-primary",
				FallbackChain: []string{"model-fallback-1", "model-fallback-2"},
			},
			TierMid: {
				Tier:          TierMid,
				PrimaryModel:  "model-mid",
				FallbackChain: []string{"model-mid-fallback"},
			},
		},
	}
}

func TestFallbackCompleter_PrimarySucceeds(t *testing.T) {
	cfg := testConfig()
	completers := map[string]Completer{
		"model-primary":    &stubCompleter{provider: "p1", resp: CompletionResponse{Content: "hello"}},
		"model-fallback-1": &stubCompleter{provider: "p2", err: errors.New("should not be called")},
	}

	fc := NewFallbackCompleter(completers, cfg)
	resp, err := fc.Complete(context.Background(), CompletionRequest{Model: "model-primary"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FallbackUsed {
		t.Error("FallbackUsed should be false when primary succeeds")
	}
	if resp.Content != "hello" {
		t.Errorf("Content = %q, want %q", resp.Content, "hello")
	}
}

func TestFallbackCompleter_PrimaryFails_FirstFallbackSucceeds(t *testing.T) {
	cfg := testConfig()
	completers := map[string]Completer{
		"model-primary":    &stubCompleter{provider: "p1", err: &ProviderError{Provider: "p1", Err: errors.New("down")}},
		"model-fallback-1": &stubCompleter{provider: "p2", resp: CompletionResponse{Content: "fallback1"}},
		"model-fallback-2": &stubCompleter{provider: "p3", err: errors.New("should not be called")},
	}

	fc := NewFallbackCompleter(completers, cfg)
	resp, err := fc.Complete(context.Background(), CompletionRequest{Model: "model-primary"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.FallbackUsed {
		t.Error("FallbackUsed should be true when falling back")
	}
	if resp.Content != "fallback1" {
		t.Errorf("Content = %q, want fallback1", resp.Content)
	}
}

func TestFallbackCompleter_PrimaryAndFirstFail_SecondSucceeds(t *testing.T) {
	cfg := testConfig()
	completers := map[string]Completer{
		"model-primary":    &stubCompleter{provider: "p1", err: errors.New("fail")},
		"model-fallback-1": &stubCompleter{provider: "p2", err: errors.New("also fail")},
		"model-fallback-2": &stubCompleter{provider: "p3", resp: CompletionResponse{Content: "fallback2"}},
	}

	fc := NewFallbackCompleter(completers, cfg)
	resp, err := fc.Complete(context.Background(), CompletionRequest{Model: "model-primary"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.FallbackUsed {
		t.Error("FallbackUsed should be true")
	}
	if resp.Content != "fallback2" {
		t.Errorf("Content = %q, want fallback2", resp.Content)
	}
}

func TestFallbackCompleter_AllFail_ReturnsFallbackError(t *testing.T) {
	cfg := testConfig()
	completers := map[string]Completer{
		"model-primary":    &stubCompleter{provider: "p1", err: errors.New("fail1")},
		"model-fallback-1": &stubCompleter{provider: "p2", err: errors.New("fail2")},
		"model-fallback-2": &stubCompleter{provider: "p3", err: errors.New("fail3")},
	}

	fc := NewFallbackCompleter(completers, cfg)
	_, err := fc.Complete(context.Background(), CompletionRequest{Model: "model-primary"})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}

	var fe *FallbackError
	if !errors.As(err, &fe) {
		t.Fatalf("error = %T, want *FallbackError", err)
	}
	if len(fe.Errors) != 3 {
		t.Errorf("FallbackError.Errors len = %d, want 3", len(fe.Errors))
	}
	if !errors.Is(err, ErrAllProvidersFailed) {
		t.Error("error should unwrap to ErrAllProvidersFailed")
	}
}

func TestFallbackCompleter_UnknownModel_ReturnsErrNoProvider(t *testing.T) {
	cfg := testConfig()
	fc := NewFallbackCompleter(map[string]Completer{}, cfg)

	_, err := fc.Complete(context.Background(), CompletionRequest{Model: "unknown-model"})
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
	if !errors.Is(err, ErrNoProvider) {
		t.Errorf("error = %v, want wrapping ErrNoProvider", err)
	}
}

func TestFallbackCompleter_CompleteWithTier_PrimarySucceeds(t *testing.T) {
	cfg := testConfig()
	completers := map[string]Completer{
		"model-mid": &stubCompleter{provider: "mid", resp: CompletionResponse{Content: "mid-response"}},
	}

	fc := NewFallbackCompleter(completers, cfg)
	resp, err := fc.CompleteWithTier(context.Background(), TierMid, CompletionRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FallbackUsed {
		t.Error("FallbackUsed should be false")
	}
	if resp.Content != "mid-response" {
		t.Errorf("Content = %q, want mid-response", resp.Content)
	}
}

func TestFallbackCompleter_CompleteWithTier_InvalidTier(t *testing.T) {
	cfg := testConfig()
	fc := NewFallbackCompleter(map[string]Completer{}, cfg)

	_, err := fc.CompleteWithTier(context.Background(), ModelTier("bogus"), CompletionRequest{})
	if err == nil {
		t.Fatal("expected error for invalid tier")
	}
	if !errors.Is(err, ErrInvalidTier) {
		t.Errorf("error = %v, want ErrInvalidTier", err)
	}
}
