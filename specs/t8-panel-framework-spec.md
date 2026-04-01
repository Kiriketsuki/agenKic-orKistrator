# Feature: T8 Panel Framework

## Overview

**User Story**: As a user monitoring AI agents in the pixel office, I want a tiling panel system so that I can view agent output, terminals, and status information in organized, resizable panels alongside the magic tower.

**Problem**: The tower scene currently has no way to display detailed agent information. Users can see agents working but cannot inspect their output, interact with terminals, or view detailed status. Without a panel framework, every downstream interaction feature (T9-T14) is blocked.

**Out of Scope**:
- Themed panel rendering (parchment textures, quill animations) — T9 (Spell scroll panel)
- Terminal PTY integration — T10 (Raw terminal + PTY)
- Agent status overlay content — T11
- Actual floaty animations (hover bob, parchment flutter) — T18 (Phase 3); only hook points in T8
- Panel content implementations — T8 provides the empty container framework only

---

## Success Condition

> This feature is complete when panels can be opened from agent clicks or a panel menu, spawn floating with a materialize + particle entrance animation, dock into a Hyprland-inspired master-dwindle tiling layout on either side of the tower, persist their layout across sessions, and respond to all specified hotkeys.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should T8 also update project.godot from 320x180 to native resolution, or is that a separate task? | Spec author | [x] T8 handles it (T8.1 + T8.2) |
| 2 | What entrance animation style for floating panels? | Spec author | [x] Materialize + particles (matches rune aesthetic) |
| 3 | Minimum panel size in native resolution? | Spec author | [x] DPI-aware: 300x200 at 1080p, scales proportionally (400x267 at 1440p, 600x400 at 4K) |

---

## Scope

### Must-Have
- **PanelBase Control node**: title bar with drag handle, resize handles on edges/corners, close button — acceptance: panel can be freely moved and resized when floating
- **Float state with materialize animation**: panels spawn floating with a materialize + particle effect (panel fades in with glowing particles dispersing outward, matching the rune aesthetic), freely draggable, overlay on tower and docked panels — acceptance: new panel appears with particle materialize animation at a default position, can be dragged anywhere
- **DPI-aware minimum size**: panels enforce a minimum size of 300x200 at 1080p, scaling proportionally with resolution (400x267 at 1440p, 600x400 at 4K) — acceptance: panels cannot be resized below the DPI-scaled minimum
- **Dock state via edge drag**: dragging a floating panel to the left or right viewport edge docks it into the tiling tree — acceptance: visual dock zone indicator appears near edge, releasing panel snaps it into the dwindle tree
- **Master-dwindle tiling**: tower is master window, left and right zones each have an independent BSP dwindle tree with vertical-first alternating splits — acceptance: 3+ panels docked on one side tile correctly via BSP subdivision
- **Master boundary**: resizable divider between tower and panel zones, clamped 30%-80% of viewport width, snaps at 25/50/75% — acceptance: dragging the boundary snaps to thresholds, tower scene repositions accordingly
- **Undock via drag**: dragging a docked panel's title bar off the tiling tree undocks it back to floating, sibling expands to fill gap — acceptance: panel pops out of tree with animation, BSP rebalances
- **Sibling expand on close**: closing a docked panel causes its BSP sibling to expand; closing the last panel on a side causes the tower to reclaim that zone — acceptance: no empty gaps remain after panel close
- **True fullscreen**: double-click title bar or press F to expand panel to full viewport with dimmed overlay behind; Escape or F to restore — acceptance: panel covers entire viewport, previous state restored on exit
- **Layout persistence**: save/load panel tree structure, master boundary ratio, split ratios, panel mode preferences, and float positions to user://layout.json — acceptance: layout survives application restart
- **Hotkeys**: F (fullscreen toggle), Escape (close/restore), Ctrl+\ (hide all panels), Ctrl+Shift+R (reset layout to defaults) — acceptance: all hotkeys functional in both float and dock states
- **Per-panel mode memory**: each panel remembers user preference per agent (e.g., scroll vs terminal view mode) — acceptance: switching modes persists across panel close/reopen and app restart
- **Open panel via agent click**: clicking an AgentCharacter opens (or focuses) their panel — acceptance: character_clicked signal triggers panel creation or focus
- **Panel menu**: hotkey or UI button to list available panels and open by selection — acceptance: menu shows all agents, clicking one opens their panel

