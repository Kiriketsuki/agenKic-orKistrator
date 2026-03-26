package state

import (
	"errors"
	"fmt"
)

// ErrAgentNotFound is returned when an agent ID has no record in the store.
var ErrAgentNotFound = errors.New("agent not found")

// ErrQueueEmpty is returned when DequeueTask is called on an empty task queue.
var ErrQueueEmpty = errors.New("task queue is empty")

// StateConflictError is returned by CompareAndSetAgentState when the current
// state does not match the expected value. Callers can inspect Expected and
// Actual to decide whether to retry.
type StateConflictError struct {
	Expected string
	Actual   string
}

func (e *StateConflictError) Error() string {
	return fmt.Sprintf("state conflict: expected %q, got %q", e.Expected, e.Actual)
}
