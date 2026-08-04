package httpbridge_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/httpbridge"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

func TestSSE_InitialOK(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)

	server := httptest.NewServer(bridge)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	scanner := bufio.NewScanner(resp.Body)
	if scanner.Scan() {
		line := scanner.Text()
		if line != ":ok" {
			t.Fatalf("expected initial :ok, got %q", line)
		}
	}
}

func TestSSE_ReceivesAgentRegisteredEvent(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)

	server := httptest.NewServer(bridge)
	defer server.Close()

	// Publish an event before connecting
	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "agent_registered",
		AgentID:   "agent-abc",
		Timestamp: 1234567890,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	foundEvent := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: agent.registered") {
			foundEvent = true
		}
		if foundEvent && strings.HasPrefix(line, "data:") {
			if !strings.Contains(line, "agent-abc") {
				t.Fatalf("expected agent-abc in data, got %s", line)
			}
			return // success
		}
	}

	if !foundEvent {
		t.Fatal("never received agent.registered event")
	}
}

func TestSSE_SinceCursorSkipsPriorEvents(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)

	server := httptest.NewServer(bridge)
	defer server.Close()

	// Publish two events; we will connect with ?since= after the first.
	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "agent_registered",
		AgentID:   "agent-old",
		Timestamp: 1000,
	})
	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "agent_registered",
		AgentID:   "agent-new",
		Timestamp: 2000,
	})

	// Read events to get the first event's cursor ID.
	events, _ := store.ReadEvents(context.Background(), "0", 1)
	if len(events) == 0 {
		t.Fatal("expected at least one event in store")
	}
	firstCursor := events[0].ID

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Connect with ?since= set to the first event's ID — should skip it.
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events/stream?since="+firstCursor, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "agent-old") {
			t.Fatal("received agent-old event — ?since= cursor should have skipped it")
		}
		if strings.Contains(line, "agent-new") {
			return // success — only the second event was received
		}
	}
	t.Fatal("never received agent-new event")
}

// TestSSE_ResumeAfterCursorEvictedDoesNotStarve exercises handleSSE (real
// HTTP, not a hand-rolled reimplementation of its gating logic) in the
// resume-after-restart edge case: a client reconnects with ?since=<oldID>
// while the broker's cursor is still fresh ("0", e.g. right after a
// server/broker restart) AND the store no longer holds `oldID` at all
// (simulating Redis XTRIM/MAXLEN retention, or a non-persistent restart).
//
// Before the fix, gating armed on the literal `since` ID and could only
// clear on an exact match — which never occurs once that ID is evicted, so
// the connection silently received zero events forever despite new events
// continuously arriving. This test fails (times out) against that behavior
// and passes once gating correctly recognises "backfill found nothing to
// replay" as "nothing to skip" rather than "wait for an ID that can never
// reappear."
func TestSSE_ResumeAfterCursorEvictedDoesNotStarve(t *testing.T) {
	store := state.NewMockStore()

	// Publish a "pre-restart" event and capture its ID as the client's
	// last-known cursor.
	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "agent_registered",
		AgentID:   "pre-restart",
		Timestamp: 1000,
	})
	pre, err := store.ReadEvents(context.Background(), "0", 10)
	if err != nil || len(pre) != 1 {
		t.Fatalf("setup: expected 1 event, got %d err=%v", len(pre), err)
	}
	since := pre[0].ID

	// Simulate retention fully evicting the entry the client is resuming
	// from — the store now holds nothing at or after `since`.
	store.TrimEvents(0)
	drained, err := store.ReadEvents(context.Background(), since, 10)
	if err != nil {
		t.Fatalf("sanity ReadEvents: %v", err)
	}
	if len(drained) != 0 {
		t.Fatalf("setup: expected since's entry to be evicted, got %d events", len(drained))
	}

	// A long poll interval guarantees the request below reaches handleSSE
	// (and Subscribe) before the broker's first tick — reproducing the
	// exact cut=="0" && since!="0" branch deterministically rather than
	// relying on a race against the default 200ms interval.
	bridge := httpbridge.NewBridge(":0", store, nil, httpbridge.WithBrokerInterval(500*time.Millisecond))
	server := httptest.NewServer(bridge)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events/stream?since="+since, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// A genuinely new post-restart event. The evicted `since` ID will never
	// reappear live, so this only arrives if resume gating correctly stays
	// off instead of waiting forever for an impossible exact match.
	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "agent_registered",
		AgentID:   "post-restart",
		Timestamp: 2000,
	})

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "post-restart") {
			return // success
		}
	}
	t.Fatal("never received post-restart event — resume gating starved the connection after an evicted cursor")
}

func TestSSE_AgentRegisteredCarriesFullAgentFields(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)

	server := httptest.NewServer(bridge)
	defer server.Close()

	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "agent_registered",
		AgentID:   "agent-full",
		Timestamp: 1700000000,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)
		if !strings.Contains(data, "agent-full") {
			continue
		}
		// Must use "id" key (not "agent_id") so Godot AgentData.from_dict works.
		if !strings.Contains(data, `"id"`) {
			t.Fatalf("expected \"id\" field in agent.registered payload, got %s", data)
		}
		if strings.Contains(data, `"agent_id"`) {
			t.Fatalf("agent.registered must not use \"agent_id\" — Godot reads \"id\": %s", data)
		}
		// Must carry state field (defaults to "idle" for new agents).
		if !strings.Contains(data, `"state":"idle"`) {
			t.Fatalf("expected state=idle in agent.registered payload, got %s", data)
		}
		// Must carry registered_at.
		if !strings.Contains(data, `"registered_at"`) {
			t.Fatalf("expected registered_at in agent.registered payload, got %s", data)
		}
		return // success
	}
	t.Fatal("never received agent.registered data with agent-full")
}