### Should-Have
- **Dock zone preview**: visual indicator (glow, highlight region) when dragging a panel near a dock edge, showing where it will land in the BSP tree
- **Animated transitions**: smooth tweened animations for dock, undock, resize, open, close transitions (not just instant snaps)
- **Split divider drag**: dwindle split dividers between docked panels are draggable to adjust ratios (free-ratio, no snapping)

### Nice-to-Have
- **Animation hooks for T18**: virtual methods or signals that themed panel subclasses can override for floaty animations (hover bob, parchment flutter, particle trail on drag)
- **Panel opacity/transparency**: floating panels slightly transparent to see tower behind them
- **Ultrawide layout adaptation**: auto-detect ultrawide aspect ratios and adjust default master boundary accordingly

---

## Technical Plan

**Affected Components**:

| File | Change |
|------|--------|
| `godot/project.godot` | Update viewport from 320x180 to native resolution, update stretch settings |
| `godot/scenes/main.tscn` | Add PanelManager node under UILayer |
| `godot/scripts/panel_base.gd` | New: PanelBase Control node class |
| `godot/scripts/panel_manager.gd` | New: PanelManager — owns dwindle trees, master boundary, layout persistence |
| `godot/scripts/dwindle_tree.gd` | New: BSP tree for one dock zone |
| `godot/scripts/dwindle_node.gd` | New: Branch/Leaf node for BSP tree |
| `godot/scripts/layout_persistence.gd` | New: JSON serializer for layout state |
| `godot/scripts/tower_manager.gd` | Modify: respond to master boundary changes (reposition/resize tower) |
| `godot/scripts/agent_character.gd` | No change — already emits `character_clicked` |
| `godot/config/tower.json` | Possibly add default panel layout config |

**Architecture**:

Five core classes implementing a master-dwindle hybrid tiling system (Hyprland-inspired):

- **PanelManager**: singleton owner of two dwindle trees (left/right zones), manages master boundary divider, routes dock/undock/close operations, delegates layout persistence
- **DwindleTree**: BSP tree for one dock zone, handles insert/remove/rebalance with vertical-first alternating splits, free-ratio dividers
- **DwindleNode**: branch node (split direction + ratio + two children) or leaf node (holds PanelBase reference)
- **PanelBase**: Control node with title bar, drag/resize handles, close button; state machine (FLOATING/DOCKING/DOCKED/UNDOCKING/FULLSCREEN); animation hooks for T18
- **LayoutPersistence**: serializes/deserializes tree structure, ratios, float positions, and mode preferences to user://layout.json

**Panel State Machine**:

```
FLOATING --> DOCKING --> DOCKED
   ^                      |
   +---- UNDOCKING <------+

FLOATING/DOCKED --> FULLSCREEN --> (restore to previous state)
```

- FLOATING: spawns here with materialize + particle entrance animation, freely draggable, overlays everything
- DOCKING: transition animation playing (panel slides/snaps into tiling tree)
- DOCKED: locked into dwindle tree; programmatic split ratio updates exist, and divider drag UI is deferred
- UNDOCKING: transition animation playing (panel pops out of tree, sibling expands)
- FULLSCREEN: covers viewport with dimmed overlay, remembers previous state for restore

**Data Model — user://layout.json**:
```json
{
  "master_ratio": 0.6,
  "left_zone": {
    "type": "branch",
    "split": "vertical",
    "ratio": 0.5,
    "children": [
      { "type": "leaf", "panel_id": "agent-01-scroll", "mode": "scroll" },
      { "type": "leaf", "panel_id": "agent-02-terminal", "mode": "terminal" }
    ]
  },
  "right_zone": null,
  "floating": [
    { "panel_id": "agent-03-scroll", "mode": "scroll", "position": [800, 200], "size": [400, 300] }
  ]
}
```

