package dag

import "context"

// TaskEnqueuer is the narrow interface StoreSubmitter needs.
// state.StateStore satisfies it implicitly.
type TaskEnqueuer interface {
	EnqueueTask(ctx context.Context, taskID string, priority float64) error
}

// StoreSubmitter adapts a TaskEnqueuer to the TaskSubmitter interface.
// MVP: prompt and modelTier are accepted but not stored — full TaskSpec
// persistence is deferred to model gateway integration.
type StoreSubmitter struct {
	enqueuer TaskEnqueuer
}

// NewStoreSubmitter returns a StoreSubmitter that delegates to the given enqueuer.
func NewStoreSubmitter(enqueuer TaskEnqueuer) *StoreSubmitter {
	return &StoreSubmitter{enqueuer: enqueuer}
}

// SubmitTask enqueues the task via the underlying store.
// prompt and modelTier are intentionally ignored in the MVP.
func (s *StoreSubmitter) SubmitTask(ctx context.Context, taskID, prompt, modelTier string, priority float64) error {
	return s.enqueuer.EnqueueTask(ctx, taskID, priority)
}
