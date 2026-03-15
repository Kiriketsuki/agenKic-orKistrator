# Research Report: Anthropic's Claude Code Multi-Agent System

**Date:** 2026-03-14
**Researcher:** Research Agent (orchestration-research team)
**Task:** Deep research on Claude Code's multi-agent orchestration system

---

## Executive Summary

Anthropic has built a comprehensive multi-agent orchestration system into Claude Code featuring:
- **TeamCreate** for spawning collaborative agent teams
- **SendMessage** for inter-agent communication (direct + broadcast)
- **TaskCreate/TaskUpdate** for coordinated work assignment
- **Shared task lists** with file-locking for race condition prevention
- **Mailbox system** for asynchronous message delivery and idle notifications

The system is currently **experimental** (disabled by default, requires `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` flag) but represents a significant architectural achievement in autonomous agent coordination.

---

## 1. Core Architecture: Orchestrator-Worker Pattern

### Design Philosophy

Anthropic's multi-agent approach follows an **orchestrator-worker pattern**:

- **Lead Agent** (team lead): Analyzes requests, develops strategy, spawns subagents, coordinates work, synthesizes results
- **Worker Agents** (teammates): Execute specialized tasks in parallel, operate independently in isolated context windows, communicate via mailbox system

This pattern decouples reasoning from execution—the lead agent focuses on strategy while workers handle implementation details.

**Key Evidence:** Anthropic's multi-agent research system with Claude Opus 4 as lead + Claude Sonnet 4 workers achieved **90.2% performance improvement** over single-agent Claude Opus 4 on complex research tasks.

### Context Management Strategy

Each teammate gets its own context window (typically 200K+ tokens). This isolation enables:
- Parallel exploration without context pollution
- Specialized focus for workers
- Reduced interference between concurrent tasks
- Automatic context compaction when limits approach

**Critical Insight:** Token usage alone explains 80% of performance variance. Multi-agent systems consume ~15× more tokens than single-agent chat, but achieve superior results on complex tasks.

---

## 2. Coordination Mechanisms

### TeamCreate - Spawning Collaborative Teams

The `TeamCreate` tool initializes a team with:
- **Team name** and **description**
- **Agent type** specification for the team lead
- Automatic creation of team config at `~/.claude/teams/{team-name}.json`
- Automatic creation of shared task list at `~/.claude/tasks/{team-name}/`

Teams are **persistent** — the team and task list remain on disk after session end, enabling resumption across sessions.

### Task List System - Race-Free Work Assignment

The shared task list is the coordination backbone:

**Design Properties:**
- Located at `~/.claude/tasks/{team-name}/`
- Uses **file locking** to prevent multiple teammates claiming the same task
- Supports task dependencies via `blockedBy` and `blocks` fields
- Automatic unblocking when dependency tasks complete
- Tasks have clear **status workflow**: pending → in_progress → completed

**How Task Claiming Works:**
1. Teammates call `TaskList` to see available work
2. Teammate claims task with `TaskUpdate` (sets `owner` to its name)
3. File lock prevents race conditions during concurrent claims
4. Task status updates are atomic

**Dependency Management:**
- `blockedBy`: Lists task IDs that must complete first
- `blocks`: Lists task IDs waiting on this task
- Automatic propagation: completing a task auto-unblocks dependents

### SendMessage - Inter-Agent Communication

Two communication primitives:

