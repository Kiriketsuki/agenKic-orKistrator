package ipc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Handler routing rules:
//
//   Supervisor-routed (agent state transitions):
//     RegisterAgent  — creates agent record, initializes per-agent mutex
//     StartWork      — ASSIGNED → WORKING
//     ReportOutput   — WORKING → REPORTING
//     CompleteAgent  — REPORTING → IDLE; policy update, cooldown/circuit reset
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
//     StreamOutput   — bidi stream; publishes chunks to event stream,
//                      triggers WORKING→REPORTING on FINAL chunk
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

// StreamOutput receives OutputChunk messages, publishes each to the event
// stream, triggers working→reporting on FINAL chunks, and acks every chunk.
func (s *OrchestratorServer) StreamOutput(stream grpc.BidiStreamingServer[pb.OutputChunk, pb.OutputAck]) error {
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// 1. Publish chunk to event stream.
		serialized, marshalErr := proto.Marshal(chunk)
		if marshalErr != nil {
			log.Printf("StreamOutput: marshal chunk failed: %v", marshalErr)
		} else {
			ts := chunk.Timestamp
			if ts == 0 {
				ts = time.Now().UnixMilli()
			}
			_ = s.store.PublishEvent(stream.Context(), state.Event{
				Type:      "output_chunk",
				AgentID:   chunk.AgentId,
				TaskID:    chunk.TaskId,
				Timestamp: ts,
				Payload:   string(serialized),
			})
		}

		// 2. On FINAL chunk, trigger working→reporting transition.
		// If the transition fails, close the stream with an error so the
		// agent has a recovery signal (rather than receiving a silent success ack).
		//
		// Ack-loss recovery: if ReportOutput succeeds but the subsequent ack
		// send fails (stream.Send at step 3), the agent's stream is closed
		// with the agent already in REPORTING state. The agent should treat
		// stream closure after sending a FINAL chunk as an implicit success
		// signal and proceed to call CompleteAgent. If the agent is uncertain,
		// it can re-send FINAL — a FailedPrecondition response confirms the
		// transition already occurred.
		if chunk.Type == pb.OutputType_OUTPUT_TYPE_FINAL {
			if err := s.supervisor.ReportOutput(stream.Context(), chunk.AgentId); err != nil {
				log.Printf("StreamOutput: ReportOutput failed for agent %s: %v", chunk.AgentId, err)
				return status.Errorf(codes.Internal, "ReportOutput failed for agent %s: %v", chunk.AgentId, err)
			}
		}

		// 3. Send ack with sequence-encoded chunk ID.
		chunkID := fmt.Sprintf("%s:%s:%d", chunk.AgentId, chunk.TaskId, chunk.Sequence)
		if err := stream.Send(&pb.OutputAck{
			ChunkId:  chunkID,
			Received: true,
		}); err != nil {
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

// StartWork signals that an agent has begun executing its task (ASSIGNED → WORKING).
func (s *OrchestratorServer) StartWork(ctx context.Context, req *pb.StartWorkRequest) (*pb.StartWorkResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if err := s.supervisor.StartWork(ctx, req.AgentId); err != nil {
		var invalidTx *agent.InvalidTransitionError
		if errors.As(err, &invalidTx) {
			return nil, status.Errorf(codes.FailedPrecondition, "start work: %v", err)
		}
		if errors.Is(err, state.ErrAgentNotFound) {
			return nil, status.Errorf(codes.NotFound, "agent %s not found", req.AgentId)
		}
		return nil, status.Errorf(codes.Internal, "start work: %v", err)
	}
	return &pb.StartWorkResponse{}, nil
}

// ReportOutput signals that an agent has output ready (WORKING → REPORTING).
func (s *OrchestratorServer) ReportOutput(ctx context.Context, req *pb.ReportOutputRequest) (*pb.ReportOutputResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if err := s.supervisor.ReportOutput(ctx, req.AgentId); err != nil {
		var invalidTx *agent.InvalidTransitionError
		if errors.As(err, &invalidTx) {
			return nil, status.Errorf(codes.FailedPrecondition, "report output: %v", err)
		}
		if errors.Is(err, state.ErrAgentNotFound) {
			return nil, status.Errorf(codes.NotFound, "agent %s not found", req.AgentId)
		}
		return nil, status.Errorf(codes.Internal, "report output: %v", err)
	}
	return &pb.ReportOutputResponse{}, nil
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

// Heartbeat refreshes the agent's liveness timestamp.
func (s *OrchestratorServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if req.AgentId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "agent_id is required")
	}
	if err := s.supervisor.Heartbeat(ctx, req.AgentId); err != nil {
		if errors.Is(err, state.ErrAgentNotFound) {
			return nil, status.Errorf(codes.NotFound, "agent %s not found", req.AgentId)
		}
		return nil, status.Errorf(codes.Internal, "heartbeat: %v", err)
	}
	return &pb.HeartbeatResponse{}, nil
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
