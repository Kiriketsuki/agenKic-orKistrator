package health

import (
	"context"
	"fmt"
	"log/slog"
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
	AgentsUnknown   int // agents in an unrecognised state
	TasksQueued     int64
	TasksInFlight   int // equals AgentsWorking
	DAGsInProgress  int
	RedisOK         bool
	AgentDataValid  bool // true when GetAllAgentStates succeeded
	QueueDataValid  bool // true when QueueLength succeeded
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
	pingErr := a.store.Ping(ctx)
	redisOK := pingErr == nil
	if pingErr != nil {
		slog.ErrorContext(ctx, "health check: Redis Ping failed", "error", pingErr)
	}

	// Batch-fetch all agent states; propagate errors to readiness reasons.
	var storeReasons []string
	agentDataValid := true

	agentStates, err := a.store.GetAllAgentStates(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "health check: GetAllAgentStates failed", "error", err)
		storeReasons = append(storeReasons, "agent states unavailable")
		agentDataValid = false
		agentStates = map[string]string{}
	}

	var idle, working, assigned, reporting, unknown int
	for _, st := range agentStates {
		switch st {
		case state.AgentStateIdle:
			idle++
		case state.AgentStateWorking:
			working++
		case state.AgentStateAssigned:
			assigned++
		case state.AgentStateReporting:
			reporting++
		default:
			unknown++
		}
	}

	queueDataValid := true
	queueLen, err := a.store.QueueLength(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "health check: QueueLength failed", "error", err)
		storeReasons = append(storeReasons, "queue length unavailable")
		queueDataValid = false
	}

	dagsInProgress := a.dagProvider.ActiveExecutionCount()
	total := len(agentStates)

	ready, reason := a.readiness(redisOK, agentDataValid, total, storeReasons...)

	return HealthSnapshot{
		Alive:           true,
		Ready:           ready,
		ReadyReason:     reason,
		AgentsTotal:     total,
		AgentsIdle:      idle,
		AgentsWorking:   working,
		AgentsAssigned:  assigned,
		AgentsReporting: reporting,
		AgentsUnknown:   unknown,
		TasksQueued:     queueLen,
		TasksInFlight:   working,
		DAGsInProgress:  dagsInProgress,
		RedisOK:         redisOK,
		AgentDataValid:  agentDataValid,
		QueueDataValid:  queueDataValid,
	}
}

// readiness returns (ready, reason). reason is empty when ready=true.
// storeErrors are specific failure messages from store method calls; when
// present they replace the generic "redis unreachable" message.
// agentDataValid indicates whether GetAllAgentStates succeeded; the agent-count
// check is skipped only when it is false (not whenever any store method fails).
func (a *Aggregator) readiness(redisOK bool, agentDataValid bool, agentCount int, storeErrors ...string) (bool, string) {
	var reasons []string
	reasons = append(reasons, storeErrors...)

	if !redisOK && len(storeErrors) == 0 {
		// Ping failed but no specific store method errors: report generically.
		reasons = append(reasons, "redis unreachable")
	}

	if agentDataValid {
		// Only check agent count when GetAllAgentStates returned valid data.
		if agentCount == 0 {
			reasons = append(reasons, "no agents registered")
		} else if agentCount < a.minAgents {
			reasons = append(reasons, fmt.Sprintf("agents below minimum: %d < %d", agentCount, a.minAgents))
		}
	}

	if len(reasons) > 0 {
		return false, strings.Join(reasons, "; ")
	}
	return true, ""
}
