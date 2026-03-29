package httpbridge

import (
	"context"
	"net/http"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/ipc"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

// Bridge is a thin HTTP/SSE layer over the orchestrator's core services.
// It wraps the same StateStore, Supervisor, and DAGEngine that the gRPC
// handlers use — no business logic duplication.
type Bridge struct {
	store     state.StateStore
	dag       ipc.DAGEngine
	substrate terminal.Substrate // optional; nil = PTY endpoints return 501

	mux    *http.ServeMux
	server *http.Server
}

// BridgeOption configures the Bridge.
type BridgeOption func(*Bridge)

// WithSubstrate enables terminal-related endpoints (output capture, PTY input).
func WithSubstrate(s terminal.Substrate) BridgeOption {
	return func(b *Bridge) { b.substrate = s }
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

	// REST endpoints
	b.mux.HandleFunc("GET /api/agents", b.handleListAgents)
	b.mux.HandleFunc("GET /api/agents/{id}/output", b.handleAgentOutput)
	b.mux.HandleFunc("GET /api/floors", b.handleListFloors)
	b.mux.HandleFunc("POST /api/tasks", b.handleSubmitTask)
	b.mux.HandleFunc("POST /api/dags", b.handleSubmitDAG)
	b.mux.HandleFunc("POST /api/agents/{id}/input", b.handleSendInput)

	// SSE stream
	b.mux.HandleFunc("GET /events/stream", b.handleSSE)

	b.server = &http.Server{
		Addr:              addr,
		Handler:           b.mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout intentionally omitted — SSE connections are long-lived.
	}

	return b
}

// ServeHTTP implements http.Handler, enabling use with httptest.
func (b *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mux.ServeHTTP(w, r)
}

// Start begins serving in a blocking call. Returns http.ErrServerClosed on
// clean shutdown.
func (b *Bridge) Start() error {
	return b.server.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (b *Bridge) Shutdown(ctx context.Context) error {
	return b.server.Shutdown(ctx)
}
