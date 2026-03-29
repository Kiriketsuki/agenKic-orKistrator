package httpbridge

// ── Request types ────────────────────────────────────────────────────────────

// SubmitTaskRequest is the JSON body for POST /api/tasks.
type SubmitTaskRequest struct {
	TaskID   string  `json:"task_id"`
	Priority float64 `json:"priority"`
}

// SubmitDAGRequest is the JSON body for POST /api/dags.
type SubmitDAGRequest struct {
	Nodes []DAGNodeJSON `json:"nodes"`
	Edges []DAGEdgeJSON `json:"edges"`
}

// DAGNodeJSON is a task node within a DAG submission.
type DAGNodeJSON struct {
	NodeID   string  `json:"node_id"`
	TaskID   string  `json:"task_id"`
	Priority float64 `json:"priority"`
}

// DAGEdgeJSON is a dependency edge between two DAG nodes.
type DAGEdgeJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// SendInputRequest is the JSON body for POST /api/agents/{id}/input.
type SendInputRequest struct {
	Keys string `json:"keys"`
}

// ── Response types ───────────────────────────────────────────────────────────

// AgentJSON is the JSON representation of an agent.
type AgentJSON struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	CurrentTaskID string `json:"current_task_id,omitempty"`
	LastHeartbeat int64  `json:"last_heartbeat"`
	RegisteredAt  int64  `json:"registered_at"`
}

// FloorJSON represents a terminal session (floor) with its metadata.
type FloorJSON struct {
	Name       string `json:"name"`
	AgentCount int    `json:"agent_count"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// ── SSE event payload types ──────────────────────────────────────────────────

// SSEAgentRegistered is the payload for agent.registered events.
type SSEAgentRegistered struct {
	AgentID   string `json:"agent_id"`
	Timestamp int64  `json:"timestamp"`
}

// SSEAgentStateChanged is the payload for agent.state_changed events.
type SSEAgentStateChanged struct {
	AgentID   string `json:"agent_id"`
	State     string `json:"state"`
	TaskID    string `json:"task_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// SSEAgentOutput is the payload for agent.output events.
type SSEAgentOutput struct {
	AgentID   string `json:"agent_id"`
	Payload   string `json:"payload"`
	Timestamp int64  `json:"timestamp"`
}
