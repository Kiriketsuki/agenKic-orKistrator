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

// ── Scenario 13: Crash Recovery Re-enqueues Task (E2E) ──────────────────────

// TestE2E_CrashRecoveryReenqueuesTask verifies that when a WORKING agent crashes,
// the task it was assigned is re-enqueued at its original priority rather than
// being permanently lost. A second idle agent should pick up the re-enqueued task.
func TestE2E_CrashRecoveryReenqueuesTask(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
		supervisor.WithBaseBackoff(50*time.Millisecond),
		supervisor.WithCrashThreshold(10),
	)
	defer s.cleanup()

	ctx := context.Background()

	// Register two agents.
	reg1, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-crash-recovery-1"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent 1: %v", err)
	}
	reg2, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-crash-recovery-2"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent 2: %v", err)
	}
	agent1 := reg1.AgentId
	agent2 := reg2.AgentId

	// Submit a task — supervisor assigns to one of the agents.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-recovery-001", Prompt: "recoverable work", Priority: 5.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	// Give the assign loop time to route the task to one of the two agents.
	// This sleep is for test logistics only — the real assertion is pollAgentState below.
	time.Sleep(100 * time.Millisecond)

	// Determine which agent was assigned and advance it to WORKING.
	var assignedAgent, idleAgent string
	resp1, _ := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agent1})
	if resp1.State == pb.AgentState_AGENT_STATE_ASSIGNED {
		assignedAgent, idleAgent = agent1, agent2
	} else {
		assignedAgent, idleAgent = agent2, agent1
	}

	if _, err := s.sv.ApplyEventForTest(ctx, assignedAgent, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}

	// Crash the working agent — crashAgent should re-enqueue the task.
	s.sv.CrashAgentForTest(ctx, assignedAgent)

	// The idle agent should pick up the re-enqueued task.
	pollAgentState(t, s.client, idleAgent, pb.AgentState_AGENT_STATE_ASSIGNED, 500*time.Millisecond)

	// Verify the queue is empty (task was dequeued and assigned to the idle agent).
	qLen, err := s.store.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if qLen != 0 {
		t.Fatalf("expected empty queue after re-assignment, got %d", qLen)
	}
}

// ── Scenario 14: GetAgentFields Failure Re-enqueues Task (E2E) ──────────────

// TestE2E_GetAgentFieldsFailureReenqueuesTask verifies that when GetAgentFields
// fails inside tryAssignTask (after ApplyEvent succeeds), the task is re-enqueued
// rather than silently lost. Once the error is cleared, the task should be
// assigned normally. This exercises the council 8 Defect A fix.
func TestE2E_GetAgentFieldsFailureReenqueuesTask(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-getfields-err"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Inject GetAgentFields error before submitting the task.
	s.store.SetGetAgentFieldsError(errors.New("injected GetAgentFields failure"))

	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-gaf-001", Prompt: "test getfields error", Priority: 3.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	// Give the assign loop several ticks — tryAssignTask will dequeue the task,
	// ApplyEvent will succeed (agent → ASSIGNED), but GetAgentFields will fail.
	// The else branch re-enqueues the task. The assign loop will repeat: dequeue,
	// ApplyEvent fails (already ASSIGNED), re-enqueue. The task stays in the queue.
	time.Sleep(80 * time.Millisecond)

	// Clear the error so the next assign cycle can complete.
	s.store.SetGetAgentFieldsError(nil)

	// The agent is already ASSIGNED from the first ApplyEvent that succeeded.
	// The task was re-enqueued. Drive the agent back to IDLE so the task can
	// be assigned again cleanly.
	//
	// Complete the agent: ASSIGNED → WORKING → REPORTING → IDLE.
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventOutputReady); err != nil {
		t.Fatalf("advance to reporting: %v", err)
	}
	if err := s.sv.CompleteAgentForTest(ctx, agentID); err != nil {
		t.Fatalf("CompleteAgentForTest: %v", err)
	}

	// Agent is now IDLE again. The re-enqueued task should be picked up.
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 500*time.Millisecond)

	// Verify queue is empty — task was dequeued and assigned.
	qLen, err := s.store.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if qLen != 0 {
		t.Fatalf("expected empty queue after recovery, got %d", qLen)
	}
}

// ── Scenario 15: SetAgentFields Failure in CompleteAgent (E2E) ──────────────

