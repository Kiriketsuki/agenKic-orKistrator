package health_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/health"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// stubDAG is a minimal DAGStatusProvider for testing.
type stubDAG struct{ count int }

func (s *stubDAG) ActiveExecutionCount() int { return s.count }

func TestAggregator_AllHealthy(t *testing.T) {
	t.Parallel()
	store := state.NewMockStore()
	ctx := context.Background()

	// Register 2 agents: 1 idle, 1 working.
	_ = store.SetAgentState(ctx, "agent-1", "idle")
	_ = store.SetAgentState(ctx, "agent-2", "working")
	_ = store.EnqueueTask(ctx, "task-1", 1.0)
	_ = store.EnqueueTask(ctx, "task-2", 2.0)
	_ = store.EnqueueTask(ctx, "task-3", 3.0)

	agg := health.NewAggregator(store, &stubDAG{count: 1})
	snap := agg.Check(ctx)

	if !snap.Alive {
		t.Error("expected Alive=true")
	}
	if !snap.Ready {
		t.Errorf("expected Ready=true, reason=%q", snap.ReadyReason)
	}
	if snap.AgentsTotal != 2 {
		t.Errorf("AgentsTotal = %d, want 2", snap.AgentsTotal)
	}
	if snap.AgentsIdle != 1 {
		t.Errorf("AgentsIdle = %d, want 1", snap.AgentsIdle)
	}
	if snap.AgentsWorking != 1 {
		t.Errorf("AgentsWorking = %d, want 1", snap.AgentsWorking)
	}
	if snap.TasksQueued != 3 {
		t.Errorf("TasksQueued = %d, want 3", snap.TasksQueued)
	}
	if snap.TasksInFlight != 1 {
		t.Errorf("TasksInFlight = %d, want 1", snap.TasksInFlight)
	}
	if snap.DAGsInProgress != 1 {
		t.Errorf("DAGsInProgress = %d, want 1", snap.DAGsInProgress)
	}
	if !snap.RedisOK {
		t.Error("expected RedisOK=true")
	}
}

func TestAggregator_NoAgents(t *testing.T) {
	t.Parallel()
	store := state.NewMockStore()
	agg := health.NewAggregator(store, &stubDAG{})
	snap := agg.Check(context.Background())

	if !snap.Alive {
		t.Error("expected Alive=true")
	}
	if snap.Ready {
		t.Error("expected Ready=false with no agents")
	}
	if !strings.Contains(snap.ReadyReason, "no agents") {
		t.Errorf("ReadyReason = %q, want to contain 'no agents'", snap.ReadyReason)
	}
}

func TestAggregator_RedisPingFails(t *testing.T) {
	t.Parallel()
	store := state.NewMockStore()
	ctx := context.Background()
	_ = store.SetAgentState(ctx, "agent-1", "idle")
	store.SetPingError(errors.New("connection refused"))

	agg := health.NewAggregator(store, &stubDAG{})
	snap := agg.Check(ctx)

	if !snap.Alive {
		t.Error("expected Alive=true")
	}
	if snap.Ready {
		t.Error("expected Ready=false when Redis is down")
	}
	if snap.RedisOK {
		t.Error("expected RedisOK=false")
	}
	if !strings.Contains(snap.ReadyReason, "redis") {
		t.Errorf("ReadyReason = %q, want to contain 'redis'", snap.ReadyReason)
	}
}

func TestAggregator_BothDown(t *testing.T) {
	t.Parallel()
	store := state.NewMockStore()
	store.SetPingError(errors.New("connection refused"))

	agg := health.NewAggregator(store, &stubDAG{})
	snap := agg.Check(context.Background())

	if snap.Ready {
		t.Error("expected Ready=false")
	}
	if !strings.Contains(snap.ReadyReason, "redis") {
		t.Errorf("ReadyReason = %q, want to contain 'redis'", snap.ReadyReason)
	}
	if !strings.Contains(snap.ReadyReason, "no agents") {
		t.Errorf("ReadyReason = %q, want to contain 'no agents'", snap.ReadyReason)
	}
}

