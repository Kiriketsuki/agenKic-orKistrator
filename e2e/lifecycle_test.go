//go:build testenv

package e2e_test

import (
	"context"
	"testing"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
)

// ── Scenario 4: Full Agent Lifecycle ─────────────────────────────────────────

// TestE2E_FullAgentLifecycle verifies all four state transitions in sequence:
// idle → assigned → working → reporting → idle.
func TestE2E_FullAgentLifecycle(t *testing.T) {
	s := newTestStack(t, nil)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-lifecycle-test"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// idle → assigned (supervisor does this automatically)
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-lifecycle-001", Prompt: "lifecycle task", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)

	// assigned → working
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("EventWorkStarted: %v", err)
	}
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState (working): %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_WORKING {
		t.Fatalf("expected WORKING, got %v", stateResp.State)
	}

	// working → reporting
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventOutputReady); err != nil {
		t.Fatalf("EventOutputReady: %v", err)
	}
	stateResp, err = s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState (reporting): %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_REPORTING {
		t.Fatalf("expected REPORTING, got %v", stateResp.State)
	}

	// reporting → idle via gRPC CompleteAgent (production path)
	if _, err := s.client.CompleteAgent(ctx, &pb.CompleteAgentRequest{AgentId: agentID}); err != nil {
		t.Fatalf("CompleteAgent: %v", err)
	}
	stateResp, err = s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState (idle): %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected IDLE, got %v", stateResp.State)
	}
}

// ── Scenario 5: Heartbeat Stale Detection ────────────────────────────────────

// TestE2E_HeartbeatStaleDetection verifies that the supervisor's heartbeat loop
// detects a stale heartbeat on a non-idle agent and resets its state to idle via
// EventAgentFailed.
func TestE2E_HeartbeatStaleDetection(t *testing.T) {
	// Use a tight stale threshold so the heartbeat loop fires quickly.
	s := newTestStack(t, []supervisor.SupervisorOption{supervisor.WithStaleThreshold(50 * time.Millisecond)})
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-heartbeat-test"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Advance agent to WORKING.
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("advance to assigned: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}

	// Backdate the heartbeat beyond the stale threshold (50ms).
	staleTime := time.Now().Add(-200 * time.Millisecond).UnixMilli()
	if err := s.store.SetAgentFields(ctx, agentID, state.AgentFields{
		State:         string(agent.StateWorking),
		LastHeartbeat: staleTime,
	}); err != nil {
		t.Fatalf("SetAgentFields: %v", err)
	}

	// Heartbeat loop fires every 20ms; wait for it to detect the stale agent and
	// apply EventAgentFailed (which resets state to idle).
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_IDLE, 500*time.Millisecond)

	// NOTE: checkHeartbeats calls crashAgent which records the crash with the
	// RestartPolicy and sets cooldown. This test verifies heartbeat stale detection
	// and agent reset to IDLE only; cooldown enforcement is tested in
	// TestE2E_CooldownEnforcement and circuit breaker in TestE2E_CircuitBreakerBlocksAssignment.
}
