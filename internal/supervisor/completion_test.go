package supervisor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// newCompletionTestSupervisor builds a Supervisor wired to a MockStore and a
// real CompletionRegistry, matching the wiring used in production.
func newCompletionTestSupervisor(t *testing.T) (*Supervisor, *state.MockStore, *CompletionRegistry) {
	t.Helper()
	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := NewRestartPolicy(
		WithCrashThreshold(10),
		WithCrashWindow(60*time.Second),
	)
	registry := NewCompletionRegistry()
	sv := NewSupervisor(machine, store, policy, WithCompletionRegistry(registry))
	return sv, store, registry
}

// driveToReporting registers agentID, drives it IDLE→ASSIGNED→WORKING→REPORTING,
// and persists taskID as its CurrentTaskID.
func driveToReporting(t *testing.T, sv *Supervisor, store *state.MockStore, agentID, taskID string) {
	t.Helper()
	ctx := context.Background()

	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if _, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("ApplyEvent(task_assigned): %v", err)
	}
	fields, err := store.GetAgentFields(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentFields: %v", err)
	}
	fields.CurrentTaskID = taskID
	if err := store.SetAgentFields(ctx, agentID, fields); err != nil {
		t.Fatalf("SetAgentFields: %v", err)
	}
	if _, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("ApplyEvent(work_started): %v", err)
	}
	if _, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventOutputReady); err != nil {
		t.Fatalf("ApplyEvent(output_ready): %v", err)
	}
}

// TestCompleteAgent_TaskIDProvided_SignalsDespiteGetAgentFieldsFailure is the
// regression guard for issue #82: when the caller supplies a task_id,
// completeAgent must signal CompletionRegistry.Complete directly, without
// relying on GetAgentFields — so a store failure on that read cannot drop
// the completion signal.
func TestCompleteAgent_TaskIDProvided_SignalsDespiteGetAgentFieldsFailure(t *testing.T) {
	t.Parallel()

	sv, store, registry := newCompletionTestSupervisor(t)
	ctx := context.Background()
	const agentID = "agent-82-provided"
	const taskID = "task-x"

	driveToReporting(t, sv, store, agentID, taskID)

	// Inject a GetAgentFields failure — the fix must not depend on this read
	// when taskID is supplied.
	store.SetGetAgentFieldsError(errors.New("injected GetAgentFields failure"), agentID)

	waitCtx, waitCancel := context.WithTimeout(ctx, 2*time.Second)
	defer waitCancel()
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- registry.Wait(waitCtx, taskID)
	}()

	// Give the Wait goroutine a moment to register before completing.
	time.Sleep(20 * time.Millisecond)

	if err := sv.completeAgent(ctx, agentID, taskID); err != nil {
		t.Fatalf("completeAgent returned error: %v", err)
	}

	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("expected Wait to unblock with nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("completion signal was dropped — Wait did not unblock in time")
	}

	stateStr, err := store.GetAgentState(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateStr != string(agent.StateIdle) {
		t.Fatalf("expected agent IDLE, got %q", stateStr)
	}
}

// TestCompleteAgent_EmptyTaskID_FallsBackToGetAgentFields verifies the legacy
// path is preserved: when no task_id is supplied and the store is healthy,
// completeAgent still signals completion using CurrentTaskID from
// GetAgentFields.
func TestCompleteAgent_EmptyTaskID_FallsBackToGetAgentFields(t *testing.T) {
	t.Parallel()

	sv, store, registry := newCompletionTestSupervisor(t)
	ctx := context.Background()
	const agentID = "agent-82-fallback-healthy"
	const taskID = "task-y"

	driveToReporting(t, sv, store, agentID, taskID)

	waitCtx, waitCancel := context.WithTimeout(ctx, 2*time.Second)
	defer waitCancel()
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- registry.Wait(waitCtx, taskID)
	}()

	time.Sleep(20 * time.Millisecond)

	if err := sv.completeAgent(ctx, agentID, ""); err != nil {
		t.Fatalf("completeAgent returned error: %v", err)
	}

	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("expected Wait to unblock with nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected fallback completion signal via GetAgentFields, but Wait did not unblock")
	}
}