func TestAggregator_CustomMinAgents_NotMet(t *testing.T) {
	t.Parallel()
	store := state.NewMockStore()
	ctx := context.Background()
	_ = store.SetAgentState(ctx, "agent-1", "idle")
	_ = store.SetAgentState(ctx, "agent-2", "idle")

	agg := health.NewAggregator(store, &stubDAG{}, health.WithMinAgents(3))
	snap := agg.Check(ctx)

	if snap.Ready {
		t.Error("expected Ready=false: 2 agents < 3 minimum")
	}
	if !strings.Contains(snap.ReadyReason, "below minimum") {
		t.Errorf("ReadyReason = %q, want 'below minimum'", snap.ReadyReason)
	}
}

func TestAggregator_CustomMinAgents_Met(t *testing.T) {
	t.Parallel()
	store := state.NewMockStore()
	ctx := context.Background()
	_ = store.SetAgentState(ctx, "agent-1", "idle")
	_ = store.SetAgentState(ctx, "agent-2", "idle")

	agg := health.NewAggregator(store, &stubDAG{}, health.WithMinAgents(2))
	snap := agg.Check(ctx)

	if !snap.Ready {
		t.Errorf("expected Ready=true with 2 agents and minAgents=2, reason=%q", snap.ReadyReason)
	}
}

func TestAggregator_AlwaysAlive(t *testing.T) {
	t.Parallel()
	store := state.NewMockStore()
	store.SetPingError(errors.New("down"))

	agg := health.NewAggregator(store, &stubDAG{})
	snap := agg.Check(context.Background())

	if !snap.Alive {
		t.Error("expected Alive=true regardless of subsystem state")
	}
}

func TestAggregator_UnknownAgentState(t *testing.T) {
	t.Parallel()
	store := state.NewMockStore()
	ctx := context.Background()
	_ = store.SetAgentState(ctx, "agent-1", "idle")
	_ = store.SetAgentState(ctx, "agent-2", "transmogrifying") // unrecognised

	agg := health.NewAggregator(store, &stubDAG{})
	snap := agg.Check(ctx)

	if snap.AgentsTotal != 2 {
		t.Errorf("AgentsTotal = %d, want 2", snap.AgentsTotal)
	}
	if snap.AgentsIdle != 1 {
		t.Errorf("AgentsIdle = %d, want 1", snap.AgentsIdle)
	}
	if snap.AgentsUnknown != 1 {
		t.Errorf("AgentsUnknown = %d, want 1", snap.AgentsUnknown)
	}
	// Total must equal the sum of all buckets.
	bucketSum := snap.AgentsIdle + snap.AgentsWorking + snap.AgentsAssigned + snap.AgentsReporting + snap.AgentsUnknown
	if bucketSum != snap.AgentsTotal {
		t.Errorf("bucket sum %d != AgentsTotal %d", bucketSum, snap.AgentsTotal)
	}
}

func TestAggregator_GetAllAgentStatesError(t *testing.T) {
	t.Parallel()
	store := state.NewMockStore()
	ctx := context.Background()
	_ = store.SetAgentState(ctx, "agent-1", "idle")
	store.SetGetAllAgentStatesError(errors.New("store failure"))

	agg := health.NewAggregator(store, &stubDAG{})
	snap := agg.Check(ctx)

	if snap.Ready {
		t.Error("expected Ready=false when GetAllAgentStates errors")
	}
	if snap.RedisOK {
		t.Error("expected RedisOK=false when store method fails")
	}
	if !strings.Contains(snap.ReadyReason, "agent states unavailable") {
		t.Errorf("ReadyReason = %q, want to contain 'agent states unavailable'", snap.ReadyReason)
	}
	// Should not say "no agents registered" — that would be misleading.
	if strings.Contains(snap.ReadyReason, "no agents") {
		t.Errorf("ReadyReason = %q, must not mention 'no agents' on store failure", snap.ReadyReason)
	}
}
