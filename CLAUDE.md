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
2. Label hierarchy: `epic` → `feature` (sub-issue of epic) → `task` (sub-issue of feature/epic) → `bug` / `hotfix` (always from main)
3. Sub-issues automatically branch from their parent's branch

### Branch naming (auto-generated)
```
epic/{n}-kebab-title
feature/{n}-kebab-title
task/{n}-kebab-title
bug/{n}-kebab-title
hotfix/{n}-kebab-title
```

### Versioning — `YY.Major.Minor.Patch[Suffix]`
- `epic/*` merged → main: **Major +1**, Minor → 0, Patch → 0
- `feature/*` merged → epic or main: **Minor +1**, Patch → 0
- `task/*` or `bug/*` merged → parent or main: **Patch +1**
- `hotfix/*` merged → main: appends suffix letter (a, b … z, aa …)
- Year rollover: run **Manual Version Bump** → `year-rollover` from Actions → `workflow_dispatch`

### PR titles (auto-generated, conventional commits)
| Label | PR Title Prefix |
|-------|----------------|
| epic | `epic:` |
| feature | `feat:` |
| task | `chore:` |
| bug | `fix:` |
| hotfix | `hotfix:` |

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
- `manual-version-bump.yml` — manual bumps and year rollover via `workflow_dispatch`

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
