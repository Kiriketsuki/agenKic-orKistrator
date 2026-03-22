//go:build testenv

// Package e2e_test exercises the in-process orchestrator stack
// (MockStore -> Machine -> Supervisor -> DAG Executor -> gRPC via bufconn).
//
// Scenarios that use ApplyEventForTest to inject state transitions are
// marked [gRPC-bypassed]; full gRPC lifecycle coverage for those paths
// will follow once agent-side gRPC clients are implemented.
package e2e_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/ipc"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// ── Test Stack ────────────────────────────────────────────────────────────────

// testStack holds all wired-up components for an E2E test.
// Both gRPC client and internal components are exposed so tests can simulate
// agent-side behaviour (state transitions, heartbeat manipulation) that doesn't
// yet have a gRPC interface.
type testStack struct {
	client  pb.OrchestratorServiceClient
	store   *state.MockStore
	sv      *supervisor.Supervisor
	machine *agent.Machine
	policy  *supervisor.RestartPolicy
	cancel  context.CancelFunc
	cleanup func()
}

// newTestStack creates a fully wired orchestrator stack with a running supervisor
// and an in-process gRPC connection. Task polling is tight (20ms) for fast tests.
// Stale threshold is generous (10s) so heartbeat detection doesn't evict agents
// in tests that aren't testing heartbeat behaviour.
// Pass extra supervisor.SupervisorOption values to override (e.g. WithStaleThreshold).
func newTestStack(t *testing.T, svOpts []supervisor.SupervisorOption, policyOpts ...supervisor.RestartPolicyOption) *testStack {
	t.Helper()

	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := supervisor.NewRestartPolicy(policyOpts...)

	defaults := []supervisor.SupervisorOption{
		supervisor.WithHeartbeatInterval(20 * time.Millisecond),
		supervisor.WithStaleThreshold(10 * time.Second), // generous — won't fire in normal tests
		supervisor.WithTaskPollInterval(20 * time.Millisecond),
	}
	sv := supervisor.NewSupervisor(machine, store, policy, append(defaults, svOpts...)...)

	ctx, cancel := context.WithCancel(context.Background())

	submitter := dag.NewStoreSubmitter(store)
	executor := dag.NewExecutor(ctx, submitter)
	server := ipc.NewOrchestratorServer(sv, store, executor)

	lis := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	pb.RegisterOrchestratorServiceServer(grpcServer, server)

	servErr := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			servErr <- err
		}
		close(servErr)
	}()

	// Start the supervisor run loop in the background.
	go func() { _ = sv.Run(ctx) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cancel()
		t.Fatalf("grpc.NewClient: %v", err)
	}

	cleanup := func() {
		cancel()                  // 1. stop supervisor goroutines
		grpcServer.GracefulStop() // 2. drain gRPC server
		conn.Close()              // 3. close client connection
		executor.Shutdown()       // 4. stop DAG executor
		if err := <-servErr; err != nil {
			t.Errorf("grpcServer.Serve: %v", err)
		}
	}

	return &testStack{
		client:  pb.NewOrchestratorServiceClient(conn),
		store:   store,
		sv:      sv,
		machine: machine,
		policy:  policy,
		cancel:  cancel,
		cleanup: cleanup,
	}
}

// ── Poll Helpers ──────────────────────────────────────────────────────────────

// pollAgentState polls GetAgentState every 10 ms until the state matches
// expected or the timeout expires.
func pollAgentState(
	t *testing.T,
	client pb.OrchestratorServiceClient,
	agentID string,
	expected pb.AgentState,
	timeout time.Duration,
) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.GetAgentState(context.Background(), &pb.GetAgentStateRequest{AgentId: agentID})
		if err == nil && resp.State == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Final attempt so we get a meaningful failure message.
	resp, err := client.GetAgentState(context.Background(), &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("pollAgentState: GetAgentState error: %v", err)
	}
	if resp.State != expected {
		t.Fatalf("pollAgentState: timeout waiting for state %v — got %v", expected, resp.State)
	}
}

// pollDAGComplete polls GetDAGStatus every 20 ms until the execution reaches
// COMPLETED or FAILED, or the timeout expires.
func pollDAGComplete(
	t *testing.T,
	client pb.OrchestratorServiceClient,
	execID string,
	timeout time.Duration,
) *pb.GetDAGStatusResponse {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.GetDAGStatus(context.Background(), &pb.GetDAGStatusRequest{DagExecutionId: execID})
		if err != nil {
			t.Logf("GetDAGStatus error (will retry): %v", err)
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if resp.State == pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED ||
			resp.State == pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED {
			return resp
		}
		time.Sleep(20 * time.Millisecond)
	}
	resp, err := client.GetDAGStatus(context.Background(), &pb.GetDAGStatusRequest{DagExecutionId: execID})
	if err != nil {
		t.Fatalf("pollDAGComplete: GetDAGStatus error: %v", err)
	}
	t.Fatalf("pollDAGComplete: timeout — final state %v", resp.State)
	return nil
}

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

// ── Scenario 6: Linear DAG ───────────────────────────────────────────────────

