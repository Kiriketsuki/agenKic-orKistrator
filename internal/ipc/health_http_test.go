package ipc_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/health"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/ipc"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// newHealthServer creates a HealthHTTPServer using httptest-style handler
// invocation (no real port binding).
func newHealthServer(store *state.MockStore, executor *dag.Executor) *ipc.HealthHTTPServer {
	agg := health.NewAggregator(store, executor)
	return ipc.NewHealthHTTPServer(":0", agg)
}

func TestHealthz_Alive(t *testing.T) {
	store := state.NewMockStore()
	executor := dag.NewExecutor(context.Background(), dag.NewStoreSubmitter(store))
	srv := newHealthServer(store, executor)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "alive" {
		t.Errorf("status = %q, want 'alive'", body["status"])
	}
}

func TestReadyz_NotReady_NoAgents(t *testing.T) {
	store := state.NewMockStore()
	executor := dag.NewExecutor(context.Background(), dag.NewStoreSubmitter(store))
	srv := newHealthServer(store, executor)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "not_ready" {
		t.Errorf("status = %q, want 'not_ready'", body["status"])
	}
	if body["reason"] == "" {
		t.Error("expected non-empty reason")
	}
}

func TestReadyz_Ready(t *testing.T) {
	store := state.NewMockStore()
	ctx := context.Background()
	_ = store.SetAgentState(ctx, "agent-1", "idle")

	executor := dag.NewExecutor(ctx, dag.NewStoreSubmitter(store))
	srv := newHealthServer(store, executor)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ready" {
		t.Errorf("status = %q, want 'ready'", body["status"])
	}
	if body["agents"].(float64) != 1 {
		t.Errorf("agents = %v, want 1", body["agents"])
	}
	if body["redis"] != "ok" {
		t.Errorf("redis = %q, want 'ok'", body["redis"])
	}
}

func TestReadyz_NotReady_RedisDown(t *testing.T) {
	store := state.NewMockStore()
	ctx := context.Background()
	_ = store.SetAgentState(ctx, "agent-1", "idle")
	store.SetPingError(errors.New("connection refused"))

	executor := dag.NewExecutor(ctx, dag.NewStoreSubmitter(store))
	srv := newHealthServer(store, executor)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "not_ready" {
		t.Errorf("status = %q, want 'not_ready'", body["status"])
	}
	if body["reason"] == "" {
		t.Error("expected non-empty reason mentioning redis")
	}
}

func TestProgress_Counts(t *testing.T) {
	store := state.NewMockStore()
	ctx := context.Background()
	_ = store.SetAgentState(ctx, "agent-1", "idle")
	_ = store.SetAgentState(ctx, "agent-2", "working")
	_ = store.EnqueueTask(ctx, "task-1", 1.0)
	_ = store.EnqueueTask(ctx, "task-2", 2.0)
	_ = store.EnqueueTask(ctx, "task-3", 3.0)

	executor := dag.NewExecutor(ctx, dag.NewStoreSubmitter(store))
	srv := newHealthServer(store, executor)

	req := httptest.NewRequest(http.MethodGet, "/progress", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)

	check := func(key string, want float64) {
		t.Helper()
		if body[key].(float64) != want {
			t.Errorf("%s = %v, want %v", key, body[key], want)
		}
	}
	check("agents_total", 2)
	check("agents_idle", 1)
	check("agents_working", 1)
	check("agents_assigned", 0)
	check("agents_reporting", 0)
	check("agents_unknown", 0)
	check("tasks_queued", 3)
	check("tasks_in_flight", 1)
	check("dags_in_progress", 0)

	checkBool := func(key string, want bool) {
		t.Helper()
		if body[key].(bool) != want {
			t.Errorf("%s = %v, want %v", key, body[key], want)
		}
	}
	checkBool("agent_data_valid", true)
	checkBool("queue_data_valid", true)
	checkBool("redis_ping_ok", true)
}

func TestProgress_Empty(t *testing.T) {
	store := state.NewMockStore()
	executor := dag.NewExecutor(context.Background(), dag.NewStoreSubmitter(store))
	srv := newHealthServer(store, executor)

	req := httptest.NewRequest(http.MethodGet, "/progress", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	for _, key := range []string{"agents_total", "agents_idle", "agents_working", "agents_assigned", "agents_reporting", "agents_unknown", "tasks_queued", "tasks_in_flight", "dags_in_progress"} {
		if body[key].(float64) != 0 {
			t.Errorf("%s = %v, want 0", key, body[key])
		}
	}
	for _, key := range []string{"agent_data_valid", "queue_data_valid", "redis_ping_ok"} {
		if body[key].(bool) != true {
			t.Errorf("%s = %v, want true", key, body[key])
		}
	}
}

// TestProgress_SentinelFields_Failure verifies that agent_data_valid and
// queue_data_valid serialize as false over HTTP when the underlying store
// calls fail, while the response still returns 200 (data endpoint contract).
func TestProgress_SentinelFields_Failure(t *testing.T) {
	store := state.NewMockStore()
	store.SetGetAllAgentStatesError(errors.New("injected agent error"))
	store.SetQueueLengthError(errors.New("injected queue error"))
	executor := dag.NewExecutor(context.Background(), dag.NewStoreSubmitter(store))
	srv := newHealthServer(store, executor)

	req := httptest.NewRequest(http.MethodGet, "/progress", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (data endpoint always returns 200)", rec.Code)
	}
	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)

	if body["agent_data_valid"].(bool) != false {
		t.Errorf("agent_data_valid = %v, want false when GetAllAgentStates errors", body["agent_data_valid"])
	}
	if body["queue_data_valid"].(bool) != false {
		t.Errorf("queue_data_valid = %v, want false when QueueLength errors", body["queue_data_valid"])
	}
	// Ping succeeds in this test — only store ops fail.
	if body["redis_ping_ok"].(bool) != true {
		t.Errorf("redis_ping_ok = %v, want true (Ping succeeds even when store ops fail)", body["redis_ping_ok"])
	}
}

// TestProgress_RedisPingFailed verifies that redis_ping_ok serializes as false
// over HTTP when Ping returns an error, while the endpoint still returns 200.
func TestProgress_RedisPingFailed(t *testing.T) {
	store := state.NewMockStore()
	store.SetPingError(errors.New("connection refused"))
	executor := dag.NewExecutor(context.Background(), dag.NewStoreSubmitter(store))
	srv := newHealthServer(store, executor)

	req := httptest.NewRequest(http.MethodGet, "/progress", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (data endpoint always returns 200)", rec.Code)
	}
	var body map[string]interface{}
	_ = json.NewDecoder(rec.Body).Decode(&body)

	if body["redis_ping_ok"].(bool) != false {
		t.Errorf("redis_ping_ok = %v, want false when Ping errors", body["redis_ping_ok"])
	}
}

// TestHealthEndpoints_MethodNotAllowed verifies that POST requests to all three
// health endpoints are rejected with 405 Method Not Allowed (Go 1.22 routing).
func TestHealthEndpoints_MethodNotAllowed(t *testing.T) {
	store := state.NewMockStore()
	executor := dag.NewExecutor(context.Background(), dag.NewStoreSubmitter(store))
	srv := newHealthServer(store, executor)

	for _, path := range []string{"/healthz", "/readyz", "/progress"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s: status = %d, want 405", path, rec.Code)
		}
	}
}