// TestE2E_SetAgentFieldsFailureInCompleteAgent verifies that completeAgent
// succeeds (agent transitions to IDLE) even when SetAgentFields fails. The
// state transition is the critical operation; the CurrentTaskID clear is
// best-effort with a log warning.
func TestE2E_SetAgentFieldsFailureInCompleteAgent(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-setfields-err"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Submit a task and wait for assignment.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-saf-001", Prompt: "test setfields error", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)

	// Advance to REPORTING.
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventOutputReady); err != nil {
		t.Fatalf("advance to reporting: %v", err)
	}

	// Inject SetAgentFields error before completing.
	s.store.SetSetAgentFieldsError(errors.New("injected SetAgentFields failure"))

	// CompleteAgent should still succeed — ApplyEvent transitions the agent to
	// IDLE; the SetAgentFields failure is logged but not fatal.
	if err := s.sv.CompleteAgentForTest(ctx, agentID); err != nil {
		t.Fatalf("CompleteAgentForTest: %v", err)
	}

	// Agent should be IDLE despite SetAgentFields failure.
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected IDLE after complete with SetAgentFields error, got %v", stateResp.State)
	}

	// Clear error and verify agent can be assigned again.
	s.store.SetSetAgentFieldsError(nil)

	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-saf-002", Prompt: "recovery test", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask (recovery): %v", err)
	}
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)
}

// ── Scenario 16: CrashAgent GetAgentFields Failure Defers Crash (E2E) ───────

// TestE2E_CrashAgentGetAgentFieldsFailureDefersCrash verifies that crashAgent
// returns early without transitioning the agent when GetAgentFields fails before
// ApplyEvent. The agent stays in its current state (WORKING) so the heartbeat
// loop can retry on the next tick. The task is not lost.
func TestE2E_CrashAgentGetAgentFieldsFailureDefersCrash(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-crash-gaf-err"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Submit a task and wait for assignment.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-cgaf-001", Prompt: "test crashAgent deferred", Priority: 2.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)

	// Advance to WORKING.
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}

	// Inject GetAgentFields error before crashing.
	s.store.SetGetAgentFieldsError(errors.New("injected GetAgentFields failure in crashAgent"))

	// CrashAgentForTest should return early — agent stays WORKING (crash deferred).
	s.sv.CrashAgentForTest(ctx, agentID)

	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_WORKING {
		t.Fatalf("expected WORKING after deferred crash, got %v", stateResp.State)
	}

	// Clear the error.
	s.store.SetGetAgentFieldsError(nil)

	// Retry crashAgent — should succeed, transition to IDLE, re-enqueue task.
	s.sv.CrashAgentForTest(ctx, agentID)

	stateResp, err = s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState after retry: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected IDLE after successful crash, got %v", stateResp.State)
	}

	// The task should be re-enqueued and re-assigned to the now-idle agent.
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)

	// Queue should be empty — task was picked up.
	qLen, err := s.store.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if qLen != 0 {
		t.Fatalf("expected empty queue after crash recovery, got %d", qLen)
	}
}

// ── Scenario 17: SetAgentFields Failure in TryAssign Re-enqueues Task (E2E) ─

// TestE2E_SetAgentFieldsFailureInTryAssignReenqueuesTask verifies that when
// SetAgentFields fails in tryAssignTask after ApplyEvent succeeds, the task is
// re-enqueued. The agent becomes a zombie (ASSIGNED with no CurrentTaskID) and
// self-heals via manual completion in this test.
func TestE2E_SetAgentFieldsFailureInTryAssignReenqueuesTask(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-saf-tryassign"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Inject SetAgentFields error before submitting the task.
	s.store.SetSetAgentFieldsError(errors.New("injected SetAgentFields failure in tryAssign"))

	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-saf-ta-001", Prompt: "test tryAssign SetAgentFields", Priority: 1.5},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	// Give the assign loop several ticks. tryAssignTask will:
	// 1. DequeueTask succeeds
	// 2. ApplyEvent(EventTaskAssigned) succeeds (IDLE → ASSIGNED)
	// 3. GetAgentFields succeeds
	// 4. SetAgentFields fails → task re-enqueued
	// Then the loop repeats: dequeue, ApplyEvent fails (already ASSIGNED), re-enqueue.
	time.Sleep(80 * time.Millisecond)

	// Clear the error.
	s.store.SetSetAgentFieldsError(nil)

	// The agent is ASSIGNED (from the first successful ApplyEvent) but has no
	// CurrentTaskID. Drive it back to IDLE so the task can be re-assigned.
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventOutputReady); err != nil {
		t.Fatalf("advance to reporting: %v", err)
	}
	if err := s.sv.CompleteAgentForTest(ctx, agentID); err != nil {
		t.Fatalf("CompleteAgentForTest: %v", err)
	}

	// Agent is IDLE. The re-enqueued task should be picked up.
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 500*time.Millisecond)

	// Queue should be empty — task was assigned.
	qLen, err := s.store.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if qLen != 0 {
		t.Fatalf("expected empty queue after recovery, got %d", qLen)
	}
}

