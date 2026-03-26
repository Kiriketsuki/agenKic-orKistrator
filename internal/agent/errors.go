package agent

import "fmt"

// InvalidTransitionError is returned by ValidTransition and Machine.ApplyEvent
// when the (from, event) pair has no valid target state.
type InvalidTransitionError struct {
	From  AgentState
	Event AgentEvent
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid transition: state=%s event=%s", e.From, e.Event)
}
