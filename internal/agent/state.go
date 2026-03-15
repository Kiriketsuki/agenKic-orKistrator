package agent

import "fmt"

// AgentState represents the lifecycle state of a single agent.
type AgentState string

const (
	StateIdle      AgentState = "idle"
	StateAssigned  AgentState = "assigned"
	StateWorking   AgentState = "working"
	StateReporting AgentState = "reporting"
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
