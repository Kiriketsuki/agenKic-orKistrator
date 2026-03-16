package state

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MockStore is a thread-safe, in-memory StateStore implementation for use in
// unit tests. It has no external dependencies.
type MockStore struct {
	mu      sync.RWMutex
	agents  map[string]*agentRecord // agentID -> record
	events  []Event
	queue   []queueItem // sorted by priority ascending
	pingErr error
}

type agentRecord struct {
	fields AgentFields
}

type queueItem struct {
	taskID   string
	priority float64
}

// NewMockStore returns a ready-to-use in-memory store.
func NewMockStore() *MockStore {
	return &MockStore{
		agents: make(map[string]*agentRecord),
	}
}

// ── Agent state ───────────────────────────────────────────────────────────────

func (m *MockStore) SetAgentState(ctx context.Context, agentID string, state string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.agents[agentID]
	if !ok {
		rec = &agentRecord{}
		m.agents[agentID] = rec
	}
	// Return a new AgentFields with only State updated; other fields preserved.
	updated := rec.fields
	updated.State = state
	rec.fields = updated
	return nil
}

func (m *MockStore) GetAgentState(ctx context.Context, agentID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.agents[agentID]
	if !ok {
		return "", ErrAgentNotFound
	}
	return rec.fields.State, nil
}

// ── Agent full record ─────────────────────────────────────────────────────────

func (m *MockStore) SetAgentFields(ctx context.Context, agentID string, fields AgentFields) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.agents[agentID] = &agentRecord{fields: fields}
	return nil
}

func (m *MockStore) GetAgentFields(ctx context.Context, agentID string) (AgentFields, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.agents[agentID]
	if !ok {
		return AgentFields{}, ErrAgentNotFound
	}
	// Return a copy; AgentFields is a value type (immutable pattern).
	return rec.fields, nil
}

func (m *MockStore) DeleteAgent(ctx context.Context, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.agents, agentID)
	return nil
}

func (m *MockStore) ListAgents(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.agents))
	for id := range m.agents {
		ids = append(ids, id)
	}
	return ids, nil
}

// ── Event stream ──────────────────────────────────────────────────────────────

func (m *MockStore) PublishEvent(ctx context.Context, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	m.events = append(m.events, event)
	return nil
}

// ── Task queue ────────────────────────────────────────────────────────────────

func (m *MockStore) EnqueueTask(ctx context.Context, taskID string, priority float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.queue = append(m.queue, queueItem{taskID: taskID, priority: priority})
	sort.Slice(m.queue, func(i, j int) bool {
		return m.queue[i].priority < m.queue[j].priority
	})
	return nil
}

func (m *MockStore) DequeueTask(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.queue) == 0 {
		return "", ErrQueueEmpty
	}
	item := m.queue[0]
	// Build a new slice rather than modifying in place (immutable pattern).
	m.queue = append([]queueItem{}, m.queue[1:]...)
	return item.taskID, nil
}

func (m *MockStore) QueueLength(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return int64(len(m.queue)), nil
}

// ── Lifecycle ─────────────────────────────────────────────────────────────────

// SetPingError configures Ping to return err. Pass nil to reset to healthy.
func (m *MockStore) SetPingError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pingErr = err
}

func (m *MockStore) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pingErr
}

func (m *MockStore) Close() error { return nil }
