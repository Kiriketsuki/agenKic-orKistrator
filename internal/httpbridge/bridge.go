package httpbridge

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"
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

	// spawner is optional; nil means POST /api/agents/spawn returns 501.
	// Wired from main via WithAgentSpawner — spawns a simulated in-process
	// worker agent (demo scaffolding, not a real external agent).
	spawner AgentSpawner

	// names maps agent ID to the fantasy display name chosen by
	// handleSpawnAgent. providers maps agent ID to the spawn kind, such as
	// "claude" or "codex". Both share namesMu and the same lifetime. See
	// setAgentName for the scope and the limitation.
	namesMu   sync.RWMutex
	names     map[string]string
	providers map[string]string

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

// AgentSpawner launches a new worker agent of the given kind ("sim",
// "claude", "codex", "opencode") and returns its agent ID.
type AgentSpawner func(kind, name, tier string) (string, error)

// WithAgentSpawner enables POST /api/agents/spawn, letting the UI summon
// simulated worker agents. Without it the endpoint returns 501.
func WithAgentSpawner(s AgentSpawner) BridgeOption {
	return func(b *Bridge) { b.spawner = s }
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
		store:     store,
		dag:       dag,
		names:     make(map[string]string),
		providers: make(map[string]string),
		mux:       http.NewServeMux(),
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
	b.mux.HandleFunc("POST /api/agents/spawn", b.handleSpawnAgent)

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

// setAgentName records the display name the spawn endpoint picked for
// agentID. The registry lives in the Bridge process only, because
// state.AgentFields has no name field and the gRPC RegisterAgent handler
// drops pb.AgentInfo.Name. Two consequences follow, and both are deliberate
// for now. An orchestrator restart loses every name, and an agent that
// registered through gRPC without going through POST /api/agents/spawn never
// has one. Every consumer falls back to the agent UUID when the name is
// empty.
//
// A further known gap: the agent registers itself during the spawner call,
// so the agent.registered SSE event can reach a client before this line
// runs. The Godot client closes that window by applying the name from the
// spawn response, and every later projection carries the name.
func (b *Bridge) setAgentName(agentID, name string) {
	if agentID == "" || name == "" {
		return
	}
	b.namesMu.Lock()
	b.names[agentID] = name
	b.namesMu.Unlock()
}

// agentName returns the recorded display name for agentID, or an empty
// string when the Bridge never saw a spawn for it.
func (b *Bridge) agentName(agentID string) string {
	b.namesMu.RLock()
	defer b.namesMu.RUnlock()
	return b.names[agentID]
}

// setAgentProvider records the spawn kind ("claude", "codex", ...) for
// agentID. Same in-process scope and limitations as setAgentName.
func (b *Bridge) setAgentProvider(agentID, provider string) {
	if agentID == "" || provider == "" {
		return
	}
	b.namesMu.Lock()
	b.providers[agentID] = provider
	b.namesMu.Unlock()
}

// agentProvider returns the recorded spawn kind for agentID, or an empty
// string when the Bridge never saw a spawn for it.
func (b *Bridge) agentProvider(agentID string) string {
	b.namesMu.RLock()
	defer b.namesMu.RUnlock()
	return b.providers[agentID]
}

// isAgentSession reports whether a terminal session name belongs to a single
// agent's PTY rather than to a tower floor. The supervisor builds these names
// as "agent-" + agentID in RegisterAgent.
func isAgentSession(session string) bool {
	return strings.HasPrefix(session, "agent-")
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
