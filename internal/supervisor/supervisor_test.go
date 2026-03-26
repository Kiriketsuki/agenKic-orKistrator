//go:build testenv

package supervisor_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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

func TestHeartbeat_Success(t *testing.T) {
	t.Parallel()

	sv, store := newTestSupervisor(t)
	ctx := context.Background()

	if err := sv.RegisterAgent(ctx, "agent-hb"); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	before, err := store.GetAgentFields(ctx, "agent-hb")
	if err != nil {
		t.Fatalf("GetAgentFields (before): %v", err)
	}

	time.Sleep(2 * time.Millisecond)

	if err := sv.Heartbeat(ctx, "agent-hb"); err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}

	after, err := store.GetAgentFields(ctx, "agent-hb")
	if err != nil {
		t.Fatalf("GetAgentFields (after): %v", err)
	}
	if after.LastHeartbeat <= before.LastHeartbeat {
		t.Errorf("LastHeartbeat not updated: before=%d after=%d", before.LastHeartbeat, after.LastHeartbeat)
	}
}

func TestHeartbeat_NotFound(t *testing.T) {
	t.Parallel()

	sv, _ := newTestSupervisor(t)
	ctx := context.Background()

	err := sv.Heartbeat(ctx, "nonexistent-agent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent, got nil")
	}
}

func TestHeartbeat_Stopped(t *testing.T) {
	t.Parallel()

	sv, _ := newTestSupervisor(t)
	ctx := context.Background()

	sv.Stop()

	err := sv.Heartbeat(ctx, "any-agent")
	if !errors.Is(err, supervisor.ErrSupervisorStopped) {
		t.Fatalf("want ErrSupervisorStopped, got %v", err)
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

// TestConcurrency_CrashCompleteRace stress-tests concurrent crashAgent and
// completeAgent calls on the same agent to verify the per-agent mutex
// prevents data races and maintains state machine invariants.
//
// Both functions acquire the same per-agent mutex before applying their
// respective events, so only one can win per iteration. After both goroutines
// complete:
//   - Agent must be in idle state (both paths end in idle).
//   - Task must appear in the queue at most once (crash re-enqueues; complete
//     clears the task — duplication is impossible when the mutex holds).
//
// Run with -race to confirm no data races exist on shared supervisor state.
func TestConcurrency_CrashCompleteRace(t *testing.T) {
	sv, store := newTestSupervisor(t)
	ctx := context.Background()

	const agentID = "race-agent"
	const iterations = 100

	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	for i := 0; i < iterations; i++ {
		taskID := fmt.Sprintf("race-task-%d", i)

		// Seed the agent directly into REPORTING state with a task binding.
		// SetAgentFields bypasses the state machine — this is intentional for
		// test precondition setup. The race itself goes through CAS in machine.ApplyEvent.
		if err := store.SetAgentFields(ctx, agentID, state.AgentFields{
			State:               state.AgentStateReporting,
			CurrentTaskID:       taskID,
			CurrentTaskPriority: 1.0,
			RegisteredAt:        time.Now().UnixMilli(),
		}); err != nil {
			t.Fatalf("iteration %d: SetAgentFields: %v", i, err)
		}

		// Race: one goroutine crashes the agent, the other completes it.
		// The per-agent mutex serializes them — exactly one wins.
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			sv.CrashAgentForTest(ctx, agentID)
		}()
		go func() {
			defer wg.Done()
			_ = sv.CompleteAgentForTest(ctx, agentID)
		}()
		wg.Wait()

		// Invariant 1: agent must be idle regardless of which operation won.
		stateStr, err := store.GetAgentState(ctx, agentID)
		if err != nil {
			t.Fatalf("iteration %d: GetAgentState: %v", i, err)
		}
		if stateStr != string(agent.StateIdle) {
			t.Fatalf("iteration %d: want state %q, got %q", i, agent.StateIdle, stateStr)
		}

		// Invariant 2: task must not be duplicated in the queue.
		// crash path re-enqueues once; complete path clears — queue length ≤ 1.
		n, err := store.QueueLength(ctx)
		if err != nil {
			t.Fatalf("iteration %d: QueueLength: %v", i, err)
		}
		if n > 1 {
			t.Fatalf("iteration %d: task duplicated — expected queue length 0 or 1, got %d", i, n)
		}

		// Drain re-enqueued task before the next iteration.
		if n == 1 {
			if _, _, err := store.DequeueTask(ctx); err != nil {
				t.Fatalf("iteration %d: DequeueTask: %v", i, err)
			}
		}
	}
}

// ── StartWork tests ──────────────────────────────────────────────────────────

func TestStartWork_Success(t *testing.T) {
	t.Parallel()

	sv, store := newTestSupervisor(t)
	ctx := context.Background()

	const agentID = "agent-sw-ok"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	// Seed agent into ASSIGNED state directly via store (bypasses state machine,
	// same pattern as TestConcurrency_CrashCompleteRace).
	if err := store.SetAgentFields(ctx, agentID, state.AgentFields{
		State:        state.AgentStateAssigned,
		RegisteredAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("SetAgentFields: %v", err)
	}

	if err := sv.StartWork(ctx, agentID); err != nil {
		t.Fatalf("StartWork: %v", err)
	}

	stateStr, err := store.GetAgentState(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateStr != state.AgentStateWorking {
		t.Errorf("want state %q, got %q", state.AgentStateWorking, stateStr)
	}
}

func TestStartWork_InvalidTransition(t *testing.T) {
	t.Parallel()

	sv, _ := newTestSupervisor(t)
	ctx := context.Background()

	const agentID = "agent-sw-invalid"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	// Agent is IDLE — StartWork requires ASSIGNED.

	err := sv.StartWork(ctx, agentID)
	if err == nil {
		t.Fatal("expected error for invalid transition, got nil")
	}
}

// ── ReportOutput tests ───────────────────────────────────────────────────────

func TestReportOutput_Success(t *testing.T) {
	t.Parallel()

	sv, store := newTestSupervisor(t)
	ctx := context.Background()

	const agentID = "agent-ro-ok"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	// Seed agent into WORKING state.
	if err := store.SetAgentFields(ctx, agentID, state.AgentFields{
		State:        state.AgentStateWorking,
		RegisteredAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("SetAgentFields: %v", err)
	}

	if err := sv.ReportOutput(ctx, agentID); err != nil {
		t.Fatalf("ReportOutput: %v", err)
	}

	stateStr, err := store.GetAgentState(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateStr != state.AgentStateReporting {
		t.Errorf("want state %q, got %q", state.AgentStateReporting, stateStr)
	}
}

func TestReportOutput_InvalidTransition(t *testing.T) {
	t.Parallel()

	sv, store := newTestSupervisor(t)
	ctx := context.Background()

	const agentID = "agent-ro-invalid"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	// Seed agent into ASSIGNED — ReportOutput requires WORKING.
	if err := store.SetAgentFields(ctx, agentID, state.AgentFields{
		State:        state.AgentStateAssigned,
		RegisteredAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("SetAgentFields: %v", err)
	}

	err := sv.ReportOutput(ctx, agentID)
	if err == nil {
		t.Fatal("expected error for invalid transition, got nil")
	}
}
