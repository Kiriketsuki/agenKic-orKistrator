package dag

import "context"

// TaskEnqueuer is the narrow interface StoreSubmitter needs.
// state.StateStore satisfies it implicitly.
type TaskEnqueuer interface {
	EnqueueTask(ctx context.Context, taskID string, priority float64) error
}

// StoreSubmitter adapts a TaskEnqueuer to the TaskSubmitter interface.
//
// MVP LIMITATION: This is an enqueue-only adapter. It does NOT provide
// completion-tracking semantics. When the DAG executor calls SubmitTask
// and receives nil, it calls MarkNodeCompleted — but this reflects
// enqueue success, not actual task execution by an agent. A production
// TaskSubmitter (provided by the model gateway epic) must block until
// the agent finishes the task so that DAG dependency ordering is real.
//
// Additionally, prompt and modelTier are accepted but not stored — full
// TaskSpec persistence is deferred to model gateway integration.
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
