# Feature: Pixel Office UI

## Overview

**User Story**: As an orchestrator operator, I want a pixel-art desktop application where AI agents appear as animated characters at desks so that I can visually monitor agent activity, observe terminal output, and interact with the orchestration system through a delightful retro interface.

**Problem**: CLI-only orchestration lacks visual feedback and makes it hard to monitor multiple agents simultaneously. Without a visual layer, operators must switch between terminal panes manually and have no spatial awareness of agent states.

**Out of Scope**: Orchestrator logic (core), model routing (gateway), raw tmux management (terminal substrate — this feature consumes the substrate's output for display).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | | | [ ] |

---

## Scope

### Must-Have
- **Godot 4 project with pixel-art office tilemap** — a room with desks, one per agent: office renders at low internal resolution (e.g., 320x180) upscaled with nearest-neighbor filtering
- **Agent sprite characters** with animation states mapped to state machine (idle, assigned, working, reporting): sprite visually changes when agent state transitions
- **godot-xterm terminal embedding** — each agent's terminal output displayed in a Godot panel: live shell output from a tmux pane renders in the godot-xterm node (Linux/macOS; display-only on Windows)
- **Agent status overlay** — show agent name, current task, and state as UI elements: clicking an agent shows its status panel
- **Panel layout system** — resizable/dockable panels for terminal views and agent details: user can rearrange panels and layout persists across sessions

### Should-Have
- Palette-restricted rendering (16-64 colors) with optional CRT scanline shader
- Keyboard navigation and shortcuts for panel switching
- Monospace bitmap font (Press Start 2P or similar)

### Nice-to-Have
- Agent pathfinding (A* to desk/bookshelf based on activity type)
- Sound effects for state transitions
- Multi-client view (multiple operators watching same orchestrator instance)

---

## Technical Plan

**Affected Components** (new Godot project):
- `godot/project.godot` — Godot project config, viewport scaling
- `godot/scenes/office.tscn` — main office tilemap scene
- `godot/scenes/agent.tscn` — agent character scene (sprite + animations)
- `godot/scenes/terminal_panel.tscn` — godot-xterm terminal panel
- `godot/scenes/status_overlay.tscn` — agent status UI
- `godot/scripts/office.gd` — office room logic
- `godot/scripts/agent_character.gd` — agent sprite state machine
- `godot/scripts/terminal_manager.gd` — connects to orchestrator, feeds terminal output to godot-xterm
- `godot/scripts/panel_layout.gd` — dockable panel management, layout persistence
- `godot/assets/sprites/` — pixel-art sprite sheets
- `godot/assets/tiles/` — office tilemap tiles

**API Contracts** (data flow):
- Orchestrator (Go/gRPC) -> gRPC client bridge (GDExtension or HTTP) -> agent scenes + terminal panels

**Dependencies**: Godot 4.2+, godot-xterm GDExtension, orchestrator core gRPC API

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| godot-xterm PTY is Linux/macOS only — Windows gets display-only | High | Document; consider SSH output bridge as Windows fallback |
| godot-xterm has no stable v1.0 release | Medium | Pin to specific commit; wrap in abstraction layer |
| gRPC client in GDScript is non-trivial | Medium | Use GDExtension (C++) for gRPC; expose to GDScript via signals |
| Pixel-perfect rendering breaks at non-integer DPI | Medium | Test at multiple resolutions; force integer scaling in project settings |

---

## Acceptance Scenarios

```gherkin
Feature: Pixel Office UI
  As an orchestrator operator
  I want a pixel-art desktop app showing agents as animated characters
  So that I can visually monitor and interact with the orchestration system

  Background:
    Given the Godot application is running
    And the orchestrator core is connected via gRPC

  Rule: Office world renders with agent characters

    Scenario: Office loads with registered agents
      Given the orchestrator has 3 registered agents
      When the application starts
      Then the office tilemap renders with 3 desks
      And each desk has a pixel-art agent character

    Scenario: Agent sprite reflects state machine
      Given agent "agent-01" is in "idle" state
      When the orchestrator transitions "agent-01" to "working"
      Then the agent sprite animation changes from idle to typing

  Rule: Terminal output embedded via godot-xterm

    Scenario: Agent terminal output displays in panel
      Given agent "agent-01" is in "working" state
      When the agent produces terminal output
      Then the output appears in the godot-xterm panel for "agent-01"
      And scrollback is navigable

    Scenario: Display-only mode on unsupported platform
      Given the platform does not support PTY (Windows)
      When the terminal panel renders
      Then output is displayed read-only
      And a notice indicates PTY is unavailable

  Rule: Agent status interaction

    Scenario: Click agent to view status
      Given agent "agent-01" is working on task "refactor-auth"
      When the user clicks on agent-01's character
      Then a status overlay shows: name, current task, state, uptime

  Rule: Panel layout persistence

    Scenario: User rearranges panels and layout persists
      Given the user drags the terminal panel to the right side
      When the application is closed and reopened
      Then the terminal panel is on the right side
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T1   | Godot project scaffold: project.godot, viewport scaling, pixel-perfect settings | High | None | pending |
| T2   | Office tilemap: room with desks, walls, floor tiles | High | T1 | pending |
| T3   | Agent character scene: sprite sheet, animation states (idle, working, reporting) | High | T1 | pending |
| T3.1 | Agent state machine in GDScript: transitions driven by orchestrator events | High | T3 | pending |
| T4   | godot-xterm integration: terminal panel scene, PTY connection | High | T1 | pending |
| T4.1 | Connect terminal panel to orchestrator tmux output (via gRPC stream) | High | T4 | pending |
| T5   | Status overlay: agent name, task, state, uptime on click | High | T3 | pending |
| T6   | Panel layout system: dockable/resizable panels, save/load layout | Med  | T4, T5 | pending |
| T7   | gRPC client: GDExtension or HTTP bridge to orchestrator API | High | None | pending |
| T8   | Pixel-art assets: sprite sheets, tilemap tiles, bitmap font | High | T2, T3 | pending |
| T9   | Integration test: orchestrator state change -> sprite animation update | High | T7 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass manually on Linux/macOS
- [ ] No regressions on orchestrator core or terminal substrate
- [ ] Agent sprite animations match state machine transitions
- [ ] godot-xterm displays live terminal output on Linux/macOS
- [ ] Panel layout persists across application restarts
- [ ] Renders correctly at 1080p and 1440p with integer scaling

---

## References

- Research MOC: `docs/research/Agentic-Orchestrator-MOC.md`
- UI framework evaluation: `docs/research/pixelated-desktop-ui-framework-evaluation.md`
- Competitive landscape: `docs/research/orchestration-research-findings.md`
- Native desktop rendering patterns: `docs/research/patterns/Native-Desktop-Rendering.md`

---
*Authored by: Clault KiperS 4.6*
