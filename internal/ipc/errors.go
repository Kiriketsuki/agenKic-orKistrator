package ipc

import "errors"

// ErrAgentNotRegistered is returned when an operation references an agent not in the pool.
var ErrAgentNotRegistered = errors.New("agent not registered")