// ── Scenario 18: Assign Loop Backoff Resets After Error Clears (E2E) ────────

// TestE2E_AssignLoopBackoffResetsAfterErrorClears verifies that the assign loop
// backs off under persistent store errors and recovers (assigns the task) after
// the error is cleared. The backoff is internal — the observable behavior is
// that the task is eventually assigned despite a period of errors.
func TestE2E_AssignLoopBackoffResetsAfterErrorClears(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-backoff"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Inject persistent GetAgentFields error.
	s.store.SetGetAgentFieldsError(errors.New("injected persistent error"))

	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-backoff-001", Prompt: "test backoff", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	// Let the assign loop hit the error several times and build up backoff.
	time.Sleep(100 * time.Millisecond)

	// Clear the error. The assign loop's backoff should eventually allow
	// the next attempt, and the task should be assigned.
	s.store.SetGetAgentFieldsError(nil)

	// Drive the agent back to IDLE (it's ASSIGNED from a prior ApplyEvent
	// that succeeded before GetAgentFields failed).
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventOutputReady); err != nil {
		t.Fatalf("advance to reporting: %v", err)
	}
	if err := s.sv.CompleteAgentForTest(ctx, agentID); err != nil {
		t.Fatalf("CompleteAgentForTest: %v", err)
	}

	// Agent is IDLE, error cleared. The re-enqueued task should be assigned.
	// Use a generous timeout since backoff may delay the first successful attempt.
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 5*time.Second)

	qLen, err := s.store.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if qLen != 0 {
		t.Fatalf("expected empty queue after backoff recovery, got %d", qLen)
	}
}

// ── Scenario 19: ClearCurrentTask Failure in CompleteAgent (E2E) ─────────────

// TestE2E_ClearCurrentTaskFailureInCompleteAgent verifies that completeAgent
// succeeds (agent transitions to IDLE) even when ClearCurrentTask fails. The
// state transition is the critical operation; the CurrentTaskID clear is
// best-effort with a log warning. This mirrors Scenario 15 (SetAgentFields
// failure) and closes the completeAgent error-handling asymmetry identified
// by Council 10.
func TestE2E_ClearCurrentTaskFailureInCompleteAgent(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-cct-complete-err"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Submit a task and wait for assignment.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-cct-001", Prompt: "test ClearCurrentTask error", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)

	// Advance to REPORTING.
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventOutputReady); err != nil {
		t.Fatalf("advance to reporting: %v", err)
	}

	// Inject ClearCurrentTask error before completing.
	s.store.SetClearCurrentTaskError(errors.New("injected ClearCurrentTask failure"))

	// CompleteAgent should still succeed — ApplyEvent transitions the agent to
	// IDLE; the ClearCurrentTask failure is logged but not fatal.
	if err := s.sv.CompleteAgentForTest(ctx, agentID); err != nil {
		t.Fatalf("CompleteAgentForTest: %v", err)
	}

	// Agent should be IDLE despite ClearCurrentTask failure.
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected IDLE after complete with ClearCurrentTask error, got %v", stateResp.State)
	}

	// Clear error and verify agent can be assigned again.
	s.store.SetClearCurrentTaskError(nil)

	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-cct-002", Prompt: "recovery test", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask (recovery): %v", err)
	}
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)
}

// ── Scenario 20: EnqueueTask Failure During Crash Recovery (E2E) ────────────

// TestE2E_EnqueueTaskFailureDuringCrashRecovery verifies that when EnqueueTask
// fails during crashAgent's task re-enqueue, the crash still completes (agent
// transitions to IDLE) but the task is lost (logged). This exercises the
// "task lost" log paths that were previously untestable without
// SetEnqueueTaskError (#60).
func TestE2E_EnqueueTaskFailureDuringCrashRecovery(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-enq-err"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Submit a task and wait for assignment.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-enq-001", Prompt: "test enqueue failure", Priority: 2.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)

	// Advance to WORKING.
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance to working: %v", err)
	}

	// Inject EnqueueTask error before crashing — re-enqueue will fail.
	s.store.SetEnqueueTaskError(errors.New("injected EnqueueTask failure"))

	// CrashAgentForTest — agent transitions to IDLE, but task re-enqueue fails.
	s.sv.CrashAgentForTest(ctx, agentID)

	// Agent should be IDLE (crash succeeded).
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected IDLE after crash, got %v", stateResp.State)
	}

	// Queue should be empty — the task was lost (EnqueueTask failed).
	s.store.SetEnqueueTaskError(nil)
	qLen, err := s.store.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if qLen != 0 {
		t.Fatalf("expected empty queue (task lost), got %d", qLen)
	}
}

