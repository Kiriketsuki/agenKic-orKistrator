package supervisor_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
)

func TestShutdownHandler_DrainCalledOnContextCancel(t *testing.T) {
	t.Parallel()

	drained := make(chan struct{})
	handler := supervisor.NewShutdownHandler(func(ctx context.Context) error {
		close(drained)
		return nil
	}, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- handler.WaitForSignal(ctx)
	}()

	// Cancel the context to simulate shutdown trigger.
	cancel()

	select {
	case <-drained:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("drain was not called within 2s after context cancel")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("WaitForSignal returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSignal did not return within 2s")
	}
}

func TestShutdownHandler_TimeoutRespected(t *testing.T) {
	t.Parallel()

	handler := supervisor.NewShutdownHandler(func(ctx context.Context) error {
		// Block until drain context expires.
		<-ctx.Done()
		return ctx.Err()
	}, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- handler.WaitForSignal(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		// We expect an error from timeout (context.DeadlineExceeded or similar).
		if err == nil {
			t.Error("expected error when drain times out, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSignal did not return within 2s after timeout")
	}
}

func TestShutdownHandler_DrainErrorPropagated(t *testing.T) {
	t.Parallel()

	drainErr := errors.New("drain failed")
	handler := supervisor.NewShutdownHandler(func(ctx context.Context) error {
		return drainErr
	}, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- handler.WaitForSignal(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, drainErr) {
			t.Errorf("expected drainErr, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForSignal did not return within 2s")
	}
}
