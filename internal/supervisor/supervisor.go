package supervisor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

const (
	defaultHeartbeatInterval = 5 * time.Second
	defaultStaleThreshold    = 30 * time.Second
	defaultTaskPollInterval  = 100 * time.Millisecond
)

// Supervisor manages the agent pool: heartbeat monitoring and task assignment.
type Supervisor struct {
	machine *agent.Machine
	store   state.StateStore
	policy  *RestartPolicy

	heartbeatInterval time.Duration
	staleThreshold    time.Duration
	taskPollInterval  time.Duration

	mu            sync.RWMutex
	agentMu       map[string]*sync.Mutex // per-agentID mutex for Machine.ApplyEvent serialization
	agentCooldown map[string]time.Time   // per-agent cooldown expiry after crash
	circuitOpen   map[string]bool        // per-agent circuit breaker state
	stopped       bool
}

// SupervisorOption configures the Supervisor.
type SupervisorOption func(*Supervisor)

// WithHeartbeatInterval sets how often the heartbeat loop checks agent health.
func WithHeartbeatInterval(d time.Duration) SupervisorOption {
	return func(sv *Supervisor) { sv.heartbeatInterval = d }
}

// WithStaleThreshold sets the maximum age of a heartbeat before an agent is considered failed.
func WithStaleThreshold(d time.Duration) SupervisorOption {
	return func(sv *Supervisor) { sv.staleThreshold = d }
}

// WithTaskPollInterval sets how often the task assignment loop polls the queue.
func WithTaskPollInterval(d time.Duration) SupervisorOption {
	return func(sv *Supervisor) { sv.taskPollInterval = d }
}

// NewSupervisor returns a Supervisor wired to the given machine, store, and policy.
func NewSupervisor(machine *agent.Machine, store state.StateStore, policy *RestartPolicy, opts ...SupervisorOption) *Supervisor {
	sv := &Supervisor{
		machine:           machine,
		store:             store,
		policy:            policy,
		heartbeatInterval: defaultHeartbeatInterval,
		staleThreshold:    defaultStaleThreshold,
		taskPollInterval:  defaultTaskPollInterval,
		agentMu:           make(map[string]*sync.Mutex),
		agentCooldown:     make(map[string]time.Time),
		circuitOpen:       make(map[string]bool),
	}
	for _, opt := range opts {
		opt(sv)
	}
	return sv
}

// RegisterAgent adds an agent to the pool in idle state.
func (sv *Supervisor) RegisterAgent(ctx context.Context, agentID string) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	if sv.stopped {
		return ErrSupervisorStopped
	}

	now := time.Now().UnixMilli()
	if err := sv.store.SetAgentFields(ctx, agentID, state.AgentFields{
		State:         string(agent.StateIdle),
		LastHeartbeat: now,
		RegisteredAt:  now,
	}); err != nil {
		return fmt.Errorf("register agent %s: %w", agentID, err)
	}

	sv.agentMu[agentID] = &sync.Mutex{}
	return nil
}

// Stop marks the supervisor as stopped. Subsequent RegisterAgent calls return ErrSupervisorStopped.
func (sv *Supervisor) Stop() {
	sv.mu.Lock()
	sv.stopped = true
	sv.mu.Unlock()
}

// Run starts the heartbeat and task-assignment loops. Blocks until ctx is done.
func (sv *Supervisor) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		sv.heartbeatLoop(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sv.taskAssignLoop(ctx)
	}()

	wg.Wait()
	return nil
}

// heartbeatLoop periodically checks LastHeartbeat for each agent;
// if stale and not idle, applies EventAgentFailed.
func (sv *Supervisor) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(sv.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sv.checkHeartbeats(ctx)
		}
	}
}

func (sv *Supervisor) checkHeartbeats(ctx context.Context) {
	agents, err := sv.store.ListAgents(ctx)
	if err != nil {
		return
	}

	now := time.Now().UnixMilli()
	staleMS := sv.staleThreshold.Milliseconds()

	for _, agentID := range agents {
		fields, err := sv.store.GetAgentFields(ctx, agentID)
		if err != nil {
			continue
		}

		// Only mark as failed if the heartbeat is stale and agent is not idle.
		if fields.State == string(agent.StateIdle) {
			continue
		}
		if now-fields.LastHeartbeat <= staleMS {
			continue
		}

		sv.crashAgent(ctx, agentID)
	}
}

