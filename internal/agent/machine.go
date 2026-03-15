package agent

import (
	"context"
	"fmt"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// Machine drives an agent through its state lifecycle. Each ApplyEvent call
// reads the current state from the StateStore, validates the transition, and
// writes the new state back — returning an immutable AgentSnapshot.
//
// Machine has no internal mutable state; all state lives in the StateStore.
type Machine struct {
	store state.StateStore
}

// NewMachine returns a Machine that persists state via the given StateStore.
func NewMachine(store state.StateStore) *Machine {
	return &Machine{store: store}
}

// ApplyEvent reads the agent's current state, validates the transition, and
// persists the new state. It returns an immutable AgentSnapshot on success.
//
// Errors:
//   - state.ErrAgentNotFound if the agent has no record in the store.
//   - *InvalidTransitionError if the (current, event) pair is not valid.
//   - Any storage error from the underlying StateStore.
func (m *Machine) ApplyEvent(ctx context.Context, agentID string, event AgentEvent) (AgentSnapshot, error) {
	rawState, err := m.store.GetAgentState(ctx, agentID)
	if err != nil {
		return AgentSnapshot{}, err
	}

	current, err := ParseAgentState(rawState)
	if err != nil {
		return AgentSnapshot{}, fmt.Errorf("agent %s has unrecognised state %q: %w", agentID, rawState, err)
	}

	next, err := ValidTransition(current, event)
	if err != nil {
		return AgentSnapshot{}, err
	}

	if err := m.store.SetAgentState(ctx, agentID, string(next)); err != nil {
		return AgentSnapshot{}, fmt.Errorf("persist state for agent %s: %w", agentID, err)
	}

	return AgentSnapshot{AgentID: agentID, State: next}, nil
}