**1. Directed Messages** (one-to-one):
```
SendMessage(to: "teammate-name", message: "task description")
```
- Push-based, ephemeral (lands in recipient's context, no shared record)
- Used for task assignment, status updates, requesting help

**2. Broadcast Messages** (one-to-all):
```
SendMessage(to: "*", message: "critical issue")
```
- Sends to all teammates simultaneously
- Expensive operation (scales with team size)
- Used sparingly for critical announcements

**Important:** Only text messages are visible to teammates. Team lead's text output is NOT visible to teammates—must use SendMessage for all communication.

### Mailbox & Idle Lifecycle

Each agent has a **mailbox** for asynchronous message delivery:

**Idle Lifecycle:**
1. Agent completes task, goes idle
2. System fires idle notification hook
3. Agent awaits messages from team lead or other teammates
4. Messages automatically delivered to agent's context when it wakes
5. Exit code 2 from hook prevents idle (keeps agent working)

**Key Feature:** Idle teammates CAN receive messages—they wake up and process immediately. "Idle" just means the agent finished its turn and is waiting for input.

---

## 3. Implementation Details from Anthropic's Research System

Anthropic published detailed architectural decisions from their multi-agent research system (June 2025):

### Planning & Memory

The system **saves plans to memory** because context windows (200K+ tokens) require persistent storage:
- Lead agent develops strategy, writes to memory
- Subagents reference the plan without re-deriving it
- Reduces redundant reasoning across parallel agents

### Parallel Exploration with Thinking

Subagents use **interleaved thinking** to evaluate search results:
- Reason about information quality in real-time
- Adapt search strategy based on findings
- Share relevant sources with lead agent

### Multi-Step Dynamic Search (Not Static RAG)

The system uses **dynamic search** rather than traditional retrieval:
1. Execute search query
2. Analyze results to identify gaps
3. Formulate new targeted queries
4. Iterate until sufficient information gathered

This approach outperforms static RAG because agents can judge information relevance in real-time.

### Citation Agent Pattern

A dedicated agent attributes all claims to sources:
- Ensures all findings are traceable
- Prevents hallucination-prone synthesis
- Produces auditable results

### Clear Task Specifications

Each worker needs:
- Clear objective statement
- Expected output format
- Guidance on tools/sources to use
- Explicit task boundaries

Ambiguous specifications cause agents to struggle with effort allocation and drift from objectives.

---

## 4. Claude Agent SDK Design Philosophy

Anthropic codified agent design principles in the **Claude Agent SDK**, with core guidance published in "[Building Effective Agents](https://www.anthropic.com/research/building-effective-agents)" (Dec 2024):

### Workflows vs. Agents

**Workflows** use "LLMs and tools orchestrated through predefined code paths"
- Deterministic, predictable, low latency
- Use for well-defined tasks with fixed procedures

**Agents** have "LLMs dynamically direct their own processes and tool usage"
- Flexible, model-driven decision-making
- Use for open-ended problems, research, exploration

**Key Guidance:** Start simple. "Agentic systems often trade latency and cost for better task performance"—only add agent complexity when simpler solutions provably fail.

### Production-Proven Design Patterns

1. **Prompt Chaining** — Sequential calls where each processes previous output
2. **Routing** — Classify input to specialized downstream handlers
3. **Parallelization** — Simultaneous task execution with voting/consensus
4. **Orchestrator-Workers** — Dynamic decomposition + delegation
5. **Evaluator-Optimizer** — Iterative refinement with feedback loops

### Three Core Implementation Principles

1. **Simplicity** — Avoid unnecessary complexity
2. **Transparency** — Explicitly surface agent's planning steps
3. **Tool Excellence** — Treat agent-tool interfaces with same care as human interfaces (docs, testing, clarity)

---

## 5. Community & Open-Source Ecosystem

### GitHub Feature Requests

Claude Code agent teams are still **experimental and not available on all plans**, with active feature requests for:
- **Issue #24316**: Custom `.claude/agents/` definitions for team members (enable specialization)
- **Issue #30140**: Shared channels for persistent group communication (replace ephemeral SendMessage)
- **Issue #25148**: Enable agent teams on all plans (expand access)
- **Issue #26107**: Use existing subagent definitions in teams (avoid re-prompting)

### Open-Source Reimplementations

Several projects have reimplemented Claude Code's multi-agent system:

**OpenCode** (Most Feature-Complete):
- Event-driven instead of polling-based
- Peer-to-peer messaging (not lead-centric)
- Multi-model support (Claude + Ollama + others)
- Cleaner inbox/session injection
- Single-process architecture (vs. isolated sessions)

**OpenWork**:
- Open-source alternative to Claude Code's desktop app
- Runs locally without remote server
- Supports agentic workflows

**Cline** (4M+ installs):
- VS Code extension for terminal-first agents
- Plan → Review → Run loops
- MCP tool integrations
- Permissioned file/terminal access

---

## 6. Performance Characteristics & Tradeoffs

### Performance Gains

**Multi-agent research system (Claude Opus lead + Sonnet workers):**
- +90.2% improvement over single-agent Opus
- Strongest on complex research tasks (requiring multiple perspectives)
- Consistent across diverse query types

### Token Cost Tradeoff

- Multi-agent systems use **~15× more tokens** than single-agent chat
- Token usage explains **80% of performance variance**
- Cost: $15-25 per complex task (Anthropic's code review benchmark)
- Latency: ~20 minutes for thorough multi-agent review

### Use Cases Where Multi-Agent Excels

1. **Research & Review** — Different aspects require specialized investigation
2. **New Features/Modules** — Teammates own distinct pieces without interference
3. **Competing Hypotheses** — Parallel exploration of different solutions
4. **Cross-Layer Coordination** — Frontend + backend + tests changes
5. **Code Review** — Different reviewer agents search for different error types

**Not Recommended For:**
- Simple deterministic tasks (overhead not justified)
- Real-time latency-critical applications
- Tasks with high state interdependence (hard to parallelize)

---

## 7. Current Status & Limitations

### Experimental Feature Flag

Agent teams require explicit enablement:
```json
// ~/.claude/settings.json
{
  "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": true
}
```

Or via environment variable:
```bash
export CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=true
```

### Access Restrictions

Currently **not available on all plans**—appears limited to higher-tier subscriptions. Community requests to democratize access.

### Specialization Gap

Current implementation spawns all teammates as undifferentiated general-purpose agents. Specialization only possible through natural language prompts in spawn message. Community requesting:
- Ability to use `.claude/agents/` definitions for teammates
- Tool restrictions per teammate
- Custom skill loading per agent type

### Communication Limitations

SendMessage is **push-based and ephemeral**:
- Messages land in individual context windows
- No shared persistent communication log
- Hard to debug team interactions retroactively
- Open issue #30140 requests shared channels

---

## 8. Integration with Broader Anthropic Systems

### Relationship to Claude Agent SDK

Claude Code's team system is a **practical instantiation** of Claude Agent SDK principles:
- Each teammate is an agent with isolated context
- Lead agent is the orchestrator
- Task list implements the worker-pool pattern
- SendMessage implements agent-to-agent communication

The SDK itself focuses on single-agent design; team orchestration extends it for multi-agent scenarios.

### Model Tier Recommendations

Anthropic's research system uses **differentiated model tiers**:
- **Lead Agent:** Claude Opus 4 (best reasoning for strategy)
- **Worker Agents:** Claude Sonnet 4 (fast execution, strong coding)
- **Citation/Verification:** Sonnet 4 (cost-effective verification)

This tiering balances reasoning cost (lead) with execution throughput (workers).

---

## 9. Key Findings Summary

| Finding | Impact | Evidence |
|---------|--------|----------|
| **Orchestrator-worker pattern** | Enables 90%+ performance gains on complex tasks | Anthropic's multi-agent research system |
| **Task list + file-locking** | Race-free concurrent work assignment | GitHub issue discussions, code.claude.com/docs |
| **Isolated context windows** | 80% of performance variance comes from token budget | Anthropic's performance analysis |
| **Dynamic search > Static RAG** | Better information quality for research | Multi-agent research system design |
| **Ephemeral messaging** | Fast but hard to debug; community wants persistence | Open issues #30140 |
| **Experimental status** | Limited adoption; access restrictions slow growth | GitHub feature requests |
| **OpenCode reimplementation** | Event-driven design shows architectural evolution | github.com/different-ai/openwork |
| **Clear task specs are critical** | Ambiguous tasks cause agent drift | Anthropic's architectural guidance |
| **Simple design principles** | Success isn't complexity, it's fit-for-purpose | Building Effective Agents blog |

---

## 10. Resources for Further Exploration

**Official Anthropic:**
- [Building Effective Agents](https://www.anthropic.com/research/building-effective-agents) — Core design principles
- [Claude Code Docs: Agent Teams](https://code.claude.com/docs/en/agent-teams) — Official documentation
- [Multi-Agent Research System](https://www.anthropic.com/engineering/multi-agent-research-system) — Architecture deep-dive
- [Claude Agent SDK](https://platform.claude.com/docs/en/agent-sdk/overview) — SDK design

**GitHub Resources:**
- [Anthropic Claude Code Issues](https://github.com/anthropics/claude-code) — Feature requests and discussions
- [Claude Agent SDK Demos](https://github.com/anthropics/claude-agent-sdk-demos) — Example implementations
- [OpenCode (Reimplementation)](https://github.com/different-ai/openwork) — Community alternative

**Community Guides:**
- [Claude Code Swarm Orchestration](https://gist.github.com/kieranklaassen/4f2aba89594a4aea4ad64d753984b2ea) — Comprehensive pattern guide
- [Claude Code Agent Teams Guide](https://claudefa.st/blog/guide/agents/agent-teams) — 2026 complete guide
- [DEV Community Articles](https://dev.to) — Multiple practical tutorials

---

## Recommendations for Team Lead

1. **Immediate:** Enable `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` in settings if multi-agent orchestration needed for project
2. **Planning:** Review "Building Effective Agents" (Dec 2024) for design patterns before implementing agents
3. **Architecture:** Consider orchestrator-worker pattern for tasks with natural parallelization (review, research, cross-layer changes)
4. **Tokenomics:** Budget 15× token increase vs. single-agent approaches; multi-agent justified for complex tasks only
5. **Specialization:** Watch GitHub issues #24316 and #26107 for agent customization features
6. **Debugging:** Recognize SendMessage limitation (ephemeral); consider logging patterns for audit trails

---

*Report compiled with research from: Anthropic blog/engineering posts, GitHub discussions, community implementations, and official Claude Code documentation.*
