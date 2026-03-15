package ipc

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func setupTestServer(t *testing.T) (pb.OrchestratorServiceClient, func()) {
	t.Helper()

	store := state.NewMockStore()
	machine := agent.NewMachine(store)
	policy := supervisor.NewRestartPolicy()
	sv := supervisor.NewSupervisor(machine, store, policy)

	submitter := dag.NewStoreSubmitter(store)
	executor := dag.NewExecutor(context.Background(), submitter)
	server := NewOrchestratorServer(sv, store, executor)

	buf := 1024 * 1024
	lis := bufconn.Listen(buf)

	grpcServer := grpc.NewServer()
	pb.RegisterOrchestratorServiceServer(grpcServer, server)

	go func() { _ = grpcServer.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	client := pb.NewOrchestratorServiceClient(conn)
	cleanup := func() {
		conn.Close()
		grpcServer.GracefulStop()
	}
	return client, cleanup
}

func TestRegisterAgent(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	resp, err := client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{
			Name:      "test-agent",
			ModelTier: "sonnet",
		},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	if resp.AgentId == "" {
		t.Fatal("expected non-empty agent_id")
	}
	t.Logf("registered agent: %s", resp.AgentId)
}

func TestSubmitTask(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	resp, err := client.SubmitTask(ctx, &pb.SubmitTaskRequest{
		Task: &pb.TaskSpec{
			TaskId:   "task-001",
			Prompt:   "summarize this document",
			Priority: 1.0,
		},
	})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}
	if resp.TaskId != "task-001" {
		t.Fatalf("expected task_id=task-001, got %s", resp.TaskId)
	}
}

