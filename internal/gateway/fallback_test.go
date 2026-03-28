package gateway

import (
	"context"
	"errors"
	"fmt"
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

func TestFallbackCompleter_FallbackOnlyModel_SingleAttempt(t *testing.T) {
	cfg := testConfig()
	// "model-fallback-1" appears only in cheap tier's fallback chain, never as a PrimaryModel.
	completers := map[string]Completer{
		"model-fallback-1": &stubCompleter{provider: "p2", resp: CompletionResponse{Content: "direct"}},
	}

	fc := NewFallbackCompleter(completers, cfg)
	resp, err := fc.Complete(context.Background(), CompletionRequest{Model: "model-fallback-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FallbackUsed {
		t.Error("FallbackUsed should be false for single-entry chain")
	}
	if resp.Content != "direct" {
		t.Errorf("Content = %q, want %q", resp.Content, "direct")
	}
}

// cancelAfterNCompleter cancels the given cancel func after n calls to Complete.
type cancelAfterNCompleter struct {
	n      int
	calls  int
	cancel context.CancelFunc
	inner  Completer
}

func (c *cancelAfterNCompleter) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	c.calls++
	resp, err := c.inner.Complete(ctx, req)
	if c.calls >= c.n {
		c.cancel()
	}
	return resp, err
}

func (c *cancelAfterNCompleter) Provider() string { return c.inner.Provider() }

func TestFallbackCompleter_MidChainCancellation_PreservesAccumulatedErrors(t *testing.T) {
	cfg := testConfig()
	ctx, cancel := context.WithCancel(context.Background())

	// Primary fails normally, then context is cancelled after primary's Complete returns.
	// The guard at fallback.go:80 should fire at i=1 with errs already containing
	// the primary's failure.
	primaryErr := errors.New("primary down")
	primary := &cancelAfterNCompleter{
		n:      1,
		cancel: cancel,
		inner:  &stubCompleter{provider: "p1", err: primaryErr},
	}
	completers := map[string]Completer{
		"model-primary":    primary,
		"model-fallback-1": &stubCompleter{provider: "p2", resp: CompletionResponse{Content: "should not reach"}},
		"model-fallback-2": &stubCompleter{provider: "p3", resp: CompletionResponse{Content: "should not reach"}},
	}

	fc := NewFallbackCompleter(completers, cfg)
	_, err := fc.Complete(ctx, CompletionRequest{Model: "model-primary"})
	if err == nil {
		t.Fatal("expected error for mid-chain cancellation, got nil")
	}
	var fe *FallbackError
	if !errors.As(err, &fe) {
		t.Fatalf("error type = %T, want *FallbackError", err)
	}
	// Should have 2 entries: primary failure + context cancellation
	if len(fe.Errors) != 2 {
		t.Fatalf("FallbackError.Errors len = %d, want 2", len(fe.Errors))
	}
	// Entry 0: primary's error
	if fe.Errors[0].Err.Error() != "primary down" {
		t.Errorf("fe.Errors[0].Err = %v, want 'primary down'", fe.Errors[0].Err)
	}
	// Entry 1: context cancellation from the guard
	if !errors.Is(fe.Errors[1].Err, context.Canceled) {
		t.Errorf("fe.Errors[1].Err = %v, want context.Canceled", fe.Errors[1].Err)
	}
}

func TestFallbackCompleter_CancelledContext_StopsChain(t *testing.T) {
	cfg := testConfig()
	completers := map[string]Completer{
		"model-primary":    &stubCompleter{provider: "p1", resp: CompletionResponse{Content: "should not reach"}},
		"model-fallback-1": &stubCompleter{provider: "p2", resp: CompletionResponse{Content: "should not reach"}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling Complete

	fc := NewFallbackCompleter(completers, cfg)
	_, err := fc.Complete(ctx, CompletionRequest{Model: "model-primary"})
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	var fe *FallbackError
	if !errors.As(err, &fe) {
		t.Fatalf("error type = %T, want *FallbackError", err)
	}
	if len(fe.Errors) == 0 {
		t.Fatal("FallbackError.Errors is empty, want at least one entry")
	}
	if !errors.Is(fe.Errors[0].Err, context.Canceled) {
		t.Errorf("fe.Errors[0].Err = %v, want context.Canceled", fe.Errors[0].Err)
	}
}

func TestFallbackError_UnwrapExposesContextCanceled(t *testing.T) {
	cfg := testConfig()
	completers := map[string]Completer{
		"model-primary":    &stubCompleter{provider: "p1", resp: CompletionResponse{Content: "should not reach"}},
		"model-fallback-1": &stubCompleter{provider: "p2", resp: CompletionResponse{Content: "should not reach"}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fc := NewFallbackCompleter(completers, cfg)
	_, err := fc.Complete(ctx, CompletionRequest{Model: "model-primary"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// errors.Is should now find context.Canceled through FallbackError.Unwrap
	if !errors.Is(err, context.Canceled) {
		t.Error("errors.Is(err, context.Canceled) = false, want true")
	}
	// Should still match ErrAllProvidersFailed
	if !errors.Is(err, ErrAllProvidersFailed) {
		t.Error("errors.Is(err, ErrAllProvidersFailed) = false, want true")
	}
}

func TestFallbackError_UnwrapExposesWrappedContextCanceled(t *testing.T) {
	// Verify errors.Is works when context.Canceled is wrapped with extra context,
	// which requires errors.Is (not ==) in FallbackError.Unwrap.
	wrappedCancel := fmt.Errorf("provider timeout: %w", context.Canceled)
	fe := &FallbackError{
		Errors: []ProviderError{
			{Op: "Complete", Provider: "p1", Err: wrappedCancel},
		},
	}
	if !errors.Is(fe, context.Canceled) {
		t.Error("errors.Is(fe, context.Canceled) = false, want true for wrapped context error")
	}
	if !errors.Is(fe, ErrAllProvidersFailed) {
		t.Error("errors.Is(fe, ErrAllProvidersFailed) = false, want true")
	}
}

func TestFallbackCompleter_Provider(t *testing.T) {
	fc := NewFallbackCompleter(nil, GatewayConfig{})
	if got := fc.Provider(); got != "fallback" {
		t.Errorf("Provider() = %q, want %q", got, "fallback")
	}
}

func TestFallbackCompleter_CompleteWithTier_MissingTierConfig(t *testing.T) {
	cfg := testConfig() // has cheap and mid tiers only
	fc := NewFallbackCompleter(map[string]Completer{}, cfg)

	_, err := fc.CompleteWithTier(context.Background(), TierFrontier, CompletionRequest{})
	if err == nil {
		t.Fatal("expected error for missing tier config, got nil")
	}
	if !errors.Is(err, ErrInvalidTier) {
		t.Errorf("error = %v, want wrapping ErrInvalidTier", err)
	}
}

func TestFallbackCompleter_PlainErrorWrapped(t *testing.T) {
	cfg := testConfig()
	completers := map[string]Completer{
		"model-primary":    &stubCompleter{provider: "p1", err: errors.New("plain error")},
		"model-fallback-1": &stubCompleter{provider: "p2", err: errors.New("also plain")},
		"model-fallback-2": &stubCompleter{provider: "p3", resp: CompletionResponse{Content: "ok"}},
	}
	fc := NewFallbackCompleter(completers, cfg)
	resp, err := fc.Complete(context.Background(), CompletionRequest{Model: "model-primary"})
	if err != nil {
		t.Fatalf("expected success after fallback, got error: %v", err)
	}
	if resp.Content != "ok" {
		t.Errorf("Content = %q, want %q", resp.Content, "ok")
	}
}

func TestFallbackCompleter_ChainSkipsMissingCompleter(t *testing.T) {
	cfg := testConfig()
	// Only register primary and fallback-2; fallback-1 is missing from the map.
	// tryChain should skip fallback-1 (ErrNoProvider) and succeed on fallback-2.
	completers := map[string]Completer{
		"model-primary":    &stubCompleter{provider: "p1", err: &ProviderError{Provider: "p1", Err: errors.New("fail")}},
		"model-fallback-2": &stubCompleter{provider: "p3", resp: CompletionResponse{Content: "recovered"}},
	}
	fc := NewFallbackCompleter(completers, cfg)
	resp, err := fc.Complete(context.Background(), CompletionRequest{Model: "model-primary"})
	if err != nil {
		t.Fatalf("expected success after skipping missing completer, got: %v", err)
	}
	if resp.Content != "recovered" {
		t.Errorf("Content = %q, want %q", resp.Content, "recovered")
	}
	if !resp.FallbackUsed {
		t.Error("FallbackUsed = false, want true")
	}
}
