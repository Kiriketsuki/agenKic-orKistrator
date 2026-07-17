package httpbridge

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/ipc"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

// Bridge is a thin HTTP/SSE layer over the orchestrator's core services.
// It wraps the same StateStore, Supervisor, and DAGEngine that the gRPC
// handlers use — no business logic duplication.
type Bridge struct {
	store     state.StateStore
	dag       ipc.DAGEngine
	substrate terminal.Substrate // optional; nil = PTY endpoints return 501
	apiKey    string             // optional; empty = no auth required

	// completionRegistry is optional; nil means handleCancelAgent cannot
	// unblock a DAG node's dag.BlockingSubmitter.Wait for the cancelled
	// task, so a cancel on a DAG-member task will strand that DAG node
	// forever instead of just detaching the task from the agent (T14
	// council finding #2). Wire it via WithCompletionRegistry using the
	// same *supervisor.CompletionRegistry instance passed to
	// supervisor.WithCompletionRegistry / dag.NewBlockingSubmitter so all
	// three components agree on what "this task is done" means.
	completionRegistry *supervisor.CompletionRegistry

	broker         *Broker // shared SSE fan-out broker; owns the single poll goroutine
	brokerInterval time.Duration

	mux     *http.ServeMux
	handler http.Handler // authMiddleware(mux) or mux — used by ServeHTTP
	server  *http.Server
}

// BridgeOption configures the Bridge.
type BridgeOption func(*Bridge)

// WithAPIKey enables bearer-token authentication on all endpoints.
// When set, every request must include "Authorization: Bearer <key>".
func WithAPIKey(key string) BridgeOption {
	return func(b *Bridge) { b.apiKey = key }
}

// WithSubstrate enables terminal-related endpoints (output capture, PTY input).
func WithSubstrate(s terminal.Substrate) BridgeOption {
	return func(b *Bridge) { b.substrate = s }
}

// WithCompletionRegistry wires the same *supervisor.CompletionRegistry used
// by the supervisor and the DAG's BlockingSubmitter into the Bridge, so
// handleCancelAgent can call Complete(taskID) on cancel to unblock any DAG
// node waiting on that task (see the Bridge.completionRegistry doc comment
// and handleCancelAgent for the exact, honest-minimal semantics: Complete
// has no success/failure signal, so a cancelled DAG-member task is observed
// by the DAG as having completed with no output — not as having failed).
// Optional: nil (the default) preserves the pre-T14-fix behavior of never
// signalling completion from the Bridge.
func WithCompletionRegistry(r *supervisor.CompletionRegistry) BridgeOption {
	return func(b *Bridge) { b.completionRegistry = r }
}

// WithBrokerInterval overrides the SSE broker's poll interval (default
// ssePollInterval). Primarily for tests that need deterministic control over
// the broker's poll timing — e.g. asserting on behavior that only manifests
// before the first poll tick has advanced the broker's cursor past "0".
func WithBrokerInterval(d time.Duration) BridgeOption {
	return func(b *Bridge) { b.brokerInterval = d }
}

// NewBridge creates a Bridge bound to addr. Call Start() to begin serving.
func NewBridge(addr string, store state.StateStore, dag ipc.DAGEngine, opts ...BridgeOption) *Bridge {
	b := &Bridge{
		store: store,
		dag:   dag,
		mux:   http.NewServeMux(),
	}
	for _, opt := range opts {
		opt(b)
	}

	// The broker's poll goroutine must launch here — not in Start — because
	// tests exercise Bridge via httptest+ServeHTTP and never call Start.
	interval := b.brokerInterval
	if interval <= 0 {
		interval = ssePollInterval
	}
	b.broker = NewBroker(store, interval, sseBatchSize, brokerDefaultBufSize)

	// REST endpoints
	b.mux.HandleFunc("GET /api/agents", b.handleListAgents)
	b.mux.HandleFunc("GET /api/agents/{id}/output", b.handleAgentOutput)
	b.mux.HandleFunc("GET /api/floors", b.handleListFloors)
	b.mux.HandleFunc("POST /api/tasks", b.handleSubmitTask)
	b.mux.HandleFunc("POST /api/dags", b.handleSubmitDAG)
	b.mux.HandleFunc("POST /api/agents/{id}/input", b.handleSendInput)
	b.mux.HandleFunc("POST /api/agents/{id}/cancel", b.handleCancelAgent)
	b.mux.HandleFunc("POST /api/agents/{id}/reassign", b.handleReassignAgent)

	// SSE stream
	b.mux.HandleFunc("GET /events/stream", b.handleSSE)

	b.handler = b.mux
	if b.apiKey != "" {
		b.handler = b.authMiddleware(b.mux)
	}

	b.server = &http.Server{
		Addr:              addr,
		Handler:           b.handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout intentionally omitted — SSE connections are long-lived.
	}

	return b
}

// ServeHTTP implements http.Handler, enabling use with httptest.
// Routes through the same handler as ListenAndServe (including auth middleware).
func (b *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.handler.ServeHTTP(w, r)
}

// Start begins serving in a blocking call. Returns http.ErrServerClosed on
// clean shutdown.
func (b *Bridge) Start() error {
	return b.server.ListenAndServe()
}

// Shutdown gracefully stops the server. The broker's poll goroutine and all
// subscriber channels are closed before the HTTP server itself shuts down.
func (b *Bridge) Shutdown(ctx context.Context) error {
	if b.broker != nil {
		b.broker.Close()
	}
	return b.server.Shutdown(ctx)
}

// authMiddleware rejects requests that do not carry a valid Bearer token.
func (b *Bridge) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if !strings.HasPrefix(auth, "Bearer ") || subtle.ConstantTimeCompare([]byte(token), []byte(b.apiKey)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized","code":"unauthenticated"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
