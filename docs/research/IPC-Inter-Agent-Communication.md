# IPC and Inter-Agent Communication Primitives

**Research Date**: 2026-03-14
**Scope**: Local multi-agent orchestration systems
**Focus**: Latency, throughput, complexity, and agent-specific tradeoffs

---

## Executive Summary

For local multi-agent orchestration, communication primitives span a spectrum from ultra-low-latency kernel mechanisms (shared memory, Unix sockets) to feature-rich frameworks (gRPC, NATS, ZeroMQ). The choice depends critically on:

- **Latency sensitivity**: Shared memory (~100ns) vs. sockets (~µs) vs. frameworks (~ms)
- **Message ordering guarantees**: POSIX queues vs. idempotent designs
- **Backpressure requirements**: How to prevent fast producers from overwhelming slow agents
- **Persistence needs**: Whether agents require message replay and durability
- **Complexity budget**: Runtime overhead and operational burden

---

## 1. Unix Domain Sockets (UDS)

### Performance Characteristics

[UDS consistently deliver 30–50% better latency than TCP loopback](https://www.yanxurui.cc/posts/server/2023-11-28-benchmark-tcp-uds-namedpipe/) for local inter-process communication:

- **Latency**: ~130µs (UDS) vs. ~334µs (TCP loopback) — **61% reduction**
- **Throughput**: ~40% improvement over TCP loopback in benchmarks
- **Why faster**: Bypasses entire IP stack, no checksums, no packet encapsulation/decapsulation

### Architecture for Agent Orchestration

**Per-agent control channel**: Each agent gets a dedicated UDS for:
- Task distribution from orchestrator to agent
- Bidirectional status/heartbeat signaling
- Non-streaming tool calls (synchronous request/response)

**Modes**:
- **Stream mode**: Persistent bidirectional connection for continuous signaling
- **Datagram mode**: Connectionless message passing (less common)

### Limitations

- **Local-only**: Cannot cross machine boundaries
- **Scaling degradation**: Performance benefits diminish with heavy pipelining (deep queues)
- **No ordering guarantees**: Order must be enforced at application level

### Use Case for Agents

**Excellent for**: Orchestrator-to-agent control channels, synchronous tool calls, local multi-agent coordination
**Poor for**: Streaming continuous agent output, persistence, complex message ordering

---

## 2. Named Pipes (FIFOs)

### Characteristics

[Named pipes are unidirectional, kernel-buffered IPC channels](https://man7.org/linux/man-pages/man7/fifo.7.html) with strong FIFO semantics:

- **Direction**: Unidirectional (one reader, one or more writers)
- **Blocking behavior**: By default, both read and write operations block until both ends are open
- **Capacity**: Limited kernel buffer; writes block if pipe is full
- **Guarantees**: FIFO ordering is enforced; messages not dropped (just blocking)

### Non-Blocking Mode

A process can open a FIFO in `O_NONBLOCK` mode:
- Read-only opens succeed even if no writer exists yet
- Write-only opens fail with `ENXIO` unless reader already open
- Useful for avoiding deadlocks in orchestrator code

### Use Case for Agents

**Excellent for**: Streaming agent stdout/stderr, unidirectional output piping
**Poor for**: Bidirectional communication, request/response patterns, low-latency critical paths

---

## 3. Shared Memory (POSIX shm_open + mmap)

### Performance Characteristics

[Shared memory remains the gold standard for sub-microsecond latency inter-process communication](https://www.softprayog.in/programming/interprocess-communication-using-posix-shared-memory-in-linux):

- **Latency**: Sub-microsecond (~100ns) — effectively zero-copy memory access
- **Throughput**: Highest possible for local systems (bounded only by CPU cache bandwidth)
- **Mechanism**: `shm_open()` creates a memory-mapped file; `mmap()` attaches it to process address space

### Synchronization Overhead

[Processes must coordinate using POSIX semaphores or similar](https://www.softprayog.in/programming/interprocess-communication-using-posix-shared-memory-in-linux):
- `sem_open()` for named semaphores (cross-process)
- Mutex operations add synchronization cost (~few µs per lock/unlock)
- Signal delivery needed for inter-process notifications

### Use Case for Agents

**Excellent for**: High-bandwidth agent-to-agent data transfer (embeddings, tensors, large payloads), low-latency shared state
**Poor for**: Simple control messages, transient agents (creation/cleanup overhead), ordering guarantees

---

## 4. POSIX Message Queues (mq_open)

### Architecture

[POSIX message queues provide kernel-buffered, priority-ordered messaging](https://man7.org/linux/man-pages/man7/mq_overview.7.html):

- **Creation**: `mq_open(name, oflag, mode, attr)` where `attr` specifies queue depth and message size
- **Message ordering**: **Priority-based FIFO** — messages always delivered highest-priority first
- **Priority range**: 0 (lowest) to `sysconf(_SC_MQ_PRIO_MAX) - 1` (highest); POSIX requires at least 0-31
- **Within-priority FIFO**: Messages at same priority delivered in arrival order
- **Blocking semantics**: Read blocks if queue empty; write blocks if queue full

### Advantages Over Pipes

- **Typed messages**: Each message has a priority and payload
- **Kernel buffering**: Unlike pipes, queue capacity is explicit and configurable
- **Priority scheduling**: Critical tasks can jump the queue

### Limitations

- **Message size limits**: Typically 4096 bytes default (configurable at creation)
- **No exactly-once semantics**: Duplicate detection must be application-level
- **Learning curve**: More complex API than pipes/sockets

### Use Case for Agents

**Excellent for**: Priority-based task distribution (urgent vs. routine tasks), kernel-buffered work queues
**Poor for**: Large payloads, streaming data, exactly-once delivery requirements

---

## 5. ZeroMQ

### Core Patterns

[ZeroMQ is a message-oriented middleware with four built-in patterns](https://zguide.zeromq.org/docs/chapter2/):

#### 5.1 REQ/REP (Request-Reply)

- **Synchronous request/response**: Client sends request, blocks for reply
- **Round-robin load balancing**: One client can connect to multiple REP servers
- **Strict ordering**: Enforces alternating send/receive sequences

**Agent use**: Synchronous tool calls (agent calls orchestrator for resources)

#### 5.2 PUSH/PULL (Pipeline)

- **Work distribution**: PUSH node distributes tasks; PULL workers consume
- **Load balancing**: Fair round-robin distribution across workers
- **Reliability**: Won't discard messages unless node unexpectedly disconnects

**Agent use**: Task distribution to workers, result collection

#### 5.3 PUB/SUB (Publish-Subscribe)

- **Broadcast messaging**: One publisher sends to many subscribers
- **Subscriber filtering**: Subscribers choose topics by prefix matching
- **Fire-and-forget**: No acknowledgment, no buffering for late subscribers

**Agent use**: Broadcasting events (agent started, completed, errored)

#### 5.4 DEALER/ROUTER (Asynchronous Multiplexing)

- **Async replacement for REQ**: Non-blocking, async-friendly
- **Connection identification**: ROUTER creates identities for peers, passes them in messages
- **Complex topologies**: Enables sophisticated routing patterns

**Agent use**: Asynchronous multiplexed communication, complex agent graphs

### Performance

- Typical latency: 50-100µs (higher than UDS, lower than TCP)
- Throughput: Millions of messages/sec in benchmarks
- Complexity: Rich pattern support comes with API overhead

### Limitations

- **No message ordering across topics**: PUB/SUB doesn't guarantee order
- **No persistence**: Messages in-flight only; late subscribers miss messages
- **No flow control by default**: Slow subscribers can be overrun (sockets have small buffers)

---

## 6. NATS (Cloud Native Messaging)

### Architecture

[NATS is a lightweight pub/sub system with optional JetStream persistence](https://docs.nats.io/nats-concepts/jetstream):

- **Core NATS**: Fire-and-forget pub/sub (like ZeroMQ PUB/SUB)
- **JetStream**: Optional persistence layer for message replay and durability
- **Subject hierarchy**: Topics organized as `domain.service.event` (e.g., `agents.worker-1.completed`)

### JetStream Persistence

- **Streams**: Durable message stores keyed by subject
- **Consumers**: Pointers into streams; multiple consumers can independently replay messages
- **Delivery guarantees**:
  - At-least-once with server-side deduplication windows
  - Exactly-once semantics via idempotent consumers
- **Consumer groups**: Multiple agents can share work from a stream

### Performance

- Latency: Single-digit milliseconds (higher than UDS, lower than gRPC)
- Throughput: Designed for high-volume streaming (100k+ msg/sec)
- Backpressure: Built-in flow control; consumers signal capacity

### Use Case for Agents

**Excellent for**: Durable event streaming (agent lifecycle events, audit logs), persistent work queues with replay
**Poor for**: Ultra-low-latency control channels, simple synchronous calls

---

## 7. gRPC

### Architecture

[gRPC is a protocol-buffers-based RPC framework using HTTP/2](https://grpc.io/docs/what-is-grpc/core-concepts/):

- **Serialization**: Protocol Buffers — 3-10x faster than JSON, smaller payloads
- **Transport**: HTTP/2 with multiplexing (multiple streams over one TCP connection)
- **Code generation**: Compile-time type safety from `.proto` definitions

### Streaming Patterns

1. **Unary**: Client sends one request, server sends one response (traditional RPC)
2. **Server streaming**: Server returns stream of responses; client waits for completion
3. **Client streaming**: Client sends stream of requests; server returns one response
4. **Bidirectional streaming**: Both sides stream messages independently

### Performance

- Latency: 5-20ms typical (TCP overhead + protobuf serialization)
- Throughput: Multiplexing enables concurrent streams; significantly better than multiple TCP connections
- Serialization: 3-10x faster than JSON encoding/decoding

### Advantages

- **Type safety**: Code generation from `.proto` files prevents versioning issues
- **Language agnostic**: Go, Python, Node.js, Java, etc. all supported
- **Streaming**: Bidirectional streaming excellent for continuous agent output (logs, status updates)
- **Ecosystem**: Widely adopted, mature tooling

### Limitations

- **HTTP/2 overhead**: Not suitable for microsecond-level latencies
- **Connection complexity**: Each agent needs a gRPC server or async client stubs
- **Learning curve**: Protocol Buffers and async streaming patterns non-trivial

### Use Case for Agents

**Excellent for**: Agent APIs with streaming output (logs, continuous metrics), polyglot agent deployments, complex tool schemas
**Poor for**: Ultra-low-latency control, simple synchronous work distribution

---

## 8. Backpressure and Flow Control

### The Problem

[Backpressure occurs when a producer sends messages faster than a consumer can process them](https://medium.com/@jayphelps/backpressure-explained-the-flow-of-data-through-software-2350b3e77ce7):

- Queue fills to capacity
- Producer options: block (stall), drop (lose data), or buffer in-memory (memory leak)
- Slow agent blocks fast producer; no feedback to prevent overload

### Solutions

[Three approaches exist](https://codeopinion.com/avoiding-a-queue-backlog-disaster-with-backpressure-flow-control/):

1. **Producer control** (optimal): Consumer signals capacity; producer slows down
   - Implementation: Explicit flow control (gRPC window updates, NATS pull consumers)
   - Benefit: No data loss, no stalling, no memory leak

2. **Buffering**: Queue spikes in-memory; producer keeps sending
   - Benefit: Smooth out transient bursts
   - Risk: Unbounded memory growth if producer outpaces consumer

3. **Sampling/dropping**: Producer or queue samples only a percentage of messages
   - Use case: Metrics, logs where some data loss acceptable
   - Risk: Loss of precision, hard to debug intermittent issues

### Framework Support

| Framework | Backpressure Support |
|-----------|----------------------|
| UDS       | TCP buffer limits; application must handle |
| Named pipes | Automatic blocking when full |
| Shared memory | Application-level (semaphores) |
| POSIX queues | Automatic blocking; queue capacity explicit |
| ZeroMQ | Limited; small socket buffers |
| NATS | Excellent; pull consumers, flow control |
| gRPC | Excellent; window-based flow control |

### Recommendation for Agents

**Implement explicit producer control**:
- Consumer advertises available capacity (e.g., "accept 10 tasks")
- Producer respects advertised capacity before sending
- Prevents cascading failures and queue bloat

---

## 9. Message Ordering and Exactly-Once Delivery

### Delivery Guarantees (Impossible Tradeoff)

[True exactly-once delivery is impossible in distributed systems](https://bravenewgeek.com/you-cannot-have-exactly-once-delivery/) due to FLP impossibility and the Two Generals Problem:

- **At-most-once**: Message delivered 0 or 1 times (may be lost)
- **At-least-once**: Message delivered 1 or more times (may be duplicated)
- **Exactly-once**: Theoretically impossible; practically simulated via idempotency

### Message Ordering Techniques

[Solutions must impose structure on asynchronous systems](https://blog.bulloak.io/post/20200917-the-impossibility-of-exactly-once/):

1. **Causal ordering**: If event A causes event B, A is delivered before B
   - Implementation: Vector clocks, Lamport timestamps
   - Cost: Extra metadata, ordering delays

2. **Total ordering**: All agents see messages in identical order
   - Implementation: Atomic broadcast, consensus (Raft, Paxos)
   - Cost: Throughput penalty, coordination overhead
   - Use case: Agent state synchronization, global consistency

3. **Sequencing**: Messages tagged with sequence numbers; receiver detects gaps
   - Implementation: Application-level; detect and re-request missing messages
   - Cost: Moderate; re-transmission overhead

### Achieving "Exactly-Once" Semantics

[Idempotency is the practical solution](https://www.confluent.io/blog/exactly-once-semantics-are-possible-heres-how-apache-kafka-does-it/):

- **Idempotent operation**: Applying it multiple times = applying it once
- **Example**: "Agent 1 claims task 42" is idempotent (same result if called twice)
- **Implementation**:
  - Give each message a unique ID (timestamp + producer ID)
  - Track which message IDs have been processed
  - Duplicate deliveries are automatically ignored

### When Ordering Matters for Agents

- **Task distribution**: No strict ordering needed (independent tasks)
- **Agent state updates**: Causal ordering critical (e.g., "start agent before assigning work")
- **Audit logs**: Total ordering desired but not critical
- **Tool calls**: Idempotency via request deduplication sufficient

---

## 10. Comparison Table: Latency, Throughput, Complexity

| Mechanism | Latency | Throughput | Ordering | Persistence | Complexity | Best For |
|-----------|---------|-----------|----------|-------------|-----------|----------|
| **Shared memory** | ~100ns | Highest | None | No | High | Tensor/embedding sharing |
| **Unix sockets** | ~130µs | High | None | No | Low | Control channels |
| **Named pipes** | ~200µs | High | FIFO | No | Very low | Output streaming |
| **POSIX queues** | ~500ns (worst case) | Medium | Priority FIFO | No | Medium | Priority task distribution |
| **ZeroMQ** | ~50-100µs | Very high | Topic-dependent | No | Medium | Event broadcasts |
| **NATS (core)** | ~1-5ms | Very high | Per-subject | No | Low | Fast pub/sub |
| **NATS JetStream** | ~5-10ms | High | Per-subject | Yes | Medium | Durable event streaming |
| **gRPC** | ~5-20ms | High | Per-stream | No | High | Agent APIs, streaming |

---

## Key Findings and Recommendations

### 1. **Hybrid Architecture for Agent Orchestration**

No single mechanism fits all use cases. Recommend:

- **Control channels** (orchestrator ↔ agent): Unix domain sockets or POSIX queues
- **Task distribution**: POSIX queues (priority) or NATS (durability)
- **Output streaming** (agent logs): Named pipes or gRPC streams
- **High-bandwidth data sharing**: Shared memory for large tensors; gRPC for structured data
- **Persistence**: NATS JetStream for audit trails, event replay

### 2. **Backpressure is Non-Negotiable**

Fast orchestrators will overwhelm slow agents without explicit flow control:

- Use gRPC window-based flow control or NATS pull consumers
- Avoid fire-and-forget patterns (PUB/SUB) for critical work
- Implement producer throttling (e.g., "accept 5 tasks, wait for completion before more")

### 3. **Exactly-Once is Achievable via Idempotency**

Don't pursue true exactly-once delivery (impossible). Instead:

- Assign each message a unique ID (timestamp + producer ID)
- Deduplicate on the receiver side (track seen IDs)
- Design operations to be idempotent (e.g., "agent 1 claims task 42" can be retried safely)

### 4. **Message Ordering Depends on Scope**

- **Global ordering** (all agents see same sequence): Expensive, use only for state sync
- **Per-stream ordering** (preserve order within one subject/channel): Cheap, use by default
- **Causal ordering** (respect dependencies): Medium cost, use for critical workflows

### 5. **Shared Memory Shines for Data Transfer**

For large payloads (embeddings, model weights, tensors):

- Shared memory + semaphore coordination = sub-microsecond handoff
- Beats serialization/deserialization overhead of sockets/gRPC
- Synchronization cost (~few µs) amortized over large transfers

### 6. **NATS as Orchestration Backbone**

If you need durability, ordering, and simplicity:

- NATS core for real-time event broadcasts (agent started, completed, failed)
- JetStream consumers for reliable work distribution and replay
- Subject hierarchy (`orchestrator.task.assigned`, `agent.task.completed`) provides routing structure
- Built-in backpressure via pull consumers

### 7. **gRPC for Complex Agent APIs**

If agents expose varied tool schemas:

- Protocol Buffers enforce versioning at the boundary
- Bidirectional streaming enables continuous log/metric output
- Multiplexing reduces connection overhead
- Trade-off: ~5-20ms latency vs. untyped socket protocols

---

## Decision Tree for Choosing an IPC Mechanism

```
├─ Is this an orchestrator-to-agent control channel?
│  ├─ Yes, synchronous (tool calls)?
│  │  └─ Use Unix domain sockets (fast, simple)
│  └─ Yes, asynchronous work distribution?
│     └─ Use POSIX message queues (priority scheduling) or NATS (durability)
│
├─ Is this high-bandwidth data transfer (tensors, embeddings)?
│  └─ Yes → Use shared memory + semaphores
│
├─ Is this agent output streaming (logs, metrics)?
│  └─ Yes → Use named pipes or gRPC server streaming
│
├─ Does this need to persist or replay?
│  └─ Yes → Use NATS JetStream
│
├─ Is this a simple event broadcast?
│  └─ Yes → Use ZeroMQ PUB/SUB or NATS core
│
└─ Is this a complex polyglot orchestration?
   └─ Yes → Use gRPC with protocol buffers
```

---

## Sources

- [Benchmark TCP/IP, Unix domain socket and Named pipe](https://www.yanxurui.cc/posts/server/2023-11-28-benchmark-tcp-uds-namedpipe/)
- [The Node.js Developer's Guide to Unix Domain Sockets: 50% Lower Latency Than TCP loopback](https://nodevibe.substack.com/p/the-nodejs-developers-guide-to-unix)
- [IPC Performance Comparison: Anonymous Pipes, Named Pipes, Unix Sockets, and TCP Sockets | Baeldung on Linux](https://www.baeldung.com/linux/ipc-performance-comparison)
- [POSIX Shared Memory in Linux - SoftPrayog](https://www.softprayog.in/programming/interprocess-communication-using-posix-shared-memory-in-linux)
- [Shared memory - Wikipedia](https://en.wikipedia.org/wiki/Shared_memory)
- [mmap - Wikipedia](https://en.wikipedia.org/wiki/Mmap)
- [5. Advanced Pub-Sub Patterns | ØMQ - The Guide](https://zguide.zeromq.org/docs/chapter5/)
- [2. Sockets and Patterns | ØMQ - The Guide](https://zguide.zeromq.org/docs/chapter2/)
- [3. Advanced Request-Reply Patterns | ØMQ - The Guide](https://zguide.zeromq.org/docs/chapter3/)
- [JetStream | NATS Docs](https://docs.nats.io/nats-concepts/jetstream)
- [How to Use NATS JetStream for Persistence](https://oneuptime.com/blog/post/2026-01-26-nats-jetstream-persistence/view)
- [Core concepts, architecture and lifecycle | gRPC](https://grpc.io/docs/what-is-grpc/core-concepts/)
- [gRPC in Go: Streaming RPCs, Interceptors, and Metadata](https://victoriametrics.com/blog/go-grpc-basic-streaming-interceptor/)
- [Performance best practices with gRPC | Microsoft Learn](https://learn.microsoft.com/en-us/aspnet/core/grpc/performance?view=aspnetcore-10.0)
- [Understanding Back Pressure in Message Queues: A Guide for Developers](https://akashrajpurohit.com/blog/understanding-back-pressure-in-message-queues-a-guide-for-developers/)
- [Avoiding a QUEUE Backlog Disaster with Backpressure & Flow Control - CodeOpinion](https://codeopinion.com/avoiding-a-queue-backlog-disaster-with-backpressure-flow-control/)
- [Backpressure explained — the resisted flow of data through software | by Jay Phelps | Medium](https://medium.com/@jayphelps/backpressure-explained-the-flow-of-data-through-software-2350b3e77ce7)
- [Named pipe - Wikipedia](https://en.wikipedia.org/wiki/Named_pipe)
- [fifo(7) - Linux manual page](https://man7.org/linux/man-pages/man7/fifo.7.html)
- [POSIX message queues in Linux - SoftPrayog](https://www.softprayog.in/programming/interprocess-communication-using-posix-message-queues-in-linux)
- [mq_overview(7) - Linux manual page](https://man7.org/linux/man-pages/man7/mq_overview.7.html)
- [You Cannot Have Exactly-Once Delivery – Brave New Geek](https://bravenewgeek.com/you-cannot-have-exactly-once-delivery/)
- [The impossibility of exactly-once delivery - Savvas' blog](https://blog.bulloak.io/post/20200917-the-impossibility-of-exactly-once/)
- [Exactly-Once Delivery of Message in Distributed System](https://www.linkedin.com/pulse/exactly-once-delivery-message-distributed-system-arun-dhwaj/)
- [Exactly-once Semantics is Possible: Here's How Apache Kafka Does it](https://www.confluent.io/blog/exactly-once-semantics-are-possible-heres-how-apache-kafka-does-it/)

---

*Authored by: Claude Haiku 4.5*
