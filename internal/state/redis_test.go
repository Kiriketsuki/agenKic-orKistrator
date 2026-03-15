//go:build integration

package state_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
	"github.com/google/uuid"
)

func TestRedisStore_Conformance(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	// Each test run gets a unique key prefix to avoid cross-test pollution.
	prefix := fmt.Sprintf("test:%s:", uuid.New().String())

	store, err := state.NewRedisStore(redisURL, state.WithKeyPrefix(prefix))
	if err != nil {
		t.Skipf("cannot connect to Redis at %s: %v", redisURL, err)
	}
	t.Cleanup(func() { store.Close() })

	RunStateStoreConformance(t, store)
}
