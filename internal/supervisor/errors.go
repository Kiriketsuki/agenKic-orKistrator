package supervisor

import "errors"

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open")

// ErrSupervisorStopped is returned when operations are attempted on a stopped supervisor.
var ErrSupervisorStopped = errors.New("supervisor stopped")

// ErrInvalidAgentID is returned when an agentID is empty or exceeds 128 characters.
var ErrInvalidAgentID = errors.New("invalid agent ID: must be non-empty and at most 128 characters")
