# Agent State Management and Shared Context: Comprehensive Research Report

## Executive Summary

Managing agent state in multi-agent systems requires choosing between centralized (blackboard), distributed (event sourcing, CRDTs), and hybrid approaches. The context window is an agent's working memory—a finite resource requiring strategic compression, externalization, and checkpoint recovery. This report synthesizes 10 core patterns for building resilient, scalable agentic systems.

---

## 1. Blackboard Architecture: Collaborative Problem-Solving

### Overview
The blackboard system is a classic AI pattern where multiple independent agents (called knowledge sources) collaborate by reading from and writing to a shared workspace—the "blackboard." A control component orchestrates which agent acts next based on the current state.

### How It Works
1. **Shared Knowledge Base**: The blackboard serves as public shared memory
2. **Opportunistic Reasoning**: Agents respond dynamically to evolving state, not predetermined sequences
3. **Multi-Agent Collaboration**: Diverse specialist agents incrementally contribute partial solutions
4. **Control Unit**: Decides activation order based on blackboard state

### Modern Applications
- Multi-agent LLM coordination (e.g., specialized roles reading/writing to a common state)
- Complex problem-solving requiring incremental contributions
- Asynchronous agent communication without direct coupling

### Strengths & Trade-offs
✓ **Strengths**: Clear separation of concerns, asynchronous coordination, simple to understand
✗ **Trade-offs**: Central bottleneck, state consistency challenges at scale, requires careful conflict resolution