// crashAgent applies EventAgentFailed and records the crash with the restart policy.
// If the policy returns a backoff, the agent is placed in cooldown.
// If the circuit breaker opens, the agent is marked circuit-open.
//
// A cooldown sentinel is pre-populated before the state transition so that
// findIdleAgent cannot observe the agent as IDLE without a cooldown entry
// during the window between applyEvent returning and the final cooldown write.
func (sv *Supervisor) crashAgent(ctx context.Context, agentID string) {
	// Pre-populate cooldown sentinel so findIdleAgent skips this agent
	// during the window between applyEvent and the actual cooldown write.
	sv.mu.Lock()
	sv.agentCooldown[agentID] = time.Now().Add(24 * time.Hour)
	sv.mu.Unlock()

	snap, _ := sv.applyEvent(ctx, agentID, agent.EventAgentFailed)

	// TOCTOU guard: if the agent was already IDLE when applyEvent fired,
	// this is a spurious crash (the agent completed work concurrently).
	// Clean up the sentinel and return without recording a crash.
	if snap.PreviousState == agent.StateIdle {
		sv.mu.Lock()
		delete(sv.agentCooldown, agentID)
		sv.mu.Unlock()
		return
	}

	decision := sv.policy.RecordCrash(agentID)

	sv.mu.Lock()
	defer sv.mu.Unlock()

	if !decision.ShouldRestart {
		sv.circuitOpen[agentID] = true
		delete(sv.agentCooldown, agentID)
		return
	}
	if decision.Backoff > 0 {
		sv.agentCooldown[agentID] = time.Now().Add(decision.Backoff)
	} else {
		delete(sv.agentCooldown, agentID)
	}
}

// taskAssignLoop dequeues tasks and assigns them to idle agents.
func (sv *Supervisor) taskAssignLoop(ctx context.Context) {
	ticker := time.NewTicker(sv.taskPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sv.tryAssignTask(ctx)
		}
	}
}

func (sv *Supervisor) tryAssignTask(ctx context.Context) {
	taskID, err := sv.store.DequeueTask(ctx)
	if err != nil {
		// ErrQueueEmpty is expected — not a failure.
		return
	}

	agentID, found := sv.findIdleAgent(ctx)
	if !found {
		// Re-enqueue the task at default priority since no idle agent available.
		_ = sv.store.EnqueueTask(ctx, taskID, 0)
		return
	}

	snap, err := sv.applyEvent(ctx, agentID, agent.EventTaskAssigned)
	if err != nil {
		// Could not assign; re-enqueue.
		_ = sv.store.EnqueueTask(ctx, taskID, 0)
		return
	}

	_ = sv.store.PublishEvent(ctx, state.Event{
		Type:      string(agent.EventTaskAssigned),
		AgentID:   snap.AgentID,
		TaskID:    taskID,
		Timestamp: time.Now().UnixMilli(),
	})
}

// findIdleAgent returns the ID of any idle agent that is not in cooldown or
// circuit-open state, or ("", false) if none exists.
func (sv *Supervisor) findIdleAgent(ctx context.Context) (string, bool) {
	agents, err := sv.store.ListAgents(ctx)
	if err != nil {
		return "", false
	}

	sv.mu.RLock()
	defer sv.mu.RUnlock()

	now := time.Now()
	for _, agentID := range agents {
		stateStr, err := sv.store.GetAgentState(ctx, agentID)
		if err != nil {
			continue
		}
		if stateStr != string(agent.StateIdle) {
			continue
		}
		if sv.circuitOpen[agentID] {
			continue
		}
		if exp, ok := sv.agentCooldown[agentID]; ok && now.Before(exp) {
			continue
		}
		return agentID, true
	}
	return "", false
}

// completeAgent applies EventOutputDelivered and records a success with the
// restart policy, resetting consecutive crash counters and clearing any
// cooldown or circuit-breaker state for the agent.
func (sv *Supervisor) completeAgent(ctx context.Context, agentID string) {
	_, err := sv.applyEvent(ctx, agentID, agent.EventOutputDelivered)
	if err != nil {
		return
	}

	sv.policy.RecordSuccess(agentID)

	sv.mu.Lock()
	delete(sv.agentCooldown, agentID)
	delete(sv.circuitOpen, agentID)
	sv.mu.Unlock()
}

// applyEvent serializes Machine.ApplyEvent per agentID using per-agent mutex.
func (sv *Supervisor) applyEvent(ctx context.Context, agentID string, event agent.AgentEvent) (agent.AgentSnapshot, error) {
	mu := sv.getAgentMutex(agentID)
	if mu == nil {
		return agent.AgentSnapshot{}, fmt.Errorf("apply event for %s: %w", agentID, ErrSupervisorStopped)
	}

	mu.Lock()
	defer mu.Unlock()

	return sv.machine.ApplyEvent(ctx, agentID, event)
}

// getAgentMutex returns the per-agent mutex, or nil if not registered.
func (sv *Supervisor) getAgentMutex(agentID string) *sync.Mutex {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.agentMu[agentID]
}
