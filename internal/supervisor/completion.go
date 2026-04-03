package supervisor

import (
	"context"
	"sync"
)

// CompletionRegistry allows callers to block until a task is completed.
// The supervisor signals completion; the DAG executor's BlockingSubmitter waits.
type CompletionRegistry struct {
	mu      sync.Mutex
	waiters map[string]chan struct{}
	cleaned map[string]struct{} // tombstones for cancelled tasks; prevents orphaned entries
}

// NewCompletionRegistry creates a new CompletionRegistry.
func NewCompletionRegistry() *CompletionRegistry {
	return &CompletionRegistry{
		waiters: make(map[string]chan struct{}),
		cleaned: make(map[string]struct{}),
	}
}

// Wait blocks until Complete(taskID) is called or ctx is cancelled.
// If the task was already completed before Wait is called, returns immediately.
func (r *CompletionRegistry) Wait(ctx context.Context, taskID string) error {
	r.mu.Lock()
	// A new Wait clears any stale tombstone from a prior cancelled lifecycle.
	delete(r.cleaned, taskID)
	ch, exists := r.waiters[taskID]
	if !exists {
		ch = make(chan struct{})
		r.waiters[taskID] = ch
	}
	r.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Complete signals that the task is done. All waiters are unblocked.
// Safe to call multiple times (idempotent after first call).
func (r *CompletionRegistry) Complete(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// If the task was already cleaned up (cancelled context path), consume
	// the tombstone and return — prevents orphaned channel entries.
	if _, ok := r.cleaned[taskID]; ok {
		delete(r.cleaned, taskID)
		return
	}
	ch, exists := r.waiters[taskID]
	if !exists {
		// No waiter yet — create a pre-closed channel so future Wait returns immediately.
		ch = make(chan struct{})
		r.waiters[taskID] = ch
	}
	select {
	case <-ch:
		// Already closed.
	default:
		close(ch)
	}
}

// Cleanup removes the entry for a task. Call after Wait returns to prevent memory leaks.
func (r *CompletionRegistry) Cleanup(taskID string) {
	r.mu.Lock()
	delete(r.waiters, taskID)
	r.cleaned[taskID] = struct{}{} // tombstone for late Complete
	r.mu.Unlock()
}
