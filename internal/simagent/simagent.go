// Package simagent runs a simulated worker agent against the orchestrator's
// own gRPC API: register, heartbeat, poll for assignment, then
// StartWork → stream output chunks → ReportOutput → CompleteAgent.
// Demo/dev scaffolding — real agents are external processes.
package simagent

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	heartbeatInterval = 3 * time.Second
	pollInterval      = 500 * time.Millisecond
	workSteps         = 5
	stepDuration      = 1200 * time.Millisecond
)

// Spawn dials addr, registers one agent, and runs its worker loop in a
// goroutine until ctx is cancelled. Returns the assigned agent ID.
func Spawn(ctx context.Context, addr, name, tier string) (string, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", fmt.Errorf("simagent dial %s: %w", addr, err)
	}
	client := pb.NewOrchestratorServiceClient(conn)

	resp, err := client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: name, ModelTier: tier},
	})
	if err != nil {
		_ = conn.Close()
		return "", fmt.Errorf("simagent register %q: %w", name, err)
	}

	go func() {
		defer conn.Close()
		Run(ctx, client, resp.AgentId, name)
	}()
	return resp.AgentId, nil
}

// Run drives the worker loop for an already-registered agent until ctx is
// cancelled.
func Run(ctx context.Context, client pb.OrchestratorServiceClient, agentID, name string) {
	heartbeat := time.NewTicker(heartbeatInterval)
	poll := time.NewTicker(pollInterval)
	defer heartbeat.Stop()
	defer poll.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := client.Heartbeat(ctx, &pb.HeartbeatRequest{AgentId: agentID}); err != nil {
				log.Printf("simagent [%s] heartbeat: %v", name, err)
			}
		case <-poll.C:
			st, err := client.GetAgentState(ctx, &pb.GetAgentStateRequest{AgentId: agentID})
			if err != nil {
				log.Printf("simagent [%s] state: %v", name, err)
				continue
			}
			if st.State == pb.AgentState_AGENT_STATE_ASSIGNED {
				workTask(ctx, client, agentID, name)
			}
		}
	}
}

func workTask(ctx context.Context, client pb.OrchestratorServiceClient, agentID, name string) {
	if _, err := client.StartWork(ctx, &pb.StartWorkRequest{AgentId: agentID}); err != nil {
		log.Printf("simagent [%s] StartWork: %v", name, err)
		return
	}
	log.Printf("simagent [%s] working...", name)

	stream, err := client.StreamOutput(ctx)
	if err == nil {
		for i := 0; i < workSteps; i++ {
			chunk := &pb.OutputChunk{
				AgentId:   agentID,
				Type:      pb.OutputType_OUTPUT_TYPE_STDOUT,
				Payload:   []byte(fmt.Sprintf("[%s] casting spell... step %d/%d\n", name, i+1, workSteps)),
				Timestamp: time.Now().UnixMilli(),
				Sequence:  int64(i),
			}
			if err := stream.Send(chunk); err != nil {
				break
			}
			select {
			case <-ctx.Done():
				_ = stream.CloseSend()
				return
			case <-time.After(stepDuration):
			}
		}
		_ = stream.CloseSend()
	}

	if _, err := client.ReportOutput(ctx, &pb.ReportOutputRequest{AgentId: agentID}); err != nil {
		log.Printf("simagent [%s] ReportOutput: %v", name, err)
	}
	if _, err := client.CompleteAgent(ctx, &pb.CompleteAgentRequest{AgentId: agentID}); err != nil {
		log.Printf("simagent [%s] CompleteAgent: %v", name, err)
	}
	log.Printf("simagent [%s] task complete, back to idle", name)
}
