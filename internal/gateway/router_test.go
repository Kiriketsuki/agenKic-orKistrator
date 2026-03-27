package gateway

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// mockCompleter is a test double for the Completer interface.
type mockCompleter struct {
	response    CompletionResponse
	err         error
	calls       int
	lastRequest CompletionRequest
}

func (m *mockCompleter) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	m.calls++
	m.lastRequest = req
	return m.response, m.err
}

func (m *mockCompleter) Provider() string { return "mock" }

func TestJudgeRouter_Classify(t *testing.T) {
	tests := []struct {
		name            string
		task            TaskSpec
		completer       *mockCompleter
		wantTier        ModelTier
		wantCalls       int // expected calls to completer.Complete
		wantReasonSubs  []string
		wantRawResponse string
	}{
		{
			name: "override tier skips classification",
			task: TaskSpec{
				ID:           "t1",
				Description:  "anything",
				OverrideTier: TierFrontier,
			},
			completer:       &mockCompleter{},
			wantTier:        TierFrontier,
			wantCalls:       0,
			wantReasonSubs:  []string{"override"},
			wantRawResponse: "",
		},
		{
			name: "judge returns cheap",
			task: TaskSpec{ID: "t2", Description: "format a JSON blob"},
			completer: &mockCompleter{
				response: CompletionResponse{Content: "cheap", Model: defaultJudgeModel},
			},
			wantTier:        TierCheap,
			wantCalls:       1,
			wantReasonSubs:  []string{"classified as cheap"},
			wantRawResponse: "cheap",
		},
		{
			name: "judge returns mid",
			task: TaskSpec{ID: "t-mid", Description: "generate a moderately complex report"},
			completer: &mockCompleter{
				response: CompletionResponse{Content: "mid", Model: defaultJudgeModel},
			},
			wantTier:        TierMid,
			wantCalls:       1,
			wantReasonSubs:  []string{"classified as mid"},
			wantRawResponse: "mid",
		},
		{
			name: "judge returns frontier",
			task: TaskSpec{ID: "t3", Description: "design a distributed system"},
			completer: &mockCompleter{
				response: CompletionResponse{Content: "frontier", Model: defaultJudgeModel},
			},
			wantTier:        TierFrontier,
			wantCalls:       1,
			wantReasonSubs:  []string{"classified as frontier"},
			wantRawResponse: "frontier",
		},
		{
			name: "judge returns uppercase CHEAP through full classify path",
			task: TaskSpec{ID: "t-upper", Description: "format a CSV"},
			completer: &mockCompleter{
				response: CompletionResponse{Content: "CHEAP", Model: defaultJudgeModel},
			},
			wantTier:        TierCheap,
			wantCalls:       1,
			wantReasonSubs:  []string{"classified as cheap"},
			wantRawResponse: "CHEAP",
		},
		{
			name: "judge returns garbage falls back to default",
			task: TaskSpec{ID: "t4", Description: "do something"},
			completer: &mockCompleter{
				response: CompletionResponse{Content: "bananas", Model: defaultJudgeModel},
			},
			wantTier:        TierMid, // defaultTier
			wantCalls:       1,
			wantReasonSubs:  []string{"falling back to default tier", "unrecognised response", "bananas"},
			wantRawResponse: "bananas",
		},
		{
			name: "completer error falls back to default",
			task: TaskSpec{ID: "t5", Description: "do something"},
			completer: &mockCompleter{
				err: errors.New("network timeout"),
			},
			wantTier:        TierMid,
			wantCalls:       1,
			wantReasonSubs:  []string{"falling back to default tier", "network timeout"},
			wantRawResponse: "",
		},
		{
			name: "completer HTTP 500 error falls back to default",
			task: TaskSpec{ID: "t-http500", Description: "do something"},
			completer: &mockCompleter{
				err: errors.New("HTTP 500 Internal Server Error"),
			},
			wantTier:        TierMid,
			wantCalls:       1,
			wantReasonSubs:  []string{"falling back to default tier", "HTTP 500 Internal Server Error"},
			wantRawResponse: "",
		},
		{
			name: "empty string response falls back to default",
			task: TaskSpec{ID: "t6", Description: "do something"},
			completer: &mockCompleter{
				response: CompletionResponse{Content: "", Model: defaultJudgeModel},
			},
			wantTier:        TierMid,
			wantCalls:       1,
			wantReasonSubs:  []string{"falling back to default tier"},
			wantRawResponse: "",
		},
		{
			name: "whitespace-padded frontier is parsed correctly",
			task: TaskSpec{ID: "t7", Description: "build a spaceship"},
			completer: &mockCompleter{
				response: CompletionResponse{Content: "  frontier  ", Model: defaultJudgeModel},
			},
			wantTier:        TierFrontier,
			wantCalls:       1,
			wantReasonSubs:  []string{"classified as frontier"},
			wantRawResponse: "  frontier  ",
		},
		{
			name: "override cheap on complex task returns cheap",
			task: TaskSpec{
				ID:           "t8",
				Description:  "build a distributed system from scratch",
				OverrideTier: TierCheap,
			},
			completer:       &mockCompleter{},
			wantTier:        TierCheap,
			wantCalls:       0,
			wantReasonSubs:  []string{"override"},
			wantRawResponse: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := NewJudgeRouter(
				WithCompleter(tc.completer),
				WithDefaultTier(TierMid),
			)

			decision, err := router.Classify(context.Background(), tc.task)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if decision.Tier != tc.wantTier {
				t.Errorf("tier = %q, want %q", decision.Tier, tc.wantTier)
			}

			if tc.completer.calls != tc.wantCalls {
				t.Errorf("completer called %d times, want %d", tc.completer.calls, tc.wantCalls)
			}

			for _, sub := range tc.wantReasonSubs {
				if !strings.Contains(decision.Reason, sub) {
					t.Errorf("reason = %q, want it to contain %q", decision.Reason, sub)
				}
			}

			if decision.RawResponse != tc.wantRawResponse {
				t.Errorf("RawResponse = %q, want %q", decision.RawResponse, tc.wantRawResponse)
			}
		})
	}
}

