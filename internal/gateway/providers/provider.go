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
	// provider. It must not mutate the original request.
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
// for Anthropic, OpenAI, and Ollama.
func DefaultRegistry() *Registry {
	return NewRegistry(
		&AnthropicAdapter{},
		&OpenAIAdapter{},
		&OllamaAdapter{},
	)
}

// Find returns the FormatAdapter whose ParseModelName reports true for model,
// and the boolean is true. If no adapter matches, (nil, false) is returned.
func (r *Registry) Find(model string) (FormatAdapter, bool) {
	for _, a := range r.adapters {
		if a.ParseModelName(model) {
			return a, true
		}
	}
	return nil, false
}
