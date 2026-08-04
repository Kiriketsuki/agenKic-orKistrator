package httpbridge

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

	// supervisor is optional; nil means POST /api/agents/{id}/despawn skips
	// the supervisor-side cleanup (per-agent mutex, cooldown, and
	// circuit-breaker map entries, and the agent's tmux session) and only
	// removes the agent from the store and the Bridge's own name/provider/
	// floor registries. Wired via WithSupervisor.
	supervisor *supervisor.Supervisor

	// restartFn is optional; nil means POST /api/admin/restart responds 202
	// but performs no actual restart. Wired from main via WithRestartFunc —
	// it runs the graceful shutdown sequence and then re-execs the binary.
	// Kept as an injectable function so a test can assert the endpoint
	// triggers the shutdown path without ever calling syscall.Exec.
	restartFn func()

	// names maps agent ID to the fantasy display name chosen by
	// handleSpawnAgent. providers maps agent ID to the spawn kind, such as
	// "claude" or "codex". Both share namesMu and the same lifetime. See
	// setAgentName for the scope and the limitation.
	//
	// Persistence stays deferred by design (finding 9): none of names,
	// providers, or floors survives a process restart, including a
	// GUI-triggered one via POST /api/admin/restart. The agents themselves
	// survive in the store (state.StateStore is external), but a restart
	// wipes every name, provider, and floor assignment this process
	// remembered for them. A client reads back the bare agent UUID and floor
	// 0 for every agent that existed before the restart, until it re-spawns
	// or the registries gain real persistence.
	namesMu   sync.RWMutex
	names     map[string]string
	providers map[string]string
	// floors maps agent ID to the tower floor it spawned on, and also holds
	// reservation placeholders (see reservationPrefix) between
	// reserveFloorSlot/reserveNextAvailableFloor and their commit or
	// release. Every spawn now records a floor, explicit or auto-assigned
	// (finding 8). Same in-process scope and reset-on-restart policy as
	// names and providers.
	floors map[string]int

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

// WithSupervisor wires the orchestrator's Supervisor into the Bridge, so
// POST /api/agents/{id}/despawn can call Supervisor.RemoveAgent to clean up
// the agent's tmux session and per-agent bookkeeping. Optional: without it,
// despawn still deletes the agent from the store and the Bridge's own
// registries, but leaves any supervisor-side state and tmux session behind.
func WithSupervisor(sv *supervisor.Supervisor) BridgeOption {
	return func(b *Bridge) { b.supervisor = sv }
}

