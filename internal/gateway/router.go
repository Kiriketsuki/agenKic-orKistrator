package gateway

import (
	"context"
	"fmt"
	"log"
	"strings"
)

const (
	defaultJudgeModel = "claude-haiku-4-5-20251001"

	classificationPrompt = `You are a task complexity classifier. Classify the following task into exactly one of three tiers.

Respond with ONLY one word — no punctuation, no explanation:
- "cheap"    — simple lookups, formatting, summarizing short text, straightforward code fixes
- "mid"      — moderate analysis, code generation, multi-step reasoning
- "frontier" — complex architecture, novel problem-solving, long-form creative work

Task: %s`
)

// JudgeRouter classifies task complexity by consulting a cheap/fast judge model.
// It implements the Router interface.
type JudgeRouter struct {
	completer   Completer
	judgeModel  string
	defaultTier ModelTier
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

// NewJudgeRouter returns a JudgeRouter configured with the given options.
func NewJudgeRouter(opts ...RouterOption) *JudgeRouter {
	r := &JudgeRouter{
		judgeModel:  defaultJudgeModel,
		defaultTier: TierMid,
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
		return RoutingDecision{
			Tier:   task.OverrideTier,
			Reason: fmt.Sprintf("override: tier forced to %s", task.OverrideTier),
		}, nil
	}

	if r.completer == nil {
		log.Printf("gateway/router: no completer configured, using default tier %s for task %s", r.defaultTier, task.ID)
		return RoutingDecision{
			Tier:   r.defaultTier,
			Reason: "no completer configured; using default tier",
		}, nil
	}

	prompt := fmt.Sprintf(classificationPrompt, task.Description)
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
		reason := fmt.Sprintf("judge call failed (%v); falling back to default tier %s", err, r.defaultTier)
		log.Printf("gateway/router: %s (task %s)", reason, task.ID)
		return RoutingDecision{
			Tier:   r.defaultTier,
			Reason: reason,
		}, nil
	}

	tier := parseTier(resp.Content)
	if !tier.Valid() {
		reason := fmt.Sprintf("judge returned unrecognised response %q; falling back to default tier %s", resp.Content, r.defaultTier)
		log.Printf("gateway/router: %s (task %s)", reason, task.ID)
		return RoutingDecision{
			Tier:   r.defaultTier,
			Reason: reason,
		}, nil
	}

	return RoutingDecision{
		Tier:   tier,
		Model:  resp.Model,
		Reason: fmt.Sprintf("judge classified as %s", tier),
	}, nil
}

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
