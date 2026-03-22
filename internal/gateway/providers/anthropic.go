package providers

import (
	"strings"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/gateway"
)

// AnthropicAdapter handles models whose names start with "claude-".
// LiteLLM forwards these to the Anthropic API transparently; the only
// provider-specific quirk is that Anthropic's temperature range is [0, 1].
type AnthropicAdapter struct{}

func (a *AnthropicAdapter) Name() string { return "anthropic" }

// ParseModelName returns true for any model name prefixed with "claude-".
func (a *AnthropicAdapter) ParseModelName(model string) bool {
	return strings.HasPrefix(model, "claude-")
}

// FormatRequest clamps temperature to [0, 1] for Anthropic models.
// All other fields pass through unchanged.
func (a *AnthropicAdapter) FormatRequest(req gateway.CompletionRequest) gateway.CompletionRequest {
	if req.Temperature > 1.0 {
		return gateway.CompletionRequest{
			Model:       req.Model,
			Messages:    req.Messages,
			MaxTokens:   req.MaxTokens,
			Temperature: 1.0,
			Metadata:    req.Metadata,
		}
	}
	return req
}
