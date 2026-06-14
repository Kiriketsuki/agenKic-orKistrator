package supervisor

import (
	"context"
	"sync"
	"time"
)

// defaultTombstoneTTL is how long a Cleanup tombstone is retained before the
// reaper sweeps it. Set to 2x defaultStaleThreshold (supervisor.go) so a
// tombstone comfortably outlives the window in which a legitimately-slow
// Complete could still arrive.
const defaultTombstoneTTL = 2 * defaultStaleThreshold

// defaultSweepInterval is how often the background reaper checks for expired
// tombstones. Worst case, a tombstone can live up to
// ~defaultTombstoneTTL+defaultSweepInterval before being reaped.
const defaultSweepInterval = defaultStaleThreshold

// CompletionRegistry allows callers to block until a task is completed.
// The supervisor signals completion; the DAG executor's BlockingSubmitter waits.
type CompletionRegistry struct {
	mu      sync.Mutex
	waiters map[string]chan struct{}
	cleaned map[string]time.Time // tombstones for cancelled tasks; prevents orphaned entries

	// completed tracks pre-closed channels planted by Complete's "no waiter
	// yet" branch (completion.go: Complete), timestamped so the reaper can
	// TTL-expire ones nobody ever claims via Wait+Cleanup. Without this, a
	// late Complete that arrives after its tombstone has already been reaped
	// (see Complete's doc comment) permanently orphans an entry in `waiters`
	// instead of `cleaned` — unbounded growth just relocated to a different
	// map. See #83.
	completed map[string]time.Time

	clock         func() time.Time
	tombstoneTTL  time.Duration
	sweepInterval time.Duration

	reaperStarted sync.Once
	reaperDone    chan struct{} // nil until StartReaper is called
}

// CompletionOption configures a CompletionRegistry at construction time.
type CompletionOption func(*CompletionRegistry)

// WithCompletionClock injects a custom clock function (used in tests for
// deterministic, sleep-free TTL logic). Named WithCompletionClock (rather
// than WithClock) to avoid colliding with the existing RestartPolicyOption
// WithClock in restart.go, which is a different function in this package.
func WithCompletionClock(fn func() time.Time) CompletionOption {
	return func(r *CompletionRegistry) {
		r.clock = fn
	}
}

// WithTombstoneTTL overrides the default tombstone TTL.
func WithTombstoneTTL(d time.Duration) CompletionOption {
	return func(r *CompletionRegistry) {
		r.tombstoneTTL = d
	}
}

// WithSweepInterval overrides the default reaper sweep interval.
func WithSweepInterval(d time.Duration) CompletionOption {
	return func(r *CompletionRegistry) {
		r.sweepInterval = d
	}
}

// NewCompletionRegistry creates a new CompletionRegistry. Variadic options
// keep this source-compatible with existing zero-arg call sites.
func NewCompletionRegistry(opts ...CompletionOption) *CompletionRegistry {
	r := &CompletionRegistry{
		waiters:       make(map[string]chan struct{}),
		cleaned:       make(map[string]time.Time),
		completed:     make(map[string]time.Time),
		clock:         time.Now,
		tombstoneTTL:  defaultTombstoneTTL,
		sweepInterval: defaultSweepInterval,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Wait blocks until Complete(taskID) is called or ctx is cancelled.
// If the task was already completed before Wait is called, returns immediately.
func (r *CompletionRegistry) Wait(ctx context.Context, taskID string) error {
	r.mu.Lock()
	// A new Wait clears any stale tombstone from a prior cancelled lifecycle.
	delete(r.cleaned, taskID)
	// This Wait claims any pre-closed channel Complete may have planted via
	// its "no waiter yet" branch — clear the completed-timestamp so the
	// reaper never mistakes a now-claimed entry for an orphan.
	delete(r.completed, taskID)
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
//
// Accepted semantics: if the task's tombstone was already reaped by the
// background TTL sweeper (see SweepOnce/StartReaper), Complete can no longer
// distinguish this late Complete from a legitimate complete-before-wait call
// — both arrive with no tombstone and no waiter. In that case Complete takes
// the complete-before-wait path below and creates one pre-closed waiters
// entry, timestamped in `completed`. That timestamp lets the reaper's
// SweepOnce TTL-expire the entry if nothing ever claims it via Wait+Cleanup
// (see SweepOnce), so a late Complete after reap does not permanently orphan
// an entry — it just gets a second, bounded TTL window before being reaped
// from `waiters` instead of `cleaned`.
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
		// No waiter yet — create a pre-closed channel so future Wait returns
		// immediately. Timestamp it so an unclaimed entry (nobody ever calls
		// Wait/Cleanup for this taskID again) is eventually TTL-reaped
		// instead of living forever.
		ch = make(chan struct{})
		r.waiters[taskID] = ch
		r.completed[taskID] = r.clock()
	}
	select {
	case <-ch:
		// Already closed.
	default:
		close(ch)
	}
}

// Cleanup removes the entry for a task. Call after Wait returns to prevent
// memory leaks. Plants a tombstone recording the current time so a
// subsequent late Complete can still be recognized and swallowed — until the
// tombstone is reaped by the TTL sweeper (see SweepOnce/StartReaper), after
// which the accepted-semantics tradeoff documented on Complete applies.
func (r *CompletionRegistry) Cleanup(taskID string) {
	r.mu.Lock()
	delete(r.waiters, taskID)
	delete(r.completed, taskID)
	r.cleaned[taskID] = r.clock() // tombstone for late Complete, timestamped for TTL reaping
	r.mu.Unlock()
}

// SweepOnce removes any tombstone older than tombstoneTTL, and reaps any
// pre-closed `waiters` entry planted by Complete's "no waiter yet" branch
// (see Complete) that nobody has claimed via Wait/Cleanup within
// tombstoneTTL. Without this second sweep, a late Complete arriving after
// its tombstone was already reaped permanently orphans an entry in
// `waiters` instead of `cleaned` — unbounded growth just relocated to a
// different map (#83). It is pure, lock-guarded, and deterministic given
// the registry's clock — tests drive TTL logic through this method with a
// fake clock instead of sleeping.
func (r *CompletionRegistry) SweepOnce() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clock()
	for taskID, insertedAt := range r.cleaned {
		if now.Sub(insertedAt) >= r.tombstoneTTL {
			delete(r.cleaned, taskID)
		}
	}
	for taskID, insertedAt := range r.completed {
		if now.Sub(insertedAt) >= r.tombstoneTTL {
			delete(r.completed, taskID)
			delete(r.waiters, taskID)
		}
	}
}

// StartReaper launches a background goroutine that calls SweepOnce on a
// ticker until ctx is done. It is idempotent (safe to call more than once;
// only the first call starts a goroutine) and never auto-started by
// NewCompletionRegistry, so callers that never invoke StartReaper (tests,
// BlockingSubmitter unit tests) spawn no goroutine.
func (r *CompletionRegistry) StartReaper(ctx context.Context) {
	r.reaperStarted.Do(func() {
		r.reaperDone = make(chan struct{})
		go func() {
			defer close(r.reaperDone)
			ticker := time.NewTicker(r.sweepInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					r.SweepOnce()
				}
			}
		}()
	})
}
