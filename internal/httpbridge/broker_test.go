package httpbridge_test

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/httpbridge"
	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// countingStore wraps a *state.MockStore and counts ReadEvents calls, so
// tests can assert the broker performs exactly one store read per poll
// interval regardless of how many subscribers are attached.
type countingStore struct {
	*state.MockStore
	reads int64
}

func newCountingStore() *countingStore {
	return &countingStore{MockStore: state.NewMockStore()}
}

func (c *countingStore) ReadEvents(ctx context.Context, lastID string, count int64) ([]state.StreamEvent, error) {
	atomic.AddInt64(&c.reads, 1)
	return c.MockStore.ReadEvents(ctx, lastID, count)
}

func (c *countingStore) Reads() int64 { return atomic.LoadInt64(&c.reads) }

const (
	testBrokerInterval = 30 * time.Millisecond
	testBrokerBatch    = 50
	testBrokerBufSize  = 4 // small on purpose for the slow-consumer test
)

func settle(d time.Duration) { time.Sleep(d) }

func TestBroker_FanOutToMultipleSubscribers(t *testing.T) {
	store := newCountingStore()
	b := httpbridge.NewBroker(store, testBrokerInterval, testBrokerBatch, brokerTestBufSizeLarge)
	defer b.Close()

	const n = 3
	subs := make([]*httpbridge.Subscription, n)
	for i := 0; i < n; i++ {
		sub, _ := b.Subscribe()
		subs[i] = sub
	}

	settle(2 * testBrokerInterval)

	if err := store.PublishEvent(context.Background(), state.Event{Type: "agent_registered", AgentID: "agent-fanout"}); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}

	for i, sub := range subs {
		select {
		case ev := <-sub.Events():
			if ev.Event.AgentID != "agent-fanout" {
				t.Fatalf("subscriber %d: expected agent-fanout, got %q", i, ev.Event.AgentID)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("subscriber %d: never received fanned-out event", i)
		}
	}
}

func TestBroker_SlowConsumerDroppedWithoutBlockingOthers(t *testing.T) {
	store := newCountingStore()
	// A small poll batch spreads the burst across several ticks so the fast
	// subscriber's drain goroutine gets scheduled time between deliveries —
	// mirroring how events trickle in against a real Redis stream rather
	// than arriving as one instantaneous in-process burst.
	const slowTestBatch = 3
	b := httpbridge.NewBroker(store, testBrokerInterval, slowTestBatch, testBrokerBufSize)
	defer b.Close()

	slow, _ := b.Subscribe() // never drained
	fast, _ := b.Subscribe()

	// Drain fast concurrently so it never blocks the sender either.
	var lastFast atomic.Value // string
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case ev, ok := <-fast.Events():
				if !ok {
					return
				}
				lastFast.Store(ev.Event.AgentID)
			case <-stop:
				return
			}
		}
	}()
	defer close(stop)

	// Publish more events than the slow subscriber's buffer can hold.
	const total = testBrokerBufSize + 10
	for i := 0; i < total; i++ {
		if err := store.PublishEvent(context.Background(), state.Event{
			Type:    "agent_registered",
			AgentID: "agent-slow-" + string(rune('a'+i)),
		}); err != nil {
			t.Fatalf("PublishEvent: %v", err)
		}
	}
	wantLast := "agent-slow-" + string(rune('a'+total-1))

	// Give the poll loop several intervals to drain the store and fan out;
	// the poll loop must not be stalled by the slow subscriber, so the fast
	// subscriber should catch up to the latest event within a few ticks.
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if v, ok := lastFast.Load().(string); ok && v == wantLast {
			break
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("fast subscriber never caught up to latest event %q (last seen: %v) — poll loop appears blocked", wantLast, lastFast.Load())
		}
	}

	if got := slow.Dropped(); got == 0 {
		t.Fatalf("expected slow subscriber to have dropped events, got 0")
	}

	// The poll loop must still be ticking (not deadlocked by the slow sub).
	before := store.Reads()
	settle(3 * testBrokerInterval)
	if store.Reads() <= before {
		t.Fatalf("poll loop appears stalled: reads before=%d after=%d", before, store.Reads())
	}
}

