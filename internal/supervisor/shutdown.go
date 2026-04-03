package supervisor

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ShutdownHandler manages graceful shutdown on SIGTERM/SIGINT or context cancellation.
type ShutdownHandler struct {
	drain   func(ctx context.Context) error
	timeout time.Duration
}

// NewShutdownHandler creates a ShutdownHandler that calls drain on shutdown,
// bounded by the given timeout.
func NewShutdownHandler(drain func(ctx context.Context) error, timeout time.Duration) *ShutdownHandler {
	return &ShutdownHandler{
		drain:   drain,
		timeout: timeout,
	}
}

// WaitForSignal blocks until SIGTERM, SIGINT, or the parent context is cancelled,
// then calls drain within the configured timeout.
// Returns when drain completes or timeout elapses.
func (h *ShutdownHandler) WaitForSignal(ctx context.Context) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	select {
	case <-sigCh:
	case <-ctx.Done():
	}

	drainCtx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	return h.drain(drainCtx)
}
