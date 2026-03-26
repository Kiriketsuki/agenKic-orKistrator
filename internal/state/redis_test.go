//go:build integration

package state_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

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

func TestRedisStore_CompetingConsumers(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	prefix := fmt.Sprintf("test:%s:", uuid.New().String())
	store, err := state.NewRedisStore(redisURL, state.WithKeyPrefix(prefix))
	if err != nil {
		t.Skipf("cannot connect to Redis at %s: %v", redisURL, err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := t.Context()

	// Publish 10 events.
	const total = 10
	for i := 1; i <= total; i++ {
		ev := state.Event{
			Type:    "compete-type",
			AgentID: fmt.Sprintf("cc-agent-%d", i),
			TaskID:  fmt.Sprintf("cc-task-%d", i),
		}
		if err := store.PublishEvent(ctx, ev); err != nil {
			t.Fatalf("PublishEvent %d: %v", i, err)
		}
	}

	if err := store.CreateConsumerGroup(ctx, "competing-test", "0"); err != nil {
		t.Fatalf("CreateConsumerGroup: %v", err)
	}

	// Two consumers collect events concurrently; each loops until a call
	// returns empty (no new events available with a short block).
	type result struct {
		events []state.StreamEvent
	}
	results := make([]result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			consumer := fmt.Sprintf("c%d", idx+1)
			var collected []state.StreamEvent
			for {
				batch, err := store.SubscribeEvents(ctx, "competing-test", consumer, 10, 10*time.Millisecond)
				if err != nil {
					t.Errorf("SubscribeEvents %s: %v", consumer, err)
					return
				}
				if len(batch) == 0 {
					break
				}
				collected = append(collected, batch...)
			}
			results[idx].events = collected
		}(i)
	}
	wg.Wait()

	// Union all received IDs — expect exactly 10 with no duplicates.
	seen := make(map[string]bool)
	for _, r := range results {
		for _, ev := range r.events {
			if seen[ev.ID] {
				t.Fatalf("duplicate event ID %q received by multiple consumers", ev.ID)
			}
			seen[ev.ID] = true
		}
	}
	if len(seen) != total {
		t.Fatalf("want %d distinct events across consumers, got %d (c1=%d, c2=%d)",
			total, len(seen), len(results[0].events), len(results[1].events))
	}
}
