package state

import (
	"context"
	"time"
)

// AgentState* are the canonical lifecycle state strings stored in Redis.
// Both the state package and any consumer that inspects state strings must use
// these constants so a value change here is caught at compile time everywhere.
const (
	AgentStateIdle      = "idle"
	AgentStateAssigned  = "assigned"
	AgentStateWorking   = "working"
	AgentStateReporting = "reporting"
)

// AgentFields holds the full set of mutable agent metadata stored in Redis.
type AgentFields struct {
	State               string
	LastHeartbeat       int64 // unix millis
	CurrentTaskID       string
	CurrentTaskPriority float64 // original priority for crash re-enqueue
	RegisteredAt        int64   // unix millis
}

// StreamEvent wraps an Event with its Redis-assigned stream entry ID.
type StreamEvent struct {
	ID    string // Redis stream entry ID (e.g., "1234567890-0")
	Event Event
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
//
// State-write safety:
//   - SetAgentState: registration and seeding ONLY. Not safe for in-flight
//     state transitions — bypasses optimistic locking. Use
//     CompareAndSetAgentState for all transitions on agents already
//     participating in the lifecycle.
//   - CompareAndSetAgentState: the ONLY safe method for in-flight state
//     transitions. Provides atomic compare-and-swap; returns
//     *StateConflictError on concurrent modification.
//   - SetAgentFields: field-only updates (LastHeartbeat, CurrentTaskID, etc.).
//     Does not participate in CAS — callers must serialize field writes that
//     must be consistent with a preceding CAS transition (e.g., writing
//     CurrentTaskID immediately after transitioning state to assigned).
type StateStore interface {
	// ── Agent state ──────────────────────────────────────────────────────────
	SetAgentState(ctx context.Context, agentID string, state string) error
	GetAgentState(ctx context.Context, agentID string) (string, error)
	// CompareAndSetAgentState atomically sets the agent's state to next only
	// if the current state equals expected. Returns *StateConflictError if the
	// current state does not match expected, or ErrAgentNotFound if the agent
	// does not exist.
	CompareAndSetAgentState(ctx context.Context, agentID string, expected, next string) error

	// ── Agent full record ────────────────────────────────────────────────────
	SetAgentFields(ctx context.Context, agentID string, fields AgentFields) error
	GetAgentFields(ctx context.Context, agentID string) (AgentFields, error)
	DeleteAgent(ctx context.Context, agentID string) error
	ListAgents(ctx context.Context) ([]string, error)
	// GetAllAgentStates returns a map of agentID → state for all registered
	// agents in a single batch operation, reducing per-probe round trips.
	GetAllAgentStates(ctx context.Context) (map[string]string, error)

	// ── Event stream ─────────────────────────────────────────────────────────
	PublishEvent(ctx context.Context, event Event) error
	// ReadEvents returns up to count StreamEvents published after lastID.
	// Use "0" to read from the beginning. Returns an empty slice when no
	// new events exist.
	ReadEvents(ctx context.Context, lastID string, count int64) ([]StreamEvent, error)
	// CreateConsumerGroup creates a consumer group on the event stream.
	// Idempotent — returns nil if the group already exists.
	CreateConsumerGroup(ctx context.Context, group string, startID string) error
	// SubscribeEvents reads events via a consumer group (XREADGROUP).
	// Returns events assigned to the named consumer; competing consumers
	// in the same group each receive distinct events.
	SubscribeEvents(ctx context.Context, group, consumer string, count int64, block time.Duration) ([]StreamEvent, error)
	// AckEvent acknowledges processed events so they are not re-delivered.
	AckEvent(ctx context.Context, group string, ids ...string) error

	// ── Agent task binding ──────────────────────────────────────────────────
	// ClearCurrentTask atomically zeroes CurrentTaskID and CurrentTaskPriority
	// for the given agent. Returns ErrAgentNotFound if the agent does not exist.
	ClearCurrentTask(ctx context.Context, agentID string) error

	// ── Task queue (Sorted Set / priority queue) ──────────────────────────────
	EnqueueTask(ctx context.Context, taskID string, priority float64) error
	DequeueTask(ctx context.Context) (string, float64, error)
	QueueLength(ctx context.Context) (int64, error)

	// ── Lifecycle ────────────────────────────────────────────────────────────
	Ping(ctx context.Context) error
	Close() error
}
