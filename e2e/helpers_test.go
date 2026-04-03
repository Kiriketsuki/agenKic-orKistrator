//go:build testenv

// Package e2e_test exercises the in-process orchestrator stack
// (MockStore → Machine → Supervisor → DAG Executor → gRPC via bufconn).
//
// Three test-only shim functions from internal/supervisor/export_e2e.go are
// used throughout these tests:
//
//   - ApplyEventForTest: injects a state-machine event directly on the
//     supervisor (bypasses gRPC). Used in most scenarios as precondition
//     setup — advancing an agent to a specific state before testing something
//     else. Scenarios 8 and 9 use it as the behaviour under test because no
//     agent-initiated state-transition RPCs exist yet (see #73).
//
//   - CrashAgentForTest: triggers the supervisor's internal crashAgent path
//     directly (bypasses gRPC). Used in crash-recovery, cooldown, circuit-
//     breaker, and stress-test scenarios.
//
//   - CompleteAgentForTest: triggers the supervisor's internal completeAgent
//     path directly (bypasses gRPC). Used in scenarios that need to cycle an
//     agent back to idle without a CompleteAgent RPC round-trip.
//
// Scenarios whose primary behaviour under test is driven entirely through
// these shims (rather than gRPC) are marked [gRPC-bypassed] in their doc
// comment. The remaining scenarios use the shims only for precondition setup
// while testing gRPC-reachable behaviour (registration, task assignment,
// heartbeat detection, DAG submission, error re-enqueue paths).
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
