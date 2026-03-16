# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

**agenKic-ochKistrator** — a pixelated AI office orchestrator. A native desktop application where AI agents appear as pixel-art workers at desks. Users see agent activity in real time, route tasks across models (Claude, Gemini, Ollama), and coordinate multi-agent workflows from a retro-aesthetic interface.

This repo is currently in the **research and spike phase**. No application code exists yet — the codebase will be built out from the decisions documented in `docs/research/`.

## Planned Stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| Desktop UI | Godot 4 + godot-xterm | Pixel-art office; PTY is Linux/macOS only — Windows gets display-only |
| Orchestrator | Go + gRPC | Supervisor/worker + LangGraph-style DAG |
| IPC | gRPC agent-to-agent + Redis Streams | Type-safe, durable, backpressure |
| State | Redis (Streams + Hashes) | Context window = RAM; all persistent state externalized |
| Terminal substrate | tmux or WezTerm Lua | Programmatic pane control per agent |
| TUI (alt) | Bubbletea + Lip Gloss (Go) | Elm architecture, no render/state divergence |
| Model gateway | LiteLLM + Judge-Router | Haiku judges → Sonnet works → Opus architects; 60–90% cost savings |

Read `docs/research/Agentic-Orchestrator-MOC.md` for the full architectural decision table and competitive landscape rationale before touching any design.

## Git Workflow

All work goes through issues. **Never push directly to `main`.**

### Starting work
1. Create an issue with the appropriate label — the `issue-branch-handler` workflow auto-creates a branch and draft PR
2. Label hierarchy: `task` (top-level) → `feature` (sub-issue of task) → `bug` (sub-issue of feature/task)
3. Sub-issues automatically branch from their parent's branch

### Branch naming (auto-generated)
```
task/{n}-kebab-title
feature/{n}-kebab-title
bug/{n}-kebab-title
```

### Versioning — `YY.MM.Major.Minor`
- `task/*` or `feature/*` merged → main: **Major +1**, Minor → 0
- `bug/*` or `hotfix/*` merged → main: **Minor +1**
- Direct push with `hotfix:` in commit message: appends suffix (a, b … z, A …)
- Monthly rollover: run **Manual Monthly Version Bump** from Actions → `workflow_dispatch`

### PR titles
| Type | Format |
|------|--------|
| Feature | `Adding [Feature]: Name` |
| Task | `Implementing [Task]: Name` |
| Bug | `Fixing [Bug]: Name` |

All PRs are squash-merged.

## Feature Specs

Four feature spec files live at repo root — these are the implementation blueprints, each with must-have/should-have scope, gRPC API contracts, acceptance scenarios (Gherkin), task breakdowns, and exit criteria:

| Spec file | Component | Key dependencies |
|-----------|-----------|-----------------|
| `go-orchestrator-core-spec.md` | Supervisor process, agent state machine, DAG engine | Redis, gRPC |
| `terminal-substrate-spec.md` | tmux session management, command injection, output capture | tmux binary, `os/exec` |
| `model-gateway-spec.md` | LiteLLM proxy, Judge-Router, provider adapters, cost tracking | LiteLLM sidecar |
| `pixel-office-ui-spec.md` | Godot 4 pixel-art office, godot-xterm, agent sprites, panel layout | Orchestrator gRPC API |

## Planned Go Module Layout

From the specs (no code written yet):

```
cmd/orchestrator/main.go          # supervisor entrypoint
internal/supervisor/              # supervision tree, restart strategies, health probes
internal/agent/                   # agent state machine, worker lifecycle
internal/dag/                     # task DAG engine, topological execution
internal/ipc/                     # gRPC server + service definitions
internal/state/                   # Redis client (Streams, Hashes, Sorted Sets)
internal/gateway/                 # Gateway interface, judge-router, LiteLLM client
internal/gateway/providers/       # per-provider format adapters
internal/terminal/                # Substrate interface, tmux.go, wezterm.go
proto/orchestrator.proto          # gRPC service + message definitions
config/models.yaml                # tier definitions, model assignments, fallback chains
godot/                            # Godot 4 project (separate from Go module)
```