func TestBroker_UnsubscribeStopsDelivery(t *testing.T) {
	store := newCountingStore()
	b := httpbridge.NewBroker(store, testBrokerInterval, testBrokerBatch, brokerTestBufSizeLarge)
	defer b.Close()

	baseline := runtime.NumGoroutine()

	sub, _ := b.Subscribe()
	b.Unsubscribe(sub)

	// Sending after unsubscribe must never panic (channel closed, but the
	// poll loop no longer holds a reference — it will simply not see sub).
	if err := store.PublishEvent(context.Background(), state.Event{Type: "agent_registered", AgentID: "agent-post-unsub"}); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}
	settle(3 * testBrokerInterval)

	select {
	case ev, ok := <-sub.Events():
		if ok {
			t.Fatalf("expected closed channel after unsubscribe, got event %+v", ev)
		}
	default:
		t.Fatal("expected channel receive to be ready (closed) after unsubscribe, got nothing")
	}

	// Idempotent — must not panic on double-unsubscribe.
	b.Unsubscribe(sub)

	settle(3 * testBrokerInterval)
	runtime.GC()
	if after := runtime.NumGoroutine(); after > baseline+2 {
		t.Fatalf("possible goroutine leak: baseline=%d after=%d", baseline, after)
	}
}

func TestBroker_SinglePollPerInterval(t *testing.T) {
	store := newCountingStore()
	b := httpbridge.NewBroker(store, testBrokerInterval, testBrokerBatch, brokerTestBufSizeLarge)
	defer b.Close()

	const n = 5
	for i := 0; i < n; i++ {
		b.Subscribe()
	}

	settle(3 * testBrokerInterval)
	c1 := store.Reads()

	const k = 5
	settle(time.Duration(k) * testBrokerInterval)
	c2 := store.Reads()

	delta := c2 - c1
	if delta < int64(k)-1 || delta > int64(k)+2 {
		t.Fatalf("expected ~%d reads over %d intervals independent of %d subscribers, got %d (c1=%d c2=%d)", k, k, n, delta, c1, c2)
	}
}

