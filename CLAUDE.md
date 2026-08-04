# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

**OrKi / agenKic-orKistrator** — a pixelated AI office orchestrator. Agents are summoned from a Godot 4 pixel-art UI and appear as characters on the floors of a fisheye-projected tower. Users watch agent activity live, submit tasks and small DAGs, and drive real CLI coding agents from a retro-aesthetic interface.

This is a **working two-process desktop app**, not a research repo: a Go orchestrator (`cmd/orchestrator`) plus a Godot 4 client (`godot/`), launched together by `./run.sh`. Roughly 120 `.go` files (53 `_test.go`) and 82 `.gd` files. `VERSION` is `26.4.0.0`.

### What actually runs

- The Godot client talks to the orchestrator over **HTTP + SSE on `127.0.0.1:8081`** (`internal/httpbridge`). Agents talk to the orchestrator over **gRPC on `:50051`** (`internal/ipc`). These are two separate planes — do not conflate them.
- Spawnable agent kinds are exactly five: `sim`, `claude`, `codex`, `opencode`, `pi` (`internal/httpbridge/handlers.go:719-729`). `sim` is an in-process fake worker (`internal/simagent`); the other four exec the real CLI binary inside a tmux session named `agent-<id>`, type the prompt into the pane, and poll `CaptureOutput` until two identical snapshots mean the CLI settled (`internal/cliagent/cliagent.go:117-279`).
- Gemini, OpenAI, Ollama and DeepSeek appear **only** as cosmetic glyph/colour dictionaries in Godot scripts and in the Reassign submenu (`godot/scripts/panels/terminal_view.gd:32-39`, `godot/scripts/overlays/agent_context_menu.gd:27`). They are not spawnable.
- UI surfaces: Grimoire spawn flyout (F3), Power flyout (F4), Panels flyout (F5), quest board (task + small DAG submission), spell scroll (parchment output view), terminal view, minimap, hex compass, floor tabs, right-click agent context menu.

## Real Stack (as built)

| Layer | Technology | Status |
|-------|-----------|--------|
| Desktop UI | Godot 4 project in `godot/` | **built** |
| Terminal widget | godot-xterm | **not committed** — install via `godot/addons/install_godot_xterm.sh`, see [godot/addons/VENDOR.md](godot/addons/VENDOR.md); PTY is Linux/macOS only |
| Orchestrator | Go 1.26.1, `cmd/orchestrator` | **built** |
| Agent ↔ orchestrator transport | gRPC on `:50051` (`GRPC_ADDR`), `proto/orchestrator.proto` + `internal/ipc` | **built** |
| Godot ↔ orchestrator transport | HTTP + SSE on `127.0.0.1:8081` (`BRIDGE_ADDR`), `internal/httpbridge` | **built** |
| Health endpoint | HTTP on `:8080` (`HEALTH_ADDR`), `internal/health` | **built** |
| State | `state.NewMockStore()` — in-memory only (`cmd/orchestrator/main.go:47`) | **built** |
| Redis store | `internal/state/redis.go` (~503 lines, fully tested) | **built but unwired** — zero non-test call sites |
| Terminal substrate | tmux only (`internal/terminal/tmux.go`), via `exec.LookPath("tmux")` | **built** |
| WezTerm substrate | — | **not built** — no WezTerm code exists anywhere in the repo |
| Model gateway | `internal/gateway`: LiteLLM client + JudgeRouter + provider adapters, `config/models.yaml` | **built but unwired** — imported by no `cmd/` binary; the running orchestrator makes no LLM completion calls |
| Supervisor / DAG | `internal/supervisor`, `internal/dag` | **built** |

Bridge auth is opt-in: with `BRIDGE_API_KEY` set the bridge requires a bearer token; unset, it logs a warning and runs unauthenticated (`cmd/orchestrator/main.go:131-136`, `internal/httpbridge/bridge.go:94-96`).

### HTTP bridge routes

`internal/httpbridge/bridge.go:178-192`:

```
GET  /api/agents
GET  /api/agents/{id}/output
GET  /api/floors
GET  /api/providers
POST /api/tasks
POST /api/dags
POST /api/agents/{id}/input
POST /api/agents/{id}/cancel
POST /api/agents/{id}/reassign
POST /api/agents/{id}/despawn
POST /api/agents/spawn
POST /api/admin/restart
GET  /events/stream          # SSE
```

