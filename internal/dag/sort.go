package dag

import "sort"

// TopologicalSort runs Kahn's algorithm on the graph.
// Returns levels [][]string where each level can run in parallel.
// Level 0 = no dependencies, Level 1 = depends only on Level 0, etc.
// Returns ErrCycleDetected if the graph has a cycle.
func TopologicalSort(g *Graph) ([][]string, error) {
	inDeg := g.inDegrees()
	totalNodes := len(inDeg)

	// Seed queue with all zero-in-degree nodes.
	var queue []string
	for id, deg := range inDeg {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	levels := make([][]string, 0)
	processed := 0

	for len(queue) > 0 {
		// Current level: all nodes currently in the queue.
		currentLevel := make([]string, len(queue))
		copy(currentLevel, queue)
		levels = append(levels, currentLevel)
		processed += len(currentLevel)

		// Find nodes that become ready after processing this level.
		var nextQueue []string
		for _, nodeID := range currentLevel {
			for _, succ := range g.successorsOf(nodeID) {
				inDeg[succ]--
				if inDeg[succ] == 0 {
					nextQueue = append(nextQueue, succ)
				}
			}
		}
		sort.Strings(nextQueue)
		queue = nextQueue
	}

	if processed < totalNodes {
		return nil, ErrCycleDetected
	}

	return levels, nil
}
