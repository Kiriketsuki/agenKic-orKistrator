package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

const (
	defaultHeartbeatInterval = 5 * time.Second
	defaultStaleThreshold    = 30 * time.Second
	defaultTaskPollInterval  = 100 * time.Millisecond
)

// Supervisor manages the agent pool: heartbeat monitoring and task assignment.
type Supervisor struct {
	machine            *agent.Machine
	store              state.StateStore
	policy             *RestartPolicy
	substrate          terminal.Substrate  // optional; nil disables terminal management
	completionRegistry *CompletionRegistry // optional; nil disables completion signalling

	heartbeatInterval time.Duration
	staleThreshold    time.Duration
	taskPollInterval  time.Duration

	mu            sync.RWMutex
	agentMu       map[string]*sync.Mutex // per-agentID mutex for Machine.ApplyEvent serialization
	agentCooldown map[string]time.Time   // per-agent cooldown expiry after crash
	circuitOpen   map[string]bool        // per-agent circuit breaker state
	stopped       bool

	// Assign-loop backoff: exponential backoff on consecutive store errors
	// in tryAssignTask. Only accessed from the taskAssignLoop goroutine.
	consecutiveAssignErrors int
	nextAssignAttempt       time.Time
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

// WithSubstrate wires a terminal.Substrate into the supervisor.
// When set, the supervisor spawns a tmux session per agent on RegisterAgent
// and destroys it on agent crash. A nil substrate disables terminal management.
func WithSubstrate(s terminal.Substrate) SupervisorOption {
	return func(sv *Supervisor) { sv.substrate = s }
}

// WithCompletionRegistry wires a CompletionRegistry into the supervisor.
// When set, completeAgent signals the registry before clearing the task,
// allowing BlockingSubmitter callers to unblock when a task finishes.
func WithCompletionRegistry(r *CompletionRegistry) SupervisorOption {
	return func(sv *Supervisor) { sv.completionRegistry = r }
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
// Store I/O is performed outside sv.mu to avoid holding the lock during
// network round-trips (consistent with findIdleAgent's snapshot pattern).
func (sv *Supervisor) RegisterAgent(ctx context.Context, agentID string) error {
	if agentID == "" || len(agentID) > 128 {
		return ErrInvalidAgentID
	}

	sv.mu.RLock()
	if sv.stopped {
		sv.mu.RUnlock()
		return ErrSupervisorStopped
	}
	sv.mu.RUnlock()

	now := time.Now().UnixMilli()
	if err := sv.store.SetAgentFields(ctx, agentID, state.AgentFields{
		State:         string(agent.StateIdle),
		LastHeartbeat: now,
		RegisteredAt:  now,
	}); err != nil {
		return fmt.Errorf("register agent %s: %w", agentID, err)
	}

	sv.mu.Lock()
	if sv.stopped {
		sv.mu.Unlock()
		return ErrSupervisorStopped
	}
	sv.agentMu[agentID] = &sync.Mutex{}
	sv.mu.Unlock()

	if sv.substrate != nil {
		sessionName := "agent-" + agentID
		if _, err := sv.substrate.SpawnSession(ctx, sessionName, ""); err != nil {
			log.Printf("supervisor: substrate: SpawnSession %q failed (agent %s continues): %v", sessionName, agentID, err)
		}
	}

	return nil
}

// Stop marks the supervisor as stopped and destroys all spawned tmux sessions.
// Callers must ensure no concurrent RegisterAgent calls are in-flight before
// calling Stop (e.g. by draining the gRPC server with GracefulStop first).
// Subsequent RegisterAgent calls return ErrSupervisorStopped.
func (sv *Supervisor) Stop() {
	sv.mu.Lock()
	sv.stopped = true
	agentIDs := make([]string, 0, len(sv.agentMu))
	for id := range sv.agentMu {
		agentIDs = append(agentIDs, id)
	}
	sv.mu.Unlock()

	if sv.substrate != nil {
		for _, id := range agentIDs {
			sessionName := "agent-" + id
			if err := sv.substrate.DestroySession(context.Background(), sessionName); err != nil {
				log.Printf("supervisor: Stop: DestroySession %q failed (agent %s): %v", sessionName, id, err)
			}
		}
	}
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
		log.Printf("supervisor: checkHeartbeats — ListAgents failed: %v", err)
		return
	}

	now := time.Now().UnixMilli()
	staleMS := sv.staleThreshold.Milliseconds()

	for _, agentID := range agents {
		fields, err := sv.store.GetAgentFields(ctx, agentID)
		if err != nil {
			log.Printf("supervisor: checkHeartbeats — GetAgentFields %s failed: %v", agentID, err)
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
// The per-agent mutex is held for the entire operation to prevent interleaving
// with completeAgent (which deletes cooldown entries). This serializes all
// state-transition + policy operations for a given agent.
//
// A cooldown sentinel is pre-populated before the state transition so that
// findIdleAgent cannot observe the agent as IDLE without a cooldown entry.
func (sv *Supervisor) crashAgent(ctx context.Context, agentID string) {
	mu := sv.getAgentMutex(agentID)
	if mu == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	// Pre-read task binding before transitioning — if store is degraded,
	// skip the crash so the heartbeat loop can retry next tick.
	// Per-agent mutex is held: no concurrent modification of CurrentTaskID
	// is possible (tryAssignTask, completeAgent, Heartbeat, StartWork, and
	// ReportOutput all acquire the same per-agent mutex).
	preFields, preErr := sv.store.GetAgentFields(ctx, agentID)
	if preErr != nil {
		log.Printf("supervisor: crashAgent %s — GetAgentFields failed, deferring crash: %v", agentID, preErr)
		return
	}

	// Pre-populate cooldown sentinel so findIdleAgent skips this agent.
	sv.mu.Lock()
	sv.agentCooldown[agentID] = time.Now().Add(24 * time.Hour)
	sv.mu.Unlock()

	snap, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventAgentFailed)
	if err != nil {
		log.Printf("supervisor: crashAgent %s — ApplyEvent failed, deferring crash: %v", agentID, err)
		sv.mu.Lock()
		delete(sv.agentCooldown, agentID)
		sv.mu.Unlock()
		return
	}

	// TOCTOU guard: if the agent was already IDLE when applyEvent fired,
	// this is a spurious crash (the agent completed work concurrently).
	// Clean up the sentinel and return without recording a crash.
	if snap.PreviousState == agent.StateIdle {
		sv.mu.Lock()
		delete(sv.agentCooldown, agentID)
		sv.mu.Unlock()
		return
	}

	// Re-enqueue the agent's assigned task using pre-read CurrentTaskID.
	// SetAgentState (inside ApplyEvent) does not modify CurrentTaskID,
	// so the pre-read value is still valid after the transition.
	if preFields.CurrentTaskID != "" {
		if err := sv.store.EnqueueTask(ctx, preFields.CurrentTaskID, preFields.CurrentTaskPriority); err != nil {
			log.Printf("supervisor: task %s lost — re-enqueue failed (agent %s crashed): %v", preFields.CurrentTaskID, agentID, err)
		}
	}

	decision := sv.policy.RecordCrash(agentID)

	sv.mu.Lock()
	if !decision.ShouldRestart {
		sv.circuitOpen[agentID] = true
		delete(sv.agentCooldown, agentID)
	} else if decision.Backoff > 0 {
		sv.agentCooldown[agentID] = time.Now().Add(decision.Backoff)
	} else {
		delete(sv.agentCooldown, agentID)
	}
	sv.mu.Unlock()

	if sv.substrate != nil {
		sessionName := "agent-" + agentID
		if err := sv.substrate.DestroySession(ctx, sessionName); err != nil {
			log.Printf("supervisor: substrate: DestroySession %q failed (agent %s): %v", sessionName, agentID, err)
		}
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
	// Exponential backoff: skip this tick if backing off from store errors.
	if !sv.nextAssignAttempt.IsZero() && time.Now().Before(sv.nextAssignAttempt) {
		return
	}

	taskID, priority, err := sv.store.DequeueTask(ctx)
	if errors.Is(err, state.ErrQueueEmpty) {
		return
	}
	if err != nil {
		sv.recordAssignError()
		return
	}

	agentID, found := sv.findIdleAgent(ctx)
	if !found {
		// Re-enqueue at original priority since no idle agent available.
		if err := sv.store.EnqueueTask(ctx, taskID, priority); err != nil {
			log.Printf("supervisor: task %s lost — re-enqueue failed (no idle agent): %v", taskID, err)
		}
		return
	}

	// Acquire per-agent mutex for the entire assign+persist operation.
	// This prevents crashAgent from interleaving between the state
	// transition and the CurrentTaskID write (council 7, Defect 1).
	mu := sv.getAgentMutex(agentID)
	if mu == nil {
		if err := sv.store.EnqueueTask(ctx, taskID, priority); err != nil {
			log.Printf("supervisor: task %s lost — re-enqueue failed (agent unregistered): %v", taskID, err)
		}
		return
	}
	mu.Lock()

	snap, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventTaskAssigned)
	if err != nil {
		mu.Unlock()
		// Could not assign; re-enqueue at original priority.
		if err := sv.store.EnqueueTask(ctx, taskID, priority); err != nil {
			log.Printf("supervisor: task %s lost — re-enqueue failed (assign error): %v", taskID, err)
		}
		// CAS conflicts indicate healthy concurrency (another writer won the
		// race), not store degradation. Only apply backoff for non-CAS errors.
		var conflict *state.StateConflictError
		if !errors.As(err, &conflict) {
			sv.recordAssignError()
		}
		return
	}

	// Record the assigned task INSIDE the mutex so crashAgent reads a
	// consistent CurrentTaskID (read-modify-write to preserve
	// LastHeartbeat and RegisteredAt).
	if cur, fErr := sv.store.GetAgentFields(ctx, agentID); fErr == nil {
		cur.CurrentTaskID = taskID
		cur.CurrentTaskPriority = priority
		if err := sv.store.SetAgentFields(ctx, agentID, cur); err != nil {
			log.Printf("supervisor: task %s — CurrentTaskID not persisted (agent %s): %v", taskID, agentID, err)
			// Re-enqueue: the agent is ASSIGNED but has no CurrentTaskID.
			// It will self-heal via stale heartbeat → crashAgent → IDLE.
			// No dual-ownership risk: the zombie agent has no task context.
			if rErr := sv.store.EnqueueTask(ctx, taskID, priority); rErr != nil {
				log.Printf("supervisor: task %s lost — re-enqueue after SetAgentFields failure failed: %v", taskID, rErr)
			}
			sv.recordAssignError()
		} else {
			sv.resetAssignBackoff()
		}
	} else {
		// GetAgentFields failed after ApplyEvent succeeded — the agent is
		// ASSIGNED but CurrentTaskID is empty. Re-enqueue the task so it is
		// not permanently lost (council 8, Defect A).
		log.Printf("supervisor: task %s — GetAgentFields failed (agent %s), re-enqueueing: %v", taskID, agentID, fErr)
		if err := sv.store.EnqueueTask(ctx, taskID, priority); err != nil {
			log.Printf("supervisor: task %s lost — re-enqueue after GetAgentFields failure failed: %v", taskID, err)
		}
		sv.recordAssignError()
	}

	mu.Unlock()

	_ = sv.store.PublishEvent(ctx, state.Event{
		Type:      string(agent.EventTaskAssigned),
		AgentID:   snap.AgentID,
		TaskID:    taskID,
		Timestamp: time.Now().UnixMilli(),
	})
}

// recordAssignError increments the consecutive error counter and sets
// exponential backoff for tryAssignTask. Cap at 64x taskPollInterval.
// Only called from taskAssignLoop goroutine — no locking needed.
func (sv *Supervisor) recordAssignError() {
	sv.consecutiveAssignErrors++
	shift := sv.consecutiveAssignErrors
	if shift > 6 {
		shift = 6 // cap at 2^6 = 64x
	}
	sv.nextAssignAttempt = time.Now().Add(sv.taskPollInterval * time.Duration(1<<shift))
}

// resetAssignBackoff clears the backoff state after a successful assignment.
// Only called from taskAssignLoop goroutine — no locking needed.
func (sv *Supervisor) resetAssignBackoff() {
	sv.consecutiveAssignErrors = 0
	sv.nextAssignAttempt = time.Time{}
}

// findIdleAgent returns the ID of any idle agent that is not in cooldown or
// circuit-open state, or ("", false) if none exists.
// Cooldown/circuit-open maps are snapshotted under RLock then released before
// per-agent store I/O, so store latency does not block crashAgent's sv.mu.Lock.
func (sv *Supervisor) findIdleAgent(ctx context.Context) (string, bool) {
	agents, err := sv.store.ListAgents(ctx)
	if err != nil {
		return "", false
	}

	sv.mu.RLock()
	cooldownSnap := make(map[string]time.Time, len(sv.agentCooldown))
	for k, v := range sv.agentCooldown {
		cooldownSnap[k] = v
	}
	circuitSnap := make(map[string]bool, len(sv.circuitOpen))
	for k, v := range sv.circuitOpen {
		circuitSnap[k] = v
	}
	sv.mu.RUnlock()

	now := time.Now()
	for _, agentID := range agents {
		stateStr, err := sv.store.GetAgentState(ctx, agentID)
		if err != nil {
			continue
		}
		if stateStr != string(agent.StateIdle) {
			continue
		}
		if circuitSnap[agentID] {
			continue
		}
		if exp, ok := cooldownSnap[agentID]; ok && now.Before(exp) {
			continue
		}
		// Re-verify under fresh RLock: crashAgent may have updated circuit/cooldown
		// after the snapshot was taken (outer race window before sentinel pre-population).
		sv.mu.RLock()
		recheckCircuit := sv.circuitOpen[agentID]
		recheckExp, recheckCooling := sv.agentCooldown[agentID]
		sv.mu.RUnlock()
		if recheckCircuit || (recheckCooling && now.Before(recheckExp)) {
			continue
		}
		return agentID, true
	}
	return "", false
}

// Heartbeat refreshes the agent's LastHeartbeat timestamp.
// The per-agent mutex is held for the entire read-modify-write to prevent
// interleaving with tryAssignTask (which writes CurrentTaskID under the same mutex).
func (sv *Supervisor) Heartbeat(ctx context.Context, agentID string) error {
	if agentID == "" || len(agentID) > 128 {
		return ErrInvalidAgentID
	}
	sv.mu.RLock()
	if sv.stopped {
		sv.mu.RUnlock()
		return ErrSupervisorStopped
	}
	sv.mu.RUnlock()

	mu := sv.getAgentMutex(agentID)
	if mu == nil {
		return ErrSupervisorStopped
	}
	mu.Lock()
	defer mu.Unlock()

	fields, err := sv.store.GetAgentFields(ctx, agentID)
	if err != nil {
		return fmt.Errorf("heartbeat agent %s: %w", agentID, err)
	}
	fields.LastHeartbeat = time.Now().UnixMilli()
	if err := sv.store.SetAgentFields(ctx, agentID, fields); err != nil {
		return fmt.Errorf("heartbeat agent %s: %w", agentID, err)
	}
	return nil
}

// StartWork signals that an agent has started executing its assigned task
// (ASSIGNED → WORKING). The per-agent mutex is held to prevent interleaving
// with crashAgent's pre-read/ApplyEvent compound operation (council 7 invariant).
func (sv *Supervisor) StartWork(ctx context.Context, agentID string) error {
	if agentID == "" || len(agentID) > 128 {
		return ErrInvalidAgentID
	}
	mu := sv.getAgentMutex(agentID)
	if mu == nil {
		// Agent not registered (no mutex entry) — let ApplyEvent return the
		// proper ErrAgentNotFound rather than masking it as ErrSupervisorStopped.
		_, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventWorkStarted)
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	_, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventWorkStarted)
	return err
}

// ReportOutput signals that an agent has output ready (WORKING → REPORTING).
// The per-agent mutex is held to prevent interleaving with crashAgent (council 7 invariant).
func (sv *Supervisor) ReportOutput(ctx context.Context, agentID string) error {
	if agentID == "" || len(agentID) > 128 {
		return ErrInvalidAgentID
	}
	mu := sv.getAgentMutex(agentID)
	if mu == nil {
		// Agent not registered — let ApplyEvent return ErrAgentNotFound.
		_, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventOutputReady)
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	_, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventOutputReady)
	return err
}

// CompleteAgent is the public entry point for signaling agent task completion.
// It applies EventOutputDelivered, records success, and clears cooldown/circuit state.
func (sv *Supervisor) CompleteAgent(ctx context.Context, agentID string) error {
	return sv.completeAgent(ctx, agentID)
}

// completeAgent applies EventOutputDelivered and records a success with the
// restart policy, resetting consecutive crash counters and clearing any
// cooldown or circuit-breaker state for the agent.
//
// The per-agent mutex is held for the entire operation to prevent interleaving
// with crashAgent (which pre-populates cooldown sentinels).
func (sv *Supervisor) completeAgent(ctx context.Context, agentID string) error {
	mu := sv.getAgentMutex(agentID)
	if mu == nil {
		return ErrSupervisorStopped
	}
	mu.Lock()
	defer mu.Unlock()

	_, err := sv.machine.ApplyEvent(ctx, agentID, agent.EventOutputDelivered)
	if err != nil {
		return err
	}

	sv.policy.RecordSuccess(agentID)

	// Signal task completion BEFORE clearing CurrentTaskID so the task ID is
	// still readable. BlockingSubmitter callers unblock here.
	if sv.completionRegistry != nil {
		if fields, fErr := sv.store.GetAgentFields(ctx, agentID); fErr == nil && fields.CurrentTaskID != "" {
			sv.completionRegistry.Complete(fields.CurrentTaskID)
		} else if fErr != nil {
			log.Printf("supervisor: completeAgent %s — GetAgentFields failed, CompletionRegistry.Complete skipped: %v", agentID, fErr)
		} else {
			log.Printf("supervisor: completeAgent %s — CurrentTaskID empty, completion signal not sent", agentID)
		}
	}

	// Clear the assigned task so crashAgent doesn't re-enqueue a completed task.
	// Uses ClearCurrentTask (conditional write that returns ErrAgentNotFound for
	// unknown agents) instead of read-modify-write to avoid stale-CurrentTaskID
	// risk from GetAgentFields failure.
	if err := sv.store.ClearCurrentTask(ctx, agentID); err != nil {
		log.Printf("supervisor: CurrentTaskID not cleared for agent %s — duplicate re-enqueue possible on next crash: %v", agentID, err)
	}

	sv.mu.Lock()
	delete(sv.agentCooldown, agentID)
	delete(sv.circuitOpen, agentID)
	sv.mu.Unlock()

	return nil
}

// getAgentMutex returns the per-agent mutex, or nil if not registered.
func (sv *Supervisor) getAgentMutex(agentID string) *sync.Mutex {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.agentMu[agentID]
}
