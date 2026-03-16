package health

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// DAGStatusProvider is the consumer-defined interface for querying active DAG
// executions. dag.Executor satisfies this interface.
type DAGStatusProvider interface {
	ActiveExecutionCount() int
}

// HealthSnapshot is an immutable point-in-time snapshot of orchestrator health.
type HealthSnapshot struct {
	Alive           bool
	Ready           bool
	ReadyReason     string
	AgentsTotal     int
	AgentsIdle      int
	AgentsWorking   int
	AgentsAssigned  int
	AgentsReporting int
	TasksQueued     int64
	TasksInFlight   int // equals AgentsWorking
	DAGsInProgress  int
	RedisOK         bool
}

// AggregatorOption configures an Aggregator.
type AggregatorOption func(*Aggregator)

// WithMinAgents sets the minimum number of registered agents required for
// the orchestrator to report ready. Default is 1.
func WithMinAgents(n int) AggregatorOption {
	return func(a *Aggregator) {
		a.minAgents = n
	}
}

// Aggregator collects health signals from the state store and DAG executor
// and produces a HealthSnapshot.
type Aggregator struct {
	store       state.StateStore
	dagProvider DAGStatusProvider
	minAgents   int
}

// NewAggregator creates an Aggregator with the given store and DAG provider.
func NewAggregator(store state.StateStore, dag DAGStatusProvider, opts ...AggregatorOption) *Aggregator {
	a := &Aggregator{
		store:       store,
		dagProvider: dag,
		minAgents:   1,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Check gathers current health signals and returns an immutable snapshot.
// Alive is always true (if the code is running, the process is alive).
func (a *Aggregator) Check(ctx context.Context) HealthSnapshot {
	redisOK := a.store.Ping(ctx) == nil

	agentIDs, _ := a.store.ListAgents(ctx)

	var idle, working, assigned, reporting int
	for _, id := range agentIDs {
		st, err := a.store.GetAgentState(ctx, id)
		if err != nil {
			continue
		}
		switch st {
		case "idle":
			idle++
		case "working":
			working++
		case "assigned":
			assigned++
		case "reporting":
			reporting++
		}
	}

	queueLen, _ := a.store.QueueLength(ctx)
	dagsInProgress := a.dagProvider.ActiveExecutionCount()
	total := len(agentIDs)

	ready, reason := a.readiness(redisOK, total)

	return HealthSnapshot{
		Alive:           true,
		Ready:           ready,
		ReadyReason:     reason,
		AgentsTotal:     total,
		AgentsIdle:      idle,
		AgentsWorking:   working,
		AgentsAssigned:  assigned,
		AgentsReporting: reporting,
		TasksQueued:     queueLen,
		TasksInFlight:   working,
		DAGsInProgress:  dagsInProgress,
		RedisOK:         redisOK,
	}
}

// readiness returns (ready, reason). reason is empty when ready=true.
func (a *Aggregator) readiness(redisOK bool, agentCount int) (bool, string) {
	var reasons []string

	if !redisOK {
		reasons = append(reasons, "redis unreachable")
	}
	if agentCount == 0 {
		reasons = append(reasons, "no agents registered")
	} else if agentCount < a.minAgents {
		reasons = append(reasons, fmt.Sprintf("agents below minimum: %d < %d", agentCount, a.minAgents))
	}

	if len(reasons) > 0 {
		return false, strings.Join(reasons, "; ")
	}
	return true, ""
}
