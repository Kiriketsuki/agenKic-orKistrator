---
title: IPC for Multi-Agent Systems
tags: [ai, agents, ipc, zeromq, grpc, nats, redis, sockets]
date: 2026-03-14
type: evergreen
---

# IPC for Multi-Agent Systems

No single IPC mechanism is best for all agent communication needs. The right choice depends on whether you need control signalling, work distribution, data streaming, or bulk transfer.

## Latency Reference

| Mechanism | P99 Latency | Notes |
|-----------|------------|-------|
| Shared memory (lock-free) | ~100ns | Complex synchronisation required |
| Unix domain socket | ~130µs | Best for local control channels |
| POSIX message queue | ~12µs | Kernel-buffered, typed, ordered |
| ZeroMQ | ~50-100µs | Flexible patterns, no persistence |
| NATS core | ~1-5ms | Simple, no disk |
| NATS JetStream | ~5-10ms | Persistent, ordered |
| gRPC | ~5-20ms | Type-safe, streaming, backpressure |

## Recommended Architecture (Local Multi-Agent)

```
Control channel:    Unix domain socket (per-agent, synchronous calls)
Work distribution:  NATS JetStream or Redis Streams (durable, ordered)
Agent-to-agent RPC: gRPC (type-safe, backpressure via HTTP/2 flow control)
Bulk data transfer: Shared memory + semaphore (embeddings, large files)
```

## ZeroMQ Patterns

| Pattern | Use For |
|---------|---------|
| PUSH/PULL | Task distribution with fair-queue load balancing |
| PUB/SUB | Loose-coupled event broadcasting |
| DEALER/ROUTER | Async multiplexed RPC (best for orchestrator ↔ many workers) |
| REQ/REP | Simple synchronous request-response |

ZeroMQ is fast and flexible but has **no persistence or acks** — pair with a durable store if message loss is unacceptable.

## Critical Principles

### Backpressure is Non-Negotiable
Without explicit flow control, a fast orchestrator overwhelms slow agents. gRPC provides window-based flow control automatically. For NATS, use pull consumers (not push).

### Exactly-Once is Impossible — Design for Idempotency Instead
Assign unique message IDs (timestamp + producer ID), deduplicate on receiver, design operations to be safely replayable. This solves 99% of ordering/reliability needs without consensus complexity.

### Match Ordering Granularity to Need
- **Global ordering** (all agents see same sequence): expensive, rarely needed
- **Per-stream ordering** (preserve order within one channel): cheap, usually sufficient
- **Causal ordering** (respect dependencies): medium cost, use when agents have data dependencies

## For High-Bandwidth Agent Data (Embeddings, Files)

Use **content-addressed storage** instead of IPC:
1. Agent A writes output → `hash(content)` → stores at `artifacts/{hash}`
2. Agent A sends hash over IPC (tiny message)
3. Agent B reads from `artifacts/{hash}` directly

Avoids serialising large payloads over message queues entirely.

## See Also

- [[Agent-State-Management]] — Redis as shared state store
- [[Agentic-Orchestration-Patterns]] — how work gets dispatched to agents

---

*Authored by: Clault KiperS 4.6*
