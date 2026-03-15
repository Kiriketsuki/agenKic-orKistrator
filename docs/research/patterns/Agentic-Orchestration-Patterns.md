---
title: Agentic Orchestration Patterns
tags: [ai, agents, orchestration, patterns, langgraph, crewai, react]
date: 2026-03-14
type: evergreen
---

# Agentic Orchestration Patterns

## The Fundamental Primitive: Tool Calling

Every agent action — whether calling an API, running code, or delegating to another agent — reduces to a **structured tool call** (name + JSON params). The LLM reasons about *which* tool to invoke; the runtime executes it. This is the atomic unit underlying all orchestration patterns.

## The ReAct Loop

```
Reason → Act (tool call) → Observe (result) → Reason → ...
```

The LLM interleaves reasoning traces with tool calls. Each observation feeds back into the next reasoning step. Flexible but token-expensive — requires an LLM invocation after every action.

## Control Flow Spectrum

### Agent-Driven (ReAct)
LLM decides next action at every step. Maximum flexibility, minimum predictability. Use for open-ended tasks where the path is unknown.

### Orchestrator-Driven (DAG)
Directed acyclic graph: nodes are agents/tools, edges are data flow. Deterministic, debuggable, fast. Use for well-defined multi-step workflows. LangGraph is the standard implementation.

### Hybrid (Plan-and-Execute)
Explicit planning phase produces a task list; execution phase runs steps in parallel where possible. ~10× cheaper than pure ReAct for structured workflows with variable steps.

### Hierarchical (Supervisor/Worker)
Central orchestrator decomposes tasks, delegates to specialist workers, aggregates results, handles retries. This is the dominant production pattern in 2025 — implemented natively by AWS Bedrock, Azure Agent Framework, MetaGPT.

## Framework Comparison

| Framework | Model | Differentiator |
|-----------|-------|---------------|
| **LangGraph** | DAG + state machine | Explicit graph, checkpointing, conditional edges |
| **CrewAI** | Role-based crews | Human-readable roles, structured output |
| **AutoGen** | Conversational agents | Multi-agent group chat |
| **Swarm** | Handoff-based | Simplest API — agent-to-agent handoffs |
| **MetaGPT** | Software company sim | PM → Architect → Engineer → QA |

All implement the same underlying primitives. Choice is API style, not capability.

## State Management

**Explicit versioned state** (LangGraph approach) beats implicit state (AutoGen):
- Every state transition is logged
- Checkpoints enable fault recovery
- Any prior state is reconstructable

## Robustness Checklist

**Brittle orchestrators have:**
- Hidden state passed implicitly between agents
- Non-deterministic agent selection
- Missing error propagation
- No termination conditions

**Robust orchestrators have:**
- Deterministic routing (rules or explicit DAG edges)
- Fault isolation (one crash doesn't cascade)
- All tool calls logged with inputs/outputs
- Idempotent operations (safe to retry)
- Explicit termination conditions in the state machine

## Dispatch Mechanisms

| Mechanism | Latency | Use When |
|-----------|---------|---------|
| Direct call | Lowest | Simple single-agent |
| Event-driven (SSE/WS) | Low | Real-time progress, human-in-the-loop |
| Queue | Low | Backpressure, ordering guarantees |
| Pub/Sub | Low | Fan-out to many workers |

Modern production systems use **streaming events** — discrete events per node/tool emit a live progress stream, enabling real-time monitoring and pause points.

## See Also

- [[IPC-for-Multi-Agent-Systems]] — how agents communicate at the transport level
- [[Process-Supervision-for-AI-Agents]] — how to keep agents alive and healthy
- [[Agent-State-Management]] — how agents share and persist state

---

*Authored by: Clault KiperS 4.6*
