package state

import "errors"

// ErrAgentNotFound is returned when an agent ID has no record in the store.
var ErrAgentNotFound = errors.New("agent not found")

// ErrQueueEmpty is returned when DequeueTask is called on an empty task queue.
var ErrQueueEmpty = errors.New("task queue is empty")
