//go:build testenv

package supervisor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

// stubSubstrate records spawn/destroy calls for assertion.
type stubSubstrate struct {
	mu         sync.Mutex
	spawned    []string
	destroyed  []string
	spawnErr   error
	destroyErr error
}

func (s *stubSubstrate) SpawnSession(_ context.Context, name, _ string) (terminal.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spawned = append(s.spawned, name)
	return terminal.Session{Name: name}, s.spawnErr
}

func (s *stubSubstrate) DestroySession(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.destroyed = append(s.destroyed, name)
	return s.destroyErr
}

func (s *stubSubstrate) SendCommand(_ context.Context, _, _ string) error {
	return nil
}

func (s *stubSubstrate) CaptureOutput(_ context.Context, _ string, _ int) (string, error) {
	return "", nil
}

func (s *stubSubstrate) ListSessions(_ context.Context) ([]terminal.Session, error) {
	return nil, nil
}

func (s *stubSubstrate) SplitPane(_ context.Context, _ string, _ terminal.Direction) (terminal.Pane, error) {
	return terminal.Pane{}, nil
}

func (s *stubSubstrate) getSpawned() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(s.spawned))
	copy(cp, s.spawned)
	return cp
}

func (s *stubSubstrate) getDestroyed() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(s.destroyed))
	copy(cp, s.destroyed)
	return cp
}

func newTestSupervisorWithSubstrate(sub terminal.Substrate) (*Supervisor, state.StateStore) {
	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := NewRestartPolicy()
	sv := NewSupervisor(machine, store, policy, WithSubstrate(sub))
	return sv, store
}

func TestSupervisor_RegisterAgent_SpawnsSession(t *testing.T) {
	stub := &stubSubstrate{}
	sv, _ := newTestSupervisorWithSubstrate(stub)
	ctx := context.Background()

	agentID := "test-agent-001"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	spawned := stub.getSpawned()
	if len(spawned) != 1 {
		t.Fatalf("expected 1 spawn call, got %d", len(spawned))
	}
	expected := "agent-" + agentID
	if spawned[0] != expected {
		t.Errorf("spawned session name = %q, want %q", spawned[0], expected)
	}
}

func TestSupervisor_RegisterAgent_SubstrateFailure_StillRegisters(t *testing.T) {
	stub := &stubSubstrate{spawnErr: errors.New("tmux down")}
	sv, store := newTestSupervisorWithSubstrate(stub)
	ctx := context.Background()

	agentID := "test-agent-002"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent should succeed despite substrate failure: %v", err)
	}

	// Agent should be registered in the store.
	stateStr, err := store.GetAgentState(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentState failed: %v", err)
	}
	if stateStr != string(agent.StateIdle) {
		t.Errorf("agent state = %q, want %q", stateStr, agent.StateIdle)
	}
}

func TestSupervisor_CrashAgent_DestroysSession(t *testing.T) {
	stub := &stubSubstrate{}
	sv, _ := newTestSupervisorWithSubstrate(stub)
	ctx := context.Background()

	agentID := "test-agent-003"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Drive agent to WORKING so crashAgent has a non-idle previous state.
	if _, err := sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("EventTaskAssigned failed: %v", err)
	}
	if _, err := sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("EventWorkStarted failed: %v", err)
	}

	// Update heartbeat to stale so crashAgent proceeds.
	fields, _ := sv.store.GetAgentFields(ctx, agentID)
	fields.LastHeartbeat = time.Now().Add(-1 * time.Hour).UnixMilli()
	_ = sv.store.SetAgentFields(ctx, agentID, fields)

	sv.CrashAgentForTest(ctx, agentID)

	destroyed := stub.getDestroyed()
	if len(destroyed) != 1 {
		t.Fatalf("expected 1 destroy call, got %d", len(destroyed))
	}
	expected := "agent-" + agentID
	if destroyed[0] != expected {
		t.Errorf("destroyed session name = %q, want %q", destroyed[0], expected)
	}
}

func TestSupervisor_CrashAgent_SubstrateFailure_StillCrashes(t *testing.T) {
	stub := &stubSubstrate{destroyErr: errors.New("tmux down")}
	sv, _ := newTestSupervisorWithSubstrate(stub)
	ctx := context.Background()

	agentID := "test-agent-004"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Drive to WORKING.
	if _, err := sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("EventTaskAssigned failed: %v", err)
	}
	if _, err := sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("EventWorkStarted failed: %v", err)
	}

	sv.CrashAgentForTest(ctx, agentID)

	// Agent should be back to IDLE despite substrate failure.
	stateStr, err := sv.store.GetAgentState(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentState failed: %v", err)
	}
	if stateStr != string(agent.StateIdle) {
		t.Errorf("agent state after crash = %q, want %q", stateStr, agent.StateIdle)
	}
}

func TestSupervisor_NilSubstrate_NoPanic(t *testing.T) {
	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := NewRestartPolicy()
	sv := NewSupervisor(machine, store, policy) // no WithSubstrate

	ctx := context.Background()

	agentID := "test-agent-005"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Drive to WORKING and crash — should not panic with nil substrate.
	if _, err := sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("EventTaskAssigned failed: %v", err)
	}
	if _, err := sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("EventWorkStarted failed: %v", err)
	}

	sv.CrashAgentForTest(ctx, agentID)

	stateStr, err := store.GetAgentState(ctx, agentID)
	if err != nil {
		t.Fatalf("GetAgentState failed: %v", err)
	}
	if stateStr != string(agent.StateIdle) {
		t.Errorf("agent state after crash = %q, want %q", stateStr, agent.StateIdle)
	}
}
