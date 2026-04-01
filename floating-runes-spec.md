# Feature: Floating Runes (T6)

## Overview

**User Story**: As an orchestrator operator, I want to see meaningful output lines floating upward from agent characters so that I can glance at the tower and understand what each agent is doing without opening a terminal panel.

**Problem**: After T5, agents have visual presence in the tower but are opaque — you can see their state (idle/working/etc.) but not what they're actually producing. Without an ambient output layer, the tower feels static and operators must click into individual terminal views to understand agent activity.

**Out of Scope**: Full terminal output display (that's T10 — Raw Terminal + PTY), agent interaction menu (T14), sound effects for output events (T21), real sprite art (T22 — this task uses the Phase 1 placeholder characters).

---

## Success Condition

> This feature is complete when significant output lines from active agents float upward as provider-colored, glowing text with keyword highlighting, capped at 5 visible runes per agent, driven by the existing `agent.output` SSE event with hybrid significance filtering.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should the rune glow use a RichTextLabel shader or a custom BBCode effect? | Spec author | [ ] — decide during implementation; shader is simpler but BBCode effect allows per-character control |
| 2 | What specific regex patterns constitute the keyword extraction set? | Spec author | [ ] — define initial set in T6.2, iterate based on real orchestrator output |

---

## Scope

### Must-Have

- **Data model: provider field on AgentData**: `provider: String` field set at registration, used as default rune color source. Acceptance: `BridgeData.AgentData.from_dict` parses `provider` from SSE payload.
- **Data model: provider + significant on AgentOutputChunk**: `provider: String` (per-output override, empty = use agent default) and `significant: bool` (orchestrator hint). Acceptance: `BridgeData.AgentOutputChunk.from_dict` parses both fields.
- **Hybrid significance filtering**: Server-side `significant=true` always shown. Client-side fallback: keyword heuristic (errors, filenames, commands, status transitions), excludes blank/progress/ANSI-only lines, rate-limited to 1 rune per agent per 2 seconds from fallback. Acceptance: only filtered lines spawn runes; chatty agents don't flood the tower.
- **Text processing pipeline**: Strip ANSI codes, timestamps, log-level prefixes. Extract keywords (filenames, error strings, command names, model names) via regex. Truncate to 40 chars preserving keywords. Acceptance: rune text is clean, keywords are bolded, no raw ANSI visible.
- **FloatingRune scene**: `Node2D` + `RichTextLabel` (BBCode enabled). Drift-rise animation: float up at ~8px/sec with sine-wave horizontal drift (amplitude 4px, random phase). Linear opacity fade 1.0 to 0.0 over 7 seconds. Provider-colored glow (text-shadow via shader or BBCode effect) fading proportionally. Acceptance: runes visually float and fade with organic drift.
- **Keyword highlighting**: Extracted keywords rendered bold with brightened provider color. Non-keyword text in base provider color. Acceptance: keywords visually pop within rune text.
- **Rune stack management on AgentCharacter**: Max 5 active runes. On overflow: oldest rune gets accelerated fade (0.3s) then freed. Spawn position centered ~4px above character top edge. Acceptance: never more than 5 visible runes; oldest gracefully exits.
- **Provider color resolution**: `chunk.provider` (non-empty) > `agent.provider` > `"unknown"`. Acceptance: rune color always matches the model that produced the output.
- **No runes on idle agents**: Runes only spawn when `_anim_state != IDLE`. Existing runes finish natural fade on state transition to idle. Acceptance: idle agents have no new rune spawns.
- **Signal wiring**: `BridgeManager.agent_output` > `TowerManager._on_agent_output` > `FloorScene.get_agent_character` > `AgentCharacter.receive_output`. Acceptance: output events flow end-to-end from SSE to visual runes.

### Should-Have

- **Configurable rune cap**: Allow tower config to override the default max-5 runes per agent (for dense vs sparse layouts).
- **Significance filter tuning**: Expose the keyword list and rate-limit interval as constants that can be adjusted without code changes.

### Nice-to-Have

- **Rune click interaction**: Clicking a rune copies its full (untruncated) text to clipboard or opens a tooltip.

---

## Technical Plan

**Affected Components**:

| File | Change |
|:-----|:-------|
| `godot/scripts/models/bridge_data.gd` | Add `provider` to `AgentData`, add `provider` + `significant` to `AgentOutputChunk` |
| `godot/scripts/agents/agent_character.gd` | Add `_provider` field, `receive_output()` method, rune stack management |
| `godot/scripts/tower/tower_manager.gd` | Connect `agent_output` signal, add `_on_agent_output()` routing |
| `godot/scenes/floating_rune.tscn` (new) | Node2D + RichTextLabel scene |
| `godot/scripts/agents/floating_rune.gd` (new) | Drift-rise animation, fade, glow, lifecycle |
| `godot/scripts/agents/rune_filter.gd` (new) | Significance filter + text processing pipeline |

**Data Model Changes**:

`BridgeData.AgentData` — add:
```gdscript
var provider: String = ""
```

`BridgeData.AgentOutputChunk` — add:
```gdscript
var provider: String = ""
var significant: bool = false
```

**Provider Color Map** (constants on `FloatingRune` or shared autoload):

| Provider | Base Color | Keyword Color |
|:---------|:-----------|:-------------|
| `claude` | `#D4AF37` | `#FFD75E` |
| `gemini` | `#5A9BD5` | `#8DC4FF` |
| `openai` | `#00BFA5` | `#4DFFE5` |
| `ollama` | `#F4851E` | `#FFB366` |
| `deepseek` | `#8B5CF6` | `#B794FF` |
| `unknown` | `#888888` | `#BBBBBB` |

**Signal Flow**:
```
BridgeManager.agent_output(chunk: AgentOutputChunk)
  -> TowerManager._on_agent_output(chunk)
    -> lookup _agent_assignments[chunk.agent_id].floor
      -> floor_node.get_agent_character(chunk.agent_id)
        -> AgentCharacter.receive_output(chunk)
          -> RuneFilter.process(chunk) -> {significant, text, keywords}
          -> if significant: spawn FloatingRune
```

**Dependencies**: T5 (merged), BridgeManager `agent_output` signal (exists), AgentCharacter scene (exists).

**Risks**:

| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| RichTextLabel BBCode glow not performant with many agents | Medium | Profile early; fallback to simple modulate if shader is too expensive; max 5 runes x N agents is bounded |
| Keyword regex too aggressive/too conservative | Medium | Start with a conservative set (file extensions, ERROR/WARN, known commands); tune with real output |
| Agent output arrives before character is on active edge | Low | Guard in `receive_output()`: if no live character node, discard silently |
| Provider field missing from existing orchestrator payloads | Medium | Default to `"unknown"` — grey runes still functional, just uncolored |

---

## Acceptance Scenarios

```gherkin
Feature: Floating Runes
  As an orchestrator operator
  I want meaningful output to float above agent characters
  So that I can see what agents are doing at a glance

  Background:
    Given the tower is connected to the orchestrator
    And at least one agent is registered with provider "claude" on the active floor edge

  Rule: Significant output spawns runes

    Scenario: Server-flagged significant output spawns a rune
      Given agent "agent-1" is in "working" state
      When an agent.output SSE event arrives with significant=true and payload "validated config.yaml"
      Then a FloatingRune spawns above agent-1
      And the rune text contains "config.yaml" in bold brightened amber
      And the rune base color is Claude amber (#D4AF37)

    Scenario: Client-filtered output spawns a rune
      Given agent "agent-1" is in "working" state
      When an agent.output SSE event arrives with significant=false and payload "[INFO] 2026-04-01 loaded user_model.py"
      Then the text pipeline strips "[INFO] 2026-04-01" prefix
      And a FloatingRune spawns with cleaned text "loaded user_model.py"
      And "user_model.py" is bold in brightened provider color

    Scenario: Insignificant output is filtered out
      Given agent "agent-1" is in "working" state
      When an agent.output SSE event arrives with significant=false and payload "████████░░ 80%"
      Then no rune spawns (progress bar filtered)

    Scenario: Idle agents produce no runes
      Given agent "agent-1" is in "idle" state
      When an agent.output SSE event arrives with significant=true
      Then no rune spawns

  Rule: Rune visual behavior

    Scenario: Rune drift-rise and fade
      Given a FloatingRune has just spawned
      Then it rises at approximately 8px/sec
      And it drifts horizontally with sine-wave motion (amplitude ~4px)
      And its opacity decreases linearly from 1.0 to 0.0 over 7 seconds
      And the text glow fades proportionally
      And the rune is freed when opacity reaches 0

  Rule: Rune stack management

    Scenario: Stack overflow triggers accelerated fade
      Given agent "agent-1" has 5 visible runes
      When a 6th significant output arrives
      Then the oldest rune's fade accelerates to 0.3 seconds
      And a new rune spawns at the bottom of the stack
      And there are never more than 5 visible runes

  Rule: Provider color resolution

    Scenario: Per-output provider overrides agent default
      Given agent "agent-1" is registered with provider "claude"
      When an agent.output event arrives with provider "gemini"
      Then the rune is tinted Gemini blue (#5A9BD5), not Claude amber

    Scenario: Empty output provider falls back to agent default
      Given agent "agent-1" is registered with provider "ollama"
      When an agent.output event arrives with provider ""
      Then the rune is tinted Ollama warm orange (#F4851E)

    Scenario: Unknown provider renders grey
      Given agent "agent-1" is registered with provider ""
      When an agent.output event arrives with provider ""
      Then the rune is tinted unknown grey (#888888)

  Rule: Client-side rate limiting

    Scenario: Fallback filter rate-limits to 1 rune per 2 seconds
      Given agent "agent-1" is in "working" state
      When 5 agent.output events arrive within 1 second, all with significant=false but matching keywords
      Then only 1 rune spawns (the first matching line)
      And the remaining 4 are discarded by the rate limiter

    Scenario: Server-flagged significant output bypasses rate limit
      Given agent "agent-1" just had a fallback rune 0.5 seconds ago
      When an agent.output event arrives with significant=true
      Then a rune spawns regardless of the rate limit timer
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T6.1 | Data model: add `provider` to `AgentData` and `provider` + `significant` to `AgentOutputChunk` in `bridge_data.gd` | High | None | pending |
| T6.2 | Rune filter: create `rune_filter.gd` — significance check, ANSI/timestamp stripping, keyword extraction, rate limiting | High | T6.1 | pending |
| T6.3 | FloatingRune scene: create `floating_rune.tscn` + `floating_rune.gd` — drift-rise animation, opacity fade, glow, provider color map, keyword BBCode rendering | High | T6.1 | pending |
| T6.4 | AgentCharacter integration: add `_provider` field, `receive_output()`, rune stack management (max 5, accelerated fade on overflow), idle guard | High | T6.2, T6.3 | pending |
| T6.5 | Signal wiring: connect `BridgeManager.agent_output` in `TowerManager`, route to `AgentCharacter.receive_output()` via floor lookup | High | T6.4 | pending |
| T6.6 | Manual integration test: mock SSE output events, verify runes spawn/float/fade/overflow correctly | High | T6.5 | pending |

---

## Exit Criteria

- [ ] All Must-Have acceptance scenarios pass manual verification
- [ ] No regressions on T5 agent character behavior (state transitions, spawn/despawn, click area)
- [ ] Rune stack never exceeds 5 per agent under burst output
- [ ] Idle agents produce zero runes
- [ ] Provider color fallback chain works: chunk > agent > unknown
- [ ] Text pipeline produces clean, truncated, keyword-highlighted output from raw payloads
- [ ] Performance: 10 agents with 5 runes each (50 simultaneous runes) runs at 60fps

---

## References

- Parent feature: [#86 — Phase 1: Skeleton Tower](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/86)
- Depends on: [#102 — T5: Agent character scene](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/102) (merged as PR #105)
- Issue: [#104 — T6: Floating runes](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/104)
- PR: [#107 — chore: T6 — Floating runes](https://github.com/Kiriketsuki/agenKic-orKistrator/pulls/107)
- Provider color brainstorm: `.brainstorm/3720598-1775023135/content/provider-colors.html`
- Rune text style brainstorm: `.brainstorm/3720598-1775023135/content/keyword-highlight-color.html`

---

*Authored by: Clault KiperS 4.6*