## Build, Run, Test

```bash
./setup.sh                 # toolchain + deps; also starts a redis:7-alpine container
make generate              # protoc -> gen/pb/orchestrator (needs $(go env GOPATH)/bin on PATH)
make build                 # bin/orchestrator from ./cmd/orchestrator
make test                  # go test -race -count=1 ./internal/...
make test-integration      # same, -tags=integration (needs REDIS_URL)
make lint                  # golangci-lint run ./...
make clean                 # rm -rf bin/ gen/pb/
go vet ./...               # CI runs this between generate and test
```

`./run.sh` is the normal entrypoint: it rebuilds only when a `.go` source or `go.mod` is newer than `bin/orchestrator`, starts the orchestrator, polls the bridge address for up to 10s, then launches `godot --path godot`. Ctrl-C kills Godot and sends SIGTERM to the orchestrator for the graceful shutdown path.

**Third test tier — not automated.** Files tagged `//go:build testenv` live in `internal/dag`, `internal/supervisor` and `e2e/`. No Makefile target and no CI step runs them. Manual invocation:

```bash
go test -tags=testenv ./internal/... ./e2e/...
```

**Godot tests** are standalone headless GDScript files under `godot/tests/` (29 files). No GUT or other Godot test runner is vendored:

```bash
godot --headless --path godot --script tests/wander_math_test.gd
```

### Environment variables

| Var | Default | Effect |
|-----|---------|--------|
| `GRPC_ADDR` | `:50051` | gRPC listen address (agents) |
| `BRIDGE_ADDR` | `127.0.0.1:8081` | HTTP+SSE bridge listen address (Godot) |
| `HEALTH_ADDR` | `:8080` | health HTTP listen address |
| `BRIDGE_API_KEY` | unset | when set, bridge requires bearer auth; unset = no auth |
| `MIN_AGENT_COUNT` | `1` | health aggregator minimum-agents threshold |
| `REDIS_URL` | unset | consumed only by `make test-integration` |

## Load-bearing constraints an agent must know

