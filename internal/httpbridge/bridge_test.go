package httpbridge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/httpbridge"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

// stubDAGEngine is a minimal ipc.DAGEngine implementation that records the
// last spec it was asked to execute, for assertions on node construction.
type stubDAGEngine struct {
	lastSpec *pb.DAGSpec
	execID   string
}

func (s *stubDAGEngine) Execute(_ context.Context, spec *pb.DAGSpec) (string, error) {
	s.lastSpec = spec
	if s.execID == "" {
		s.execID = "exec-1"
	}
	return s.execID, nil
}

func (s *stubDAGEngine) Status(_ context.Context, _ string) (*pb.GetDAGStatusResponse, error) {
	return &pb.GetDAGStatusResponse{}, nil
}

// stubSubstrate is a minimal terminal.Substrate implementation for testing
// handlers that require a non-nil substrate.
type stubSubstrate struct{}

func (s *stubSubstrate) SpawnSession(_ context.Context, _ string, _ string) (terminal.Session, error) {
	return terminal.Session{}, nil
}
func (s *stubSubstrate) DestroySession(_ context.Context, _ string) error { return nil }
func (s *stubSubstrate) SendCommand(_ context.Context, _ string, _ string) error {
	return nil
}
func (s *stubSubstrate) CaptureOutput(_ context.Context, _ string, _ int) (string, error) {
	return "", nil
}
func (s *stubSubstrate) ListSessions(_ context.Context) ([]terminal.Session, error) {
	return nil, nil
}
func (s *stubSubstrate) SplitPane(_ context.Context, _ string, _ terminal.Direction) (terminal.Pane, error) {
	return terminal.Pane{}, nil
}

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
	// TaskID may be omitted as long as Description is present — the server
	// generates a task_id and returns it. See #118.
	bridge, _ := newTestBridge(t)

	body, _ := json.Marshal(httpbridge.SubmitTaskRequest{
		Priority:    1.0,
		Description: "scout the eastern ridge",
	})
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["task_id"] == "" {
		t.Fatal("expected a generated task_id, got empty string")
	}
}

func TestSubmitTask_EmptyBody(t *testing.T) {
	// Neither task_id nor description supplied — must be rejected.
	bridge, _ := newTestBridge(t)

	body, _ := json.Marshal(httpbridge.SubmitTaskRequest{Priority: 1.0})
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSubmitTask_WithMeta(t *testing.T) {
	bridge, store := newTestBridge(t)

	body, _ := json.Marshal(httpbridge.SubmitTaskRequest{
		TaskID:      "task-meta-1",
		Priority:    1.0,
		Description: "clear the goblin warren",
		Project:     "agenKic-orKistrator",
		Floor:       "floor-2",
	})
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	meta, err := store.GetTaskMeta(context.Background(), "task-meta-1")
	if err != nil {
		t.Fatalf("GetTaskMeta: %v", err)
	}
	if meta.Description != "clear the goblin warren" {
		t.Errorf("expected description to roundtrip, got %q", meta.Description)
	}
	if meta.Project != "agenKic-orKistrator" {
		t.Errorf("expected project to roundtrip, got %q", meta.Project)
	}
	if meta.Floor != "floor-2" {
		t.Errorf("expected floor to roundtrip, got %q", meta.Floor)
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

func TestSubmitDAG_DescriptionMapsToPrompt(t *testing.T) {
	store := state.NewMockStore()
	engine := &stubDAGEngine{}
	bridge := httpbridge.NewBridge(":0", store, engine)

	body, _ := json.Marshal(httpbridge.SubmitDAGRequest{
		Nodes: []httpbridge.DAGNodeJSON{
			{NodeID: "n1", Description: "gather herbs"},
			{NodeID: "n2", TaskID: "t2", Description: "brew potion"},
		},
		Edges: []httpbridge.DAGEdgeJSON{{From: "n1", To: "n2"}},
	})
	req := httptest.NewRequest("POST", "/api/dags", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if engine.lastSpec == nil {
		t.Fatal("expected DAG engine to receive a spec")
	}
	if len(engine.lastSpec.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(engine.lastSpec.Nodes))
	}

	n1 := engine.lastSpec.Nodes[0]
	if n1.Task.Prompt != "gather herbs" {
		t.Errorf("expected node n1 prompt %q, got %q", "gather herbs", n1.Task.Prompt)
	}
	if n1.Task.TaskId == "" {
		t.Error("expected node n1 to receive a generated task_id")
	}

	n2 := engine.lastSpec.Nodes[1]
	if n2.Task.Prompt != "brew potion" {
		t.Errorf("expected node n2 prompt %q, got %q", "brew potion", n2.Task.Prompt)
	}
	if n2.Task.TaskId != "t2" {
		t.Errorf("expected node n2 to keep explicit task_id %q, got %q", "t2", n2.Task.TaskId)
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

func TestSendInput_EmptyKeys_WithSubstrate(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil, httpbridge.WithSubstrate(&stubSubstrate{}))

	body, _ := json.Marshal(httpbridge.SendInputRequest{Keys: ""})
	req := httptest.NewRequest("POST", "/api/agents/agent-1/input", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	// With substrate present, empty keys should be rejected with 400.
	// This exercises the validation at handlers.go:200-206.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (empty keys), got %d", w.Code)
	}
}
