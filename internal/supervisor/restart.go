package supervisor

import (
	"sync"
	"time"
)

// Clock allows injecting fake time in tests.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// RestartDecision is an immutable value type returned by RestartPolicy.
type RestartDecision struct {
	ShouldRestart bool
	Backoff       time.Duration
	// Reason is ErrCircuitOpen if the circuit breaker is open.
	Reason error
}

// RestartPolicyOption configures the policy.
type RestartPolicyOption func(*RestartPolicy)

// WithClock injects a custom clock (used in tests for deterministic time).
func WithClock(c Clock) RestartPolicyOption {
	return func(p *RestartPolicy) { p.clock = c }
}

// WithMaxBackoff sets the upper bound on exponential backoff duration.
func WithMaxBackoff(d time.Duration) RestartPolicyOption {
	return func(p *RestartPolicy) { p.maxBackoff = d }
}

// WithCrashWindow sets the sliding window duration for crash counting.
func WithCrashWindow(d time.Duration) RestartPolicyOption {
	return func(p *RestartPolicy) { p.crashWindow = d }
}

// WithCrashThreshold sets the maximum number of crashes allowed within the window
// before the circuit breaker opens.
func WithCrashThreshold(n int) RestartPolicyOption {
	return func(p *RestartPolicy) { p.crashThreshold = n }
}

// WithBaseBackoff sets the initial backoff duration (default 1s).
func WithBaseBackoff(d time.Duration) RestartPolicyOption {
	return func(p *RestartPolicy) { p.baseBackoff = d }
}

// RestartPolicy tracks per-agent crash history and returns restart decisions.
// Circuit breaker: if > threshold crashes in window -> open -> half-open probe after window expires.
type RestartPolicy struct {
	clock          Clock
	baseBackoff    time.Duration
	maxBackoff     time.Duration
	crashThreshold int
	crashWindow    time.Duration

	mu               sync.Mutex
	agentCrashes     map[string][]time.Time // per-agent crash timestamps
	agentConsecutive map[string]int         // per-agent consecutive crash count
}

// NewRestartPolicy returns a RestartPolicy with sensible defaults.
func NewRestartPolicy(opts ...RestartPolicyOption) *RestartPolicy {
	p := &RestartPolicy{
		clock:            realClock{},
		baseBackoff:      1 * time.Second,
		maxBackoff:       30 * time.Second,
		crashThreshold:   5,
		crashWindow:      60 * time.Second,
		agentCrashes:     make(map[string][]time.Time),
		agentConsecutive: make(map[string]int),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// RecordCrash records a crash for the given agent and returns a restart decision.
// It prunes old crashes, checks the circuit breaker, and computes backoff.
func (p *RestartPolicy) RecordCrash(agentID string) RestartDecision {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.clock.Now()

	// Record this crash.
	p.agentCrashes[agentID] = append(p.agentCrashes[agentID], now)

	// Prune crashes older than the window.
	cutoff := now.Add(-p.crashWindow)
	crashes := p.agentCrashes[agentID]
	recent := make([]time.Time, 0, len(crashes))
	for _, t := range crashes {
		if !t.Before(cutoff) {
			recent = append(recent, t)
		}
	}
	p.agentCrashes[agentID] = recent

	// Circuit breaker check: > threshold crashes in window opens the circuit.
	if len(p.agentCrashes[agentID]) > p.crashThreshold {
		return RestartDecision{
			ShouldRestart: false,
			Reason:        ErrCircuitOpen,
		}
	}

	// Compute exponential backoff.
	p.agentConsecutive[agentID]++
	backoff := p.computeBackoff(p.agentConsecutive[agentID])

	return RestartDecision{
		ShouldRestart: true,
		Backoff:       backoff,
	}
}

// RecordSuccess resets the consecutive crash counter and clears crash history
// for the given agent, resetting its circuit breaker.
func (p *RestartPolicy) RecordSuccess(agentID string) {
	p.mu.Lock()
	p.agentConsecutive[agentID] = 0
	delete(p.agentCrashes, agentID)
	p.mu.Unlock()
}

// computeBackoff returns min(base * 2^(n-1), max).
func (p *RestartPolicy) computeBackoff(consecutive int) time.Duration {
	backoff := p.baseBackoff
	for i := 1; i < consecutive; i++ {
		backoff *= 2
		if backoff >= p.maxBackoff {
			return p.maxBackoff
		}
	}
	return backoff
}
