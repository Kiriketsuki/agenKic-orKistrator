//go:build testenv

package supervisor

import (
	"context"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
)

// CrashAgentForTest exposes crashAgent for test packages.
// It records the crash with the RestartPolicy and sets cooldown/circuit-breaker state.
func (sv *Supervisor) CrashAgentForTest(ctx context.Context, agentID string) {
	sv.crashAgent(ctx, agentID)
}

// CompleteAgentForTest exposes completeAgent for test packages.
// It calls policy.RecordSuccess and clears cooldown/circuit-breaker state.
func (sv *Supervisor) CompleteAgentForTest(ctx context.Context, agentID string) error {
	return sv.completeAgent(ctx, agentID)
}

// ApplyEventForTest exposes machine.ApplyEvent for test packages.
// It drives an agent through a state transition, bypassing supervisor gRPC handlers,
// which is useful for setting up E2E test preconditions.
func (sv *Supervisor) ApplyEventForTest(ctx context.Context, agentID string, event agent.AgentEvent) (agent.AgentSnapshot, error) {
	return sv.machine.ApplyEvent(ctx, agentID, event)
}
