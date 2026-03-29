package httpbridge

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/agent"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

const (
	ssePollInterval = 200 * time.Millisecond
	sseKeepalive    = 15 * time.Second
	sseBatchSize    = 50
)

// handleSSE streams server-sent events to the client.
// Each connection maintains its own cursor via ReadEvents — broadcast semantics.
func (b *Bridge) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // nginx proxy support

	// Initial keepalive
	fmt.Fprint(w, ":ok\n\n")
	flusher.Flush()

	ctx := r.Context()
	cursor := "0"
	if since := r.URL.Query().Get("since"); since != "" {
		cursor = since
	}
	lastKeepalive := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		events, err := b.store.ReadEvents(ctx, cursor, sseBatchSize)
		if err != nil {
			log.Printf("httpbridge: SSE ReadEvents: %v", err)
			time.Sleep(ssePollInterval)
			continue
		}

		for _, se := range events {
			cursor = se.ID
			sseType, data := mapStoreEvent(se.Event)
			if sseType == "" {
				continue
			}
			payload, jErr := json.Marshal(data)
			if jErr != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sseType, payload)
		}

		if len(events) > 0 {
			flusher.Flush()
			lastKeepalive = time.Now()
		}

		// Keepalive if no events for a while
		if time.Since(lastKeepalive) >= sseKeepalive {
			fmt.Fprint(w, ":ping\n\n")
			flusher.Flush()
			lastKeepalive = time.Now()
		}

		if len(events) == 0 {
			time.Sleep(ssePollInterval)
		}
	}
}

// mapStoreEvent converts a store event to an SSE event type and payload.
// Returns empty string if the event should be skipped.
func mapStoreEvent(e state.Event) (string, interface{}) {
	switch e.Type {
	case "agent_registered":
		return "agent.registered", SSEAgentRegistered{
			AgentID:   e.AgentID,
			Timestamp: e.Timestamp,
		}

	case string(agent.EventTaskAssigned):
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "assigned",
			TaskID:    e.TaskID,
			Timestamp: e.Timestamp,
		}

	case string(agent.EventWorkStarted):
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "working",
			Timestamp: e.Timestamp,
		}

	case string(agent.EventOutputReady):
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "reporting",
			Timestamp: e.Timestamp,
		}

	case string(agent.EventOutputDelivered):
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "idle",
			Timestamp: e.Timestamp,
		}

	case string(agent.EventAgentFailed):
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "crashed",
			Timestamp: e.Timestamp,
		}

	case "output_chunk":
		return "agent.output", SSEAgentOutput{
			AgentID:   e.AgentID,
			Payload:   e.Payload,
			Timestamp: e.Timestamp,
		}

	default:
		log.Printf("httpbridge: unrecognised event type %q — skipped", e.Type)
		return "", nil
	}
}
