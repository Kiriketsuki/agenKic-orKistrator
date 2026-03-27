package gateway

import (
	"context"
	"fmt"
	"time"
)

// ModelTier classifies the cost-performance level of a model.
type ModelTier string

const (
	TierCheap    ModelTier = "cheap"
	TierMid      ModelTier = "mid"
	TierFrontier ModelTier = "frontier"
)

// String implements fmt.Stringer.
func (t ModelTier) String() string { return string(t) }

// Valid returns true if t is a recognised tier.
func (t ModelTier) Valid() bool {
	switch t {
	case TierCheap, TierMid, TierFrontier:
		return true
	default:
		return false
	}
}

// MarshalText implements encoding.TextMarshaler for YAML/JSON serde.
func (t ModelTier) MarshalText() ([]byte, error) {
	if !t.Valid() {
		return nil, ErrInvalidTier
	}
	return []byte(t), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for YAML/JSON serde.
func (t *ModelTier) UnmarshalText(b []byte) error {
	v := ModelTier(b)
	if !v.Valid() {
		return ErrInvalidTier
	}
	*t = v
	return nil
}

// TaskSpec describes a task submitted for routing classification.
type TaskSpec struct {
	ID          string
	Description string
	// Payload carries the actual task content for classification.
	// The judge-router uses this (not just Description) to assess complexity.
	Payload string
	// Metadata is arbitrary key-value data passed through to cost tracking.
	Metadata map[string]string
	// OverrideTier, if set, bypasses the judge-router and forces this tier.
	OverrideTier ModelTier
}

// RoutingDecision captures the judge-router's classification output.
type RoutingDecision struct {
	Tier        ModelTier
	Model       string
	Reason      string
	RawResponse string // Raw judge model output; empty on override/error paths.
}

// CompletionRequest is the provider-agnostic input to a model completion call.
type CompletionRequest struct {
	Model    string
	Messages []Message
	// SystemPrompt is handled separately from Messages for providers (e.g.
	// Anthropic) that require system prompts outside the message array.
	SystemPrompt string
	// MaxTokens caps the response length. Zero means provider default.
	MaxTokens int
	// Temperature controls randomness. Negative means provider default.
	Temperature float64
	// Stream enables streaming response mode via the provider.
	Stream bool
	// Tier carries the routing decision so cost tracking receives tier context.
	Tier ModelTier
	// Metadata is passed through to cost tracking; not sent to the provider.
	Metadata map[string]string
}

// Message is a single turn in a conversation.
type Message struct {
	Role    string
	Content string
}

// CompletionResponse is the provider-agnostic output from a completion call.
type CompletionResponse struct {
	Content      string
	Model        string
	InputTokens  int
	OutputTokens int
	// FallbackUsed is true when the primary provider failed and a fallback
	// provider served the request.
	FallbackUsed bool
	// ProviderName identifies which provider actually served the request.
	ProviderName string
}

// CostRecord is a single billing entry written by the cost tracker.
type CostRecord struct {
	RequestID     string
	Timestamp     time.Time
	Model         string
	Tier          ModelTier
	Provider      string
	InputTokens   int
	OutputTokens  int
	EstimatedCost float64 // USD
	// CacheHit indicates whether the request used cached input tokens,
	// which affects pricing (e.g. DeepSeek V3: 10x cheaper for cached).
	CacheHit bool
	Metadata map[string]string
}

// CostReport aggregates spend over a time period.
type CostReport struct {
	Period    TimePeriod
	TierCosts map[ModelTier]TierCostSummary
	Total     CostSummary
}

// TierCostSummary holds per-tier aggregated metrics.
type TierCostSummary struct {
	Tier          ModelTier
	RequestCount  int
	InputTokens   int
	OutputTokens  int
	EstimatedCost float64
}

// CostSummary holds aggregated metrics across all tiers.
type CostSummary struct {
	RequestCount  int
	InputTokens   int
	OutputTokens  int
	EstimatedCost float64
}

// TimePeriod defines the time range for a cost report as a half-open
// interval [Start, End).
type TimePeriod struct {
	Start time.Time
	End   time.Time
}

// Today returns a TimePeriod covering the current calendar day in UTC.
func Today() TimePeriod {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return TimePeriod{Start: start, End: start.AddDate(0, 0, 1)}
}

// LastNDays returns a TimePeriod covering the last n days up to now (UTC).
// If n is negative, it is clamped to 0 (today only).
// Note: LastNDays(0) returns [midnight, now), not [midnight, midnight+24h).
// Use Today() for a fixed calendar-day window.
func LastNDays(n int) TimePeriod {
	if n < 0 {
		n = 0
	}
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -n)
	return TimePeriod{Start: start, End: now}
}

