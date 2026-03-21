package dag_test

import (
	"sync"
	"testing"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
)

// fixedClock is a deterministic clock for testing.
type fixedClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFixedClock(t time.Time) *fixedClock {
	return &fixedClock{now: t}
}

func (c *fixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestStatusTracker_CreateExecution(t *testing.T) {
	t.Parallel()
	clk := newFixedClock(time.Now())
	tracker := dag.NewStatusTracker(clk)

	tracker.CreateExecution("exec-1", "dag-1", []string{"A", "B", "C"})
	snap := tracker.Snapshot("exec-1")

	if snap.ExecutionID != "exec-1" {
		t.Errorf("ExecutionID = %q, want exec-1", snap.ExecutionID)
	}
	if snap.DAGID != "dag-1" {
		t.Errorf("DAGID = %q, want dag-1", snap.DAGID)
	}
	// Execution should be RUNNING after creation
	if snap.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_RUNNING {
		t.Errorf("State = %v, want RUNNING", snap.State)
	}
	if len(snap.NodeStatuses) != 3 {
		t.Fatalf("NodeStatuses count = %d, want 3", len(snap.NodeStatuses))
	}
	for _, ns := range snap.NodeStatuses {
		if ns.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_PENDING {
			t.Errorf("node %s state = %v, want PENDING", ns.NodeID, ns.State)
		}
	}
}

func TestStatusTracker_MarkNodeCompleted_AllComplete(t *testing.T) {
	t.Parallel()
	clk := newFixedClock(time.Now())
	tracker := dag.NewStatusTracker(clk)

	tracker.CreateExecution("exec-2", "dag-2", []string{"A", "B"})
	tracker.MarkNodeRunning("exec-2", "A")
	tracker.MarkNodeCompleted("exec-2", "A")
	tracker.MarkNodeRunning("exec-2", "B")
	tracker.MarkNodeCompleted("exec-2", "B")

	snap := tracker.Snapshot("exec-2")
	if snap.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Errorf("State = %v, want COMPLETED", snap.State)
	}
	for _, ns := range snap.NodeStatuses {
		if ns.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
			t.Errorf("node %s state = %v, want COMPLETED", ns.NodeID, ns.State)
		}
	}
}

func TestStatusTracker_MarkNodeFailed(t *testing.T) {
	t.Parallel()
	clk := newFixedClock(time.Now())
	tracker := dag.NewStatusTracker(clk)

	tracker.CreateExecution("exec-3", "dag-3", []string{"A", "B"})
	tracker.MarkNodeRunning("exec-3", "A")
	tracker.MarkNodeFailed("exec-3", "A", "some error")

	snap := tracker.Snapshot("exec-3")
	if snap.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED {
		t.Errorf("State = %v, want FAILED", snap.State)
	}

	var failedNode dag.NodeStatus
	for _, ns := range snap.NodeStatuses {
		if ns.NodeID == "A" {
			failedNode = ns
		}
	}
	if failedNode.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED {
		t.Errorf("node A state = %v, want FAILED", failedNode.State)
	}
	if failedNode.ErrorMessage != "some error" {
		t.Errorf("node A ErrorMessage = %q, want 'some error'", failedNode.ErrorMessage)
	}
}

func TestStatusTracker_Snapshot_Immutable(t *testing.T) {
	t.Parallel()
	clk := newFixedClock(time.Now())
	tracker := dag.NewStatusTracker(clk)

	tracker.CreateExecution("exec-4", "dag-4", []string{"A"})
	snap1 := tracker.Snapshot("exec-4")

	// Mutate the returned snapshot's NodeStatuses slice.
	snap1.NodeStatuses = nil

	// The tracker's internal state should not be affected.
	snap2 := tracker.Snapshot("exec-4")
	if len(snap2.NodeStatuses) != 1 {
		t.Errorf("After mutating snap1, snap2.NodeStatuses count = %d, want 1", len(snap2.NodeStatuses))
	}
}

func TestStatusTracker_UnknownExecution(t *testing.T) {
	t.Parallel()
	clk := newFixedClock(time.Now())
	tracker := dag.NewStatusTracker(clk)

	snap := tracker.Snapshot("nonexistent")
	if snap.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_UNSPECIFIED {
		t.Errorf("State = %v, want UNSPECIFIED for unknown execution", snap.State)
	}
}

func TestStatusTracker_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	clk := newFixedClock(time.Now())
	tracker := dag.NewStatusTracker(clk)

	const execID = "exec-concurrent"
	nodeIDs := make([]string, 20)
	for i := range nodeIDs {
		nodeIDs[i] = string(rune('A' + i))
	}
	tracker.CreateExecution(execID, "dag-concurrent", nodeIDs)

	var wg sync.WaitGroup
	for _, id := range nodeIDs {
		wg.Add(1)
		go func(nid string) {
			defer wg.Done()
			tracker.MarkNodeRunning(execID, nid)
			tracker.MarkNodeCompleted(execID, nid)
		}(id)
	}
	wg.Wait()

	snap := tracker.Snapshot(execID)
	if snap.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Errorf("State = %v, want COMPLETED after all nodes complete", snap.State)
	}
}

func TestStatusTracker_ActiveCount(t *testing.T) {
	t.Parallel()
	clk := newFixedClock(time.Now())
	tracker := dag.NewStatusTracker(clk)

	// Empty tracker: zero active.
	if got := tracker.ActiveCount(); got != 0 {
		t.Fatalf("empty tracker: ActiveCount = %d, want 0", got)
	}

	// Create first execution: count = 1.
	tracker.CreateExecution("exec-ac-1", "dag-ac", []string{"A"})
	if got := tracker.ActiveCount(); got != 1 {
		t.Fatalf("after create 1: ActiveCount = %d, want 1", got)
	}

	// Create second execution: count = 2.
	tracker.CreateExecution("exec-ac-2", "dag-ac", []string{"B"})
	if got := tracker.ActiveCount(); got != 2 {
		t.Fatalf("after create 2: ActiveCount = %d, want 2", got)
	}

	// Complete all nodes in exec-ac-1: count = 1.
	tracker.MarkNodeRunning("exec-ac-1", "A")
	tracker.MarkNodeCompleted("exec-ac-1", "A")
	if got := tracker.ActiveCount(); got != 1 {
		t.Fatalf("after completing exec-ac-1: ActiveCount = %d, want 1", got)
	}
}

func TestToProtoResponse(t *testing.T) {
	t.Parallel()
	clk := newFixedClock(time.Now())
	tracker := dag.NewStatusTracker(clk)

	tracker.CreateExecution("exec-5", "dag-5", []string{"A"})
	tracker.MarkNodeRunning("exec-5", "A")
	tracker.MarkNodeCompleted("exec-5", "A")

	snap := tracker.Snapshot("exec-5")
	resp := dag.ToProtoResponse(snap)

	if resp.DagExecutionId != "exec-5" {
		t.Errorf("DagExecutionId = %q, want exec-5", resp.DagExecutionId)
	}
	if resp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Errorf("State = %v, want COMPLETED", resp.State)
	}
	if len(resp.NodeStatuses) != 1 {
		t.Fatalf("NodeStatuses count = %d, want 1", len(resp.NodeStatuses))
	}
	if resp.NodeStatuses[0].NodeId != "A" {
		t.Errorf("NodeStatuses[0].NodeId = %q, want A", resp.NodeStatuses[0].NodeId)
	}
}
