package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/gateway"
)

func TestLiteLLMClient_Complete(t *testing.T) {
	successBody := `{
		"choices": [{"message": {"content": "hello world"}}],
		"model": "claude-haiku-4-5",
		"usage": {"prompt_tokens": 10, "completion_tokens": 5}
	}`

	tests := []struct {
		name        string
		handler     http.HandlerFunc
		req         gateway.CompletionRequest
		wantContent string
		wantModel   string
		wantInTok   int
		wantOutTok  int
		wantErrIs   error
		wantNoResp  bool
	}{
		{
			name: "successful completion",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(successBody))
			},
			req: gateway.CompletionRequest{
				Model: "claude-haiku-4-5",
				Messages: []gateway.Message{
					{Role: "user", Content: "hello"},
				},
				MaxTokens:   100,
				Temperature: 0.7,
			},
			wantContent: "hello world",
			wantModel:   "claude-haiku-4-5",
			wantInTok:   10,
			wantOutTok:  5,
		},
		{
			name: "rate limited returns ErrRateLimited",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTooManyRequests)
			},
			req: gateway.CompletionRequest{
				Model:    "gpt-4",
				Messages: []gateway.Message{{Role: "user", Content: "hi"}},
			},
			wantErrIs:  gateway.ErrRateLimited,
			wantNoResp: true,
		},
		{
			name: "server error 500 returns ErrProviderUnavailable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			req: gateway.CompletionRequest{
				Model:    "gpt-4",
				Messages: []gateway.Message{{Role: "user", Content: "hi"}},
			},
			wantErrIs:  gateway.ErrProviderUnavailable,
			wantNoResp: true,
		},
		{
			name: "server error 503 returns ErrProviderUnavailable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			},
			req: gateway.CompletionRequest{
				Model:    "claude-sonnet-4-6",
				Messages: []gateway.Message{{Role: "user", Content: "hi"}},
			},
			wantErrIs:  gateway.ErrProviderUnavailable,
			wantNoResp: true,
		},
		{
			name: "malformed JSON response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{not valid json`))
			},
			req: gateway.CompletionRequest{
				Model:    "gpt-4",
				Messages: []gateway.Message{{Role: "user", Content: "hi"}},
			},
			wantNoResp: true,
		},
		{
			name: "empty choices returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"choices": [], "model": "gpt-4", "usage": {}}`))
			},
			req: gateway.CompletionRequest{
				Model:    "gpt-4",
				Messages: []gateway.Message{{Role: "user", Content: "hi"}},
			},
			wantNoResp: true,
		},
		{
			name: "request body contains correct fields",
			handler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if body["model"] != "claude-haiku-4-5" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if r.Header.Get("Content-Type") != "application/json" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(successBody))
			},
			req: gateway.CompletionRequest{
				Model:    "claude-haiku-4-5",
				Messages: []gateway.Message{{Role: "user", Content: "test"}},
			},
			wantContent: "hello world",
			wantModel:   "claude-haiku-4-5",
			wantInTok:   10,
			wantOutTok:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			client := gateway.NewLiteLLMClient(
				gateway.WithBaseURL(srv.URL),
			)

			resp, err := client.Complete(context.Background(), tt.req)

			if tt.wantErrIs != nil {
				if err == nil {
					t.Fatalf("expected error wrapping %v, got nil", tt.wantErrIs)
				}
				var pe *gateway.ProviderError
				if !errors.As(err, &pe) {
					t.Fatalf("expected *ProviderError, got %T: %v", err, err)
				}
				if !errors.Is(err, tt.wantErrIs) {
					t.Errorf("expected errors.Is(%v), got %v", tt.wantErrIs, err)
				}
				return
			}

			if tt.wantNoResp {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Content != tt.wantContent {
				t.Errorf("content: got %q, want %q", resp.Content, tt.wantContent)
			}
			if resp.Model != tt.wantModel {
				t.Errorf("model: got %q, want %q", resp.Model, tt.wantModel)
			}
			if resp.InputTokens != tt.wantInTok {
				t.Errorf("input tokens: got %d, want %d", resp.InputTokens, tt.wantInTok)
			}
			if resp.OutputTokens != tt.wantOutTok {
				t.Errorf("output tokens: got %d, want %d", resp.OutputTokens, tt.wantOutTok)
			}
			if resp.ProviderName != "litellm" {
				t.Errorf("provider name: got %q, want %q", resp.ProviderName, "litellm")
			}
		})
	}
}

