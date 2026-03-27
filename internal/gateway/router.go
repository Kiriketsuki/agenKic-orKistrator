package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

const (
	defaultJudgeModel = "claude-haiku-4-5-20251001"
)

var defaultClassificationPrompt = `You are a task complexity classifier. Classify the following task into exactly one of three tiers.

Respond with ONLY one word — no punctuation, no explanation:
- "cheap"    — simple lookups, formatting, summarizing short text, straightforward code fixes
- "mid"      — moderate analysis, code generation, multi-step reasoning
- "frontier" — complex architecture, novel problem-solving, long-form creative work

Task: %s`

// JudgeRouter classifies task complexity by consulting a cheap/fast judge model.
// It implements the Router interface.
type JudgeRouter struct {
	completer            Completer
	judgeModel           string
	defaultTier          ModelTier
	classificationPrompt string
}

// RouterOption configures a JudgeRouter.
type RouterOption func(*JudgeRouter)

// WithJudgeModel sets the model used for classification.
// Defaults to "claude-haiku-4-5-20251001".
func WithJudgeModel(model string) RouterOption {
	return func(r *JudgeRouter) { r.judgeModel = model }
}

// WithDefaultTier sets the fallback tier when classification fails.
// Defaults to TierMid.
func WithDefaultTier(tier ModelTier) RouterOption {
	return func(r *JudgeRouter) { r.defaultTier = tier }
}

// WithCompleter sets the Completer used for classification calls.
func WithCompleter(c Completer) RouterOption {
	return func(r *JudgeRouter) { r.completer = c }
}

// WithClassificationPrompt sets the prompt template used for classification.
// The template must contain a single %s verb for the task description.
func WithClassificationPrompt(prompt string) RouterOption {
	return func(r *JudgeRouter) { r.classificationPrompt = prompt }
}

// NewJudgeRouter returns a JudgeRouter configured with the given options.
func NewJudgeRouter(opts ...RouterOption) *JudgeRouter {
	r := &JudgeRouter{
		judgeModel:           defaultJudgeModel,
		defaultTier:          TierMid,
		classificationPrompt: defaultClassificationPrompt,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Classify implements Router. If task.OverrideTier is set, it is returned
// immediately. Otherwise the judge model is consulted to classify the task.
func (r *JudgeRouter) Classify(ctx context.Context, task TaskSpec) (RoutingDecision, error) {
	if task.OverrideTier != "" && task.OverrideTier.Valid() {
		slog.InfoContext(ctx, "gateway/router: override tier", "task_id", task.ID, "tier", task.OverrideTier)
		return RoutingDecision{
			Tier:        task.OverrideTier,
			Reason:      fmt.Sprintf("override: tier forced to %s", task.OverrideTier),
			RawResponse: "",
		}, nil
	} else if task.OverrideTier != "" {
		slog.WarnContext(ctx, "gateway/router: invalid override tier ignored", "task_id", task.ID, "tier", task.OverrideTier)
	}

	if r.completer == nil {
		slog.WarnContext(ctx, "gateway/router: no completer configured", "task_id", task.ID, "tier", r.defaultTier)
		return RoutingDecision{
			Tier:        r.defaultTier,
			Reason:      "no completer configured; using default tier",
			RawResponse: "",
		}, nil
	}

	if strings.Count(r.classificationPrompt, "%s") != 1 {
		slog.WarnContext(ctx, "gateway/router: classification prompt must contain exactly one %s verb", "task_id", task.ID, "tier", r.defaultTier)
		return RoutingDecision{
			Tier:        r.defaultTier,
			Reason:      "classification prompt must contain exactly one %s verb; using default tier",
			RawResponse: "",
		}, nil
	}

	prompt := fmt.Sprintf(r.classificationPrompt, task.Description)
	req := CompletionRequest{
		Model: r.judgeModel,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   10,
		Temperature: 0,
	}

	resp, err := r.completer.Complete(ctx, req)
	if err != nil {
		slog.WarnContext(ctx, "gateway/router: judge call failed", "task_id", task.ID, "tier", r.defaultTier, "model", r.judgeModel, "error", err)
		reason := fmt.Sprintf("judge call failed (%v); falling back to default tier %s", err, r.defaultTier)
		return RoutingDecision{
			Tier:        r.defaultTier,
			Reason:      reason,
			RawResponse: "",
		}, nil
	}

	tier := parseTier(resp.Content)
	if !tier.Valid() {
		slog.WarnContext(ctx, "gateway/router: unrecognised judge response", "task_id", task.ID, "tier", r.defaultTier, "raw_response", resp.Content)
		reason := fmt.Sprintf("judge returned unrecognised response %q; falling back to default tier %s", resp.Content, r.defaultTier)
		return RoutingDecision{
			Tier:        r.defaultTier,
			Reason:      reason,
			RawResponse: resp.Content,
		}, nil
	}

	slog.InfoContext(ctx, "gateway/router: classified", "task_id", task.ID, "tier", tier, "model", resp.Model)
	return RoutingDecision{
		Tier:        tier,
		Model:       resp.Model,
		Reason:      fmt.Sprintf("judge classified as %s", tier),
		RawResponse: resp.Content,
	}, nil
}

// Compile-time assertion that JudgeRouter implements Router.
var _ Router = (*JudgeRouter)(nil)

// parseTier converts a raw judge response string into a ModelTier.
// Returns an empty (invalid) ModelTier if the word is unrecognised.
func parseTier(raw string) ModelTier {
	word := strings.ToLower(strings.TrimSpace(raw))
	t := ModelTier(word)
	if t.Valid() {
		return t
	}
	return ModelTier("")
}
