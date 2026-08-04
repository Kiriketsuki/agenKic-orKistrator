# agenKic-orKistrator

![Version](https://img.shields.io/badge/version-26.4.0.0-blue)
![Go](https://img.shields.io/badge/go-1.26+-00ADD8?logo=go)
![Godot](https://img.shields.io/badge/godot-4.x-478CBF?logo=godotengine)
![Tests](https://img.shields.io/badge/tests-go%20%2B%20godot%20headless-brightgreen)
![Build](https://img.shields.io/github/actions/workflow/status/Kiriketsuki/agenKic-orKistrator/version-bump.yml?label=ci)
![License](https://img.shields.io/badge/license-MIT-green)
![Status](https://img.shields.io/badge/status-alpha%20%2F%20working%20vertical%20slice-yellow)

OrKi is a two-process desktop app: a Go orchestrator and a Godot 4 pixel-art client. You
summon coding-CLI agents from the GUI; the orchestrator spawns each one inside its own
`tmux` session, feeds it a task prompt, polls the pane until the CLI settles, and streams
state back to the client, where every agent is drawn as a pixel character on a floor of a
fisheye-projected tower.

---

## What you actually see when you run it

`./run.sh` builds the orchestrator if it is stale, starts it, waits for the HTTP bridge to
accept connections, then launches Godot. Ctrl-C tears both down.

- **F3 — Grimoire.** The summoning flyout. Pick a sigil, pick a floor, and an agent spawns.
  Floor 0 is reserved and never accepts a spawn (`internal/httpbridge/handlers.go`).
- **Spawnable kinds are exactly five:** `sim`, `claude`, `codex`, `opencode`, `pi`. `sim` is
  an in-process fake worker used for demos and tests; the other four exec the real CLI
  binary of that name. **Gemini, OpenAI, Ollama and DeepSeek are cosmetic only** — they
  exist as glyph/colour entries in the Godot scripts and in the Reassign submenu, and are
  not spawnable.
- **The agent appears as a pixel character** on its floor and animates through its lifecycle
  states.
- **F5 — Panels.** Opens per-agent panels: terminal view, spell scroll (parchment output
  view), quest board (task and small-DAG submission), plus layout presets.
- **Right-click an agent** for View Details / Open Terminal / Open Scroll / Reassign /
  Cancel / Copy Output / Banish.
- **F4 — Power.** Banish all, restart orchestrator, settings, quit to title.
- **Minimap, hex compass and floor tabs** are read-only views over the same state.

Reassign persists a provider hint on the task and requeues it — the supervisor's assign
loop does not read that field, so it is not a live migration of a running agent.

## Quickstart

Prerequisites:

| Requirement | Notes |
|---|---|
| Go 1.26+ | `go.mod` declares `go 1.26.1` |
| `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc` | plugins must be on `PATH` — they land in `$(go env GOPATH)/bin` |
| `tmux` | required for live terminals; without it the orchestrator runs headless |
| Godot >= 4.3 | `setup.sh` pins 4.4.1 when installing from GitHub releases |
| Docker + compose | only for the Redis container used by `make test-integration` |

```bash
./setup.sh   # system packages, protoc plugins, godot-xterm addon, generate + build, redis
./run.sh     # orchestrator + Godot
```

`setup.sh` supports Arch, Debian/Ubuntu and macOS. `run.sh` rebuilds `bin/orchestrator`
itself when any Go source is newer than the binary, polls the bridge address for up to 10s
before launching Godot, and traps INT/TERM to kill Godot and SIGTERM the orchestrator.

## Ports and environment

| Variable | Default | Purpose |
|---|---|---|
| `GRPC_ADDR` | `:50051` | gRPC control plane (agents dial this) |
| `HEALTH_ADDR` | `:8080` | health HTTP server |
| `BRIDGE_ADDR` | `127.0.0.1:8081` | HTTP + SSE bridge (the Godot client talks to this) |
| `BRIDGE_API_KEY` | unset | when set, the bridge requires a bearer token; when unset it logs `no BRIDGE_API_KEY set — running without auth` and serves unauthenticated |
| `MIN_AGENT_COUNT` | `1` | minimum agent count used by the health aggregator |

## Architecture at a glance

Full reference: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

`cmd/orchestrator` runs two servers in one process:

- **HTTP + SSE bridge (`internal/httpbridge`) on `127.0.0.1:8081`** — this is the
  Godot-facing transport, not gRPC. Routes: `GET /api/agents`,
  `GET /api/agents/{id}/output`, `GET /api/floors`, `GET /api/providers`, `POST /api/tasks`,
  `POST /api/dags`, `POST /api/agents/{id}/input|cancel|reassign|despawn`,
  `POST /api/agents/spawn`, `POST /api/admin/restart`, `GET /events/stream`.
- **gRPC (`internal/ipc`, `proto/orchestrator.proto`) on `:50051`** — the
  agent-to-orchestrator control plane only, dialled by `internal/cliagent` and
  `internal/simagent`.

Behind those: a supervisor doing registration, heartbeat staleness detection and crash
recovery with exponential backoff (1s base, 30s cap) plus a sliding-window circuit breaker;
a DAG engine with cycle detection, Kahn topological levels and level-by-level fail-fast
execution; and a terminal substrate.

Two things worth knowing before you read the code:

- **State is in-memory.** `cmd/orchestrator/main.go` constructs `state.NewMockStore()`. A
  complete `RedisStore` exists in `internal/state/redis.go` but has no non-test call sites,
  so the running binary never connects to Redis. Redis is still started by `setup.sh` /
  `docker-compose.yml` and required by `make test-integration`.
- **tmux is the only terminal substrate.** `internal/terminal.Substrate` has exactly one
  implementation, `TmuxSubstrate`. There is no WezTerm implementation in this repo.

`internal/gateway` contains a complete, tested LiteLLM client and Haiku-based judge-router,
but it is imported by no `cmd/` binary — the running orchestrator makes no LLM completion
calls and does no cost-based model routing. Treat it as built-but-unwired.

## Repo layout

| Path | Contents |
|---|---|
| `cmd/orchestrator` | supervisor entrypoint — gRPC, health and bridge servers |
| `cmd/demoworker` | standalone demo worker binary |
| `cmd/spritegen`, `cmd/assetslice` | asset dev tools |
| `internal/agent` | agent state machine |
| `internal/cliagent` | spawns real coding CLIs in tmux, types prompts, polls output |
| `internal/simagent` | in-process fake worker (`sim` kind) |
| `internal/supervisor` | registration, heartbeats, restart policy, reapers, assign loop |
| `internal/dag` | graph, cycle detection, topological sort, executor, status tracker |
| `internal/httpbridge` | HTTP + SSE bridge for the Godot client |
| `internal/ipc` | gRPC server, handlers, health HTTP server |
| `internal/state` | store interface, in-memory `MockStore`, unwired `RedisStore` |
| `internal/terminal` | `Substrate` interface and the tmux implementation |
| `internal/health` | health aggregator |
| `internal/gateway` (+ `providers/`) | LiteLLM client and judge-router (not wired into any binary) |
| `proto/` | `orchestrator.proto` |
| `gen/` | generated protobuf/gRPC Go code (gitignored — run `make generate`) |
| `e2e/` | end-to-end tests, `testenv` build tag |
| `godot/` | Godot 4 project — scripts, scenes, assets, `tests/`, `addons/` |
| `specs/` | feature specs |
| `docs/research/` | pre-build research notes |
| `config/models.yaml` | model tier definitions used by the gateway package |

## Build and test

```bash
make generate         # protoc -> gen/pb/orchestrator (needs protoc plugins on PATH)
make build            # bin/orchestrator
make test             # go test -race -count=1 ./internal/...
make test-integration # adds -tags=integration; needs Redis (REDIS_URL)
make lint             # golangci-lint run ./...
make clean
```

A third test tier is gated behind the `testenv` build tag (`internal/dag`,
`internal/supervisor`, and everything in `e2e/`). **No committed automation runs it** — there
is no Makefile target and no CI step. To run it by hand:

```bash
go test -tags=testenv ./internal/... ./e2e/...
```

Godot tests are standalone headless GDScript files; no GUT or other test runner is vendored:

```bash
godot --headless --path godot --script tests/<name>.gd
```

CI (`.github/workflows/ci.yml`) runs on pushes to `main`, `epic/**`, `feature/**` and PRs to
`main`, `epic/**`: `make generate`, `go vet ./...`, `make test`, then `make test-integration`
against a `redis:7-alpine` service.

## Versioning

Format: `YY.Major.Minor.Patch[Suffix]` — e.g. `26.4.0.0` or `26.4.0.1a`.

| Merged branch | Effect |
|---|---|
| `epic/*` | Major +1, Minor and Patch → 0 |
| `feature/*` | Minor +1, Patch → 0 |
| `task/*` or `bug/*` | Patch +1 |
| `hotfix/*` | append/increment suffix letter (a, b … z, aa …) |

Bumps fire only on merged PRs into `main` or `epic/**`. Year rollover is automatic: if the
`YY` component no longer matches the current year, the version resets to `YY.0.0.0`.

Current version: `26.4.0.0`.

## Contributing

All work goes through issues. **Never push directly to `main`.**

1. Create an issue and label it — `issue-branch-handler.yml` auto-creates the branch and a
   draft PR. Label priority: `epic` > `feature` > `task` > `bug` > `hotfix`.
2. Branch names are generated as `<type>/<issue#>-kebab-title`; PR titles get the matching
   conventional-commit prefix (`epic:`, `feat:`, `chore:`, `fix:`, `hotfix:`).
3. PRs are squash-merged into their parent branch.

See [`.github/CI-CD-Guide.md`](.github/CI-CD-Guide.md) for the workflow details — note its
versioning section is stale; the table above matches `version-bump.yml`.

## godot-xterm (optional native PTY)

The raw-terminal panel mode's live PTY (Linux/macOS) depends on the
[godot-xterm](https://github.com/lihop/godot-xterm) GDExtension. It is
**not committed** to this repo — prebuilt native binaries don't belong in
source control, and it has no stable v1 release, so we pin to an exact
commit instead.

To install it locally:

```bash
godot/addons/install_godot_xterm.sh
```

See [`godot/addons/VENDOR.md`](godot/addons/VENDOR.md) for the pinned
version, verification steps, and platform notes.

Platform support:

- **Linux/macOS with the addon installed** — live PTY terminal panels backed by tmux.
- **Without the addon** — the raw terminal panel falls back to a polling, read-only ANSI
  view. The project runs cleanly either way.
- **Windows, or any machine without tmux** — the orchestrator logs
  `terminal substrate unavailable, running headless`. Agents then fall back to one-shot CLI
  exec (`claude -p …`, `codex exec …`, `opencode run …`, `pi -p …`), and
  `GET /api/agents/{id}/output` and `POST /api/agents/{id}/input` return `501`.

Other Godot-side notes: [`godot/PORTING.md`](godot/PORTING.md),
[`godot/PARITY.md`](godot/PARITY.md).

## Specs and research

| Spec | Component |
|---|---|
| [`specs/go-orchestrator-core-spec.md`](specs/go-orchestrator-core-spec.md) | Supervisor, agent state machine, DAG engine |
| [`specs/terminal-substrate-spec.md`](specs/terminal-substrate-spec.md) | tmux session management, command injection |
| [`specs/model-gateway-spec.md`](specs/model-gateway-spec.md) | LiteLLM proxy, judge-router, cost tracking (built, not wired) |
| [`specs/pixel-office-ui-spec.md`](specs/pixel-office-ui-spec.md) | Godot 4 pixel office, godot-xterm |

[`specs/`](specs/) holds the full set; [`docs/specs/todo/`](docs/specs/todo/) holds specs not
yet started. Specs describe intended behaviour — check the code before assuming a spec
section is implemented.

Pre-build research, including the architectural decision table and competitive landscape,
lives in [`docs/research/Agentic-Orchestrator-MOC.md`](docs/research/Agentic-Orchestrator-MOC.md).
