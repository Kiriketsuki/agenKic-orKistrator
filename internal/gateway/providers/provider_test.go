package providers_test

import (
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/gateway"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/gateway/providers"
)

// ── Registry ────────────────────────────────────────────────────────────────

func TestRegistry_Find(t *testing.T) {
	reg := providers.DefaultRegistry()

	tests := []struct {
		model        string
		wantFound    bool
		wantProvider string
	}{
		{"claude-haiku-4-5", true, "anthropic"},
		{"claude-sonnet-4-6", true, "anthropic"},
		{"claude-opus-4-6", true, "anthropic"},
		{"gpt-4o", true, "openai"},
		{"gpt-3.5-turbo", true, "openai"},
		{"o1-preview", true, "openai"},
		{"o3-mini", true, "openai"},
		{"ollama/llama3", true, "ollama"},
		{"ollama/mistral:7b", true, "ollama"},
		{"unknown-model", false, ""},
		{"gemini-pro", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			adapter, found := reg.Find(tt.model)
			if found != tt.wantFound {
				t.Errorf("Find(%q) found=%v, want %v", tt.model, found, tt.wantFound)
			}
			if tt.wantFound {
				if adapter.Name() != tt.wantProvider {
					t.Errorf("Find(%q) provider=%q, want %q", tt.model, adapter.Name(), tt.wantProvider)
				}
			}
		})
	}
}

func TestRegistry_EmptyRegistry(t *testing.T) {
	reg := providers.NewRegistry()
	_, found := reg.Find("claude-haiku-4-5")
	if found {
		t.Error("empty registry should not find any adapter")
	}
}

// ── AnthropicAdapter ────────────────────────────────────────────────────────