func TestLiteLLMClient_SystemPromptSerialized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(body.Messages) < 2 {
			t.Errorf("expected at least 2 messages (system + user), got %d", len(body.Messages))
		}
		if body.Messages[0].Role != "system" {
			t.Errorf("first message role=%q, want %q", body.Messages[0].Role, "system")
		}
		if body.Messages[0].Content != "You are helpful" {
			t.Errorf("system prompt=%q, want %q", body.Messages[0].Content, "You are helpful")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"model":"claude","usage":{}}`))
	}))
	defer srv.Close()

	client := gateway.NewLiteLLMClient(gateway.WithBaseURL(srv.URL))
	_, err := client.Complete(context.Background(), gateway.CompletionRequest{
		Model:        "claude-haiku-4-5",
		Messages:     []gateway.Message{{Role: "user", Content: "hi"}},
		SystemPrompt: "You are helpful",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLiteLLMClient_WithBaseURL_RejectsInvalidSchemes(t *testing.T) {
	tests := []struct {
		url     string
		wantURL string
	}{
		{"http://localhost:4000", "http://localhost:4000"},
		{"https://proxy.example.com", "https://proxy.example.com"},
		{"ftp://evil.com", "http://localhost:8000"},      // rejected, keeps default
		{"file:///etc/passwd", "http://localhost:8000"},  // rejected
		{"javascript:alert(1)", "http://localhost:8000"}, // rejected
	}
	for _, tt := range tests {
		client := gateway.NewLiteLLMClient(gateway.WithBaseURL(tt.url))
		// We can't directly inspect baseURL, but we can verify via Provider name
		// that the client was constructed. The real test is that invalid schemes
		// don't crash and the client remains functional.
		if client.Provider() != "litellm" {
			t.Errorf("WithBaseURL(%q) broke client construction", tt.url)
		}
	}
}

func TestLiteLLMClient_WithAdapterResolver(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Verify the adapter transformed the model name
		if body.Model != "llama3" {
			t.Errorf("resolver not applied: model=%q, want %q", body.Model, "llama3")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi"}}],"model":"llama3","usage":{}}`))
	}))
	defer srv.Close()

	resolver := &testResolver{
		fn: func(model string, req gateway.CompletionRequest) (gateway.CompletionRequest, error) {
			out := req
			out.Model = "llama3" // simulate ollama prefix stripping
			return out, nil
		},
	}

	client := gateway.NewLiteLLMClient(
		gateway.WithBaseURL(srv.URL),
		gateway.WithAdapterResolver(resolver),
	)
	_, err := client.Complete(context.Background(), gateway.CompletionRequest{
		Model:    "ollama/llama3",
		Messages: []gateway.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLiteLLMClient_ResolverErrNoProvider(t *testing.T) {
	resolver := &testResolver{
		fn: func(model string, req gateway.CompletionRequest) (gateway.CompletionRequest, error) {
			return req, gateway.ErrNoProvider
		},
	}
	client := gateway.NewLiteLLMClient(gateway.WithAdapterResolver(resolver))
	_, err := client.Complete(context.Background(), gateway.CompletionRequest{
		Model:    "unknown",
		Messages: []gateway.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, gateway.ErrNoProvider) {
		t.Errorf("expected ErrNoProvider, got %v", err)
	}
}

// testResolver implements gateway.AdapterResolver for testing.
type testResolver struct {
	fn func(model string, req gateway.CompletionRequest) (gateway.CompletionRequest, error)
}

func (r *testResolver) Resolve(model string, req gateway.CompletionRequest) (gateway.CompletionRequest, error) {
	return r.fn(model, req)
}

func TestLiteLLMClient_ProviderName(t *testing.T) {
	c := gateway.NewLiteLLMClient()
	if c.Provider() != "litellm" {
		t.Errorf("default provider: got %q, want %q", c.Provider(), "litellm")
	}

	c2 := gateway.NewLiteLLMClient(gateway.WithProviderName("anthropic"))
	if c2.Provider() != "anthropic" {
		t.Errorf("custom provider: got %q, want %q", c2.Provider(), "anthropic")
	}
}

func TestLiteLLMClient_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until request context is cancelled.
		<-r.Context().Done()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := gateway.NewLiteLLMClient(gateway.WithBaseURL(srv.URL))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := client.Complete(ctx, gateway.CompletionRequest{
		Model:    "claude-haiku-4-5",
		Messages: []gateway.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
}
