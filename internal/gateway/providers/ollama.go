package providers

import (
	"strings"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/gateway"
)

// OllamaAdapter handles models whose names start with "ollama/".
// LiteLLM routes "ollama/<model>" to a local Ollama instance; no format
// transformation is required beyond model name recognition.
type OllamaAdapter struct{}

func (a *OllamaAdapter) Name() string { return "ollama" }

// ParseModelName returns true for model names prefixed with "ollama/".
func (a *OllamaAdapter) ParseModelName(model string) bool {
	return strings.HasPrefix(model, "ollama/")
}

// FormatRequest passes the request through unchanged. LiteLLM understands
// the "ollama/<model>" naming convention without further transformation.
func (a *OllamaAdapter) FormatRequest(req gateway.CompletionRequest) gateway.CompletionRequest {
	return req
}