func TestAnthropicAdapter_ParseModelName(t *testing.T) {
	a := &providers.AnthropicAdapter{}
	tests := []struct {
		model string
		want  bool
	}{
		{"claude-haiku-4-5", true},
		{"claude-sonnet-4-6", true},
		{"claude-opus-4-6", true},
		{"claude-", true},
		{"gpt-4", false},
		{"ollama/llama3", false},
		{"o1-preview", false},
	}
	for _, tt := range tests {
		if got := a.ParseModelName(tt.model); got != tt.want {
			t.Errorf("AnthropicAdapter.ParseModelName(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestAnthropicAdapter_FormatRequest_ClampsTemperature(t *testing.T) {
	a := &providers.AnthropicAdapter{}

	req := gateway.CompletionRequest{
		Model:       "claude-haiku-4-5",
		Messages:    []gateway.Message{{Role: "user", Content: "hi"}},
		MaxTokens:   100,
		Temperature: 1.5, // above Anthropic's max of 1.0
	}
	got := a.FormatRequest(req)
	if got.Temperature != 1.0 {
		t.Errorf("temperature not clamped: got %v, want 1.0", got.Temperature)
	}
	// Other fields unchanged.
	if got.Model != req.Model {
		t.Errorf("model changed: got %q, want %q", got.Model, req.Model)
	}
	if got.MaxTokens != req.MaxTokens {
		t.Errorf("max_tokens changed: got %d, want %d", got.MaxTokens, req.MaxTokens)
	}
}

func TestAnthropicAdapter_FormatRequest_PassthroughValid(t *testing.T) {
	a := &providers.AnthropicAdapter{}

	req := gateway.CompletionRequest{
		Model:       "claude-sonnet-4-6",
		Messages:    []gateway.Message{{Role: "user", Content: "test"}},
		Temperature: 0.7,
	}
	got := a.FormatRequest(req)
	if got.Temperature != 0.7 {
		t.Errorf("temperature changed: got %v, want 0.7", got.Temperature)
	}
}

func TestAnthropicAdapter_FormatRequest_PreservesAllFields(t *testing.T) {
	a := &providers.AnthropicAdapter{}

	req := gateway.CompletionRequest{
		Model:        "claude-haiku-4-5",
		Messages:     []gateway.Message{{Role: "user", Content: "hi"}},
		SystemPrompt: "You are a helpful assistant",
		MaxTokens:    100,
		Temperature:  1.5, // triggers clamping
		Stream:       true,
		Tier:         gateway.TierCheap,
		Metadata:     map[string]string{"key": "val"},
	}
	got := a.FormatRequest(req)

	if got.Temperature != 1.0 {
		t.Errorf("temperature not clamped: got %v, want 1.0", got.Temperature)
	}
	if got.SystemPrompt != req.SystemPrompt {
		t.Errorf("SystemPrompt dropped: got %q, want %q", got.SystemPrompt, req.SystemPrompt)
	}
	if got.Stream != req.Stream {
		t.Errorf("Stream dropped: got %v, want %v", got.Stream, req.Stream)
	}
	if got.Tier != req.Tier {
		t.Errorf("Tier dropped: got %v, want %v", got.Tier, req.Tier)
	}
	if got.Metadata["key"] != "val" {
		t.Errorf("Metadata dropped: got %v, want map[key:val]", got.Metadata)
	}
}

// ── OpenAIAdapter ────────────────────────────────────────────────────────────

func TestOpenAIAdapter_ParseModelName(t *testing.T) {
	a := &providers.OpenAIAdapter{}
	tests := []struct {
		model string
		want  bool
	}{
		{"gpt-4o", true},
		{"gpt-3.5-turbo", true},
		{"gpt-4-turbo", true},
		{"o1-preview", true},
		{"o1-mini", true},
		{"o3-mini", true},
		{"claude-haiku-4-5", false},
		{"ollama/llama3", false},
	}
	for _, tt := range tests {
		if got := a.ParseModelName(tt.model); got != tt.want {
			t.Errorf("OpenAIAdapter.ParseModelName(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestOpenAIAdapter_FormatRequest_ReasoningModels(t *testing.T) {
	a := &providers.OpenAIAdapter{}

	for _, model := range []string{"o1-preview", "o1-mini", "o3-mini"} {
		req := gateway.CompletionRequest{
			Model:       model,
			Messages:    []gateway.Message{{Role: "user", Content: "hi"}},
			Temperature: 0.7,
		}
		got := a.FormatRequest(req)
		if got.Temperature >= 0 {
			t.Errorf("reasoning model %q: temperature should be negative (omit), got %v", model, got.Temperature)
		}
	}
}

func TestOpenAIAdapter_FormatRequest_GPTPassthrough(t *testing.T) {
	a := &providers.OpenAIAdapter{}

	req := gateway.CompletionRequest{
		Model:       "gpt-4o",
		Messages:    []gateway.Message{{Role: "user", Content: "hi"}},
		Temperature: 0.5,
		MaxTokens:   200,
	}
	got := a.FormatRequest(req)
	if got.Temperature != req.Temperature {
		t.Errorf("gpt temperature changed: got %v, want %v", got.Temperature, req.Temperature)
	}
	if got.MaxTokens != req.MaxTokens {
		t.Errorf("max_tokens changed: got %d, want %d", got.MaxTokens, req.MaxTokens)
	}
}

func TestOpenAIAdapter_FormatRequest_PreservesAllFields(t *testing.T) {
	a := &providers.OpenAIAdapter{}

	req := gateway.CompletionRequest{
		Model:        "o1-preview",
		Messages:     []gateway.Message{{Role: "user", Content: "hi"}},
		SystemPrompt: "You are a reasoning engine",
		MaxTokens:    200,
		Temperature:  0.7,
		Stream:       true,
		Tier:         gateway.TierFrontier,
		Metadata:     map[string]string{"task": "reason"},
	}
	got := a.FormatRequest(req)

	if got.Temperature >= 0 {
		t.Errorf("temperature should be negative for reasoning model, got %v", got.Temperature)
	}
	if got.SystemPrompt != req.SystemPrompt {
		t.Errorf("SystemPrompt dropped: got %q, want %q", got.SystemPrompt, req.SystemPrompt)
	}
	if got.Stream != req.Stream {
		t.Errorf("Stream dropped: got %v, want %v", got.Stream, req.Stream)
	}
	if got.Tier != req.Tier {
		t.Errorf("Tier dropped: got %v, want %v", got.Tier, req.Tier)
	}
	if got.MaxTokens != req.MaxTokens {
		t.Errorf("MaxTokens dropped: got %d, want %d", got.MaxTokens, req.MaxTokens)
	}
	if got.Metadata["task"] != "reason" {
		t.Errorf("Metadata dropped: got %v, want map[task:reason]", got.Metadata)
	}
}

// ── OllamaAdapter ────────────────────────────────────────────────────────────

func TestOllamaAdapter_ParseModelName(t *testing.T) {
	a := &providers.OllamaAdapter{}
	tests := []struct {
		model string
		want  bool
	}{
		{"ollama/llama3", true},
		{"ollama/mistral:7b", true},
		{"ollama/codellama", true},
		{"llama3", false},
		{"gpt-4", false},
		{"claude-haiku-4-5", false},
	}
	for _, tt := range tests {
		if got := a.ParseModelName(tt.model); got != tt.want {
			t.Errorf("OllamaAdapter.ParseModelName(%q) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestOllamaAdapter_FormatRequest_Passthrough(t *testing.T) {
	a := &providers.OllamaAdapter{}

	req := gateway.CompletionRequest{
		Model:       "ollama/llama3",
		Messages:    []gateway.Message{{Role: "user", Content: "hi"}},
		Temperature: 0.8,
		MaxTokens:   512,
	}
	got := a.FormatRequest(req)
	if got.Model != req.Model {
		t.Errorf("model changed: got %q, want %q", got.Model, req.Model)
	}
	if got.Temperature != req.Temperature {
		t.Errorf("temperature changed: got %v, want %v", got.Temperature, req.Temperature)
	}
	if got.MaxTokens != req.MaxTokens {
		t.Errorf("max_tokens changed: got %d, want %d", got.MaxTokens, req.MaxTokens)
	}
}

func TestAdapterNames(t *testing.T) {
	tests := []struct {
		adapter providers.FormatAdapter
		want    string
	}{
		{&providers.AnthropicAdapter{}, "anthropic"},
		{&providers.OpenAIAdapter{}, "openai"},
		{&providers.OllamaAdapter{}, "ollama"},
	}
	for _, tt := range tests {
		if got := tt.adapter.Name(); got != tt.want {
			t.Errorf("%T.Name() = %q, want %q", tt.adapter, got, tt.want)
		}
	}
}
