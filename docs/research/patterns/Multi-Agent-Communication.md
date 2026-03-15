---
title: Multi-Agent Communication & IPC
tags: [ai, agents, ipc, zeromq, grpc, nats, redis, communication]
type: reference
---

# Multi-Agent Communication & IPC

## Choosing the Right Mechanism

No single IPC mechanism wins across all scenarios. Match to your use case:

| Use Case | Recommended |
|---------|------------|
| Orchestrator → agent control | Unix domain sockets (~130µs) |
| Agent → agent structured calls | gRPC (type-safe, streaming, backpressure) |
| Task distribution to workers | ZeroMQ PUSH/PULL or NATS pull consumers |
| Broadcast agent events | ZeroMQ PUB/SUB or NATS core |
| Durable ordered event log | Redis Streams or NATS JetStream |
| High-bandwidth data (tensors, embeddings) | Shared memory + semaphore |

## Latency Reference

| Mechanism | P99 Latency | Notes |
|-----------|------------|-------|
| Shared memory | ~100ns | Complex sync required |
| Unix domain socket | ~130µs | Simple, local-only |
| POSIX message queue | ~12µs | Kernel-buffered, typed |
| ZeroMQ | 50-100µs | Flexible patterns |
| gRPC | 5-20ms | HTTP/2, structured |
| NATS core | 1-5ms | Simple pub/sub |
| NATS JetStream | 5-10ms | Persistent, durable |

## ZeroMQ Patterns

- **PUSH/PULL** — work distribution with fair-queue load balancing
- **PUB/SUB** — loose-coupled broadcasts (no delivery guarantees)
- **DEALER/ROUTER** — async multiplexed RPC
- **REQ/REP** — synchronous tool calls (simple but blocks)

## Critical: Backpressure

Without flow control, a fast orchestrator overwhelms slow agents. Use:
- gRPC window-based flow control (built-in)
- NATS pull consumers (agent pulls when ready)
- Never use fire-and-forget PUB/SUB for critical work items

## Message Ordering

- **Global ordering** (all agents see same sequence) — expensive, usually unnecessary
- **Per-stream ordering** — cheap, sufficient for most cases
- **Causal ordering** — medium cost, use when agent B depends on agent A's output

## Serialization

- **Protobuf**: 6.5× faster than JSON, 2.6× smaller — use for hot paths
- **JSON**: human-readable, good for tooling/debugging
- Rule: Protobuf for agent-to-agent, JSON for config and logging

## Recommended Local Stack

```
Orchestrator
├── gRPC ←→ Agent A (bidirectional streaming)
├── gRPC ←→ Agent B
└── Redis Streams (durable task log + replay)
```

---

*Authored by: Clault KiperS 4.6*
