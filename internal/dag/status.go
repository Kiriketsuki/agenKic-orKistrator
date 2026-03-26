package dag

import (
	"sort"
	"sync"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
)

// Clock allows injecting fake time in tests.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// NodeStatus is an immutable snapshot of a single node's execution state.
type NodeStatus struct {
	NodeID       string
	State        pb.DAGExecutionState
	StartedAt    time.Time
	CompletedAt  time.Time
	ErrorMessage string
}

// ExecutionRecord is an immutable snapshot of a full DAG execution.
type ExecutionRecord struct {
	ExecutionID  string
	DAGID        string
	State        pb.DAGExecutionState
	NodeStatuses []NodeStatus
	StartedAt    time.Time
	CompletedAt  time.Time
}

// StatusTracker is a thread-safe tracker for DAG execution state.
type StatusTracker struct {
	mu      sync.RWMutex
	records map[string]*mutableRecord
	clock   Clock
}

// mutableRecord is the internal mutable state — never exposed externally.
type mutableRecord struct {
	executionID string
	dagID       string
	state       pb.DAGExecutionState
	nodes       map[string]*mutableNodeStatus
	startedAt   time.Time
	completedAt time.Time
}

type mutableNodeStatus struct {
	state        pb.DAGExecutionState
	startedAt    time.Time
	completedAt  time.Time
	errorMessage string
}

// NewStatusTracker creates a new StatusTracker with the given clock.
func NewStatusTracker(clock Clock) *StatusTracker {
	if clock == nil {
		clock = realClock{}
	}
	return &StatusTracker{
		records: make(map[string]*mutableRecord),
		clock:   clock,
	}
}

// CreateExecution initializes a new execution record with all nodes in PENDING state.
func (t *StatusTracker) CreateExecution(executionID, dagID string, nodeIDs []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	nodes := make(map[string]*mutableNodeStatus, len(nodeIDs))
	for _, id := range nodeIDs {
		nodes[id] = &mutableNodeStatus{
			state: pb.DAGExecutionState_DAG_EXECUTION_STATE_PENDING,
		}
	}

	t.records[executionID] = &mutableRecord{
		executionID: executionID,
		dagID:       dagID,
		state:       pb.DAGExecutionState_DAG_EXECUTION_STATE_RUNNING,
		nodes:       nodes,
		startedAt:   t.clock.Now(),
	}
}

// MarkNodeRunning transitions a node to RUNNING.
func (t *StatusTracker) MarkNodeRunning(executionID, nodeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.records[executionID]
	if !ok {
		return
	}
	node, ok := rec.nodes[nodeID]
	if !ok {
		return
	}
	node.state = pb.DAGExecutionState_DAG_EXECUTION_STATE_RUNNING
	node.startedAt = t.clock.Now()
}

// MarkNodeCompleted transitions a node to COMPLETED.
// If all nodes are now complete, marks the execution as COMPLETED.
func (t *StatusTracker) MarkNodeCompleted(executionID, nodeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.records[executionID]
	if !ok {
		return
	}
	node, ok := rec.nodes[nodeID]
	if !ok {
		return
	}
	now := t.clock.Now()
	node.state = pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED
	node.completedAt = now

	if allNodesCompleted(rec.nodes) {
		rec.state = pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED
		rec.completedAt = now
	}
}

// MarkNodeFailed transitions a node to FAILED and marks the execution as FAILED.
func (t *StatusTracker) MarkNodeFailed(executionID, nodeID, errMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.records[executionID]
	if !ok {
		return
	}
	node, ok := rec.nodes[nodeID]
	if !ok {
		return
	}
	now := t.clock.Now()
	node.state = pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED
	node.completedAt = now
	node.errorMessage = errMsg

	rec.state = pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED
	if rec.completedAt.IsZero() {
		rec.completedAt = now
	}
}

// ActiveCount returns the number of executions currently in RUNNING state.
func (t *StatusTracker) ActiveCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	count := 0
	for _, rec := range t.records {
		if rec.state == pb.DAGExecutionState_DAG_EXECUTION_STATE_RUNNING {
			count++
		}
	}
	return count
}

// Snapshot returns an immutable ExecutionRecord.
// Returns ExecutionRecord with zero State if executionID not found.
func (t *StatusTracker) Snapshot(executionID string) ExecutionRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	rec, ok := t.records[executionID]
	if !ok {
		return ExecutionRecord{}
	}
	return recordToSnapshot(rec)
}

// ToProtoResponse converts an ExecutionRecord to the proto response type.
func ToProtoResponse(r ExecutionRecord) *pb.GetDAGStatusResponse {
	nodeStatuses := make([]*pb.DAGNodeStatus, len(r.NodeStatuses))
	for i, ns := range r.NodeStatuses {
		nodeStatuses[i] = &pb.DAGNodeStatus{
			NodeId: ns.NodeID,
			State:  ns.State,
			Error:  ns.ErrorMessage,
		}
	}
	return &pb.GetDAGStatusResponse{
		DagExecutionId: r.ExecutionID,
		State:          r.State,
		NodeStatuses:   nodeStatuses,
	}
}

// allNodesCompleted checks if every node is in COMPLETED state.
func allNodesCompleted(nodes map[string]*mutableNodeStatus) bool {
	for _, n := range nodes {
		if n.state != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
			return false
		}
	}
	return true
}

// recordToSnapshot creates an immutable copy of a mutableRecord.
func recordToSnapshot(rec *mutableRecord) ExecutionRecord {
	nodeStatuses := make([]NodeStatus, 0, len(rec.nodes))
	for id, n := range rec.nodes {
		nodeStatuses = append(nodeStatuses, NodeStatus{
			NodeID:       id,
			State:        n.state,
			StartedAt:    n.startedAt,
			CompletedAt:  n.completedAt,
			ErrorMessage: n.errorMessage,
		})
	}
	sort.Slice(nodeStatuses, func(i, j int) bool {
		return nodeStatuses[i].NodeID < nodeStatuses[j].NodeID
	})
	return ExecutionRecord{
		ExecutionID:  rec.executionID,
		DAGID:        rec.dagID,
		State:        rec.state,
		NodeStatuses: nodeStatuses,
		StartedAt:    rec.startedAt,
		CompletedAt:  rec.completedAt,
	}
}
