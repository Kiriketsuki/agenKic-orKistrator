package supervisor

import "errors"

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker open")

// ErrSupervisorStopped is returned when operations are attempted on a stopped supervisor.
var ErrSupervisorStopped = errors.New("supervisor stopped")
