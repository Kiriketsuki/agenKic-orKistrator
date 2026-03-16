package dag

import "errors"

// ErrCycleDetected is returned when the DAG contains a cycle.
var ErrCycleDetected = errors.New("dag: cycle detected")

// ErrEmptyDAG is returned when the DAG has no nodes.
var ErrEmptyDAG = errors.New("dag: empty DAG")

// ErrNodeNotFound is returned when an edge references a non-existent node.
var ErrNodeNotFound = errors.New("dag: node not found")

// ErrDuplicateNode is returned when the DAG has two nodes with the same ID.
var ErrDuplicateNode = errors.New("dag: duplicate node ID")

// ErrMissingTaskSpec is returned when a node has no task spec.
var ErrMissingTaskSpec = errors.New("dag: node has no task spec")

// ErrExecutionNotFound is returned when a status query references an unknown execution.
var ErrExecutionNotFound = errors.New("dag: execution not found")
