package providers

import (
	"strings"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/gateway"
)

// OpenAIAdapter handles models whose names start with "gpt-", "o1-", or "o3-".
// LiteLLM uses the OpenAI API format natively, so this is mostly a pass-through
// adapter. The one quirk handled here is that OpenAI reasoning models (o1-*, o3-*)
// do not support a temperature parameter — it is zeroed out for those models.
type OpenAIAdapter struct{}

func (a *OpenAIAdapter) Name() string { return "openai" }

// ParseModelName returns true for model names prefixed with "gpt-", "o1-", or "o3-".
func (a *OpenAIAdapter) ParseModelName(model string) bool {
	return strings.HasPrefix(model, "gpt-") ||
		strings.HasPrefix(model, "o1-") ||
		strings.HasPrefix(model, "o3-")
}

// FormatRequest passes through the request unchanged for standard GPT models.
// For o1-* and o3-* reasoning models, temperature is zeroed out since those
// models do not accept a temperature parameter.
func (a *OpenAIAdapter) FormatRequest(req gateway.CompletionRequest) gateway.CompletionRequest {
	if strings.HasPrefix(req.Model, "o1-") || strings.HasPrefix(req.Model, "o3-") {
		out := req           // shallow copy preserves all fields
		out.Temperature = -1 // negative → use provider default (omitted in serialisation)
		return out
	}
	return req
}
