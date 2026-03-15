package state_test

import (
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

func TestMockStore_Conformance(t *testing.T) {
	store := state.NewMockStore()
	RunStateStoreConformance(t, store)
}
