package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "http://localhost:8000"
	defaultTimeout = 30 * time.Second
)

// liteLLMRequest is the OpenAI-compatible request body sent to the LiteLLM proxy.
type liteLLMRequest struct {
	Model       string           `json:"model"`
	Messages    []liteLLMMessage `json:"messages"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
}

type liteLLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// liteLLMResponse is the OpenAI-compatible response from the LiteLLM proxy.
type liteLLMResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// liteLLMErrorBody is the error response body returned by the LiteLLM proxy.
type liteLLMErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// LiteLLMClient implements Completer against a LiteLLM proxy endpoint.
type LiteLLMClient struct {
	baseURL      string
	timeout      time.Duration
	httpClient   *http.Client
	providerName string
}

// LiteLLMOption configures the LiteLLMClient.
type LiteLLMOption func(*LiteLLMClient)

// WithBaseURL sets the base URL of the LiteLLM proxy (default: http://localhost:8000).
func WithBaseURL(u string) LiteLLMOption {
	return func(c *LiteLLMClient) { c.baseURL = u }
}

// WithTimeout sets the HTTP request timeout (default: 30s).
func WithTimeout(d time.Duration) LiteLLMOption {
	return func(c *LiteLLMClient) {
		c.timeout = d
		c.httpClient = &http.Client{Timeout: d}
	}
}

// WithHTTPClient replaces the underlying *http.Client entirely.
func WithHTTPClient(client *http.Client) LiteLLMOption {
	return func(c *LiteLLMClient) { c.httpClient = client }
}

// WithProviderName sets the provider name returned by Provider() (default: "litellm").
func WithProviderName(name string) LiteLLMOption {
	return func(c *LiteLLMClient) { c.providerName = name }
}

// NewLiteLLMClient returns a LiteLLMClient with the given options applied.
func NewLiteLLMClient(opts ...LiteLLMOption) *LiteLLMClient {
	c := &LiteLLMClient{
		baseURL:      defaultBaseURL,
		timeout:      defaultTimeout,
		providerName: "litellm",
	}
	c.httpClient = &http.Client{Timeout: c.timeout}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Provider implements Completer.
func (c *LiteLLMClient) Provider() string { return c.providerName }

// Complete sends a completion request to the LiteLLM proxy and returns the response.
// It implements the Completer interface.
func (c *LiteLLMClient) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	body, err := c.buildRequest(req)
	if err != nil {
		return CompletionResponse{}, &ProviderError{Provider: c.providerName, Err: fmt.Errorf("marshal request: %w", err)}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return CompletionResponse{}, &ProviderError{Provider: c.providerName, Err: fmt.Errorf("build http request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, &ProviderError{Provider: c.providerName, Err: fmt.Errorf("%w: %s", ErrProviderUnavailable, err.Error())}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return CompletionResponse{}, &ProviderError{Provider: c.providerName, Err: ErrRateLimited}
	}
	if resp.StatusCode >= 500 {
		return CompletionResponse{}, &ProviderError{Provider: c.providerName, Err: fmt.Errorf("%w: status %d", ErrProviderUnavailable, resp.StatusCode)}
	}
	if resp.StatusCode != http.StatusOK {
		var errBody liteLLMErrorBody
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		msg := errBody.Error.Message
		if msg == "" {
			msg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return CompletionResponse{}, &ProviderError{Provider: c.providerName, Err: fmt.Errorf("unexpected response: %s", msg)}
	}

	var liteLLMResp liteLLMResponse
	if err := json.NewDecoder(resp.Body).Decode(&liteLLMResp); err != nil {
		return CompletionResponse{}, &ProviderError{Provider: c.providerName, Err: fmt.Errorf("decode response: %w", err)}
	}

	if len(liteLLMResp.Choices) == 0 {
		return CompletionResponse{}, &ProviderError{Provider: c.providerName, Err: fmt.Errorf("empty choices in response")}
	}

	return CompletionResponse{
		Content:      liteLLMResp.Choices[0].Message.Content,
		Model:        liteLLMResp.Model,
		InputTokens:  liteLLMResp.Usage.PromptTokens,
		OutputTokens: liteLLMResp.Usage.CompletionTokens,
		ProviderName: c.providerName,
	}, nil
}

// buildRequest serialises a CompletionRequest into the LiteLLM JSON request body.
func (c *LiteLLMClient) buildRequest(req CompletionRequest) ([]byte, error) {
	msgs := make([]liteLLMMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = liteLLMMessage{Role: m.Role, Content: m.Content}
	}

	lr := liteLLMRequest{
		Model:    req.Model,
		Messages: msgs,
	}
	if req.MaxTokens > 0 {
		lr.MaxTokens = req.MaxTokens
	}
	if req.Temperature >= 0 {
		t := req.Temperature
		lr.Temperature = &t
	}

	return json.Marshal(lr)
}
