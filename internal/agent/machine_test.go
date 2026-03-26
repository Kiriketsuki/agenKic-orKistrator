package agent_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

func newMachine(t *testing.T) (*agent.Machine, *state.MockStore) {
	t.Helper()
	store := state.NewMockStore()
	return agent.NewMachine(store), store
}

func TestMachine_FullLifecycleRoundtrip(t *testing.T) {
	m, store := newMachine(t)
	ctx := context.Background()
	const id = "agent-01"

	// Seed idle state so GetAgentState finds the agent.
	if err := store.SetAgentState(ctx, id, string(agent.StateIdle)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	steps := []struct {
		event    agent.AgentEvent
		prevWant agent.AgentState
		want     agent.AgentState
	}{
		{agent.EventTaskAssigned, agent.StateIdle, agent.StateAssigned},
		{agent.EventWorkStarted, agent.StateAssigned, agent.StateWorking},
		{agent.EventOutputReady, agent.StateWorking, agent.StateReporting},
		{agent.EventOutputDelivered, agent.StateReporting, agent.StateIdle},
	}

	for _, step := range steps {
		snap, err := m.ApplyEvent(ctx, id, step.event)
		if err != nil {
			t.Fatalf("ApplyEvent(%s): %v", step.event, err)
		}
		if snap.State != step.want {
			t.Fatalf("after %s: want state=%s, got %s", step.event, step.want, snap.State)
		}
		if snap.PreviousState != step.prevWant {
			t.Fatalf("after %s: want prevState=%s, got %s", step.event, step.prevWant, snap.PreviousState)
		}
		if snap.Event != step.event {
			t.Fatalf("after %s: want event=%s, got %s", step.event, step.event, snap.Event)
		}
		if snap.AgentID != id {
			t.Fatalf("snapshot AgentID: want %s, got %s", id, snap.AgentID)
		}
		// Verify state was persisted.
		stored, err := store.GetAgentState(ctx, id)
		if err != nil {
			t.Fatalf("GetAgentState: %v", err)
		}
		if stored != string(step.want) {
			t.Fatalf("persisted state: want %s, got %s", step.want, stored)
		}
	}
}

func TestMachine_AgentFailed_ResetsToIdle(t *testing.T) {
	m, store := newMachine(t)
	ctx := context.Background()
	const id = "agent-fail-01"

	if err := store.SetAgentState(ctx, id, string(agent.StateWorking)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	snap, err := m.ApplyEvent(ctx, id, agent.EventAgentFailed)
	if err != nil {
		t.Fatalf("ApplyEvent(AgentFailed): %v", err)
	}
	if snap.State != agent.StateIdle {
		t.Fatalf("want idle, got %s", snap.State)
	}
	if snap.PreviousState != agent.StateWorking {
		t.Fatalf("want prevState=working, got %s", snap.PreviousState)
	}
	if snap.Event != agent.EventAgentFailed {
		t.Fatalf("want event=AgentFailed, got %s", snap.Event)
	}
}

func TestMachine_InvalidTransition_ReturnsError(t *testing.T) {
	m, store := newMachine(t)
	ctx := context.Background()
	const id = "agent-inv-01"

	if err := store.SetAgentState(ctx, id, string(agent.StateIdle)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := m.ApplyEvent(ctx, id, agent.EventOutputDelivered)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var te *agent.InvalidTransitionError
	if !errors.As(err, &te) {
		t.Fatalf("want *InvalidTransitionError, got %T: %v", err, err)
	}
	// State must remain unchanged.
	stored, _ := store.GetAgentState(ctx, id)
	if stored != string(agent.StateIdle) {
		t.Fatalf("state must not change on invalid transition; got %s", stored)
	}
}

func TestMachine_UnknownAgent_ReturnsError(t *testing.T) {
	m, _ := newMachine(t)
	ctx := context.Background()

	_, err := m.ApplyEvent(ctx, "no-such-agent", agent.EventTaskAssigned)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !errors.Is(err, state.ErrAgentNotFound) {
		t.Fatalf("want ErrAgentNotFound, got %v", err)
	}
}

func TestMachine_UnrecognisedState_ReturnsError(t *testing.T) {
	m, store := newMachine(t)
	ctx := context.Background()
	const id = "agent-badstate-01"

	if err := store.SetAgentState(ctx, id, "bogus_state"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err := m.ApplyEvent(ctx, id, agent.EventTaskAssigned)
	if err == nil {
		t.Fatal("expected error for unrecognised state, got nil")
	}
	if errors.Unwrap(err) == nil {
		t.Fatal("expected wrapped error, got unwrapped")
	}
	if !strings.Contains(err.Error(), "bogus_state") {
		t.Fatalf("error should contain seeded state; got: %v", err)
	}
}

// racyStore wraps a real StateStore and mutates state between Get and CAS to
// simulate a concurrent writer. This lets us test that ApplyEvent propagates
// *StateConflictError when the CAS detects a race.
type racyStore struct {
	state.StateStore
	interceptOnce sync.Once
	agentID       string
	raceTo        string
}

func (r *racyStore) CompareAndSetAgentState(ctx context.Context, agentID string, expected, next string) error {
	// On the first CAS call for the target agent, sneak in a state change
	// so the CAS will see a mismatch.
	if agentID == r.agentID {
		r.interceptOnce.Do(func() {
			if err := r.StateStore.SetAgentState(ctx, agentID, r.raceTo); err != nil {
				panic(fmt.Sprintf("racyStore: SetAgentState failed: %v", err))
			}
		})
	}
	return r.StateStore.CompareAndSetAgentState(ctx, agentID, expected, next)
}

func TestMachine_ApplyEvent_CASConflict(t *testing.T) {
	base := state.NewMockStore()
	ctx := context.Background()
	const id = "agent-cas-conflict"

	if err := base.SetAgentState(ctx, id, string(agent.StateIdle)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Wrap the store so it mutates state to "assigned" just before CAS fires.
	racy := &racyStore{StateStore: base, agentID: id, raceTo: string(agent.StateAssigned)}
	m := agent.NewMachine(racy)

	// ApplyEvent reads "idle", computes next="assigned", but CAS sees "assigned"
	// (set by the concurrent writer, not by this ApplyEvent call).
	_, err := m.ApplyEvent(ctx, id, agent.EventTaskAssigned)
	if err == nil {
		t.Fatal("expected error from CAS conflict, got nil")
	}
	var conflict *state.StateConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want *StateConflictError, got %T: %v", err, err)
	}
	if conflict.Expected != string(agent.StateIdle) {
		t.Fatalf("Expected: want %q, got %q", agent.StateIdle, conflict.Expected)
	}
	if conflict.Actual != string(agent.StateAssigned) {
		t.Fatalf("Actual: want %q, got %q", agent.StateAssigned, conflict.Actual)
	}
	// State should remain "assigned" (the concurrent writer's value).
	got, _ := base.GetAgentState(ctx, id)
	if got != string(agent.StateAssigned) {
		t.Fatalf("state: want %q, got %q", agent.StateAssigned, got)
	}
}

func TestMachine_ApplyEvent_HappyPath_UsesCAS(t *testing.T) {
	m, store := newMachine(t)
	ctx := context.Background()
	const id = "agent-cas-happy"

	if err := store.SetAgentState(ctx, id, string(agent.StateIdle)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	snap, err := m.ApplyEvent(ctx, id, agent.EventTaskAssigned)
	if err != nil {
		t.Fatalf("ApplyEvent: %v", err)
	}
	if snap.State != agent.StateAssigned {
		t.Fatalf("want assigned, got %s", snap.State)
	}
	if snap.PreviousState != agent.StateIdle {
		t.Fatalf("want prevState=idle, got %s", snap.PreviousState)
	}
	got, _ := store.GetAgentState(ctx, id)
	if got != string(agent.StateAssigned) {
		t.Fatalf("persisted state: want assigned, got %s", got)
	}
}

// TestMachine_ApplyEvent_ConcurrentCAS verifies that n goroutines calling
// ApplyEvent on the same idle agent produce exactly one success. With n=10,
// logs CAS conflict vs InvalidTransition counts for observability. The CAS
// conflict path is proven hermetically by TestMachine_ApplyEvent_CASConflict
// (racyStore); this test validates the exactly-one-winner invariant under
// real concurrency.
func TestMachine_ApplyEvent_ConcurrentCAS(t *testing.T) {
	store := state.NewMockStore()
	m := agent.NewMachine(store)
	ctx := context.Background()
	const id = "agent-concurrent-cas"

	if err := store.SetAgentState(ctx, id, string(agent.StateIdle)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const n = 10
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := m.ApplyEvent(ctx, id, agent.EventTaskAssigned)
			errs <- err
		}()
	}

	var wins, casConflicts, invalidTransitions int
	for i := 0; i < n; i++ {
		err := <-errs
		if err == nil {
			wins++
			continue
		}
		// The losing goroutine may get either:
		// - *StateConflictError: read "idle", CAS detected mismatch
		// - *InvalidTransitionError: read "assigned" (post-CAS), transition invalid
		// Both are valid concurrent outcomes.
		var conflict *state.StateConflictError
		var invalid *agent.InvalidTransitionError
		switch {
		case errors.As(err, &conflict):
			casConflicts++
		case errors.As(err, &invalid):
			invalidTransitions++
		default:
			t.Errorf("unexpected error type %T: %v", err, err)
		}
	}
	if wins != 1 {
		t.Fatalf("want exactly 1 winner, got %d", wins)
	}
	if casConflicts+invalidTransitions != n-1 {
		t.Fatalf("want %d losers, got %d (cas=%d, invalid=%d)",
			n-1, casConflicts+invalidTransitions, casConflicts, invalidTransitions)
	}
	t.Logf("loser distribution: cas_conflicts=%d, invalid_transitions=%d", casConflicts, invalidTransitions)

	got, _ := store.GetAgentState(ctx, id)
	if got != string(agent.StateAssigned) {
		t.Fatalf("final state: want %q, got %q", agent.StateAssigned, got)
	}
}

// TestMachine_AgentFailed_FromIdle documents intentional self-loop behaviour:
// EventAgentFailed is accepted from any state including StateIdle, producing a
// snapshot where PreviousState == State. Callers may use this for operational
// telemetry or choose to filter no-op transitions before publishing domain events.
func TestMachine_AgentFailed_FromIdle(t *testing.T) {
	m, store := newMachine(t)
	ctx := context.Background()
	const id = "agent-failidle-01"

	if err := store.SetAgentState(ctx, id, string(agent.StateIdle)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	snap, err := m.ApplyEvent(ctx, id, agent.EventAgentFailed)
	if err != nil {
		t.Fatalf("ApplyEvent(AgentFailed from Idle): %v", err)
	}
	if snap.State != agent.StateIdle {
		t.Fatalf("want state=idle, got %s", snap.State)
	}
	if snap.PreviousState != agent.StateIdle {
		t.Fatalf("want prevState=idle (self-loop), got %s", snap.PreviousState)
	}
	if snap.Event != agent.EventAgentFailed {
		t.Fatalf("want event=AgentFailed, got %s", snap.Event)
	}
}
