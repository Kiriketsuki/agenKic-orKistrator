package state_test

import (
	"errors"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

func TestStateConflictError_ErrorMessage(t *testing.T) {
	err := &state.StateConflictError{Expected: "idle", Actual: "working"}
	want := "state conflict: expected \"idle\", got \"working\""
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestStateConflictError_ErrorsAs(t *testing.T) {
	err := &state.StateConflictError{Expected: "idle", Actual: "working"}
	var target *state.StateConflictError
	if !errors.As(err, &target) {
		t.Fatal("errors.As should match *StateConflictError")
	}
	if target.Expected != "idle" || target.Actual != "working" {
		t.Fatalf("fields: Expected=%q Actual=%q", target.Expected, target.Actual)
	}
}
