# Feature: Magic Tower UI

## Overview

**User Story**: As an orchestrator operator, I want a fantasy-themed desktop application where AI agents appear as animated character classes working inside a vertical magic tower so that I can visually monitor agent activity across project floors, observe terminal output through layered display modes, and interact with the orchestration system through a delightful fantasy interface.

**Problem**: CLI-only orchestration lacks visual feedback and makes it hard to monitor multiple agents simultaneously. Without a visual layer, operators must switch between terminal panes manually and have no spatial awareness of agent states or project workloads.

**Out of Scope**: Orchestrator logic (core), model routing (gateway), raw tmux management (terminal substrate — this feature consumes the substrate's output for display).

---

## Success Condition

> This feature is complete when the Godot 4 magic tower application renders project floors with animated character-class agents, streams real-time state from the orchestrator via HTTP/SSE, and supports all three output layers (floating runes, spell scrolls, raw terminal with PTY) with movable/resizable/dockable panels.

---

## Open Questions

No known unknowns. All design decisions resolved during brainstorming (2026-03-29).

---

## Scope

### Must-Have

- **Godot 4 project with vertical magic tower architecture** — a tower where each floor is a project/workspace mapped by cwd: tower renders at low internal resolution (e.g., 320x180) upscaled with nearest-neighbor filtering
- **Floor scenes as hexagonal polygons** — each floor is a hex that grows into larger polygons under load via a composite load metric (active agents, queue depth, token throughput)
- **Fantasy character class agents** with animation states mapped to agent roles and state machine: alchemist, scribe, archmage, wardkeeper, librarian, enchanter, apprentice — sprites visually change on state transitions (idle, assigned, working, reporting)
- **Three-layer output display** — floating runes (glance: ambient status symbols above agents), spell scrolls (focus: formatted task output in parchment-styled panels), raw terminal + PTY (power-user: godot-xterm live shell, Linux/macOS; display-only on Windows)
- **Basic tower navigation** — vertical scroll/pan through floors with mouse wheel and keyboard
- **HTTP/SSE bridge** — REST endpoints for commands, SSE stream for real-time orchestrator events; Godot HTTPClient (no GDExtension needed)
- **Panel framework** — resizable/dockable panels for spell scroll views, agent details, and raw terminal: layout persists across sessions
- **Agent status overlay** — show agent name, character class, current task, and state as UI elements: clicking an agent shows its status panel with enchanted nameplate styling
- **Full tower navigation** — floor select sidebar, minimap, keyboard shortcuts for floor jumping
- **Task submission UI** — quest board panel where the operator submits new tasks to the orchestrator
- **Agent interaction menu** — right-click context menu on agent characters for reassign, inspect, pause, and terminate actions

### Should-Have

- **Polygon morphing animation** — smooth geometric transitions as floors grow/shrink under changing load
- **Palette swap shader** — provider-themed colour shifts on agent sprites and floor accents based on which model is active (Arcane/amber for Claude, Prismatic/crystal for Gemini, Infernal/crimson for OpenAI, Forge/ember for Ollama, Abyssal/violet for DeepSeek)
- **Tier particles** — ambient particle effects per provider element theme (arcane sparks, prismatic shards, infernal embers, forge cinders, abyssal wisps)
- **Panel floaty animations** — spell scroll panels flutter and settle when opened; parchment texture grain on UI surfaces
- **Ambient effects** — parallax background layers, floor lighting that dims on idle and brightens under load, moss/vine detail on lower floors transitioning to arcane runes on upper floors
- **Sound design** — ambient tower hum, state transition chimes, floor-switch whoosh, task completion fanfare
- **Pixel-art sprite sheets** — full character class sprite sets with directional animation (idle, walk, cast, report)

### Nice-to-Have

- **Isometric tower view** — toggle between side-scroll and isometric perspective
- **Agent pathfinding** (A* within floor based on activity type — move to cauldron, lectern, anvil)
- **Day/night cycle** — Canopy Spire palette shifts with time of day; tower windows glow at night
- **Command palette** — fuzzy-search command bar for power users (assign task, switch floor, filter agents)
- **Remote connection dialog** — connect to orchestrator instances on other machines via SSH tunnel

---

## Technical Plan

**Affected Components** (new Godot project):
- `godot/project.godot` — Godot project config, viewport scaling
- `godot/scenes/tower.tscn` — main tower scene with vertical floor stack
- `godot/scenes/floor.tscn` — floor scene (hexagonal polygon, dynamic sizing)
- `godot/scenes/agent.tscn` — agent character scene (character class sprite + animations)
- `godot/scenes/terminal_panel.tscn` — godot-xterm raw terminal panel
- `godot/scenes/spell_scroll.tscn` — spell scroll formatted output panel
- `godot/scenes/floating_runes.tscn` — ambient status rune overlay
- `godot/scenes/status_overlay.tscn` — agent status UI with enchanted nameplate
- `godot/scenes/quest_board.tscn` — task submission panel
- `godot/scenes/tower_nav.tscn` — floor select sidebar and minimap
- `godot/scripts/tower.gd` — tower logic, floor management, vertical navigation
- `godot/scripts/floor.gd` — floor polygon sizing, load-based morphing
- `godot/scripts/agent_character.gd` — agent sprite state machine, character class mapping
- `godot/scripts/terminal_manager.gd` — connects to orchestrator via HTTP/SSE, feeds terminal output to godot-xterm
- `godot/scripts/spell_scroll.gd` — formatted output rendering on parchment panel
- `godot/scripts/floating_runes.gd` — ambient rune placement and animation
- `godot/scripts/panel_layout.gd` — dockable panel management, layout persistence
- `godot/scripts/http_sse_bridge.gd` — HTTPClient REST commands + SSE event stream parsing
- `godot/scripts/quest_board.gd` — task submission form and validation
- `godot/assets/sprites/` — character class sprite sheets (alchemist, scribe, archmage, wardkeeper, librarian, enchanter, apprentice)
- `godot/assets/tiles/` — tower floor tiles, stone textures, moss/vine overlays
- `godot/assets/ui/` — parchment textures, enchanted nameplate frames, scroll backgrounds

**API Contracts** (data flow):
- Orchestrator (Go/gRPC) -> HTTP/SSE bridge endpoint -> Godot HTTPClient -> agent scenes + terminal panels + floor state
- REST: `POST /tasks`, `PUT /agents/{id}/action`, `GET /floors`
- SSE: `GET /events` — streams agent state changes, task updates, floor load metrics

**Composite Load Metric** (drives floor polygon size):
- `load = w1 * active_agents + w2 * queue_depth + w3 * token_throughput`
- Hex (6 sides) at base load, grows to heptagon, octagon, etc. as load increases
- Floor polygon vertex count = `6 + floor(load / threshold)`

**Provider Elemental Themes** (drives palette swap shader):

| Provider | Element | Primary Colour | Particle |
|----------|---------|---------------|----------|
| Claude (Anthropic) | Arcane | Warm amber `#d4a574` | Arcane sparks |
| Gemini (Google) | Prismatic | Cool crystal `#88c8f7` | Prismatic shards |
| OpenAI | Infernal | Crimson `#d06060` | Infernal embers |
| Ollama (Local) | Forge | Ember `#f0a070` | Forge cinders |
| DeepSeek | Abyssal | Violet `#a088e0` | Abyssal wisps |

**Canopy Spire Colour Scheme** (vertical gradient):

| Zone | Hex Range | Feel |
|------|-----------|------|
| Base/ground floors | `#142a20` → `#1a2e30` | Earthy grove, grounded |
| Mid floors | `#162228` | Teal transition |
| Upper/summit floors | `#1e1a35` → `#2a2545` | Twilight arcane |
| Accents | `#c8a84e` gold, `#d4a574` amber | Gold leaf highlights |
| Parchment UI | `#ede0c4` bg, `#2e2a18` ink | Scroll surfaces |
| Terminal UI | `#0a0e14` dark | Raw terminal background |

**Dependencies**: Godot 4.2+, godot-xterm GDExtension, orchestrator HTTP/SSE endpoints

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| godot-xterm PTY is Linux/macOS only — Windows gets display-only | High | Document; consider SSH output bridge as Windows fallback |
| godot-xterm has no stable v1.0 release | Medium | Pin to specific commit; wrap in abstraction layer |
| HTTPClient SSE parsing requires manual chunked-transfer handling in GDScript | Medium | Write a dedicated SSE parser node; test with high-frequency event streams |
| Polygon morphing animation may cause frame drops with many floors | Medium | Profile early; cap max polygon vertex count; use LOD for off-screen floors |
| Pixel-perfect rendering breaks at non-integer DPI | Medium | Test at multiple resolutions; force integer scaling in project settings |

---

## Planned Godot Scene Tree

```
Tower (Node2D)
├── Background (ParallaxBackground)
│   ├── SkyLayer (ParallaxLayer)
│   └── CloudLayer (ParallaxLayer)
├── FloorContainer (Node2D)  — vertically stacked
│   ├── Floor_0 (Node2D)
│   │   ├── HexPolygon (Polygon2D)  — dynamic vertex count
│   │   ├── FloorTiles (TileMap)
│   │   ├── AgentContainer (Node2D)
│   │   │   ├── Agent_alchemist (CharacterBody2D)
│   │   │   │   ├── Sprite (AnimatedSprite2D)
│   │   │   │   ├── FloatingRune (Node2D)
│   │   │   │   └── NamePlate (Control)
│   │   │   └── Agent_scribe (CharacterBody2D)
│   │   └── FloorAmbience (GPUParticles2D)
│   ├── Floor_1 (Node2D)
│   │   └── ...
│   └── Floor_N (Node2D)
├── Camera (Camera2D)  — vertical scroll/pan
├── UI (CanvasLayer)
│   ├── TowerNav (VBoxContainer)  — floor select sidebar + minimap
│   ├── PanelContainer (Control)
│   │   ├── SpellScroll (PanelContainer)  — formatted output
│   │   ├── TerminalPanel (Control)  — godot-xterm raw terminal
│   │   └── StatusOverlay (PanelContainer)  — agent details
│   ├── QuestBoard (PanelContainer)  — task submission
│   └── AgentContextMenu (PopupMenu)
└── HTTPSSEBridge (Node)  — REST + SSE connection manager
```

---

## Acceptance Scenarios

```gherkin
Feature: Magic Tower UI
  As an orchestrator operator
  I want a fantasy-themed desktop app showing agents as character classes in a tower
  So that I can visually monitor and interact with the orchestration system

  Background:
    Given the Godot application is running
    And the orchestrator is connected via HTTP/SSE bridge

  Rule: Tower renders with project floors and agent characters

    Scenario: Tower loads with registered projects and agents
      Given the orchestrator has 2 active projects and 5 registered agents
      When the application starts
      Then the tower renders with 2 floors
      And each floor displays its assigned agents as character class sprites

    Scenario: Floor polygon grows under load
      Given floor "project-alpha" has a base hexagon shape
      When the composite load metric exceeds the growth threshold
      Then the floor polygon morphs to a heptagon
      And the floor area visually expands

    Scenario: Agent sprite reflects state machine
      Given agent "alchemist-01" is in "idle" state on floor "project-alpha"
      When the orchestrator transitions "alchemist-01" to "working"
      Then the agent sprite animation changes from idle to casting

  Rule: Three-layer output display

    Scenario: Floating runes show ambient status
      Given agent "scribe-02" is in "working" state
      When the agent is processing a task
      Then floating rune symbols appear above the agent sprite
      And the runes pulse in the provider's elemental colour

    Scenario: Spell scroll shows formatted output
      Given agent "scribe-02" has produced task output
      When the user clicks on "scribe-02"
      Then a spell scroll panel opens with parchment-styled formatted output
      And the scroll shows agent name, task description, and output text

    Scenario: Raw terminal displays live shell output
      Given agent "archmage-01" is in "working" state
      When the user opens the raw terminal panel for "archmage-01"
      Then the godot-xterm panel shows live shell output from the agent's tmux pane
      And scrollback is navigable

    Scenario: Display-only mode on unsupported platform
      Given the platform does not support PTY (Windows)
      When the terminal panel renders
      Then output is displayed read-only
      And a notice indicates PTY is unavailable

  Rule: Tower navigation

    Scenario: Vertical scroll between floors
      Given the tower has 5 floors
      When the user scrolls the mouse wheel up
      Then the camera pans upward to reveal higher floors
      And the Canopy Spire gradient shifts from emerald to indigo

    Scenario: Floor select sidebar navigation
      Given the tower has 5 floors
      When the user clicks "project-gamma" in the floor select sidebar
      Then the camera snaps to the "project-gamma" floor

  Rule: Agent status interaction

    Scenario: Click agent to view status
      Given agent "wardkeeper-01" is working on task "refactor-auth"
      When the user clicks on wardkeeper-01's character
      Then an enchanted nameplate overlay shows: name, class, current task, state, uptime

    Scenario: Right-click agent for context menu
      Given agent "librarian-01" is in "working" state
      When the user right-clicks on librarian-01's character
      Then a context menu appears with: reassign, inspect, pause, terminate

  Rule: Task submission

    Scenario: Submit task via quest board
      Given the quest board panel is open
      When the user fills in a task description and selects a target floor
      And clicks "Submit Quest"
      Then the task is sent to the orchestrator via HTTP POST
      And a confirmation rune appears on the quest board

  Rule: Provider theming

    Scenario: Agent reflects provider elemental theme
      Given agent "enchanter-01" is using Claude (Arcane)
      When the agent is in "working" state
      Then the agent sprite has a warm amber palette overlay
      And arcane spark particles emit around the character

  Rule: Panel layout persistence

    Scenario: User rearranges panels and layout persists
      Given the user drags the spell scroll panel to the right side
      When the application is closed and reopened
      Then the spell scroll panel is on the right side
```

---

## Task Breakdown

**Epic**: #4 — Implement Pixel Office UI (to be renamed Magic Tower UI)

### Phase 1: Skeleton Tower — Feature #86

| ID | Issue | Task | Priority | Dependencies | Status |
|:---|:------|:-----|:---------|:-------------|:-------|
| T1 | #94 | Godot project scaffold (viewport, pixel-perfect, folder structure) | High | None | pending |
| T2 | #95 | HTTP/SSE bridge — Go side (REST + SSE alongside gRPC, 7 event types) | High | None | pending |
| T3 | #98 | BridgeManager — Godot side (HTTPClient + SSE parser + event dispatch) | High | T1, T2 | pending |
| T4 | #100 | Floor scene + tower layout (Polygon2D, TileMap, permanent + ephemeral spawn/despawn) | High | T1, T3 | pending |
| T5 | #102 | Agent character scene (placeholder sprites, 5 animation states, state machine) | High | T1, T3 | pending |
| T6 | #104 | Floating runes (filtered output, float+fade, provider colour tint) | High | T5 | pending |
| T7 | #106 | Basic navigation (scroll/keys, Camera2D, Q/E edge rotation) | High | T4 | pending |

### Phase 2: Interaction Layer — Feature #88

| ID | Issue | Task | Priority | Dependencies | Status |
|:---|:------|:-----|:---------|:-------------|:-------|
| T8 | #109 | Panel framework (move, resize, dock, snap, fullscreen, layout persistence) | High | T1 | pending |
| T9 | #111 | Spell scroll panel (parchment theme, output history, quill animation) | High | T8, T5 | pending |
| T10 | #112 | Raw terminal + PTY (godot-xterm, bidirectional, ANSI fallback on Windows) | High | T8, T9 | pending |
| T11 | #114 | Status overlay (agent nameplate on hover/click) | High | T5 | pending |
| T12 | #116 | Full navigation (minimap + floor tabs) | Med | T4, T7 | pending |
| T13 | #118 | Task submission UI (quest board, themed as commissioning a quest) | Med | T3, T8 | pending |
| T14 | #119 | Agent interaction menu (right-click context menu) | Med | T5, T9, T10, T11 | pending |

### Phase 3: Magic Polish — Feature #90

| ID | Issue | Task | Priority | Dependencies | Status |
|:---|:------|:-----|:---------|:-------------|:-------|
| T15 | #124 | Polygon morphing (composite load → side count, vertex lerp, width scaling) | Med | T4 | pending |
| T16 | #125 | Palette swap shader (power_level float + provider_lut texture) | Med | T5 | pending |
| T17 | #127 | Tier particle effects (glow, sparkles, orbiting motes, trails by power level) | Med | T16 | pending |
| T18 | #129 | Panel floaty animations (hover bob, parchment flutter, particle trail on drag) | Low | T8 | pending |
| T19 | #131 | Ambient floor effects (candles, dust motes, cauldron bubbles, dormant dimming) | Low | T4 | pending |
| T20 | #133 | Floor build/dissolve animations (brick assembly, magical shimmer, fade-out) | Low | T4 | pending |
| T21 | #134 | Sound design (ambient hum, per-class work sounds, state transition SFX) | Med | T5, T4 | pending |
| T22 | #137 | Pixel art sprites (7 classes x ~24 frames, workstation props, floor tilesets) | Med | T5, T16 | pending |

### Phase 4: Stretch Goals — Feature #92

| ID | Issue | Task | Priority | Dependencies | Status |
|:---|:------|:-----|:---------|:-------------|:-------|
| T23 | #140 | Isometric view (alternative 2.5D camera mode) | Low | T4 | pending |
| T24 | #141 | Agent pathfinding (A* to workstation, library, stairs between floors) | Low | T5, T4 | pending |
| T25 | #142 | Day/night cycle (exterior changes, interior candles brighten) | Low | T19 | pending |
| T26 | #143 | Command palette (Ctrl+K fuzzy-find agents, floors, tasks) | Low | T3, T12 | pending |
| T27 | #144 | Remote orchestrator connection dialog (local/SSH/custom URL) | Low | T3 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass manually on Linux/macOS
- [ ] No regressions on orchestrator core or terminal substrate
- [ ] Agent sprite animations match state machine transitions for all character classes
- [ ] Three-layer output works: floating runes visible, spell scroll opens on click, raw terminal displays live output (Linux/macOS)
- [ ] HTTP/SSE bridge connects to orchestrator and streams events without dropped frames
- [ ] Floor polygons respond to composite load metric changes
- [ ] Tower navigation (scroll, sidebar, keyboard) works across 5+ floors
- [ ] Panel layout persists across application restarts
- [ ] Provider elemental themes apply correct palette per model
- [ ] Renders correctly at 1080p and 1440p with integer scaling

---

## References

- Research MOC: `docs/research/Agentic-Orchestrator-MOC.md`
- UI framework evaluation: `docs/research/pixelated-desktop-ui-framework-evaluation.md`
- Competitive landscape: `docs/research/orchestration-research-findings.md`
- Native desktop rendering patterns: `docs/research/patterns/Native-Desktop-Rendering.md`

---
*Authored by: Clault KiperS 4.6*
