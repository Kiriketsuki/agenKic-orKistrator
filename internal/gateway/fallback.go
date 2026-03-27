package gateway

import (
	"context"
	"errors"
	"fmt"
)

// Compile-time interface assertion.
var _ Completer = (*FallbackCompleter)(nil)

// FallbackCompleter wraps a set of per-model Completers and applies the
// fallback chain defined in GatewayConfig when the primary model fails.
type FallbackCompleter struct {
	completers map[string]Completer
	config     GatewayConfig
}

// NewFallbackCompleter returns a FallbackCompleter backed by the given
// model-keyed completers and configuration.
func NewFallbackCompleter(completers map[string]Completer, config GatewayConfig) *FallbackCompleter {
	return &FallbackCompleter{
		completers: completers,
		config:     config,
	}
}

// Provider returns the logical provider name for this fallback dispatcher.
func (fc *FallbackCompleter) Provider() string { return "fallback" }

// Complete resolves the model in req to a tier, then attempts the primary
// model and each fallback in order. The first success is returned.
// If all attempts fail, a *FallbackError is returned.
func (fc *FallbackCompleter) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	chain, err := fc.chainForModel(req.Model)
	if err != nil {
		return CompletionResponse{}, err
	}
	return fc.tryChain(ctx, req, chain)
}

// CompleteWithTier resolves tier to its primary model, then applies the
// fallback chain from the tier configuration.
func (fc *FallbackCompleter) CompleteWithTier(ctx context.Context, tier ModelTier, req CompletionRequest) (CompletionResponse, error) {
	if !tier.Valid() {
		return CompletionResponse{}, fmt.Errorf("%w: %q", ErrInvalidTier, tier)
	}

	tc, ok := fc.config.Tiers[tier]
	if !ok {
		return CompletionResponse{}, fmt.Errorf("%w: no tier config for %q", ErrInvalidTier, tier)
	}

	r := req
	r.Model = tc.PrimaryModel
	chain := append([]string{tc.PrimaryModel}, tc.FallbackChain...)
	return fc.tryChain(ctx, r, chain)
}

// chainForModel returns the ordered list of models to try for the given model
// name, looking up the tier config that owns it.
func (fc *FallbackCompleter) chainForModel(model string) ([]string, error) {
	for _, tc := range fc.config.Tiers {
		if tc.PrimaryModel == model {
			return append([]string{model}, tc.FallbackChain...), nil
		}
	}
	// Model not found in any tier — still try it directly (single-entry chain).
	if _, ok := fc.completers[model]; ok {
		return []string{model}, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrNoProvider, model)
}

// tryChain iterates through the model chain, returning on the first success.
func (fc *FallbackCompleter) tryChain(ctx context.Context, req CompletionRequest, chain []string) (CompletionResponse, error) {
	var errs []ProviderError

	for i, model := range chain {
		if err := ctx.Err(); err != nil {
			errs = append(errs, ProviderError{Provider: model, Err: err})
			return CompletionResponse{}, &FallbackError{Errors: errs}
		}

		c, ok := fc.completers[model]
		if !ok {
			errs = append(errs, ProviderError{
				Provider: model,
				Err:      fmt.Errorf("%w: %q", ErrNoProvider, model),
			})
			continue
		}

		r := req
		r.Model = model
		resp, err := c.Complete(ctx, r)
		if err != nil {
			var pe *ProviderError
			if errors.As(err, &pe) {
				errs = append(errs, *pe)
			} else {
				errs = append(errs, ProviderError{Provider: c.Provider(), Err: err})
			}
			continue
		}

		resp.FallbackUsed = i > 0
		return resp, nil
	}

	return CompletionResponse{}, &FallbackError{Errors: errs}
}
