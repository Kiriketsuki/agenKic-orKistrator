---
title: Session Insights — 2026-03-14
date: 2026-03-14
type: insights
tags: [meta, continuous-learning, instincts, workflow]
---

# Session Insights — 2026-03-14

Derived from **1,536 observations** across **31 sessions** in the obKidian project (continuous-learning-v2 data). No formal instincts have been generated yet — the session-guardian cooldown blocks the observer from running during active work. These insights are hand-synthesized from raw tool events and search patterns.

---

## Workflow Patterns

### Tool Usage Distribution

| Tool | Calls | Observation |
|------|-------|-------------|
| WebSearch | 170 | Dominant. Extensive research before every implementation decision. |
| Read | 161 | Heavy file reading — good practice, reads before edits. |
| SendMessage | 109 | High multi-agent orchestration — sub-agents are core to workflow. |
| Bash | 89 | Infrastructure, git, package management. |
| Agent | 45 | Spawning specialized agents frequently. |
| Write | 43 | File creation heavily tied to infrastructure and templates. |
| WebFetch | 40 | Direct doc fetching alongside search. |
| Glob | 27 | Pattern-based file discovery. |
| TaskUpdate | 26 | Active task lifecycle management. |
| Edit | 19 | Targeted edits. Much less than Writes. |

**Pattern**: Research → Orchestrate → Build. WebSearch + Read dominate before any Write/Edit.

### Session Characteristics

- Average session: **26 tool calls**
- Largest session: **128 tool calls** (complex orchestration)
- Median is small — most sessions are focused and short

---

## Domain Focus

All major search queries cluster into four themes:

### 1. Agentic Orchestration (primary research area)
- LangGraph, AutoGen, CrewAI, OpenAI Swarm
- ReAct (Observe-Think-Act), CAMEL, Voyager
- Supervisor/worker, map-reduce, DAG, event-loop patterns
- Polling vs event-driven vs pub-sub dispatch

### 2. Terminal & TUI Systems
- tmux programmatic control (send-keys, capture-pane, named pipes)
- Ratatui, Textual, Bubbletea, Charm.sh
- Terminal orchestration grid layouts (4-8 pane setups)
- ANSI escape codes, sixel/Kitty graphics protocol

### 3. IPC & Messaging
- ZeroMQ patterns (PUB/SUB, PUSH/PULL, DEALER/ROUTER)
- Unix domain sockets, POSIX message queues
- gRPC/protobuf vs JSON/MessagePack for agent comms
- Backpressure, flow control, multiplexing

### 4. GUI & Rendering
- Tauri, egui, Iced, Bevy (Rust ecosystem)
- raylib, SDL2 for pixel-art retro aesthetic
- Window manager IPC (i3/Sway, EWMH, Wayland)
- Terminal embedding (VTE, libvterm, xterm.js)

### 5. Infrastructure (completed)
- Ollama + CUDA (Qwen3 8B, nomic-embed-text, Qwen3-VL 8B)
- OpenViking (local RAG + multimodal MCP server)
- GitNexus (code intelligence indexing)
- kTemplate (GitHub workflows, issue templates, branch protection)
- Continuous learning v2 instinct system

---

## What Has Been Built (This Session)

- **kTemplate** — GitHub repo template with sanitize-branch-name composite action, issue templates (task, feature, bug), PR template, and branch-handling workflow
- **Instinct system** — continuous-learning-v2 deployed with hooks, session-guardian, project scoping
- **Research notes** — Agentic-Orchestration-Patterns, IPC-Inter-Agent-Communication, Claude-Code-Multi-Agent-System, agent-state-management-report

---

## Issues & Bottlenecks Found

### Critical: Observer Never Runs
The session-guardian prevents the observer agent from spawning during active work (5-minute cooldown). Every analysis cycle shows:
```
session-guardian: cooldown active (last spawn 44s ago, interval 300s)
Observer cycle skipped
```
**Result**: 1,536 observations exist but **0 instincts have been generated**. The learning system is collecting data but never crystallizing it.

### Shell Working Directory Resets
Multiple `Shell cwd was reset to /home/kiriketsuki/dev/obKidian` errors across sessions. This indicates tool calls are sometimes run from unexpected directories, causing bash commands to fail or behave unexpectedly.

### Sequential Research Bottleneck
WebSearch is used sequentially for each sub-topic when the research phase could be parallelized with multiple agent spawns. A single session spends 5-6 searches exploring one topic before moving to the next.

### Edit vs Write Imbalance
19 Edit calls vs 43 Write calls suggests new files are being created more than existing files are being modified. This is expected for infrastructure setup but may indicate redundant file creation in future feature work.

---

## Things to Improve

### 1. Fix the Observer Deadlock (High Priority)
The session-guardian and observation interval are in conflict during active sessions. Options:
- Run `python3 ~/.claude/skills/continuous-learning-v2/scripts/instinct-cli.py analyze` manually at session end
- Lower the session-guardian cooldown to 60s or disable it during off-hours
- Add a post-session hook that bypasses the guardian

### 2. Parallelize Research Phase
Instead of sequential WebSearch calls on sub-topics, spawn 3-4 research agents in parallel at the start of a planning phase. Estimated 40-60% time saving for research-heavy sessions.

### 3. Persist Working Directory in Complex Sessions
Use absolute paths in all Bash commands during long sessions. Add a note to session-start or CLAUDE.md: "always use absolute paths in Bash to avoid cwd reset issues."

### 4. Crystallize Global Instincts Manually
Given the observer is blocked, these patterns should be promoted manually:
- `grep-before-edit` — always Grep before Edit to find target
- `read-before-write` — always Read a file before overwriting
- `parallel-research-agents` — use parallel sub-agents for multi-topic research
- `task-track-all-work` — create TaskCreate + TaskUpdate for every multi-step session

### 5. kTemplate — Add Missing Workflows
The kTemplate still needs:
- Version bumping workflow
- Release workflow
- Dependabot config
- Branch protection rules (as code/documentation)

### 6. OpenViking Integration
OpenViking is installed but may not be indexed with current vault content. Run `npx gitnexus analyze` and re-index in OpenViking after major content additions.

---

## Derived Instincts (Not Yet In System)

These patterns repeated enough to warrant formalization:

| Instinct | Confidence | Domain |
|----------|-----------|--------|
| Always read file before write/edit | 0.9 | workflow |
| Use WebSearch before implementing anything unfamiliar | 0.85 | research |
| Spawn parallel agents for multi-topic research | 0.7 | orchestration |
| Use absolute paths in Bash commands | 0.75 | reliability |
| Track all multi-step work with TaskCreate | 0.8 | workflow |
| Switch GitHub account before working on Aurrigo repos | 0.9 | git |
| Sync vault after significant content changes | 0.85 | vault |

---

*Authored by: Clault KiperS 4.6*
