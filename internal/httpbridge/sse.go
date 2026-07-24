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

// writeSSEEvent renders one StreamEvent onto the wire using the shared
// mapStoreEvent + SSE wire format, flushing after every write.
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, se state.StreamEvent) {
	sseType, data := mapStoreEvent(se.Event, se.ID)
	if sseType == "" {
		return
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sseType, payload)
	flusher.Flush()
}

// handleSSE streams server-sent events to the client.
//
// Every connection sees every event (broadcast semantics), each carrying a
// "cursor" so clients can resume via ?since=. Steady-state delivery comes
// from a single shared Broker — one poll goroutine performs the store read
// for every connection combined (see broker.go). Each connection still does
// a one-time bounded backfill replay from its own ?since= cursor up to the
// broker's cursor snapshot ("cut") taken atomically at Subscribe, so
// per-connection resume semantics are preserved without any connection
// polling the store itself.
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
	since := "0"
	if s := r.URL.Query().Get("since"); s != "" {
		since = s
	}

	sub, cut := b.broker.Subscribe()
	defer b.broker.Unsubscribe(sub)

	// One-time bounded backfill: replay [since..cut] from the store. Only ID
	// EQUALITY is ever tested against cut — store cursor IDs (e.g. Redis
	// "millis-seq" or MockStore "mock-N") are opaque and not safely
	// order-comparable across the state.StateStore abstraction.
	//
	// foundCut starts true when since already equals cut (including the
	// "0"=="0" case where the broker has not advanced past anything yet):
	// there is nothing before cut to replay, and everything the client
	// hasn't seen will arrive live. cut=="0" with since!="0" (a stale/ahead
	// resume cursor after a broker restart) must NOT be treated as
	// found-cut here — that is exactly the gating edge case below.
	//
	// Backfill must run whenever there is a chance of missed history: the
	// normal case (cut != "0", the broker has advanced) OR the resume-after-
	// restart case (cut == "0" but since != "0" — the broker's cursor reset
	// while the store may still hold everything after `since`). Restricting
	// this to cut != "0" (as an earlier version did) skipped backfill
	// entirely on that second branch, arming live-delivery gating on an
	// exact `since` ID match that may never recur if that entry was since
	// trimmed from the store (e.g. Redis XTRIM/MAXLEN) — the connection
	// would then never clear gating and silently starve forever. Store reads
	// use `since`/cursor purely as an exclusive lower bound (matching Redis
	// XREAD semantics), so this works even if the exact ID no longer exists.
	foundCut := since == cut
	lastBackfilled := since
	if !foundCut && cut != "" && (cut != "0" || since != "0") {
		cursor := since
		for {
			events, err := b.store.ReadEvents(ctx, cursor, sseBatchSize)
			if err != nil {
				log.Printf("httpbridge: SSE backfill ReadEvents: %v", err)
				break
			}
			if len(events) == 0 {
				break
			}
			for _, se := range events {
				writeSSEEvent(w, flusher, se)
				cursor = se.ID
				lastBackfilled = se.ID
				if se.ID == cut {
					foundCut = true
					break
				}
			}
			if foundCut {
				break
			}
		}
	}

	// Resume edge (server restart / stale cursor): if backfill never
	// encountered cut, the client's `since` may be ahead of (or unrelated
	// to) the broker's cursor. Gate live delivery until we've seen
	// lastBackfilled, then deliver strictly-subsequent events — reproducing
	// the original per-connection "deliver events strictly after `since`"
	// semantics without ever comparing two IDs with '<'.
	//
	// Gating is only armed when backfill actually delivered at least one
	// event (lastBackfilled != since): that is the only case where the live
	// channel — which, when cut=="0", replays every event from the
	// beginning — could hand this connection a duplicate of something
	// backfill already wrote to the client. If backfill delivered nothing
	// (e.g. `since` referenced an entry that has since been evicted from
	// the store entirely, and the store holds nothing newer either), there
	// is nothing to skip: every live event from here on is provably new, so
	// gating on the never-again-existing `since` ID would just starve the
	// connection forever instead of harmlessly passing everything through.
	gating := !foundCut && since != "0" && lastBackfilled != since
	gateID := lastBackfilled

	lastKeepalive := time.Now()
	keepaliveTicker := time.NewTicker(sseKeepalive)
	defer keepaliveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case se, ok := <-sub.Events():
			if !ok {
				// Broker shut down.
				return
			}
			if gating {
				if se.ID == gateID {
					gating = false
				}
				continue
			}
			writeSSEEvent(w, flusher, se)
			lastKeepalive = time.Now()

		case <-keepaliveTicker.C:
			if time.Since(lastKeepalive) >= sseKeepalive {
				fmt.Fprint(w, ":ping\n\n")
				flusher.Flush()
				lastKeepalive = time.Now()
			}
		}
	}
}

// mapStoreEvent converts a store event to an SSE event type and payload.
// cursor is the Redis stream entry ID, embedded so clients can resume via ?since=.
// Returns empty string if the event should be skipped.
func mapStoreEvent(e state.Event, cursor string) (string, interface{}) {
	switch e.Type {
	case "agent_registered":
		// Agents always register in idle state. Subsequent state transitions
		// arrive as separate agent.state_changed events in the stream, so
		// SSE replay naturally converges to the current state.
		return "agent.registered", SSEAgentRegistered{
			ID:            e.AgentID,
			State:         "idle",
			LastHeartbeat: e.Timestamp,
			RegisteredAt:  e.Timestamp,
			Cursor:        cursor,
		}

	case string(agent.EventTaskAssigned):
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "assigned",
			TaskID:    e.TaskID,
			Timestamp: e.Timestamp,
			Cursor:    cursor,
		}

	case string(agent.EventWorkStarted):
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "working",
			Timestamp: e.Timestamp,
			Cursor:    cursor,
		}

	case string(agent.EventOutputReady):
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "reporting",
			Timestamp: e.Timestamp,
			Cursor:    cursor,
		}

	case string(agent.EventOutputDelivered):
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "idle",
			Timestamp: e.Timestamp,
			Cursor:    cursor,
		}

	case string(agent.EventAgentFailed):
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "crashed",
			Timestamp: e.Timestamp,
			Cursor:    cursor,
		}

	case "task_cancelled":
		// Published by handleCancelAgent/handleReassignAgent (T14 / #119) once
		// the task has been detached from the agent and the agent's state has
		// been driven back to idle in the store. There is no dedicated
		// "task.cancelled" SSE type — this maps onto the same
		// agent.state_changed shape the UI already handles, so BridgeManager
		// picks up the idle transition with no new frontend event handler.
		return "agent.state_changed", SSEAgentStateChanged{
			AgentID:   e.AgentID,
			State:     "idle",
			TaskID:    e.TaskID,
			Timestamp: e.Timestamp,
			Cursor:    cursor,
		}

	case "output_chunk":
		return "agent.output", SSEAgentOutput{
			AgentID:   e.AgentID,
			Payload:   e.Payload,
			Timestamp: e.Timestamp,
			Cursor:    cursor,
		}

	case "floor_created":
		return "floor.created", SSEFloorCreated{
			Name:       e.Payload,
			AgentCount: 0,
			Cursor:     cursor,
		}

	case "floor_removed":
		return "floor.removed", SSEFloorRemoved{
			Name:   e.Payload,
			Cursor: cursor,
		}

	default:
		log.Printf("httpbridge: unrecognised event type %q — skipped", e.Type)
		return "", nil
	}
}
