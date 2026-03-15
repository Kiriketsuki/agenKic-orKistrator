package agent_test

import (
	"context"
	"errors"
	"strings"
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