func TestSSE_EventPayloadsIncludeCursor(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)

	server := httptest.NewServer(bridge)
	defer server.Close()

	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "agent_registered",
		AgentID:   "agent-cursor",
		Timestamp: 1700000000,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)
		if !strings.Contains(data, "agent-cursor") {
			continue
		}
		if !strings.Contains(data, `"cursor"`) {
			t.Fatalf("SSE payload missing cursor field — Godot needs it for ?since= resumption: %s", data)
		}
		return // success
	}
	t.Fatal("never received event data with agent-cursor")
}

func TestSSE_ReceivesFloorCreatedEvent(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)

	server := httptest.NewServer(bridge)
	defer server.Close()

	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "floor_created",
		Payload:   "workers-3",
		Timestamp: 1234567890,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	foundEvent := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: floor.created") {
			foundEvent = true
		}
		if foundEvent && strings.HasPrefix(line, "data:") {
			if !strings.Contains(line, `"name":"workers-3"`) {
				t.Fatalf("expected name=workers-3 in data, got %s", line)
			}
			return // success
		}
	}

	if !foundEvent {
		t.Fatal("never received floor.created event")
	}
}

func TestSSE_ReceivesFloorRemovedEvent(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)

	server := httptest.NewServer(bridge)
	defer server.Close()

	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "floor_removed",
		Payload:   "workers-3",
		Timestamp: 1234567890,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	foundEvent := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: floor.removed") {
			foundEvent = true
		}
		if foundEvent && strings.HasPrefix(line, "data:") {
			if !strings.Contains(line, `"name":"workers-3"`) {
				t.Fatalf("expected name=workers-3 in data, got %s", line)
			}
			return // success
		}
	}

	if !foundEvent {
		t.Fatal("never received floor.removed event")
	}
}

func TestSSE_ReceivesStateChangedEvent(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)

	server := httptest.NewServer(bridge)
	defer server.Close()

	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "task_assigned",
		AgentID:   "agent-xyz",
		TaskID:    "task-99",
		Timestamp: 1234567890,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: agent.state_changed") {
			// Next line should be data
			if scanner.Scan() {
				data := scanner.Text()
				if !strings.Contains(data, "assigned") {
					t.Fatalf("expected state=assigned in data, got %s", data)
				}
				if !strings.Contains(data, "task-99") {
					t.Fatalf("expected task-99 in data, got %s", data)
				}
				return
			}
		}
	}
	t.Fatal("never received agent.state_changed event")
}

// A floor event for a per-agent tmux session must never reach the UI, which
// would otherwise draw one tower floor per interactive CLI agent (Task D).
func TestSSE_SkipsAgentSessionFloorEvents(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil)

	server := httptest.NewServer(bridge)
	defer server.Close()

	ctx := context.Background()
	_ = store.PublishEvent(ctx, state.Event{Type: "floor_created", Payload: "agent-abc", Timestamp: 1})
	_ = store.PublishEvent(ctx, state.Event{Type: "floor_removed", Payload: "agent-abc", Timestamp: 2})
	_ = store.PublishEvent(ctx, state.Event{Type: "floor_created", Payload: "archive", Timestamp: 3})

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(reqCtx, "GET", server.URL+"/events/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "agent-abc") {
			t.Fatalf("agent session leaked into the SSE stream: %s", line)
		}
		if strings.Contains(line, "archive") {
			return // the real floor still arrives
		}
	}
	t.Fatal("never received the floor.created event for archive")
}

// The fantasy name recorded at spawn time rides along on agent SSE payloads
// so the UI can label a worker without a REST refresh (Task D).
func TestSSE_AgentEventsCarrySpawnName(t *testing.T) {
	store := state.NewMockStore()
	bridge := httpbridge.NewBridge(":0", store, nil,
		httpbridge.WithAgentSpawner(func(_, _, _ string) (string, error) { return "agent-77", nil }))

	server := httptest.NewServer(bridge)
	defer server.Close()

	spawnResp, err := http.Post(server.URL+"/api/agents/spawn", "application/json",
		strings.NewReader(`{"kind":"sim","name":"Thornquill"}`))
	if err != nil {
		t.Fatal(err)
	}
	spawnResp.Body.Close()

	_ = store.PublishEvent(context.Background(), state.Event{
		Type:      "agent_registered",
		AgentID:   "agent-77",
		Timestamp: 1,
	})

	reqCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(reqCtx, "GET", server.URL+"/events/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") || !strings.Contains(line, "agent-77") {
			continue
		}
		if !strings.Contains(line, `"name":"Thornquill"`) {
			t.Fatalf("agent payload missing the spawn name: %s", line)
		}
		return
	}
	t.Fatal("never received an agent payload for agent-77")
}
