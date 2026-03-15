package ipc

import (
	"context"
	"net"
	"testing"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
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

	server := NewOrchestratorServer(sv, store)

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
