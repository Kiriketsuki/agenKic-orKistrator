package httpbridge

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
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
	apiKey    string             // optional; empty = no auth required

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

// Shutdown gracefully stops the server.
func (b *Bridge) Shutdown(ctx context.Context) error {
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
