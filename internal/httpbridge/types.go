package httpbridge

// ── Request types ────────────────────────────────────────────────────────────

// SubmitTaskRequest is the JSON body for POST /api/tasks.
//
// TaskID is optional — when omitted the server generates one (UUID) and
// returns it in the response. Description is required when TaskID is also
// omitted (a fully-empty body is rejected). Project and Floor are optional
// quest-board metadata (#118) persisted alongside the task but not yet
// consumed by the supervisor assign loop.
type SubmitTaskRequest struct {
	TaskID      string  `json:"task_id"`
	Priority    float64 `json:"priority"`
	Description string  `json:"description"`
	Project     string  `json:"project,omitempty"`
	Floor       string  `json:"floor,omitempty"`
}

// SubmitDAGRequest is the JSON body for POST /api/dags.
type SubmitDAGRequest struct {
	Nodes []DAGNodeJSON `json:"nodes"`
	Edges []DAGEdgeJSON `json:"edges"`
}

// DAGNodeJSON is a task node within a DAG submission. Description is
// optional and maps to the underlying pb.TaskSpec.Prompt (#118); TaskID is
// generated server-side when omitted, mirroring SubmitTaskRequest.
type DAGNodeJSON struct {
	NodeID      string  `json:"node_id"`
	TaskID      string  `json:"task_id"`
	Priority    float64 `json:"priority"`
	Description string  `json:"description,omitempty"`
}

// DAGEdgeJSON is a dependency edge between two DAG nodes.
type DAGEdgeJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// SendInputRequest is the JSON body for POST /api/agents/{id}/input.
//
// The body carries either text or one key press, never both. Keys and Input
// name the same text field, so an older client that sends "keys" still works.
// Key names a single tmux key such as Up or Enter, and the handler forwards it
// through Substrate.SendKey so a curses prompt reads a real key press.
type SendInputRequest struct {
	Keys  string `json:"keys,omitempty"`
	Input string `json:"input,omitempty"`
	Key   string `json:"key,omitempty"`
}

// Text returns the literal text the request carries, from either field name.
func (r SendInputRequest) Text() string {
	if r.Input != "" {
		return r.Input
	}
	return r.Keys
}

// ReassignAgentRequest is the JSON body for POST /api/agents/{id}/reassign
// (T14 / #119). At least one of Tier or Provider must be set.
//
// Semantics: the Bridge has no Supervisor or agent.Machine reference and no
// provider/tier field anywhere in state.AgentFields, so "reassignment" here
// is NOT live migration of a running agent to a different provider. It is
// implemented as: detach the task from the current agent, then re-enqueue it
// with Tier/Provider recorded as a hint on state.TaskMeta. The supervisor's
// assign loop still dequeues strictly by taskID+priority and hands the task
// to whichever agent becomes free next — it does not read or honor this
// hint (matching the pre-existing, documented behavior of TaskMeta.Project
// and TaskMeta.Floor). See handleReassignAgent for the full precondition and
// state-transition contract.
type ReassignAgentRequest struct {
	Tier     string `json:"tier"`
	Provider string `json:"provider"`
}

// ── Response types ───────────────────────────────────────────────────────────

// AgentJSON is the JSON representation of an agent.
// Name is the fantasy display name chosen by POST /api/agents/spawn. It is
// empty for an agent that registered straight through gRPC, and every UI
// consumer falls back to ID in that case. See Bridge.setAgentName.
type AgentJSON struct {
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	Provider      string `json:"provider,omitempty"`
	State         string `json:"state"`
	CurrentTaskID string `json:"current_task_id,omitempty"`
	LastHeartbeat int64  `json:"last_heartbeat"`
	RegisteredAt  int64  `json:"registered_at"`
	Floor         int    `json:"floor,omitempty"`
}

// ProviderJSON describes one spawn-kind sigil for the Grimoire flyout.
type ProviderJSON struct {
	Kind    string `json:"kind"`
	Display string `json:"display"`
	Accent  string `json:"accent"`
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
// Carries the full agent state so Godot can construct AgentData directly.
type SSEAgentRegistered struct {
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	Provider      string `json:"provider,omitempty"`
	State         string `json:"state"`
	CurrentTaskID string `json:"current_task_id,omitempty"`
	LastHeartbeat int64  `json:"last_heartbeat"`
	RegisteredAt  int64  `json:"registered_at"`
	Cursor        string `json:"cursor,omitempty"`
}

// SSEAgentStateChanged is the payload for agent.state_changed events.
type SSEAgentStateChanged struct {
	AgentID   string `json:"agent_id"`
	Name      string `json:"name,omitempty"`
	State     string `json:"state"`
	TaskID    string `json:"task_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
	Cursor    string `json:"cursor,omitempty"`
}

// SSEAgentOutput is the payload for agent.output events.
type SSEAgentOutput struct {
	AgentID   string `json:"agent_id"`
	Payload   string `json:"payload"`
	Timestamp int64  `json:"timestamp"`
	Cursor    string `json:"cursor,omitempty"`
}

// SSEFloorCreated is the payload for floor.created events.
type SSEFloorCreated struct {
	Name       string `json:"name"`
	AgentCount int    `json:"agent_count"`
	Cursor     string `json:"cursor,omitempty"`
}

// SSEFloorRemoved is the payload for floor.removed events.
type SSEFloorRemoved struct {
	Name   string `json:"name"`
	Cursor string `json:"cursor,omitempty"`
}

// SpawnAgentRequest is the JSON body for POST /api/agents/spawn. All fields
// are optional — the server picks a fantasy name and rotates tiers.
type SpawnAgentRequest struct {
	// Kind selects the worker implementation: "sim" (default), "claude",
	// "codex", "opencode", or "pi".
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
	Tier string `json:"tier,omitempty"`
	// Floor requests placement on a specific tower floor. A nil Floor keeps
	// today's auto-assignment behavior and records no floor. Floor 0
	// (ground) is reserved for the future archmage and always rejects with
	// 400, whether it arrives as an explicit request or a full floor.
	Floor *int `json:"floor,omitempty"`
}

// SpawnAgentResponse echoes what was actually spawned.
type SpawnAgentResponse struct {
	AgentID string `json:"agent_id"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Tier    string `json:"tier"`
	Floor   int    `json:"floor"`
}