1. **State is in-memory.** `cmd/orchestrator/main.go:47` hardcodes `state.NewMockStore()`. `internal/state/redis.go` is complete and tested but has zero non-test call sites — any Redis work is dead code until someone wires the constructor into `main.go`. `internal/health` reports a field named `RedisPingOK` that in practice pings the MockStore.
2. **Redis is still started around the build.** `setup.sh`, `docker-compose.yml` and CI all start `redis:7-alpine`, and `make test-integration` needs it. The orchestrator binary itself never connects to it.
3. **`internal/gateway` is not in the running system.** `grep 'gateway\.' cmd/` returns nothing. The JudgeRouter (Haiku classifies into cheap/mid/frontier) and the LiteLLM client (default `http://localhost:8000/chat/completions`) are exercised only by their own tests. No cost-based routing happens at runtime.
4. **`supervisor.ShutdownHandler` has zero production callers.** `cmd/orchestrator/main.go:189-234` does its own signal handling and calls `gracefulShutdown` directly. Don't assume the handler is on the shutdown path.
5. **Adding a spawn kind touches three places**: `providerRoster` (`internal/httpbridge/handlers.go:674-697`), the kind switch (`:719-729`), and `cliBinaries` (`internal/cliagent/cliagent.go:79-84`). Miss one and the kind is either invisible, rejected, or unlaunchable.
6. **The Godot UI reads `/api/providers`**, so a new adapter added to `providerRoster` appears in the Grimoire flyout with no GUI change.
7. **Proto changes require `make generate`** with `$(go env GOPATH)/bin` on PATH (that's where `protoc-gen-go` and `protoc-gen-go-grpc` land). `gen/` is generated and gitignored — never commit it, never hand-edit it.
8. **Quest-board priority is a min-heap** (`DequeueTask` == `ZPOPMIN`). "High" maps to a LOW number: `PRIORITY_HIGH = 1.0`, `NORMAL = 5.0`, `LOW = 10.0` (`godot/scripts/panels/quest_board_view.gd:14-23`). Inverting this makes high-priority quests run last.
9. **Floor 0 is reserved** and never accepts a spawn — the bridge rejects it with 400 (`internal/httpbridge/handlers.go:760-796`). Omitting a floor makes the bridge pick the lowest non-full floor ≥ 1.
10. **Task cancel bypasses the agent state machine** (`internal/httpbridge/handlers.go:297-401`). Read that path before changing agent state transitions.
11. **Reassign is a hint, not a migration.** It persists `TaskMeta.Provider` and re-enqueues at the original priority, but the supervisor assign loop never reads that field (`internal/httpbridge/handlers.go:403-539`). It is requeue-with-a-hint.
12. **No tmux = degraded mode.** If `tmux` is absent, `main.go:55-60` logs `terminal substrate unavailable, running headless`; `cliagent` falls back to one-shot exec (`claude -p …`, `codex exec …`, `opencode run …`, `pi -p …`) and `/output` and `/input` return 501.

## Real Module Map

```
cmd/orchestrator/       # the supervisor binary — gRPC + bridge + health servers
cmd/demoworker/         # demo gRPC worker agent
cmd/spritegen/          # asset dev tool: sprite generation
cmd/assetslice/         # asset dev tool: spritesheet slicing

internal/agent/         # agent state machine, worker lifecycle
internal/cliagent/      # spawns real CLI agents (claude/codex/opencode/pi) into tmux panes
internal/dag/           # graph + cycle detection, Kahn levels, level-by-level executor, StatusTracker
internal/gateway/       # LiteLLM client + JudgeRouter (unwired)
internal/gateway/providers/  # per-provider request/response adapters
internal/health/        # health aggregator behind the :8080 HTTP endpoint and gRPC health service
internal/httpbridge/    # HTTP + SSE bridge — the Godot-facing transport
internal/ipc/           # gRPC server, handlers, health HTTP server
internal/simagent/      # in-process simulated worker ("sim" kind)
internal/state/         # Store interface, MockStore (used), RedisStore (unwired)
internal/supervisor/    # registration, heartbeats, crash recovery, restart policy, reapers
internal/terminal/      # Substrate interface + the single tmux implementation

proto/orchestrator.proto  # gRPC service + message definitions
config/models.yaml        # gateway tier definitions and fallback chains (unwired)
e2e/                      # testenv-tagged end-to-end tests, not run by CI
gen/                      # generated protobuf — gitignored

godot/scripts/    # GDScript: tower, panels, overlays, agent characters, bridge client
godot/scenes/     # .tscn scene files
godot/assets/     # sprites, spritesheets, fonts
godot/shaders/    # fisheye projection and visual effects
godot/themes/     # UI themes
godot/config/     # client-side config resources
godot/tests/      # standalone headless GDScript tests (no runner vendored)
godot/addons/     # godot-xterm install script + VENDOR.md; the addon itself is not committed
```

Behavioural notes worth knowing before touching these:

- **Supervisor** (`internal/supervisor/supervisor.go`, ~769 lines): registration, heartbeat staleness detection, `crashAgent` recovery, per-agent mutexes, task-assign loop. Restart policy is exponential backoff (base 1s, max 30s) with a sliding-window circuit breaker (60s window, 5 crashes) — `internal/supervisor/restart.go:54-144`. A tombstone reaper lives in `completion.go` and a separate orphan-tmux-session reaper (15s) in `reaper.go`.
- **DAG**: `main.go:73-74` wires `BlockingSubmitter` (blocks on `CompletionRegistry.Wait`), not the deprecated `StoreSubmitter`. The executor runs level-by-level with fail-fast.

## Feature Specs

The four headline implementation blueprints live in **`specs/`**, not at repo root. Each carries must-have/should-have scope, API contracts, Gherkin acceptance scenarios, task breakdowns, and exit criteria.

| Spec file | Component |
|-----------|-----------|
| [specs/go-orchestrator-core-spec.md](specs/go-orchestrator-core-spec.md) | Supervisor process, agent state machine, DAG engine |
| [specs/terminal-substrate-spec.md](specs/terminal-substrate-spec.md) | tmux session management, command injection, output capture |
| [specs/model-gateway-spec.md](specs/model-gateway-spec.md) | LiteLLM proxy, Judge-Router, provider adapters, cost tracking (built, unwired) |
| [specs/pixel-office-ui-spec.md](specs/pixel-office-ui-spec.md) | Godot 4 pixel-art office, godot-xterm, agent sprites, panel layout |

`specs/` holds 16 spec files in total; `docs/specs/todo/` holds 6 more not yet started. The loose `.md` files at repo root (council reports and older specs) are historical and out of scope — treat `specs/` as current.

Godot-side design docs: [godot/PORTING.md](godot/PORTING.md), [godot/PARITY.md](godot/PARITY.md).

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

Version bumps fire only on **merged** PRs into `main` or `epic/**`. The version regex is
`^([0-9]{2})\.([0-9]+)\.([0-9]+)\.([0-9]+)([a-zA-Z]*)$` (`version-bump.yml:69-80`,
`version-validation.yml:32-36`). Year rollover is handled automatically inside
`version-bump.yml:82-87` — the manual `year-rollover` dispatch is a fallback, not the
normal path.

> [.github/CI-CD-Guide.md](.github/CI-CD-Guide.md) and `README.md` both still describe a
> stale `YY.MM.Major.Minor` scheme. **This file is authoritative** for versioning.
> The README also carries a `go-1.23+` badge (`go.mod` declares `go 1.26.1`) and links the
> four feature specs at repo root, where they no longer are.

## CI/CD

Workflows in `.github/workflows/`:
- `ci.yml` — the real build/test gate
- `issue-branch-handler.yml` — auto-creates branch + draft PR when an issue is labelled
- `version-bump.yml` / `version-validation.yml` — automated versioning on merge
- `release.yml` — release packaging
- `manual-version-bump.yml` — manual bumps and year rollover via `workflow_dispatch`

`ci.yml` triggers on push to `main`, `epic/**`, `feature/**` and on PRs into `main`, `epic/**`.
It runs a `redis:7-alpine` service container, installs protoc plus the Go protoc plugins, then:

```
make generate  →  go vet ./...  →  make test  →  make test-integration  (REDIS_URL=redis://localhost:6379)
```

Not in CI: the `testenv`-tagged suite (`internal/dag`, `internal/supervisor`, `e2e/`) and the
headless Godot tests. Both must be run by hand.

## Architecture reference

[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — process topology, the full bridge endpoint
list, supervisor model, DAG engine, terminal substrate, Godot client structure, and a
planned-vs-built table. Read it before making structural changes.

## Research (historical)

`docs/research/` holds the pre-build research (2026-03-14, 11-agent parallel research team +
adversarial council verification). It records **intent at the time**, not the shipped system.

- **Start here**: [docs/research/Agentic-Orchestrator-MOC.md](docs/research/Agentic-Orchestrator-MOC.md) — architectural decision table and competitive landscape
- `docs/research/patterns/` — refined reference versions of core patterns

**Research says X, code does Y** — check the code before citing any of these:

| Research claims | The code does |
|---|---|
| Redis Streams + Hashes as the state layer | `main.go:47` uses `state.NewMockStore()`; `RedisStore` has zero non-test callers |
| LiteLLM + Judge-Router routing live traffic for 60–90% savings | `internal/gateway` is imported by no `cmd/` binary; no LLM calls at runtime |
| tmux **or** WezTerm Lua as the substrate | tmux only; there is no WezTerm code in the repo |
| gRPC as the UI transport | gRPC is the agent plane; the Godot client uses HTTP + SSE on `127.0.0.1:8081` |

Other caveats from council verification:
- godot-xterm PTY is Linux/macOS only — Windows shows terminal display but no live shell
- DeepSeek V3.2 $0.03/1M pricing applies to cached input only
- Pixel Agents star count has a discrepancy (4.4k vs 2.8k) in notes — verify before citing

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **agenKic-orKistrator** (3058 symbols, 7032 relationships, 52 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

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

## Keeping the Index Fresh

After committing code changes, the GitNexus index becomes stale. Re-run analyze to update it:

```bash
npx gitnexus analyze
```

If the index previously included embeddings, preserve them by adding `--embeddings`:

```bash
npx gitnexus analyze --embeddings
```

To check whether embeddings exist, inspect `.gitnexus/meta.json` — the `stats.embeddings` field shows the count (0 means no embeddings). **Running analyze without `--embeddings` will delete any previously generated embeddings.**

> Claude Code users: A PostToolUse hook handles this automatically after `git commit` and `git merge`.

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
