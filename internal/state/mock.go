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
	mu                    sync.RWMutex
	agents                map[string]*agentRecord // agentID -> record
	events                []Event
	queue                 []queueItem // sorted by priority ascending
	pingErr               error
	getAllAgentStatesErr  error
	queueLenErr           error
	getAgentFieldsErr     error            // global fallback
	setAgentFieldsErr     error            // global fallback
	getAgentFieldsByAgent map[string]error // per-agent overrides
	setAgentFieldsByAgent map[string]error // per-agent overrides
	enqueueTaskErr        error
	dequeueTaskErr        error
	clearCurrentTaskErr   error
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
		agents:                make(map[string]*agentRecord),
		getAgentFieldsByAgent: make(map[string]error),
		setAgentFieldsByAgent: make(map[string]error),
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

func (m *MockStore) CompareAndSetAgentState(ctx context.Context, agentID string, expected, next string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.agents[agentID]
	if !ok {
		return ErrAgentNotFound
	}
	if rec.fields.State != expected {
		return &StateConflictError{Expected: expected, Actual: rec.fields.State}
	}
	updated := rec.fields
	updated.State = next
	rec.fields = updated
	return nil
}

// ── Agent full record ─────────────────────────────────────────────────────────

// SetSetAgentFieldsError configures SetAgentFields to return err.
// Pass nil to reset to healthy. If agentIDs are provided, the error applies
// only to those agents; otherwise it sets the global fallback.
func (m *MockStore) SetSetAgentFieldsError(err error, agentIDs ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(agentIDs) == 0 {
		m.setAgentFieldsErr = err
	} else {
		for _, id := range agentIDs {
			if err == nil {
				delete(m.setAgentFieldsByAgent, id)
			} else {
				m.setAgentFieldsByAgent[id] = err
			}
		}
	}
}

func (m *MockStore) SetAgentFields(ctx context.Context, agentID string, fields AgentFields) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e, ok := m.setAgentFieldsByAgent[agentID]; ok {
		return e
	}
	if m.setAgentFieldsErr != nil {
		return m.setAgentFieldsErr
	}
	m.agents[agentID] = &agentRecord{fields: fields}
	return nil
}

// SetGetAgentFieldsError configures GetAgentFields to return err.
// Pass nil to reset to healthy. If agentIDs are provided, the error applies
// only to those agents; otherwise it sets the global fallback.
func (m *MockStore) SetGetAgentFieldsError(err error, agentIDs ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(agentIDs) == 0 {
		m.getAgentFieldsErr = err
	} else {
		for _, id := range agentIDs {
			if err == nil {
				delete(m.getAgentFieldsByAgent, id)
			} else {
				m.getAgentFieldsByAgent[id] = err
			}
		}
	}
}

func (m *MockStore) GetAgentFields(ctx context.Context, agentID string) (AgentFields, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if e, ok := m.getAgentFieldsByAgent[agentID]; ok {
		return AgentFields{}, e
	}
	if m.getAgentFieldsErr != nil {
		return AgentFields{}, m.getAgentFieldsErr
	}
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

// SetGetAllAgentStatesError configures GetAllAgentStates to return err.
// Pass nil to reset to healthy.
func (m *MockStore) SetGetAllAgentStatesError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getAllAgentStatesErr = err
}

func (m *MockStore) GetAllAgentStates(ctx context.Context) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getAllAgentStatesErr != nil {
		return nil, m.getAllAgentStatesErr
	}
	states := make(map[string]string, len(m.agents))
	for id, rec := range m.agents {
		states[id] = rec.fields.State
	}
	return states, nil
}

// ── Agent task binding ────────────────────────────────────────────────────────

// SetClearCurrentTaskError configures ClearCurrentTask to return err.
// Pass nil to reset to healthy.
func (m *MockStore) SetClearCurrentTaskError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearCurrentTaskErr = err
}

func (m *MockStore) ClearCurrentTask(ctx context.Context, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clearCurrentTaskErr != nil {
		return m.clearCurrentTaskErr
	}
	rec, ok := m.agents[agentID]
	if !ok {
		return ErrAgentNotFound
	}
	updated := rec.fields
	updated.CurrentTaskID = ""
	updated.CurrentTaskPriority = 0
	rec.fields = updated
	return nil
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

// SetEnqueueTaskError configures EnqueueTask to return err.
// Pass nil to reset to healthy.
func (m *MockStore) SetEnqueueTaskError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueueTaskErr = err
}

func (m *MockStore) EnqueueTask(ctx context.Context, taskID string, priority float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.enqueueTaskErr != nil {
		return m.enqueueTaskErr
	}
	m.queue = append(m.queue, queueItem{taskID: taskID, priority: priority})
	sort.Slice(m.queue, func(i, j int) bool {
		return m.queue[i].priority < m.queue[j].priority
	})
	return nil
}

// SetDequeueTaskError configures DequeueTask to return err (instead of
// the normal queue behaviour). Pass nil to reset to healthy.
func (m *MockStore) SetDequeueTaskError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dequeueTaskErr = err
}

func (m *MockStore) DequeueTask(ctx context.Context) (string, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.dequeueTaskErr != nil {
		return "", 0, m.dequeueTaskErr
	}
	if len(m.queue) == 0 {
		return "", 0, ErrQueueEmpty
	}
	item := m.queue[0]
	// Build a new slice rather than modifying in place (immutable pattern).
	m.queue = append([]queueItem{}, m.queue[1:]...)
	return item.taskID, item.priority, nil
}

// SetQueueLengthError configures QueueLength to return err.
// Pass nil to reset to healthy.
func (m *MockStore) SetQueueLengthError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queueLenErr = err
}

func (m *MockStore) QueueLength(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.queueLenErr != nil {
		return 0, m.queueLenErr
	}
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
