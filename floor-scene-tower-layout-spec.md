# Feature: Floor Scene + Tower Layout

## Overview

**User Story**: As an orchestrator operator, I want the Godot UI to render a vertical tower of floors — each a cross-section of a regular polygon — so that I can see agent activity organized spatially by floor and edge, rotate between edges to view different workgroups, and watch the tower grow and shrink as workload changes.

**Problem**: The Godot UI has no spatial representation of where agents live. Without floors and a tower structure, all agents would be in a single flat scene with no visual hierarchy, making it impossible to distinguish workgroups or observe orchestrator topology.

**Out of Scope**: Agent sprite characters (T5), pixel-art tile assets (T22), floor build/dissolve animations (T20), agent pathfinding (T24), minimap and floor tab wiring (T7/T12), polygon morphing (T15 — Phase 3). Agent slots are placeholder rectangles. TileMap uses minimal flat floor + desk rectangles.

---

## Success Condition

> This feature is complete when the Godot UI renders a vertical tower with fisheye-focused floors, each floor displays its active edge's interior with agent placeholder slots, Q/E rotates edges with a slide transition, ephemeral floors spawn/linger/dissolve on SSE events, and the Go side publishes `floor.created`/`floor.removed` events when tmux sessions are created or destroyed.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | | | [ ] |

---

## Scope

### Must-Have
- **Regular polygon tower**: Tower is a configurable regular polygon (default hexagon, `polygon_sides` from `tower.json`). Camera shows a side view of one edge at a time.
- **Fisheye floor layout**: Focused floor (d=0) at full scale with interior detail. Adjacent floors (d=1) at ~0.4 scale with simplified interior. Distant floors (d>=2) show tower exterior only (stone walls, mystical elements — no interior rendered).
- **Floor scene with procedural interior**: Each floor has one TileMap regenerated on edge rotation. `EdgeLayout` places desks based on the active edge's agent count. Placeholder rectangles represent agent slots.
- **Edge rotation**: Q/E rotates the focused floor's `active_edge` with a slide transition (~0.3s). New interior generated off-screen, tweened in, old freed. Only focused floor rotates; compressed neighbors show a static label.
- **Camera scroll and zoom**: Scroll wheel shifts the focused floor index up/down. Ctrl+scroll (or pinch) zooms in/out. Zoom out reveals more tower silhouette; zoom in fills viewport with focused floor.
- **Permanent floors from config**: `tower.json` defines permanent floors loaded at startup (name, initial properties). These persist regardless of orchestrator state.
- **Ephemeral floor lifecycle**: Floors spawn on `floor.created` SSE event. On `floor.removed`: transition to LINGERING state (dimmed, configurable duration from `tower.json`, default 30s). Reactivate if new agent arrives during linger. Dissolve (queue_free) after linger timeout. T20 will add animation to the dissolve step.
- **Agent edge assignment**: Agents assigned an `edge_index` on their floor. Auto-assigned round-robin with soft clustering by `task_id` affinity. Stored in TowerManager's agent registry.
- **Tower exterior**: Procedural roof and base polygons drawn from `polygon_sides`. Frames the tower visually. Distant floors (d>=2) render as exterior wall segments.
- **Go-side floor SSE events**: New event types `floor_created` and `floor_removed` published by the supervisor when tmux sessions are created/destroyed. Mapped to `floor.created` and `floor.removed` SSE events with payloads `SSEFloorCreated` and `SSEFloorRemoved`. BridgeManager gets `floor_created`/`floor_removed` signals and dispatches them.
- **Connection status awareness**: TowerManager listens to `connection_status_changed` — on disconnect, tower enters a "stale" visual state; on reconnect, re-syncs floors from REST.

### Should-Have
- Configurable `base_url` for tower.json path via `@export` on TowerManager
- Floor name labels visible on focused and adjacent floors
- Smooth camera tween when changing focused floor (not instant jump)

### Nice-to-Have
- Edge rotation on compressed (d=1) floors via Shift+Q/Shift+E
- Tower "breathing" idle animation (subtle scale pulse on exterior)
- Floor activity indicator (glow intensity based on agent count)

