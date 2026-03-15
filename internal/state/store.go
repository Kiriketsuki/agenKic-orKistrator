package state

import "context"

// AgentFields holds the full set of mutable agent metadata stored in Redis.
type AgentFields struct {
	State         string
	LastHeartbeat int64  // unix millis
	CurrentTaskID string
	RegisteredAt  int64  // unix millis
}

// Event represents an entry published to the event stream.
type Event struct {
	Type    string
	AgentID string
	TaskID  string
	// Timestamp is unix millis; set by the implementation if zero.
	Timestamp int64
	Payload   string
}

// StateStore is the single persistence abstraction for agent state, the event
// stream, and the task queue. Both Redis and in-memory mock implementations
// must satisfy this interface.
//
// Method contracts:
//   - GetAgentState returns ErrAgentNotFound when the agent does not exist.
//   - GetAgentFields returns ErrAgentNotFound when the agent does not exist.
//   - DequeueTask returns ErrQueueEmpty when the queue is empty.
//   - All methods must be safe for concurrent use.
//   - SetAgentState creates or updates only the state field; numeric fields
//     (LastHeartbeat, RegisteredAt) default to zero if not previously set via
//     SetAgentFields. GetAgentFields is safe to call after SetAgentState alone.
type StateStore interface {
	// ── Agent state ──────────────────────────────────────────────────────────
	SetAgentState(ctx context.Context, agentID string, state string) error
	GetAgentState(ctx context.Context, agentID string) (string, error)

	// ── Agent full record ────────────────────────────────────────────────────
	SetAgentFields(ctx context.Context, agentID string, fields AgentFields) error
	GetAgentFields(ctx context.Context, agentID string) (AgentFields, error)
	DeleteAgent(ctx context.Context, agentID string) error
	ListAgents(ctx context.Context) ([]string, error)

	// ── Event stream ─────────────────────────────────────────────────────────
	PublishEvent(ctx context.Context, event Event) error

	// ── Task queue (Sorted Set / priority queue) ──────────────────────────────
	EnqueueTask(ctx context.Context, taskID string, priority float64) error
	DequeueTask(ctx context.Context) (string, error)
	QueueLength(ctx context.Context) (int64, error)

	// ── Lifecycle ────────────────────────────────────────────────────────────
	Ping(ctx context.Context) error
	Close() error
}
