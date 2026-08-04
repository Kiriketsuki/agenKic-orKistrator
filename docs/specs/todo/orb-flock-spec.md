# Feature: Orb Flock HUD

## Overview

**User Story**: As the tower keeper, I want the HUD controls as three draggable orbs so that the screen stays clear and controls feel physical.

**Problem**: The summon bar is a static row of flat buttons. It occupies fixed screen space, scales poorly as controls grow, and has no place for panels or power actions.

**Out of Scope**: The content of the Grimoire, Panels, and Power flyouts (F3, F4, F5 fill them). Dismissing orbs (the three orbs are permanent). Agent-as-bubble representation.

---

## Success Condition

> This feature is complete when three orbs can be dragged anywhere, flicked with momentum, snap to the nearest screen edge, stack when docked, and tap-expand into a row with the active orb's flyout.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | None. | | [x] |

---

## Scope

### Must-Have
- Three permanent orbs: Grimoire (gold `#c9a227`), Panels (teal `#6fb8a8`), Power (ember `#a4443a`), circular, with rune glyphs and a soft glow.
- Drag: the flock follows the pointer with a chained trail. Release springs the flock to the nearest screen edge.
- Flick: release velocity carries the flock with momentum. Top and bottom bounces keep a configurable ricochet fraction.
- Dock stacking: docked orbs overlap into a stack with the lead orb on top.
- Tap expands the flock into a horizontal row near the top of the screen. The active orb's flyout panel opens below the row. Tapping another orb switches the flyout.
- Flyout API: each orb registers a Control as its panel. F3, F4, and F5 plug in without OrbFlock changes.
- Escape or a click outside the row collapses the flock back to its dock.
- Hotkey gating: while a flyout is open or a drag is live, PanelManager hotkeys stay suspended, following the `KeyPassthrough.hover_active` pattern.
- Physics math lives in a pure helper script with headless tests.

### Should-Have
- Reduced-motion setting: springs become short fades.

### Nice-to-Have
- Idle bob animation on the docked stack.

---

## Technical Plan

**Affected Components**:
- `godot/scripts/ui/orb_flock.gd` (new): flock state machine (docked, dragging, flying, open)
- `godot/scripts/ui/orb.gd` (new): one orb Control, glyph, glow, press handling
- `godot/scripts/ui/orb_physics.gd` (new): pure math for spring, momentum, ricochet, nearest-edge
- `godot/tests/orb_physics_test.gd` (new): headless assertions
- `godot/scenes/main.tscn`: OrbFlock replaces the SummonBar node
- `godot/scripts/ui/summon_bar.gd`: retired once F3 lands the Grimoire flyout. Until then the Grimoire flyout hosts the old spawn buttons as an interim panel.
- `godot/scripts/panel_manager.gd`: hotkey gate check extends to the flock-open state

**Data Model Changes**: None.

**API Contracts**: None. Pure GUI.

**Dependencies**: None upstream. F3, F4, and F5 depend on this component. Inspiration: the bubbles library behavior model (drag, edge snap, fling, expand).

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Physics feel wrong at pixel-scale resolutions | Medium | Tune in the live GUI with the keeper. Constants live in one config block |
| Input conflicts with panel drag and key passthrough | Medium | Route all orb input through `_gui_input`, reuse the hover_active gate, add a headless test for the gate |
| New class_name scripts fail headless parse before an editor rescan | High | Run `godot --headless --editor --path . --quit` once after adding scripts |

---

## Acceptance Scenarios

```gherkin
Feature: Orb Flock HUD
  As the tower keeper
  I want draggable orb controls
  So that the screen stays clear

  Background:
    Given the tower scene is loaded with the orb flock docked at the right edge

  Rule: The flock moves physically

    Scenario: Drag and snap
      When the keeper drags the lead orb to mid-screen and releases with low velocity
      Then the flock springs to the nearest screen edge
      And the orbs stack at the release height

    Scenario: Flick with ricochet
      When the keeper flicks the flock upward with high velocity
      Then the flock keeps momentum after release
      And a top-edge bounce keeps the configured ricochet fraction of its speed

  Rule: Tap opens, outside closes

    Scenario: Expand and switch
      When the keeper taps the docked stack
      Then the flock opens into a row
      And the lead orb's flyout shows below the row
      When the keeper taps the Power orb
      Then the flyout switches to the Power panel

    Scenario: Collapse
      Given the flock is open
      When the keeper presses Escape
      Then the flock collapses back to its dock
      And PanelManager hotkeys work again

    Scenario: Hotkeys gated while open
      Given the flock is open
      When the keeper presses a panel hotkey
      Then no panel action fires
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T1 | `orb_physics.gd`: spring, momentum, ricochet, nearest-edge math | High | None | pending |
| T2 | `orb_physics_test.gd`: headless assertions for T1 | High | T1 | pending |
| T3 | `orb.gd` visual: circle, glyph, glow, press states | High | None | pending |
| T4 | `orb_flock.gd`: state machine, drag input, dock stacking | High | T1, T3 | pending |
| T5 | Expand and collapse: row layout, flyout registration API, switch logic | High | T4 | pending |
| T6 | Hotkey gating plus Escape and click-outside handling | High | T5 | pending |
| T7 | Mount in main.tscn, interim Grimoire flyout hosts old spawn buttons | High | T5 | pending |
| T8 | Live GUI tuning pass with the keeper | High | T7 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI
- [ ] No regressions on related features
- [ ] Five Godot headless suites (plus the new orb suite) and `go test -race ./...` stay green
- [ ] Live GUI verification by the keeper

---

## References

- Related specs: `grimoire-summoning-spec.md`, `power-controls-spec.md`, `panels-orb-restyle-spec.md`
- Behavior reference: https://github.com/githyperplexed/bubbles
- Mockups: `.brainstorm/1776334-1785821252/content/feature-mockups.html`

---
*Authored by: Clault KiperF 5.0*
