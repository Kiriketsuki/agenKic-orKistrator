package httpbridge

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
)

// handleListAgents returns all registered agents with their full state.
func (b *Bridge) handleListAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	ids, err := b.store.ListAgents(ctx)
	if err != nil {
		writeError(w, err)
		return
	}

	agents := make([]AgentJSON, 0, len(ids))
	var warnings []string
	for _, id := range ids {
		fields, fErr := b.store.GetAgentFields(ctx, id)
		if fErr != nil {
			log.Printf("httpbridge: GetAgentFields %s: %v", id, fErr)
			warnings = append(warnings, "failed to load agent "+id)
			continue
		}
		agents = append(agents, AgentJSON{
			ID:            id,
			State:         fields.State,
			CurrentTaskID: fields.CurrentTaskID,
			LastHeartbeat: fields.LastHeartbeat,
			RegisteredAt:  fields.RegisteredAt,
		})
	}

	resp := map[string]interface{}{"agents": agents}
	if len(warnings) > 0 {
		resp["warnings"] = warnings
		resp["partial"] = true
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleAgentOutput returns captured terminal output for an agent.
func (b *Bridge) handleAgentOutput(w http.ResponseWriter, r *http.Request) {
	if b.substrate == nil {
		writeJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error: "terminal substrate not available",
			Code:  "not_implemented",
		})
		return
	}

	agentID := r.PathValue("id")
	linesParam := r.URL.Query().Get("lines")
	lines := 50
	if linesParam != "" {
		if n, err := strconv.Atoi(linesParam); err == nil && n > 0 {
			lines = n
		}
		if lines > 1000 {
			lines = 1000
		}
	}

	output, err := b.substrate.CaptureOutput(r.Context(), "agent-"+agentID, lines)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"output": output})
}

// handleListFloors lists terminal sessions as floors.
func (b *Bridge) handleListFloors(w http.ResponseWriter, r *http.Request) {
	if b.substrate == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"floors": []FloorJSON{}})
		return
	}

	sessions, err := b.substrate.ListSessions(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	floors := make([]FloorJSON, 0, len(sessions))
	for _, s := range sessions {
		floors = append(floors, FloorJSON{
			Name:       s.Name,
			AgentCount: s.WindowCount,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"floors": floors})
}

// handleSubmitTask enqueues a task via the store.
func (b *Bridge) handleSubmitTask(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req SubmitTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
			Code:  "invalid_argument",
		})
		return
	}
	if req.TaskID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "task_id is required",
			Code:  "invalid_argument",
		})
		return
	}

	if err := b.store.EnqueueTask(r.Context(), req.TaskID, req.Priority); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"task_id": req.TaskID})
}

// handleSubmitDAG submits a DAG for execution.
func (b *Bridge) handleSubmitDAG(w http.ResponseWriter, r *http.Request) {
	if b.dag == nil {
		writeJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error: "DAG engine not available",
			Code:  "not_implemented",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req SubmitDAGRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
			Code:  "invalid_argument",
		})
		return
	}

	spec := &pb.DAGSpec{
		Nodes: make([]*pb.DAGNode, 0, len(req.Nodes)),
		Edges: make([]*pb.DAGEdge, 0, len(req.Edges)),
	}
	for _, n := range req.Nodes {
		spec.Nodes = append(spec.Nodes, &pb.DAGNode{
			NodeId: n.NodeID,
			Task: &pb.TaskSpec{
				TaskId:   n.TaskID,
				Priority: n.Priority,
			},
		})
	}
	for _, e := range req.Edges {
		spec.Edges = append(spec.Edges, &pb.DAGEdge{
			FromNode: e.From,
			ToNode:   e.To,
		})
	}

	execID, err := b.dag.Execute(r.Context(), spec)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"dag_execution_id": execID})
}

// handleSendInput sends keystrokes to an agent's terminal session.
func (b *Bridge) handleSendInput(w http.ResponseWriter, r *http.Request) {
	if b.substrate == nil {
		writeJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error: "terminal substrate not available",
			Code:  "not_implemented",
		})
		return
	}

	agentID := r.PathValue("id")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req SendInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
			Code:  "invalid_argument",
		})
		return
	}

	if req.Keys == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "keys is required",
			Code:  "invalid_argument",
		})
		return
	}

	if err := b.substrate.SendCommand(r.Context(), "agent-"+agentID, req.Keys); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
