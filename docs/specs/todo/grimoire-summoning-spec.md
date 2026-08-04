# Feature: Grimoire Summoning

## Overview

**User Story**: As the tower keeper, I want to drag a provider sigil from the Grimoire onto a tower floor so that I choose who spawns and where they sit.

**Problem**: Summoning is a blind button press. The keeper cannot choose placement, floors hold one agent each, and the provider list is hardcoded in the GUI.

**Out of Scope**: New provider adapters (gemini and others are backend work later; the roster endpoint makes them appear automatically). The archmage itself (tower-post feature). Task submission changes.

---

## Success Condition

> This feature is complete when a sigil dragged from the Grimoire onto a real tower floor spawns that provider's agent on that floor, and the sigil roster with its defaults is user-configurable.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Desk capacity per floor | brainstorm | [x] 4, a configurable constant |
| 2 | Ground floor role | keeper | [x] Reserved for the future archmage, never a drop target |

---

## Scope

### Must-Have
- `GET /api/providers`: the bridge serves the spawn-kind roster (sim, claude, codex, opencode, pi) with display name and accent color. A new adapter appears in the GUI with no GUI change.
- Grimoire flyout shows the sigil grid built from the roster, filtered and ordered by user config.
- Drag-out enters placement mode on the real tower: the flyout dims, the sigil follows the cursor over the world.
- Real FloorScene floors highlight under the cursor with desk counts ("floor 3, 2/4"). Full floors show a reject state and shake on drop. The gap above the top floor offers "new floor".
- Floor 0 (ground) is reserved: never a drop target, never lists desks. It is the future home of the orKistrator archmage.
- Drop calls `POST /api/agents/spawn` with a new optional `floor` field. Escape or a drop outside any zone cancels.
- Shared floors: the bridge stores agent-to-floor assignment. `EdgeLayout` renders up to 4 desks per floor, per the original floor spec.
- Sigil config page, reachable from the Grimoire flyout and the title GRIMOIRE entry: provider toggles, order, per-provider defaults (tier, name pool). Persisted to `user://orki_settings.cfg`.

### Should-Have
- Spawn lands with a brief summoning flash on the assigned desk.

### Nice-to-Have
- Drag an existing agent's desk to another floor to relocate them.

---

## Technical Plan

**Affected Components**:
- Go: `internal/httpbridge/handlers.go` (`handleSpawnAgent` at :576 gains the floor field, new `handleListProviders`), `internal/httpbridge/bridge.go` (floor-assignment registry beside names and providers, route table at :124), `cmd/orchestrator/main.go` (spawner wiring at :116)
- Godot: `godot/scripts/ui/grimoire_flyout.gd` (new), `godot/scripts/ui/sigil_config.gd` (new), `godot/scripts/tower/floor_scene.gd` and `godot/scripts/tower/edge_layout.gd` (multi-desk render, drop-zone highlight API), `godot/scripts/tower/tower_manager.gd` (floor assignment from agent events), `godot/scripts/autoload/bridge_manager.gd` (`get_providers()`, spawn with floor)

**Data Model Changes**:
- Bridge in-process floor registry: agent id to floor index. Resets on restart, same policy as names and providers.
- Spawn request JSON gains optional `floor` (int). Agent list JSON gains `floor`.
- `user://orki_settings.cfg` gains a `[sigils]` section: enabled list, order, per-provider tier and name pool.

**API Contracts**:
- `GET /api/providers` — returns `{providers: [{kind, display, accent}]}`.
- `POST /api/agents/spawn` — existing contract plus optional `floor`. Omitted floor keeps today's auto behavior. A full floor returns 409.

**Dependencies**: F2 Orb Flock (the Grimoire flyout lives in it). This feature supersedes the Phase 3 FloorScene API freeze where the shared-floor work requires changes.

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| FloorScene changes ripple into Phase 3 visuals (morph, particles) | Medium | Keep the morph and particle entry points intact. Run the five headless suites after every FloorScene edit |
| Drop hit-testing fights the camera and viewport scaling | Medium | Hit-test in world space through TowerManager, with a headless test for the mapping math |
| 409 full-floor race when two spawns target one floor | Low | The bridge validates capacity at spawn time and returns 409. The GUI shakes and keeps placement mode |

---

## Acceptance Scenarios

```gherkin
Feature: Grimoire Summoning
  As the tower keeper
  I want drag-to-floor summoning
  So that I choose who spawns and where

  Background:
    Given the tower runs with floors 1 and 2, and floor 2 holds 4 agents

  Rule: Sigils come from the bridge roster

    Scenario: Roster drives the grid
      When the Grimoire flyout opens
      Then it shows one sigil per enabled provider from GET /api/providers
      And disabled providers from the config stay hidden

  Rule: Drop placement is real and validated

    Scenario: Drop on a floor with space
      When the keeper drags the claude sigil over floor 1
      Then floor 1 highlights with "1/4"
      When the keeper releases
      Then a claude agent spawns assigned to floor 1
      And EdgeLayout shows two desks on floor 1

    Scenario: Drop on a full floor
      When the keeper drags a sigil over floor 2
      Then floor 2 shows the reject state
      When the keeper releases
      Then the floor shakes and no agent spawns
      And placement mode stays active

    Scenario: Drop above the tower
      When the keeper drops a sigil on the gap above the top floor
      Then a new floor is created above floor 2
      And the agent spawns on it

    Scenario: Ground floor refuses
      When the keeper drags a sigil over floor 0
      Then floor 0 shows no drop zone

    Scenario: Cancel
      When the keeper presses Escape mid-drag
      Then placement mode ends with no spawn

  Rule: Defaults come from the config

    Scenario: Per-provider defaults apply
      Given the config sets the claude tier to "adept"
      When a claude sigil drop spawns an agent
      Then the spawn request carries tier "adept"
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T1 | Go: `GET /api/providers` plus httptest coverage | High | None | pending |
| T2 | Go: floor registry, spawn `floor` field, capacity check with 409, floor in agent JSON | High | None | pending |
| T3 | Godot: EdgeLayout multi-desk render (cap 4) plus desk-count API on FloorScene | High | None | pending |
| T4 | Godot: Grimoire flyout sigil grid from the roster | High | F2, T1 | pending |
| T5 | Godot: placement mode — drag-out, world hit-testing, highlight, reject, new-floor zone, ground-floor reserve | High | T3, T4 | pending |
| T6 | Godot: drop calls spawn with floor, cancel paths, summoning flash | High | T2, T5 | pending |
| T7 | Godot: sigil config page plus `user://orki_settings.cfg` persistence, linked from title GRIMOIRE | High | T4 | pending |
| T8 | Headless tests: hit-test math, roster parsing, config round-trip | High | T5, T7 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI
- [ ] No regressions on related features
- [ ] API contracts match implementation
- [ ] Five Godot headless suites and `go test -race ./...` stay green
- [ ] Live GUI verification by the keeper

---

## References

- Related specs: `orb-flock-spec.md`, `floor-scene-tower-layout-spec.md` (original shared-floor model), `docs/specs/todo/tower-post-spec.md` (future archmage)
- Mockups: `.brainstorm/1776334-1785821252/content/f1-f3-v2.html`

---
*Authored by: Clault KiperF 5.0*
