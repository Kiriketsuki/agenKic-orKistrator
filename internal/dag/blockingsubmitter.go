package dag

import (
	"context"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
)

// BlockingSubmitter implements TaskSubmitter by enqueuing a task and then
// blocking until the supervisor signals completion via the CompletionRegistry.
type BlockingSubmitter struct {
	enqueuer TaskEnqueuer
	registry *supervisor.CompletionRegistry
}

// NewBlockingSubmitter creates a BlockingSubmitter.
func NewBlockingSubmitter(enqueuer TaskEnqueuer, registry *supervisor.CompletionRegistry) *BlockingSubmitter {
	return &BlockingSubmitter{enqueuer: enqueuer, registry: registry}
}

// SubmitTask enqueues the task and blocks until an agent completes it.
func (b *BlockingSubmitter) SubmitTask(ctx context.Context, taskID, prompt, modelTier string, priority float64) error {
	if err := b.enqueuer.EnqueueTask(ctx, taskID, priority); err != nil {
		return err
	}
	err := b.registry.Wait(ctx, taskID)
	b.registry.Cleanup(taskID)
	return err
}
