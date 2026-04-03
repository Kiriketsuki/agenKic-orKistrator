//go:build testenv

package e2e_test

import (
	"context"
	"testing"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
)

// ── Scenario 1: Agent Registration ───────────────────────────────────────────

// TestE2E_AgentRegistration verifies that a newly registered agent receives a
// non-empty ID and starts in the idle state.
func TestE2E_AgentRegistration(t *testing.T) {
	s := newTestStack(t, nil)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-reg-test", ModelTier: "sonnet"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if regResp.AgentId == "" {
		t.Fatal("expected non-empty agent_id")
	}

	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: regResp.AgentId})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected IDLE, got %v", stateResp.State)
	}
}

// ── Scenario 2: Task Auto-Assignment ─────────────────────────────────────────

// TestE2E_TaskAutoAssignment verifies that the supervisor's background task-assign
// loop dequeues a pending task and transitions an idle agent to ASSIGNED.
func TestE2E_TaskAutoAssignment(t *testing.T) {
	s := newTestStack(t, nil)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-assign-test"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-auto-001", Prompt: "do something", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	pollAgentState(t, s.client, regResp.AgentId, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)
}

// ── Scenario 3: Task Queued When All Agents Busy ──────────────────────────────

// TestE2E_TaskNotAssignedWhileAgentBusy verifies that the supervisor's assign loop
// does not reassign a task to a WORKING agent, and auto-assigns once the agent returns idle.
func TestE2E_TaskNotAssignedWhileAgentBusy(t *testing.T) {
	s := newTestStack(t, nil)
	defer s.cleanup()

	ctx := context.Background()

	// Register the only agent and manually advance it to WORKING so no idle slot exists.
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-busy-test"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("advance to assigned: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}

	// Submit a task — no idle agent, so it must stay queued.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-queued-001", Prompt: "queued work", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	// Give the assign loop several ticks — a WORKING agent cannot transition to ASSIGNED,
	// so this sleep is a deliberate negative-case wait before the structural assertion.
	time.Sleep(100 * time.Millisecond)

	// Structural assertion: agent must still be WORKING (findIdleAgent skips non-idle agents).
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_WORKING {
		t.Fatalf("expected agent to remain WORKING, got %v", stateResp.State)
	}

	// Drive agent back to idle — the queued task should now be assigned automatically.
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventOutputReady); err != nil {
		t.Fatalf("advance to reporting: %v", err)
	}
	if err := s.sv.CompleteAgentForTest(ctx, agentID); err != nil {
		t.Fatalf("CompleteAgentForTest: %v", err)
	}

	// Supervisor should now assign the queued task.
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)
}
