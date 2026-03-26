package dag

import "context"

// TaskSubmitter is the interface for submitting individual tasks for execution.
// F2's supervisor will satisfy this interface at integration time.
// F3 tests use a mock implementation.
type TaskSubmitter interface {
	SubmitTask(ctx context.Context, taskID, prompt, modelTier string, priority float64) error
}
