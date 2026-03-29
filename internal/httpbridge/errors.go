package httpbridge

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/dag"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/supervisor"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/terminal"
)

// writeError maps domain errors to HTTP status codes and writes an ErrorResponse.
func writeError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	resp := ErrorResponse{Error: "internal error", Code: "internal"}

	var invalidTx *agent.InvalidTransitionError
	switch {
	case errors.Is(err, state.ErrAgentNotFound):
		code = http.StatusNotFound
		resp = ErrorResponse{Error: "agent not found", Code: "not_found"}
	case errors.Is(err, state.ErrQueueEmpty):
		code = http.StatusNotFound
		resp = ErrorResponse{Error: "queue empty", Code: "not_found"}
	case errors.Is(err, supervisor.ErrSupervisorStopped):
		code = http.StatusServiceUnavailable
		resp = ErrorResponse{Error: "supervisor stopped", Code: "unavailable"}
	case errors.Is(err, supervisor.ErrInvalidAgentID):
		code = http.StatusBadRequest
		resp = ErrorResponse{Error: err.Error(), Code: "invalid_argument"}
	case errors.Is(err, terminal.ErrSessionNotFound):
		code = http.StatusNotFound
		resp = ErrorResponse{Error: "session not found", Code: "not_found"}
	case errors.Is(err, terminal.ErrInvalidCommand):
		code = http.StatusBadRequest
		resp = ErrorResponse{Error: "invalid command", Code: "invalid_argument"}
	case errors.As(err, &invalidTx):
		code = http.StatusConflict
		resp = ErrorResponse{Error: err.Error(), Code: "failed_precondition"}
	case errors.Is(err, dag.ErrEmptyDAG),
		errors.Is(err, dag.ErrCycleDetected),
		errors.Is(err, dag.ErrNodeNotFound),
		errors.Is(err, dag.ErrDuplicateNode),
		errors.Is(err, dag.ErrMissingTaskSpec):
		code = http.StatusBadRequest
		resp = ErrorResponse{Error: err.Error(), Code: "invalid_argument"}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
