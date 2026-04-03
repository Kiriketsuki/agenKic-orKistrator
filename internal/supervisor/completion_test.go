package supervisor

import (
	"context"
	"testing"
	"time"
)

func TestCompletionRegistry_CompleteBeforeWait(t *testing.T) {
	r := NewCompletionRegistry()
	r.Complete("task-1")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := r.Wait(ctx, "task-1"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCompletionRegistry_WaitThenComplete(t *testing.T) {
	r := NewCompletionRegistry()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- r.Wait(ctx, "task-2")
	}()

	time.Sleep(20 * time.Millisecond)
	r.Complete("task-2")

	if err := <-done; err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCompletionRegistry_ContextCancelled(t *testing.T) {
	r := NewCompletionRegistry()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := r.Wait(ctx, "task-3")
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestCompletionRegistry_DoubleComplete(t *testing.T) {
	r := NewCompletionRegistry()

	// Should not panic on double Complete.
	r.Complete("task-4")
	r.Complete("task-4")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := r.Wait(ctx, "task-4"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCompletionRegistry_Cleanup(t *testing.T) {
	r := NewCompletionRegistry()

	// Complete and wait, then cleanup.
	r.Complete("task-5")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Wait(ctx, "task-5"); err != nil {
		t.Fatalf("Wait after Complete: %v", err)
	}
	r.Cleanup("task-5")

	// After Cleanup, a new Wait should create a fresh channel and block until Complete.
	done := make(chan error, 1)
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	go func() {
		done <- r.Wait(waitCtx, "task-5")
	}()

	time.Sleep(20 * time.Millisecond)
	r.Complete("task-5")

	if err := <-done; err != nil {
		t.Fatalf("Wait after Cleanup+Complete: %v", err)
	}
}

func TestCompletionRegistry_CompleteAfterCleanup_NoLeak(t *testing.T) {
	r := NewCompletionRegistry()

	ctx, cancel := context.WithCancel(context.Background())

	// Simulate the cancelled BlockingSubmitter path:
	// 1. Wait is called (creates channel)
	done := make(chan error, 1)
	go func() {
		done <- r.Wait(ctx, "task-leak")
	}()

	time.Sleep(20 * time.Millisecond)

	// 2. Context cancelled — Wait returns error
	cancel()
	if err := <-done; err == nil {
		t.Fatal("expected context error, got nil")
	}

	// 3. Cleanup removes the waiter (leaves tombstone)
	r.Cleanup("task-leak")

	// 4. Late Complete fires — should consume tombstone, not create orphaned entry
	r.Complete("task-leak")

	// 5. Verify both maps are empty (no leak)
	r.mu.Lock()
	waiterLen := len(r.waiters)
	cleanedLen := len(r.cleaned)
	r.mu.Unlock()

	if waiterLen != 0 {
		t.Fatalf("expected 0 waiters, got %d — orphaned entry leak", waiterLen)
	}
	if cleanedLen != 0 {
		t.Fatalf("expected 0 cleaned entries, got %d — tombstone not consumed", cleanedLen)
	}
}
