package gateway

import (
	"context"
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

// TaskSpec describes a task submitted for routing classification.
type TaskSpec struct {
	ID          string
	Description string
	// OverrideTier, if set, bypasses the judge-router and forces this tier.
	OverrideTier ModelTier
}

// RoutingDecision captures the judge-router's classification output.
type RoutingDecision struct {
	Tier   ModelTier
	Model  string
	Reason string
}

// CompletionRequest is the provider-agnostic input to a model completion call.
type CompletionRequest struct {
	Model    string
	Messages []Message
	// MaxTokens caps the response length. Zero means provider default.
	MaxTokens int
	// Temperature controls randomness. Negative means provider default.
	Temperature float64
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

// TokenUsage summarises token consumption for a single request.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

// CostRecord is a single billing entry written by the cost tracker.
type CostRecord struct {
	Timestamp     time.Time
	Model         string
	Tier          ModelTier
	Provider      string
	InputTokens   int
	OutputTokens  int
	EstimatedCost float64 // USD
	Metadata      map[string]string
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

// TimePeriod defines the time range for a cost report.
type TimePeriod struct {
	Start time.Time
	End   time.Time
}

// ProviderConfig holds the runtime configuration for a single provider.
type ProviderConfig struct {
	Name    string
	BaseURL string
	APIKey  string
	Models  []string
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
	Classify(ctx context.Context, task TaskSpec) (RoutingDecision, error)
}

// Completer executes a single completion request against a provider.
type Completer interface {
	// Complete sends a request to the underlying provider and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
	// Provider returns the provider name (e.g. "anthropic", "openai", "ollama").
	Provider() string
}

// CostTracker records and queries per-request cost data.
type CostTracker interface {
	// Record logs a single request's cost data.
	Record(ctx context.Context, record CostRecord) error
	// Report returns aggregated cost data for the given time period.
	Report(ctx context.Context, period TimePeriod) (CostReport, error)
}
