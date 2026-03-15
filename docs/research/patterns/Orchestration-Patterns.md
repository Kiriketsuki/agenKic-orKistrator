---
title: Agentic AI Orchestration Patterns
tags: [ai, agents, orchestration, patterns, langgraph, react, dag]
type: reference
---

# Agentic AI Orchestration Patterns

## The Fundamental Primitive

**Tool calling** (structured function call: name + JSON params) is the atomic action unit of every agentic system. The LLM decides *which* tool to call; the runtime executes it. Everything else — ReAct loops, handoffs, DAGs — is built on top of this.

---

## Control Flow Spectrum

### ReAct Loop (agent-driven)
```
Reason → Act (tool call) → Observe result → Reason → ...
```
- LLM decides the next step at every iteration
- Flexible, adaptive — good for open-ended tasks
- Expensive: one LLM call per action step

### Plan-and-Execute (hybrid)
- **Plan phase**: LLM produces a full task list upfront
- **Execute phase**: steps run in parallel where possible
- ~10× cheaper than pure ReAct for structured tasks

### DAG / Workflow (orchestrator-driven)
- Nodes = agents or tools; edges = data flow
- Deterministic, debuggable, fast
- LangGraph is the dominant implementation (2025 standard)

### Supervisor/Worker (hierarchical)
- Central orchestrator decomposes and delegates
- Workers run in parallel, report structured results
- Standard pattern in AWS Bedrock, Azure Agent Framework, MetaGPT

**Rule of thumb**: Start with supervisor/worker + DAG. Use ReAct only inside workers where adaptation is needed.

---

## State Management

- Pass state **explicitly** between nodes (LangGraph approach) — not hidden in conversation history
- **Checkpoint** at every node boundary → enables fault recovery without full restart
- Log every tool call input/output → full audit trail

---

## Robustness Checklist

- [ ] Deterministic routing (no random agent selection)
- [ ] Fault isolation (one crashed agent doesn't cascade)
- [ ] Explicit termination conditions (no infinite loops)
- [ ] Idempotent operations (safe to retry)
- [ ] All tool calls logged with inputs and outputs

---

## Framework Comparison (2025)

| Framework | Model | Best For |
|-----------|-------|---------|
| LangGraph | DAG + state machine | Maximum control, debugging |
| CrewAI | Role-based crews | Readable role definitions |
| AutoGen | Conversational | Multi-agent group chat |
| OpenAI Swarm | Handoff-based | Simplest API |
| MetaGPT | Software company simulation | Full dev lifecycle |

All implement the same primitives. Pick based on API style preference.

---

## Dispatch Mechanisms

| Mechanism | Use When |
|-----------|---------|
| Direct call | Single-agent, synchronous |
| Event stream (SSE/WebSocket) | Real-time progress visibility |
| Task queue | Backpressure, ordering needed |
| Pub/Sub | Fan-out to many workers |

---

## References

- Anthropic "Building Effective Agents" (Dec 2024)
- LangGraph docs — node/edge/state model
- MetaGPT ICLR 2025 paper

---

*Authored by: Clault KiperS 4.6*
