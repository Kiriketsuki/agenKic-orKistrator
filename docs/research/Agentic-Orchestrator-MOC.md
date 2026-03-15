---
title: Agentic AI Orchestrator — Research Map of Content
tags: [research, orchestration, moc, ai-agents, pixel-office]
date: 2026-03-14
type: moc
---

# Agentic AI Orchestrator — Research MOC

Research conducted 2026-03-14 via 11-agent parallel research team. This MOC links all findings relevant to designing a **pixelated AI office orchestrator** — a visual, retro-aesthetic workspace that coordinates multiple AI models (Claude, Codex, Ollama, Gemini) with a rich terminal and desktop UI.

## Core Concept

> A visual "office world" rendered in pixel-art aesthetic where each AI agent appears as a worker at a desk. Users can see what agents are doing in real time, route tasks to the right model, and orchestrate multi-agent workflows — all from a delightful, retro-inspired native desktop application.

---

## Research Notes

### Architecture & Orchestration

- [[Agentic-Orchestration-Patterns]] → DAG model, supervisor/worker, ReAct loops, plan-and-execute, tool calling as primitive
- [[000-System/Agents/Claude/sessions/orchestration-research-report]] → Full deep-dive: LangGraph, CrewAI, AutoGen, Swarm comparisons
- [[Claude-Code-Multi-Agent-System]] → Anthropic's own TeamCreate/SendMessage/TaskCreate system internals

### Inter-Agent Communication

- [[IPC-Inter-Agent-Communication]] → Unix domain sockets, ZeroMQ patterns, gRPC, NATS, shared memory — latency & use-case comparison
- **Key rec**: Central orchestrator + gRPC for agent-to-agent + Redis Streams for durability

### State Management

- [[Agent-State-Management]] → Blackboard architecture, event sourcing, CRDTs, Redis hybrid, artifact passing, checkpointing
- **Key pattern**: Context window = RAM; externalize to Redis Streams + versioned artifacts

### Process Supervision

- [[000-System/Agents/Claude/sessions/agent-process-management-research]] → OTP supervision trees, supervisord, PM2, health probes, exponential backoff
- **Key insight**: "Let it crash" + hierarchical supervision trees scale better than defensive coding

### Multi-Model Coordination

- [[Multi-Model-Coordination]] → LiteLLM, OpenRouter, Ollama, Judge-Router-Agent pattern, evaluator models
- **Key rec**: Haiku-class judge routes tasks → 60-90%+ cost savings depending on workload (RouteLLM research)

---

### Terminal Infrastructure

- [[Terminal-Multiplexing-Tmux]] → tmux architecture, programmatic API, detach/reattach, agent pane patterns
- [[000-System/Agents/Research/orchestration-research-task-3-terminals]] → Repositionable terminals, WezTerm Lua API, X11 vs Wayland positioning
- **Key rec**: tmux as orchestration substrate; WezTerm Lua for programmatic pane layout

### Terminal UI / Chat Interfaces

- [[000-System/Agents/Claude/sessions/tui-chat-research-report]] → Bubbletea (Go), Ratatui (Rust), Textual (Python), ANSI primitives, Kitty graphics protocol
- **Key rec**: Bubbletea + Lip Gloss for Go; use Elm architecture for no render/state divergence

---

### Desktop Rendering (Pixel Art)

- [[research/pixelated-desktop-ui-framework-evaluation]] → Godot 4, Tauri, egui, raylib, Bevy — with terminal embedding analysis
- **Key rec**: **Godot 4 + godot-xterm** for MVP (3-4 weeks); raylib for minimal deps; Tauri for lightest deployment. **Caveat**: godot-xterm's PTY node (live shell/agent connection) is Linux + macOS only — Windows shows terminal display only. Factor into cross-platform planning.

### Competitive Landscape

- [[orchestration-research-findings]] → Pixel Agents (4.4k stars), MetaGPT (65.1k stars), Claw-Empire, Agent Farm, AgentOffice
- **The gap**: No project combines pixel visualization + deep orchestration + multi-provider + web-native deployment

---

## Key Architectural Decisions

| Decision | Recommendation | Rationale |
|---------|---------------|-----------|
| **Orchestration pattern** | Supervisor/worker + DAG (LangGraph-style) | Best debuggability + parallelism |
| **IPC** | gRPC agent-to-agent + Redis Streams | Type-safe, durable, backpressure |
| **State store** | Redis (Streams + Hashes) | Sub-ms reads, persistence, pub/sub |
| **Process supervision** | OTP-style hierarchical trees | "Let it crash" scales to N agents |
| **Terminal substrate** | tmux or WezTerm Lua | Battle-tested, programmatic control |
| **TUI framework** | Bubbletea (Go) or Ratatui (Rust) | Elm arch, no state divergence |
| **Desktop rendering** | Godot 4 + godot-xterm | Pixel-art native, terminal embedded |
| **Multi-model gateway** | LiteLLM + Judge-Router pattern | 60-90%+ cost savings (RouteLLM), provider agnosticism |
| **Model routing** | Haiku→judge, Sonnet→work, Opus→arch | Capability-to-cost matched |

## The Unique Positioning

Pixel Agents (4.4k ★) wins on visual appeal; MetaGPT (65.1k ★) wins on orchestration depth. **Neither has both.** An orchestrator combining:

- Pixel Agents' delight + agent visualization
- MetaGPT's orchestration depth
- Claw-Empire's multi-provider support (7 providers)
- Agent Farm's multi-agent safety patterns
- Web-native or Godot-native deployment

...would own a currently unoccupied category.

## Next Steps

- [ ] Spike: Godot 4 + godot-xterm — can a terminal pane run a live Claude agent?
- [ ] Design: agent state machine (idle → assigned → working → reporting → idle)
- [ ] Design: task routing algorithm (judge model → capability matrix → model selection)
- [ ] Prototype: tmux-based multi-agent test (5 agents, shared Redis state)
- [ ] Research: Anthropic Agent SDK for structured agent-to-agent calls

---

*Authored by: Clault KiperS 4.6*
