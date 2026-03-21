package dag_test

import (
	"errors"
	"reflect"
	"testing"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
)

func buildGraph(t *testing.T, nodes ...*pb.DAGNode) *dag.Graph {
	t.Helper()
	g, err := dag.NewGraph(&pb.DAGSpec{DagId: "test", Nodes: nodes})
	if err != nil {
		t.Fatalf("buildGraph: NewGraph() error: %v", err)
	}
	return g
}

func TestTopologicalSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		nodes      []*pb.DAGNode
		wantLevels [][]string
		wantErr    error
	}{
		{
			name:       "single node",
			nodes:      []*pb.DAGNode{makeNode("A")},
			wantLevels: [][]string{{"A"}},
		},
		{
			name: "linear A->B->C",
			nodes: []*pb.DAGNode{
				makeNode("A"),
				makeNode("B", "A"),
				makeNode("C", "B"),
			},
			wantLevels: [][]string{{"A"}, {"B"}, {"C"}},
		},
		{
			name: "fork A->B,C (alphabetical within level)",
			nodes: []*pb.DAGNode{
				makeNode("A"),
				makeNode("B", "A"),
				makeNode("C", "A"),
			},
			wantLevels: [][]string{{"A"}, {"B", "C"}},
		},
		{
			name: "diamond A->B,C->D",
			nodes: []*pb.DAGNode{
				makeNode("A"),
				makeNode("B", "A"),
				makeNode("C", "A"),
				makeNode("D", "B", "C"),
			},
			wantLevels: [][]string{{"A"}, {"B", "C"}, {"D"}},
		},
		{
			name: "parallel A,B (no deps)",
			nodes: []*pb.DAGNode{
				makeNode("A"),
				makeNode("B"),
			},
			wantLevels: [][]string{{"A", "B"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := buildGraph(t, tc.nodes...)
			levels, err := dag.TopologicalSort(g)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("TopologicalSort() error = %v, wantErr %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("TopologicalSort() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(levels, tc.wantLevels) {
				t.Errorf("TopologicalSort() levels = %v, want %v", levels, tc.wantLevels)
			}
		})
	}
}

func TestTopologicalSort_CycleDetected(t *testing.T) {
	t.Parallel()
	// Can't use buildGraph because NewGraph calls TopologicalSort internally.
	// We test cycle detection through NewGraph since a cyclic graph can't be built.
	spec := &pb.DAGSpec{
		DagId: "cyclic",
		Nodes: []*pb.DAGNode{
			makeNode("A", "B"),
			makeNode("B", "A"),
		},
	}
	_, err := dag.NewGraph(spec)
	if !errors.Is(err, dag.ErrCycleDetected) {
		t.Errorf("NewGraph() with cycle error = %v, want ErrCycleDetected", err)
	}
}
