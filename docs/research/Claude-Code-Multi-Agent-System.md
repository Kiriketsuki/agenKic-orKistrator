---
title: Anthropic Claude Code Multi-Agent System — Internals
tags: [research, orchestration, claude-code, anthropic, multi-agent, teamcreate]
date: 2026-03-14
type: research
---

# Anthropic Claude Code Multi-Agent System

## Overview

Claude Code implements a **lead-agent + worker-agent pattern** where a lead develops strategy and spawns subagents for parallel work. The system uses file-based task lists, a mailbox for inter-agent messaging, and tmux panes as the process substrate for each agent.

## Architecture

```
Team Lead (orchestrator)
├── Task List (~/.claude/tasks/{team-name}/)   ← shared, file-locked
├── Team Config (~/.claude/teams/{team-name}/config.json)  ← member registry
└── Worker Agents (each in own tmux pane)
    ├── Reads task list → claims tasks via file lock
    ├── Does work (bash, read, write, web fetch...)
    └── Reports via SendMessage → team lead mailbox
```

## Coordination Primitives

### TeamCreate
- Creates team config at `~/.claude/teams/{team-name}/config.json`
- Creates task list directory at `~/.claude/tasks/{team-name}/`
- Each member entry: `{ name, agentId, agentType }`

### TaskCreate / TaskList / TaskUpdate
- Tasks stored as files in `~/.claude/tasks/{team-name}/`
- **File-locking** prevents race conditions when multiple agents claim tasks
- Tasks support `blockedBy`/`blocks` dependencies, automatic unblocking
- Status: `pending → in_progress → completed`

### SendMessage
- **Directed (one-to-one)**: direct message to a named teammate
- **Broadcast (one-to-all)**: sends to every teammate (expensive — use sparingly)
- **Ephemeral**: not persisted. If agent is offline when message arrives, it queues in mailbox
- Team lead's text output is NOT visible to teammates — must use SendMessage explicitly

### Idle/Active Lifecycle
- Agents go idle after each turn (normal behavior — NOT an error)
- Idle agents can receive messages; SendMessage wakes them
- System auto-sends idle notifications to team lead when an agent's turn ends

## Performance Data

From Anthropic's own multi-agent research system:
- **90.2% performance improvement** over single-agent Claude Opus using lead (Opus) + workers (Sonnet)
- **Token usage explains 80% of performance variance** in multi-agent systems
- Multi-agent consumes ~15× more tokens than single-agent chat
- Cost: ~$15-25 per complex research task, ~20 min latency

## Design Philosophy

Anthropic's Dec 2024 "Building Effective Agents" post distinguishes:

| | Workflows | Agents |
|--|-----------|--------|
| Control flow | Predefined code paths | LLM-directed |
| Determinism | High | Low |
| Latency | Low | Higher |
| Use when | Well-defined tasks | Open-ended problems |

**Guidance: Start simple. Only add agent complexity when provably needed.**

## Access & Status (2026)

- Agent teams require `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` flag
- Not available on all plans
- Active community requests: custom agent specialization, multi-model support, persistent shared channels

## Open-Source Reimplementations

The community has reimplemented the pattern with improvements:
- **OpenCode / OpenWork**: event-driven (vs polling), peer-to-peer messaging, multi-model support
- Shows the model is valid but tradeoffs are debated

## Gaps in Current System

1. Messages are ephemeral — no persistent shared channel
2. Single-model only (all agents use same Claude version)
3. Pane limit constrains parallelism
4. No built-in capability routing (all agents are generalists)

## References

- [Building Effective Agents](https://www.anthropic.com/research/building-effective-agents) — Anthropic, Dec 2024
- [Claude Code Agent Teams Docs](https://code.claude.com/docs/en/agent-teams)
- [Multi-Agent Research System](https://www.anthropic.com/engineering/multi-agent-research-system) — 90.2% improvement study
- [Claude Agent SDK](https://platform.claude.com/docs/en/agent-sdk/overview)

---

*Authored by: Clault KiperS 4.6*
