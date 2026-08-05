# AGENTS.md

Operating brief for any coding agent working in this repository. Everything below
describes what is actually built and running, not what the specs propose.

## Overview

OrKi (`agenKic-orKistrator`) is a working two-process desktop application:

- a **Go orchestrator** (`cmd/orchestrator`) that supervises agents, runs a task DAG,
  and drives tmux sessions
- a **Godot 4 pixel-art client** (`godot/`) that renders those agents as characters on
  the floors of a fisheye-projected tower

The user summons an agent from the UI; it appears on a floor and starts working.
Floor 0 is reserved and never accepts a spawn. Exactly five spawn kinds exist:

| Kind | What it is |
|------|-----------|
| `sim` | In-process fake worker (`internal/simagent`) — no external binary |
| `claude` | Real `claude` CLI |
| `codex` | Real `codex` CLI |
| `opencode` | Real `opencode` CLI |
| `pi` | Real `pi` CLI |

The four CLI kinds launch the real binary inside a tmux session named `agent-<id>`,
type the task prompt into that pane, and poll captured output until two identical
snapshots indicate the CLI has settled (`internal/cliagent/cliagent.go`).

Gemini, OpenAI, Ollama and DeepSeek appear only as cosmetic glyph/colour entries in
Godot scripts and the reassign submenu. They are **not** spawnable.

UI surfaces: Grimoire spawn flyout (F3), Power flyout (F4), Panels flyout (F5), quest
board (task and small-DAG submission), spell scroll (parchment output view), terminal
view, minimap, hex compass, floor tabs, and a right-click agent context menu.

## Stack as built

| Layer | Technology | Status |
|-------|-----------|--------|
| Desktop UI | Godot 4, GDScript (`godot/`) | Built |
| UI transport | HTTP + Server-Sent Events on `127.0.0.1:8081` (`internal/httpbridge`) | Built |
| Agent transport | gRPC on `:50051` (`proto/orchestrator.proto`, `internal/ipc`) | Built |
| Supervision | Supervisor with heartbeats, crash recovery, exponential backoff + circuit breaker | Built |
| Task orchestration | DAG with cycle detection, Kahn topological levels, fail-fast executor | Built |
| Terminal substrate | tmux only (`internal/terminal`) | Built |
| State | In-memory `state.MockStore` | Built and wired |
| State (durable) | `state.RedisStore` (`internal/state/redis.go`) | Built, **not wired** — zero non-test call sites |
| Model gateway | LiteLLM client + Judge-Router (`internal/gateway`) | Built and tested, **not wired** — no `cmd/` binary imports it |
| Live PTY terminal | `godot-xterm` addon | **Not committed** — install separately (see Platform support) |
| WezTerm substrate | — | **Not built.** No WezTerm code exists anywhere |
| TUI (Bubbletea/Lip Gloss) | — | **Not built** |

The running binary therefore makes **no LLM completion calls** and does **no cost-based
model routing**. All model work happens inside the external CLI agents.

## Commands

Go toolchain is `go 1.26.1` (`go.mod`). Any README badge claiming an older minimum is stale.

```bash
./setup.sh                      # one-time: toolchain, godot-xterm addon, protoc gen, build, start Redis
./run.sh                        # build if stale, start orchestrator, wait for the bridge, launch Godot

make generate                   # protoc -> gen/pb/orchestrator (needs protoc + Go plugins on PATH)
make build                      # bin/orchestrator from ./cmd/orchestrator
make test                       # go test -race -count=1 ./internal/...
make test-integration           # same, with -tags=integration (needs Redis)
make lint                       # golangci-lint run ./...
make clean                      # rm -rf bin/ gen/pb/
go vet ./...

# Third test tier, build-tagged `testenv`. Nothing committed runs it — no Makefile
# target and no CI step. Run it manually:
go test -tags=testenv ./internal/... ./e2e/...

# Godot tests are standalone headless scripts (no GUT vendored), run one at a time:
godot --headless --path godot --script tests/<name>.gd
```

Do **not** start the orchestrator or Godot without being asked — the user may already
have the app running.

### Environment variables and default ports

| Variable | Default | Purpose |
|----------|---------|---------|
| `GRPC_ADDR` | `:50051` | gRPC control plane (agents dial this) |
| `BRIDGE_ADDR` | `127.0.0.1:8081` | HTTP + SSE bridge (the Godot UI talks to this) |
| `HEALTH_ADDR` | `:8080` | HTTP health server |
| `BRIDGE_API_KEY` | unset | When set, all bridge endpoints require `Authorization: Bearer <key>`. When unset the orchestrator logs a warning and serves unauthenticated |
| `MIN_AGENT_COUNT` | `1` | Minimum agent count used by the health aggregator |
| `REDIS_URL` | — | Read by integration tests only, not by the orchestrator |

## Architecture

### Two servers, two transports

`cmd/orchestrator/main.go` starts both:

