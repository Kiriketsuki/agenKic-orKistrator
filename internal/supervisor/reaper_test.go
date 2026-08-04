package supervisor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

// reaperSubstrate reports an explicit session list (and optionally an
// error) so a sweep can be driven against a known terminal state. Declared
// standalone rather than embedding substrate_test.go's stubSubstrate,
// because that file is behind a `testenv` build tag and these cases should
// run in the default test suite.
type reaperSubstrate struct {
	sessions []terminal.Session
	listErr  error
}

func (r *reaperSubstrate) ListSessions(_ context.Context) ([]terminal.Session, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.sessions, nil
}

func (r *reaperSubstrate) SpawnSession(_ context.Context, name, _ string) (terminal.Session, error) {
	return terminal.Session{Name: name}, nil
}
func (r *reaperSubstrate) DestroySession(_ context.Context, _ string) error { return nil }
func (r *reaperSubstrate) SendCommand(_ context.Context, _, _ string) error { return nil }
func (r *reaperSubstrate) SendKey(_ context.Context, _, _ string) error     { return nil }
func (r *reaperSubstrate) CaptureOutput(_ context.Context, _ string, _ int) (string, error) {
	return "", nil
}
func (r *reaperSubstrate) SplitPane(_ context.Context, _ string, _ terminal.Direction) (terminal.Pane, error) {
	return terminal.Pane{}, nil
}

// newReaperTestSupervisor builds a supervisor whose reap interval is short
// enough that an agent registered "now" is already old enough to reap, so
// the young-agent grace period does not mask the behavior under test.
func newReaperTestSupervisor(sub terminal.Substrate) (*Supervisor, state.StateStore) {
	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := NewRestartPolicy()
	sv := NewSupervisor(machine, store, policy, WithSubstrate(sub), WithReapInterval(time.Nanosecond))
	return sv, store
}

func registeredAgentIDs(t *testing.T, ctx context.Context, store state.StateStore) []string {
	t.Helper()
	agents, err := store.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	return agents
}

// An agent whose tmux session has vanished must be deregistered — this is
// the case that previously left a healthy-looking agent in the UI whose
// every keystroke 404'd.
func TestReaper_RemovesAgentWithNoSession(t *testing.T) {
	sub := &reaperSubstrate{}
	sv, store := newReaperTestSupervisor(sub)
	ctx := context.Background()

	const agentID = "orphaned-agent"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}
	// Substrate reports no sessions at all, so this agent is orphaned.
	sub.sessions = nil

	sv.reapOrphanedAgents(ctx)

	if got := registeredAgentIDs(t, ctx, store); len(got) != 0 {
		t.Fatalf("want orphaned agent deregistered, still registered: %v", got)
	}
}

// The mirror case: an agent whose session is alive must survive a sweep.
func TestReaper_KeepsAgentWithLiveSession(t *testing.T) {
	sub := &reaperSubstrate{}
	sv, store := newReaperTestSupervisor(sub)
	ctx := context.Background()

	const agentID = "live-agent"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}
	sub.sessions = []terminal.Session{{Name: sessionPrefix + agentID}}

	sv.reapOrphanedAgents(ctx)

	got := registeredAgentIDs(t, ctx, store)
	if len(got) != 1 || got[0] != agentID {
		t.Fatalf("want live agent %q kept, got %v", agentID, got)
	}
}

// A failed session listing must abort the sweep without removing anything.
// Treating an unreadable list as "every session is gone" would deregister
// the whole roster on a transient tmux failure.
func TestReaper_ListErrorRemovesNothing(t *testing.T) {
	sub := &reaperSubstrate{listErr: errors.New("tmux unavailable")}
	sv, store := newReaperTestSupervisor(sub)
	ctx := context.Background()

	const agentID = "agent-during-outage"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	sv.reapOrphanedAgents(ctx)

	if got := registeredAgentIDs(t, ctx, store); len(got) != 1 {
		t.Fatalf("want roster untouched when ListSessions fails, got %v", got)
	}
}

// A just-registered agent races the supervisor's own session creation, so
// one younger than a full reap interval must be left alone.
func TestReaper_SkipsFreshlyRegisteredAgent(t *testing.T) {
	sub := &reaperSubstrate{}
	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := NewRestartPolicy()
	// A long interval makes every just-registered agent "too young".
	sv := NewSupervisor(machine, store, policy, WithSubstrate(sub), WithReapInterval(time.Hour))
	ctx := context.Background()

	const agentID = "just-registered"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}
	sub.sessions = nil

	sv.reapOrphanedAgents(ctx)

	if got := registeredAgentIDs(t, ctx, store); len(got) != 1 {
		t.Fatalf("want freshly-registered agent kept during grace period, got %v", got)
	}
}

// With no substrate wired there is no session list to consult, so a sweep
// must be a no-op rather than deregistering every agent.
func TestReaper_NoSubstrateRemovesNothing(t *testing.T) {
	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := NewRestartPolicy()
	sv := NewSupervisor(machine, store, policy, WithReapInterval(time.Nanosecond))
	ctx := context.Background()

	const agentID = "substrateless-agent"
	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	sv.reapOrphanedAgents(ctx)

	if got := registeredAgentIDs(t, ctx, store); len(got) != 1 {
		t.Fatalf("want roster untouched with no substrate, got %v", got)
	}
}