// Since returns a TimePeriod from t up to now (UTC).
func Since(t time.Time) TimePeriod {
	return TimePeriod{Start: t, End: time.Now().UTC()}
}

// ProviderConfig holds the runtime configuration for a single provider.
type ProviderConfig struct {
	Name string
	// BaseURL validation: LiteLLMClient.WithBaseURL rejects non-http/https schemes.
	// Full host allowlist and private CIDR blocking deferred to config loading (T5).
	BaseURL string
	APIKey  string `json:"-" yaml:"-"`
	Models  []string
}

// String returns a human-readable representation with the API key redacted.
func (p ProviderConfig) String() string {
	return "ProviderConfig{Name: " + p.Name + ", BaseURL: " + p.BaseURL + ", APIKey: [REDACTED]}"
}

// Format implements fmt.Formatter to ensure APIKey is never leaked via any
// format verb, including %+v which bypasses fmt.Stringer and uses reflection.
func (p ProviderConfig) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		if f.Flag('+') {
			fmt.Fprintf(f, "gateway.ProviderConfig{Name:%s, BaseURL:%s, APIKey:[REDACTED], Models:%v}", p.Name, p.BaseURL, p.Models)
			return
		}
		fmt.Fprint(f, p.String())
	case 's':
		fmt.Fprint(f, p.String())
	default:
		fmt.Fprint(f, p.String())
	}
}

// TierConfig maps a tier to its primary model and fallback chain.
type TierConfig struct {
	Tier          ModelTier
	PrimaryModel  string
	FallbackChain []string
}

// Gateway is the top-level interface for model routing and completion.
type Gateway interface {
	// Route classifies a task and returns the routing decision (tier + model).
	Route(ctx context.Context, task TaskSpec) (RoutingDecision, error)

	// Complete sends a completion request through the gateway, applying
	// fallback logic if the primary provider fails.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// GetCostReport returns aggregated cost data for the given time period.
	GetCostReport(ctx context.Context, period TimePeriod) (CostReport, error)
}

// Router classifies task complexity and selects the appropriate model tier.
type Router interface {
	// Classify inspects a task and returns which tier should handle it.
	// Implementations that forward task.Payload to hosted language models
	// are responsible for input validation, sanitization, and length limits.
	Classify(ctx context.Context, task TaskSpec) (RoutingDecision, error)
}

// Completer executes a single completion request against a provider.
type Completer interface {
	// Complete sends a request to the underlying provider and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
	// Provider returns the provider name (e.g. "anthropic", "openai", "ollama").
	Provider() string
}

// AdapterResolver finds and applies the format adapter for a model name.
// Implementations should return ErrNoProvider if no adapter handles the model.
type AdapterResolver interface {
	Resolve(model string, req CompletionRequest) (CompletionRequest, error)
}

// CostTracker records and queries per-request cost data.
type CostTracker interface {
	// Record logs a single request's cost data.
	Record(ctx context.Context, record CostRecord) error
	// Report returns aggregated cost data for the given time period.
	Report(ctx context.Context, period TimePeriod) (CostReport, error)
}