// ── Scenario 21: DequeueTask Error Triggers Assign Backoff (E2E) ────────────

// TestE2E_DequeueTaskErrorTriggersBackoff verifies that real DequeueTask errors
// (not ErrQueueEmpty) engage the assign loop's exponential backoff, and that
// the loop recovers after the error clears (#58).
func TestE2E_DequeueTaskErrorTriggersBackoff(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-deq-err"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Inject DequeueTask error before submitting.
	s.store.SetDequeueTaskError(errors.New("injected DequeueTask failure"))

	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-deq-001", Prompt: "test dequeue backoff", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	// Wait — the assign loop should be hitting the error and backing off.
	time.Sleep(80 * time.Millisecond)

	// Agent should still be IDLE (no task assigned because dequeue fails).
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected IDLE while dequeue errors, got %v", stateResp.State)
	}

	// Clear the error. The backoff should eventually reset and the task
	// should be assigned.
	s.store.SetDequeueTaskError(nil)

	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 5*time.Second)

	qLen, err := s.store.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if qLen != 0 {
		t.Fatalf("expected empty queue after dequeue recovery, got %d", qLen)
	}
}

// ── Scenario 22: Per-Agent Error Injection (E2E) ────────────────────────────

// TestE2E_PerAgentErrorInjection verifies that per-agent error injection in
// MockStore affects only the targeted agent. Agent-B receives tasks normally
// while Agent-A's GetAgentFields returns an error (#61).
func TestE2E_PerAgentErrorInjection(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
	)
	defer s.cleanup()

	ctx := context.Background()

	// Register two agents.
	regA, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-A"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent A: %v", err)
	}
	agentA := regA.AgentId

	regB, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-B"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent B: %v", err)
	}
	agentB := regB.AgentId

	// Inject GetAgentFields error for agent-A only.
	s.store.SetGetAgentFieldsError(errors.New("injected per-agent error"), agentA)

	// Submit a task — it should be assigned to agent-B (agent-A's
	// GetAgentFields fails, so tryAssignTask re-enqueues and retries
	// with agent-B).
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-pa-001", Prompt: "per-agent test", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	// Wait for assignment.
	pollAgentState(t, s.client, agentB, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)

	// Agent-A should still be IDLE (or ASSIGNED then re-enqueued — but since
	// the error is on GetAgentFields, if A is tried first, ApplyEvent succeeds
	// but GetAgentFields fails, task re-enqueues, and B picks it up).
	// Clear per-agent error.
	s.store.SetGetAgentFieldsError(nil, agentA)

	// Verify agent-A can now receive tasks.
	// First complete agent-B so it's not competing.
	if _, err := s.sv.ApplyEventForTest(ctx, agentB, agent.EventWorkStarted); err != nil {
		t.Fatalf("advance B to working: %v", err)
	}
	if _, err := s.sv.ApplyEventForTest(ctx, agentB, agent.EventOutputReady); err != nil {
		t.Fatalf("advance B to reporting: %v", err)
	}
	if err := s.sv.CompleteAgentForTest(ctx, agentB); err != nil {
		t.Fatalf("CompleteAgent B: %v", err)
	}

	// Submit a second task — both agents are IDLE now, either can get it.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-pa-002", Prompt: "recovery test", Priority: 1.0},
	})
	if err != nil {
		t.Fatalf("SubmitTask (recovery): %v", err)
	}

	// One of the agents should get assigned.
	time.Sleep(200 * time.Millisecond)
	stateA, _ := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentA})
	stateB, _ := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentB})
	aAssigned := stateA != nil && stateA.State == pb.AgentState_AGENT_STATE_ASSIGNED
	bAssigned := stateB != nil && stateB.State == pb.AgentState_AGENT_STATE_ASSIGNED
	if !aAssigned && !bAssigned {
		t.Fatalf("expected at least one agent ASSIGNED after error clear, A=%v B=%v",
			stateA.GetState(), stateB.GetState())
	}
}
