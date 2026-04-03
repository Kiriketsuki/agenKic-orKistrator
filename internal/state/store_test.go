package state_test

import (
	"context"
	"errors"
	"fmt"
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
		got, gotPri, err := store.DequeueTask(ctx)
		if err != nil {
			t.Fatalf("DequeueTask: %v", err)
		}
		if got != "task-q-high" {
			t.Fatalf("want task-q-high (priority=1.0), got %q", got)
		}
		if gotPri != 1.0 {
			t.Fatalf("want priority 1.0, got %v", gotPri)
		}
		// Clean up second task.
		if _, _, err := store.DequeueTask(ctx); err != nil {
			t.Fatalf("DequeueTask cleanup: %v", err)
		}
	})

	t.Run("DequeueTask on empty queue returns ErrQueueEmpty", func(t *testing.T) {
		// Drain any leftover tasks from previous sub-tests.
		for {
			_, _, err := store.DequeueTask(ctx)
			if err != nil {
				break
			}
		}
		_, _, err := store.DequeueTask(ctx)
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

	t.Run("ClearCurrentTask clears task binding", func(t *testing.T) {
		const agentID = "agent-cct-001"
		now := time.Now().UnixMilli()
		if err := store.SetAgentFields(ctx, agentID, state.AgentFields{
			State:               "assigned",
			LastHeartbeat:       now,
			CurrentTaskID:       "task-bound",
			CurrentTaskPriority: 5.0,
			RegisteredAt:        now - 1000,
		}); err != nil {
			t.Fatalf("SetAgentFields: %v", err)
		}
		if err := store.ClearCurrentTask(ctx, agentID); err != nil {
			t.Fatalf("ClearCurrentTask: %v", err)
		}
		got, err := store.GetAgentFields(ctx, agentID)
		if err != nil {
			t.Fatalf("GetAgentFields after clear: %v", err)
		}
		if got.CurrentTaskID != "" {
			t.Fatalf("want CurrentTaskID=\"\", got %q", got.CurrentTaskID)
		}
		if got.CurrentTaskPriority != 0 {
			t.Fatalf("want CurrentTaskPriority=0, got %v", got.CurrentTaskPriority)
		}
		// Other fields preserved.
		if got.State != "assigned" {
			t.Fatalf("want State=assigned, got %q", got.State)
		}
		if got.LastHeartbeat != now {
			t.Fatalf("want LastHeartbeat=%d, got %d", now, got.LastHeartbeat)
		}
	})

	t.Run("ClearCurrentTask on non-existent agent returns ErrAgentNotFound", func(t *testing.T) {
		err := store.ClearCurrentTask(ctx, "nonexistent-cct-xyz")
		if !errors.Is(err, state.ErrAgentNotFound) {
			t.Fatalf("want ErrAgentNotFound, got %v", err)
		}
	})

	t.Run("QueueLength reflects enqueued tasks", func(t *testing.T) {
		// Drain first.
		for {
			_, _, err := store.DequeueTask(ctx)
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
		_, _, _ = store.DequeueTask(ctx)
	})

	t.Run("CompareAndSetAgentState succeeds when expected matches", func(t *testing.T) {
		const agentID = "agent-cas-001"
		if err := store.SetAgentState(ctx, agentID, "idle"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		if err := store.CompareAndSetAgentState(ctx, agentID, "idle", "assigned"); err != nil {
			t.Fatalf("CAS: %v", err)
		}
		got, err := store.GetAgentState(ctx, agentID)
		if err != nil {
			t.Fatalf("GetAgentState: %v", err)
		}
		if got != "assigned" {
			t.Fatalf("want state=assigned, got %q", got)
		}
	})

	t.Run("CompareAndSetAgentState returns StateConflictError on mismatch", func(t *testing.T) {
		const agentID = "agent-cas-002"
		if err := store.SetAgentState(ctx, agentID, "working"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		err := store.CompareAndSetAgentState(ctx, agentID, "idle", "assigned")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var conflict *state.StateConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("want *StateConflictError, got %T: %v", err, err)
		}
		if conflict.Expected != "idle" {
			t.Fatalf("Expected field: want \"idle\", got %q", conflict.Expected)
		}
		if conflict.Actual != "working" {
			t.Fatalf("Actual field: want \"working\", got %q", conflict.Actual)
		}
		// State must not change.
		got, _ := store.GetAgentState(ctx, agentID)
		if got != "working" {
			t.Fatalf("state should remain \"working\", got %q", got)
		}
	})

	t.Run("CompareAndSetAgentState returns ErrAgentNotFound for unknown agent", func(t *testing.T) {
		err := store.CompareAndSetAgentState(ctx, "agent-cas-ghost", "idle", "assigned")
		if !errors.Is(err, state.ErrAgentNotFound) {
			t.Fatalf("want ErrAgentNotFound, got %v", err)
		}
	})

	t.Run("CompareAndSetAgentState concurrent race exactly one wins", func(t *testing.T) {
		const agentID = "agent-cas-race"
		if err := store.SetAgentState(ctx, agentID, "idle"); err != nil {
			t.Fatalf("seed: %v", err)
		}

		const n = 10
		errs := make(chan error, n)
		for i := 0; i < n; i++ {
			go func() {
				errs <- store.CompareAndSetAgentState(ctx, agentID, "idle", "assigned")
			}()
		}

		var wins, conflicts int
		for i := 0; i < n; i++ {
			err := <-errs
			if err == nil {
				wins++
			} else {
				var conflict *state.StateConflictError
				if !errors.As(err, &conflict) {
					t.Errorf("unexpected error type %T: %v", err, err)
				}
				conflicts++
			}
		}
		if wins != 1 {
			t.Fatalf("want exactly 1 winner, got %d", wins)
		}
		if conflicts != n-1 {
			t.Fatalf("want %d conflicts, got %d", n-1, conflicts)
		}
		got, _ := store.GetAgentState(ctx, agentID)
		if got != "assigned" {
			t.Fatalf("final state: want \"assigned\", got %q", got)
		}
	})

	t.Run("ReadEvents returns published events", func(t *testing.T) {
		evs := []state.Event{
			{Type: "re-type-1", AgentID: "re-agent-1", TaskID: "re-task-1"},
			{Type: "re-type-2", AgentID: "re-agent-2", TaskID: "re-task-2"},
			{Type: "re-type-3", AgentID: "re-agent-3", TaskID: "re-task-3"},
		}
		for _, ev := range evs {
			if err := store.PublishEvent(ctx, ev); err != nil {
				t.Fatalf("PublishEvent: %v", err)
			}
		}
		// Drain from beginning; prior sub-tests may have published events too.
		all, err := store.ReadEvents(ctx, "0", 1000)
		if err != nil {
			t.Fatalf("ReadEvents: %v", err)
		}
		if len(all) < 3 {
			t.Fatalf("want at least 3 events, got %d", len(all))
		}
		// Verify the last 3 match what we published.
		got := all[len(all)-3:]
		for i, ev := range evs {
			if got[i].ID == "" {
				t.Fatalf("event[%d] has empty ID", i)
			}
			if got[i].Event.Type != ev.Type {
				t.Fatalf("event[%d].Type: want %q, got %q", i, ev.Type, got[i].Event.Type)
			}
			if got[i].Event.AgentID != ev.AgentID {
				t.Fatalf("event[%d].AgentID: want %q, got %q", i, ev.AgentID, got[i].Event.AgentID)
			}
			if got[i].Event.TaskID != ev.TaskID {
				t.Fatalf("event[%d].TaskID: want %q, got %q", i, ev.TaskID, got[i].Event.TaskID)
			}
		}
	})

	t.Run("ReadEvents with cursor returns only newer events", func(t *testing.T) {
		evs := []state.Event{
			{Type: "cur-type-1", AgentID: "cur-agent-1", TaskID: "cur-task-1"},
			{Type: "cur-type-2", AgentID: "cur-agent-2", TaskID: "cur-task-2"},
			{Type: "cur-type-3", AgentID: "cur-agent-3", TaskID: "cur-task-3"},
		}
		for _, ev := range evs {
			if err := store.PublishEvent(ctx, ev); err != nil {
				t.Fatalf("PublishEvent: %v", err)
			}
		}
		// Read all to locate our 3 events (they are the last 3).
		all, err := store.ReadEvents(ctx, "0", 1000)
		if err != nil {
			t.Fatalf("ReadEvents all: %v", err)
		}
		if len(all) < 3 {
			t.Fatalf("want at least 3 events, got %d", len(all))
		}
		// The 2nd of our published events is at index len(all)-2.
		secondID := all[len(all)-2].ID

		// Read events after the 2nd one — should only get the 3rd.
		after, err := store.ReadEvents(ctx, secondID, 10)
		if err != nil {
			t.Fatalf("ReadEvents with cursor: %v", err)
		}
		if len(after) != 1 {
			t.Fatalf("want 1 event after cursor, got %d", len(after))
		}
		if after[0].Event.Type != "cur-type-3" {
			t.Fatalf("want cur-type-3, got %q", after[0].Event.Type)
		}
	})

	t.Run("ReadEvents on empty stream returns empty slice", func(t *testing.T) {
		// The stream may have events from prior sub-tests. Advance to the
		// current tail, then assert no further events exist.
		all, err := store.ReadEvents(ctx, "0", 1000)
		if err != nil {
			t.Fatalf("ReadEvents drain: %v", err)
		}
		if len(all) == 0 {
			// Stream was empty — already proven.
			return
		}
		lastID := all[len(all)-1].ID
		after, err := store.ReadEvents(ctx, lastID, 10)
		if err != nil {
			t.Fatalf("ReadEvents after tail: %v", err)
		}
		if len(after) != 0 {
			t.Fatalf("want empty slice after tail, got %d events", len(after))
		}
	})

	t.Run("CreateConsumerGroup is idempotent", func(t *testing.T) {
		if err := store.CreateConsumerGroup(ctx, "cg-idem", "0"); err != nil {
			t.Fatalf("CreateConsumerGroup (1st): %v", err)
		}
		if err := store.CreateConsumerGroup(ctx, "cg-idem", "0"); err != nil {
			t.Fatalf("CreateConsumerGroup (2nd): %v", err)
		}
	})

	t.Run("SubscribeEvents delivers events to a consumer", func(t *testing.T) {
		// Create group at tail before publishing so we get exactly our 3 events.
		if err := store.CreateConsumerGroup(ctx, "cg-sub", "$"); err != nil {
			t.Fatalf("CreateConsumerGroup: %v", err)
		}
		evs := []state.Event{
			{Type: "sub-type-1", AgentID: "sub-agent-1", TaskID: "sub-task-1"},
			{Type: "sub-type-2", AgentID: "sub-agent-2", TaskID: "sub-task-2"},
			{Type: "sub-type-3", AgentID: "sub-agent-3", TaskID: "sub-task-3"},
		}
		for _, ev := range evs {
			if err := store.PublishEvent(ctx, ev); err != nil {
				t.Fatalf("PublishEvent: %v", err)
			}
		}
		got, err := store.SubscribeEvents(ctx, "cg-sub", "w1", 10, 0)
		if err != nil {
			t.Fatalf("SubscribeEvents: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("want 3 events, got %d", len(got))
		}
	})

	t.Run("Competing consumers receive distinct events", func(t *testing.T) {
		// Create group at the current tail BEFORE publishing so both consumers
		// only see our 6 events (works for mock and Redis alike).
		if err := store.CreateConsumerGroup(ctx, "cg-compete", "$"); err != nil {
			t.Fatalf("CreateConsumerGroup: %v", err)
		}

		for i := 1; i <= 6; i++ {
			ev := state.Event{
				Type:    "compete-type",
				AgentID: fmt.Sprintf("compete-agent-%d", i),
				TaskID:  fmt.Sprintf("compete-task-%d", i),
			}
			if err := store.PublishEvent(ctx, ev); err != nil {
				t.Fatalf("PublishEvent: %v", err)
			}
		}

		c1, err := store.SubscribeEvents(ctx, "cg-compete", "c1", 3, 0)
		if err != nil {
			t.Fatalf("SubscribeEvents c1: %v", err)
		}
		c2, err := store.SubscribeEvents(ctx, "cg-compete", "c2", 3, 0)
		if err != nil {
			t.Fatalf("SubscribeEvents c2: %v", err)
		}

		total := len(c1) + len(c2)
		if total != 6 {
			t.Fatalf("want 6 total events across consumers, got %d (c1=%d, c2=%d)", total, len(c1), len(c2))
		}
		seen := make(map[string]bool)
		for _, ev := range c1 {
			if seen[ev.ID] {
				t.Fatalf("duplicate event ID %q in c1", ev.ID)
			}
			seen[ev.ID] = true
		}
		for _, ev := range c2 {
			if seen[ev.ID] {
				t.Fatalf("event ID %q delivered to both consumers", ev.ID)
			}
			seen[ev.ID] = true
		}
	})

	t.Run("Acked events are not re-delivered", func(t *testing.T) {
		// Create the group at the current tail BEFORE publishing, then publish
		// 2 events. This works for both mock ("$" → len(streamEvents)) and Redis.
		if err := store.CreateConsumerGroup(ctx, "cg-ack", "$"); err != nil {
			t.Fatalf("CreateConsumerGroup: %v", err)
		}

		evs := []state.Event{
			{Type: "ack-type-1", AgentID: "ack-agent-1", TaskID: "ack-task-1"},
			{Type: "ack-type-2", AgentID: "ack-agent-2", TaskID: "ack-task-2"},
		}
		for _, ev := range evs {
			if err := store.PublishEvent(ctx, ev); err != nil {
				t.Fatalf("PublishEvent: %v", err)
			}
		}

		first, err := store.SubscribeEvents(ctx, "cg-ack", "w1", 10, 0)
		if err != nil {
			t.Fatalf("SubscribeEvents (1st): %v", err)
		}
		if len(first) != 2 {
			t.Fatalf("want 2 events on first subscribe, got %d", len(first))
		}

		ids := make([]string, len(first))
		for i, ev := range first {
			ids[i] = ev.ID
		}
		if err := store.AckEvent(ctx, "cg-ack", ids...); err != nil {
			t.Fatalf("AckEvent: %v", err)
		}

		second, err := store.SubscribeEvents(ctx, "cg-ack", "w1", 10, 0)
		if err != nil {
			t.Fatalf("SubscribeEvents (2nd): %v", err)
		}
		if len(second) != 0 {
			t.Fatalf("want 0 events after ack, got %d", len(second))
		}
	})
}
