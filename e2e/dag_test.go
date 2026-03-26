//go:build testenv

package e2e_test

import (
	"context"
	"testing"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
)

// ── Scenario 6: Linear DAG ───────────────────────────────────────────────────

// TestE2E_LinearDAG verifies that a three-node linear DAG (A → B → C) completes
// with all nodes in the COMPLETED state.
// MVP: nodes complete on successful enqueue — full agent execution deferred to model gateway integration.
func TestE2E_LinearDAG(t *testing.T) {
	s := newTestStack(t, nil)
	defer s.cleanup()

	spec := &pb.DAGSpec{
		DagId: "linear-dag",
		Nodes: []*pb.DAGNode{
			{NodeId: "a", Task: &pb.TaskSpec{TaskId: "t-a", Prompt: "step A", Priority: 1.0}},
			{NodeId: "b", Task: &pb.TaskSpec{TaskId: "t-b", Prompt: "step B", Priority: 1.0}, DependsOn: []string{"a"}},
			{NodeId: "c", Task: &pb.TaskSpec{TaskId: "t-c", Prompt: "step C", Priority: 1.0}, DependsOn: []string{"b"}},
		},
	}

	ctx := context.Background()
	submitResp, err := s.client.SubmitDAG(ctx, &pb.SubmitDAGRequest{Dag: spec})
	if err != nil {
		t.Fatalf("SubmitDAG: %v", err)
	}

	statusResp := pollDAGComplete(t, s.client, submitResp.DagExecutionId, 5*time.Second)
	if statusResp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Fatalf("expected COMPLETED, got %v", statusResp.State)
	}

	for _, ns := range statusResp.NodeStatuses {
		if ns.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
			t.Fatalf("node %s: expected COMPLETED, got %v", ns.NodeId, ns.State)
		}
	}
}

// ── Scenario 7: Parallel Fork DAG ────────────────────────────────────────────

// TestE2E_ParallelForkDAG verifies that a fork-join DAG (A → {B, C} → D)
// completes with all four nodes in the COMPLETED state.  B and C execute in
// parallel after A, and D starts only after both B and C finish.
// MVP: nodes complete on successful enqueue — full agent execution deferred to model gateway integration.
func TestE2E_ParallelForkDAG(t *testing.T) {
	s := newTestStack(t, nil)
	defer s.cleanup()

	spec := &pb.DAGSpec{
		DagId: "fork-dag",
		Nodes: []*pb.DAGNode{
			{NodeId: "a", Task: &pb.TaskSpec{TaskId: "t-a", Prompt: "root", Priority: 1.0}},
			{NodeId: "b", Task: &pb.TaskSpec{TaskId: "t-b", Prompt: "fork-left", Priority: 1.0}, DependsOn: []string{"a"}},
			{NodeId: "c", Task: &pb.TaskSpec{TaskId: "t-c", Prompt: "fork-right", Priority: 1.0}, DependsOn: []string{"a"}},
			{NodeId: "d", Task: &pb.TaskSpec{TaskId: "t-d", Prompt: "join", Priority: 1.0}, DependsOn: []string{"b", "c"}},
		},
	}

	ctx := context.Background()
	submitResp, err := s.client.SubmitDAG(ctx, &pb.SubmitDAGRequest{Dag: spec})
	if err != nil {
		t.Fatalf("SubmitDAG: %v", err)
	}

	statusResp := pollDAGComplete(t, s.client, submitResp.DagExecutionId, 5*time.Second)
	if statusResp.State != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
		t.Fatalf("expected COMPLETED, got %v", statusResp.State)
	}

	nodeMap := make(map[string]pb.DAGExecutionState, len(statusResp.NodeStatuses))
	for _, ns := range statusResp.NodeStatuses {
		nodeMap[ns.NodeId] = ns.State
	}

	for _, id := range []string{"a", "b", "c", "d"} {
		if nodeMap[id] != pb.DAGExecutionState_DAG_EXECUTION_STATE_COMPLETED {
			t.Fatalf("node %s: expected COMPLETED, got %v", id, nodeMap[id])
		}
	}
}
