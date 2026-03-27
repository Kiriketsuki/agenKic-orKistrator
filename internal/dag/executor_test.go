package dag_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
)

// mockSubmitter records calls and can simulate failures.
type mockSubmitter struct {
	mu        sync.Mutex
	calls     []string // taskIDs called in order
	failIDs   map[string]error
	callTimes map[string]time.Time
	delay     time.Duration
}

func newMockSubmitter() *mockSubmitter {
	return &mockSubmitter{
		failIDs:   make(map[string]error),
		callTimes: make(map[string]time.Time),
	}
}

func (m *mockSubmitter) SubmitTask(ctx context.Context, taskID, prompt, modelTier string, priority float64) error {
	m.mu.Lock()
	m.calls = append(m.calls, taskID)
	m.callTimes[taskID] = time.Now()
	err := m.failIDs[taskID]
	delay := m.delay
	m.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (m *mockSubmitter) Calls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.calls))
	copy(result, m.calls)
	return result
}

func (m *mockSubmitter) CallTime(taskID string) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callTimes[taskID]
}

// waitForCompletion polls until the execution reaches a terminal state or times out.
func waitForCompletion(t *testing.T, e *dag.Executor, execID string, timeout time.Duration) *pb.GetDAGStatusResponse {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := e.Status(context.Background(), execID)
		if err != nil {
			t.Fatalf("Status() error: %v", err)
		}
		switch resp.State {
		case pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED,
			pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED:
			return resp
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("execution did not reach terminal state within timeout")
	return nil
}

func makeSpec(dagID string, nodes ...*pb.DAGNode) *pb.DAGSpec {
	return &pb.DAGSpec{DagId: dagID, Nodes: nodes}
}

func TestExecutor_LinearExecution(t *testing.T) {
	t.Parallel()
	sub := newMockSubmitter()
	e := dag.NewExecutor(context.Background(), sub)

	spec := makeSpec("lin",
		makeNode("A"),
		makeNode("B", "A"),
		makeNode("C", "B"),
	)

	execID, err := e.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if execID == "" {
		t.Fatal("Execute() returned empty execID")
	}

	resp := waitForCompletion(t, e, execID, 2*time.Second)
	if resp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Errorf("State = %v, want COMPLETED", resp.State)
	}

	calls := sub.Calls()
	if len(calls) != 3 {
		t.Fatalf("calls count = %d, want 3", len(calls))
	}
	// Linear execution: A must come before B, B before C.
	if calls[0] != "A-task" {
		t.Errorf("calls[0] = %q, want A-task", calls[0])
	}
	if calls[1] != "B-task" {
		t.Errorf("calls[1] = %q, want B-task", calls[1])
	}
	if calls[2] != "C-task" {
		t.Errorf("calls[2] = %q, want C-task", calls[2])
	}
}

func TestExecutor_ParallelFork(t *testing.T) {
	t.Parallel()
	sub := newMockSubmitter()
	sub.delay = 20 * time.Millisecond
	e := dag.NewExecutor(context.Background(), sub)

	// A -> B, C  (B and C run in parallel)
	spec := makeSpec("fork",
		makeNode("A"),
		makeNode("B", "A"),
		makeNode("C", "A"),
	)

	start := time.Now()
	execID, err := e.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	resp := waitForCompletion(t, e, execID, 2*time.Second)
	elapsed := time.Since(start)

	if resp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Errorf("State = %v, want COMPLETED", resp.State)
	}

	// If B and C ran sequentially, it'd take at least 3 * delay.
	// If parallel, it'd take roughly 2 * delay (A + parallel(B,C)).
	// Use 2.5x delay as threshold to verify parallelism.
	threshold := time.Duration(float64(sub.delay) * 2.5)
	if elapsed > threshold {
		t.Errorf("elapsed = %v, want < %v (suggests B,C ran sequentially not in parallel)", elapsed, threshold)
	}

	// Both B and C should have been called.
	calls := sub.Calls()
	if len(calls) != 3 {
		t.Fatalf("calls count = %d, want 3", len(calls))
	}
}

func TestExecutor_FailFast(t *testing.T) {
	t.Parallel()
	sub := newMockSubmitter()
	sub.delay = 10 * time.Millisecond
	sub.failIDs["B-task"] = errors.New("B failed")
	e := dag.NewExecutor(context.Background(), sub)

	// A -> B, C  (B fails, C may or may not run)
	spec := makeSpec("fail",
		makeNode("A"),
		makeNode("B", "A"),
		makeNode("C", "A"),
	)

	execID, err := e.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	resp := waitForCompletion(t, e, execID, 2*time.Second)
	if resp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED {
		t.Errorf("State = %v, want FAILED", resp.State)
	}
}

