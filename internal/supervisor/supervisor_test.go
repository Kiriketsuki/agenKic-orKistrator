//go:build testenv

package supervisor_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
)

func newTestSupervisor(t *testing.T) (*supervisor.Supervisor, *state.MockStore) {
	t.Helper()
	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := supervisor.NewRestartPolicy(
		supervisor.WithCrashThreshold(10),
		supervisor.WithCrashWindow(60*time.Second),
	)
	sv := supervisor.NewSupervisor(machine, store, policy)
	return sv, store
}

func TestSupervisor_RegisterAgentSetsIdleState(t *testing.T) {
	t.Parallel()

	sv, store := newTestSupervisor(t)
	ctx := context.Background()

	if err := sv.RegisterAgent(ctx, "agent-1"); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	stateStr, err := store.GetAgentState(ctx, "agent-1")
	if err != nil {
		t.Fatalf("GetAgentState failed: %v", err)
	}
	if stateStr != string(agent.StateIdle) {
		t.Errorf("expected state %q, got %q", agent.StateIdle, stateStr)
	}
}

func TestSupervisor_RegisterAgentSetsFields(t *testing.T) {
	t.Parallel()

	sv, store := newTestSupervisor(t)
	ctx := context.Background()

	before := time.Now().UnixMilli()
	if err := sv.RegisterAgent(ctx, "agent-2"); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}
	after := time.Now().UnixMilli()

	fields, err := store.GetAgentFields(ctx, "agent-2")
	if err != nil {
		t.Fatalf("GetAgentFields failed: %v", err)
	}
	if fields.RegisteredAt < before || fields.RegisteredAt > after {
		t.Errorf("RegisteredAt %d not in range [%d, %d]", fields.RegisteredAt, before, after)
	}
}

func TestSupervisor_RegisterAgentAlreadyStopped(t *testing.T) {
	t.Parallel()

	sv, _ := newTestSupervisor(t)
	ctx := context.Background()

	sv.Stop()

	err := sv.RegisterAgent(ctx, "agent-x")
	if err == nil {
		t.Error("expected error registering on stopped supervisor, got nil")
	}
}

func TestSupervisor_ApplyEventSerializes(t *testing.T) {
	t.Parallel()

	sv, store := newTestSupervisor(t)
	ctx := context.Background()

	const agentID = "agent-concurrent"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	// Apply EventTaskAssigned from multiple goroutines.
	// Only one should succeed; others should get an InvalidTransitionError.
	const workers = 5
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Exactly one transition from idle -> assigned should succeed.
	if successCount != 1 {
		t.Errorf("expected exactly 1 successful transition, got %d", successCount)
	}

	// Verify agent is in assigned state.
	stateStr, err := store.GetAgentState(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateStr != string(agent.StateAssigned) {
		t.Errorf("expected assigned state, got %q", stateStr)
	}
}

// casConflictStore wraps a StateStore and makes CompareAndSetAgentState
// always return *StateConflictError, simulating a concurrent writer that
// changes agent state between read and CAS.
type casConflictStore struct {
	state.StateStore
}

func (s *casConflictStore) CompareAndSetAgentState(_ context.Context, _ string, expected, _ string) error {
	return &state.StateConflictError{Expected: expected, Actual: "assigned-by-other"}
}

func TestSupervisor_TryAssignTask_CASConflict_ReenqueuesTask(t *testing.T) {
	t.Parallel()

	base := state.NewMockStore()
	wrapper := &casConflictStore{StateStore: base}
	machine := agent.NewMachine(wrapper)
	policy := supervisor.NewRestartPolicy(
		supervisor.WithCrashThreshold(10),
		supervisor.WithCrashWindow(60*time.Second),
	)
	sv := supervisor.NewSupervisor(machine, wrapper, policy,
		supervisor.WithTaskPollInterval(10*time.Millisecond),
	)

	ctx := context.Background()

	// Register an idle agent (uses SetAgentFields, not CAS).
	if err := sv.RegisterAgent(ctx, "agent-cas-sv"); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	// Enqueue a task at a specific priority.
	if err := wrapper.EnqueueTask(ctx, "task-cas-sv", 42.0); err != nil {
		t.Fatalf("EnqueueTask: %v", err)
	}

	// Run supervisor briefly — long enough for one task poll tick.
	runCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	_ = sv.Run(runCtx)

	// The task should be back in the queue (re-enqueued after CAS conflict).
	n, err := base.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if n < 1 {
		t.Fatal("expected task to be re-enqueued after CAS conflict, but queue is empty")
	}

	// Verify the task ID and priority are preserved.
	taskID, pri, err := base.DequeueTask(ctx)
	if err != nil {
		t.Fatalf("DequeueTask: %v", err)
	}
	if taskID != "task-cas-sv" {
		t.Fatalf("want taskID=task-cas-sv, got %q", taskID)
	}
	if pri != 42.0 {
		t.Fatalf("want priority=42.0, got %f", pri)
	}
}
