package dag

import (
	"fmt"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
)

// Graph is an immutable, validated DAG built from a pb.DAGSpec.
type Graph struct {
	dagID        string
	nodes        map[string]*pb.DAGNode
	successors   map[string][]string // node -> nodes that depend on it
	predecessors map[string][]string // node -> nodes it depends on
	nodeOrder    []string            // stable insertion order
}

// NewGraph validates and builds a Graph from a DAGSpec.
// Returns ErrEmptyDAG, ErrDuplicateNode, ErrNodeNotFound, or ErrCycleDetected on invalid input.
func NewGraph(spec *pb.DAGSpec) (*Graph, error) {
	if spec == nil || len(spec.Nodes) == 0 {
		return nil, ErrEmptyDAG
	}

	nodes := make(map[string]*pb.DAGNode, len(spec.Nodes))
	nodeOrder := make([]string, 0, len(spec.Nodes))

	for _, n := range spec.Nodes {
		if _, exists := nodes[n.NodeId]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateNode, n.NodeId)
		}
		if n.Task == nil {
			return nil, fmt.Errorf("%w: %s", ErrMissingTaskSpec, n.NodeId)
		}
		nodes[n.NodeId] = n
		nodeOrder = append(nodeOrder, n.NodeId)
	}

	predecessors := make(map[string][]string, len(nodes))
	successors := make(map[string][]string, len(nodes))

	for id := range nodes {
		predecessors[id] = []string{}
		successors[id] = []string{}
	}

	for _, n := range spec.Nodes {
		for _, dep := range n.DependsOn {
			if _, exists := nodes[dep]; !exists {
				return nil, fmt.Errorf("%w: %s referenced by %s", ErrNodeNotFound, dep, n.NodeId)
			}
			predecessors[n.NodeId] = append(predecessors[n.NodeId], dep)
			successors[dep] = append(successors[dep], n.NodeId)
		}
	}

	g := &Graph{
		dagID:        spec.DagId,
		nodes:        nodes,
		successors:   successors,
		predecessors: predecessors,
		nodeOrder:    nodeOrder,
	}

	// Detect cycles via topological sort
	if _, err := TopologicalSort(g); err != nil {
		return nil, err
	}

	return g, nil
}

// Nodes returns all node IDs in stable insertion order.
func (g *Graph) Nodes() []string {
	result := make([]string, len(g.nodeOrder))
	copy(result, g.nodeOrder)
	return result
}

// Predecessors returns nodeIDs that must complete before nodeID.
func (g *Graph) Predecessors(nodeID string) []string {
	preds, ok := g.predecessors[nodeID]
	if !ok {
		return nil
	}
	result := make([]string, len(preds))
	copy(result, preds)
	return result
}

// TaskSpec returns the TaskSpec for a given nodeID.
func (g *Graph) TaskSpec(nodeID string) *pb.TaskSpec {
	n, ok := g.nodes[nodeID]
	if !ok {
		return nil
	}
	return n.Task
}

// DAGID returns the spec's dag_id.
func (g *Graph) DAGID() string {
	return g.dagID
}

// successorsOf returns the successors of a node (internal use by sort).
func (g *Graph) successorsOf(nodeID string) []string {
	return g.successors[nodeID]
}

// inDegrees returns a map of node -> number of predecessors (internal use by sort).
func (g *Graph) inDegrees() map[string]int {
	degrees := make(map[string]int, len(g.nodes))
	for id, preds := range g.predecessors {
		degrees[id] = len(preds)
	}
	return degrees
}
