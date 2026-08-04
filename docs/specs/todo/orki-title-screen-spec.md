# Feature: OrKi Title Screen

## Overview

**User Story**: As the tower keeper, I want the app to boot into an OrKi title screen so that entry, settings, and exit have a proper front door.

**Problem**: The app boots straight into the tower scene. There is no entry point, no place for settings, and no branded identity. The project still presents itself as agenKic-orKistrator.

**Out of Scope**: The settings page content (the GRIMOIRE entry opens a placeholder until F3 ships the sigil config page). Any repo, Go module, or binary rename. Save slots or profiles.

---

## Success Condition

> This feature is complete when the app opens on the title screen, ENTER THE TOWER fades into the tower, DEPART quits, and the window title says OrKi.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | None. | | [x] |

---

## Scope

### Must-Have
- Split layout: the gold OrKi wordmark with the subtitle sits on the left half. The menu sits on the right half.
- Menu entries ENTER THE TOWER, GRIMOIRE, DEPART: keyboard (arrows plus Enter) and mouse both work. The focused entry shows the teal border and glow.
- ENTER THE TOWER fades to black, then loads the tower scene.
- DEPART quits the app.
- GRIMOIRE opens a placeholder settings panel with a back action.
- Bridge status rune, top-right: teal diamond plus "orchestrator: connected" when BridgeManager holds a live SSE connection. A dim gray state when it does not. The label polls the existing connection state.
- Version label from the repo VERSION source, bottom of the left half.
- Display rename: `project.godot` application name and the window title become OrKi.

### Should-Have
- Ambient particle drift behind the wordmark, gl_compatibility safe.

### Nice-to-Have
- A slow tower silhouette parallax in the background.

---

## Technical Plan

**Affected Components**:
- `godot/scenes/title_screen.tscn` (new) plus `godot/scripts/ui/title_screen.gd` (new)
- `godot/project.godot`: main scene switches to the title screen, application name becomes OrKi
- `godot/scenes/main.tscn`: loaded by the title screen on enter, no longer the boot scene
- `godot/scripts/autoload/bridge_manager.gd`: read-only use of connection state for the rune

**Data Model Changes**: None.

**API Contracts**: None. The rune reads the existing BridgeManager state.

**Dependencies**: None. F1 is independent of every other feature in the epic.

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Autoloads assume the tower scene exists at boot | Medium | Audit autoload `_ready` paths for hard references to main.tscn nodes before the switch |
| Headless suites boot the main scene | Medium | Point the suites at their scenes directly, not at the project main scene |

---

## Acceptance Scenarios

```gherkin
Feature: OrKi Title Screen
  As the tower keeper
  I want a proper entry screen
  So that the app has a front door

  Background:
    Given the app starts

  Rule: The title screen is the boot scene

    Scenario: Boot lands on the title
      When the app finishes loading
      Then the title screen shows the OrKi wordmark on the left
      And the menu shows ENTER THE TOWER, GRIMOIRE, DEPART on the right
      And the window title is "OrKi"

    Scenario: Enter the tower
      Given the title screen has focus on ENTER THE TOWER
      When the keeper presses Enter
      Then the screen fades out
      And the tower scene loads

    Scenario: Depart quits
      When the keeper activates DEPART
      Then the app exits cleanly

  Rule: The rune reflects bridge state

    Scenario: Bridge reachable
      Given the orchestrator bridge accepts the SSE connection
      Then the rune shows the teal connected state

    Scenario: Bridge down
      Given no orchestrator listens on the bridge port
      Then the rune shows the dim disconnected state
      And ENTER THE TOWER still works
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T1 | Build `title_screen.tscn` layout: split halves, wordmark, menu, rune, version | High | None | pending |
| T2 | `title_screen.gd`: focus handling, keyboard and mouse activation, fade transition into main.tscn | High | T1 | pending |
| T3 | Wire the rune to BridgeManager connection state | High | T2 | pending |
| T4 | Switch `project.godot` main scene and application name to OrKi | High | T2 | pending |
| T5 | Placeholder GRIMOIRE panel with back action | Med | T2 | pending |
| T6 | Headless test: menu focus order and activation logic | High | T2 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI
- [ ] No regressions on related features
- [ ] Five Godot headless suites and `go test -race ./...` stay green
- [ ] Live GUI verification by the keeper

---

## References

- Related specs: `orb-flock-spec.md`, epic mockups in `.brainstorm/1776334-1785821252/content/`
- Tickets: N/A (issue created at implementation start)

---
*Authored by: Clault KiperF 5.0*
