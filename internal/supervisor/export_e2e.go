//go:build testenv

package supervisor

import "context"

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
