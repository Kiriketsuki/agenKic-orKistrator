# Feature: Basic Tower Navigation

## Overview

**User Story**: As a user viewing the pixel-art tower, I want to scroll between floors and rotate edges with smooth animations so that navigating the tower feels polished and responsive.

**Problem**: Vertical scroll snaps instantly with no easing, the camera has no smooth follow or bounds clamping, and W/S/Arrow keyboard bindings are not wired up — only mouse wheel works. Navigation feels jarring and incomplete.

**Out of Scope**: Minimap and floor tabs (T12 — Phase 2), free camera pan/drag, A* agent pathfinding.

---

## Success Condition

> This feature is complete when the user can scroll between tower floors with smooth spring-eased camera transitions, elastic overscroll at bounds, and queued keyboard/mouse input — with no regression to edge rotation or zoom.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Exact spring damping / overshoot parameters — deferred to playtest tuning in T7.6 | brainstorm | [ ] |

---

## Scope

### Must-Have
- **Discrete vertical scroll**: W/S, Up/Down, mouse wheel each jump one floor with spring overshoot easing (`TRANS_BACK, EASE_OUT`, 0.3s duration) — acceptance: camera tweens to target floor, focused floor updates scale/opacity
- **Input queue (max 3)**: rapid presses chain smoothly up to 3 queued jumps; excess inputs silently dropped until queue drains — acceptance: 3 rapid W presses chain without jarring stops; 5 rapid presses only execute 3
- **Camera tween follow**: Camera2D tweens to focused floor Y position (tween-driven, not `_process` lerp) — acceptance: camera smoothly tracks focus changes
- **Elastic overscroll**: scrolling past roof/base rubber-bands ~0.5x `FLOOR_SPACING` then springs back — acceptance: overscroll visually overshoots then returns; focus index does not change
- **Input actions**: `scroll_up` and `scroll_down` added to `project.godot` with W, S, Up, Down bindings — acceptance: all four keys trigger scroll

### Should-Have
- **Animated fisheye transitions**: scale and opacity changes in `_apply_fisheye_layout` animate smoothly during scroll instead of snapping instantly

### Nice-to-Have
- None for this task

---

## Technical Plan

**Affected Components**:
- `godot/scripts/tower/tower_manager.gd` — main changes (scroll system, camera follow, overscroll)
- `godot/project.godot` — input action definitions

**Data Model Changes**: None

**API Contracts**: None (Godot-only, no orchestrator changes)

**Dependencies**: T4 floor scene (already merged)

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Spring overshoot feels wrong at default params | Medium | T7.6 dedicated tuning task; expose constants for easy adjustment |
| Input queue interacts badly with elastic overscroll (queue drains into bounds) | Low | Overscroll is a separate code path triggered only when focus is already at bounds; queue items that would exceed bounds trigger overscroll instead of queueing |
| Tween conflicts if scroll + rotation fire simultaneously | Low | Scroll tween targets camera Y; rotation tween targets floor X — independent axes, no conflict |

---

## Acceptance Scenarios

```gherkin
Feature: Basic Tower Navigation
  As a user viewing the pixel-art tower
  I want to scroll between floors and rotate edges with smooth animations
  So that navigating the tower feels polished and responsive

  Background:
    Given the tower has 4+ floors loaded
    And the camera is focused on a middle floor

  Rule: Discrete vertical scrolling with spring easing

    Scenario: Scroll down one floor via keyboard
      When the user presses S or Down
      Then the camera tweens to the next floor below with spring overshoot
      And the tween duration is ~0.3s
      And the new floor becomes the focused floor (scale 1.0, full opacity)

    Scenario: Scroll up one floor via mouse wheel
      When the user scrolls mouse wheel up
      Then the camera tweens to the next floor above with spring overshoot

  Rule: Input queueing with chain limit

    Scenario: Rapid input chains smoothly
      When the user presses W three times in quick succession
      Then the camera chains 3 floor jumps smoothly without jarring stops

    Scenario: Excess input beyond queue limit is dropped
      When the user presses W five times in quick succession
      Then only the first 3 presses are queued
      And the remaining 2 are silently dropped

  Rule: Elastic overscroll at tower bounds

    Scenario: Overscroll past top floor rubber-bands back
      Given the camera is focused on the top floor
      When the user presses W
      Then the camera overshoots slightly above the top floor (~0.5x FLOOR_SPACING)
      And the camera springs back to the top floor position
      And the focused index remains at the top floor

    Scenario: Overscroll past bottom floor rubber-bands back
      Given the camera is focused on the bottom floor
      When the user presses S
      Then the camera overshoots slightly below the bottom floor
      And the camera springs back to the bottom floor position
      And the focused index remains at the bottom floor

  Rule: Edge rotation is per-focused-floor only

    Scenario: Q/E rotates only the focused floor
      When the user presses Q
      Then the focused floor slides to the next edge (0.3s animation)
      And all other floors remain on their current edge

  Rule: Zoom is unchanged

    Scenario: Ctrl+wheel zooms the camera
      When the user holds Ctrl and scrolls mouse wheel
      Then the camera zoom changes (not floor focus)
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T7.1 | Add `scroll_up`/`scroll_down` input actions to `project.godot` (W, S, Up, Down) | High | None | pending |
| T7.2 | Replace `_scroll_focus()` with tween-driven spring scroll in `tower_manager.gd` | High | T7.1 | pending |
| T7.3 | Implement input queue with max 3 chained jumps | High | T7.2 | pending |
| T7.4 | Add elastic overscroll at roof/base bounds | High | T7.2 | pending |
| T7.5 | Animate fisheye scale/opacity transitions during scroll (should-have) | Should | T7.2 | pending |
| T7.6 | Manual playtest — tune spring damping, overshoot amount, queue feel | High | T7.2-T7.4 | pending |

---

## Exit Criteria

- [ ] All Must-Have acceptance scenarios pass manual testing
- [ ] No regressions on edge rotation (Q/E) or zoom (Ctrl+wheel)
- [ ] Spring easing parameters tuned to feel good (confirmed by user in T7.6)
- [ ] Camera never permanently escapes tower bounds
- [ ] Input queueing handles rapid input without jitter or dropped-then-ghost inputs

---

## References

- Issue: #106 ([Task]: T7 — Basic navigation)
- PR: #108 (chore: T7 — Basic navigation)
- Parent feature: #86 (Phase 1 — Skeleton Tower)
- Depends on: T4 (floor scene, merged as PR #103)
- Full navigation (minimap + floor tabs): T12 (Phase 2)

---
*Authored by: Clault KiperS 4.6*
