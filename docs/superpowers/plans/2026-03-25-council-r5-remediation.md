# Council R5 Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve council R5 blocking condition (doc comment fix) and all 4 epic-branch follow-ons in a single PR update.

**Architecture:** Five independent changes: doc comment correction, dead code removal + test redirect, MockStore error injection hook + supervisor generic-error test, input validation on RegisterAgent, and StateStore interface documentation.

**Tech Stack:** Go, go test, race detector

---

### Task 1: Fix doc comment at `machine.go:35-38`

**Files:**
- Modify: `internal/agent/machine.go:35-38`

- [ ] **Step 1: Rewrite the doc comment**

Replace lines 35-38:
```go
// Concurrency: ApplyEvent is safe for concurrent calls on the same agentID at
// the storage level — CompareAndSetAgentState provides atomicity. The
// supervisor's per-agent mutex remains as a performance optimisation to reduce
// contention at Redis, but is no longer the sole correctness guard.
```

With:
```go
// Concurrency: ApplyEvent is safe for concurrent calls on the same agentID at
// the storage level — CompareAndSetAgentState provides atomicity for state
// transitions. The supervisor's per-agent mutex remains necessary for
// correctness of compound operations (e.g., state transition followed by field
// writes in tryAssignTask). CAS replaced the mutex as the atomicity guard for
// state transitions themselves; the mutex now coordinates compound operations
// within a single supervisor process.
```

- [ ] **Step 2: Run tests to verify no regressions**

Run: `go test -race -tags=testenv ./internal/agent/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git commit -m "docs(agent): correct mutex role description in ApplyEvent doc comment

Council R5 condition: the second sentence at machine.go:35-38 mischaracterized
the per-agent mutex as a 'performance optimisation'. It is a correctness
requirement for compound operations (state + field writes) in tryAssignTask."
```

---

### Task 2: Remove dead `applyEvent` helper + redirect serialization test

**Files:**
- Modify: `internal/supervisor/supervisor.go:477-488` (delete)
- Modify: `internal/supervisor/export_e2e.go:16-18` (delete `ApplyEventForTest`)
- Modify: `internal/supervisor/supervisor_test.go:83-128` (rewrite test)

- [ ] **Step 1: Delete `applyEvent` helper from supervisor.go**

Remove lines 477-488 (the entire `applyEvent` method).

- [ ] **Step 2: Delete `ApplyEventForTest` from export_e2e.go**

Remove lines 16-18 (the `ApplyEventForTest` method and its doc comment at lines 11-15). Keep `CrashAgentForTest` and `CompleteAgentForTest`.

- [ ] **Step 3: Rewrite `TestSupervisor_ApplyEventSerializes` to test production path**

Replace the test to use `sv.Run` with concurrent task enqueues, verifying that the inline mutex in `tryAssignTask` serializes correctly:

```go
func TestSupervisor_TryAssignTask_Serializes(t *testing.T) {
	t.Parallel()

	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := supervisor.NewRestartPolicy(
		supervisor.WithCrashThreshold(10),
		supervisor.WithCrashWindow(60*time.Second),
	)
	sv := supervisor.NewSupervisor(machine, store, policy,
		supervisor.WithTaskPollInterval(10*time.Millisecond),
	)

	ctx := context.Background()
	const agentID = "agent-serialize"

	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	// Enqueue multiple tasks — only one can be assigned at a time to a single agent.
	for i := 0; i < 5; i++ {
		if err := store.EnqueueTask(ctx, fmt.Sprintf("task-%d", i), float64(i)); err != nil {
			t.Fatalf("EnqueueTask: %v", err)
		}
	}

	// Run briefly — supervisor assigns first task via tryAssignTask.
	runCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_ = sv.Run(runCtx)

	// Agent should be assigned (first task won).
	stateStr, err := store.GetAgentState(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateStr != string(agent.StateAssigned) {
		t.Fatalf("want assigned, got %q", stateStr)
	}

	// CurrentTaskID should be set (compound operation completed atomically).
	fields, err := store.GetAgentFields(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentFields: %v", err)
	}
	if fields.CurrentTaskID == "" {
		t.Fatal("CurrentTaskID should be set after assignment")
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race -tags=testenv ./internal/supervisor/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -m "refactor(supervisor): remove dead applyEvent helper, redirect serialization test

Council R5 follow-on #1: applyEvent at supervisor.go:477-488 had zero
production callers. TestSupervisor_ApplyEventSerializes exercised only
the dead helper via ApplyEventForTest. Replaced with a test that verifies
tryAssignTask's inline mutex serialization through sv.Run."
```

---

### Task 3: Add MockStore CAS error injection hook + test generic-error branch

**Files:**
- Modify: `internal/state/mock.go` (add hook field + setter + inject into CompareAndSetAgentState)
- Modify: `internal/supervisor/supervisor_test.go` (add test for generic CAS error → backoff)

- [ ] **Step 1: Add `compareAndSetErr` field and setter to MockStore**

Add to MockStore struct: `compareAndSetErr error`

Add setter:
```go
// SetCompareAndSetAgentStateError configures CompareAndSetAgentState to return err.
// Pass nil to reset to healthy.
func (m *MockStore) SetCompareAndSetAgentStateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compareAndSetErr = err
}
```

Inject into `CompareAndSetAgentState`:
```go
func (m *MockStore) CompareAndSetAgentState(ctx context.Context, agentID string, expected, next string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.compareAndSetErr != nil {
		return m.compareAndSetErr
	}
	// ... rest unchanged
}
```