func TestSubmitTask_NilSpec(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	_, err := client.SubmitTask(ctx, &pb.SubmitTaskRequest{})
	if err == nil {
		t.Fatal("expected error for nil task spec")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestGetAgentState(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register an agent first.
	regResp, err := client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "state-test"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}

	// Get the agent state — should be idle after registration.
	stateResp, err := client.GetAgentState(ctx, &pb.GetAgentStateRequest{
		AgentId: regResp.AgentId,
	})
	if err != nil {
		t.Fatalf("GetAgentState: %v", err)
	}
	if stateResp.AgentId != regResp.AgentId {
		t.Fatalf("expected agent_id=%s, got %s", regResp.AgentId, stateResp.AgentId)
	}
	if stateResp.State != pb.AgentState_AGENT_STATE_IDLE {
		t.Fatalf("expected AGENT_STATE_IDLE, got %v", stateResp.State)
	}
}

func TestGetAgentState_NotFound(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	_, err := client.GetAgentState(ctx, &pb.GetAgentStateRequest{
		AgentId: "nonexistent-agent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st.Code())
	}
}

func TestStreamOutput(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	stream, err := client.StreamOutput(ctx)
	if err != nil {
		t.Fatalf("StreamOutput: %v", err)
	}

	// Send a chunk.
	chunk := &pb.OutputChunk{
		AgentId: "agent-1",
		TaskId:  "task-1",
		Type:    pb.OutputType_OUTPUT_TYPE_STDOUT,
		Payload: []byte("hello world"),
	}
	if err := stream.Send(chunk); err != nil {
		t.Fatalf("Send: %v", err)
	}

	// Receive the ack.
	ack, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if !ack.Received {
		t.Fatal("expected ack.Received=true")
	}
	expectedChunkID := "agent-1:task-1"
	if ack.ChunkId != expectedChunkID {
		t.Fatalf("expected chunk_id=%s, got %s", expectedChunkID, ack.ChunkId)
	}

	// Close the send side and verify clean shutdown.
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}
}

// ── SubmitDAG tests ─────────────────────────────────────────────────────────

func twoNodeDAG() *pb.DAGSpec {
	return &pb.DAGSpec{
		DagId: "test-dag",
		Nodes: []*pb.DAGNode{
			{NodeId: "a", Task: &pb.TaskSpec{TaskId: "task-a", Prompt: "do A", Priority: 1.0}},
			{NodeId: "b", Task: &pb.TaskSpec{TaskId: "task-b", Prompt: "do B", Priority: 2.0}, DependsOn: []string{"a"}},
		},
	}
}

func TestSubmitDAG_Success(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := client.SubmitDAG(context.Background(), &pb.SubmitDAGRequest{
		Dag: twoNodeDAG(),
	})
	if err != nil {
		t.Fatalf("SubmitDAG: %v", err)
	}
	if resp.DagExecutionId == "" {
		t.Fatal("expected non-empty dag_execution_id")
	}
}

func TestSubmitDAG_NilSpec(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	_, err := client.SubmitDAG(context.Background(), &pb.SubmitDAGRequest{})
	if err == nil {
		t.Fatal("expected error for nil dag spec")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestSubmitDAG_EmptyNodes(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	_, err := client.SubmitDAG(context.Background(), &pb.SubmitDAGRequest{
		Dag: &pb.DAGSpec{DagId: "empty"},
	})
	if err == nil {
		t.Fatal("expected error for empty nodes")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestSubmitDAG_CycleDetected(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	_, err := client.SubmitDAG(context.Background(), &pb.SubmitDAGRequest{
		Dag: &pb.DAGSpec{
			DagId: "cyclic",
			Nodes: []*pb.DAGNode{
				{NodeId: "a", Task: &pb.TaskSpec{TaskId: "t-a", Prompt: "a", Priority: 1.0}, DependsOn: []string{"b"}},
				{NodeId: "b", Task: &pb.TaskSpec{TaskId: "t-b", Prompt: "b", Priority: 1.0}, DependsOn: []string{"a"}},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for cyclic DAG")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

// ── GetDAGStatus tests ──────────────────────────────────────────────────────

func TestGetDAGStatus_Success(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	submitResp, err := client.SubmitDAG(ctx, &pb.SubmitDAGRequest{Dag: twoNodeDAG()})
	if err != nil {
		t.Fatalf("SubmitDAG: %v", err)
	}

	statusResp, err := client.GetDAGStatus(ctx, &pb.GetDAGStatusRequest{
		DagExecutionId: submitResp.DagExecutionId,
	})
	if err != nil {
		t.Fatalf("GetDAGStatus: %v", err)
	}
	if statusResp.DagExecutionId != submitResp.DagExecutionId {
		t.Fatalf("expected id=%s, got %s", submitResp.DagExecutionId, statusResp.DagExecutionId)
	}
}

func TestGetDAGStatus_NotFound(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	_, err := client.GetDAGStatus(context.Background(), &pb.GetDAGStatusRequest{
		DagExecutionId: "nonexistent-execution",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent execution")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st.Code())
	}
}

func TestGetDAGStatus_EmptyID(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	_, err := client.GetDAGStatus(context.Background(), &pb.GetDAGStatusRequest{})
	if err == nil {
		t.Fatal("expected error for empty execution id")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestSubmitDAG_FullLifecycle(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	submitResp, err := client.SubmitDAG(ctx, &pb.SubmitDAGRequest{Dag: twoNodeDAG()})
	if err != nil {
		t.Fatalf("SubmitDAG: %v", err)
	}

	// Poll until COMPLETED or timeout.
	deadline := time.Now().Add(5 * time.Second)
	var statusResp *pb.GetDAGStatusResponse
	for time.Now().Before(deadline) {
		statusResp, err = client.GetDAGStatus(ctx, &pb.GetDAGStatusRequest{
			DagExecutionId: submitResp.DagExecutionId,
		})
		if err != nil {
			t.Fatalf("GetDAGStatus: %v", err)
		}
		if statusResp.State == pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED ||
			statusResp.State == pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if statusResp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Fatalf("expected COMPLETED, got %v", statusResp.State)
	}

	// Verify all nodes completed.
	for _, ns := range statusResp.NodeStatuses {
		if ns.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
			t.Fatalf("node %s: expected COMPLETED, got %v", ns.NodeId, ns.State)
		}
	}
}