// TestCompleteAgent_EmptyTaskID_GetAgentFieldsFailure_DropsSignal documents
// the intentionally-retained legacy failure mode: when task_id is empty
// (old client) AND GetAgentFields fails, the completion signal is dropped.
// completeAgent still returns nil and the agent still reaches IDLE — only
// the completion signal is lost.
func TestCompleteAgent_EmptyTaskID_GetAgentFieldsFailure_DropsSignal(t *testing.T) {
	t.Parallel()

	sv, store, registry := newCompletionTestSupervisor(t)
	ctx := context.Background()
	const agentID = "agent-82-fallback-broken"
	const taskID = "task-z"

	driveToReporting(t, sv, store, agentID, taskID)
	store.SetGetAgentFieldsError(errors.New("injected GetAgentFields failure"), agentID)

	waitCtx, waitCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer waitCancel()
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- registry.Wait(waitCtx, taskID)
	}()

	time.Sleep(20 * time.Millisecond)

	if err := sv.completeAgent(ctx, agentID, ""); err != nil {
		t.Fatalf("completeAgent returned error: %v", err)
	}

	select {
	case err := <-waitDone:
		if err == nil {
			t.Fatal("expected Wait to time out (signal dropped), but it unblocked with nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("test itself timed out waiting for the Wait goroutine to return")
	}

	stateStr, err := store.GetAgentState(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateStr != string(agent.StateIdle) {
		t.Fatalf("expected agent IDLE despite dropped completion signal, got %q", stateStr)
	}
}

// TestCompleteAgent_MismatchedTaskID_UsesStoreCurrentTaskID is the
// regression guard for issue #82's follow-up findings: when the
// caller-supplied task_id does not match the agent's actual store-side
// CurrentTaskID (e.g. a stale/retried RPC, or a client-side bug), the
// store's authoritative value must win. The genuine current task must be
// signaled complete, and the mismatched (unowned) task_id must NOT be
// completed.
func TestCompleteAgent_MismatchedTaskID_UsesStoreCurrentTaskID(t *testing.T) {
	t.Parallel()

	sv, store, registry := newCompletionTestSupervisor(t)
	ctx := context.Background()
	const agentID = "agent-82-mismatch"
	const realTaskID = "task-real"
	const staleTaskID = "task-stale-other"

	driveToReporting(t, sv, store, agentID, realTaskID)

	realWaitCtx, realCancel := context.WithTimeout(ctx, 2*time.Second)
	defer realCancel()
	realDone := make(chan error, 1)
	go func() {
		realDone <- registry.Wait(realWaitCtx, realTaskID)
	}()

	staleWaitCtx, staleCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer staleCancel()
	staleDone := make(chan error, 1)
	go func() {
		staleDone <- registry.Wait(staleWaitCtx, staleTaskID)
	}()

	time.Sleep(20 * time.Millisecond)

	// Caller supplies a task_id that does not match the agent's real
	// CurrentTaskID in the store.
	if err := sv.completeAgent(ctx, agentID, staleTaskID); err != nil {
		t.Fatalf("completeAgent returned error: %v", err)
	}

	select {
	case err := <-realDone:
		if err != nil {
			t.Fatalf("expected the agent's real current task to be signaled complete, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("real current task's completion signal was dropped in favor of the mismatched caller-supplied taskID")
	}

	select {
	case err := <-staleDone:
		if err == nil {
			t.Fatal("expected the mismatched/stale taskID to NOT be completed, but Wait unblocked with nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("test itself timed out waiting for the stale Wait goroutine to return")
	}
}

// TestCompleteAgent_MatchingTaskID_SignalsWithoutStoreDependency verifies
// that a caller-supplied taskID which matches the store's CurrentTaskID is
// still honored (the common, conformant-client case).
func TestCompleteAgent_MatchingTaskID_SignalsWithoutStoreDependency(t *testing.T) {
	t.Parallel()

	sv, store, registry := newCompletionTestSupervisor(t)
	ctx := context.Background()
	const agentID = "agent-82-match"
	const taskID = "task-match"

	driveToReporting(t, sv, store, agentID, taskID)

	waitCtx, waitCancel := context.WithTimeout(ctx, 2*time.Second)
	defer waitCancel()
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- registry.Wait(waitCtx, taskID)
	}()

	time.Sleep(20 * time.Millisecond)

	if err := sv.completeAgent(ctx, agentID, taskID); err != nil {
		t.Fatalf("completeAgent returned error: %v", err)
	}

	select {
	case err := <-waitDone:
		if err != nil {
			t.Fatalf("expected Wait to unblock with nil, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("completion signal was dropped for a matching taskID")
	}
}

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
	// path and creates ONE pre-closed waiters entry, timestamped in
	// `completed` so the reaper can TTL-expire it too if nobody ever claims
	// it via Wait+Cleanup (see SweepOnce). This is the bounded tradeoff that
	// replaces unbounded tombstone growth: at most one entry, alive for at
	// most one more TTL window, per pathological late Complete.
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

// TestCompletionRegistry_OrphanedWaiterReapedAfterTTL is the regression test
// for issue #83's high-severity finding: a late Complete arriving after its
// tombstone has already been reaped must not permanently orphan an entry in
// `waiters`. It must instead be TTL-reaped, just like a `cleaned` tombstone,
// once nobody claims it via Wait+Cleanup.
func TestCompletionRegistry_OrphanedWaiterReapedAfterTTL(t *testing.T) {
	clock := newFakeClock(time.Now())
	r := NewCompletionRegistry(WithCompletionClock(clock.now), WithTombstoneTTL(time.Minute))

	ctx, cancel := context.WithCancel(context.Background())

	// 1. Task's context is cancelled; submitter's Wait returns ctx.Err().
	done := make(chan error, 1)
	go func() {
		done <- r.Wait(ctx, "task-orphan")
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("expected context error, got nil")
	}

	// 2. Submitter's Cleanup plants a tombstone.
	r.Cleanup("task-orphan")

	// 3. Reaper sweeps the tombstone once tombstoneTTL elapses.
	clock.advance(time.Minute + time.Nanosecond)
	r.SweepOnce()

	r.mu.Lock()
	cleanedLen := len(r.cleaned)
	r.mu.Unlock()
	if cleanedLen != 0 {
		t.Fatalf("expected tombstone reaped, got %d entries", cleanedLen)
	}

	// 4. The original, legitimately-slow Complete finally arrives. No
	// tombstone and no waiter exist, so it plants a pre-closed channel in
	// `waiters` (and a timestamp in `completed`) — nobody will ever call
	// Wait/Cleanup for this taskID again.
	r.Complete("task-orphan")

	r.mu.Lock()
	waiterLen := len(r.waiters)
	r.mu.Unlock()
	if waiterLen != 1 {
		t.Fatalf("expected 1 pending pre-closed waiters entry immediately after late Complete, got %d", waiterLen)
	}

	// 5. Advance past a second TTL window with no Wait/Cleanup ever coming.
	// The reaper must reclaim the orphaned waiters entry — this is the
	// fix for #83's "unbounded growth relocated to waiters" finding.
	clock.advance(time.Minute + time.Nanosecond)
	r.SweepOnce()

	r.mu.Lock()
	waiterLen = len(r.waiters)
	completedLen := len(r.completed)
	r.mu.Unlock()
	if waiterLen != 0 {
		t.Fatalf("expected orphaned waiters entry reaped after second TTL window, got %d — unbounded growth relocated to waiters map", waiterLen)
	}
	if completedLen != 0 {
		t.Fatalf("expected completed-timestamp entry reaped alongside waiters entry, got %d", completedLen)
	}
}

// TestCompletionRegistry_ClaimedCompleteBeforeWaitNotReaped verifies the
// TTL reaping added for orphaned waiters entries does not affect the normal
// complete-before-wait race: once Wait claims a pre-closed channel, the
// entry must survive past what would have been its TTL deadline until the
// caller's own Cleanup runs.
func TestCompletionRegistry_ClaimedCompleteBeforeWaitNotReaped(t *testing.T) {
	clock := newFakeClock(time.Now())
	r := NewCompletionRegistry(WithCompletionClock(clock.now), WithTombstoneTTL(time.Minute))

	// Complete arrives before Wait (normal race) — plants a pre-closed
	// channel + completed-timestamp.
	r.Complete("task-claimed")

	// Wait claims it well within the TTL window.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Wait(ctx, "task-claimed"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	r.mu.Lock()
	_, stillTracked := r.completed["task-claimed"]
	r.mu.Unlock()
	if stillTracked {
		t.Fatal("expected completed-timestamp cleared once Wait claims the entry")
	}

	// Advancing well past the TTL and sweeping must not remove the waiters
	// entry set up by Wait's own bookkeeping being followed by Cleanup, and
	// must not have reaped anything prematurely before Cleanup ran.
	clock.advance(2 * time.Minute)
	r.SweepOnce()

	r.mu.Lock()
	_, waiterExists := r.waiters["task-claimed"]
	r.mu.Unlock()
	if !waiterExists {
		t.Fatal("expected claimed waiters entry to survive TTL sweep (only unclaimed orphans are reaped)")
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

	// Idempotent: calling StartReaper again with a fresh context must be a
	// true no-op — no second goroutine spawned, no reassignment of
	// reaperDone. Capture the channel identity before the second call so a
	// regression (e.g. the sync.Once guard weakened to a resettable flag)
	// that reassigns r.reaperDone or spawns a second ticker goroutine is
	// actually caught instead of passing silently.
	doneBefore := r.reaperDone

	ctx2, cancel2 := context.WithCancel(context.Background())
	r.StartReaper(ctx2)
	cancel2()
	cancel() // calling cancel again must not panic

	if r.reaperDone != doneBefore {
		t.Fatal("StartReaper is not idempotent: reaperDone was reassigned by a second call")
	}

	// doneBefore must still be closed (it already fired above) — if a
	// second goroutine had been spawned against ctx2, this alone wouldn't
	// prove it, but a reassigned/re-closed channel would show up as a panic
	// ("close of closed channel") surfaced by that hypothetical second
	// goroutine, or as doneBefore != r.reaperDone above.
	select {
	case <-doneBefore:
	default:
		t.Fatal("expected reaperDone to remain closed after idempotent StartReaper call")
	}

	// Cancelling ctx2 must not have started a reaper against it: SweepOnce
	// via a hypothetical second goroutine would still be harmless here
	// since nothing was inserted, but the identity check above is the
	// actual regression guard for the sync.Once weakening scenario.
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
