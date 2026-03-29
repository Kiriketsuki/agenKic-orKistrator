package httpbridge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/httpbridge"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// newTestBridge creates a Bridge backed by a MockStore with no DAG engine.
func newTestBridge(t *testing.T) (*httpbridge.Bridge, *state.MockStore) {
	t.Helper()
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)
	return bridge, store
}

func TestListAgents_Empty(t *testing.T) {
	bridge, _ := newTestBridge(t)

	req := httptest.NewRequest("GET", "/api/agents", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string][]httpbridge.AgentJSON
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp["agents"]) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(resp["agents"]))
	}
}

func TestListAgents_WithAgents(t *testing.T) {
	bridge, store := newTestBridge(t)
	ctx := context.Background()

	// Seed two agents
	_ = store.SetAgentFields(ctx, "agent-1", state.AgentFields{
		State:         "idle",
		LastHeartbeat: 1000,
		RegisteredAt:  900,
	})
	_ = store.SetAgentFields(ctx, "agent-2", state.AgentFields{
		State:         "working",
		CurrentTaskID: "task-42",
		LastHeartbeat: 2000,
		RegisteredAt:  1800,
	})

	req := httptest.NewRequest("GET", "/api/agents", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string][]httpbridge.AgentJSON
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp["agents"]) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp["agents"]))
	}

	// Verify agent fields
	byID := make(map[string]httpbridge.AgentJSON)
	for _, a := range resp["agents"] {
		byID[a.ID] = a
	}
	if byID["agent-2"].State != "working" {
		t.Errorf("expected agent-2 state=working, got %s", byID["agent-2"].State)
	}
	if byID["agent-2"].CurrentTaskID != "task-42" {
		t.Errorf("expected agent-2 task=task-42, got %s", byID["agent-2"].CurrentTaskID)
	}
}

func TestSubmitTask_Valid(t *testing.T) {
	bridge, _ := newTestBridge(t)

	body, _ := json.Marshal(httpbridge.SubmitTaskRequest{
		TaskID:   "task-1",
		Priority: 1.0,
	})
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSubmitTask_MissingID(t *testing.T) {
	bridge, _ := newTestBridge(t)

	body, _ := json.Marshal(httpbridge.SubmitTaskRequest{Priority: 1.0})
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitTask_InvalidBody(t *testing.T) {
	bridge, _ := newTestBridge(t)

	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListFloors_NoSubstrate(t *testing.T) {
	bridge, _ := newTestBridge(t)

	req := httptest.NewRequest("GET", "/api/floors", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAgentOutput_NoSubstrate(t *testing.T) {
	bridge, _ := newTestBridge(t)

	req := httptest.NewRequest("GET", "/api/agents/agent-1/output", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
}

func TestSendInput_NoSubstrate(t *testing.T) {
	bridge, _ := newTestBridge(t)

	body, _ := json.Marshal(httpbridge.SendInputRequest{Keys: "ls\n"})
	req := httptest.NewRequest("POST", "/api/agents/agent-1/input", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
}

func TestSubmitDAG_NilEngine(t *testing.T) {
	bridge, _ := newTestBridge(t) // dag is nil

	body, _ := json.Marshal(httpbridge.SubmitDAGRequest{
		Nodes: []httpbridge.DAGNodeJSON{{NodeID: "n1", TaskID: "t1"}},
	})
	req := httptest.NewRequest("POST", "/api/dags", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	// Handler returns 501 Not Implemented when DAG engine is nil.
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
}

func TestAuth_RejectsWithoutToken(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil, httpbridge.WithAPIKey("test-secret"))

	req := httptest.NewRequest("GET", "/api/agents", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_AcceptsValidToken(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil, httpbridge.WithAPIKey("test-secret"))

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("Authorization", "Bearer test-secret")
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuth_RejectsWrongToken(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil, httpbridge.WithAPIKey("test-secret"))

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestSendInput_EmptyKeys(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)

	body, _ := json.Marshal(httpbridge.SendInputRequest{Keys: ""})
	req := httptest.NewRequest("POST", "/api/agents/agent-1/input", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	// Without substrate, returns 501; with substrate but empty keys, returns 400.
	// Since no substrate is set, this test verifies the 501 path.
	// The empty-keys validation fires only when substrate is present.
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 (no substrate), got %d", w.Code)
	}
}