// TestE2E_LinearDAG verifies that a three-node linear DAG (A → B → C) completes
// with all nodes in the COMPLETED state.
// MVP: nodes complete on successful enqueue — full agent execution deferred to model gateway integration.
func TestE2E_LinearDAG(t *testing.T) {
	s := newTestStack(t, nil)
	defer s.cleanup()

	spec := &pb.DAGSpec{
		DagId: "linear-dag",
		Nodes: []*pb.DAGNode{
			{NodeId: "a", Task: &pb.TaskSpec{TaskId: "t-a", Prompt: "step A", Priority: 1.0}},
			{NodeId: "b", Task: &pb.TaskSpec{TaskId: "t-b", Prompt: "step B", Priority: 1.0}, DependsOn: []string{"a"}},
			{NodeId: "c", Task: &pb.TaskSpec{TaskId: "t-c", Prompt: "step C", Priority: 1.0}, DependsOn: []string{"b"}},
		},
	}

	ctx := context.Background()
	submitResp, err := s.client.SubmitDAG(ctx, &pb.SubmitDAGRequest{Dag: spec})
	if err != nil {
		t.Fatalf("SubmitDAG: %v", err)
	}

	statusResp := pollDAGComplete(t, s.client, submitResp.DagExecutionId, 5*time.Second)
	if statusResp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Fatalf("expected COMPLETED, got %v", statusResp.State)
	}

	for _, ns := range statusResp.NodeStatuses {
		if ns.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
			t.Fatalf("node %s: expected COMPLETED, got %v", ns.NodeId, ns.State)
		}
	}
}

// ── Scenario 7: Parallel Fork DAG ────────────────────────────────────────────

// TestE2E_ParallelForkDAG verifies that a fork-join DAG (A → {B, C} → D)
// completes with all four nodes in the COMPLETED state.  B and C execute in
// parallel after A, and D starts only after both B and C finish.
// MVP: nodes complete on successful enqueue — full agent execution deferred to model gateway integration.
func TestE2E_ParallelForkDAG(t *testing.T) {
	s := newTestStack(t, nil)
	defer s.cleanup()

	spec := &pb.DAGSpec{
		DagId: "fork-dag",
		Nodes: []*pb.DAGNode{
			{NodeId: "a", Task: &pb.TaskSpec{TaskId: "t-a", Prompt: "root", Priority: 1.0}},
			{NodeId: "b", Task: &pb.TaskSpec{TaskId: "t-b", Prompt: "fork-left", Priority: 1.0}, DependsOn: []string{"a"}},
			{NodeId: "c", Task: &pb.TaskSpec{TaskId: "t-c", Prompt: "fork-right", Priority: 1.0}, DependsOn: []string{"a"}},
			{NodeId: "d", Task: &pb.TaskSpec{TaskId: "t-d", Prompt: "join", Priority: 1.0}, DependsOn: []string{"b", "c"}},
		},
	}

	ctx := context.Background()
	submitResp, err := s.client.SubmitDAG(ctx, &pb.SubmitDAGRequest{Dag: spec})
	if err != nil {
		t.Fatalf("SubmitDAG: %v", err)
	}

	statusResp := pollDAGComplete(t, s.client, submitResp.DagExecutionId, 5*time.Second)
	if statusResp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Fatalf("expected COMPLETED, got %v", statusResp.State)
	}

	nodeMap := make(map[string]pb.DAGExecutionState, len(statusResp.NodeStatuses))
	for _, ns := range statusResp.NodeStatuses {
		nodeMap[ns.NodeId] = ns.State
	}

	for _, id := range []string{"a", "b", "c", "d"} {
		if nodeMap[id] != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
			t.Fatalf("node %s: expected COMPLETED, got %v", id, nodeMap[id])
		}
	}
}

// ── Scenario 8: Crash Cycle Policy Backoff [gRPC-bypassed] ───────────────

// TestCrashCycle_PolicyBackoff verifies that the RestartPolicy correctly computes
// exponential backoff after consecutive agent crashes driven through the full stack.
// This tests policy arithmetic in the E2E stack context — supervisor-enforced
// cooldown is tested in TestE2E_CooldownEnforcement.
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
// This tests policy arithmetic in the E2E stack context — supervisor-enforced
// circuit breaker is tested in TestE2E_CircuitBreakerBlocksAssignment.
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

// ── Scenario 19: GetAgentFields Failure in CompleteAgent (E2E) ───────────────

// TestE2E_GetAgentFieldsFailureInCompleteAgent verifies that completeAgent
// succeeds (agent transitions to IDLE) even when GetAgentFields fails. The
// state transition is the critical operation; the CurrentTaskID clear is
// best-effort with a log warning. This mirrors Scenario 15 (SetAgentFields
// failure) and closes the completeAgent error-handling asymmetry identified
// by Council 10.
func TestE2E_GetAgentFieldsFailureInCompleteAgent(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{supervisor.WithTaskPollInterval(10 * time.Millisecond)},
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-getfields-complete-err"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	// Submit a task and wait for assignment.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-gfc-001", Prompt: "test getfields complete error", Priority: 1.0},
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

	// Inject GetAgentFields error before completing.
	s.store.SetGetAgentFieldsError(errors.New("injected GetAgentFields failure in completeAgent"))

	// CompleteAgent should still succeed — ApplyEvent transitions the agent to
	// IDLE; the GetAgentFields failure is logged but not fatal.
	if err := s.sv.CompleteAgentForTest(ctx, agentID); err != nil {
		t.Fatalf("CompleteAgentForTest: %v", err)
	}

	// Agent should be IDLE despite GetAgentFields failure.
	// Note: GetAgentFields error is still injected, but GetAgentState uses
	// the state machine (not GetAgentFields), so this check works.
	s.store.SetGetAgentFieldsError(nil)
	stateResp, err := s.client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected IDLE after complete with GetAgentFields error, got %v", stateResp.State)
	}

	// Verify agent can be assigned again after error clears.
	_, err = s.client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{TaskId: "task-gfc-002", Prompt: "recovery test", Priority: 1.0},
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
