package supervisor

import (
	"context"
	"errors"
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
