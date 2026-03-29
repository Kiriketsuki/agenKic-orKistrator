# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

**agenKic-orKistrator** — a magic tower AI orchestrator. A native Godot 4 desktop application where AI agents appear as fantasy character classes (alchemists, scribes, archmages, wardkeepers, librarians, enchanters, apprentices) working inside a vertical tower. Each floor is a project/workspace; floors dynamically grow from hexagons to larger polygons based on workload. Users see agent activity in real time, route tasks across models (Claude, Gemini, Ollama, OpenAI, DeepSeek), and coordinate multi-agent workflows from a fantasy-themed interface.

Epics 1–3 are merged to main (Go orchestrator, terminal substrate, model gateway). Epic 4 (Magic Tower UI) is in progress.

## Stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| Desktop UI | Godot 4 + godot-xterm | Magic tower; PTY is Linux/macOS only — Windows gets display-only |
| UI↔Orchestrator | HTTP/SSE bridge | REST for commands, SSE for real-time events; Godot HTTPClient (no GDExtension needed) |
| Orchestrator | Go + gRPC | Supervisor/worker + LangGraph-style DAG |
| IPC | gRPC agent-to-agent + Redis Streams | Type-safe, durable, backpressure |
| State | Redis (Streams + Hashes) | Context window = RAM; all persistent state externalized |
| Terminal substrate | tmux | Programmatic pane control per agent |
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
| `pixel-office-ui-spec.md` | Godot 4 magic tower, floors as hex polygons, character class agents, HTTP/SSE bridge | Orchestrator HTTP/SSE API |

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
godot/                            # Godot 4 magic tower UI (separate from Go module)
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

## Design Context

### Users
Developers and AI power users who orchestrate multi-agent workflows across multiple projects simultaneously. They're watching agents work, submitting tasks, and debugging output — but the experience should feel like tending a living magical workshop, not operating a monitoring dashboard. Primary context: second-monitor ambient display that you actively interact with when routing work.

### Brand Personality
**Whimsical, deep, warm.** Charming and inviting on the surface with real systemic depth underneath. The tower should make you smile when you first see it, then reward you with layers of detail the more you look. Not taking itself too seriously, but the craft is serious.

### Aesthetic Direction

**Theme:** Fantasy magic tower — not retro, not nostalgic. A vertical spire where each floor is a project workspace, populated by fantasy character classes doing AI work. The tower grows and breathes with activity.

**Colour scheme — "Canopy Spire":** Vertical gradient from earthy emerald at the base to deep indigo at the summit. Gold-amber accents throughout. The tower's colour shifts as you scroll up — grounded nature at the bottom, arcane mystery at the top.

| Role | Palette |
|------|---------|
| Base/ground floors | `#142a20` grove → `#1a2e30` canopy |
| Mid floors | `#162228` teal transition |
| Upper/summit floors | `#1e1a35` twilight → `#2a2545` arcane |
| Accents | `#c8a84e` gold leaf, `#d4a574` amber |
| Parchment UI | `#ede0c4` scroll bg, `#2e2a18` ink |
| Terminal UI | `#0a0e14` dark, standard terminal colours |

**Provider elemental themes:**

| Provider | Element | Primary Colour |
|----------|---------|---------------|
| Claude (Anthropic) | Arcane | Warm amber `#d4a574` |
| Gemini (Google) | Prismatic | Cool crystal `#88c8f7` |
| OpenAI | Infernal | Crimson `#d06060` |
| Ollama (Local) | Forge | Ember `#f0a070` |
| DeepSeek | Abyssal | Violet `#a088e0` |

**Visual references:**
- **Hollow Knight** — dark, atmospheric, layered parallax depth. The sense of a vast world beyond what's visible.
- **Noita / Caves of Qud** — every pixel is alive and interactive. Dense simulation made visual.

**Anti-references (explicitly NOT this):**
- Corporate dashboards (no Grafana/Datadog energy)
- Retro nostalgia bait (not 80s arcade, not NES — this is fantasy, not nostalgia)
- Minimalist/flat UI (no Material Design, no Tailwind defaults — this has texture and personality)

**Emotional targets:** Wonder/awe (first impression), cozy mastery (daily use), playful delight (ongoing discovery).

### Design Principles

1. **The tower is alive** — Everything breathes, pulses, and responds to real data. Idle floors dim. Busy floors swell. Agents animate contextually. Nothing is static decoration.
2. **Showcase first, dashboard second** — Visual storytelling takes priority over information density. If something can be communicated through animation, colour, or spatial metaphor instead of a label, prefer the metaphor.
3. **Layers of detail** — Three levels everywhere: glance (floating runes, floor size), focus (spell scroll, status overlay), and power-user (raw terminal, keyboard shortcuts). Don't force detail on people who want ambiance.
4. **Fantasy vocabulary, real data** — Every UI element has a fantasy name and aesthetic (spell scrolls, quest boards, enchanted nameplates), but the data underneath is real and accurate. The theme enhances comprehension, never obscures it.
5. **Texture over flatness** — Parchment has grain. Stone has moss. Panels float and flutter. Every surface has material presence. The pixel art style is a medium, not a limitation.
