package ipc

import (
	"context"
	"io"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RegisterAgent generates a UUID, registers it with the supervisor, returns the ID.
func (s *OrchestratorServer) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	id := uuid.New().String()
	if err := s.supervisor.RegisterAgent(ctx, id); err != nil {
		return nil, status.Errorf(codes.Internal, "register agent: %v", err)
	}
	return &pb.RegisterAgentResponse{AgentId: id}, nil
}

// SubmitTask validates the request and enqueues the task.
func (s *OrchestratorServer) SubmitTask(ctx context.Context, req *pb.SubmitTaskRequest) (*pb.SubmitTaskResponse, error) {
	if req.GetTask() == nil {
		return nil, status.Error(codes.InvalidArgument, "task spec is required")
	}
	task := req.GetTask()
	if task.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}
	if err := s.store.EnqueueTask(ctx, task.TaskId, task.Priority); err != nil {
		return nil, status.Errorf(codes.Internal, "enqueue task: %v", err)
	}
	return &pb.SubmitTaskResponse{TaskId: task.TaskId}, nil
}

// GetAgentState looks up state via store, converts to proto.
func (s *OrchestratorServer) GetAgentState(ctx context.Context, req *pb.GetAgentStateRequest) (*pb.GetAgentStateResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	rawState, err := s.store.GetAgentState(ctx, req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "agent %s: %v", req.AgentId, err)
	}
	parsed, err := agent.ParseAgentState(rawState)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "parse state for %s: %v", req.AgentId, err)
	}
	return &pb.GetAgentStateResponse{
		AgentId: req.AgentId,
		State:   AgentStateToProto(parsed),
	}, nil
}

// StreamOutput receives OutputChunk messages and sends OutputAck replies.
// Basic MVP: ack each chunk with received=true.
func (s *OrchestratorServer) StreamOutput(stream grpc.BidiStreamingServer[pb.OutputChunk, pb.OutputAck]) error {
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		ack := &pb.OutputAck{
			ChunkId:  chunk.AgentId + ":" + chunk.TaskId,
			Received: true,
		}
		if err := stream.Send(ack); err != nil {
			return err
		}
	}
}
