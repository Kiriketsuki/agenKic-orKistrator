package ipc

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/health"
)

// HealthHTTPServer serves /healthz, /readyz, and /progress over HTTP.
type HealthHTTPServer struct {
	aggregator *health.Aggregator
	server     *http.Server
	mux        *http.ServeMux
}

// NewHealthHTTPServer creates a server bound to addr that delegates to agg.
func NewHealthHTTPServer(addr string, agg *health.Aggregator) *HealthHTTPServer {
	h := &HealthHTTPServer{
		aggregator: agg,
		mux:        http.NewServeMux(),
	}
	h.mux.HandleFunc("/healthz", h.handleHealthz)
	h.mux.HandleFunc("/readyz", h.handleReadyz)
	h.mux.HandleFunc("/progress", h.handleProgress)

	h.server = &http.Server{
		Addr:              addr,
		Handler:           h.mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return h
}

// ServeHTTP implements http.Handler, enabling use with httptest.
func (h *HealthHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// Start begins serving in a blocking call. It returns http.ErrServerClosed on
// clean shutdown.
func (h *HealthHTTPServer) Start() error {
	return h.server.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (h *HealthHTTPServer) Shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

// handleHealthz always returns 200 alive — Alive is unconditionally true.
func (h *HealthHTTPServer) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	writeJSON(w, map[string]string{"status": "alive"})
}

func (h *HealthHTTPServer) handleReadyz(w http.ResponseWriter, r *http.Request) {
	snap := h.aggregator.Check(r.Context())

	w.Header().Set("Content-Type", "application/json")
	if snap.Ready {
		// Ready implies RedisOK; the inner redis-status conditional is dead code.
		w.WriteHeader(http.StatusOK)
		writeJSON(w, map[string]interface{}{
			"status": "ready",
			"agents": snap.AgentsTotal,
			"redis":  "ok",
		})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]string{
			"status": "not_ready",
			"reason": snap.ReadyReason,
		})
	}
}

func (h *HealthHTTPServer) handleProgress(w http.ResponseWriter, r *http.Request) {
	snap := h.aggregator.Check(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	writeJSON(w, map[string]interface{}{
		"agents_total":     snap.AgentsTotal,
		"agents_idle":      snap.AgentsIdle,
		"agents_working":   snap.AgentsWorking,
		"agents_assigned":  snap.AgentsAssigned,
		"agents_reporting": snap.AgentsReporting,
		"agents_unknown":   snap.AgentsUnknown,
		"tasks_queued":     snap.TasksQueued,
		"tasks_in_flight":  snap.TasksInFlight,
		"dags_in_progress": snap.DAGsInProgress,
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	_ = json.NewEncoder(w).Encode(v)
}
