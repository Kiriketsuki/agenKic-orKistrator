//go:build testenv

package e2e_test

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
)

// ── Scenario 8: Crash Cycle Policy Backoff [gRPC-bypassed] ───────────────

// TestCrashCycle_PolicyBackoff verifies that the RestartPolicy correctly computes
// exponential backoff after consecutive agent crashes. State transitions are
// driven directly via ApplyEventForTest + policy.RecordCrash (not the integrated
// crashAgent path). Supervisor-enforced cooldown is tested in
// TestE2E_CooldownEnforcement (Scenario 10).
func TestCrashCycle_PolicyBackoff(t *testing.T) {
	s := newTestStack(t, nil)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-backoff-test"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// First crash cycle: idle → assigned → working → failed (crash).
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("advance to assigned: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventAgentFailed); err != nil {
		t.Fatalf("first crash: %v", err)
	}

	d1 := s.policy.RecordCrash(agentID)
	if !d1.ShouldRestart {
		t.Fatal("expected ShouldRestart=true after first crash")
	}
	if d1.Backoff != 1*time.Second {
		t.Fatalf("expected 1s backoff after first crash, got %v", d1.Backoff)
	}

	// Second crash cycle.
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("advance to assigned (2): %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working (2): %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventAgentFailed); err != nil {
		t.Fatalf("second crash: %v", err)
	}

	d2 := s.policy.RecordCrash(agentID)
	if !d2.ShouldRestart {
		t.Fatal("expected ShouldRestart=true after second crash")
	}
	if d2.Backoff != 2*time.Second {
		t.Fatalf("expected 2s backoff after second crash, got %v", d2.Backoff)
	}

	// Verify agent returned to idle after the crash.
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected IDLE after crash, got %v", stateResp.State)
	}
}

// ── Scenario 9: Crash Cycle Policy Circuit Breaker [gRPC-bypassed] ───────────

// TestCrashCycle_PolicyCircuitBreaker verifies that the RestartPolicy's circuit
// breaker opens after more than crashThreshold crashes within the crash window.
// State transitions are driven directly via ApplyEventForTest + policy.RecordCrash
// (not the integrated crashAgent path). Supervisor-enforced circuit breaker is
// tested in TestE2E_CircuitBreakerBlocksAssignment (Scenario 11).
func TestCrashCycle_PolicyCircuitBreaker(t *testing.T) {
	s := newTestStack(t, nil)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-circuit-test"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Simulate 5 crash cycles — all should return ShouldRestart=true.
	for i := 1; i <= 5; i++ {
		if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
			t.Fatalf("crash %d: advance to assigned: %v", i, err)
		}
		if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
			t.Fatalf("crash %d: advance to working: %v", i, err)
		}
		if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventAgentFailed); err != nil {
			t.Fatalf("crash %d: EventAgentFailed: %v", i, err)
		}

		d := s.policy.RecordCrash(agentID)
		if !d.ShouldRestart {
			t.Fatalf("crash %d: expected ShouldRestart=true, got false", i)
		}
	}

	// 6th crash — circuit breaker should trip.
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("crash 6: advance to assigned: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("crash 6: advance to working: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventAgentFailed); err != nil {
		t.Fatalf("crash 6: EventAgentFailed: %v", err)
	}

	d6 := s.policy.RecordCrash(agentID)
	if d6.ShouldRestart {
		t.Fatal("expected ShouldRestart=false after 6th crash (circuit breaker)")
	}
	if !errors.Is(d6.Reason, supervisor.ErrCircuitOpen) {
		t.Fatalf("expected Reason=ErrCircuitOpen, got %v", d6.Reason)
	}
}

// ── Scenario 10: Cooldown Enforcement (E2E) ─────────────────────────────

