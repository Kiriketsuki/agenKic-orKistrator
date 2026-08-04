# Feature: Panels Orb and Restyle

## Overview

**User Story**: As the tower keeper, I want one Panels orb that manages every panel and a consistent button style so that the interface feels like one crafted system.

**Problem**: Panel access is scattered across hotkeys and sprite clicks with no overview. The old flat buttons clash with the new orb visual language.

**Out of Scope**: New panel types. Terminal content changes inside the spell scroll. The orb component itself (F2).

---

## Success Condition

> This feature is complete when the Panels flyout lists live agents, toggles the quest board and minimap, saves and restores layout presets, and legacy buttons use the bordered-glow style.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | None. | | [x] |

---

## Scope

### Must-Have
- Panels flyout with a live agent list from BridgeManager state: name, provider accent, state. Tapping a row opens or focuses that agent's spell scroll through PanelManager.
- Quest board toggle and minimap/compass toggle in the flyout, mirroring the current hotkey actions.
- Layout presets: GRID and FOCUS built in, plus named custom saves of the current panel arrangement. Persisted to `user://orki_settings.cfg` under `[layouts]`. Restore replays panel positions and sizes.
- Global Theme resource with the bordered-glow button language (dark fill, 1px accent border, glow on hover and focus, letter-spaced caps). Applied to the quest board, dialogs, scroll chrome, and every remaining legacy button.

### Should-Have
- The agent row shows a small activity spark when its scroll has fresh output.

### Nice-to-Have
- Drag a preset name onto the row to reorder presets.

---

## Technical Plan

**Affected Components**:
- `godot/scripts/ui/panels_flyout.gd` (new): agent list, toggles, preset UI
- `godot/scripts/panel_manager.gd`: open-or-focus by agent id, capture and apply arrangement (the existing restore_rect serialization from T8 is the base)
- `godot/themes/orki_theme.tres` (new): the bordered-glow Theme resource
- Existing UI scenes adopt the theme: quest board, agent context menu dialogs, status overlays, scroll chrome
- `godot/scripts/autoload/bridge_manager.gd`: read-only agent state for the list

**Data Model Changes**: `user://orki_settings.cfg` gains a `[layouts]` section: preset name to serialized panel arrangement.

**API Contracts**: None. Pure GUI over existing state.

**Dependencies**: F2 Orb Flock (hosts the flyout). Shares `user://orki_settings.cfg` with F3.

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Theme adoption breaks clip-path-style custom panel chrome | Medium | Apply the theme scene by scene with a live GUI check after each |
| Preset restore fights the floaty panel animations | Medium | Restore through PanelManager's existing rect serialization path, then re-enable animation |

---

## Acceptance Scenarios

```gherkin
Feature: Panels Orb and Restyle
  As the tower keeper
  I want one panel control surface
  So that the interface feels like one system

  Background:
    Given the tower runs with two live agents

  Rule: The flyout is the panel hub

    Scenario: Open a scroll from the list
      When the keeper opens the Panels flyout and taps agent A's row
      Then agent A's spell scroll opens and takes focus

    Scenario: Toggles mirror hotkeys
      When the keeper taps QUEST BOARD
      Then the quest board shows, identical to the hotkey path

  Rule: Presets round-trip

    Scenario: Save and restore a layout
      Given two scrolls are open in a custom arrangement
      When the keeper saves the preset "review"
      And rearranges the panels
      And restores "review"
      Then every panel returns to its saved position and size

    Scenario: Restore with a missing agent
      Given preset "review" references a banished agent
      When the keeper restores "review"
      Then surviving panels restore and the missing one is skipped without error

  Rule: One button language

    Scenario: Legacy surfaces adopt the theme
      When the quest board opens
      Then its buttons use the bordered-glow style from orki_theme.tres
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T1 | `orki_theme.tres`: bordered-glow button, panel, and label styles | High | None | pending |
| T2 | Panels flyout: agent list with provider accents and states | High | F2 | pending |
| T3 | Open-or-focus wiring through PanelManager, quest board and minimap toggles | High | T2 | pending |
| T4 | Layout presets: capture, save, restore, config persistence | High | T3 | pending |
| T5 | Theme adoption pass across legacy UI, scene by scene with live checks | Med | T1 | pending |
| T6 | Headless tests: preset serialization round-trip, missing-agent restore | High | T4 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI
- [ ] No regressions on related features
- [ ] Five Godot headless suites and `go test -race ./...` stay green
- [ ] Live GUI verification by the keeper

---

## References

- Related specs: `orb-flock-spec.md`, `grimoire-summoning-spec.md` (shared settings file)
- Mockups: `.brainstorm/1776334-1785821252/content/feature-mockups.html` (F5 section)

---
*Authored by: Clault KiperF 5.0*
