package state_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

func TestMockStore_Conformance(t *testing.T) {
	store := state.NewMockStore()
	RunStateStoreConformance(t, store)
}

func TestMockStore_PingError(t *testing.T) {
	ctx := context.Background()
	store := state.NewMockStore()

	// Default: Ping returns nil.
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	// Set an error: Ping returns it.
	pingErr := errors.New("redis: connection refused")
	store.SetPingError(pingErr)
	if err := store.Ping(ctx); !errors.Is(err, pingErr) {
		t.Fatalf("expected %v, got %v", pingErr, err)
	}

	// Reset: Ping returns nil again.
	store.SetPingError(nil)
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("expected nil after reset, got %v", err)
	}
}
