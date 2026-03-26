package supervisor_test

import (
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
)

// fakeClock implements supervisor.Clock with controllable time.
type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time { return f.now }

func (f *fakeClock) Advance(d time.Duration) { f.now = f.now.Add(d) }

func TestRestartPolicy_ExponentialBackoff(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Now()}
	policy := supervisor.NewRestartPolicy(
		supervisor.WithClock(clock),
		supervisor.WithMaxBackoff(16*time.Second),
		supervisor.WithCrashWindow(60*time.Second),
		supervisor.WithCrashThreshold(10), // high threshold so circuit stays closed
	)

	expectedBackoffs := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second, // capped at max
		16 * time.Second, // still capped
	}

	for i, want := range expectedBackoffs {
		decision := policy.RecordCrash("test-agent")
		if !decision.ShouldRestart {
			t.Fatalf("crash %d: expected ShouldRestart=true, got false", i+1)
		}
		if decision.Backoff != want {
			t.Errorf("crash %d: backoff = %v, want %v", i+1, decision.Backoff, want)
		}
		// Advance time a little between crashes (less than window)
		clock.Advance(1 * time.Second)
	}
}

func TestRestartPolicy_CircuitBreakerOpens(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Now()}
	policy := supervisor.NewRestartPolicy(
		supervisor.WithClock(clock),
		supervisor.WithCrashWindow(60*time.Second),
		supervisor.WithCrashThreshold(5),
	)

	// 5 crashes within the window
	for i := 0; i < 5; i++ {
		d := policy.RecordCrash("test-agent")
		if !d.ShouldRestart {
			t.Fatalf("crash %d: expected ShouldRestart=true before threshold", i+1)
		}
		clock.Advance(1 * time.Second)
	}

	// 6th crash should open the circuit
	d := policy.RecordCrash("test-agent")
	if d.ShouldRestart {
		t.Errorf("expected circuit open (ShouldRestart=false), got true")
	}
	if d.Reason == nil {
		t.Errorf("expected Reason=ErrCircuitOpen, got nil")
	}
}

func TestRestartPolicy_SuccessResetsConsecutive(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Now()}
	policy := supervisor.NewRestartPolicy(
		supervisor.WithClock(clock),
		supervisor.WithCrashWindow(60*time.Second),
		supervisor.WithCrashThreshold(10),
	)

	// Record 3 crashes — backoff should be at 8s.
	for i := 0; i < 3; i++ {
		policy.RecordCrash("test-agent")
		clock.Advance(100 * time.Millisecond)
	}

	// Reset via success.
	policy.RecordSuccess("test-agent")

	// Next crash should restart from 1s backoff.
	d := policy.RecordCrash("test-agent")
	if !d.ShouldRestart {
		t.Fatal("expected ShouldRestart=true after reset")
	}
	if d.Backoff != 1*time.Second {
		t.Errorf("after reset backoff = %v, want 1s", d.Backoff)
	}
}

func TestRestartPolicy_SuccessKeepsCircuitClosed(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Now()}
	policy := supervisor.NewRestartPolicy(
		supervisor.WithClock(clock),
		supervisor.WithCrashWindow(60*time.Second),
		supervisor.WithCrashThreshold(5),
	)

	// Alternate crash and success — consecutive never climbs.
	for i := 0; i < 10; i++ {
		d := policy.RecordCrash("test-agent")
		if !d.ShouldRestart {
			t.Fatalf("iteration %d: circuit opened unexpectedly", i)
		}
		policy.RecordSuccess("test-agent")
		clock.Advance(1 * time.Second)
	}
}

func TestRestartPolicy_OldCrashesExpire(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Now()}
	policy := supervisor.NewRestartPolicy(
		supervisor.WithClock(clock),
		supervisor.WithCrashWindow(60*time.Second),
		supervisor.WithCrashThreshold(5),
	)

	// 5 crashes right at the edge of the window.
	for i := 0; i < 5; i++ {
		policy.RecordCrash("test-agent")
		clock.Advance(10 * time.Second)
	}
	// Advance past window so all old crashes expire.
	clock.Advance(61 * time.Second)

	// Next crash should succeed (old crashes pruned).
	d := policy.RecordCrash("test-agent")
	if !d.ShouldRestart {
		t.Errorf("expected ShouldRestart=true after old crashes expired, got false; reason=%v", d.Reason)
	}
}
