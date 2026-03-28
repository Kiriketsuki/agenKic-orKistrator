package providers

import (
	"strings"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/gateway"
)

// OllamaAdapter handles models whose names start with "ollama/".
// FormatRequest strips the "ollama/" prefix so the bare model name
// (e.g. "llama3") reaches LiteLLM. LiteLLM must be configured with
// model aliases or an ollama provider entry that maps bare names to
// the local Ollama instance.
type OllamaAdapter struct{}

func (a *OllamaAdapter) Name() string { return "ollama" }

// ParseModelName returns true for model names prefixed with "ollama/".
func (a *OllamaAdapter) ParseModelName(model string) bool {
	return strings.HasPrefix(model, "ollama/")
}

// FormatRequest strips the "ollama/" prefix from the model name before
// forwarding to LiteLLM. All other fields pass through unchanged.
func (a *OllamaAdapter) FormatRequest(req gateway.CompletionRequest) gateway.CompletionRequest {
	out := req
	out.Model = strings.TrimPrefix(req.Model, "ollama/")
	return out
}
