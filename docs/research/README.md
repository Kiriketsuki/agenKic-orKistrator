# Research — agenKic-ochKistrator

Research conducted 2026-03-14 via 11-agent parallel research team + adversarial council verification. Start with the MOC.

## Start Here

- [Agentic-Orchestrator-MOC.md](./Agentic-Orchestrator-MOC.md) — master map linking all findings, key architectural decisions, and recommended tech stack

## Raw Research (Session Notes)

| File | Topic |
|------|-------|
| [orchestration-research-findings.md](./orchestration-research-findings.md) | Competitive landscape — 20+ projects analyzed |
| [Agentic-Orchestration-Patterns.md](./Agentic-Orchestration-Patterns.md) | DAG, supervisor/worker, ReAct, plan-and-execute |
| [Claude-Code-Multi-Agent-System.md](./Claude-Code-Multi-Agent-System.md) | Anthropic's TeamCreate/SendMessage/TaskCreate internals |
| [claude-code-multi-agent-research.md](./claude-code-multi-agent-research.md) | Claude Code agent research deep-dive |
| [IPC-Inter-Agent-Communication.md](./IPC-Inter-Agent-Communication.md) | Unix sockets, ZeroMQ, gRPC, NATS, Redis Streams |
| [Agent-State-Management.md](./Agent-State-Management.md) | Blackboard, event sourcing, CRDTs, Redis hybrid |
| [agent-state-management-report.md](./agent-state-management-report.md) | Detailed state management report |
| [Process-Supervision.md](./Process-Supervision.md) | OTP supervision trees, supervisord, PM2, health probes |
| [Multi-Model-Coordination.md](./Multi-Model-Coordination.md) | LiteLLM, OpenRouter, Judge-Router pattern, RouteLLM |
| [Terminal-Multiplexing-Tmux.md](./Terminal-Multiplexing-Tmux.md) | tmux architecture, programmatic API, agent pane patterns |
| [Repositionable-Terminals.md](./Repositionable-Terminals.md) | WezTerm Lua API, X11 vs Wayland positioning |
| [pixelated-desktop-ui-framework-evaluation.md](./pixelated-desktop-ui-framework-evaluation.md) | Godot 4, Tauri, egui, raylib, Bevy — terminal embedding |
| [2026-03-14-council-verify-orchestration-notes.md](./2026-03-14-council-verify-orchestration-notes.md) | Adversarial council fact-check — corrections and caveats |
| [2026-03-14-council-verify-all-notes.md](./2026-03-14-council-verify-all-notes.md) | Full council verification across all research notes |
| [2026-03-14-session-insights.md](./2026-03-14-session-insights.md) | Session meta-insights |

## Refined Patterns Reference

The [`patterns/`](./patterns/) directory contains distilled reference notes from the Digital Garden — cleaner, more canonical versions of the core concepts.

| File | Topic |
|------|-------|
| [Agentic-Orchestration-Patterns.md](./patterns/Agentic-Orchestration-Patterns.md) | Core orchestration patterns |
| [Agent-State-Management.md](./patterns/Agent-State-Management.md) | State management patterns |
| [IPC-for-Multi-Agent-Systems.md](./patterns/IPC-for-Multi-Agent-Systems.md) | IPC reference |
| [Multi-Agent-Communication.md](./patterns/Multi-Agent-Communication.md) | Agent-to-agent messaging |
| [Multi-Model-Coordination.md](./patterns/Multi-Model-Coordination.md) | Multi-LLM routing |
| [Orchestration-Patterns.md](./patterns/Orchestration-Patterns.md) | Orchestration strategies |
| [Process-Supervision.md](./patterns/Process-Supervision.md) | Supervision trees |
| [Process-Supervision-for-AI-Agents.md](./patterns/Process-Supervision-for-AI-Agents.md) | Agent-specific supervision |
| [Terminal-Infrastructure-for-Agents.md](./patterns/Terminal-Infrastructure-for-Agents.md) | Terminal integration |
| [Native-Desktop-Rendering.md](./patterns/Native-Desktop-Rendering.md) | Desktop UI rendering |
| [Agentic-AI-for-Office-Productivity.md](./patterns/Agentic-AI-for-Office-Productivity.md) | Office productivity use cases |
| [Agentic-AI-for-Sensitive-Data.md](./patterns/Agentic-AI-for-Sensitive-Data.md) | Data security in agents |
| [Skill Seekers.md](./patterns/Skill%20Seekers.md) | Skill library patterns |
| [SRS and Agentic AI.md](./patterns/SRS%20and%20Agentic%20AI.md) | Requirements specifications |