---

## Technical Plan

**Affected Components**:

New Godot files:
- `godot/scripts/tower/tower_manager.gd` — coordinator: fisheye layout, input handling, BridgeManager signal routing, agent registry
- `godot/scripts/tower/tower_exterior.gd` — procedural roof/base Polygon2D drawing from `polygon_sides`
- `godot/scripts/tower/floor_scene.gd` — single floor: edge rotation, TileMap regeneration, agent slot management, linger lifecycle
- `godot/scripts/tower/edge_layout.gd` — procedural desk placement given agent count and edge_index
- `godot/scripts/models/tower_config.gd` — parsed `tower.json` data class
- `godot/scenes/floor_scene.tscn` — Background (Polygon2D) + Interior (Node2D) + AgentSlots (Node2D)
- `godot/config/tower.json` — polygon_sides, permanent floor definitions, linger_duration_sec

Modified Godot files:
- `godot/scenes/main.tscn` — attach `tower_manager.gd` to Tower node
- `godot/scripts/autoload/bridge_manager.gd` — add `floor_created`/`floor_removed` signals + SSE dispatch
- `godot/scripts/models/bridge_data.gd` — extend `FloorData` (add `is_permanent`, `polygon_sides` fields from config merge)

Modified Go files:
- `internal/httpbridge/types.go` — add `SSEFloorCreated`, `SSEFloorRemoved` payload structs
- `internal/httpbridge/sse.go` — add `floor_created`/`floor_removed` cases to `mapStoreEvent`
- `internal/httpbridge/sse_test.go` — tests for new floor event mapping
- `internal/supervisor/supervisor.go` — add `PublishEvent` calls for floor creation/destruction

**Data Model Changes**:

tower.json (new):
```json
{
  "polygon_sides": 6,
  "linger_duration_sec": 30,
  "permanent_floors": [
    { "name": "main", "label": "Main Hall" },
    { "name": "archive", "label": "Archive" }
  ]
}
```

TowerConfig (new GDScript class):
- `polygon_sides: int`
- `linger_duration_sec: float`
- `permanent_floors: Array[Dictionary]`

SSEFloorCreated (new Go struct):
- `Name string json:"name"`
- `AgentCount int json:"agent_count"`
- `Cursor string json:"cursor,omitempty"`

SSEFloorRemoved (new Go struct):
- `Name string json:"name"`
- `Cursor string json:"cursor,omitempty"`

BridgeManager signal additions:
- `signal floor_created(floor_data: BridgeData.FloorData)`
- `signal floor_removed(floor_name: String)`

Agent registry (TowerManager internal Dictionary):
- `_agent_assignments: Dictionary` — maps `agent_id → { "floor": String, "edge": int }`

**API Contracts** (SSE events — consumed by BridgeManager):

| SSE Event | Payload | Published When |
|:----------|:--------|:---------------|
| `floor.created` | `{ name, agent_count, cursor }` | Supervisor calls `SpawnSession` successfully |
| `floor.removed` | `{ name, cursor }` | Supervisor calls `DestroySession` successfully |

**Dependencies**: Godot 4.2, Go orchestrator (T2 HTTP/SSE bridge), BridgeManager (T3), terminal substrate

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Fisheye layout math produces sub-pixel artifacts at compressed scales | Medium | Snap positions to integers; test at 320x180 viewport |
| TileMap regeneration on edge rotation causes visible frame stutter | Low | Generate new TileMap off-screen before tweening; profile in Godot |
| Substrate doesn't currently publish events on session create/destroy | Certain | T4 adds `PublishEvent` calls to supervisor's session management paths |
| `tower.json` not found at startup | Low | Fall back to defaults (polygon_sides=6, no permanent floors) |

---

## Acceptance Scenarios

