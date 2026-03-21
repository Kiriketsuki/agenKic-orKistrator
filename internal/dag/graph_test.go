package dag_test

import (
	"errors"
	"testing"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
)

func makeNode(id string, deps ...string) *pb.DAGNode {
	return &pb.DAGNode{
		NodeId:    id,
		Task:      &pb.TaskSpec{TaskId: id + "-task", Prompt: "do " + id},
		DependsOn: deps,
	}
}

func TestNewGraph(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		spec       *pb.DAGSpec
		wantErr    error
		checkGraph func(t *testing.T, g *dag.Graph)
	}{
		{
			name:    "nil spec nodes -> ErrEmptyDAG",
			spec:    &pb.DAGSpec{DagId: "d1"},
			wantErr: dag.ErrEmptyDAG,
		},
		{
			name:    "empty nodes -> ErrEmptyDAG",
			spec:    &pb.DAGSpec{DagId: "d1", Nodes: []*pb.DAGNode{}},
			wantErr: dag.ErrEmptyDAG,
		},
		{
			name: "duplicate node IDs -> ErrDuplicateNode",
			spec: &pb.DAGSpec{
				DagId: "d1",
				Nodes: []*pb.DAGNode{
					makeNode("A"),
					makeNode("A"),
				},
			},
			wantErr: dag.ErrDuplicateNode,
		},
		{
			name: "nil task spec -> ErrMissingTaskSpec",
			spec: &pb.DAGSpec{
				DagId: "d1",
				Nodes: []*pb.DAGNode{
					{NodeId: "A", DependsOn: nil},
				},
			},
			wantErr: dag.ErrMissingTaskSpec,
		},
		{
			name: "edge referencing unknown node -> ErrNodeNotFound",
			spec: &pb.DAGSpec{
				DagId: "d1",
				Nodes: []*pb.DAGNode{
					makeNode("A", "X"), // X does not exist
				},
			},
			wantErr: dag.ErrNodeNotFound,
		},
		{
			name: "valid linear chain A->B->C",
			spec: &pb.DAGSpec{
				DagId: "d1",
				Nodes: []*pb.DAGNode{
					makeNode("A"),
					makeNode("B", "A"),
					makeNode("C", "B"),
				},
			},
			wantErr: nil,
			checkGraph: func(t *testing.T, g *dag.Graph) {
				t.Helper()
				if g.DAGID() != "d1" {
					t.Errorf("DAGID = %q, want d1", g.DAGID())
				}
				nodes := g.Nodes()
				if len(nodes) != 3 {
					t.Fatalf("Nodes count = %d, want 3", len(nodes))
				}
				// Predecessors
				if preds := g.Predecessors("B"); len(preds) != 1 || preds[0] != "A" {
					t.Errorf("Predecessors(B) = %v, want [A]", preds)
				}
				if preds := g.Predecessors("C"); len(preds) != 1 || preds[0] != "B" {
					t.Errorf("Predecessors(C) = %v, want [B]", preds)
				}
				if preds := g.Predecessors("A"); len(preds) != 0 {
					t.Errorf("Predecessors(A) = %v, want []", preds)
				}
			},
		},
		{
			name: "valid diamond A->B,C->D",
			spec: &pb.DAGSpec{
				DagId: "d2",
				Nodes: []*pb.DAGNode{
					makeNode("A"),
					makeNode("B", "A"),
					makeNode("C", "A"),
					makeNode("D", "B", "C"),
				},
			},
			wantErr: nil,
			checkGraph: func(t *testing.T, g *dag.Graph) {
				t.Helper()
				if len(g.Nodes()) != 4 {
					t.Fatalf("Nodes count = %d, want 4", len(g.Nodes()))
				}
				predsD := g.Predecessors("D")
				if len(predsD) != 2 {
					t.Errorf("Predecessors(D) = %v, want 2 elements", predsD)
				}
			},
		},
		{
			name: "cycle A->B->A -> ErrCycleDetected",
			spec: &pb.DAGSpec{
				DagId: "d3",
				Nodes: []*pb.DAGNode{
					makeNode("A", "B"),
					makeNode("B", "A"),
				},
			},
			wantErr: dag.ErrCycleDetected,
		},
		{
			name: "single node no deps -> success",
			spec: &pb.DAGSpec{
				DagId: "d4",
				Nodes: []*pb.DAGNode{
					makeNode("A"),
				},
			},
			wantErr: nil,
			checkGraph: func(t *testing.T, g *dag.Graph) {
				t.Helper()
				if len(g.Nodes()) != 1 {
					t.Fatalf("Nodes count = %d, want 1", len(g.Nodes()))
				}
				if preds := g.Predecessors("A"); len(preds) != 0 {
					t.Errorf("Predecessors(A) = %v, want []", preds)
				}
				if ts := g.TaskSpec("A"); ts == nil {
					t.Error("TaskSpec(A) = nil, want non-nil")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g, err := dag.NewGraph(tc.spec)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("NewGraph() error = %v, wantErr %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewGraph() unexpected error: %v", err)
			}
			if tc.checkGraph != nil {
				tc.checkGraph(t, g)
			}
		})
	}
}
