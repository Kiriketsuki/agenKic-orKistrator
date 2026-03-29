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
