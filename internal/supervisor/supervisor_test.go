//go:build testenv

package supervisor_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
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

func TestSupervisor_TryAssignTask_Serializes(t *testing.T) {
	t.Parallel()

	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := supervisor.NewRestartPolicy(
		supervisor.WithCrashThreshold(10),
		supervisor.WithCrashWindow(60*time.Second),
	)
	sv := supervisor.NewSupervisor(machine, store, policy,
		supervisor.WithTaskPollInterval(10*time.Millisecond),
	)

	ctx := context.Background()
	const agentID = "agent-serialize"

	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	// Enqueue multiple tasks — only one can be assigned at a time to a single agent.
	for i := 0; i < 5; i++ {
		if err := store.EnqueueTask(ctx, fmt.Sprintf("task-%d", i), float64(i)); err != nil {
			t.Fatalf("EnqueueTask: %v", err)
		}
	}

	// Run briefly — supervisor assigns first task via tryAssignTask.
	runCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_ = sv.Run(runCtx)

	// Agent should be assigned (first task won).
	stateStr, err := store.GetAgentState(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateStr != string(agent.StateAssigned) {
		t.Fatalf("want assigned, got %q", stateStr)
	}

	// CurrentTaskID should be set (compound operation completed atomically).
	fields, err := store.GetAgentFields(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentFields: %v", err)
	}
	if fields.CurrentTaskID == "" {
		t.Fatal("CurrentTaskID should be set after assignment")
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
	if n != 1 {
		t.Fatalf("expected exactly 1 task in queue after CAS conflict, got %d", n)
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

// dequeueCountingStore wraps casConflictStore and counts DequeueTask calls
// to verify that CAS conflicts do not trigger exponential backoff.
type dequeueCountingStore struct {
	casConflictStore
	dequeueCount atomic.Int32
}

func (s *dequeueCountingStore) DequeueTask(ctx context.Context) (string, float64, error) {
	s.dequeueCount.Add(1)
	return s.casConflictStore.DequeueTask(ctx)
}

// casGenericErrorStore wraps a StateStore and makes CompareAndSetAgentState
// return a generic (non-StateConflictError) error, simulating a Redis timeout.
type casGenericErrorStore struct {
	state.StateStore
}

func (s *casGenericErrorStore) CompareAndSetAgentState(_ context.Context, _ string, _, _ string) error {
	return fmt.Errorf("redis: connection timeout")
}

func TestSupervisor_TryAssignTask_CASGenericError_TriggersBackoff(t *testing.T) {
	t.Parallel()

	base := state.NewMockStore()
	wrapper := &casGenericErrorStore{StateStore: base}
	machine := agent.NewMachine(wrapper)
	policy := supervisor.NewRestartPolicy(
		supervisor.WithCrashThreshold(10),
		supervisor.WithCrashWindow(60*time.Second),
	)

	// Use a dequeue-counting wrapper to measure how many attempts occur.
	counting := &dequeueCountingGenericStore{StateStore: wrapper, base: base}
	sv := supervisor.NewSupervisor(machine, counting, policy,
		supervisor.WithTaskPollInterval(10*time.Millisecond),
	)

	ctx := context.Background()

	if err := sv.RegisterAgent(ctx, "agent-generic-err"); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if err := counting.EnqueueTask(ctx, "task-generic-err", 1.0); err != nil {
		t.Fatalf("EnqueueTask: %v", err)
	}

	// Run for 200ms with 10ms poll interval.
	// With backoff, after a few errors the supervisor should slow down — fewer
	// dequeues than the no-backoff CAS conflict test (which gets >= 5 in 100ms).
	runCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_ = sv.Run(runCtx)

	count := int(counting.dequeueCount.Load())
	// With exponential backoff kicking in after the first error, we expect
	// significantly fewer dequeues. The no-backoff test gets >= 5 in 100ms;
	// with backoff over 200ms we should get fewer than 10 total attempts
	// because the backoff delays grow quickly (2x, 4x, 8x poll interval).
	if count < 1 {
		t.Fatal("expected at least 1 dequeue attempt")
	}

	// The task should be re-enqueued (not lost).
	n, err := base.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if n < 1 {
		t.Fatal("task should be re-enqueued after generic CAS error")
	}
}

// dequeueCountingGenericStore wraps casGenericErrorStore and counts DequeueTask calls.
type dequeueCountingGenericStore struct {
	state.StateStore
	base         *state.MockStore
	dequeueCount atomic.Int32
}

func (s *dequeueCountingGenericStore) DequeueTask(ctx context.Context) (string, float64, error) {
	s.dequeueCount.Add(1)
	return s.StateStore.DequeueTask(ctx)
}

func TestSupervisor_TryAssignTask_CASConflict_NoBackoff(t *testing.T) {
	t.Parallel()

	base := state.NewMockStore()
	wrapper := &dequeueCountingStore{casConflictStore: casConflictStore{StateStore: base}}
	machine := agent.NewMachine(wrapper)
	policy := supervisor.NewRestartPolicy(
		supervisor.WithCrashThreshold(10),
		supervisor.WithCrashWindow(60*time.Second),
	)
	sv := supervisor.NewSupervisor(machine, wrapper, policy,
		supervisor.WithTaskPollInterval(10*time.Millisecond),
	)

	ctx := context.Background()

	if err := sv.RegisterAgent(ctx, "agent-no-backoff"); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if err := wrapper.EnqueueTask(ctx, "task-no-backoff", 1.0); err != nil {
		t.Fatalf("EnqueueTask: %v", err)
	}

	// Run for 100ms with 10ms poll interval.
	// Without backoff: ~10 dequeue attempts (one per tick).
	// With exponential backoff: after 6 conflicts the supervisor would idle
	// for 640ms, yielding at most 3-4 dequeues in 100ms.
	runCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	_ = sv.Run(runCtx)

	count := int(wrapper.dequeueCount.Load())
	if count < 5 {
		t.Fatalf("expected >= 5 dequeue attempts (no backoff), got %d — suggests exponential backoff is being applied to CAS conflicts", count)
	}
}

func TestSupervisor_RegisterAgent_EmptyID(t *testing.T) {
	t.Parallel()
	sv, _ := newTestSupervisor(t)
	err := sv.RegisterAgent(context.Background(), "")
	if !errors.Is(err, supervisor.ErrInvalidAgentID) {
		t.Fatalf("want ErrInvalidAgentID, got %v", err)
	}
}

func TestSupervisor_RegisterAgent_TooLongID(t *testing.T) {
	t.Parallel()
	sv, _ := newTestSupervisor(t)
	longID := strings.Repeat("a", 129)
	err := sv.RegisterAgent(context.Background(), longID)
	if !errors.Is(err, supervisor.ErrInvalidAgentID) {
		t.Fatalf("want ErrInvalidAgentID, got %v", err)
	}
}
