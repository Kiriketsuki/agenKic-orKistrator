//go:build integration

package state

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
)

// TestRedisStore_TaskMetaHasExpiry verifies that the task:<id>:meta hash
// written by EnqueueTaskWithMeta carries a TTL, so submitted task metadata
// does not accumulate unbounded in Redis — nothing in the assignment
// pipeline explicitly deletes it (council finding; see taskMetaTTL and
// tryAssignTask's best-effort GetTaskMeta read in internal/supervisor).
// This lives in package state (not state_test) because it needs the
// unexported client/taskMetaKey to inspect the TTL directly.
func TestRedisStore_TaskMetaHasExpiry(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	prefix := fmt.Sprintf("test:%s:", uuid.New().String())
	store, err := NewRedisStore(redisURL, WithKeyPrefix(prefix))
	if err != nil {
		t.Skipf("cannot connect to Redis at %s: %v", redisURL, err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := t.Context()
	const taskID = "meta-ttl-task"
	if err := store.EnqueueTaskWithMeta(ctx, taskID, 1.0, TaskMeta{
		Description: "scout the eastern ridge",
		Tier:        "opus",
		Provider:    "claude",
	}); err != nil {
		t.Fatalf("EnqueueTaskWithMeta: %v", err)
	}

	meta, err := store.GetTaskMeta(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTaskMeta: %v", err)
	}
	if meta.Description != "scout the eastern ridge" {
		t.Fatalf("meta.Description = %q, want %q", meta.Description, "scout the eastern ridge")
	}
	if meta.Tier != "opus" {
		t.Fatalf("meta.Tier = %q, want %q", meta.Tier, "opus")
	}
	if meta.Provider != "claude" {
		t.Fatalf("meta.Provider = %q, want %q", meta.Provider, "claude")
	}

	ttl, err := store.client.TTL(ctx, store.taskMetaKey(taskID)).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("task meta hash has no expiry (TTL=%v) — will leak forever", ttl)
	}
}
