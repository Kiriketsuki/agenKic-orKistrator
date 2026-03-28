// Package providers contains per-provider FormatAdapter implementations that
// adjust CompletionRequests for provider-specific quirks before they are sent
// through the LiteLLM proxy.
package providers

import "github.com/Kiriketsuki/agenKic-orKistrator/internal/gateway"

// FormatAdapter adjusts a CompletionRequest for a specific provider's quirks.
type FormatAdapter interface {
	// Name returns the provider identifier (e.g. "anthropic", "openai", "ollama").
	Name() string

	// FormatRequest returns a (possibly modified) copy of req tailored for this
	// provider. It must not mutate the original request. Note: Go's shallow copy
	// (out := req) shares reference-type fields like Metadata — implementations
	// must not write to shared maps or slices.
	FormatRequest(req gateway.CompletionRequest) gateway.CompletionRequest

	// ParseModelName returns true when this adapter should handle the given model
	// name. The model name is matched before FormatRequest is applied.
	ParseModelName(model string) bool
}

// Registry maps model names to their FormatAdapter.
type Registry struct {
	adapters []FormatAdapter
}

// NewRegistry returns a Registry containing the given adapters. Adapters are
// checked in order; the first match wins.
func NewRegistry(adapters ...FormatAdapter) *Registry {
	return &Registry{adapters: adapters}
}

// DefaultRegistry returns a Registry pre-populated with the built-in adapters
// for Anthropic, OpenAI, and Ollama. Providers handled natively by LiteLLM
// (e.g., Gemini) should use completers without WithAdapterResolver, since
// Find returns ErrNoProvider for model prefixes not covered by these adapters.
func DefaultRegistry() *Registry {
	return NewRegistry(
		&AnthropicAdapter{},
		&OpenAIAdapter{},
		&OllamaAdapter{},
	)
}

// Find returns the FormatAdapter whose ParseModelName reports true for model.
// If no adapter matches, it returns gateway.ErrNoProvider.
func (r *Registry) Find(model string) (FormatAdapter, error) {
	for _, a := range r.adapters {
		if a.ParseModelName(model) {
			return a, nil
		}
	}
	return nil, gateway.ErrNoProvider
}

// Resolve implements gateway.AdapterResolver. It finds the adapter for the
// given model and applies its FormatRequest transformation.
func (r *Registry) Resolve(model string, req gateway.CompletionRequest) (gateway.CompletionRequest, error) {
	adapter, err := r.Find(model)
	if err != nil {
		return req, err
	}
	return adapter.FormatRequest(req), nil
}