func TestExecutor_OrphanedNodesMarkedFailed(t *testing.T) {
	t.Parallel()
	sub := newMockSubmitter()
	sub.failIDs["A-task"] = errors.New("A failed")
	e := dag.NewExecutor(context.Background(), sub)

	// A -> B -> C  (A fails, B and C should be marked FAILED with "skipped: upstream failure")
	spec := makeSpec("orphan",
		makeNode("A"),
		makeNode("B", "A"),
		makeNode("C", "B"),
	)

	execID, err := e.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	waitForCompletion(t, e, execID, 2*time.Second)

	// Shutdown waits for the run() goroutine to finish, ensuring the
	// orphaned-node loop at executor.go:103-110 has completed before
	// we snapshot per-node states.
	e.Shutdown()

	resp, err2 := e.Status(context.Background(), execID)
	if err2 != nil {
		t.Fatalf("Status() error: %v", err2)
	}
	if resp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED {
		t.Fatalf("State = %v, want FAILED", resp.State)
	}

	// Build a map of node statuses for easy lookup.
	nodeStates := make(map[string]*pb.DAGNodeStatus, len(resp.NodeStatuses))
	for _, ns := range resp.NodeStatuses {
		nodeStates[ns.NodeId] = ns
	}

	// A should be FAILED with the actual error.
	if a, ok := nodeStates["A"]; !ok {
		t.Error("node A missing from response")
	} else if a.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED {
		t.Errorf("node A state = %v, want FAILED", a.State)
	}

	// B and C should be FAILED with "skipped: upstream failure".
	for _, nodeID := range []string{"B", "C"} {
		ns, ok := nodeStates[nodeID]
		if !ok {
			t.Errorf("node %s missing from response", nodeID)
			continue
		}
		if ns.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED {
			t.Errorf("node %s state = %v, want FAILED", nodeID, ns.State)
		}
		if ns.Error != "skipped: upstream failure" {
			t.Errorf("node %s error = %q, want %q", nodeID, ns.Error, "skipped: upstream failure")
		}
	}
}

func TestExecutor_EmptyDAG(t *testing.T) {
	t.Parallel()
	sub := newMockSubmitter()
	e := dag.NewExecutor(context.Background(), sub)

	spec := &pb.DAGSpec{DagId: "empty"}
	_, err := e.Execute(context.Background(), spec)
	if !errors.Is(err, dag.ErrEmptyDAG) {
		t.Errorf("Execute() with empty DAG error = %v, want ErrEmptyDAG", err)
	}
}

func TestExecutor_SingleNode(t *testing.T) {
	t.Parallel()
	sub := newMockSubmitter()
	e := dag.NewExecutor(context.Background(), sub)

	spec := makeSpec("single", makeNode("A"))
	execID, err := e.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	resp := waitForCompletion(t, e, execID, 2*time.Second)
	if resp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Errorf("State = %v, want COMPLETED", resp.State)
	}

	calls := sub.Calls()
	if len(calls) != 1 || calls[0] != "A-task" {
		t.Errorf("calls = %v, want [A-task]", calls)
	}
}

func TestExecutor_Status(t *testing.T) {
	t.Parallel()
	sub := newMockSubmitter()
	e := dag.NewExecutor(context.Background(), sub)

	spec := makeSpec("status-test", makeNode("A"))
	execID, err := e.Execute(context.Background(), spec)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	// Status should be available immediately.
	resp, err := e.Status(context.Background(), execID)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if resp.DagExecutionId != execID {
		t.Errorf("DagExecutionId = %q, want %q", resp.DagExecutionId, execID)
	}

	// Wait for completion and verify.
	resp = waitForCompletion(t, e, execID, 2*time.Second)
	if resp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Errorf("State = %v, want COMPLETED", resp.State)
	}
}

func TestExecutor_ShutdownCancelsRunning(t *testing.T) {
	t.Parallel()
	sub := newMockSubmitter()
	sub.delay = 100 * time.Millisecond

	ctx := context.Background()
	e := dag.NewExecutor(ctx, sub)

	// A -> B, C (B and C take 100ms each)
	spec := makeSpec("shutdown",
		makeNode("A"),
		makeNode("B", "A"),
		makeNode("C", "A"),
	)

	execID, err := e.Execute(ctx, spec)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	// Shutdown after A completes but while B,C are still running.
	time.Sleep(25 * time.Millisecond)
	e.Shutdown()

	resp, err := e.Status(ctx, execID)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if resp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_FAILED {
		t.Errorf("State = %v after Shutdown, want FAILED", resp.State)
	}
}
