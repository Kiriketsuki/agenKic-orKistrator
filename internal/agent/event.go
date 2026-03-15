package agent

// AgentEvent is a domain event that drives state transitions in the machine.
type AgentEvent string

const (
	EventTaskAssigned    AgentEvent = "task_assigned"
	EventWorkStarted     AgentEvent = "work_started"
	EventOutputReady     AgentEvent = "output_ready"
	EventOutputDelivered AgentEvent = "output_delivered"
	EventAgentFailed     AgentEvent = "agent_failed"
)
