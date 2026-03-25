package state

import (
	"context"
	"fmt"
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
	streamEvents          []mockStreamEvent
	streamSeq             int64
	groups                map[string]*mockGroup
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
	compareAndSetErr      error
}

type agentRecord struct {
	fields AgentFields
}

type queueItem struct {
	taskID   string
	priority float64
}

type mockStreamEvent struct {
	ID    string
	Event Event
}

type mockGroup struct {
	startIdx      int // index into streamEvents where this group starts consuming
	lastDelivered int // index of last delivered event (for round-robin)
	consumers     map[string]*mockConsumer
}

type mockConsumer struct {
	pending map[string]bool // set of unacked stream event IDs
}

// NewMockStore returns a ready-to-use in-memory store.
func NewMockStore() *MockStore {
	return &MockStore{
		agents:                make(map[string]*agentRecord),
		groups:                make(map[string]*mockGroup),
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

// SetCompareAndSetAgentStateError configures CompareAndSetAgentState to return err.
// Pass nil to reset to healthy.
func (m *MockStore) SetCompareAndSetAgentStateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compareAndSetErr = err
}

func (m *MockStore) CompareAndSetAgentState(ctx context.Context, agentID string, expected, next string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.compareAndSetErr != nil {
		return m.compareAndSetErr
	}
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
	m.streamSeq++
	id := fmt.Sprintf("mock-%d", m.streamSeq)
	m.streamEvents = append(m.streamEvents, mockStreamEvent{ID: id, Event: event})
	return nil
}

// ReadEvents returns up to count StreamEvents published after lastID.
// Note: unlike Redis XREAD which treats lastID as a lexicographic lower bound,
// the mock requires lastID to exactly match an existing entry's ID. Cursor IDs
// from a different mock instance or in Redis format (e.g. "1711234567890-0")
// will return empty. This is acceptable for unit tests where cursors always
// come from prior ReadEvents/SubscribeEvents calls in the same test run.
func (m *MockStore) ReadEvents(ctx context.Context, lastID string, count int64) ([]StreamEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]StreamEvent, 0)
	if lastID == "0" || lastID == "0-0" {
		for i, se := range m.streamEvents {
			if int64(i) >= count {
				break
			}
			result = append(result, StreamEvent{ID: se.ID, Event: se.Event})
		}
		return result, nil
	}

	// Find the entry matching lastID and return everything after it.
	startIdx := -1
	for i, se := range m.streamEvents {
		if se.ID == lastID {
			startIdx = i + 1
			break
		}
	}
	if startIdx < 0 {
		return result, nil
	}
	for i := startIdx; i < len(m.streamEvents) && int64(i-startIdx) < count; i++ {
		result = append(result, StreamEvent{ID: m.streamEvents[i].ID, Event: m.streamEvents[i].Event})
	}
	return result, nil
}

func (m *MockStore) CreateConsumerGroup(ctx context.Context, group string, startID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.groups[group]; ok {
		return nil
	}
	startIdx := 0
	if startID == "$" {
		startIdx = len(m.streamEvents)
	}
	m.groups[group] = &mockGroup{
		startIdx:      startIdx,
		lastDelivered: startIdx,
		consumers:     make(map[string]*mockConsumer),
	}
	return nil
}

func (m *MockStore) subscribeEventsLocked(g *mockGroup, c *mockConsumer, count int64) []StreamEvent {
	result := make([]StreamEvent, 0)
	delivered := 0
	for g.lastDelivered < len(m.streamEvents) && int64(delivered) < count {
		se := m.streamEvents[g.lastDelivered]
		result = append(result, StreamEvent{ID: se.ID, Event: se.Event})
		c.pending[se.ID] = true
		g.lastDelivered++
		delivered++
	}
	return result
}

func (m *MockStore) SubscribeEvents(ctx context.Context, group, consumer string, count int64, block time.Duration) ([]StreamEvent, error) {
	m.mu.Lock()

	g, ok := m.groups[group]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("SubscribeEvents: group %q not found", group)
	}
	c, ok := g.consumers[consumer]
	if !ok {
		c = &mockConsumer{pending: make(map[string]bool)}
		g.consumers[consumer] = c
	}

	result := m.subscribeEventsLocked(g, c, count)
	if len(result) > 0 || block <= 0 {
		m.mu.Unlock()
		return result, nil
	}
	m.mu.Unlock()

	// Honor the block parameter: wait up to block duration for new events,
	// matching Redis XREADGROUP behavior. Poll in short intervals so we
	// respond promptly to context cancellation and new events.
	deadline := time.After(block)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return []StreamEvent{}, ctx.Err()
		case <-deadline:
			return []StreamEvent{}, nil
		case <-ticker.C:
			m.mu.Lock()
			result = m.subscribeEventsLocked(g, c, count)
			m.mu.Unlock()
			if len(result) > 0 {
				return result, nil
			}
		}
	}
}

func (m *MockStore) AckEvent(ctx context.Context, group string, ids ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	g, ok := m.groups[group]
	if !ok {
		return fmt.Errorf("AckEvent: group %q not found", group)
	}
	for _, c := range g.consumers {
		for _, id := range ids {
			delete(c.pending, id)
		}
	}
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