**Dependencies**:
- Phase 1 complete (T1-T7) — rebased onto `feature/86-feature-phase-1-skeleton-tower`
- Godot 4.2+ (already in project)
- BridgeManager autoload (already exists — panels subscribe to its signals)

**Risks**:

| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Viewport resolution change breaks existing tower layout (fisheye, floor positioning) | High | T8 must recalibrate TowerManager layout constants for native res |
| BSP tree rebalancing causes visual jank during dock/undock | Medium | Tween all position/size changes; never teleport nodes |
| Input routing conflicts between floating panels and tower (click-through) | Medium | Floating panels consume input via `_gui_input`; tower gets unhandled input only |
| Layout.json corruption on crash mid-save | Low | Atomic write (write to .tmp, rename) |
| Ultrawide aspect ratios break assumptions about left/right zone sizing | Medium | Use viewport-relative percentages, not absolute pixels; test at 21:9 |

---

## Acceptance Scenarios

```gherkin
Feature: T8 Panel Framework
  As a user monitoring AI agents
  I want a tiling panel system
  So that I can organize agent information alongside the magic tower

  Background:
    Given the application is running at native resolution
    And the tower scene is displayed as the master window
    And BridgeManager has connected with at least 2 registered agents

  Rule: Panel spawning

    Scenario: Open panel by clicking agent
      Given agent "agent-01" is visible in the tower
      When the user clicks on agent-01's character
      Then a new panel spawns in FLOATING state
      And the panel plays a materialize + particle entrance animation
      And the panel is freely draggable over the tower

    Scenario: Open panel via panel menu
      When the user opens the panel menu
      Then a list of all registered agents is displayed
      When the user selects "agent-02"
      Then a new panel spawns in FLOATING state for agent-02

    Scenario: Focus existing panel on re-click
      Given agent-01 already has an open panel
      When the user clicks on agent-01's character
      Then the existing panel is focused (brought to front)
      And no duplicate panel is created

  Rule: Docking via edge drag

    Scenario: Dock panel to left edge
      Given a floating panel exists
      When the user drags the panel to the left viewport edge
      Then a dock zone preview appears on the left
      When the user releases the panel in the dock zone
      Then the panel transitions to DOCKED state with animation
      And the tower master zone shrinks to accommodate the left panel zone
      And the master boundary is visible and draggable

    Scenario: Dock second panel to same side (dwindle split)
      Given one panel is docked on the left
      When the user docks a second panel to the left
      Then the left zone splits vertically (top/bottom) via BSP
      And both panels are visible in the left zone

    Scenario: Dock third panel triggers alternating split
      Given two panels are docked on the left (vertical split)
      When the user docks a third panel to the left
      Then the BSP tree alternates to a horizontal split within one of the existing leaves
      And all three panels are visible

  Rule: Master boundary

    Scenario: Resize master boundary
      Given panels are docked on the left
      When the user drags the master boundary divider
      Then the tower and panel zone resize proportionally
      And the boundary snaps to 25%, 50%, or 75% of viewport width

    Scenario: Master boundary clamped to bounds
      Given panels are docked on the left
      When the user drags the master boundary past 80% of viewport width
      Then the boundary stops at 80%
      And the tower does not shrink below 20% width

  Rule: Undocking

    Scenario: Undock panel by dragging off tree
      Given a panel is docked in the left zone
      When the user drags the panel's title bar away from the tiling tree
      Then the panel transitions to FLOATING state with animation
      And the panel's BSP sibling expands to fill the vacated space

    Scenario: Close last panel on a side
      Given only one panel is docked on the left
      When the user closes that panel
      Then the left panel zone disappears
      And the tower master expands to reclaim the space

  Rule: Fullscreen

    Scenario: Toggle fullscreen
      Given a panel exists (floating or docked)
      When the user presses F or double-clicks the title bar
      Then the panel covers the entire viewport
      And a dimmed overlay appears behind the panel
      When the user presses Escape or F again
      Then the panel restores to its previous state (floating or docked)

  Rule: Layout persistence

    Scenario: Layout persists across restart
      Given panels are arranged in a specific layout (docked and floating)
      When the user closes and reopens the application
      Then all panels restore to their saved positions and sizes
      And the master boundary ratio is preserved
      And per-panel mode preferences are preserved

    Scenario: Reset layout to defaults
      Given a custom layout is active
      When the user presses Ctrl+Shift+R
      Then all panels are closed
      And the master boundary resets to 60%
      And layout.json is reset

  Rule: Hotkeys

    Scenario: Hide all panels
      Given panels are visible (floating and/or docked)
      When the user presses Ctrl+\
      Then all panels are hidden
      When the user presses Ctrl+\ again
      Then all panels are restored to their previous state

  Rule: Per-panel mode memory

    Scenario: Mode preference persists
      Given agent-01's panel is open in "scroll" mode
      When the user switches it to "terminal" mode
      And closes and reopens the panel
      Then the panel opens in "terminal" mode
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T8.1 | Update project.godot: native resolution (1080p base, 1440p window), stretch mode, scaling | High | None | pending |
| T8.2 | Recalibrate TowerManager layout constants for native resolution (fisheye scale, floor sizes, camera bounds) | High | T8.1 | pending |
| T8.3 | PanelBase class: Control node with title bar, drag, resize handles, close button, state machine (FLOATING/DOCKING/DOCKED/UNDOCKING/FULLSCREEN) | High | T8.1 | pending |
| T8.4 | DwindleNode + DwindleTree: BSP tree with insert, remove, rebalance, vertical-first alternation | High | None | pending |
| T8.5 | PanelManager: owns left/right DwindleTrees, master boundary divider (draggable, snaps 25/50/75%, clamped 30-80%), dock zone detection | High | T8.3, T8.4 | pending |
| T8.6 | Dock/undock flow: edge drag detection, dock zone preview, animated transitions, sibling expand on close, tower reclaim on zone empty | High | T8.5 | pending |
| T8.7 | TowerManager integration: respond to master boundary changes, reposition/resize tower scene | High | T8.2, T8.5 | pending |
| T8.8 | Fullscreen mode: dimmed overlay, expand/restore, state memory | Med | T8.3 | pending |
| T8.9 | Hotkeys: F (fullscreen), Escape (close/restore), Ctrl+\ (hide/show all), Ctrl+Shift+R (reset layout) | Med | T8.3, T8.8 | pending |
| T8.10 | LayoutPersistence: serialize/deserialize tree + ratios + float positions + mode prefs to user://layout.json | Med | T8.5 | pending |
| T8.11 | Panel open triggers: wire character_clicked signal, panel menu UI | Med | T8.3, T8.5 | pending |
| T8.12 | Per-panel mode memory: store/restore mode preference per agent | Low | T8.10 | pending |
| T8.13 | Animation hooks: virtual methods/signals for T18 themed animations | Low | T8.3 | pending |

---

## Exit Criteria

- [ ] All Must-Have acceptance scenarios pass manually on Linux
- [ ] No regressions on tower rendering, agent characters, floating runes, or navigation
- [ ] Panels dock/undock/resize without visual jank (all transitions tweened)
- [ ] Layout persists correctly across application restart (verified by close/reopen test)
- [ ] All hotkeys functional in both floating and docked states
- [ ] Master boundary snaps correctly at 25/50/75% thresholds
- [ ] BSP tree correctly rebalances on panel close (no empty gaps)
- [ ] Renders correctly at 1080p, 1440p, and 4K resolutions
- [ ] Ultrawide (21:9) does not break layout assumptions

---

## References

- Parent feature: [#88 — Phase 2: Interaction Layer](../../issues/88)
- Epic: [#4 — Pixel Office UI](../../issues/4)
- Task issue: [#109 — T8 Panel framework](../../issues/109)
- Draft PR: [#110](../../pulls/110)
- Phase 1 spec: `specs/pixel-office-ui-spec.md`
- Research MOC: `docs/research/Agentic-Orchestrator-MOC.md`
- Brainstorm session: `.brainstorm/1433058-1775043636/`

---
*Authored by: Clault KiperS 4.6*
