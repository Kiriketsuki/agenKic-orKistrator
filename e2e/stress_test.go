//go:build testenv

package e2e_test

import (
	"context"
	"sync"
	"testing"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
)

// ── Stress Test: Concurrent crashAgent + completeAgent (#64) [gRPC-bypassed] ─

// TestStress_ConcurrentCrashAndComplete launches concurrent crashAgent and
// completeAgent calls on the same agent and verifies the agent always ends in
// a valid state with no panics. Runs with -race.
func TestStress_ConcurrentCrashAndComplete(t *testing.T) {
	s := newTestStack(t,
		[]supervisor.SupervisorOption{
			supervisor.WithTaskPollInterval(10 * time.Millisecond),
			supervisor.WithStaleThreshold(10 * time.Second),
		},
	)
	defer s.cleanup()

	ctx := context.Background()
	regResp, err := s.client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		Info: &pb.AgentInfo{Name: "agent-stress"},
	})
	if err != nil {
		t.Fatalf("RegisterAgent: %v", err)
	}
	agentID := regResp.AgentId

	const iterations = 50
	for i := 0; i < iterations; i++ {
		// Put agent in REPORTING state (valid for both crash and complete).
		if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventTaskAssigned); err != nil {
			// May fail if agent is already ASSIGNED — that's fine in stress.
			continue
		}
		if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventWorkStarted); err != nil {
			continue
		}
		if _, err := s.sv.ApplyEventForTest(ctx, agentID, agent.EventOutputReady); err != nil {
			continue
		}

		// Launch crash and complete concurrently.
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.sv.CrashAgentForTest(ctx, agentID)
		}()
		go func() {
			defer wg.Done()
			_ = s.sv.CompleteAgentForTest(ctx, agentID)
		}()
		wg.Wait()

		// Agent must be in a valid state (IDLE is expected from either path).
		stateStr, err := s.store.GetAgentState(ctx, agentID)
		if err != nil {
			t.Fatalf("iteration %d: GetAgentState: %v", i, err)
		}
		if stateStr != "idle" {
			t.Fatalf("iteration %d: expected idle, got %q", i, stateStr)
		}
	}
}