### Sources
- [Blackboard system - Wikipedia](https://en.wikipedia.org/wiki/Blackboard_system)
- [Building Intelligent Multi-Agent Systems with MCPs and the Blackboard Pattern - Medium](https://medium.com/@dp2580/building-intelligent-multi-agent-systems-with-mcps-and-the-blackboard-pattern-to-build-systems-a454705d5672)
- [Collaborative Problem-Solving in Multi-Agent Systems with the Blackboard Architecture - Engineering Notes](https://notes.muthu.co/2025/10/collaborative-problem-solving-in-multi-agent-systems-with-the-blackboard-architecture/)

---

## 2. Tuple Spaces (Linda Model): Associative Memory for Agents

### Overview
Linda is a coordination model for parallel computing that provides a distributed shared memory called a tuple space. Processes communicate through tuples—ordered sequences of typed data—rather than direct message passing.

### Core Operations
- **`out(tuple)`**: Write a tuple to the space
- **`in(tuple)`**: Remove a matching tuple (blocking operation)
- **`rd(tuple)`**: Read/inspect a tuple without removing it (non-destructive, blocking)
- **Pattern Matching**: Tuples matched by pattern, not by name

### Key Advantage
Processes are completely decoupled—they need no notion of other processes, only the kinds of tuples they produce or consume.

### Modern Relevance
- Foundational concept for distributed coordination
- Basis for modern event-driven architectures
- Maps to content-based routing in message brokers
- Useful for agent communication where multiple agents inspect shared facts

### Linda Implementation Characteristics
- Simple API with only six core operations
- Tuples addressed as ordered sequences of typed fields (up to 16 fields traditionally)
- Non-destructive reads (`rd`) enable multiple agents to access same information
- Blocking semantics naturally handle synchronization

### Sources
- [Linda (coordination language) - Wikipedia](https://en.wikipedia.org/wiki/Linda_(coordination_language))
- [The Linda Model and System - NetLib](https://netlib.org/utk/papers/comp-phy7/node3.html)
- [Tuple Space Overview - ScienceDirect](https://www.sciencedirect.com/topics/computer-science/tuple-space)

---

## 3. Event Sourcing: Immutable Event Logs as Source of Truth

### Overview
Event sourcing logs every state change as an immutable sequence of events. The current state is derived by replaying the event log—creating a complete, auditable history of all agent decisions.

### Architecture Pattern
```
Commands → Events → Event Log (immutable) → State (derived) → Queries
```

### Key Benefits for Multi-Agent Systems
1. **Resilience**: Recover from failures by replaying events up to failure point
2. **Auditability**: Every agent decision is permanently recorded
3. **Coordination**: Agents communicate through durable events, not ephemeral messages
4. **Context Passing**: Event log serves as mechanism for passing context between agents

### Implementation Advantages
- **Replayability**: Reconstruct any past state by replaying partial log
- **Snapshotting**: Periodically save state snapshots for faster recovery
- **Sophisticated Consumer Models**: Multiple agents consume same event stream independently
- **Temporal Queries**: Ask "what was the state at time T?" by replaying up to T

### Design Patterns in Multi-Agent Context
- **Orchestrator-Worker**: Central orchestrator delegates to workers via event stream
- **Hierarchical Agent**: Parent agents emit events consumed by child agents
- **Market-Based**: Agents bid on work, decisions recorded as events

### Sources
- [Four Design Patterns for Event-Driven, Multi-Agent Systems - Confluent](https://www.confluent.io/blog/event-driven-multi-agent-systems/)
- [Event Sourcing: The Backbone of Agentic AI - Akka](https://akka.io/blog/event-sourcing-the-backbone-of-agentic-ai)
- [ESAA: Event Sourcing for Autonomous Agents - ArXiv](https://arxiv.org/pdf/2602.23193)

---

## 4. CRDTs: Conflict-Free Replicated Data Types for Concurrent Agents

### Overview
A CRDT is a data structure allowing independent, concurrent modifications across multiple replicas without coordination. Conflicts automatically resolve to a consistent state.

### The Core Problem CRDT Solves
In distributed systems:
- **Strong Consistency**: Coordinate all changes → slow, unavailable during disconnection
- **Optimistic Replication**: Modify independently → fast, available, but conflicts emerge
- **CRDT Solution**: Automatic conflict resolution without user intervention or central server

### How CRDTs Work
1. **Decentralized Operation**: No central server needed
2. **Conflict Automatic Resolution**: Algebraic properties ensure convergence
3. **Offline Operation**: Devices work independently, sync later
4. **Merger Semantics**: Multiple replicas automatically merge to same final state

### Two CRDT Approaches
1. **State-based (CvRDTs)**: Send entire state, merge using join operation
2. **Operation-based (CmRDTs)**: Send operations, apply to local state

### Common CRDT Types
- **G-Counter**: Grow-only counter (increment only)
- **OR-Set**: Observed-Remove Set (add/remove with unique IDs)
- **LWW-Register**: Last-Write-Wins Register (keeps latest write by timestamp)
- **Text CRDT**: Character-level operations for collaborative editing

### Agent System Applications
- Agent state replication without central coordination
- Distributed decision-making where agents propose changes
- Peer-to-peer agent networks (no single point of failure)
- Collaborative task planning where agents contribute independently

### Sources
- [About CRDTs - crdt.tech](https://crdt.tech/)
- [Conflict-free replicated data type - Wikipedia](https://en.wikipedia.org/wiki/Conflict-free_replicated_data_type)
- [Diving into CRDTs - Redis](https://redis.io/blog/diving-into-crdts/)

---

## 5. Vector Clocks & Lamport Timestamps: Ordering Events Without Global Clock

### Overview
In distributed systems without synchronized clocks, vector clocks and Lamport timestamps provide logical ordering of events.

### Lamport Timestamps (Simple Ordering)
- **What It Is**: A counter incremented with every event
- **How It Works**: Each process maintains a counter, increments on local event, timestamps messages, receives max(local, received)
- **Guarantee**: If timestamp(A) < timestamp(B), then A happened before B
- **Limitation**: Doesn't capture causality; clock values alone don't reveal if events are concurrent

### Vector Clocks (Causal Ordering)
- **What It Is**: Vector of N counters (one per agent/process)
- **Comparison Rule**: `C1 < C2` iff all C1 values ≤ C2 values AND at least one < (strict ordering)
- **What You Learn**:
  - Concurrent events: C1 and C2 incomparable → events happened without causal relationship
  - Causal ordering: C1 < C2 → C1 caused C2
  - Message causality: Detect if two messages are causally related

### Agent System Applications
- **Detect Concurrent Decisions**: Identify when agents made independent decisions
- **Causal Message Ordering**: Ensure agents process messages in causally consistent order
- **Distributed Consensus**: Determine if events are independent or ordered
- **Debugging**: Understand message causality chains in agent traces

### Vector Clock Example
```
System with 3 agents: [A_counter, B_counter, C_counter]
A sends message with [1, 0, 0]
B receives, increments its counter: [1, 1, 0]
B sends to C with [1, 1, 0]
C receives: [1, 1, 1]
```

### Trade-offs
- **Lamport**: O(1) space per timestamp, one total order, doesn't reveal causality
- **Vector Clocks**: O(N) space (N = number of agents), reveals all causal relationships

### Sources
- [Vector Clocks - Kevin Sookocheff](https://sookocheff.com/post/time/vector-clocks/)
- [Time, Clocks, and the Ordering of Events - Lamport's Original Paper](https://lamport.azurewebsites.net/pubs/time-clocks.pdf)
- [Lamport Clock - Baeldung on Computer Science](https://www.baeldung.com/cs/lamport-clock)

---

## 6. Redis as Agent Shared Store: In-Memory State & Event Coordination

### Overview
Redis provides sub-millisecond access to agent state and supports multiple coordination patterns: pub/sub for real-time signals, streams for message persistence and replayability, hashes for agent state.

### Redis Data Structures for Multi-Agent Systems

#### Pub/Sub: Real-Time, Ephemeral Communication
- **Use Case**: Live notifications, status updates, agent discovery
- **Characteristics**: Fire-and-forget, no persistence, broadcasts to all subscribers
- **Limitation**: If agent offline, it misses messages

#### Streams: Persistent, Ordered Event Log
- **Commands**: `XADD` (write), `XREAD` (consume)
- **Characteristics**: Immutable event log, consumer groups, last-known position tracking
- **Use Case**: Agent task distribution, event sourcing, event replay
- **Advantage**: If agent crashes, it resumes from where it left off

#### Hashes: Agent State Snapshots
- **Use Case**: Store agent-specific state: current goal, last tool output, context window summary
- **Characteristics**: Sub-millisecond reads for hot state
- **Benefit**: Faster than disk-backed databases for in-memory working state

### Hybrid Architecture for Long-Running Agents
```
Streams (events)  → distributed task queue
Hashes (state)    → agent working state
Pub/Sub (signals) → real-time notifications
```

### Performance Characteristics
- **Sub-millisecond local reads**: On localhost or <1ms RTT networks
- **Task Distribution**: Agent process consumes from dedicated stream
- **State Tracking**: Redis hashes track task lifecycle (pending → running → completed → failed)

### Scalability Considerations
- In-memory constraint: Redis not suitable for multi-GB agent context
- Hybrid: Redis for hot state, S3/GCS for large artifacts
- Consumer groups: Scale multiple agent workers consuming same task stream
- Sharding: Partition agent streams by agent ID for horizontal scaling

### Sources
- [Agent State Management: Redis vs Postgres - SitePoint](https://www.sitepoint.com/state-management-for-long-running-agents-redis-vs-postgres/)
- [Redis Pub/Sub vs Streams - Redis Docs](https://redis.io/docs/latest/develop/pubsub/)
- [What to Choose: Redis Streams, Redis Pub/Sub, Kafka - Redis Blog](https://redis.io/blog/what-to-choose-for-your-synchronous-and-asynchronous-communication-needs-redis-streams-redis-pub-sub-kafka-etc-best-approaches-synchronous-asynchronous-communication/)

---

## 7. LLM Context Windows & State: Memory Management Strategies

### Overview
The context window is the agent's working memory—finite tokens that must contain everything the model can access. Strategies include compression, summarization, externalization, and isolation.

### Context Window as Working Memory
- **What It Is**: The model's RAM; everything immediately available for reasoning
- **Constraint**: Token limits (Claude 3.5 Sonnet: 200K tokens; Haiku: 100K; Opus: 200K)
- **Performance Cliff**: When exceeding 95% capacity, auto-compact triggers (in Claude Code)

### Four Core Management Strategies

#### 1. Write Context: External Storage
- **Scratchpad**: Agent writes intermediate reasoning, discoveries to files
- **Long-term Memory**: Persist facts across sessions (like ChatGPT/Cursor user preferences)
- **Observation Masking**: Replace old tool outputs with placeholders; keep tool calls visible
- **Benefit**: Agent remembers what actions it took without carrying full output

#### 2. Select Context: Targeted Retrieval
- **Embeddings + RAG**: Fetch most relevant memories for current task
- **Knowledge Graphs**: Navigate to relevant facts in large memory
- **Tool Selection**: Use embeddings to find most useful tools from large collections
- **Benefit**: Brings in only what's needed, reduces noise

#### 3. Compress Context: Token Reduction
- **Summarization**: LLM rewrites old messages into condensed natural language
  - Risk: May hallucinate details, lose technical specifics
  - Benefit: Maximum semantic preservation
- **Compaction**: Delete tokens from original text (character-for-character faithful)
  - Risk: May lose context
  - Benefit: Lossless, no hallucinations
- **Observation Masking**: Keep structure, replace details with placeholders
- **Recent Research**: ACON framework optimizes both observation and interaction history compression

#### 4. Isolate Context: Separate Agents
- **Subagent Windowing**: Dedicate separate context windows for independent tasks
- **Sandboxes**: Store token-heavy objects (images, audio) separately
- **Hierarchical Agents**: Parent maintains overview, children handle detailed work

### Context Poisoning Problems (When Compression Fails)
- **Distraction**: Irrelevant details overwhelm reasoning
- **Confusion**: Conflicting information from stale context
- **Clash**: Long context introduces subtle contradictions

### Real-World Scale Example
- **Task**: Code review of 1000-file codebase
  - Old approach: Exceed context window, hallucinate
  - Scratchpad approach: External file-by-file log of issues, agent only loads relevant section when re-examining
- **Benefit**: 100-1000x context efficiency improvement

### Sources
- [The Ultimate Guide to LLM Memory - Medium](https://medium.com/@sonitanishk2003/the-ultimate-guide-to-llm-memory-from-context-windows-to-advanced-agent-memory-systems-3ec106d2a345)
- [Context Management for Deep Agents - LangChain Blog](https://blog.langchain.com/context-management-for-deepagents/)
- [Compaction vs Summarization - Morph](https://www.morphllm.com/compaction-vs-summarization)
- [ACON: Optimizing Context Compression - ArXiv](https://arxiv.org/html/2510.00615v2)
- [Context Engineering for Agents - LangChain](https://blog.langchain.com/context-engineering-for-agents/)

---

## 8. Artifact Passing: File Transfer & Agent Handoff

### Overview
Artifacts are large binary or textual data (files, logs, images) stored outside the context window and referenced by name/version. Agents hand off control by passing artifacts to successor agents.

### The Artifact Pattern
```
Context Window (chat, reasoning) ← [reference] → Artifact Store (files, blobs)
```

### Key Concepts

#### Artifacts as External File System
- **What They Are**: Persistent data addressed by name and version
- **Storage**: Cloud storage (GCS, S3) with versioning
- **Addressing**: `{app_name}/{user_id}/{session_id}/{filename}`
- **Benefit**: Decouples reasoning (context) from storage; scales to 100K+ records

#### Agent Handoff Semantics
- **Control Transfer**: Sub-agent receives view over session and takes over workflow
- **Artifact Verification**: Project Manager Agent confirms artifacts exist before handoff
- **Context Inheritance**: Sub-agent inherits access to all parent artifacts
- **Chain of Agents**: Agents can transfer further down the chain

#### Storage Implementation
- **Google ADK Artifacts**: Each version stored as separate blob in GCS bucket
- **Indexing**: (app_name, user_id, session_id, filename) → blob ID
- **Versioning**: Multiple versions of same file tracked separately
- **Permissions**: Sub-agents inherit parent's artifact access

### Practical Examples
1. **Data Processing Pipeline**:
   - Agent 1: Extract data → save to artifact
   - Agent 2: Transform data → save new version
   - Agent 3: Validate → confirm with Manager, then handoff to Agent 4

2. **Large File Analysis**:
   - Agent 1: Fetch 100MB log file → store as artifact (not in context)
   - Agent 2: Parse and index sections → create summary artifacts
   - Agent 3: Query summaries, fetch specific sections as needed

### Scaling Benefits
- **100 → 100K records**: Same architecture handles both
- **Context efficiency**: Only load relevant portions into context window
- **Concurrent agent access**: Multiple agents read same artifact without duplication

### Sources
- [Introducing Google ADK Artifacts - Medium](https://medium.com/google-cloud/introducing-google-adk-artifacts-for-multi-modal-file-handling-a-rickbot-blog-08ca6adf34c2)
- [Artifacts - Agent Development Kit Docs](https://google.github.io/adk-docs/artifacts/)
- [Context Is Not a Storage Unit: The Artifact Pattern - YESS.ai](https://www.yess.ai/post/context-is-not-a-storage-unit)
- [Handoffs - OpenAI Agents SDK](https://openai.github.io/openai-agents-python/handoffs/)

---

## 9. Checkpointing & Recovery: Mid-Task Resumption

### Overview
Checkpointing saves an agent's running state to persistent storage, enabling recovery from crashes without restarting from the beginning.

### Core Motivation
- **Failure Cost**: Long-running tasks fail 20-30% of the time
- **Reprocessing Waste**: Without checkpoints, re-run entire task (cost: 100%)
- **With Checkpoints**: Resume from last saved state (cost: 5-40% of original)
- **ROI**: Save 60% of wasted processing time

### State Components to Checkpoint

1. **Mission State & Planning**
   - Main goal, completed steps, remaining tasks
   - Agent's current hypothesis or direction

2. **Tool & Environment Context**
   - Database cursors, search result offsets
   - Temporary file handles, API session tokens
   - Last known state of external systems

3. **System & Model Configuration**
   - Model version (important for reproducibility)
   - Temperature, top_p, and other sampling parameters
   - Prompt templates and dynamic instructions

### Implementation Approaches

#### Synchronous Checkpointing
- Persist state **before** next step executes
- Guarantees: Every checkpoint survives before continuing
- Trade-off: Performance overhead (blocks execution)
- Use When: High durability requirements, moderate-frequency checkpoints

#### Asynchronous Checkpointing
- Background process persists state
- Benefit: No execution blocking
- Risk: Checkpoint may lag behind current state
- Use When: Throughput-critical, acceptable staleness

#### Snapshot-Based Recovery
- Checkpoint captures complete graph state:
  - Config and metadata
  - State channel values (agent state, tool outputs, conversation history)
  - Next nodes to execute
  - Task-specific information
- Recovery: Load snapshot, resume from next node

### Advanced Patterns

#### Incremental Checkpointing
- Instead of full state snapshots, log only state changes
- Checkpoint log = event stream similar to event sourcing
- Replay events up to crash point to recover full state

#### Hybrid Periodic + Event-Driven
- Periodic snapshots (every N steps) for fast recovery
- Event-driven checkpoints on critical state changes
- Balance: Fast recovery + lower overhead

### Sources
- [AI Agent State Checkpointing: A Practical Guide - Fast.io](https://fast.io/resources/ai-agent-state-checkpointing/)
- [What Does Checkpointing Mean - Dagster Glossary](https://dagster.io/glossary/checkpointing)
- [Checkpoint/Restore Systems: Evolution, Techniques, and Applications - yunwei37](https://www.yunwei37.com/blog/check-restore)
- [Build durable AI agents with LangGraph and DynamoDB - AWS Blog](https://aws.amazon.com/blogs/database/build-durable-ai-agents-with-langgraph-and-amazon-dynamodb/)

---

## 10. Scratchpad Pattern: External Intermediate Reasoning

### Overview
The scratchpad is structured external memory where agents write intermediate reasoning, plans, and discoveries—visible to the agent but not exposed to end users.

### What It Is
- **External Working Memory**: Whiteboard for intermediate reasoning outside context window
- **Ephemeral**: Not persistent across sessions (unlike long-term memory)
- **Explicit**: Model writes structured notes, not just internal reasoning
- **Accessible**: Agent can read and revise its own notes

### Practical Applications

#### Code Review Use Case
```
Agent reads file 1 → writes scratchpad: "Issue found: null check missing"
Agent reads file 2 → reads scratchpad, continues noting other issues
Agent finishes → delivers final list (scratchpad never shown to user)
```

#### Multi-Step Complex Tasks
1. **Breakdown Phase**: Write task decomposition to scratchpad
2. **Execution Phase**: Update scratchpad with progress after each step
3. **Reflection Phase**: Review scratchpad, adjust approach if needed

#### Reasoning Frameworks

**ReAct Agent Pattern**:
```
{agent_scratchpad} = [
  "Thought: <reasoning>",
  "Action: <tool>",
  "Observation: <result>",
  ... (previous iterations)
]
```
Agent includes scratchpad in context, appends new entries iteratively.

**Self-Notes for Iterative Reasoning**:
- Model explicitly writes notes to itself
- Notes act as both intermediate reasoning steps AND working memory for state-tracking
- Enables models to think through complex problems iteratively, not single-pass

### Memory Hierarchy Integration
- **Working Memory**: Scratchpad (current task reasoning)
- **Medium-Term**: Compressed summaries of recent sessions
- **Long-Term**: Persistent facts across sessions
- **External**: Artifacts and files outside context

### Why It Works
- **Reduces Context Pollution**: Intermediate reasoning doesn't bloat context
- **Supports Iteration**: Model can revise based on scratchpad notes
- **Improves Reasoning Quality**: Writing forces explicit thinking vs. implicit
- **Enables Recovery**: If agent crashes, scratchpad shows where it was

### Implementation Considerations
- **Format**: Structured markdown or JSON (not freeform text)
- **Scope**: Task-local (not cross-session) unless explicitly saved
- **Access**: Agent reads/writes to local file or in-memory structure
- **Cleanup**: Discard after task complete (unless promoting to long-term memory)

### Sources
- [Show Your Work: Scratchpads for Intermediate Computation - OpenReview](https://openreview.net/pdf?id=HBlx2idbkbq)
- [What is a ReAct Agent? - IBM](https://www.ibm.com/think/topics/react-agent)
- [Learning to Reason and Memorize with Self-Notes - ArXiv](https://arxiv.org/pdf/2305.00833)
- [Agentic Memory Patterns & Context Engineering - Medium](https://medium.com/@chetankerhalkar/agentic-memory-patterns-context-engineering-the-real-os-of-ai-agents-614cf0cf98b3)

---

## Synthesis: Architectural Decision Matrix

| Pattern | State Model | Coordination | Consistency | Fault Recovery | Best For |
|---------|------------|--------------|-------------|----------------|----------|
| **Blackboard** | Centralized shared state | Polling/events | Eventually consistent | Replay from blackboard | Discrete problem-solving tasks |
| **Linda/Tuple Space** | Associative memory | Pattern matching | Immediate (blocking) | Restart agents | Loosely coupled agents |
| **Event Sourcing** | Immutable event log | Event stream | Eventually consistent | Replay log | Audit-critical workflows, multi-step processes |
| **CRDTs** | Replicated with conflict resolution | P2P merge | Strong eventual consistency | Merge at recovery | Offline-capable, peer-to-peer networks |
| **Lamport Timestamps** | Logical ordering | Message piggybacking | Eventual ordering | Resync with timestamps | Simple causality tracking |
| **Vector Clocks** | Causal ordering | Message piggybacking | Causal consistency | Resync with vectors | Complex causality, concurrent detection |
| **Redis (Streams)** | In-memory + persisted events | Pub/Sub or Streams | Strong consistency (single point) | Consumer group offset | High-throughput, real-time agents |
| **Context Window Strategy** | External artifact + in-memory window | Direct access + retrieval | Local (per agent) | Rehydrate from artifacts | Token-constrained reasoning |
| **Artifacts** | External versioned files | Direct reference | Strong consistency (cloud storage) | Object versioning | Large data, multi-agent pipelines |
| **Checkpointing** | Snapshot + event log | Save on critical events | Depends on checkpoint frequency | Replay from checkpoint | Long-running, fault-sensitive tasks |
| **Scratchpad** | Task-local working memory | Internal note-taking | Local consistency | Task restart | Complex multi-step reasoning |

---

## Recommended Integration Strategy for Production Agent Systems

### Tier 1: Foundation
- **State Model**: Event Sourcing (immutable audit trail)
- **Ordering**: Vector Clocks (understand causality)
- **Storage**: Redis Streams (task queue) + Cloud Storage (artifacts)

### Tier 2: Resilience
- **Checkpointing**: Synchronous after each agent step
- **Recovery**: Replay checkpoint, re-run from next node
- **Conflict Resolution**: CRDTs for peer-to-peer agent replication

### Tier 3: Intelligence
- **Context Management**: Write (scratchpad) + Select (embeddings) + Compress (observation masking)
- **Memory Hierarchy**: Working (scratchpad) → Medium (summaries) → Long (artifacts)
- **Reasoning**: ReAct with scratchpad, Self-Notes for iteration

### Tier 4: Coordination
- **Multi-Agent**: Blackboard for shared problem state + Event Sourcing for decisions
- **Handoff**: Artifact passing + explicit control transfer
- **Verification**: Project Manager validates artifacts before handoff

---

## Key Takeaways

1. **Context Window is RAM**: Treat it as scarce working memory, not storage
2. **Events Over Mutations**: Immutable event logs enable audit, replay, and recovery
3. **Decoupling via Artifacts**: Keep reasoning (context) separate from storage (artifacts)
4. **Causal Ordering Matters**: Vector clocks reveal concurrent vs. ordered decisions
5. **Resilience is Mandatory**: Checkpointing and recovery are not optional for production agents
6. **Hybrid is Optimal**: Combine patterns—event sourcing (state) + Redis (coordination) + artifacts (storage)
7. **Compression is Unavoidable**: No single-agent system escapes context window limits; choose compression strategy early
8. **Scratchpad Amplifies Reasoning**: Explicit working memory improves both reasoning quality and debuggability

---

**Report Generated**: 2026-03-14
**Research Scope**: 10 core patterns for agent state management and shared context
**Sources Reviewed**: 40+ academic papers, blog posts, documentation, and implementation guides