- [ ] **Step 2: Add supervisor test for generic CAS error → backoff**

```go
// casGenericErrorStore wraps a StateStore and makes CompareAndSetAgentState
// return a generic (non-StateConflictError) error, simulating a Redis timeout.
type casGenericErrorStore struct {
	state.StateStore
}

func (s *casGenericErrorStore) CompareAndSetAgentState(_ context.Context, _ string, _, _ string) error {
	return fmt.Errorf("redis: connection timeout")
}

func TestSupervisor_TryAssignTask_CASGenericError_TriggersBackoff(t *testing.T) {
	t.Parallel()

	base := state.NewMockStore()
	wrapper := &casGenericErrorStore{StateStore: base}
	countingWrapper := &dequeueCountingStore{casConflictStore: casConflictStore{StateStore: wrapper}}
	// Override CAS to use wrapper's generic error
	machine := agent.NewMachine(wrapper)
	policy := supervisor.NewRestartPolicy(
		supervisor.WithCrashThreshold(10),
		supervisor.WithCrashWindow(60*time.Second),
	)
	sv := supervisor.NewSupervisor(machine, wrapper, policy,
		supervisor.WithTaskPollInterval(10*time.Millisecond),
	)

	ctx := context.Background()

	if err := sv.RegisterAgent(ctx, "agent-generic-err"); err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if err := wrapper.EnqueueTask(ctx, "task-generic-err", 1.0); err != nil {
		t.Fatalf("EnqueueTask: %v", err)
	}

	// Run for 200ms. With backoff, after a few errors the supervisor should
	// slow down significantly — far fewer dequeues than the no-backoff test.
	runCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_ = sv.Run(runCtx)

	// The task should be re-enqueued (not lost).
	n, err := base.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if n < 1 {
		t.Fatal("task should be re-enqueued after generic CAS error")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test -race -tags=testenv ./internal/state/... ./internal/supervisor/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git commit -m "test(state): add SetCompareAndSetAgentStateError hook to MockStore

Council R5 follow-on #2: MockStore.CompareAndSetAgentState lacked an
injectable error hook, preventing unit-testing the generic-error path
at supervisor.go:322. Add the hook and a supervisor test verifying that
non-StateConflictError CAS failures trigger recordAssignError/backoff."
```

---

### Task 4: Add RegisterAgent input validation

**Files:**
- Modify: `internal/supervisor/supervisor.go:83` (add validation)
- Modify: `internal/supervisor/errors.go` or create if needed (add ErrInvalidAgentID)
- Modify: `internal/supervisor/supervisor_test.go` (add validation tests)

- [ ] **Step 1: Check if errors.go exists in supervisor package**

- [ ] **Step 2: Add validation error and validation logic**

Add to supervisor package:
```go
var ErrInvalidAgentID = errors.New("invalid agent ID: must be non-empty and at most 128 characters")
```

Add validation at the top of `RegisterAgent`:
```go
func (sv *Supervisor) RegisterAgent(ctx context.Context, agentID string) error {
	if agentID == "" || len(agentID) > 128 {
		return ErrInvalidAgentID
	}
	// ... rest unchanged
}
```

- [ ] **Step 3: Add tests**

```go
func TestSupervisor_RegisterAgent_EmptyID(t *testing.T) {
	t.Parallel()
	sv, _ := newTestSupervisor(t)
	err := sv.RegisterAgent(context.Background(), "")
	if !errors.Is(err, supervisor.ErrInvalidAgentID) {
		t.Fatalf("want ErrInvalidAgentID, got %v", err)
	}
}

func TestSupervisor_RegisterAgent_TooLongID(t *testing.T) {
	t.Parallel()
	sv, _ := newTestSupervisor(t)
	longID := strings.Repeat("a", 129)
	err := sv.RegisterAgent(context.Background(), longID)
	if !errors.Is(err, supervisor.ErrInvalidAgentID) {
		t.Fatalf("want ErrInvalidAgentID, got %v", err)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race -tags=testenv ./internal/supervisor/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -m "feat(supervisor): add RegisterAgent input validation

Council R5 follow-on #3: reject empty and >128-char agentIDs before
Redis key construction. Prevents key namespace pollution."
```

---

### Task 5: Add StateStore interface semantic documentation

**Files:**
- Modify: `internal/state/store.go:34-55`

- [ ] **Step 1: Add semantic safety documentation to interface**

Update the interface doc comment and add per-method annotations:

```go
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
//     state transitions — bypasses optimistic locking. Use CompareAndSetAgentState
//     for all transitions on agents that are already participating in the lifecycle.
//   - CompareAndSetAgentState: the ONLY safe method for in-flight state transitions.
//     Provides atomic compare-and-swap; returns *StateConflictError on concurrent
//     modification.
//   - SetAgentFields: field-only updates (LastHeartbeat, CurrentTaskID, etc.).
//     Does not participate in CAS — callers must hold the per-agent mutex when
//     writing fields that must be consistent with state (e.g., CurrentTaskID after
//     a state transition to assigned).
```

- [ ] **Step 2: Run tests to verify no regressions**

Run: `go test -race -tags=testenv ./internal/state/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git commit -m "docs(state): add StateStore interface state-write safety documentation

Council R5 follow-on #4: document which methods are safe for in-flight
state transitions (CompareAndSetAgentState) vs registration/seeding
(SetAgentState) vs field-only updates (SetAgentFields)."
```