1. **gRPC on `:50051`** — the agent-to-orchestrator control plane only. Defined in
   `proto/orchestrator.proto`, served by `internal/ipc`. Dialled by `internal/cliagent`
   and `internal/simagent` (sim agents loop back to the orchestrator's own address so
   they exercise the identical API).
2. **HTTP + SSE on `127.0.0.1:8081`** — `internal/httpbridge`. This, not gRPC, is the
   Godot-facing transport.

A separate HTTP health server runs on `:8080`.

### Bridge routes (`internal/httpbridge/bridge.go`)

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
GET  /events/stream          (SSE)
```

### Packages

`cmd/`:

| Binary | Purpose |
|--------|---------|
| `orchestrator` | The supervisor process — gRPC + HTTP/SSE bridge + health |
| `demoworker` | Standalone demo gRPC worker |
| `spritegen` | Asset development tool |
| `assetslice` | Asset development tool |

`internal/` — 11 packages plus `internal/gateway/providers`:

| Package | Purpose |
|---------|---------|
| `agent` | Agent state machine and transitions |
| `cliagent` | Spawns and drives real CLI binaries (claude/codex/opencode/pi) |
| `dag` | Graph + cycle detection, topological levels, executor, status tracker |
| `gateway` | LiteLLM client and Judge-Router (unwired — see gotchas) |
| `gateway/providers` | Per-provider request/response adapters (anthropic, openai, ollama) |
| `health` | Health aggregator feeding both the gRPC and HTTP health surfaces |
| `httpbridge` | HTTP + SSE bridge for the Godot UI |
| `ipc` | gRPC server, handlers, health HTTP server |
| `simagent` | In-process simulated worker |
| `state` | `StateStore` interface, `MockStore` (wired), `RedisStore` (unwired) |
| `supervisor` | Registration, heartbeat staleness, crash recovery, restart policy, reapers |
| `terminal` | `Substrate` interface with a single tmux implementation |

### Supervisor, DAG, terminal

- **Supervisor** (`internal/supervisor/supervisor.go`): registration, heartbeat
  staleness detection, `crashAgent` recovery, per-agent mutexes, task-assign loop.
- **Restart policy** (`restart.go`): exponential backoff, base 1s / max 30s, with a
  sliding-window circuit breaker (window 60s, threshold 5 crashes).
- **Reapers**: a tombstone reaper in `completion.go`; a separate orphan-tmux-session
  reaper on a 15s interval in `reaper.go`.
- **DAG**: `graph.go` (build + cycle detection), `sort.go` (Kahn levels), `executor.go`
  (level-by-level, fail-fast), `status.go` (tracker). `main.go` wires
  `dag.NewBlockingSubmitter`, which blocks on the shared `CompletionRegistry`.
- **Terminal**: `terminal.Substrate` has exactly one implementation, `TmuxSubstrate`,
  which shells out via `exec.LookPath("tmux")`.

### Protobuf regeneration

`proto/orchestrator.proto` is the contract. `make generate` writes into `gen/pb/orchestrator`.
`gen/` is gitignored — generated code is never committed, so a fresh clone must run
`make generate` (or `./setup.sh`) before anything compiles.

## Constraints and gotchas

1. **State is in-memory only.** `main.go` constructs `state.NewMockStore()` and nothing
   else. All state is lost on restart. `RedisStore` is complete (~500 lines) but has no
   non-test caller.
2. **Redis is not used by the orchestrator.** `setup.sh`, `docker-compose.yml` and CI all
   start a `redis:7-alpine` container, and `go-redis` is a direct dependency, but the
   shipped binary never connects. Redis is needed for `make test-integration`, nothing else.
3. **`internal/health` reports `RedisPingOK`** — that field pings whatever store is wired,
   which today is the MockStore. It is not evidence of a Redis connection.
4. **`internal/gateway` is dead code from the running binary's perspective.** Do not
   describe judge-routing or cost tracking as live behaviour. `config/models.yaml` is
   likewise only read by gateway code.
5. **`supervisor.ShutdownHandler` has zero callers.** `main.go` implements its own signal
   handling in `gracefulShutdown`. Editing `ShutdownHandler` changes nothing at runtime.
6. **Adding a spawn kind touches three places**: the `switch req.Kind` validation in
   `handleSpawnAgent`, the `providerRoster` slice in `internal/httpbridge/handlers.go`,
   and the `cliCommands` / `interactiveCommands` / `cliBinaries` maps in
   `internal/cliagent/cliagent.go`. Missing any one produces a half-registered kind.
7. **Task priority is min-first.** The queue dequeues the *lowest* priority number first
   (`MockStore` sorts ascending; `RedisStore` uses `ZPopMin`). A "high priority" task is a
   low number.
8. **Floor 0 is reserved.** An explicit `floor: 0` spawn is rejected with 400; an omitted
   floor auto-picks the lowest non-full floor >= 1.
9. **Cancel bypasses the agent state machine.** The bridge holds no `Supervisor` or
   `agent.Machine`, so `/cancel` acts directly on the store: best-effort Ctrl-C to the
   PTY, then `ClearCurrentTask`, then a compare-and-set to idle. A cancelled DAG node is
   observed by the executor as *completed with no output*, not as failed.
10. **Reassign is an inert hint.** `/reassign` persists `TaskMeta.Provider`, but the
    supervisor's assign loop never reads it. It is requeue-with-a-hint, not live migration.
11. **`make generate` needs the protoc Go plugins on PATH.** `protoc-gen-go` and
    `protoc-gen-go-grpc` usually live in `~/go/bin`, which many shells omit. `run.sh`
    prepends it explicitly.
12. **`gen/` is gitignored.** Never commit generated protobuf output; never assume it
    exists in a clean checkout.

## Platform support

- **Linux and macOS**: full experience. tmux drives live agent sessions; `/output` and
  `/input` work; the terminal view shows a live PTY.
- **Windows or any host without tmux**: `main.go` logs
  `terminal substrate unavailable, running headless` and continues. `cliagent` then falls
  back to one-shot process execution (`claude -p …`, `codex exec …`, `opencode run …`,
  `pi -p …`) with output streamed back over gRPC. `GET /api/agents/{id}/output` and
  `POST /api/agents/{id}/input` both return **501 Not Implemented**.
- **`godot-xterm` is not committed.** The live PTY terminal requires installing it via
  `godot/addons/install_godot_xterm.sh` (documented in [godot/addons/VENDOR.md](godot/addons/VENDOR.md));
  `setup.sh` does this automatically. Its PTY support is Linux/macOS only.

## Active work and known gaps

Honest gap list, roughly in priority order:

- Wire `state.RedisStore` into `cmd/orchestrator` so state survives a restart, or delete it.
- Wire `internal/gateway` into something, or mark it explicitly experimental.
- Wire `supervisor.ShutdownHandler` into the shutdown path, or remove it in favour of
  `gracefulShutdown`.
- Put the `testenv` suite (`internal/...` + `e2e/...`) into a Makefile target and CI.
- Put the Godot headless test scripts under `godot/tests/` into CI.
- Correct the versioning section in [.github/CI-CD-Guide.md](.github/CI-CD-Guide.md) and the
  README — both still describe a stale `YY.MM.Major.Minor` scheme.
- Fix the README's spec links: the four headline specs live under `specs/`, not repo root.

## Git workflow

All work is issue-driven. **Never push directly to `main`.**

1. Create an issue and label it. `issue-branch-handler.yml` auto-creates a branch and a
   draft PR. Label priority when several are present: `epic` > `feature` > `task` > `bug` > `hotfix`.
2. Sub-issues branch from their parent's branch.
3. All PRs are squash-merged.

| Label | Branch | PR title prefix |
|-------|--------|-----------------|
| `epic` | `epic/{n}-kebab-title` | `epic:` |
| `feature` | `feature/{n}-kebab-title` | `feat:` |
| `task` | `task/{n}-kebab-title` | `chore:` |
| `bug` | `bug/{n}-kebab-title` | `fix:` |
| `hotfix` | `hotfix/{n}-kebab-title` | `hotfix:` |

### Versioning — `YY.Major.Minor.Patch[Suffix]`

Validated against `^([0-9]{2})\.([0-9]+)\.([0-9]+)\.([0-9]+)([a-zA-Z]*)$`. Current `VERSION`
is `26.4.0.0`.

| Merged branch | Effect |
|---------------|--------|
| `epic/*` | Major +1, Minor -> 0, Patch -> 0, suffix cleared |
| `feature/*` | Minor +1, Patch -> 0, suffix cleared |
| `task/*`, `bug/*` | Patch +1, suffix cleared |
| `hotfix/*` | Append/increment suffix letter (a, b … z, aa …) |

Bumps fire only on **merged** PRs targeting `main` or `epic/**`. Year rollover is automatic:
if the stored `YY` differs from the current year, the version resets to `YY.0.0.0`.

### Reference documents

- [specs/go-orchestrator-core-spec.md](specs/go-orchestrator-core-spec.md)
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — how the system actually fits together
- [specs/terminal-substrate-spec.md](specs/terminal-substrate-spec.md)
- [specs/model-gateway-spec.md](specs/model-gateway-spec.md)
- [specs/pixel-office-ui-spec.md](specs/pixel-office-ui-spec.md)
- [godot/PORTING.md](godot/PORTING.md), [godot/PARITY.md](godot/PARITY.md)
- [.github/CI-CD-Guide.md](.github/CI-CD-Guide.md)
- [docs/research/Agentic-Orchestrator-MOC.md](docs/research/Agentic-Orchestrator-MOC.md) — pre-build research; historical, not a description of the current system

Specs describe intent. Where a spec and the code disagree, the code wins — check before
treating a spec feature as built.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **agenKic-orKistrator** (3333 symbols, 7616 relationships, 55 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

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
