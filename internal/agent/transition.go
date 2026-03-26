package agent

// transitionTable maps (from, event) → to for all defined valid transitions.
// Any combination not present in this table is invalid.
var transitionTable = map[AgentState]map[AgentEvent]AgentState{
	StateIdle: {
		EventTaskAssigned: StateAssigned,
	},
	StateAssigned: {
		EventWorkStarted: StateWorking,
	},
	StateWorking: {
		EventOutputReady: StateReporting,
	},
	StateReporting: {
		EventOutputDelivered: StateIdle,
	},
}

// ValidTransition returns the target state for the given (from, event) pair.
// AgentFailed is accepted from any state and always resets to StateIdle.
// Any other unrecognised combination returns an *InvalidTransitionError.
//
// ValidTransition is a pure function; it has no side effects.
func ValidTransition(from AgentState, event AgentEvent) (AgentState, error) {
	if event == EventAgentFailed {
		return StateIdle, nil
	}

	if targets, ok := transitionTable[from]; ok {
		if to, ok := targets[event]; ok {
			return to, nil
		}
	}

	return "", &InvalidTransitionError{From: from, Event: event}
}
