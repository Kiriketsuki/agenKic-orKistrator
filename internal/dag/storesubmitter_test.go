package dag

import (
	"context"
	"errors"
	"testing"
)

// mockEnqueuer is a minimal TaskEnqueuer for testing StoreSubmitter.
type mockEnqueuer struct {
	called   bool
	taskID   string
	priority float64
	err      error
}

func (m *mockEnqueuer) EnqueueTask(_ context.Context, taskID string, priority float64) error {
	m.called = true
	m.taskID = taskID
	m.priority = priority
	return m.err
}

func TestStoreSubmitter_Success(t *testing.T) {
	enq := &mockEnqueuer{}
	sub := NewStoreSubmitter(enq)

	err := sub.SubmitTask(context.Background(), "task-1", "do stuff", "sonnet", 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enq.called {
		t.Fatal("expected EnqueueTask to be called")
	}
	if enq.taskID != "task-1" {
		t.Fatalf("expected taskID=task-1, got %s", enq.taskID)
	}
	if enq.priority != 5.0 {
		t.Fatalf("expected priority=5.0, got %f", enq.priority)
	}
}

func TestStoreSubmitter_Error(t *testing.T) {
	enq := &mockEnqueuer{err: errors.New("queue full")}
	sub := NewStoreSubmitter(enq)

	err := sub.SubmitTask(context.Background(), "task-2", "prompt", "haiku", 1.0)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "queue full" {
		t.Fatalf("expected 'queue full', got %q", err.Error())
	}
}

func TestStoreSubmitter_IgnoresPromptAndModelTier(t *testing.T) {
	enq := &mockEnqueuer{}
	sub := NewStoreSubmitter(enq)

	// Call with non-empty prompt and modelTier — both should be dropped.
	_ = sub.SubmitTask(context.Background(), "task-3", "important prompt", "opus", 10.0)

	// The enqueuer only sees taskID and priority — no way to verify prompt/modelTier
	// were stored because they aren't. This test documents the MVP gap.
	if enq.taskID != "task-3" {
		t.Fatalf("expected taskID=task-3, got %s", enq.taskID)
	}
	if enq.priority != 10.0 {
		t.Fatalf("expected priority=10.0, got %f", enq.priority)
	}
}