func TestJudgeRouter_ParseTierCaseInsensitive(t *testing.T) {
	cases := []struct {
		input string
		want  ModelTier
	}{
		{"cheap", TierCheap},
		{"CHEAP", TierCheap},
		{"  mid  ", TierMid},
		{"Frontier", TierFrontier},
		{"FRONTIER", TierFrontier},
		{"unknown", ModelTier("")},
		{"", ModelTier("")},
	}
	for _, c := range cases {
		got := parseTier(c.input)
		if got != c.want {
			t.Errorf("parseTier(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestJudgeRouter_NoCompleterUsesDefault(t *testing.T) {
	router := NewJudgeRouter(WithDefaultTier(TierCheap))
	decision, err := router.Classify(context.Background(), TaskSpec{ID: "x", Description: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Tier != TierCheap {
		t.Errorf("tier = %q, want %q", decision.Tier, TierCheap)
	}
	if decision.RawResponse != "" {
		t.Errorf("RawResponse = %q, want empty on no-completer path", decision.RawResponse)
	}
	if !strings.Contains(decision.Reason, "no completer configured") {
		t.Errorf("reason = %q, want it to contain %q", decision.Reason, "no completer configured")
	}
}

func TestJudgeRouter_MissingFormatVerb(t *testing.T) {
	mc := &mockCompleter{
		response: CompletionResponse{Content: "cheap", Model: defaultJudgeModel},
	}
	router := NewJudgeRouter(
		WithCompleter(mc),
		WithClassificationPrompt("Classify as cheap, mid, or frontier."),
	)

	decision, err := router.Classify(context.Background(), TaskSpec{ID: "mv1", Description: "format a CSV"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Tier != TierMid {
		t.Errorf("tier = %q, want %q (default tier)", decision.Tier, TierMid)
	}

	if mc.calls != 0 {
		t.Errorf("completer called %d times, want 0 (should not reach completer)", mc.calls)
	}

	if !strings.Contains(decision.Reason, "exactly one %s verb and no other format verbs") {
		t.Errorf("reason = %q, want it to contain %q", decision.Reason, "exactly one %s verb and no other format verbs")
	}

	if decision.RawResponse != "" {
		t.Errorf("RawResponse = %q, want empty on missing-verb fallback", decision.RawResponse)
	}
}

func TestJudgeRouter_ExcessFormatVerbs(t *testing.T) {
	mc := &mockCompleter{
		response: CompletionResponse{Content: "cheap", Model: defaultJudgeModel},
	}
	router := NewJudgeRouter(
		WithCompleter(mc),
		WithClassificationPrompt("Rate task %s on scale %s"),
	)

	decision, err := router.Classify(context.Background(), TaskSpec{ID: "ev1", Description: "format a CSV"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Tier != TierMid {
		t.Errorf("tier = %q, want %q (default tier)", decision.Tier, TierMid)
	}

	if mc.calls != 0 {
		t.Errorf("completer called %d times, want 0 (should not reach completer)", mc.calls)
	}

	if !strings.Contains(decision.Reason, "exactly one %s verb and no other format verbs") {
		t.Errorf("reason = %q, want it to contain %q", decision.Reason, "exactly one %s verb and no other format verbs")
	}

	if decision.RawResponse != "" {
		t.Errorf("RawResponse = %q, want empty on excess-verb fallback", decision.RawResponse)
	}
}

func TestJudgeRouter_NonSFormatVerb(t *testing.T) {
	mc := &mockCompleter{
		response: CompletionResponse{Content: "cheap", Model: defaultJudgeModel},
	}
	router := NewJudgeRouter(
		WithCompleter(mc),
		WithClassificationPrompt("Classify task %s. Priority: %d"),
	)

	decision, err := router.Classify(context.Background(), TaskSpec{ID: "fv1", Description: "format a CSV"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Tier != TierMid {
		t.Errorf("tier = %q, want %q (default tier)", decision.Tier, TierMid)
	}

	if mc.calls != 0 {
		t.Errorf("completer called %d times, want 0 (should not reach completer)", mc.calls)
	}

	if !strings.Contains(decision.Reason, "no other format verbs") {
		t.Errorf("reason = %q, want it to contain %q", decision.Reason, "no other format verbs")
	}

	if decision.RawResponse != "" {
		t.Errorf("RawResponse = %q, want empty on format-verb fallback", decision.RawResponse)
	}
}

func TestJudgeRouter_InvalidOverrideTier(t *testing.T) {
	mc := &mockCompleter{
		response: CompletionResponse{Content: "cheap", Model: defaultJudgeModel},
	}
	router := NewJudgeRouter(
		WithCompleter(mc),
		WithDefaultTier(TierMid),
	)

	// Capture slog output to verify warn-leg of warn-and-continue path.
	var logBuf strings.Builder
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	decision, err := router.Classify(context.Background(), TaskSpec{
		ID:           "iot1",
		Description:  "format a CSV",
		OverrideTier: ModelTier("bogus"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Tier != TierCheap {
		t.Errorf("tier = %q, want %q (from classification, not override)", decision.Tier, TierCheap)
	}

	if mc.calls != 1 {
		t.Errorf("completer called %d times, want 1 (should fall through to classification)", mc.calls)
	}

	if !strings.Contains(decision.Reason, "classified as cheap") {
		t.Errorf("reason = %q, want it to contain %q", decision.Reason, "classified as cheap")
	}

	if decision.RawResponse != "cheap" {
		t.Errorf("RawResponse = %q, want %q", decision.RawResponse, "cheap")
	}

	if !strings.Contains(logBuf.String(), "invalid override tier ignored") {
		t.Errorf("expected warn log containing %q, got: %q", "invalid override tier ignored", logBuf.String())
	}
}

func TestJudgeRouter_CustomClassificationPrompt(t *testing.T) {
	mc := &mockCompleter{
		response: CompletionResponse{Content: "frontier", Model: defaultJudgeModel},
	}
	router := NewJudgeRouter(
		WithCompleter(mc),
		WithClassificationPrompt("Rate this: %s\nAnswer: cheap/mid/frontier"),
	)

	_, err := router.Classify(context.Background(), TaskSpec{ID: "p1", Description: "build a spaceship"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mc.calls != 1 {
		t.Fatalf("expected 1 completer call, got %d", mc.calls)
	}

	if len(mc.lastRequest.Messages) == 0 {
		t.Fatal("no messages sent to completer")
	}

	prompt := mc.lastRequest.Messages[0].Content
	if !strings.Contains(prompt, "Rate this: build a spaceship") {
		t.Errorf("prompt = %q, want it to contain %q", prompt, "Rate this: build a spaceship")
	}
}