func TestBroker_ShutdownClosesSubscribers(t *testing.T) {
	store := newCountingStore()
	b := httpbridge.NewBroker(store, testBrokerInterval, testBrokerBatch, brokerTestBufSizeLarge)

	sub1, _ := b.Subscribe()
	sub2, _ := b.Subscribe()

	settle(2 * testBrokerInterval)
	before := store.Reads()

	b.Close()

	for i, sub := range []*httpbridge.Subscription{sub1, sub2} {
		select {
		case _, ok := <-sub.Events():
			if ok {
				t.Fatalf("subscriber %d: expected closed channel after Close", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: channel not closed after Close", i)
		}
	}

	settle(5 * testBrokerInterval)
	after := store.Reads()
	if after != before {
		t.Fatalf("expected poll loop to stop after Close: before=%d after=%d", before, after)
	}
}

func TestBroker_SubscribeSinceReplaysThenLive(t *testing.T) {
	store := newCountingStore()

	// Publish three events before the broker exists so nothing is live yet.
	if err := store.PublishEvent(context.Background(), state.Event{Type: "agent_registered", AgentID: "e1"}); err != nil {
		t.Fatalf("PublishEvent e1: %v", err)
	}
	if err := store.PublishEvent(context.Background(), state.Event{Type: "agent_registered", AgentID: "e2"}); err != nil {
		t.Fatalf("PublishEvent e2: %v", err)
	}
	if err := store.PublishEvent(context.Background(), state.Event{Type: "agent_registered", AgentID: "e3"}); err != nil {
		t.Fatalf("PublishEvent e3: %v", err)
	}

	all, err := store.ReadEvents(context.Background(), "0", 10)
	if err != nil || len(all) != 3 {
		t.Fatalf("setup: expected 3 events, got %d err=%v", len(all), err)
	}
	e1ID := all[0].ID

	b := httpbridge.NewBroker(store, testBrokerInterval, testBrokerBatch, brokerTestBufSizeLarge)
	defer b.Close()

	// Let the broker's poll loop advance its cursor past all 3 events.
	settle(3 * testBrokerInterval)

	sub, cut := b.Subscribe()

	// Simulate handleSSE's one-time bounded backfill from since=e1ID up to cut.
	backfilled := replay(t, store, e1ID, cut)
	wantBackfill := []string{"e2", "e3"}
	assertAgentIDs(t, "backfill(since=e1)", backfilled, wantBackfill)

	// Now publish a 4th event live — it must be delivered exactly once via
	// the channel, with no duplicate of e2/e3 at the handoff boundary.
	if err := store.PublishEvent(context.Background(), state.Event{Type: "agent_registered", AgentID: "e4"}); err != nil {
		t.Fatalf("PublishEvent e4: %v", err)
	}
	select {
	case ev := <-sub.Events():
		if ev.Event.AgentID != "e4" {
			t.Fatalf("expected e4 live, got %q", ev.Event.AgentID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("never received e4 live")
	}
	b.Unsubscribe(sub)

	// A fresh subscribe with since="0" replays the full history, then live.
	sub2, cut2 := b.Subscribe()
	defer b.Unsubscribe(sub2)
	full := replay(t, store, "0", cut2)
	assertAgentIDs(t, "backfill(since=0)", full, []string{"e1", "e2", "e3", "e4"})
}

func TestBroker_ResumeAfterCursorAhead(t *testing.T) {
	store := newCountingStore()
	b := httpbridge.NewBroker(store, testBrokerInterval, testBrokerBatch, brokerTestBufSizeLarge)
	defer b.Close()

	// Broker starts fresh (cursor="0") — simulate a client resuming with a
	// `since` cursor from a previous server incarnation that the current
	// store doesn't recognise as a valid entry ID.
	sub, cut := b.Subscribe()
	if cut != "0" {
		t.Fatalf("expected fresh broker cut=0, got %q", cut)
	}

	staleSince := "mock-does-not-exist"
	backfilled := replay(t, store, staleSince, cut)
	if len(backfilled) != 0 {
		t.Fatalf("expected no backfill from an unrecognised cursor, got %d events", len(backfilled))
	}

	// Publish after resume — gating path (handler-level) would deliver this
	// since it is strictly after `since` was ever observed. At the broker
	// level we simply assert it is delivered live, undropped, exactly once.
	if err := store.PublishEvent(context.Background(), state.Event{Type: "agent_registered", AgentID: "post-resume"}); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}
	select {
	case ev := <-sub.Events():
		if ev.Event.AgentID != "post-resume" {
			t.Fatalf("expected post-resume, got %q", ev.Event.AgentID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("never received post-resume event")
	}

	// No duplicate should follow.
	select {
	case ev := <-sub.Events():
		t.Fatalf("unexpected duplicate delivery: %+v", ev)
	case <-time.After(150 * time.Millisecond):
	}
}

// TestBroker_ResumeAfterCursorAhead_GatingClearsOnMatch exercises the full
// handleSSE resume protocol end-to-end at the broker+store level: a client
// resumes with a `since` cursor from before a simulated server restart
// (broker recreated, cursor reset to "0", but the underlying store's data
// persists — as Redis would). The gating path must skip the replayed old
// event (already seen by the client) and deliver only what comes after it.
func TestBroker_ResumeAfterCursorAhead_GatingClearsOnMatch(t *testing.T) {
	store := newCountingStore()

	if err := store.PublishEvent(context.Background(), state.Event{Type: "agent_registered", AgentID: "pre-restart-1"}); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}
	if err := store.PublishEvent(context.Background(), state.Event{Type: "agent_registered", AgentID: "pre-restart-2"}); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}
	pre, err := store.ReadEvents(context.Background(), "0", 10)
	if err != nil || len(pre) != 2 {
		t.Fatalf("setup: expected 2 events, got %d err=%v", len(pre), err)
	}
	since := pre[1].ID // client's last-known cursor before "restart" — pre-restart-2

	// Simulate a server restart: a fresh Broker over the same (persisted) store.
	b := httpbridge.NewBroker(store, testBrokerInterval, testBrokerBatch, brokerTestBufSizeLarge)
	defer b.Close()

	sub, cut := b.Subscribe()
	if cut != "0" {
		t.Fatalf("expected fresh broker cut=0, got %q", cut)
	}

	// Per handleSSE's logic: since != cut and cut == "0", so backfill is
	// skipped and gating is armed on gateID = since.
	gating := true
	gateID := since

	// Publish a genuinely new post-restart event.
	if err := store.PublishEvent(context.Background(), state.Event{Type: "agent_registered", AgentID: "post-restart"}); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}

	var delivered []string
	deadline := time.After(2 * time.Second)
	for len(delivered) < 1 {
		select {
		case ev := <-sub.Events():
			if gating {
				if ev.Event.AgentID != "" && ev.ID == gateID {
					gating = false
				}
				continue
			}
			delivered = append(delivered, ev.Event.AgentID)
		case <-deadline:
			t.Fatalf("timed out waiting for post-restart delivery; gating=%v delivered=%v", gating, delivered)
		}
	}

	assertStrings(t, "post-restart gating", delivered, []string{"post-restart"})
}

// TestBroker_SubscribeAfterClose covers the shutdown race where a new SSE
// request calls Subscribe() concurrently with (or after) Broker.Close(). A
// subscription registered after Close() has already run its close loop
// would otherwise never be delivered to (pollLoop has exited) and never be
// closed (Close() only closes subscribers once) — leaving handleSSE's
// select blocked forever on sub.Events() instead of seeing ok==false and
// returning, as every other subscriber does on shutdown. Subscribe() must
// hand back an already-closed channel once the broker is closed.
func TestBroker_SubscribeAfterClose(t *testing.T) {
	store := newCountingStore()
	b := httpbridge.NewBroker(store, testBrokerInterval, testBrokerBatch, brokerTestBufSizeLarge)

	b.Close()

	sub, _ := b.Subscribe()
	select {
	case _, ok := <-sub.Events():
		if ok {
			t.Fatal("expected closed channel (ok=false), got a delivered event")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Subscribe() after Close() returned a channel that never closes — handleSSE would block forever")
	}
}

// TestBroker_SubscribeCloseRace hammers Subscribe() concurrently with
// Close() so `go test -race` can catch any data race on the closed/subs
// state, and confirms every subscription — win or lose the race — ends up
// with a channel that closes (rather than one that hangs forever).
func TestBroker_SubscribeCloseRace(t *testing.T) {
	store := newCountingStore()
	b := httpbridge.NewBroker(store, testBrokerInterval, testBrokerBatch, brokerTestBufSizeLarge)

	const n = 50
	subs := make([]*httpbridge.Subscription, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			sub, _ := b.Subscribe()
			subs[i] = sub
		}(i)
	}
	// Close concurrently with the Subscribe burst above.
	go b.Close()
	wg.Wait()

	// b.Close() may still be finishing (its own wg.Wait() on the poll
	// goroutine) when our wg above completes, but every Subscribe() call
	// has already returned — and per the fix, has already been placed
	// either fully before or fully after Close()'s critical section. Give
	// the (already-returned) Close() call a moment to finish either way.
	b.Close() // idempotent-in-effect: closing an already-closed broker's cancel is safe

	for i, sub := range subs {
		select {
		case _, ok := <-sub.Events():
			if ok {
				t.Fatalf("subscriber %d: expected closed channel eventually, got a live event", i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("subscriber %d: channel never closed after broker Close() — potential goroutine/connection leak", i)
		}
	}
}

func assertStrings(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: expected %v, got %v", label, want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: expected %v, got %v", label, want, got)
		}
	}
}

// replay performs the same bounded backfill loop handleSSE performs:
// starting at `since`, read forward until the store returns no more events
// or an event with ID == cut is seen (inclusive stop). Only ID equality is
// ever used — never ordering comparison, since store cursor IDs are opaque.
func replay(t *testing.T, store state.StateStore, since, cut string) []state.StreamEvent {
	t.Helper()
	var out []state.StreamEvent
	cursor := since
	for {
		evs, err := store.ReadEvents(context.Background(), cursor, testBrokerBatch)
		if err != nil {
			t.Fatalf("replay ReadEvents: %v", err)
		}
		if len(evs) == 0 {
			return out
		}
		for _, se := range evs {
			out = append(out, se)
			cursor = se.ID
			if se.ID == cut {
				return out
			}
		}
	}
}

func assertAgentIDs(t *testing.T, label string, got []state.StreamEvent, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: expected %d events %v, got %d: %+v", label, len(want), want, len(got), got)
	}
	for i, w := range want {
		if got[i].Event.AgentID != w {
			t.Fatalf("%s: event %d: expected %q, got %q", label, i, w, got[i].Event.AgentID)
		}
	}
}

const brokerTestBufSizeLarge = 256
