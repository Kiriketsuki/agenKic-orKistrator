package agent

// AgentSnapshot is an immutable point-in-time view of a single agent's state.
// All fields are value types; no pointers are exposed.
type AgentSnapshot struct {
	AgentID       string
	PreviousState AgentState
	State         AgentState
	Event         AgentEvent
}
