//go:build testenv

package supervisor

import (
	"context"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
)

// ApplyEventForTest exposes applyEvent for test packages; prefer the public API
// in unit tests. For EventAgentFailed, prefer CrashAgentForTest — it integrates
// the restart policy. For EventOutputDelivered, prefer CompleteAgentForTest —
// it integrates the restart policy success path.
// This file is excluded from production builds via the testenv build tag.
func (sv *Supervisor) ApplyEventForTest(ctx context.Context, agentID string, event agent.AgentEvent) (agent.AgentSnapshot, error) {
	return sv.applyEvent(ctx, agentID, event)
}

// CrashAgentForTest exposes crashAgent for test packages.
// Unlike ApplyEventForTest(EventAgentFailed), this method also records the crash
// with the RestartPolicy and sets cooldown/circuit-breaker state.
func (sv *Supervisor) CrashAgentForTest(ctx context.Context, agentID string) {
	sv.crashAgent(ctx, agentID)
}

// CompleteAgentForTest exposes completeAgent for test packages.
// Unlike ApplyEventForTest(EventOutputDelivered), this method also calls
// policy.RecordSuccess and clears cooldown/circuit-breaker state.
func (sv *Supervisor) CompleteAgentForTest(ctx context.Context, agentID string) error {
	return sv.completeAgent(ctx, agentID)
}
