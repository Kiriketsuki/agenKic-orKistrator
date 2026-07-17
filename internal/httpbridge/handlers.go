package httpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/google/uuid"
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
//
// TaskID is optional — a description-only submission (the quest-board "quick
// quest" flow, #118) generates a server-side UUID. At least one of TaskID or
// Description must be present; a fully-empty body is rejected.
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

	taskID := strings.TrimSpace(req.TaskID)
	description := strings.TrimSpace(req.Description)
	if taskID == "" && description == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "description is required",
			Code:  "invalid_argument",
		})
		return
	}
	if taskID == "" {
		taskID = uuid.New().String()
	}

	meta := state.TaskMeta{
		Description: description,
		Project:     strings.TrimSpace(req.Project),
		Floor:       strings.TrimSpace(req.Floor),
	}
	if err := b.store.EnqueueTaskWithMeta(r.Context(), taskID, req.Priority, meta); err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"task_id": taskID})
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
		taskID := strings.TrimSpace(n.TaskID)
		if taskID == "" {
			taskID = uuid.New().String()
		}
		spec.Nodes = append(spec.Nodes, &pb.DAGNode{
			NodeId: n.NodeID,
			Task: &pb.TaskSpec{
				TaskId:   taskID,
				Prompt:   n.Description,
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

// handleCancelAgent cancels the agent's current task (T14 / #119).
//
// Semantics (honest-minimal — see TaskMeta.Tier/Provider doc comment for the
// companion reassign endpoint's caveat): the Bridge holds only a StateStore
// and an optional terminal.Substrate — no Supervisor and no agent.Machine
// reference — so this cannot drive the agent state machine's
// EventAgentFailed transition (the only machine-modeled path to a terminal
// state, which the UI would render as "crashed"). Cancellation is instead
// performed directly against the store:
//
//  1. best-effort PTY interrupt (Ctrl-C, "\x03") via the terminal substrate,
//     if one is configured. This is logged-only on failure and is NOT the
//     mechanism this endpoint depends on for correctness — the store detach
//     in step 2 is.
//  2. ClearCurrentTask detaches the task from the agent unconditionally.
//  3. If the agent's last-observed state was not already idle,
//     CompareAndSetAgentState drives it from that state to idle. Losing the
//     compare-and-swap race (a concurrent supervisor transition) is
//     tolerated: a re-read that finds the agent already idle is treated as
//     success (the store's end state already matches this endpoint's
//     promise); any other observed state is reported as 409 "aborted" so the
//     caller can retry.
//  4. A "task_cancelled" event is published so SSE subscribers see the
//     agent's idle transition live (mapped to agent.state_changed by
//     mapStoreEvent — no new SSE event type or frontend handler needed).
//  5. If a *supervisor.CompletionRegistry was wired in via
//     WithCompletionRegistry, Complete(taskID) is called so a
//     dag.BlockingSubmitter.Wait blocking on this exact task (i.e. taskID
//     belongs to a DAG node) unblocks instead of hanging until the DAG's own
//     context is cancelled (T14 council finding #2). CompletionRegistry has
//     no notion of "completed via cancellation" vs. "completed normally" —
//     Complete just unblocks every waiter — so a cancelled DAG node is
//     observed by the DAG engine as having finished successfully with no
//     output, not as having failed. This is an honest limitation of the
//     current registry API, not a design goal; a step-in change to signal
//     failure would require widening CompletionRegistry's contract, which is
//     out of scope here. Reassign does NOT need this: it re-enqueues the
//     same taskID, so some agent eventually completes it and the normal
//     completeAgent path signals Complete(taskID) as usual.
//
// This bypasses the supervisor's per-agent mutex entirely (the Bridge has no
// way to take it), so a narrow race with a concurrent
// tryAssignTask/completeAgent is possible — e.g. a transient idle-with-
// stale-CurrentTaskID state, or a duplicate re-enqueue if a crash fires
// between the GetAgentFields read and ClearCurrentTask. These mirror races
// the supervisor already tolerates and self-heals via heartbeat; this
// endpoint does not introduce a new class of them.
func (b *Bridge) handleCancelAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentID := r.PathValue("id")

	fields, err := b.store.GetAgentFields(ctx, agentID)
	if err != nil {
		writeError(w, err)
		return
	}
	if fields.CurrentTaskID == "" {
		writeJSON(w, http.StatusConflict, ErrorResponse{
			Error: "agent has no active task",
			Code:  "failed_precondition",
		})
		return
	}

	prevState := fields.State
	taskID := fields.CurrentTaskID

	if b.substrate != nil {
		if serr := b.substrate.SendCommand(ctx, "agent-"+agentID, "\x03"); serr != nil {
			log.Printf("httpbridge: cancel agent %s: best-effort PTY interrupt failed: %v", agentID, serr)
		}
	}

	if err := b.store.ClearCurrentTask(ctx, agentID); err != nil {
		writeError(w, err)
		return
	}

	if prevState != state.AgentStateIdle {
		if aborted := b.settleToIdle(ctx, w, agentID, prevState); aborted {
			return
		}
	}

	if err := b.store.PublishEvent(ctx, state.Event{
		Type:    "task_cancelled",
		AgentID: agentID,
		TaskID:  taskID,
	}); err != nil {
		log.Printf("httpbridge: cancel agent %s: PublishEvent: %v", agentID, err)
	}

	if b.completionRegistry != nil {
		// Unblock any dag.BlockingSubmitter.Wait(ctx, taskID) — see the
		// completionRegistry field/WithCompletionRegistry doc comments for
		// the honest-minimal semantics (T14 council finding #2).
		b.completionRegistry.Complete(taskID)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agent_id":  agentID,
		"task_id":   taskID,
		"cancelled": true,
	})
}

// handleReassignAgent requeues the agent's current task with a tier/provider
// hint (T14 / #119).
//
// Semantics: there is no provider/tier field anywhere in state.AgentFields
// and the supervisor's assign loop dequeues strictly by taskID+priority,
// ignoring TaskMeta entirely when picking which agent services a task. This
// endpoint is therefore NOT live migration of a running agent to a different
// provider or tier — nothing here can force the requeued task onto a
// different agent, and it is frequently the very same agent that ends up
// picking it back up. What actually happens:
//
//  1. The agent's current task's existing metadata (description/project/
//     floor) is read via GetTaskMeta so it survives the requeue.
//  2. Best-effort PTY interrupt, exactly as in handleCancelAgent.
//  3. EnqueueTaskWithMeta re-enqueues the task at its original priority, with
//     Tier/Provider persisted into TaskMeta — a hint that is stored but,
//     like TaskMeta.Project/Floor, not consumed by the assign loop today.
//     This runs BEFORE ClearCurrentTask (deliberately the reverse of an
//     earlier version of this handler) so that if the enqueue fails, the
//     task is still fully attached to this agent and nothing is lost — the
//     handler can simply be retried, and the supervisor's own crash-recovery
//     path (crashAgent, which re-enqueues from the still-intact
//     CurrentTaskID) remains a safety net. This mirrors completeAgent's own
//     documented ordering (signal/enqueue BEFORE clearing CurrentTaskID),
//     chosen there for the identical reason: clearing first and then failing
//     to enqueue orphans the task permanently (T14 council finding #3).
//  4. ClearCurrentTask detaches the task from the agent now that the requeue
//     has landed. Trade-off: because this Bridge endpoint runs outside the
//     supervisor's per-agent mutex (see handleCancelAgent's doc comment), a
//     concurrent crash-triggered re-enqueue (heartbeat timeout on this same
//     agent) landing in the narrow window between steps 3 and 4 could
//     observe the still-attached CurrentTaskID and re-enqueue the same task
//     a second time — a duplicate-delivery risk, not a loss risk. Given the
//     choice between a rare duplicate and a guaranteed loss on any
//     transient enqueue error, this endpoint accepts the former, consistent
//     with the rest of the codebase's stated preference.
//  5. The agent is settled back to idle (same CAS-with-tolerant-reread
//     handling as cancel) and a "task_cancelled" event is published so SSE
//     subscribers observe the idle transition live.
//
// Overstating this as live provider/tier reassignment would be fantasy
// plumbing; callers must treat the response as "requeued with a hint",
// nothing more.
func (b *Bridge) handleReassignAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentID := r.PathValue("id")

	fields, err := b.store.GetAgentFields(ctx, agentID)
	if err != nil {
		writeError(w, err)
		return
	}
	if fields.CurrentTaskID == "" {
		writeJSON(w, http.StatusConflict, ErrorResponse{
			Error: "agent has no active task",
			Code:  "failed_precondition",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req ReassignAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
			Code:  "invalid_argument",
		})
		return
	}
	tier := strings.TrimSpace(req.Tier)
	provider := strings.TrimSpace(req.Provider)
	if tier == "" && provider == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "tier or provider is required",
			Code:  "invalid_argument",
		})
		return
	}

	prevState := fields.State
	taskID := fields.CurrentTaskID
	priority := fields.CurrentTaskPriority

	existingMeta, err := b.store.GetTaskMeta(ctx, taskID)
	if err != nil {
		writeError(w, err)
		return
	}

	if b.substrate != nil {
		if serr := b.substrate.SendCommand(ctx, "agent-"+agentID, "\x03"); serr != nil {
			log.Printf("httpbridge: reassign agent %s: best-effort PTY interrupt failed: %v", agentID, serr)
		}
	}

	newMeta := state.TaskMeta{
		Description: existingMeta.Description,
		Project:     existingMeta.Project,
		Floor:       existingMeta.Floor,
		Tier:        tier,
		Provider:    provider,
	}
	// Enqueue BEFORE detaching from the agent — see doc comment above (T14
	// council finding #3): if this fails, the task is still fully attached
	// to agentID and nothing is lost.
	if err := b.store.EnqueueTaskWithMeta(ctx, taskID, priority, newMeta); err != nil {
		writeError(w, err)
		return
	}

	if err := b.store.ClearCurrentTask(ctx, agentID); err != nil {
		writeError(w, err)
		return
	}

	if prevState != state.AgentStateIdle {
		if aborted := b.settleToIdle(ctx, w, agentID, prevState); aborted {
			return
		}
	}

	if err := b.store.PublishEvent(ctx, state.Event{
		Type:    "task_cancelled",
		AgentID: agentID,
		TaskID:  taskID,
	}); err != nil {
		log.Printf("httpbridge: reassign agent %s: PublishEvent: %v", agentID, err)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agent_id": agentID,
		"task_id":  taskID,
		"tier":     tier,
		"provider": provider,
		"requeued": true,
	})
}

// settleToIdle drives agentID from prevState to state.AgentStateIdle via
// CompareAndSetAgentState, tolerating a lost race against a concurrent
// transition IF that transition already landed the agent on idle (the
// store's end state then already satisfies this endpoint's contract). Any
// other observed state, or a non-conflict error, writes an error response
// and returns aborted=true so the caller must stop processing.
func (b *Bridge) settleToIdle(ctx context.Context, w http.ResponseWriter, agentID, prevState string) (aborted bool) {
	err := b.store.CompareAndSetAgentState(ctx, agentID, prevState, state.AgentStateIdle)
	if err == nil {
		return false
	}

	var conflict *state.StateConflictError
	if !errors.As(err, &conflict) {
		writeError(w, err)
		return true
	}

	current, readErr := b.store.GetAgentState(ctx, agentID)
	if readErr == nil && current == state.AgentStateIdle {
		return false
	}
	writeJSON(w, http.StatusConflict, ErrorResponse{
		Error: "agent state changed concurrently; retry",
		Code:  "aborted",
	})
	return true
}