// TestE2E_CooldownEnforcement verifies that the supervisor's task-assign loop
// does not assign a task to an agent that is in cooldown after a crash. Once the
// cooldown expires, the agent becomes eligible for assignment again.
func TestE2E_CooldownEnforcement(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
		supervisor.WithBaseBackoff(80*time.Millisecond),
		supervisor.WithCrashThreshold(10),
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-cooldown-e2e"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Advance agent to WORKING, then crash via CrashAgentForTest (integrates policy).
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
		t.Fatalf("advance to assigned: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}
	s.sv.CrashAgentForTest(ctx, agentID)

	// Agent is IDLE but in cooldown (80ms). Submit a task.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-cd-001", Prompt: "cooldown test", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	// Give the assign loop several ticks — task should NOT be assigned (agent in cooldown).
	time.Sleep(40 * time.Millisecond) // well within the 80ms cooldown

	// Structural assertion: agent must remain IDLE (findIdleAgent skips agents in cooldown).
	// GetAgentState is cycle-stable — unlike QueueLength, it is not subject to the
	// transient-zero window caused by tryAssignTask's dequeue-before-requeue pattern.
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected agent to remain IDLE during cooldown, got %v", stateResp.State)
	}

	// Wait for cooldown to expire, then the task should be assigned.
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 500*time.Millisecond)
}

// ── Scenario 11: Circuit Breaker Blocks Assignment (E2E) ─────────────────

// TestE2E_CircuitBreakerBlocksAssignment verifies that the supervisor's
// task-assign loop permanently skips an agent whose circuit breaker is open.
// After exceeding the crash threshold, the agent should never receive a task.
func TestE2E_CircuitBreakerBlocksAssignment(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
		supervisor.WithBaseBackoff(10*time.Millisecond),
		supervisor.WithCrashThreshold(3),
		supervisor.WithCrashWindow(60*time.Second),
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-circuit-e2e"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Crash the agent 4 times (threshold=3, so >3 opens circuit).
	for i := 1; i <= 4; i++ {
		if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
			t.Fatalf("crash %d: advance to assigned: %v", i, err)
		}
		if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
			t.Fatalf("crash %d: advance to working: %v", i, err)
		}
		s.sv.CrashAgentForTest(ctx, agentID)
	}

	// Agent is IDLE but circuit-open. Submit a task.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-cb-001", Prompt: "circuit test", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	// Wait generously — agent should never be assigned (circuit open).
	time.Sleep(200 * time.Millisecond)

	// Structural assertion: agent must remain IDLE (findIdleAgent skips circuit-open agents).
	// GetAgentState is cycle-stable — unlike QueueLength, it is not subject to the
	// transient-zero window caused by tryAssignTask's dequeue-before-requeue pattern.
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected agent to remain IDLE (circuit open), got %v", stateResp.State)
	}
}

// ── Scenario 12: Spurious Crash Guard (E2E) ────────────────────────────────

// TestE2E_SpuriousCrashGuard verifies that CrashAgentForTest on an already-IDLE
// agent is a no-op: the TOCTOU guard (supervisor.go PreviousState==StateIdle)
// fires, no crash is recorded, no cooldown is set, and the agent remains
// eligible for immediate task assignment.
func TestE2E_SpuriousCrashGuard(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
		supervisor.WithCrashThreshold(3),
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-spurious-test"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Agent is IDLE. Crash it — the TOCTOU guard should fire and skip RecordCrash.
	s.sv.CrashAgentForTest(ctx, agentID)

	// Agent should still be IDLE (EventAgentFailed from IDLE → IDLE is a no-op transition).
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected IDLE after spurious crash, got %v", stateResp.State)
	}

	// Behavioral proof: submit a task and verify it gets assigned promptly.
	// If RecordCrash had fired, cooldown would block assignment.
	// NOTE: 500ms timeout < default baseBackoff (1s, restart.go:72). If baseBackoff
	// is reduced below this timeout, this test stops detecting TOCTOU guard regressions.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-spurious-001", Prompt: "spurious test", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 500*time.Millisecond)
}
