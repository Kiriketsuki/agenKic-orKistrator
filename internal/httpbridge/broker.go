package httpbridge

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kiriketsuki/agenKic-orKistrator/internal/state"
)

// brokerDefaultBufSize is the default per-subscriber channel buffer depth.
const brokerDefaultBufSize = 256

// Subscription is one broker subscriber's live event feed. Obtained from
// Broker.Subscribe and released via Broker.Unsubscribe.
//
// Slow-consumer policy: the poll loop fans out with a non-blocking
// (tail-drop) send. If a subscriber's buffered channel is full, the event is
// dropped for that subscriber rather than blocking the shared poll loop —
// one slow HTTP client (or a stalled reader) must never stall delivery to
// every other subscriber. Every fanned-out event carries a cursor, so a
// client that notices a gap can reconnect with ?since=<lastCursor> and
// backfill the dropped range from the store — drops are recoverable by
// design. Dropped is exposed for observability.
type Subscription struct {
	ch      chan state.StreamEvent
	dropped int64
}

// Events returns the channel new events are delivered on. The channel is
// closed when the subscription is unsubscribed or the broker is closed.
func (s *Subscription) Events() <-chan state.StreamEvent { return s.ch }

// Dropped returns the number of events tail-dropped for this subscriber
// because its buffer was full when the poll loop tried to deliver.
func (s *Subscription) Dropped() int64 { return atomic.LoadInt64(&s.dropped) }

// Broker owns a single poll goroutine that reads new StreamEvents from the
// underlying state.StateStore once per poll interval and fans each one out
// to every registered Subscription.
//
// Correctness core — the atomic cut handoff: Subscribe snapshots the
// broker's current cursor ("cut") and inserts the new subscriber into the
// fan-out set in the SAME critical section (b.mu). The poll loop advances
// the cursor and fans out to the subscriber set in ITS OWN critical section.
// Because these two operations are mutually exclusive under b.mu, a
// subscriber that joins when cursor == cut is guaranteed to receive, via its
// channel, exactly the events fanned out strictly AFTER cut — in order, with
// no gap and no duplicate at the boundary. Callers that need to replay
// history from an earlier client-supplied cursor do a one-time bounded
// backfill from the store themselves (see handleSSE) using cut as the stop
// point — the broker itself never needs to compare two store-assigned
// cursor IDs with '<': store IDs are opaque strings (see state.StateStore),
// and this design only ever tests cursor IDs for EQUALITY.
//
// store.ReadEvents runs OUTSIDE b.mu so Redis latency never blocks
// Subscribe/Unsubscribe; only the cursor-advance + fan-out step is inside
// the lock.
type Broker struct {
	store    state.StateStore
	interval time.Duration
	batch    int64
	bufSize  int

	mu     sync.Mutex
	subs   map[*Subscription]struct{}
	cursor string
	closed bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewBroker constructs a Broker and immediately launches its poll goroutine.
// It must be launched here (not in a separate Start method) because
// httptest-based tests exercise Bridge via ServeHTTP only and never call
// Bridge.Start.
func NewBroker(store state.StateStore, interval time.Duration, batch int64, bufSize int) *Broker {
	if bufSize <= 0 {
		bufSize = brokerDefaultBufSize
	}
	ctx, cancel := context.WithCancel(context.Background())
	b := &Broker{
		store:    store,
		interval: interval,
		batch:    batch,
		bufSize:  bufSize,
		subs:     make(map[*Subscription]struct{}),
		cursor:   "0",
		ctx:      ctx,
		cancel:   cancel,
	}
	b.wg.Add(1)
	go b.pollLoop()
	return b
}

// pollLoop is the broker's single reader. Exactly one store.ReadEvents call
// happens per tick, regardless of how many subscribers are registered.
func (b *Broker) pollLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			// select does not prioritize cases — Done() and ticker.C can
			// both be ready if Close() races the ticker firing. Re-check
			// here so a shutdown is never missed in favor of one more tick.
			select {
			case <-b.ctx.Done():
				return
			default:
			}

			cursor := b.snapshotCursor()

			// Outside the lock: never let store latency block Subscribe/Unsubscribe.
			evs, err := b.store.ReadEvents(b.ctx, cursor, b.batch)
			if err != nil {
				log.Printf("httpbridge: broker ReadEvents: %v", err)
				continue
			}
			if len(evs) == 0 {
				continue
			}

			b.mu.Lock()
			for _, se := range evs {
				b.cursor = se.ID
				for sub := range b.subs {
					select {
					case sub.ch <- se:
					default:
						atomic.AddInt64(&sub.dropped, 1)
					}
				}
			}
			b.mu.Unlock()
		}
	}
}

// snapshotCursor reads the broker's shared cursor under a short lock. The
// poll loop is the only writer of b.cursor, but Subscribe reads it too, so
// both paths go through the same mutex.
func (b *Broker) snapshotCursor() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cursor
}

// Subscribe registers a new Subscription and returns it along with "cut" —
// a snapshot of the broker's cursor taken atomically with the insertion.
// See the Broker doc comment for why this makes the handoff race-free.
//
// If the broker has already been Close()d, Subscribe returns a Subscription
// whose channel is already closed (rather than inserting into subs) — the
// same critical section Close() uses to close every subscriber is used here
// to check the closed flag, so the two paths can never race: either
// Subscribe fully precedes Close() (and gets closed normally by Close()'s
// loop), or Close() fully precedes Subscribe (and Subscribe hands back an
// already-closed channel here). Without this, a Subscribe that lands after
// Close() has already run its close loop would register a subscriber that
// pollLoop (already exited) never delivers to and Close() (already run)
// never closes, leaving handleSSE blocked forever on a live channel instead
// of seeing ok==false and returning.
func (b *Broker) Subscribe() (*Subscription, string) {
	sub := &Subscription{ch: make(chan state.StreamEvent, b.bufSize)}
	b.mu.Lock()
	if b.closed {
		cut := b.cursor
		b.mu.Unlock()
		close(sub.ch)
		return sub, cut
	}
	cut := b.cursor
	b.subs[sub] = struct{}{}
	b.mu.Unlock()
	return sub, cut
}

// Unsubscribe removes sub from the fan-out set and closes its channel.
// Idempotent — calling it more than once (or after Close) is a no-op on the
// second call. Removal-then-close happens in the same critical section the
// poll loop's fan-out uses, so the poll loop can never hold a reference to
// sub.ch while it is being (or has been) closed — no send-on-closed-channel
// panic is possible.
func (b *Broker) Unsubscribe(sub *Subscription) {
	b.mu.Lock()
	if _, ok := b.subs[sub]; ok {
		delete(b.subs, sub)
		close(sub.ch)
	}
	b.mu.Unlock()
}

// Close stops the poll loop and closes every remaining subscriber's
// channel, then waits for the poll goroutine to exit.
func (b *Broker) Close() {
	b.cancel()
	b.mu.Lock()
	b.closed = true
	for sub := range b.subs {
		delete(b.subs, sub)
		close(sub.ch)
	}
	b.mu.Unlock()
	b.wg.Wait()
}
