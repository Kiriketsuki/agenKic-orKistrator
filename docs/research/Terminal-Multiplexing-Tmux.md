---
title: Terminal Multiplexing & tmux for Agent Orchestration
tags: [research, orchestration, tmux, terminal, infrastructure]
date: 2026-03-14
type: research
---

# Terminal Multiplexing & tmux for Agent Orchestration

## Overview

tmux is the most proven and widely-adopted substrate for AI agent orchestration as of early 2026 — a battle-tested, 15+ year old terminal multiplexer with a complete programmatic API, SSH-friendly design, and a detach/reattach model that aligns naturally with async agent workflows. (Zellij is approaching a 1.0 production release with a plugin system that may challenge this position over time.)

## Core Architecture

tmux runs as a **server process** that manages a hierarchy of abstractions:

- **Session** — top-level container, survives client disconnects
- **Window** — tab-like pane groups within a session
- **Pane** — individual terminal cells, each running its own process

An orchestrator can spawn agents into panes, detach from the session, and reattach later to inspect results — all without the agent process being interrupted.

## Programmatic API

tmux exposes a complete control interface via CLI subcommands:

| Command | Purpose |
|---------|---------|
| `tmux new-session -d -s name` | Spawn a detached session |
| `tmux send-keys -t pane "cmd" Enter` | Inject input into a pane |
| `tmux capture-pane -t pane -p` | Read current pane output |
| `tmux split-window -h -t session` | Add a new pane |
| `tmux list-sessions / list-panes` | Query running sessions/panes |
| `tmux new-window -t session` | Create a new window |

Critical: `send-keys` and `Enter` must be sent as **separate commands** — appending `\n` to the command string does not work reliably.

## Key Patterns for Orchestration

### Detach/Reattach = Async-Native
Spawn agent → `detach` → continue orchestrating. Later `attach` to check results. A supervisor can manage 100+ agents without blocking on any single one.

### Multi-Client Attachment = Real-Time Observability
Multiple clients can attach to the same session simultaneously. A supervisor agent monitors workers in pane A while a developer watches live from another terminal — no agent restarts needed.

### Named Sessions as Agent Registry
```bash
tmux new-session -d -s "agent-codegen-01"
tmux new-session -d -s "agent-reviewer-01"
tmux list-sessions  # enumerate running agents
```

### Capturing Agent Output
```bash
# Get last 500 lines of agent output
tmux capture-pane -t agent-codegen-01 -p -S -500
```

## Alternatives

| Tool | Strengths | Weaknesses vs tmux |
|------|-----------|-------------------|
| **Zellij** | Plugin system, modern Rust, layout files; approaching v1.0 with growing Claude Code integration requests | Smaller ecosystem, less scripting maturity today |
| **WezTerm** | Lua scripting API, cross-platform, GPU-rendered | No server model, panes tied to GUI process |
| **Screen** | Universal availability | Limited pane management, dated API |

## Verdict

**tmux is the recommended orchestration substrate** for local multi-agent systems today. Its API is complete, SSH-friendly, and proven at scale. WezTerm is a strong alternative when a GUI with pixel-perfect rendering is desired. Zellij is the one to watch — its plugin system and active development make it a likely future contender.

## Coordination Caveats

Multiple agents writing to the same files causes conflicts. Solutions must be baked into the orchestrator:
- File locking (`flock`)
- Branch-per-agent git isolation
- Queue-based coordination (one writer, N readers)

## References

- [tmux man page](https://man7.org/linux/man-pages/man1/tmux.1.html)
- NTM (tmux-based multi-agent) patterns
- Agent Farm (Claude Code + tmux) — 691 GitHub stars

---

*Authored by: Clault KiperS 4.6*
