package ipc

import (
	"context"
	"errors"
	"io"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Handler routing rules:
//
//   Supervisor-routed (agent state transitions):
//     RegisterAgent  — creates agent record, initializes per-agent mutex
//     CompleteAgent   — CAS state transition, policy update, cooldown/circuit reset
//
//   Store-direct (stateless queue/read operations):
//     SubmitTask     — appends to task queue; no agent state involved
//     GetAgentState  — pure read; no coordination needed
//
//   DAG engine:
//     SubmitDAG      — delegates to DAG executor
//     GetDAGStatus   — reads DAG execution state from status tracker
//
//   Streaming:
//     StreamOutput   — bidi stream; MVP echo-ack (no supervisor interaction)
//
// Queue writes (SubmitTask) bypass the supervisor because they are append-only
// operations on a shared buffer. The supervisor's role is to dequeue and assign
// tasks to agents, not to gate task submission. Reads (GetAgentState) are
// stateless and need no coordination.

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

// SubmitDAG validates the DAG spec and delegates execution to the DAG engine.
func (s *OrchestratorServer) SubmitDAG(ctx context.Context, req *pb.SubmitDAGRequest) (*pb.SubmitDAGResponse, error) {
	if req.GetDag() == nil {
		return nil, status.Error(codes.InvalidArgument, "dag spec is required")
	}

	execID, err := s.dag.Execute(ctx, req.GetDag())
	if err != nil {
		if errors.Is(err, dag.ErrEmptyDAG) ||
			errors.Is(err, dag.ErrCycleDetected) ||
			errors.Is(err, dag.ErrNodeNotFound) ||
			errors.Is(err, dag.ErrDuplicateNode) ||
			errors.Is(err, dag.ErrMissingTaskSpec) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid dag: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "execute dag: %v", err)
	}

	return &pb.SubmitDAGResponse{DagExecutionId: execID}, nil
}

// CompleteAgent signals that an agent has finished its task (REPORTING → IDLE).
func (s *OrchestratorServer) CompleteAgent(ctx context.Context, req *pb.CompleteAgentRequest) (*pb.CompleteAgentResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if err := s.supervisor.CompleteAgent(ctx, req.AgentId); err != nil {
		return nil, status.Errorf(codes.Internal, "complete agent %s: %v", req.AgentId, err)
	}
	return &pb.CompleteAgentResponse{}, nil
}

// GetDAGStatus returns the current execution state of a DAG.
func (s *OrchestratorServer) GetDAGStatus(ctx context.Context, req *pb.GetDAGStatusRequest) (*pb.GetDAGStatusResponse, error) {
	if req.DagExecutionId == "" {
		return nil, status.Error(codes.InvalidArgument, "dag_execution_id is required")
	}

	resp, err := s.dag.Status(ctx, req.DagExecutionId)
	if err != nil {
		if errors.Is(err, dag.ErrExecutionNotFound) {
			return nil, status.Errorf(codes.NotFound, "execution %s not found", req.DagExecutionId)
		}
		return nil, status.Errorf(codes.Internal, "get dag status: %v", err)
	}

	return resp, nil
}
