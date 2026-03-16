package dag

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/google/uuid"
)

// Executor validates a DAGSpec, builds the execution plan, and runs tasks
// level-by-level with parallel fan-out within each level. On any task
// failure the executor cancels all in-flight work (fail-fast).
type Executor struct {
	submitter TaskSubmitter
	tracker   *StatusTracker
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewExecutor returns an Executor that delegates individual task execution
// to the provided TaskSubmitter. The ctx parameter controls the executor's
// lifetime — cancelling it aborts all running DAG executions.
func NewExecutor(ctx context.Context, submitter TaskSubmitter) *Executor {
	ctx, cancel := context.WithCancel(ctx)
	return &Executor{
		submitter: submitter,
		tracker:   NewStatusTracker(nil),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// ActiveExecutionCount returns the number of DAG executions currently running.
func (e *Executor) ActiveExecutionCount() int {
	return e.tracker.ActiveCount()
}

// Shutdown cancels all running executions and waits for them to finish.
func (e *Executor) Shutdown() {
	e.cancel()
	e.wg.Wait()
}

// Execute validates spec, registers the execution synchronously, then
// launches the level-by-level fan-out in a background goroutine.
// Returns (execID, nil) on success or ("", err) for validation errors.
func (e *Executor) Execute(ctx context.Context, spec *pb.DAGSpec) (string, error) {
	graph, err := NewGraph(spec)
	if err != nil {
		return "", err
	}

	levels, err := TopologicalSort(graph)
	if err != nil {
		return "", err
	}

	execID := uuid.New().String()

	// CreateExecution must happen synchronously so Status() works
	// immediately after Execute() returns.
	e.tracker.CreateExecution(execID, graph.DAGID(), graph.Nodes())

	// Use the executor's server-lifetime context so the execution outlives
	// the RPC but respects graceful shutdown.
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.run(e.ctx, execID, graph, levels)
	}()

	return execID, nil
}

// Status returns the current execution state as a proto response.
// Returns an error if the execID is unknown.
func (e *Executor) Status(_ context.Context, execID string) (*pb.GetDAGStatusResponse, error) {
	snap := e.tracker.Snapshot(execID)
	if snap.State == 0 {
		return nil, fmt.Errorf("%w: %s", ErrExecutionNotFound, execID)
	}
	resp := ToProtoResponse(snap)
	return resp, nil
}

// run processes levels sequentially, fanning out goroutines for each node
// within a level. It cancels remaining work on the first failure.
func (e *Executor) run(ctx context.Context, execID string, graph *Graph, levels [][]string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for levelIdx, level := range levels {
		var wg sync.WaitGroup

		for _, nodeID := range level {
			wg.Add(1)
			go func(nID string) {
				defer wg.Done()
				e.executeNode(ctx, execID, graph, nID, cancel)
			}(nodeID)
		}

		wg.Wait()

		if ctx.Err() != nil {
			for _, remaining := range levels[levelIdx+1:] {
				for _, nodeID := range remaining {
					e.tracker.MarkNodeFailed(execID, nodeID, "skipped: upstream failure")
				}
			}
			break
		}
	}
}

// executeNode runs a single node: marks it running, submits the task,
// then marks it completed or failed.
func (e *Executor) executeNode(
	ctx context.Context,
	execID string,
	graph *Graph,
	nodeID string,
	cancel context.CancelFunc,
) {
	e.tracker.MarkNodeRunning(execID, nodeID)

	ts := graph.TaskSpec(nodeID)

	err := e.submitter.SubmitTask(ctx, ts.TaskId, ts.Prompt, ts.ModelTier, ts.Priority)
	if err != nil {
		e.tracker.MarkNodeFailed(execID, nodeID, err.Error())
		cancel()
		return
	}

	e.tracker.MarkNodeCompleted(execID, nodeID)
}
