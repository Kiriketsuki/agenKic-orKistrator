//go:build testenv

package supervisor

import (
	"context"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
)

// ApplyEventForTest exposes applyEvent for external test packages (e.g. e2e/).
// This file is excluded from production builds via the testenv build tag.
// Do not call from production code.
func (sv *Supervisor) ApplyEventForTest(ctx context.Context, agentID string, event agent.AgentEvent) (agent.AgentSnapshot, error) {
	return sv.applyEvent(ctx, agentID, event)
}