// WithRestartFunc wires the function POST /api/admin/restart runs after
// responding 202. The Bridge always responds 202 first and calls restartFn
// from a separate goroutine after a short delay, so a restart never blocks
// the HTTP response and a slow or hanging restartFn never wedges the
// handler. Optional: without it, the endpoint still responds 202 but
// performs no restart.
func WithRestartFunc(restartFn func()) BridgeOption {
	return func(b *Bridge) { b.restartFn = restartFn }
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
		floors:    make(map[string]int),
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
	b.mux.HandleFunc("GET /api/providers", b.handleListProviders)
	b.mux.HandleFunc("POST /api/tasks", b.handleSubmitTask)
	b.mux.HandleFunc("POST /api/dags", b.handleSubmitDAG)
	b.mux.HandleFunc("POST /api/agents/{id}/input", b.handleSendInput)
	b.mux.HandleFunc("POST /api/agents/{id}/cancel", b.handleCancelAgent)
	b.mux.HandleFunc("POST /api/agents/{id}/reassign", b.handleReassignAgent)
	b.mux.HandleFunc("POST /api/agents/{id}/despawn", b.handleDespawnAgent)
	b.mux.HandleFunc("POST /api/agents/spawn", b.handleSpawnAgent)
	b.mux.HandleFunc("POST /api/admin/restart", b.handleRestartAdmin)

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

// floorCapacity is the maximum number of desks EdgeLayout renders per floor.
// The Godot side reads the same value from floor_scene.gd's own constant, so
// keep both in step per the original floor spec.
const floorCapacity = 4

// groundFloor is reserved for the future orKistrator archmage. It never
// accepts a spawn and never counts toward any floor's desk capacity.
const groundFloor = 0

// setAgentFloor records the tower floor agentID spawned on. Same in-process
// scope and reset-on-restart policy as setAgentName.
func (b *Bridge) setAgentFloor(agentID string, floor int) {
	if agentID == "" {
		return
	}
	b.namesMu.Lock()
	b.floors[agentID] = floor
	b.namesMu.Unlock()
}

// agentFloor returns the recorded floor for agentID, or 0 when the Bridge
// never assigned one (auto-placement was used, or the agent registered
// outside POST /api/agents/spawn).
func (b *Bridge) agentFloor(agentID string) int {
	b.namesMu.RLock()
	defer b.namesMu.RUnlock()
	return b.floors[agentID]
}

// floorAgentCount returns how many agents the Bridge has assigned to floor,
// used to enforce floorCapacity at spawn time.
func (b *Bridge) floorAgentCount(floor int) int {
	b.namesMu.RLock()
	defer b.namesMu.RUnlock()
	count := 0
	for _, f := range b.floors {
		if f == floor {
			count++
		}
	}
	return count
}

// reservationPrefix marks a placeholder entry in b.floors: a slot claimed
// before the spawner runs, not yet a real agent ID. reserveFloorSlot and
// releaseFloorReservation use this prefix so a placeholder counts toward
// floorAgentCount exactly like a real agent, closing the TOCTOU window
// where two concurrent spawns both read the floor as having room.
const reservationPrefix = "\x00reservation:"

// reserveFloorSlot atomically checks floor's capacity and claims one slot
// under a single lock, closing the gap between floorAgentCount's read and
// setAgentFloor's later write. It returns a reservation token to pass to
// commitFloorReservation on spawner success or releaseFloorReservation on
// spawner failure. ok is false when the floor is already at floorCapacity,
// in which case no slot was claimed.
func (b *Bridge) reserveFloorSlot(floor int) (token string, ok bool) {
	b.namesMu.Lock()
	defer b.namesMu.Unlock()

	count := 0
	for _, f := range b.floors {
		if f == floor {
			count++
		}
	}
	if count >= floorCapacity {
		return "", false
	}

	n := reservationCounter.Add(1)
	token = reservationPrefix + strconv.FormatInt(n, 10)
	b.floors[token] = floor
	return token, true
}

// commitFloorReservation converts a reservation token into the real agentID
// once the spawner has succeeded, so agentFloor and floorAgentCount report
// the agent under its actual ID from this point on.
func (b *Bridge) commitFloorReservation(token, agentID string, floor int) {
	if agentID == "" {
		return
	}
	b.namesMu.Lock()
	delete(b.floors, token)
	b.floors[agentID] = floor
	b.namesMu.Unlock()
}

// releaseFloorReservation frees a slot claimed by reserveFloorSlot when the
// spawner call fails, so the floor's capacity count reflects reality again.
func (b *Bridge) releaseFloorReservation(token string) {
	b.namesMu.Lock()
	delete(b.floors, token)
	b.namesMu.Unlock()
}

// reservationCounter hands out unique reservation tokens for reserveFloorSlot.
var reservationCounter atomic.Int64

// reserveNextAvailableFloor picks the lowest floor >= 1 that has room under
// floorCapacity, reserves a slot on it under one lock, and returns the floor
// and its reservation token. When every floor in use is full, it opens the
// next floor index (finding 8: an omitted floor must still land somewhere
// the bridge tracks, so auto-assigned agents count toward capacity exactly
// like explicit-floor ones).
func (b *Bridge) reserveNextAvailableFloor() (floor int, token string) {
	b.namesMu.Lock()
	defer b.namesMu.Unlock()

	counts := make(map[int]int)
	highest := groundFloor
	for _, f := range b.floors {
		counts[f]++
		if f > highest {
			highest = f
		}
	}

	floor = highest + 1
	for f := 1; f <= highest; f++ {
		if counts[f] < floorCapacity {
			floor = f
			break
		}
	}

	n := reservationCounter.Add(1)
	token = reservationPrefix + strconv.FormatInt(n, 10)
	b.floors[token] = floor
	return floor, token
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
