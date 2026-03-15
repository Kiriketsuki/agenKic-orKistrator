package agent_test

import (
	"context"
	"errors"
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
		event agent.AgentEvent
		want  agent.AgentState
	}{
		{agent.EventTaskAssigned, agent.StateAssigned},
		{agent.EventWorkStarted, agent.StateWorking},
		{agent.EventOutputReady, agent.StateReporting},
		{agent.EventOutputDelivered, agent.StateIdle},
	}

	for _, step := range steps {
		snap, err := m.ApplyEvent(ctx, id, step.event)
		if err != nil {
			t.Fatalf("ApplyEvent(%s): %v", step.event, err)
		}
		if snap.State != step.want {
			t.Fatalf("after %s: want state=%s, got %s", step.event, step.want, snap.State)
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
