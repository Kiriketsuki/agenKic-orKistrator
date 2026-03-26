package agent

import (
	"fmt"

	pkgstate "github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// AgentState represents the lifecycle state of a single agent.
type AgentState string

// State constants are aliases of the canonical values in the state package so
// that a value change propagates here at compile time.
const (
	StateIdle      AgentState = pkgstate.AgentStateIdle
	StateAssigned  AgentState = pkgstate.AgentStateAssigned
	StateWorking   AgentState = pkgstate.AgentStateWorking
	StateReporting AgentState = pkgstate.AgentStateReporting
)

// String implements fmt.Stringer.
func (s AgentState) String() string { return string(s) }

// ParseAgentState converts a raw string into an AgentState.
// Returns an error for unrecognised values.
func ParseAgentState(s string) (AgentState, error) {
	switch AgentState(s) {
	case StateIdle, StateAssigned, StateWorking, StateReporting:
		return AgentState(s), nil
	default:
		return "", fmt.Errorf("unknown agent state: %q", s)
	}
}