```gherkin
Feature: Floor Scene + Tower Layout
  As an orchestrator operator
  I want to see agents organized in a vertical tower of floors
  So that I can visually navigate workgroups and observe orchestrator topology

  Background:
    Given the Go orchestrator is running on localhost:8081
    And BridgeManager.base_url is set to "http://localhost:8081"
    And tower.json defines polygon_sides=6 and 2 permanent floors

  Rule: Tower renders with fisheye layout

    Scenario: Startup renders permanent floors in fisheye
      When the Godot application starts
      Then 2 permanent floors are rendered in the FloorsContainer
      And the bottom floor is focused (full scale, interior visible)
      And the top floor is adjacent (compressed, simplified interior)
      And TowerExterior renders a hexagonal roof and base

    Scenario: Distant floors show exterior only
      Given the tower has 5 floors
      And floor 3 is focused
      Then floors 1 and 5 (d=2) render as exterior wall segments
      And no TileMap interior is visible on distant floors

  Rule: Edge rotation via Q/E

    Scenario: Rotate edge on focused floor
      Given floor 1 is focused with active_edge=0
      When the user presses E
      Then active_edge advances to 1
      And a slide transition animates the interior swap (~0.3s)
      And the new edge's agents are displayed in placeholder slots

    Scenario: Edge rotation wraps around
      Given polygon_sides=6 and active_edge=5
      When the user presses E
      Then active_edge wraps to 0

    Scenario: Only focused floor rotates
      Given floor 2 is focused and floor 1 is adjacent
      When the user presses Q
      Then only floor 2's edge rotates
      And floor 1's display does not change

  Rule: Camera scroll and zoom

    Scenario: Scroll changes focused floor
      Given floor 1 is focused and 3 floors exist
      When the user scrolls up
      Then floor 2 becomes focused
      And the fisheye layout re-centers on floor 2

    Scenario: Zoom out reveals tower silhouette
      Given floor 2 is focused
      When the user zooms out (ctrl+scroll)
      Then the camera zoom decreases
      And more floors become visible in the viewport

    Scenario: Zoom in fills viewport with focused floor
      Given the camera is zoomed out
      When the user zooms in
      Then the focused floor fills more of the viewport

  Rule: Agent assignment to edges

    Scenario: New agent assigned to floor and edge
      Given the orchestrator emits agent.registered for agent "a1" on floor "main"
      Then TowerManager assigns agent "a1" to an edge on floor "main"
      And a placeholder rectangle appears in the active edge's AgentSlots (if that edge is visible)

    Scenario: Agents cluster by task affinity
      Given agents "a1" and "a2" are both working on task "t1"
      Then they are assigned to the same edge when possible

  Rule: Ephemeral floor lifecycle

    Scenario: Ephemeral floor spawns on SSE event
      Given the orchestrator creates a new tmux session "workers-3"
      When BridgeManager receives floor.created with name="workers-3"
      Then a new FloorScene is added to the tower
      And the fisheye layout adjusts to include it

    Scenario: Ephemeral floor lingers on removal
      Given ephemeral floor "workers-3" is ACTIVE
      When BridgeManager receives floor.removed with name="workers-3"
      Then the floor transitions to LINGERING state (dimmed, alpha ~0.5)
      And a 30s linger timer starts

    Scenario: Lingering floor reactivates on new agent
      Given floor "workers-3" is LINGERING
      When a new agent registers on floor "workers-3"
      Then the floor transitions back to ACTIVE (full opacity)
      And the linger timer is cancelled

    Scenario: Lingering floor dissolves after timeout
      Given floor "workers-3" has been LINGERING for 30s with no reactivation
      Then the floor is freed from the scene tree
      And the fisheye layout adjusts

  Rule: Go-side floor SSE events

    Scenario: Floor created event published
      Given the supervisor spawns a new tmux session "workers-3"
      Then a floor_created event is published to the event stream
      And SSE clients receive floor.created with name="workers-3"

    Scenario: Floor removed event published
      Given the supervisor destroys tmux session "workers-3"
      Then a floor_removed event is published to the event stream
      And SSE clients receive floor.removed with name="workers-3"

  Rule: Tower exterior framing

    Scenario: Roof and base drawn from polygon_sides
      Given tower.json defines polygon_sides=6
      Then TowerExterior draws a hexagonal roof at the top of the tower
      And a hexagonal base at the bottom

  Rule: Permanent floors persist

    Scenario: Permanent floors not affected by floor.removed
      Given floor "main" is permanent (from tower.json)
      When BridgeManager receives floor.removed with name="main"
      Then floor "main" remains in the tower (not lingered or removed)
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T4.1 | Go-side: add `SSEFloorCreated`/`SSEFloorRemoved` types to `types.go`, map in `sse.go`, add tests | High | None | pending |
| T4.2 | Go-side: add `PublishEvent` calls for `floor_created`/`floor_removed` in supervisor session management | High | T4.1 | pending |
| T4.3 | Godot: add `floor_created`/`floor_removed` signals to BridgeManager + SSE dispatch | High | T4.1 | pending |
| T4.4 | Godot: create `tower_config.gd` and `tower.json` config with defaults | High | None | pending |
| T4.5 | Godot: create `floor_scene.tscn` + `floor_scene.gd` — Background Polygon2D, Interior Node2D, AgentSlots Node2D, placeholder agent rendering | High | T4.4 | pending |
| T4.6 | Godot: create `edge_layout.gd` — procedural desk placement given agent count + edge_index | High | T4.5 | pending |
| T4.7 | Godot: create `tower_manager.gd` — fisheye layout engine, floor ordering, focused floor tracking, scroll/zoom input | High | T4.5 | pending |
| T4.8 | Godot: implement edge rotation — Q/E input, slide transition tween, TileMap regeneration via EdgeLayout | High | T4.6, T4.7 | pending |
| T4.9 | Godot: create `tower_exterior.gd` — procedural roof/base Polygon2D from polygon_sides | Med | T4.7 | pending |
| T4.10 | Godot: implement ephemeral floor lifecycle — ACTIVE/LINGERING/DISSOLVING states, linger timer, reactivation | High | T4.5, T4.3 | pending |
| T4.11 | Godot: wire TowerManager to BridgeManager signals — agent_registered, agent_state_changed, floor_created, floor_removed | High | T4.7, T4.3 | pending |
| T4.12 | Godot: implement agent edge assignment — round-robin with task_id clustering affinity | Med | T4.11 | pending |
| T4.13 | Godot: distant floor exterior rendering (d>=2 shows wall segments, no interior) | Med | T4.7, T4.9 | pending |
| T4.14 | Godot: update `main.tscn` — attach tower_manager.gd to Tower node, verify scene tree | Med | T4.7 | pending |

---

## Exit Criteria

- [ ] Permanent floors render on startup from tower.json config
- [ ] Fisheye layout correctly scales focused (d=0), adjacent (d=1), and distant (d>=2) floors
- [ ] Distant floors (d>=2) show tower exterior only, no interior
- [ ] Q/E edge rotation works with slide transition on focused floor
- [ ] Camera scroll changes focused floor; zoom in/out works
- [ ] Ephemeral floors spawn on floor.created SSE event
- [ ] Ephemeral floors linger on floor.removed, reactivate on new agent, dissolve after timeout
- [ ] Permanent floors are not affected by floor.removed events
- [ ] Go-side floor_created/floor_removed events published and mapped to SSE
- [ ] Go-side tests pass for new SSE event mapping
- [ ] Agent placeholder slots appear on the active edge of focused floors
- [ ] No regressions on T1 Godot project scaffold or T3 BridgeManager

---

## References

- Parent feature: #86 (Phase 1 — Skeleton Tower)
- Issue: #100
- PR: #103
- Depends on: T1 (#94, merged), T2 (#95, merged), T3 (#98, merged)
- Pixel Office UI spec: `specs/pixel-office-ui-spec.md`
- BridgeManager spec: `bridgemanager-godot-side-spec.md`
- Go HTTP/SSE bridge: `internal/httpbridge/` — types.go, handlers.go, sse.go, bridge.go
- Terminal substrate interface: `internal/terminal/substrate.go`

---
*Authored by: Clault KiperS 4.6*
