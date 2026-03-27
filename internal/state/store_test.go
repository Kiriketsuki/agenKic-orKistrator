package state_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// RunStateStoreConformance runs the shared behavioural contract for any
// StateStore implementation. Call it from each implementation's own _test.go
// file with a freshly-initialised store.
func RunStateStoreConformance(t *testing.T, store state.StateStore) {
	t.Helper()
	ctx := context.Background()

	t.Run("Ping", func(t *testing.T) {
		if err := store.Ping(ctx); err != nil {
			t.Fatalf("Ping: %v", err)
		}
	})

	t.Run("SetAgentState/GetAgentState roundtrip", func(t *testing.T) {
		const agentID = "agent-rt-001"
		if err := store.SetAgentState(ctx, agentID, "idle"); err != nil {
			t.Fatalf("SetAgentState: %v", err)
		}
		got, err := store.GetAgentState(ctx, agentID)
		if err != nil {
			t.Fatalf("GetAgentState: %v", err)
		}
		if got != "idle" {
			t.Fatalf("want state=idle, got %q", got)
		}
	})

	t.Run("GetAgentState unknown agent returns ErrAgentNotFound", func(t *testing.T) {
		_, err := store.GetAgentState(ctx, "nonexistent-agent-xyz")
		if !errors.Is(err, state.ErrAgentNotFound) {
			t.Fatalf("want ErrAgentNotFound, got %v", err)
		}
	})

	t.Run("SetAgentFields/GetAgentFields roundtrip", func(t *testing.T) {
		const agentID = "agent-fields-001"
		now := time.Now().UnixMilli()
		want := state.AgentFields{
			State:         "working",
			LastHeartbeat: now,
			CurrentTaskID: "task-42",
			RegisteredAt:  now - 5000,
		}
		if err := store.SetAgentFields(ctx, agentID, want); err != nil {
			t.Fatalf("SetAgentFields: %v", err)
		}
		got, err := store.GetAgentFields(ctx, agentID)
		if err != nil {
			t.Fatalf("GetAgentFields: %v", err)
		}
		if got != want {
			t.Fatalf("fields mismatch:\n got  %+v\n want %+v", got, want)
		}
	})

	t.Run("GetAgentFields unknown agent returns ErrAgentNotFound", func(t *testing.T) {
		_, err := store.GetAgentFields(ctx, "nonexistent-fields-xyz")
		if !errors.Is(err, state.ErrAgentNotFound) {
			t.Fatalf("want ErrAgentNotFound, got %v", err)
		}
	})

	t.Run("SetAgentState then GetAgentFields returns zero-value numerics", func(t *testing.T) {
		const agentID = "agent-partial-001"
		if err := store.SetAgentState(ctx, agentID, "idle"); err != nil {
			t.Fatalf("SetAgentState: %v", err)
		}
		got, err := store.GetAgentFields(ctx, agentID)
		if err != nil {
			t.Fatalf("GetAgentFields after SetAgentState: %v", err)
		}
		if got.State != "idle" {
			t.Fatalf("want state=idle, got %q", got.State)
		}
		if got.LastHeartbeat != 0 {
			t.Fatalf("want LastHeartbeat=0 (unset), got %d", got.LastHeartbeat)
		}
		if got.RegisteredAt != 0 {
			t.Fatalf("want RegisteredAt=0 (unset), got %d", got.RegisteredAt)
		}
	})

	t.Run("DeleteAgent removes agent", func(t *testing.T) {
		const agentID = "agent-del-001"
		if err := store.SetAgentState(ctx, agentID, "idle"); err != nil {
			t.Fatalf("SetAgentState: %v", err)
		}
		if err := store.DeleteAgent(ctx, agentID); err != nil {
			t.Fatalf("DeleteAgent: %v", err)
		}
		_, err := store.GetAgentState(ctx, agentID)
		if !errors.Is(err, state.ErrAgentNotFound) {
			t.Fatalf("want ErrAgentNotFound after delete, got %v", err)
		}
	})

	t.Run("ListAgents returns registered agents", func(t *testing.T) {
		// Use unique IDs to avoid cross-test pollution.
		ids := []string{"agent-list-001", "agent-list-002"}
		for _, id := range ids {
			if err := store.SetAgentState(ctx, id, "idle"); err != nil {
				t.Fatalf("SetAgentState(%s): %v", id, err)
			}
		}
		agents, err := store.ListAgents(ctx)
		if err != nil {
			t.Fatalf("ListAgents: %v", err)
		}
		found := map[string]bool{}
		for _, a := range agents {
			found[a] = true
		}
		for _, id := range ids {
			if !found[id] {
				t.Errorf("ListAgents: missing %s", id)
			}
		}
	})

	t.Run("PublishEvent does not error", func(t *testing.T) {
		ev := state.Event{
			Type:    "task_assigned",
			AgentID: "agent-ev-001",
			TaskID:  "task-ev-001",
			Payload: `{"detail":"test"}`,
		}
		if err := store.PublishEvent(ctx, ev); err != nil {
			t.Fatalf("PublishEvent: %v", err)
		}
	})

	t.Run("EnqueueTask/DequeueTask roundtrip", func(t *testing.T) {
		// Enqueue two tasks with different priorities; higher priority (lower
		// score) should be dequeued first.
		if err := store.EnqueueTask(ctx, "task-q-low", 10.0); err != nil {
			t.Fatalf("EnqueueTask low: %v", err)
		}
		if err := store.EnqueueTask(ctx, "task-q-high", 1.0); err != nil {
			t.Fatalf("EnqueueTask high: %v", err)
		}
		got, err := store.DequeueTask(ctx)
		if err != nil {
			t.Fatalf("DequeueTask: %v", err)
		}
		if got != "task-q-high" {
			t.Fatalf("want task-q-high (priority=1.0), got %q", got)
		}
		// Clean up second task.
		if _, err := store.DequeueTask(ctx); err != nil {
			t.Fatalf("DequeueTask cleanup: %v", err)
		}
	})

	t.Run("DequeueTask on empty queue returns ErrQueueEmpty", func(t *testing.T) {
		// Drain any leftover tasks from previous sub-tests.
		for {
			_, err := store.DequeueTask(ctx)
			if err != nil {
				break
			}
		}
		_, err := store.DequeueTask(ctx)
		if !errors.Is(err, state.ErrQueueEmpty) {
			t.Fatalf("want ErrQueueEmpty, got %v", err)
		}
	})

	t.Run("GetAllAgentStates returns all agent states", func(t *testing.T) {
		const (
			id1 = "agent-gas-001"
			id2 = "agent-gas-002"
		)
		if err := store.SetAgentState(ctx, id1, "idle"); err != nil {
			t.Fatalf("SetAgentState: %v", err)
		}
		if err := store.SetAgentState(ctx, id2, "working"); err != nil {
			t.Fatalf("SetAgentState: %v", err)
		}
		got, err := store.GetAllAgentStates(ctx)
		if err != nil {
			t.Fatalf("GetAllAgentStates: %v", err)
		}
		if got[id1] != "idle" {
			t.Errorf("state[%s] = %q, want 'idle'", id1, got[id1])
		}
		if got[id2] != "working" {
			t.Errorf("state[%s] = %q, want 'working'", id2, got[id2])
		}
	})

	t.Run("QueueLength reflects enqueued tasks", func(t *testing.T) {
		// Drain first.
		for {
			_, err := store.DequeueTask(ctx)
			if err != nil {
				break
			}
		}
		n, err := store.QueueLength(ctx)
		if err != nil {
			t.Fatalf("QueueLength (empty): %v", err)
		}
		if n != 0 {
			t.Fatalf("want 0, got %d", n)
		}
		if err := store.EnqueueTask(ctx, "task-ql-1", 1.0); err != nil {
			t.Fatalf("EnqueueTask: %v", err)
		}
		n, err = store.QueueLength(ctx)
		if err != nil {
			t.Fatalf("QueueLength (one): %v", err)
		}
		if n != 1 {
			t.Fatalf("want 1, got %d", n)
		}
		// Clean up.
		store.DequeueTask(ctx) //nolint:errcheck
	})
}
