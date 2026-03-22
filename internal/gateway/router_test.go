package gateway

import (
	"context"
	"errors"
	"testing"
)

// mockCompleter is a test double for the Completer interface.
type mockCompleter struct {
	response CompletionResponse
	err      error
	calls    int
}

func (m *mockCompleter) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	m.calls++
	return m.response, m.err
}

func (m *mockCompleter) Provider() string { return "mock" }

func TestJudgeRouter_Classify(t *testing.T) {
	tests := []struct {
		name          string
		task          TaskSpec
		completer     *mockCompleter
		wantTier      ModelTier
		wantCalls     int // expected calls to completer.Complete
		wantReasonSub string
	}{
		{
			name: "override tier skips classification",
			task: TaskSpec{
				ID:           "t1",
				Description:  "anything",
				OverrideTier: TierFrontier,
			},
			completer:     &mockCompleter{},
			wantTier:      TierFrontier,
			wantCalls:     0,
			wantReasonSub: "override",
		},
		{
			name: "judge returns cheap",
			task: TaskSpec{ID: "t2", Description: "format a JSON blob"},
			completer: &mockCompleter{
				response: CompletionResponse{Content: "cheap", Model: defaultJudgeModel},
			},
			wantTier:      TierCheap,
			wantCalls:     1,
			wantReasonSub: "classified as cheap",
		},
		{
			name: "judge returns frontier",
			task: TaskSpec{ID: "t3", Description: "design a distributed system"},
			completer: &mockCompleter{
				response: CompletionResponse{Content: "frontier", Model: defaultJudgeModel},
			},
			wantTier:      TierFrontier,
			wantCalls:     1,
			wantReasonSub: "classified as frontier",
		},
		{
			name: "judge returns garbage falls back to default",
			task: TaskSpec{ID: "t4", Description: "do something"},
			completer: &mockCompleter{
				response: CompletionResponse{Content: "bananas", Model: defaultJudgeModel},
			},
			wantTier:      TierMid, // defaultTier
			wantCalls:     1,
			wantReasonSub: "falling back to default tier",
		},
		{
			name: "completer error falls back to default",
			task: TaskSpec{ID: "t5", Description: "do something"},
			completer: &mockCompleter{
				err: errors.New("network timeout"),
			},
			wantTier:      TierMid,
			wantCalls:     1,
			wantReasonSub: "falling back to default tier",
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

			if tc.wantReasonSub != "" {
				if !containsSubstring(decision.Reason, tc.wantReasonSub) {
					t.Errorf("reason = %q, want it to contain %q", decision.Reason, tc.wantReasonSub)
				}
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
}

func containsSubstring(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && searchSubstring(s, sub))
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
