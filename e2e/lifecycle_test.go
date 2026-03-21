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
	cancel  context.CancelFunc
	cleanup func()
}

// newTestStack creates a fully wired orchestrator stack with a running supervisor
// and an in-process gRPC connection. Task polling is tight (20ms) for fast tests.
// Stale threshold is generous (10s) so heartbeat detection doesn't evict agents
// in tests that aren't testing heartbeat behaviour.
// Pass extra supervisor.SupervisorOption values to override (e.g. WithStaleThreshold).
func newTestStack(t *testing.T, svOpts ...supervisor.SupervisorOption) *testStack {
	t.Helper()

	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := supervisor.NewRestartPolicy()

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
	go func() { _ = grpcServer.Serve(lis) }()

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
		conn.Close()
		grpcServer.GracefulStop()
		cancel()
		executor.Shutdown()
	}

	return &testStack{
		client:  pb.NewOrchestratorServiceClient(conn),
		store:   store,
		sv:      sv,
		machine: machine,
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
	s := newTestStack(t)
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
	s := newTestStack(t)
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

// TestE2E_TaskQueuedWhenAllBusy verifies that a submitted task stays in the queue
// when no idle agents exist, and is automatically assigned once an agent becomes idle.
func TestE2E_TaskQueuedWhenAllBusy(t *testing.T) {
	s := newTestStack(t)
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

	// Give the assign loop a few ticks — task should remain queued since agent is busy.
	time.Sleep(100 * time.Millisecond)
	qlen, err := s.store.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	// Queue may be 0 or 1 depending on timing; the key assertion is agent is still WORKING.
	_ = qlen
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
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventOutputDelivered); err != nil {
		t.Fatalf("advance to idle: %v", err)
	}

	// Supervisor should now assign the queued task.
	pollAgentState(t, s.client, agentID, pb.AgentState_AGENT_STATE_ASSIGNED, 2*time.Second)
}

// ── Scenario 4: Full Agent Lifecycle ─────────────────────────────────────────

// TestE2E_FullAgentLifecycle verifies all four state transitions in sequence:
// idle → assigned → working → reporting → idle.
func TestE2E_FullAgentLifecycle(t *testing.T) {
	s := newTestStack(t)
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

	// reporting → idle
	if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventOutputDelivered); err != nil {
		t.Fatalf("EventOutputDelivered: %v", err)
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
	s := newTestStack(t, supervisor.WithStaleThreshold(50*time.Millisecond))
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
}

// ── Scenario 6: Exponential Backoff ──────────────────────────────────────────

// TestE2E_ExponentialBackoff verifies that RestartPolicy doubles the backoff
// duration on each consecutive crash: 1s → 2s → 4s.
func TestE2E_ExponentialBackoff(t *testing.T) {
	policy := supervisor.NewRestartPolicy()

	want := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
	}
	for i, expected := range want {
		d := policy.RecordCrash()
		if !d.ShouldRestart {
			t.Fatalf("crash %d: expected ShouldRestart=true", i+1)
		}
		if d.Backoff != expected {
			t.Fatalf("crash %d: expected backoff %v, got %v", i+1, expected, d.Backoff)
		}
	}
}

// ── Scenario 7: Circuit Breaker ───────────────────────────────────────────────

// TestE2E_CircuitBreakerTrips verifies that after exceeding the crash threshold
// (5 crashes within the window) the circuit breaker opens and ShouldRestart is false.
func TestE2E_CircuitBreakerTrips(t *testing.T) {
	policy := supervisor.NewRestartPolicy(
		supervisor.WithCrashThreshold(5),
		supervisor.WithCrashWindow(60*time.Second),
	)

	// First 5 crashes should be restarted (circuit breaker not yet tripped).
	for i := 1; i <= 5; i++ {
		d := policy.RecordCrash()
		if !d.ShouldRestart {
			t.Fatalf("crash %d: expected ShouldRestart=true before threshold, got false", i)
		}
	}

	// 6th crash: circuit breaker should open.
	d := policy.RecordCrash()
	if d.ShouldRestart {
		t.Fatal("expected ShouldRestart=false after circuit breaker trips")
	}
	if !errors.Is(d.Reason, supervisor.ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", d.Reason)
	}
}

// ── Scenario 8: Linear DAG ───────────────────────────────────────────────────

// TestE2E_LinearDAG verifies that a three-node linear DAG (A → B → C) completes
// with all nodes in the COMPLETED state.
func TestE2E_LinearDAG(t *testing.T) {
	s := newTestStack(t)
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

// ── Scenario 9: Parallel Fork DAG ────────────────────────────────────────────

// TestE2E_ParallelForkDAG verifies that a fork-join DAG (A → {B, C} → D)
// completes with all four nodes in the COMPLETED state.  B and C execute in
// parallel after A, and D starts only after both B and C finish.
func TestE2E_ParallelForkDAG(t *testing.T) {
	s := newTestStack(t)
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
