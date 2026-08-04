package httpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	pb "github.com/Kiriketsuki/agenKic-orKistrator/gen/pb/orchestrator"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
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
			Name:          b.agentName(id),
			Provider:      b.agentProvider(id),
			State:         fields.State,
			CurrentTaskID: fields.CurrentTaskID,
			LastHeartbeat: fields.LastHeartbeat,
			RegisteredAt:  fields.RegisteredAt,
			Floor:         b.agentFloor(id),
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

	// screen=1 asks for the visible pane instead of scrollback history. A
	// redrawing TUI overwrites one screen, so history capture repeats frames.
	// A substrate without the capability falls back to history capture.
	session := "agent-" + agentID
	var output string
	var err error
	if screenParam := r.URL.Query().Get("screen"); screenParam == "1" || screenParam == "true" {
		if sc, ok := b.substrate.(terminal.ScreenCapturer); ok {
			output, err = sc.CaptureScreen(r.Context(), session)
		} else {
			output, err = b.substrate.CaptureOutput(r.Context(), session, lines)
		}
	} else {
		output, err = b.substrate.CaptureOutput(r.Context(), session, lines)
	}
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
		// Per-agent tmux sessions are the agent's own PTY, not a tower
		// floor. The supervisor names them "agent-<uuid>" in RegisterAgent,
		// and the UI renders every floor as a room in the tower, so listing
		// them here turns each interactive CLI agent into a spurious floor.
		if isAgentSession(s.Name) {
			continue
		}
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

	text := req.Text()
	if req.Key != "" && text != "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "key and input are mutually exclusive",
			Code:  "invalid_argument",
		})
		return
	}

	// A key request forwards one key press. A text request types a line and
	// then presses Enter. The two paths never mix in one call.
	if req.Key != "" {
		if err := terminal.ValidateKeyName(req.Key); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: err.Error(),
				Code:  "invalid_argument",
			})
			return
		}
		if err := b.substrate.SendKey(r.Context(), "agent-"+agentID, req.Key); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if text == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "input or key is required",
			Code:  "invalid_argument",
		})
		return
	}

	if err := b.substrate.SendCommand(r.Context(), "agent-"+agentID, text); err != nil {
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

// restartDelay is how long handleRestartAdmin waits, after writing the 202
// response, before it calls restartFn. The delay gives the response time to
// reach the client before the process shuts down and re-execs.
const restartDelay = 200 * time.Millisecond

// handleDespawnAgent removes one agent everywhere (F4 / power controls).
//
// This reuses handleCancelAgent's PTY-interrupt and task-detach steps, but
// drops the task instead of leaving it cancellable-and-idle: despawn is a
// permanent removal, so no code path here ever calls EnqueueTask for the
// dropped task. Unlike handleCancelAgent, an agent with no current task is
// not an error. The order matters for the partial-failure case (T1 risk in
// the spec): the tmux session destroy (inside Supervisor.RemoveAgent) and
// the store delete run before the Bridge's own name/provider/floor registry
// deletes, and the agent_deregistered event publishes last, regardless of
// whether an earlier step logged a failure. This way a client that only
// observes the SSE stream never sees agent.deregistered before the agent is
// actually gone from the substrate and the store.
//
// Idempotent: despawning an agent ID the store no longer knows about returns
// 404 and calls nothing else.
func (b *Bridge) handleDespawnAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentID := r.PathValue("id")

	fields, err := b.store.GetAgentFields(ctx, agentID)
	if err != nil {
		writeError(w, err)
		return
	}

	if fields.CurrentTaskID != "" {
		taskID := fields.CurrentTaskID
		if b.substrate != nil {
			if serr := b.substrate.SendCommand(ctx, "agent-"+agentID, "\x03"); serr != nil {
				log.Printf("httpbridge: despawn agent %s: best-effort PTY interrupt failed: %v", agentID, serr)
			}
		}
		if err := b.store.ClearCurrentTask(ctx, agentID); err != nil {
			log.Printf("httpbridge: despawn agent %s: ClearCurrentTask: %v", agentID, err)
		}
		if b.completionRegistry != nil {
			// Unblock any dag.BlockingSubmitter.Wait(ctx, taskID), mirroring
			// handleCancelAgent. Without this a DAG node waiting on a
			// despawned agent's task hangs forever (finding 6, mirrors T14
			// council finding #2).
			b.completionRegistry.Complete(taskID)
		}
	}

	if b.supervisor != nil {
		b.supervisor.RemoveAgent(ctx, agentID)
	}

	if err := b.store.DeleteAgent(ctx, agentID); err != nil {
		log.Printf("httpbridge: despawn agent %s: DeleteAgent: %v", agentID, err)
	}

	b.namesMu.Lock()
	delete(b.names, agentID)
	delete(b.providers, agentID)
	delete(b.floors, agentID)
	b.namesMu.Unlock()

	if err := b.store.PublishEvent(ctx, state.Event{
		Type:    "agent_deregistered",
		AgentID: agentID,
	}); err != nil {
		log.Printf("httpbridge: despawn agent %s: PublishEvent: %v", agentID, err)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agent_id":  agentID,
		"despawned": true,
	})
}

