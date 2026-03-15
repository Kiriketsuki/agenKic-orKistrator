---
title: Agent State Management & Shared Context
tags: [research, orchestration, state, redis, crdt, event-sourcing, blackboard]
date: 2026-03-14
type: research
---

# Agent State Management & Shared Context

## The Core Insight: Context Window = RAM

An LLM's context window is **finite working memory** (RAM), not storage. Production systems must externalize state via:
- **Scratchpads** — in-task working notes written to external store
- **Artifacts** — files/embeddings/structured data passed between agents
- **Checkpoints** — snapshots enabling crash recovery

Token compression strategies: observation masking (keep actions, drop old outputs), summarization, selective context injection.

---

## The Memory Stack

```
Working Memory  → scratchpad (in-task, context window)
Session Memory  → Redis Hashes (current task state, <1ms reads)
Long-term Store → Versioned artifacts (cross-session, content-addressed)
Audit Log       → Event stream (replay to any prior state)
```

---

## Classic Patterns

### Blackboard Architecture
All agents read/write to a shared "blackboard" (mutable shared data structure). Simple and effective for small teams; conflicts require explicit locking at scale.

### Tuple Spaces (Linda Model)
Associative memory with three operations:
- `out(tuple)` — write a tuple
- `in(tuple)` — destructively read (blocks if no match)
- `rd(tuple)` — non-destructive read

Elegant for producer/consumer coordination. Modern revival in some distributed systems.

### Event Sourcing
Agents emit **immutable events**; state is derived by replaying the log. Benefits:
- Full audit trail of all agent decisions
- Crash recovery: replay from checkpoint
- Any prior state is reconstructable

**Preferred over mutable blackboard** for production systems.

---

## Concurrency: CRDTs & Vector Clocks

### CRDTs (Conflict-free Replicated Data Types)
Allow concurrent writes from multiple agents without coordination:
- **G-Counter**: monotonically increasing count per agent
- **OR-Set**: concurrent add/remove without conflicts
- **LWW-Register**: last-write-wins (use when ordering matters less)

### Vector Clocks
Track causal ordering of agent events without a global clock:
```
Agent A: [A:1, B:0, C:0]
Agent B: [A:1, B:1, C:0]   (happened after receiving A's message)
```
Reveals which agent decisions were concurrent vs. causally ordered. Essential for detecting when agents made independent vs. dependent choices.

---

## Redis as Agent Coordination Store

### Recommended Redis Hybrid
| Data Structure | Purpose |
|---------------|---------|
| **Streams** (XADD/XREAD) | Task distribution, event log, ordered agent output |
| **Hashes** | Agent working state snapshots (<1ms reads) |
| **Pub/Sub** | Real-time signals (volatile — no persistence) |
| **Sorted Sets** | Priority queues for task scheduling |

**Warning**: Do not use Pub/Sub alone — no persistence. Use Streams for durable coordination.

---

## Artifact Passing

For large payloads (files, embeddings, model outputs), use **content-addressed storage (CAS)**:

```
Agent A: process data → hash(result) → store at artifacts/{hash}
Agent B: receive hash → read from artifacts/{hash}
```

Benefits: deduplication, zero-copy sharing, immutable (no accidental mutation), verifiable integrity.

---

## Checkpointing

Long-running tasks fail 20-30% of the time. Checkpointing cost vs savings:
- Checkpoint overhead: 5-40% of runtime
- Recovery savings: 60%+ of reprocessing avoided

**Synchronous checkpointing**: full durability, slightly slower
**Asynchronous checkpointing**: higher throughput, small window of potential data loss

Choose based on task criticality.

---

## The Scratchpad Pattern

ReAct agents write intermediate reasoning to an external scratchpad before committing to output:

```
Think → Write to scratchpad → Act → Observe → Think again → ...
```

Scratchpad content is readable by the orchestrator for monitoring. Agents with external scratchpads outperform single-pass reasoning on complex tasks.

**Observation masking optimization**: After many steps, keep only the actions in context (drop old observations). Reduces context bloat by ~40% with minimal accuracy loss.

---

## References

- Blackboard architecture (classical AI — Hayes-Roth 1985)
- Linda/tuple spaces — Gelernter 1985
- Martin Fowler: Event Sourcing pattern
- Redis documentation: Streams, CRDT use cases
- Anthropic "Building Effective Agents" — context management section

---

*Authored by: Clault KiperS 4.6*
