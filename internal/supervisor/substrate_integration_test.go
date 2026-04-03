//go:build integration && testenv

package supervisor

import (
	"context"
	"os/exec"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

func skipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not found on PATH; skipping integration test")
	}
}

func TestIntegration_RegisterAgent_SpawnsTmuxSession(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := terminal.NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate failed: %v", err)
	}

	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := NewRestartPolicy()
	sv := NewSupervisor(machine, store, policy, WithSubstrate(sub))

	ctx := context.Background()
	agentID := "integ-agent-001"
	sessionName := "agent-" + agentID

	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Verify session exists.
	sessions, err := sub.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	found := false
	for _, s := range sessions {
		if s.Name == sessionName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("session %q not found after RegisterAgent", sessionName)
	}

	// Cleanup: destroy session.
	_ = sub.DestroySession(ctx, sessionName)
}

func TestIntegration_CrashAgent_DestroysTmuxSession(t *testing.T) {
	skipIfNoTmux(t)

	sub, err := terminal.NewTmuxSubstrate()
	if err != nil {
		t.Fatalf("NewTmuxSubstrate failed: %v", err)
	}

	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := NewRestartPolicy()
	sv := NewSupervisor(machine, store, policy, WithSubstrate(sub))

	ctx := context.Background()
	agentID := "integ-agent-002"
	sessionName := "agent-" + agentID

	if err := sv.RegisterAgent(ctx, agentID); err != nil {
		t.Fatalf("RegisterAgent failed: %v", err)
	}

	// Drive to WORKING so crash is non-spurious.
	if _, err := sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("EventTaskAssigned failed: %v", err)
	}
	if _, err := sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("EventWorkStarted failed: %v", err)
	}

	sv.CrashAgentForTest(ctx, agentID)

	// Verify session is gone.
	sessions, err := sub.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	for _, s := range sessions {
		if s.Name == sessionName {
			// Cleanup in case of test failure.
			_ = sub.DestroySession(ctx, sessionName)
			t.Fatalf("session %q still exists after crash", sessionName)
		}
	}
}
