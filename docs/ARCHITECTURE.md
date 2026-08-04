# OrKi Architecture

How the system is actually assembled as of `VERSION` **26.4.0.0** (`VERSION:1`).

This document describes what is built and running. Every load-bearing claim carries a
`file:line` citation. Things that exist only in a spec, or exist as code but are not wired
into any binary, are confined to [Planned vs built](#12-planned-vs-built) — they are not
described anywhere else as if they work.

Repo shape at a glance: 120 `.go` files, 53 of them `*_test.go`; 82 `.gd` files; 29 files
under `godot/tests/`. Go module is `github.com/Kiriketsuki/agenKic-orKistrator`, language
version `go 1.26.1` (`go.mod:1-3`).

---

## 1. Purpose and scope

OrKi is a two-process desktop application. A Go orchestrator supervises agents; a Godot 4
client renders them as pixel characters on the floors of a fisheye-projected tower
(`godot/scripts/tower/tower_manager.gd:1-2`).

The user summons an agent from the client. Floor 0 is reserved and never accepts a spawn
(`internal/httpbridge/handlers.go:775-780`). Exactly five spawn kinds are accepted:
`sim`, `claude`, `codex`, `opencode`, `pi` (`internal/httpbridge/handlers.go:719-729`).
`sim` is an in-process fake worker (`internal/simagent/simagent.go:1-4`); the other four
exec a real CLI binary (`internal/cliagent/cliagent.go:71-77`) inside a tmux session named
`agent-<id>`, type the task prompt into that pane, and poll `CaptureOutput` until two
identical snapshots indicate the CLI has settled (`internal/cliagent/cliagent.go:242-278`,
`330-356`).

The bridge also serves that same five-kind roster to the client's summon grid
(`internal/httpbridge/handlers.go:684-690`).

UI surfaces on the Godot side: the Grimoire spawn flyout (F3), the Panels flyout (F5), the
Power flyout (F4), a quest board (single task plus small DAG submission), a spell scroll
(parchment output view), a raw terminal view, a minimap, a hex compass, floor tabs, and a
right-click agent context menu.

---

## 2. Process topology

Three kinds of process exist at runtime:

1. **`bin/orchestrator`** — the Go supervisor. One process, three listeners.
2. **The Godot client** — one process, launched with `godot --path godot`.
3. **One tmux session per CLI agent**, named `agent-<agentID>`
   (`internal/cliagent/cliagent.go:156`), each running a real CLI (`claude`, `codex`,
   `opencode`, `pi`) via `cd <workdir> && clear && exec <bin>`
   (`internal/cliagent/cliagent.go:201`).

```
                      ./run.sh
                         |
        +----------------+-----------------+
        |                                  |
  bin/orchestrator                    godot --path godot
        |                                  |
        |  :50051  gRPC   <----------------+---- NOT used by Godot
        |  :8080   health HTTP (/healthz /readyz /progress)
        |  127.0.0.1:8081  HTTP + SSE bridge  <--- Godot talks here
        |
        |  supervisor + tmux substrate
        v
   tmux sessions:  agent-<id>   agent-<id>   agent-<id>  ...
                      |             |            |
                    claude        codex       opencode
                      \             |            /
                       \            |           /
                        gRPC dial back to :50051
                        (cliagent / simagent only)
```

Ports and their env overrides:

| Listener | Default | Env var | Cite |
|---|---|---|---|
| gRPC | `:50051` | `GRPC_ADDR` | `cmd/orchestrator/main.go:30-33` |
| Health HTTP | `:8080` | `HEALTH_ADDR` | `cmd/orchestrator/main.go:35-38` |
| HTTP/SSE bridge | `127.0.0.1:8081` | `BRIDGE_ADDR` | `cmd/orchestrator/main.go:84-87` |

The startup banner prints all three (`cmd/orchestrator/main.go:196`).

### The launcher

`./run.sh` is the supported way to start both processes.

- Rebuild only when stale: any `.go` file under `cmd/` or `internal/`, or `go.mod`, newer
  than `bin/orchestrator` triggers `make generate build` with `$HOME/go/bin` prepended to
  `PATH` for the protoc plugins (`run.sh:29-41`).
- Start the orchestrator in the background (`run.sh:62-64`).
- Poll `BRIDGE_HOST` (default `127.0.0.1:8081`) via `/dev/tcp`, 50 attempts at 0.2s — up
  to 10 seconds — and abort if the orchestrator exits during startup (`run.sh:66-81`).
- Launch `godot --path godot` (`run.sh:85-87`).
- `wait -n` on both PIDs, so either process dying ends the script (`run.sh:91`).
- A single `cleanup` trap on `INT TERM EXIT` kills Godot, then `kill -TERM`s the
  orchestrator so its graceful-shutdown path runs (`run.sh:44-58`).

Graceful shutdown inside the orchestrator cancels the root context, stops the gRPC server,
drains the health and bridge HTTP servers with a 5s timeout, shuts the DAG executor down,
and stops the supervisor — in that order (`cmd/orchestrator/main.go:224-234`). The same
function serves both the signal handler (`cmd/orchestrator/main.go:189-193`) and
`POST /api/admin/restart`.

---

## 3. Orchestrator-to-client protocol: the HTTP/SSE bridge

The Godot client speaks HTTP and Server-Sent Events to `internal/httpbridge`. It does not
speak gRPC.

### Endpoints

All routes are registered in one place (`internal/httpbridge/bridge.go:178-192`):

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/agents` | List agents and their states |
| GET | `/api/agents/{id}/output` | Visible-pane snapshot from the agent's tmux session |
| GET | `/api/floors` | Tower floors, derived from tmux sessions |
| GET | `/api/providers` | Spawn-kind roster for the summon grid |
| POST | `/api/tasks` | Enqueue a task |
| POST | `/api/dags` | Submit a DAG for execution |
| POST | `/api/agents/{id}/input` | Send keystrokes/text into the agent's pane |
| POST | `/api/agents/{id}/cancel` | Interrupt and detach the agent's current task |
| POST | `/api/agents/{id}/reassign` | Requeue the current task with a tier/provider hint |
| POST | `/api/agents/{id}/despawn` | Remove the agent and destroy its tmux session |
| POST | `/api/agents/spawn` | Summon a new agent |
| POST | `/api/admin/restart` | Re-exec the orchestrator binary |
| GET | `/events/stream` | SSE event stream |

### SSE events

`/events/stream` emits dot-namespaced event types. The client dispatches on them in
`godot/scripts/autoload/bridge_manager.gd:297-354`; the orchestrator emits them in
`internal/httpbridge/sse.go:194-310`:

| Wire event type | Emitted at | Godot signal |
|---|---|---|
| `agent.registered` | `sse.go:194` | `agent_registered` (`bridge_manager.gd:299`) |
| `agent.state_changed` | `sse.go:205,215,224,233,242,276` | `agent_state_changed` (`bridge_manager.gd:306`) |
| `agent.deregistered` | `sse.go:256` | `agent_deregistered` (`bridge_manager.gd:334`) |
| `agent.output` | `sse.go:285` | `agent_output` (`bridge_manager.gd:341`) |
| `floor.created` | `sse.go:300` | `floor_created` (`bridge_manager.gd:344`) |
| `floor.removed` | `sse.go:310` | `floor_removed` (`bridge_manager.gd:348`) |

The client reconnects with `?since=<cursor>` from the last event carrying a `cursor` field
(`bridge_manager.gd:245-251,353-354`) and treats 20s of silence as a dead connection
(`bridge_manager.gd:358-360`).

### Auth

Bearer-token auth is enabled **only** when `BRIDGE_API_KEY` is set. Otherwise the
orchestrator logs `HTTP bridge: no BRIDGE_API_KEY set — running without auth` and serves
every route unauthenticated (`cmd/orchestrator/main.go:131-136`). The middleware is only
installed when a key is present (`internal/httpbridge/bridge.go:195-197`) and compares in
constant time (`internal/httpbridge/bridge.go:425-438`).

### Semantics worth knowing before you change a handler

- **Task priority is a min-heap.** Lower score dequeues first
  (`internal/state/store.go:139-141`).
- **Cancel bypasses the state machine.** It sends `\x03` (Ctrl-C) into `agent-<id>`
  best-effort, clears the current task, then settles the agent to idle via a CAS with a
  tolerant re-read — not through `agent.Machine` (`internal/httpbridge/handlers.go:363-379`,
  `651`). It also calls `CompletionRegistry.Complete(taskID)` so a DAG node blocked on that
  task unblocks (`internal/httpbridge/handlers.go:390-395`).
- **Reassign is a hint, not a migration.** It re-enqueues the task at its original priority
  with `Tier`/`Provider` written into `TaskMeta`, but nothing in the supervisor's assign
  loop reads those fields, so the very same agent frequently picks the task back up. The
  handler's own doc comment says so explicitly (`internal/httpbridge/handlers.go:403-421`,
  `484-508`).
- **Despawn destroys the tmux session** (`internal/httpbridge/handlers.go:562`).
- **`/output` and `/input` return 501** when no terminal substrate is wired
  (`internal/httpbridge/handlers.go:62-69`, `234-241`).
- **`/floors` filters out `agent-*` sessions**, because a per-agent PTY is not a tower floor
  (`internal/httpbridge/handlers.go:107-135`). With no substrate it returns an empty list
  rather than 501.
- **Spawn reserves the floor slot before calling the spawner** and releases the reservation
  if the spawner fails, closing a TOCTOU window on floor capacity
  (`internal/httpbridge/handlers.go:766-800`).

---

## 4. Agent control plane: gRPC

`proto/orchestrator.proto` defines one service, `OrchestratorService`
(`proto/orchestrator.proto:9`), implemented by `internal/ipc`
(`internal/ipc/server.go:27-65`):

| RPC | Proto | Handler |
|---|---|---|
| `RegisterAgent` | `:11` | `handlers.go:48` |
| `SubmitTask` | `:14` | `handlers.go:57` |
| `StreamOutput` (bidi stream) | `:17` | `handlers.go:92` |
| `GetAgentState` | `:20` | `handlers.go:72` |
| `SubmitDAG` | `:23` | `handlers.go:150` |
| `GetDAGStatus` | `:26` | `handlers.go:232` |
| `CompleteAgent` | `:29` | `handlers.go:207` |
| `Heartbeat` | `:32` | `handlers.go:218` |
| `StartWork` | `:35` | `handlers.go:171` |
| `ReportOutput` | `:38` | `handlers.go:189` |

`StartGRPC` also registers the standard gRPC health service when a health server is
supplied (`internal/ipc/server.go:60-62`), which `main` wires and drives on a 2s ticker.

**Who dials `:50051`:** only `internal/cliagent` (`cliagent.go:130-134`) and
`internal/simagent` (`simagent.go:27-32`). Both are in-process spawners that connect back
to the orchestrator's own endpoint — `main` rewrites a bare `:50051` to `127.0.0.1:50051`
for the loopback dial (`cmd/orchestrator/main.go:99-102`). The Godot client is not a gRPC
client.

**Regeneration:** `make generate` runs `protoc` with the Go and Go-gRPC plugins, writing
into `gen/pb/orchestrator` (`Makefile:8-17`). `gen/` is gitignored, so a fresh checkout
must generate before it will build; `protoc-gen-go` and `protoc-gen-go-grpc` live in
`$HOME/go/bin`, which `run.sh` and `setup.sh` prepend to `PATH` explicitly
(`run.sh:38-40`, `setup.sh:108`).

---

## 5. Agent lifecycle

Spawn, end to end:

1. The Grimoire flyout issues `POST /api/agents/spawn`.
2. Kind is validated against the five-item allowlist; empty defaults to `sim`
   (`internal/httpbridge/handlers.go:718-729`). Name and tier are defaulted from
   round-robin tables when omitted (`handlers.go:730-737`).
3. `workdir`, if given, must be an existing absolute directory (`handlers.go:739-758`).
4. A floor slot is reserved atomically; floor 0 is rejected outright
   (`handlers.go:766-796`).
5. The `AgentSpawner` closure runs (`cmd/orchestrator/main.go:117-129`): `sim` goes to
   `simagent.Spawn`, everything else to `cliagent.Spawn` with the substrate and workdir
   options attached.
6. `cliagent.Spawn` checks the CLI binary is on `PATH`, dials gRPC, and calls
   `RegisterAgent` (`internal/cliagent/cliagent.go:117-141`).
7. The supervisor creates the bare tmux session during registration; `cliagent` then
   launches the interactive CLI inside it (`internal/cliagent/cliagent.go:156-163`,
   `173-207`). If that fails it logs and falls back to headless one-shot exec.
8. The agent's worker loop heartbeats every 3s and polls its own state. On `ASSIGNED` it
   resolves the prompt from the store, calls `StartWork`, and either types the prompt into
   the pane (interactive) or runs the one-shot command (headless)
   (`internal/cliagent/cliagent.go:219-256`).
9. Settle detection: poll `CaptureOutput(session, 50)` every 5s; return when two
   consecutive snapshots match **and** at least 15s has elapsed, or at the 5-minute
   `taskTimeout` cap (`internal/cliagent/cliagent.go:35`, `45-46`, `329-356`).
10. One output chunk is streamed, then `ReportOutput` + `CompleteAgent`
    (`internal/cliagent/cliagent.go:283-290`).

### State machine

Four states — `idle`, `assigned`, `working`, `reporting` (`internal/agent/state.go:14-19`) —
and five events (`internal/agent/event.go:6-12`).

```
  idle --task_assigned--> assigned --work_started--> working
    ^                                                   |
    |                                             output_ready
    |                                                   v
    +----------output_delivered----------------- reporting

  agent_failed: from ANY state -> idle
```

The table is the whole truth; anything not in it is an `InvalidTransitionError`
(`internal/agent/transition.go:5-37`). `Machine.ApplyEvent` reads the current state,
validates the transition, and persists via `CompareAndSetAgentState`, returning an
immutable snapshot — the `Machine` holds no mutable state of its own
(`internal/agent/machine.go:14-73`). CAS is the atomicity guard for the transition itself;
the supervisor's per-agent mutex remains necessary for compound operations
(`internal/agent/machine.go:34-41`).

---

## 6. Supervisor

`internal/supervisor/supervisor.go` (~769 lines) owns agent registration
(`:108`), a heartbeat loop (`:251`) with staleness checking (`:265`) against a 30s
threshold (`:18`), crash recovery (`crashAgent`, `:304`), the task-assign loop (`:390`,
`:404`) with idle-agent selection (`:531`), and the gRPC-facing lifecycle methods
`Heartbeat` / `StartWork` / `ReportOutput` / `CompleteAgent` (`:580`, `:612`, `:641`,
`:669`). Per-agent mutexes serialize compound operations (`:765`).

### Restart policy

`internal/supervisor/restart.go` — exponential backoff with a sliding-window circuit
breaker. Defaults (`restart.go:70-77`):

| Knob | Default |
|---|---|
| `baseBackoff` | 1s |
| `maxBackoff` | 30s |
| `crashThreshold` | 5 |
| `crashWindow` | 60s |

More than `crashThreshold` crashes inside `crashWindow` opens the circuit and returns
`ShouldRestart: false` with `ErrCircuitOpen` (`restart.go:97-113`). Otherwise backoff is
`min(base * 2^(n-1), max)` over the consecutive-crash counter (`restart.go:136-144`).
`RecordSuccess` clears both the counter and the crash history (`restart.go:126-132`).

### Two reapers

They are separate and easily confused:

1. **Tombstone sweep** — `CompletionRegistry.SweepOnce` drops expired cancel tombstones and
   unclaimed pre-closed waiter entries (`completion.go:166-190`), driven by
   `StartReaper` on a ticker (`completion.go:191-214`). TTL is `2 * staleThreshold` and the
   sweep interval is `staleThreshold` (`completion.go:9-18`). `main` starts it against the
   root context (`cmd/orchestrator/main.go:71`).
2. **Orphan tmux session sweep** — every 15s, drop agents whose terminal session no longer
   exists (`reaper.go:12-16`, `reapLoop` at `reaper.go:43`). Session-name matching uses the
   shared `agent-` prefix constant so it cannot drift from the name construction
   (`reaper.go:18-21`).

### ShutdownHandler is not used

`supervisor.NewShutdownHandler` exists (`internal/supervisor/shutdown.go:11-19`) but has no
non-test callers — the only references are in `internal/supervisor/shutdown_test.go`.
`main` does its own signal handling and calls its own `gracefulShutdown`
(`cmd/orchestrator/main.go:189-193`, `224-234`).

---

## 7. DAG engine

`internal/dag`:

- **Graph construction and validation** — `NewGraph` returns `ErrEmptyDAG`,
  `ErrDuplicateNode`, `ErrNodeNotFound`, or `ErrCycleDetected` (`graph.go:19-70`), with
  `Predecessors`, `TaskSpec`, `successorsOf`, and `inDegrees` accessors (`graph.go:74-111`).
- **Topological levels** — Kahn's algorithm, returning `[][]string` levels and
  `ErrCycleDetected` on a cycle (`sort.go:8-47`).
- **Executor** — level-by-level execution with fail-fast: `Execute` (`executor.go:50`),
  `run` (`executor.go:91`), `executeNode` (`executor.go:121`), plus `ActiveExecutionCount`
  and `Shutdown` (`executor.go:37-42`).
- **StatusTracker** — per-execution node status with `MarkNodeRunning` /
  `MarkNodeCompleted` / `MarkNodeFailed`, an `ActiveCount`, and proto conversion
  (`status.go:64-205`).

### Two submitters

`StoreSubmitter` is **deprecated** and enqueue-only. Its own doc comment states that a nil
return reflects enqueue success, not task execution, so the executor's `MarkNodeCompleted`
would be a lie about dependency ordering (`storesubmitter.go:11-24`).

`BlockingSubmitter` blocks on `CompletionRegistry.Wait` and is the one that is wired:

```go
submitter := dag.NewBlockingSubmitter(store, registry)
executor  := dag.NewExecutor(ctx, submitter)
```
— `cmd/orchestrator/main.go:73-74`. The same `registry` instance is handed to the
supervisor and to the HTTP bridge, so a cancel through the bridge unblocks the exact DAG
node waiting on that task (`cmd/orchestrator/main.go:51`, `88-95`).

---

## 8. State layer

`state.StateStore` is the single storage interface: agent state (with CAS), agent field
records, an event stream with consumer groups, agent/task binding, and a priority task
queue, plus `Ping`/`Close` (`internal/state/store.go:95-153`).

**The running orchestrator uses `MockStore`, in memory:**

```go
store := state.NewMockStore()
```
— `cmd/orchestrator/main.go:47`. That is the only store constructed anywhere in `cmd/`.

`RedisStore` is complete (`internal/state/redis.go`, ~503 lines) but `NewRedisStore` has
**zero non-test call sites** — the only callers are `internal/state/redis_test.go:25,41` and
`internal/state/redis_taskmeta_test.go:27`. `go-redis` remains a direct dependency
(`go.mod:7`).

The health aggregator's `HealthSnapshot.RedisPingOK` field is a naming holdover: it is set
from a `Ping` on whatever store was injected (`internal/health/aggregator.go:19-35`,
`132`), which in the shipped binary is the `MockStore`.

**Where Redis is genuinely required today:** `make test-integration`, which runs the
`integration`-tagged tests with a live `REDIS_URL` (`Makefile:22-23`), and CI, which starts
a `redis:7-alpine` service for exactly that step (`.github/workflows/ci.yml:13-22`,
`50-53`). `setup.sh` starts the same container via `docker-compose.yml`
(`setup.sh:110-115`, `docker-compose.yml:3-13`). The orchestrator binary itself never
connects to it.

---

## 9. Terminal substrate

`terminal.Substrate` is the substrate-agnostic interface
(`internal/terminal/substrate.go:9-38`):

| Method | Purpose |
|---|---|
| `SpawnSession(ctx, name, cmd)` | Create a detached session running `cmd` |
| `DestroySession(ctx, name)` | Kill the session and all its panes |
| `SendCommand(ctx, session, cmd)` | Type a command into the active pane, then Enter |
| `SendKey(ctx, session, key)` | Send one real key press (not typed literally) |
| `CaptureOutput(ctx, session, lines)` | Read the last N lines from the active pane |
| `ListSessions(ctx)` | All sessions the substrate manages |
| `SplitPane(ctx, session, direction)` | Split the active pane |

**`TmuxSubstrate` is the only implementation** (`internal/terminal/tmux.go:10-26`). It
resolves the binary with `exec.LookPath("tmux")` and returns `ErrTmuxNotFound` if it is
absent. There is no WezTerm code in the repo — `find . -iname '*wezterm*'` returns nothing.

Session naming is `agent-<agentID>` (`internal/cliagent/cliagent.go:156`), with the
`agent-` prefix pinned to one constant so the reaper and the floor listing cannot drift
from it (`internal/supervisor/reaper.go:18-21`). `/api/floors` skips any session matching
that prefix (`internal/httpbridge/handlers.go:120-125`).

### Headless degradation

If tmux is not on `PATH`, `main` logs `terminal substrate unavailable, running headless`
and simply never attaches a substrate (`cmd/orchestrator/main.go:55-60`). Consequences:

- `cliagent` falls back to one-shot exec (`internal/cliagent/cliagent.go:53-67`), running:
  - `claude -p <prompt> --output-format text`
  - `codex exec --skip-git-repo-check <prompt>`
  - `opencode run <prompt>`
  - `pi -p <prompt>`
  Output is streamed line by line over `StreamOutput`, killed at the 5-minute timeout, then
  `ReportOutput` + `CompleteAgent` (`internal/cliagent/cliagent.go:358-436`).
- `/api/agents/{id}/output` and `/api/agents/{id}/input` return 501
  (`internal/httpbridge/handlers.go:62-69`, `234-241`).
- `/api/floors` returns an empty list (`internal/httpbridge/handlers.go:108-111`).

---

## 10. Godot client

The client is a Godot 4 project rooted at `godot/`. One autoload:
`godot/scripts/autoload/bridge_manager.gd` — the HTTP client plus the SSE reader. It owns
connection setup (`:220-231`), the poll/parse loop (`:232-289`), event dispatch
(`:297-354`), cursor tracking for resumable reconnects (`:353-354`), and a 20s keepalive
timeout (`:358-360`). It also caches agent and floor state so views can read it
synchronously.

Script layout under `godot/scripts/`:

| Directory | Responsibility |
|---|---|
| `tower/` | `tower_manager.gd` (fisheye layout, floor ordering, scroll/zoom, signal routing), `floor_scene.gd`, `floor_prism.gd`, `floor_morph.gd`, `floor_hit_test.gd`, `edge_layout.gd`, `backdrop_parallax.gd`, `tower_exterior.gd` |
| `panels/` | `terminal_view.gd`, `spell_scroll_view.gd`, `quest_board_view.gd`, `panel_content_router.gd`, ANSI parsing (`ansi_sepia_parser.gd`, `ansi_sgr_scanner.gd`), `key_passthrough.gd` |
| `overlays/` | `agent_context_menu.gd`, `status_overlay.gd`, `status_overlay_manager.gd` |
| `ui/` | flyouts (`grimoire_flyout.gd`, `panels_flyout.gd`, `power_flyout.gd`), `minimap.gd`, `hex_compass.gd`, `edge_compass.gd`, `floor_tabs.gd`, `floor_banner.gd`, orbs, title screen, sigil config |
| `agents/` | `agent_character.gd`, `agent_sprite.gd`, `floating_rune.gd`, `rune_filter.gd` |
| `models/` | pure-logic modules: `bridge_data.gd`, `wander_math.gd`, `particle_math.gd`, `panel_float_math.gd`, `palette_math.gd`, `tower_config.gd`, `provider_palette.gd` |

Panel framework lives at the `scripts/` root: `panel_manager.gd` with its extracted
`panel_manager_math.gd`, `panel_base.gd`, `layout_presets.gd` (persisted F5 arrangements,
`layout_presets.gd:2-9`), `layout_persistence.gd`, and the `dwindle_tree.gd` /
`dwindle_node.gd` tiling pair.

### TerminalView has two modes

`godot/scripts/panels/terminal_view.gd:1-22` documents both, and the code implements both:

1. **Live PTY** (Linux/macOS) — godot-xterm's `Terminal` and `PTY` GDExtension classes,
   instantiated reflectively via `ClassDB` so the script still parses when the addon is
   absent (`terminal_view.gd:185`, `213-229`). The PTY forks
   `tmux attach -t agent-<id>` at 80x24 (`terminal_view.gd:236-239`).
2. **Polled fallback** — a `RichTextLabel` refreshed from
   `GET /api/agents/{id}/output` every `SCREEN_POLL_SECONDS = 0.7`
   (`terminal_view.gd:26-27`, `350-374`). This is what runs when the addon is missing or
   the agent has no live tmux session (`terminal_view.gd:275`).

godot-xterm is **not committed** — `godot/addons/godot_xterm/` is gitignored
(`.gitignore:50`). Install it with `godot/addons/install_godot_xterm.sh`, which `setup.sh`
invokes (`setup.sh:100`); provenance is recorded in `godot/addons/VENDOR.md`.

### Cosmetic layers carry no runtime meaning

Several dictionaries name providers that cannot be spawned. `TerminalView.PROVIDER_GLYPHS`
carries entries for `gemini`, `openai`, `ollama`, and `deepseek`
(`terminal_view.gd:32-39`), and the context menu's Reassign submenu offers the same list
(`godot/scripts/overlays/agent_context_menu.gd:27`). These are glyph and colour choices
only. The spawnable set is the bridge's five kinds
(`internal/httpbridge/handlers.go:684-690`, `719-729`), and Reassign persists an inert hint
regardless of which one is picked (§3).

### Pure logic and headless tests

Math and configuration logic is deliberately extracted into plain scripts with no autoload
or scene dependencies — `models/wander_math.gd`, `models/particle_math.gd`,
`models/panel_float_math.gd`, `ui/panels_flyout_math.gd`, `ui/power_flyout_math.gd`,
`ui/title_focus_math.gd`, `panel_manager_math.gd` — so they can be exercised without
booting the UI.

No GUT or other Godot test runner is vendored (`godot/tests/wander_math_test.gd:5-6`). Each
test is a standalone `SceneTree` script that exits 1 on failure
(`godot/tests/wander_math_test.gd:18-21`), run as:

```
godot --headless --path godot --script tests/wander_math_test.gd
```
— `godot/tests/wander_math_test.gd:9`. There are 29 files under `godot/tests/` (`.gd`
scripts plus their `.uid` companions). Nothing in CI runs them.

---

## 11. Gateway — built, not wired

`internal/gateway` is a complete and well-tested LiteLLM client and judge-router. It is
imported by **no** binary under `cmd/` (`grep -rn "gateway" cmd/` returns nothing), so the
running orchestrator makes no LLM completion calls and performs no cost-based routing.

What is there:

- **Interfaces** — `Gateway`, `Router`, `Completer`, `AdapterResolver`, `CostTracker`
  (`internal/gateway/gateway.go:220-255`), over a three-tier model:
  `cheap` / `mid` / `frontier` (`gateway.go:12-16`).
- **JudgeRouter** — asks a cheap judge model to classify a task into one of the three
  tiers, defaulting to `claude-haiku-4-5-20251001` and falling back to `mid` when
  classification fails (`internal/gateway/router.go:11`, `23-77`).
- **LiteLLMClient** — POSTs OpenAI-compatible requests to `<baseURL>/chat/completions`,
  default base URL `http://localhost:8000` (`internal/gateway/litellm.go:18`, `107-140`).
- **Provider adapters** — a `FormatAdapter` interface and `Registry`
  (`internal/gateway/providers/provider.go:9-31`) with `anthropic.go`, `openai.go`, and
  `ollama.go`.
- **Supporting code** — `config.go`, `cost.go`, `fallback.go`, each with a test file.
- **`config/models.yaml`** — tier definitions, primary models, and fallback chains. Note it
  names `litellm_base_url: "http://localhost:4000"` (`config/models.yaml:2`), which does not
  match the client's compiled-in default of port 8000; nothing loads the file at runtime
  today, so the two have never had to agree.

---

## 12. Planned vs built

| Item | What the research/spec said | What exists | What is missing |
|---|---|---|---|
| Redis-backed state | Redis Streams + Hashes as the durable state layer | `internal/state/redis.go` (~503 lines), fully tested against a live Redis | `NewRedisStore` has no non-test caller; `main.go:47` constructs `MockStore`. Redis is only needed for `make test-integration` and CI |
| LiteLLM gateway + judge-router | Haiku judges, Sonnet works, Opus architects; cost tracking and fallback chains | `internal/gateway` complete with tests, plus `config/models.yaml` | No `cmd/` binary imports it; no runtime completion calls, no routing, no cost tracking |
| WezTerm substrate | A second `Substrate` implementation alongside tmux | Nothing. `TmuxSubstrate` is the only implementation (`internal/terminal/tmux.go:10-11`) | All of it. `find -iname '*wezterm*'` is empty |
| gRPC as the UI transport | Godot talks to the orchestrator over gRPC | gRPC exists on `:50051` but serves only agents (`cliagent`, `simagent`) | The UI transport is the HTTP/SSE bridge on `127.0.0.1:8081` (`internal/httpbridge/bridge.go:178-192`) |
| Bubbletea / Lip Gloss TUI | An alternative Go TUI front end | No such code found in the repo | All of it |
| Gemini / OpenAI / Ollama / DeepSeek agents | Multi-provider agent roster | Glyph and colour dictionaries only (`terminal_view.gd:32-39`, `agent_context_menu.gd:27`) | They are not spawnable — spawn accepts exactly `sim`, `claude`, `codex`, `opencode`, `pi` (`internal/httpbridge/handlers.go:719-729`) |
| Reassign as live migration | Move a running agent to another provider/tier | Requeue with `Tier`/`Provider` written into `TaskMeta` (`handlers.go:484-508`) | The assign loop never reads those fields; the same agent often re-takes the task (`handlers.go:403-421`) |
| `testenv` suite and Godot tests in CI | Full lifecycle coverage in the pipeline | `testenv`-tagged tests in `internal/dag/executor_test.go`, `internal/supervisor/*`, and all of `e2e/`; 29 files under `godot/tests/` | Neither runs in CI and neither has a Makefile target. Manual invocation is `go test -tags=testenv ./internal/... ./e2e/...` (inferred from the build tags — not documented anywhere in the repo) and `godot --headless --path godot --script tests/<name>.gd` |

---

## Build, run, test

```
make generate         # protoc -> gen/pb/orchestrator (gitignored)
make build            # bin/orchestrator from ./cmd/orchestrator
make test             # go test -race -count=1 ./internal/...
make test-integration # adds -tags=integration; needs REDIS_URL
make lint             # golangci-lint run ./...
make clean            # rm -rf bin/ gen/pb/
```
— `Makefile:1-31`. `make generate` needs `protoc-gen-go` and `protoc-gen-go-grpc` on `PATH`;
both live in `$HOME/go/bin`.

`cmd/` holds four binaries: `orchestrator` (the supervisor), `demoworker`, and two asset dev
tools, `spritegen` and `assetslice`.

CI (`.github/workflows/ci.yml`) runs on push to `main`, `epic/**`, `feature/**` and on PRs
to `main`, `epic/**` (`:3-7`). Steps: install protoc 27.x and the Go plugins,
`make generate`, `go vet ./...`, `make test`, then `make test-integration` with
`REDIS_URL=redis://localhost:6379` against a `redis:7-alpine` service (`:31-53`). The Go
version comes from `go.mod` (`:27-29`).

### Versioning

`YY.Major.Minor.Patch[Suffix]`, validated against
`^([0-9]{2})\.([0-9]+)\.([0-9]+)\.([0-9]+)([a-zA-Z]*)$`
(`.github/workflows/version-bump.yml:69-80`, `.github/workflows/version-validation.yml:31-36`).
Bumps fire only on merged PRs into `main` or `epic/**`:

| Branch prefix | Effect |
|---|---|
| `epic/*` | Major +1, Minor and Patch reset to 0 |
| `feature/*` | Minor +1, Patch reset to 0 |
| `task/*`, `bug/*` | Patch +1 |
| `hotfix/*` | Append/increment a suffix letter (a…z, aa…) |

— `version-bump.yml:88-131`. Year rollover is automatic: a `YY` that no longer matches
`date +%y` resets the version to `<YY>.0.0.0` (`version-bump.yml:82-87`).

`issue-branch-handler.yml` auto-creates a branch and a draft PR when an issue is labelled.
Label priority is `epic` > `feature` > `task` > `bug` > `hotfix`, and PR titles get the
prefixes `epic:` / `feat:` / `chore:` / `fix:` / `hotfix:`.

> The version scheme described in `README.md` and `.github/CI-CD-Guide.md` (`YY.MM.Major.Minor`)
> is stale. The workflow regex above is authoritative.

---

## 13. Pointers

Feature specs live in **`specs/`**, not at the repo root:

- [`specs/go-orchestrator-core-spec.md`](../specs/go-orchestrator-core-spec.md)
- [`specs/terminal-substrate-spec.md`](../specs/terminal-substrate-spec.md)
- [`specs/model-gateway-spec.md`](../specs/model-gateway-spec.md)
- [`specs/pixel-office-ui-spec.md`](../specs/pixel-office-ui-spec.md)

`specs/` holds 16 spec files in total; `docs/specs/todo/` holds 6 more that are not yet
built.

Other useful documents:

- [`docs/research/Agentic-Orchestrator-MOC.md`](research/Agentic-Orchestrator-MOC.md) —
  the pre-build research map. Read it as **historical intent**, not as a description of the
  current system; several of its decisions were superseded (see §12).
- [`godot/PORTING.md`](../godot/PORTING.md)
- [`godot/PARITY.md`](../godot/PARITY.md)
- [`godot/addons/VENDOR.md`](../godot/addons/VENDOR.md) — godot-xterm provenance and install
- [`.github/CI-CD-Guide.md`](../.github/CI-CD-Guide.md) — CI/CD walkthrough; its version
  section is stale (see above)