// handleRestartAdmin responds 202 immediately, then runs restartFn (the
// graceful shutdown sequence followed by a re-exec of the same binary) from
// a separate goroutine after restartDelay. Responding before the restart
// runs is deliberate: the process may shut down its own HTTP listener
// before the client reads the response otherwise (T4 risk in the spec).
// With no restartFn wired (WithRestartFunc never called), the endpoint still
// responds 202 but performs no restart.
//
// A GUI-triggered restart re-execs into a fresh process, so the Bridge's
// names, providers, and floors registries start empty again even though the
// agents themselves survive in the store (finding 9, persistence deferred by
// design — see the Bridge struct field docs).
func (b *Bridge) handleRestartAdmin(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusAccepted)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	if b.restartFn == nil {
		return
	}
	restartFn := b.restartFn
	go func() {
		time.Sleep(restartDelay)
		restartFn()
	}()
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

// Fantasy names cycled by handleSpawnAgent when the request omits one.
var spawnNames = []string{
	"Emberwick", "Thornquill", "Moonshard", "Grimtome", "Silverbough",
	"Ashveil", "Runeholt", "Duskmantle", "Brightforge", "Stormvellum",
}

// providerRoster is the single source of truth for the spawn-kind sigils the
// Grimoire flyout draws from. It shares its kind list with the switch in
// handleSpawnAgent, so a new adapter here appears in the GUI with no GUI
// change, per the Grimoire Summoning spec.
var providerRoster = []ProviderJSON{
	{Kind: "sim", Display: "Simulacrum", Accent: "#8892b0"},
	{Kind: "claude", Display: "Claude", Accent: "#cc785c"},
	{Kind: "codex", Display: "Codex", Accent: "#10a37f"},
	{Kind: "opencode", Display: "OpenCode", Accent: "#4f8cff"},
	{Kind: "pi", Display: "Pi", Accent: "#b967ff"},
}

// handleListProviders returns the spawn-kind roster the Grimoire flyout uses
// to build its sigil grid. The Godot config page filters and orders this
// list locally, so the bridge always serves the full set.
func (b *Bridge) handleListProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"providers": providerRoster})
}

var spawnCounter atomic.Int64

// handleSpawnAgent launches a simulated in-process worker agent (demo
// scaffolding). Returns 501 unless an AgentSpawner was wired in.
func (b *Bridge) handleSpawnAgent(w http.ResponseWriter, r *http.Request) {
	if b.spawner == nil {
		writeJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error: "agent spawner not available",
			Code:  "not_implemented",
		})
		return
	}

	var req SpawnAgentRequest
	if r.Body != nil {
		// Empty body is fine — everything is defaultable.
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	n := spawnCounter.Add(1)
	switch req.Kind {
	case "":
		req.Kind = "sim"
	case "sim", "claude", "codex", "opencode", "pi":
	default:
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "kind must be one of: sim, claude, codex, opencode, pi",
			Code:  "invalid_argument",
		})
		return
	}
	if req.Name == "" {
		req.Name = spawnNames[int(n-1)%len(spawnNames)]
	}
	switch req.Tier {
	case "haiku", "sonnet", "opus":
	default:
		req.Tier = []string{"haiku", "sonnet", "opus"}[int(n)%3]
	}

	// Workdir is validated at the boundary: it must name an existing
	// directory by absolute path, because the spawner cd's the agent's
	// shell into it and a bad path would only surface as a broken pane.
	if req.Workdir != "" {
		if !filepath.IsAbs(req.Workdir) {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: "workdir must be an absolute path",
				Code:  "invalid_argument",
			})
			return
		}
		info, err := os.Stat(req.Workdir)
		if err != nil || !info.IsDir() {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: "workdir does not exist or is not a directory",
				Code:  "invalid_argument",
			})
			return
		}
	}

	// Floor 0 (ground) is reserved for the future archmage and never accepts
	// a spawn. A nonzero floor must have room under floorCapacity. An
	// omitted floor makes the bridge pick the lowest non-full floor >= 1
	// itself (see nextAvailableFloor), so every agent ends up with a
	// bridge-side floor and the capacity check counts everyone (finding 8).
	//
	// The slot is reserved atomically here, before the spawner call, and
	// only committed to the real agent ID on spawner success. This closes
	// the TOCTOU window where two concurrent spawns both read the floor as
	// having room, then both write past floorCapacity (finding 5).
	var floor int
	var token string
	var reserved bool
	if req.Floor != nil {
		floor = *req.Floor
		if floor <= groundFloor {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				Error: "floor 0 is reserved for the archmage and never accepts a spawn",
				Code:  "invalid_argument",
			})
			return
		}
		var ok bool
		token, ok = b.reserveFloorSlot(floor)
		if !ok {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: "floor is full",
				Code:  "floor_full",
			})
			return
		}
		reserved = true
	} else {
		floor, token = b.reserveNextAvailableFloor()
		reserved = true
	}

	agentID, err := b.spawner(req.Kind, req.Name, req.Tier, req.Workdir)
	if err != nil {
		if reserved {
			b.releaseFloorReservation(token)
		}
		writeJSON(w, http.StatusBadGateway, ErrorResponse{
			Error: err.Error(),
			Code:  "spawn_failed",
		})
		return
	}
	// Record the fantasy name so every later agent projection (REST list and
	// SSE payloads) can show it instead of the raw UUID. See
	// Bridge.setAgentName for the honest limitation of this registry.
	b.setAgentName(agentID, req.Name)
	// The provider shown in the UI is the spawn kind. Same registry
	// lifetime and gaps as the name.
	b.setAgentProvider(agentID, req.Kind)
	if reserved {
		b.commitFloorReservation(token, agentID, floor)
	}
	writeJSON(w, http.StatusOK, SpawnAgentResponse{
		AgentID: agentID,
		Kind:    req.Kind,
		Name:    req.Name,
		Tier:    req.Tier,
		Floor:   floor,
		Workdir: req.Workdir,
	})
}