## CI/CD

Workflows in `.github/workflows/`:
- `issue-branch-handler.yml` — auto-creates branch + draft PR when an issue is labelled
- `version-bump.yml` / `version-validation.yml` — automated versioning on merge
- `release.yml` — release packaging
- `manual-version-bump.yml` — monthly rollover via `workflow_dispatch`

No build/test commands exist yet — this section should be updated when the Go module is scaffolded.

## Research

`docs/research/` contains all pre-build research (2026-03-14, 11-agent parallel research team + adversarial council verification).

- **Start here**: `docs/research/Agentic-Orchestrator-MOC.md` — links every finding, key decisions, and next spikes
- `docs/research/` — raw session notes (IPC, state management, process supervision, competitive landscape, terminal infra, pixel UI eval, council fact-checks)
- `docs/research/patterns/` — refined reference versions of core patterns from Digital Garden

Key caveats from council verification (read before citing numbers):
- godot-xterm PTY is Linux/macOS only — Windows shows terminal display but no live shell
- DeepSeek V3.2 $0.03/1M pricing applies to cached input only
- Pixel Agents star count has a discrepancy (4.4k vs 2.8k) in notes — verify before citing

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **agenKic-orKistrator** (736 symbols, 1645 relationships, 33 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## When Debugging

1. `gitnexus_query({query: "<error or symptom>"})` — find execution flows related to the issue
2. `gitnexus_context({name: "<suspect function>"})` — see all callers, callees, and process participation
3. `READ gitnexus://repo/agenKic-orKistrator/process/{processName}` — trace the full execution flow step by step
4. For regressions: `gitnexus_detect_changes({scope: "compare", base_ref: "main"})` — see what your branch changed

## When Refactoring

- **Renaming**: MUST use `gitnexus_rename({symbol_name: "old", new_name: "new", dry_run: true})` first. Review the preview — graph edits are safe, text_search edits need manual review. Then run with `dry_run: false`.
- **Extracting/Splitting**: MUST run `gitnexus_context({name: "target"})` to see all incoming/outgoing refs, then `gitnexus_impact({target: "target", direction: "upstream"})` to find all external callers before moving code.
- After any refactor: run `gitnexus_detect_changes({scope: "all"})` to verify only expected files changed.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Tools Quick Reference

| Tool | When to use | Command |
|------|-------------|---------|
| `query` | Find code by concept | `gitnexus_query({query: "auth validation"})` |
| `context` | 360-degree view of one symbol | `gitnexus_context({name: "validateUser"})` |
| `impact` | Blast radius before editing | `gitnexus_impact({target: "X", direction: "upstream"})` |
| `detect_changes` | Pre-commit scope check | `gitnexus_detect_changes({scope: "staged"})` |
| `rename` | Safe multi-file rename | `gitnexus_rename({symbol_name: "old", new_name: "new", dry_run: true})` |
| `cypher` | Custom graph queries | `gitnexus_cypher({query: "MATCH ..."})` |

## Impact Risk Levels

| Depth | Meaning | Action |
|-------|---------|--------|
| d=1 | WILL BREAK — direct callers/importers | MUST update these |
| d=2 | LIKELY AFFECTED — indirect deps | Should test |
| d=3 | MAY NEED TESTING — transitive | Test if critical path |

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/agenKic-orKistrator/context` | Codebase overview, check index freshness |
| `gitnexus://repo/agenKic-orKistrator/clusters` | All functional areas |
| `gitnexus://repo/agenKic-orKistrator/processes` | All execution flows |
| `gitnexus://repo/agenKic-orKistrator/process/{name}` | Step-by-step execution trace |

## Self-Check Before Finishing

Before completing any code modification task, verify:
1. `gitnexus_impact` was run for all modified symbols
2. No HIGH/CRITICAL risk warnings were ignored
3. `gitnexus_detect_changes()` confirms changes match expected scope
4. All d=1 (WILL BREAK) dependents were updated

## CLI

- Re-index: `npx gitnexus analyze`
- Check freshness: `npx gitnexus status`
- Generate docs: `npx gitnexus wiki`

<!-- gitnexus:end -->
