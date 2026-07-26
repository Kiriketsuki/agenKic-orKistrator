package httpbridge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/httpbridge"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

// errFakeEnqueue is injected via MockStore.SetEnqueueTaskError to simulate a
// transient store failure for TestReassignAgent_EnqueueFailure_TaskNotLost.
var errFakeEnqueue = errors.New("fake enqueue failure")

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

// recordingSubstrate wraps stubSubstrate but records every SendCommand call
// (session, cmd) for assertions on the best-effort PTY interrupt sent by
// handleCancelAgent/handleReassignAgent (T14 / #119).
type recordingSubstrate struct {
	stubSubstrate
	sentSession string
	sentCmd     string
}

func (s *recordingSubstrate) SendCommand(_ context.Context, session string, cmd string) error {
	s.sentSession = session
	s.sentCmd = cmd
	return nil
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

// ── Cancel agent task (T14 / #119) ──────────────────────────────────────────

func TestCancelAgent_UnknownAgent(t *testing.T) {
	bridge, _ := newTestBridge(t)

	req := httptest.NewRequest("POST", "/api/agents/ghost/cancel", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCancelAgent_NoActiveTask(t *testing.T) {
	bridge, store := newTestBridge(t)
	ctx := context.Background()

	_ = store.SetAgentFields(ctx, "agent-1", state.AgentFields{State: "idle"})

	req := httptest.NewRequest("POST", "/api/agents/agent-1/cancel", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCancelAgent_Success(t *testing.T) {
	store := state.NewMockStore()
	substrate := &recordingSubstrate{}
	bridge := httpbridge.NewBridge(":0", store, nil, httpbridge.WithSubstrate(substrate))
	ctx := context.Background()

	_ = store.SetAgentFields(ctx, "agent-1", state.AgentFields{
		State:         "working",
		CurrentTaskID: "task-1",
	})

	req := httptest.NewRequest("POST", "/api/agents/agent-1/cancel", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["task_id"] != "task-1" {
		t.Errorf("expected task_id=task-1 in response, got %v", resp["task_id"])
	}
	if resp["cancelled"] != true {
		t.Errorf("expected cancelled=true in response, got %v", resp["cancelled"])
	}

	fields, err := store.GetAgentFields(ctx, "agent-1")
	if err != nil {
		t.Fatalf("GetAgentFields: %v", err)
	}
	if fields.State != "idle" {
		t.Errorf("expected agent state=idle after cancel, got %q", fields.State)
	}
	if fields.CurrentTaskID != "" {
		t.Errorf("expected CurrentTaskID cleared after cancel, got %q", fields.CurrentTaskID)
	}

	if substrate.sentSession != "agent-agent-1" {
		t.Errorf("expected SendCommand session %q, got %q", "agent-agent-1", substrate.sentSession)
	}
	if substrate.sentCmd != "\x03" {
		t.Errorf("expected SendCommand cmd %q (Ctrl-C), got %q", "\\x03", substrate.sentCmd)
	}
}

func TestCancelAgent_NoSubstrate_StillDetaches(t *testing.T) {
	bridge, store := newTestBridge(t)
	ctx := context.Background()

	_ = store.SetAgentFields(ctx, "agent-1", state.AgentFields{
		State:         "working",
		CurrentTaskID: "task-1",
	})

	req := httptest.NewRequest("POST", "/api/agents/agent-1/cancel", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	fields, err := store.GetAgentFields(ctx, "agent-1")
	if err != nil {
		t.Fatalf("GetAgentFields: %v", err)
	}
	if fields.State != "idle" || fields.CurrentTaskID != "" {
		t.Errorf("expected idle+detached with no substrate, got state=%q task=%q", fields.State, fields.CurrentTaskID)
	}
}

// TestCancelAgent_SignalsCompletionRegistry verifies the T14 council finding
// #2 fix: cancelling an agent whose current task a
// dag.BlockingSubmitter.Wait is blocked on must unblock that wait via
// CompletionRegistry.Complete, rather than stranding it forever.
func TestCancelAgent_SignalsCompletionRegistry(t *testing.T) {
	store := state.NewMockStore()
	registry := supervisor.NewCompletionRegistry()
	bridge := httpbridge.NewBridge(":0", store, nil, httpbridge.WithCompletionRegistry(registry))
	ctx := context.Background()

	_ = store.SetAgentFields(ctx, "agent-1", state.AgentFields{
		State:         "working",
		CurrentTaskID: "task-1",
	})

	waitErrCh := make(chan error, 1)
	go func() {
		waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		waitErrCh <- registry.Wait(waitCtx, "task-1")
	}()

	// Give the goroutine a moment to register as a waiter before cancelling.
	time.Sleep(20 * time.Millisecond)

	req := httptest.NewRequest("POST", "/api/agents/agent-1/cancel", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	select {
	case err := <-waitErrCh:
		if err != nil {
			t.Fatalf("expected Wait to return nil (unblocked by cancel), got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("registry.Wait was never unblocked by cancel — DAG node would hang forever")
	}
}

// TestCancelAgent_NoCompletionRegistry_StillWorks verifies the
// completionRegistry field is genuinely optional — cancel must not panic or
// otherwise misbehave when no registry was wired in (the common case for
// deployments/tests that never call WithCompletionRegistry).
func TestCancelAgent_NoCompletionRegistry_StillWorks(t *testing.T) {
	bridge, store := newTestBridge(t)
	ctx := context.Background()

	_ = store.SetAgentFields(ctx, "agent-1", state.AgentFields{
		State:         "working",
		CurrentTaskID: "task-1",
	})

	req := httptest.NewRequest("POST", "/api/agents/agent-1/cancel", nil)
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Reassign agent task (T14 / #119) ────────────────────────────────────────

func TestReassignAgent_UnknownAgent(t *testing.T) {
	bridge, _ := newTestBridge(t)

	body, _ := json.Marshal(httpbridge.ReassignAgentRequest{Provider: "gemini"})
	req := httptest.NewRequest("POST", "/api/agents/ghost/reassign", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReassignAgent_NoActiveTask(t *testing.T) {
	bridge, store := newTestBridge(t)
	ctx := context.Background()

	_ = store.SetAgentFields(ctx, "agent-1", state.AgentFields{State: "idle"})

	body, _ := json.Marshal(httpbridge.ReassignAgentRequest{Provider: "gemini"})
	req := httptest.NewRequest("POST", "/api/agents/agent-1/reassign", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReassignAgent_EmptyTierAndProvider(t *testing.T) {
	bridge, store := newTestBridge(t)
	ctx := context.Background()

	_ = store.SetAgentFields(ctx, "agent-1", state.AgentFields{
		State:         "working",
		CurrentTaskID: "task-1",
	})

	body, _ := json.Marshal(httpbridge.ReassignAgentRequest{})
	req := httptest.NewRequest("POST", "/api/agents/agent-1/reassign", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReassignAgent_Success(t *testing.T) {
	bridge, store := newTestBridge(t)
	ctx := context.Background()

	_ = store.SetAgentFields(ctx, "agent-1", state.AgentFields{
		State:               "working",
		CurrentTaskID:       "task-1",
		CurrentTaskPriority: 3.0,
	})
	// Existing description/project/floor metadata must survive the requeue.
	if err := store.EnqueueTaskWithMeta(ctx, "task-1", 3.0, state.TaskMeta{
		Description: "scout the eastern ridge",
		Project:     "agenKic-orKistrator",
		Floor:       "floor-1",
	}); err != nil {
		t.Fatalf("seed EnqueueTaskWithMeta: %v", err)
	}
	// Drain the seeded queue entry — CurrentTaskID/CurrentTaskPriority on the
	// agent record (set above) is what the handler actually reads.
	if _, _, err := store.DequeueTask(ctx); err != nil {
		t.Fatalf("drain seeded queue entry: %v", err)
	}

	body, _ := json.Marshal(httpbridge.ReassignAgentRequest{Tier: "opus", Provider: "claude"})
	req := httptest.NewRequest("POST", "/api/agents/agent-1/reassign", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	qlen, err := store.QueueLength(ctx)
	if err != nil {
		t.Fatalf("QueueLength: %v", err)
	}
	if qlen != 1 {
		t.Fatalf("expected task requeued (queue length 1), got %d", qlen)
	}

	meta, err := store.GetTaskMeta(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetTaskMeta: %v", err)
	}
	if meta.Tier != "opus" {
		t.Errorf("expected meta.Tier=opus, got %q", meta.Tier)
	}
	if meta.Provider != "claude" {
		t.Errorf("expected meta.Provider=claude, got %q", meta.Provider)
	}
	if meta.Description != "scout the eastern ridge" {
		t.Errorf("expected description preserved, got %q", meta.Description)
	}
	if meta.Project != "agenKic-orKistrator" {
		t.Errorf("expected project preserved, got %q", meta.Project)
	}
	if meta.Floor != "floor-1" {
		t.Errorf("expected floor preserved, got %q", meta.Floor)
	}

	fields, err := store.GetAgentFields(ctx, "agent-1")
	if err != nil {
		t.Fatalf("GetAgentFields: %v", err)
	}
	if fields.State != "idle" {
		t.Errorf("expected agent state=idle after reassign, got %q", fields.State)
	}
	if fields.CurrentTaskID != "" {
		t.Errorf("expected CurrentTaskID cleared after reassign, got %q", fields.CurrentTaskID)
	}
}

// TestReassignAgent_EnqueueFailure_TaskNotLost verifies the T14 council
// finding #3 fix: when EnqueueTaskWithMeta fails, the task must still be
// attached to the agent (CurrentTaskID intact) rather than orphaned —
// EnqueueTaskWithMeta now runs BEFORE ClearCurrentTask.
func TestReassignAgent_EnqueueFailure_TaskNotLost(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)
	ctx := context.Background()

	_ = store.SetAgentFields(ctx, "agent-1", state.AgentFields{
		State:               "working",
		CurrentTaskID:       "task-1",
		CurrentTaskPriority: 3.0,
	})
	store.SetEnqueueTaskError(errFakeEnqueue)

	body, _ := json.Marshal(httpbridge.ReassignAgentRequest{Provider: "gemini"})
	req := httptest.NewRequest("POST", "/api/agents/agent-1/reassign", bytes.NewReader(body))
	w := httptest.NewRecorder()
	bridge.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on enqueue failure, got %d: %s", w.Code, w.Body.String())
	}

	fields, err := store.GetAgentFields(ctx, "agent-1")
	if err != nil {
		t.Fatalf("GetAgentFields: %v", err)
	}
	if fields.CurrentTaskID != "task-1" {
		t.Errorf("expected task-1 to remain attached to agent-1 after failed enqueue (task must not be lost), got CurrentTaskID=%q", fields.CurrentTaskID)
	}
	if fields.State != "working" {
		t.Errorf("expected agent state unchanged (working) after failed enqueue, got %q", fields.State)
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
