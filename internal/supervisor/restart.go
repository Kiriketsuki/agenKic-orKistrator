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

// RestartPolicy tracks crash history and returns restart decisions.
// Circuit breaker: if > threshold crashes in window -> open -> half-open probe after window expires.
type RestartPolicy struct {
	clock          Clock
	baseBackoff    time.Duration
	maxBackoff     time.Duration
	crashThreshold int
	crashWindow    time.Duration

	mu          sync.Mutex
	crashes     []time.Time // timestamps of recent crashes
	consecutive int         // for exponential backoff
}

// NewRestartPolicy returns a RestartPolicy with sensible defaults.
func NewRestartPolicy(opts ...RestartPolicyOption) *RestartPolicy {
	p := &RestartPolicy{
		clock:          realClock{},
		baseBackoff:    1 * time.Second,
		maxBackoff:     30 * time.Second,
		crashThreshold: 5,
		crashWindow:    60 * time.Second,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// RecordCrash records a crash timestamp and returns a restart decision.
// It prunes old crashes, checks the circuit breaker, and computes backoff.
func (p *RestartPolicy) RecordCrash() RestartDecision {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.clock.Now()

	// Record this crash.
	p.crashes = append(p.crashes, now)

	// Prune crashes older than the window.
	cutoff := now.Add(-p.crashWindow)
	recent := make([]time.Time, 0, len(p.crashes))
	for _, t := range p.crashes {
		if !t.Before(cutoff) {
			recent = append(recent, t)
		}
	}
	p.crashes = recent

	// Circuit breaker check: > threshold crashes in window opens the circuit.
	if len(p.crashes) > p.crashThreshold {
		return RestartDecision{
			ShouldRestart: false,
			Reason:        ErrCircuitOpen,
		}
	}

	// Compute exponential backoff.
	p.consecutive++
	backoff := p.computeBackoff(p.consecutive)

	return RestartDecision{
		ShouldRestart: true,
		Backoff:       backoff,
	}
}

// RecordSuccess resets the consecutive crash counter and clears crash history,
// resetting the circuit breaker.
func (p *RestartPolicy) RecordSuccess() {
	p.mu.Lock()
	p.consecutive = 0
	p.crashes = p.crashes[:0]
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
