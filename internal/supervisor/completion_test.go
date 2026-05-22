package supervisor

import (
	"context"
	"sync"
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

// fakeClock provides a controllable, mutex-protected clock for deterministic
// TTL tests (no sleeping).
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{t: start}
}

func (f *fakeClock) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

func TestCompletionRegistry_TombstoneReapedAfterTTL(t *testing.T) {
	clock := newFakeClock(time.Now())
	r := NewCompletionRegistry(WithCompletionClock(clock.now), WithTombstoneTTL(time.Minute))

	r.Cleanup("t")

	clock.advance(time.Minute + time.Nanosecond)
	r.SweepOnce()

	r.mu.Lock()
	n := len(r.cleaned)
	r.mu.Unlock()

	if n != 0 {
		t.Fatalf("expected tombstone reaped after TTL, got %d entries", n)
	}
}

func TestCompletionRegistry_TombstoneNotReapedBeforeTTL(t *testing.T) {
	clock := newFakeClock(time.Now())
	r := NewCompletionRegistry(WithCompletionClock(clock.now), WithTombstoneTTL(time.Minute))

	r.Cleanup("t")

	clock.advance(time.Minute - time.Nanosecond)
	r.SweepOnce()

	r.mu.Lock()
	_, exists := r.cleaned["t"]
	n := len(r.cleaned)
	r.mu.Unlock()

	if n != 1 || !exists {
		t.Fatalf("expected tombstone NOT reaped before TTL, got %d entries (exists=%v)", n, exists)
	}

	t.Run("exactly at TTL boundary reaps (>= semantics)", func(t *testing.T) {
		clock.advance(time.Nanosecond) // now exactly at TTL
		r.SweepOnce()

		r.mu.Lock()
		n := len(r.cleaned)
		r.mu.Unlock()

		if n != 0 {
			t.Fatalf("expected tombstone reaped exactly at TTL boundary, got %d entries", n)
		}
	})
}

func TestCompletionRegistry_LateCompleteAfterReap_Semantics(t *testing.T) {
	clock := newFakeClock(time.Now())
	r := NewCompletionRegistry(WithCompletionClock(clock.now), WithTombstoneTTL(time.Minute))

	ctx, cancel := context.WithCancel(context.Background())

	// Mirror the cancelled BlockingSubmitter path from
	// TestCompletionRegistry_CompleteAfterCleanup_NoLeak.
	done := make(chan error, 1)
	go func() {
		done <- r.Wait(ctx, "task-reap")
	}()
	time.Sleep(20 * time.Millisecond)

	cancel()
	if err := <-done; err == nil {
		t.Fatal("expected context error, got nil")
	}

	r.Cleanup("task-reap")

	clock.advance(time.Minute + time.Nanosecond)
	r.SweepOnce()

	r.mu.Lock()
	cleanedLen := len(r.cleaned)
	r.mu.Unlock()
	if cleanedLen != 0 {
		t.Fatalf("expected tombstone reaped, got %d entries", cleanedLen)
	}

	// ACCEPTED SEMANTICS: once the tombstone is reaped, Complete can no
	// longer distinguish this late-Complete from a legitimate
	// complete-before-wait call. It therefore takes the complete-before-wait
	// path and creates ONE benign pre-closed waiters entry. This is the
	// documented, bounded tradeoff (at most one benign entry per pathological
	// late Complete, only reachable >TTL after cancellation) that replaces
	// unbounded tombstone growth. It is NOT the leak this feature fixes —
	// the leak was unbounded tombstone accumulation, not this single
	// pre-closed channel.
	r.Complete("task-reap")

	r.mu.Lock()
	waiterLen := len(r.waiters)
	ch, exists := r.waiters["task-reap"]
	r.mu.Unlock()

	if waiterLen != 1 || !exists {
		t.Fatalf("expected exactly 1 benign pre-closed waiters entry, got %d (exists=%v)", waiterLen, exists)
	}

	// Verify the channel is pre-closed: a following Wait returns immediately.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	select {
	case <-ch:
		// closed, as expected
	default:
		t.Fatal("expected pre-closed channel")
	}
	if err := r.Wait(waitCtx, "task-reap"); err != nil {
		t.Fatalf("expected Wait to return immediately on pre-closed channel, got %v", err)
	}
}

func TestCompletionRegistry_ReaperStopsCleanly(t *testing.T) {
	r := NewCompletionRegistry(WithSweepInterval(5 * time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	r.StartReaper(ctx)
	cancel()

	select {
	case <-r.reaperDone:
		// stopped cleanly
	case <-time.After(time.Second):
		t.Fatal("reaper did not stop within timeout — possible goroutine leak")
	}

	// Idempotent: calling StartReaper again with an already-cancelled/new
	// context (and cancelling again) must not panic.
	ctx2, cancel2 := context.WithCancel(context.Background())
	r.StartReaper(ctx2)
	cancel2()
	cancel() // calling cancel again must not panic
}

func TestCompletionRegistry_ReaperSweepsOnTick(t *testing.T) {
	r := NewCompletionRegistry(WithTombstoneTTL(time.Millisecond), WithSweepInterval(5*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.StartReaper(ctx)

	r.Cleanup("t")

	deadline := time.After(time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		r.mu.Lock()
		n := len(r.cleaned)
		r.mu.Unlock()
		if n == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("reaper did not sweep tombstone within deadline")
		case <-ticker.C:
		}
	}
}

func TestCompletionRegistry_Options(t *testing.T) {
	clock := newFakeClock(time.Unix(0, 0))
	r := NewCompletionRegistry(
		WithCompletionClock(clock.now),
		WithTombstoneTTL(7*time.Second),
		WithSweepInterval(3*time.Second),
	)

	r.mu.Lock()
	ttl := r.tombstoneTTL
	interval := r.sweepInterval
	got := r.clock()
	r.mu.Unlock()

	if ttl != 7*time.Second {
		t.Fatalf("expected tombstoneTTL=7s, got %v", ttl)
	}
	if interval != 3*time.Second {
		t.Fatalf("expected sweepInterval=3s, got %v", interval)
	}
	if !got.Equal(clock.now()) {
		t.Fatalf("expected clock override to be wired, got %v want %v", got, clock.now())
	}
}
