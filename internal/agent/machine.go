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
// atomically persists the new state via CompareAndSetAgentState. It returns an
// immutable AgentSnapshot on success.
//
// Errors:
//   - state.ErrAgentNotFound if the agent has no record in the store.
//   - *InvalidTransitionError if the (current, event) pair is not valid.
//   - *state.StateConflictError if the state changed between read and CAS
//     (concurrent modification detected).
//   - Any storage error from the underlying StateStore.
//
// Concurrency: ApplyEvent is safe for concurrent calls on the same agentID at
// the storage level — CompareAndSetAgentState provides atomicity for state
// transitions. The supervisor's per-agent mutex remains necessary for
// correctness of compound operations (e.g., state transition followed by field
// writes in tryAssignTask). CAS replaced the mutex as the atomicity guard for
// state transitions themselves; the mutex now coordinates compound operations
// within a single supervisor process.
//
// Event publishing: the Machine handles only state transitions. Callers
// are responsible for publishing domain events via StateStore.PublishEvent
// with full context (TaskID, Payload). The returned AgentSnapshot includes
// PreviousState and Event to support this pattern.
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

	if err := m.store.CompareAndSetAgentState(ctx, agentID, rawState, string(next)); err != nil {
		return AgentSnapshot{}, err
	}

	return AgentSnapshot{
		AgentID:       agentID,
		PreviousState: current,
		State:         next,
		Event:         event,
	}, nil
}
