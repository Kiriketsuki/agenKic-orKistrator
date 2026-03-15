---
title: Agent State Management
tags: [ai, agents, state, redis, crdt, event-sourcing, blackboard, memory]
type: reference
---

# Agent State Management

## The Core Mental Model

```
Context window  = RAM (finite, volatile working memory)
Redis / DB      = Hard drive (persistent, shared state)
Artifacts       = Files (large data passed between agents)
Event log       = Git history (full audit, replayable)
```

Long-running agents **must** externalize state — context windows fill up and processes crash.

---

## The Memory Stack

| Layer | Store | Lifespan |
|-------|-------|---------|
| Working (scratchpad) | Context window + external text file | Current task |
| Session | Redis Hashes | Current session |
| Cross-session | Versioned artifacts (content-addressed) | Persistent |
| Audit | Event stream (Redis Streams / append-only log) | Forever |

---

## Classic Patterns

### Blackboard
All agents read/write a shared mutable structure. Simple for small teams; requires explicit locking at scale.

### Event Sourcing (preferred)
Agents emit **immutable events**; state derived by replay. Benefits:
- Full audit trail of all decisions
- Crash recovery: replay from last checkpoint
- Any prior state reconstructable

### Tuple Spaces (Linda model)
- `out(tuple)` — write
- `in(tuple)` — destructive read (blocks until match)
- `rd(tuple)` — non-destructive read
Elegant for producer/consumer coordination.

---

## Concurrency

**CRDTs** (Conflict-free Replicated Data Types) for concurrent agent writes without coordination:
- G-Counter: monotonically increasing count
- OR-Set: concurrent add/remove
- LWW-Register: last-write-wins

**Vector clocks** for causal ordering — reveals which agent decisions were concurrent vs. sequential.

---

## Redis Hybrid Pattern

```
Redis Streams  → task distribution, event log, ordered agent output
Redis Hashes   → agent working state snapshots (<1ms reads)
Redis Pub/Sub  → real-time signals only (volatile — no persistence)
Redis Sorted Sets → priority task queues
```

**Don't** rely on Pub/Sub alone — no persistence. Use Streams for anything that must survive a restart.

---

## Artifact Passing (Content-Addressed Storage)

```
Agent A: result = process(data)
         hash = sha256(result)
         store(f"artifacts/{hash}", result)
         send_to_agent_b(hash)

Agent B: result = load(f"artifacts/{hash}")
```

Benefits: deduplication, zero-copy sharing, immutable, verifiable.

---

## Checkpointing

Long-running tasks fail 20-30% of the time. Checkpointing saves 60%+ of reprocessing cost.
- **Synchronous**: full durability, slightly slower
- **Asynchronous**: higher throughput, small data-loss window

Rule: checkpoint at every meaningful task boundary, not just on crash.

---

## Context Compression Strategies

When context windows fill up:
1. **Observation masking** — keep actions, drop old observations (~40% reduction)
2. **Summarization** — compress old turns into a summary paragraph
3. **Selective injection** — only load context relevant to the current sub-task

---

*Authored by: Clault KiperS 4.6*
