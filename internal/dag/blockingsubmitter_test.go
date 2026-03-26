package dag

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
)

func TestBlockingSubmitter_BlocksUntilComplete(t *testing.T) {
	enqueuer := &mockEnqueuer{}
	registry := supervisor.NewCompletionRegistry()
	sub := NewBlockingSubmitter(enqueuer, registry)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- sub.SubmitTask(ctx, "task-1", "prompt", "standard", 1.0)
	}()

	time.Sleep(20 * time.Millisecond)
	registry.Complete("task-1")

	if err := <-done; err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if enqueuer.taskID != "task-1" {
		t.Fatalf("expected task-1 to be enqueued, got %q", enqueuer.taskID)
	}
}

func TestBlockingSubmitter_EnqueueError(t *testing.T) {
	enqueueErr := errors.New("store unavailable")
	enqueuer := &mockEnqueuer{err: enqueueErr}
	registry := supervisor.NewCompletionRegistry()
	sub := NewBlockingSubmitter(enqueuer, registry)

	ctx := context.Background()
	err := sub.SubmitTask(ctx, "task-2", "prompt", "standard", 1.0)
	if !errors.Is(err, enqueueErr) {
		t.Fatalf("expected enqueue error, got %v", err)
	}
}

func TestBlockingSubmitter_ContextCancelled(t *testing.T) {
	enqueuer := &mockEnqueuer{}
	registry := supervisor.NewCompletionRegistry()
	sub := NewBlockingSubmitter(enqueuer, registry)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- sub.SubmitTask(ctx, "task-3", "prompt", "standard", 1.0)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	err := <-done
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
