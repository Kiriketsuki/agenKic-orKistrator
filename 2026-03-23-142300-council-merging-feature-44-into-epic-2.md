## Adversarial Council — Merge PR #46: Feature #44 (F2) into Epic #2

> Convened: 2026-03-23T14:23:00 | Advocates: 2 | Critics: 1 | Rounds: 4/4 | Motion type: CODE

### Motion

Merge PR #46 — feature/44 (Command Injection & Output Capture, F2) into epic/2-implement-terminal-substrate-layer. PR changes 15 files (+1365 / -131). Key changes since prior council (which returned CONDITIONAL FOR): merge conflicts resolved, compile-time interface assertion added, SCROLLTEST-LINE-401 boundary assertion added, PaneCount renamed to WindowCount, centralized error mapping via parseTmuxError, context.Context threaded through all Substrate methods, new TestSendCommand_SpecialCharacters and TestCaptureOutput_ScrollbackMath tests, 4 feature spec files added.

### Prior Council Context

Prior council returned CONDITIONAL FOR with three conditions:
1. Resolve merge conflicts — RESOLVED (PR is MERGEABLE/CLEAN)
2. Add compile-time interface assertion — ADDED: `internal/terminal/tmux.go:11`
3. Add SCROLLTEST-LINE-401 boundary assertion — ADDED: `internal/terminal/tmux_capture_test.go:110-112`

All three conditions were satisfied before this council convened.

### Advocate Positions

**ADVOCATE-1**: FOR unconditionally. All three prior conditions met at specific citations. Implementation is correct: `context.Context` threaded through all six Substrate methods (`substrate.go:12-31`) via `exec.CommandContext` at `tmux.go:31`; error mapping centralized in `parseTmuxError` (`tmux_errors.go:38-53`) with no inline string matching remaining; `WindowCount` rename (`types.go:18`, `tmux.go:62`) resolves the prior semantic mismatch; `SendCommand` passes `"Enter"` as a separate argument (`tmux_command.go:14`) — the correct tmux idiom for avoiding special-character interpretation. Integration test at `integration_test.go:15-92` validates the full lifecycle: SpawnSession → SendCommand → CaptureOutput → SplitPane → ListSessions → DestroySession. Conceded Defects 3 and 4 as quality gaps; maintained both are non-blocking.

**ADVOCATE-2**: FOR unconditionally. Reinforced the compile-time assertion argument. Key citation on the determinative Defect 1 dispute: `tmux_capture.go:19` comment states `// -S  start-line: negative offset counts back from the visible pane top`, confirming that `-S -100` captures 100 scrollback lines above the visible pane top plus the 50-row visible area (set at `tmux.go:49` with `-y 50`), yielding approximately 150 content rows — not 100 from the bottom as CRITIC-1's model assumed. The LINE-401 boundary is therefore inside the capture window. Granted Defect 2(b) as a real edge case but characterized failure mode as conservative. Conceded Defect 4 as minor.

### Critic Positions

**CRITIC-1**: Opened AGAINST with four defects, all with file:line citations. Moved to CONDITIONAL FOR after Defect 1 was resolved on the merits and Defects 2–4 were reclassified as tracked follow-up items.

- **Defect 1** (LINE-401 boundary flakiness): Fully conceded after ADVOCATE-2 cited the code's own comment at `tmux_capture.go:19`. CRITIC-1's original arithmetic assumed `-S -N` captured N lines from the bottom; the correct model anchors `-S` to the visible pane top, making the effective window ~150 lines. LINE-401 sits well inside that window.
- **Defect 2** (ListSessions layering violation): Version-specific claim withdrawn. Structural concern maintained as follow-up: `parseTmuxError` (`tmux_errors.go:38-53`) has no case for "no server running," so `ListSessions` does secondary string matching on the already-wrapped error at `tmux_layout.go:19-24`. This violates the design intent of centralizing error translation. Reclassified: tracked follow-up for F3, not a merge condition.
- **Defect 3** (TestSendCommand_SpecialCharacters false confidence): Narrowed from "false confidence" to a naming/documentation concern. The test correctly verifies that `send-keys` accepts the keystroke sequences at the tmux API layer; but the test name implies shell-side execution verification that the test does not perform. Reclassified: doc comment concern, not a blocker.
- **Defect 4** (TestCaptureOutput_InvalidLines unnecessary skip): Maintained. `ErrInvalidLines` fires at `tmux_capture.go:14-16` before any tmux subprocess call, so the test at `tmux_capture_test.go:11-24` has no dependency on tmux presence. The `NewTmuxSubstrate()` call that gates the skip is unnecessary for this validation path. Conceded by both advocates; reclassified: minor test quality issue, not a merge condition.

### Questioner Findings

1. **CRITIC-1's tmux 3.2+ error string claim** (Defect 2): probed for citation — "Which tmux version specifically emits this string, and under what condition?" CRITIC-1 conceded: "I cannot cite a specific tmux version or changelog entry for that precise message. The '3.2+' attribution was not from a documented source." Marked: **raised but unsubstantiated**. The structural argument for Defect 2 was maintained independently.

2. **ADVOCATE-1's CI prompt claim** (Defect 1): probed — "How do you know CI environments use a single-character prompt?" ADVOCATE-1 conceded: "I concede the claim was ungrounded." Marked: **raised but unsubstantiated**. Rendered moot by CRITIC-1's full concession of Defect 1 on separate grounds (capture-pane -S arithmetic).

3. **ADVOCATE-2's capture-pane -S semantics claim** (Defect 1): probed — "Does `-S -100` yield 100 scrollback lines plus the visible area, or 100 total?" ADVOCATE-2 cited `tmux_capture.go:19` comment as primary evidence; CRITIC-1 reviewed the same citation and accepted the correction. Marked: **substantiated** — resolves Defect 1.

### Key Conflicts

- **`capture-pane -S -100` semantics** — RESOLVED. The flag anchors to the visible pane top (not the bottom), so the effective capture window is approximately 150 lines (100 scrollback + 50 visible rows from `tmux.go:49`). CRITIC-1's original model was inverted; they conceded on the merits after ADVOCATE-2 cited the code's own comment.
- **tmux 3.2+ "error connecting to" error format** — RESOLVED. Claim withdrawn by CRITIC-1 as unsubstantiated. The structural observation about `ListSessions`'s two-layer error matching stands independently.

### Concessions

- CRITIC-1 conceded Defect 1 (LINE-401 boundary assertion) to ADVOCATE-2 and code evidence — the arithmetic was based on an inverted model of `capture-pane -S`
- CRITIC-1 conceded Defect 2 tmux 3.2+ version-specific framing to QUESTIONER — marked unsubstantiated
- ADVOCATE-1 conceded Defect 3 (test does not verify output) to CRITIC-1 — characterized as a scope gap alongside a stronger integration test
- ADVOCATE-1 conceded Defect 4 (unnecessary skip) to CRITIC-1 — minor test design issue
- ADVOCATE-2 conceded Defect 4 (minor) to CRITIC-1
- ADVOCATE-2 granted Defect 2(b) as "a real edge case" — acknowledged the conservative failure mode

### Regression Lineage

The prior council's CONDITIONAL FOR was earned: this PR resolves eight specific remediation items (conflict resolution, compile-time assertion, SCROLLTEST-LINE-401 boundary assertion, PaneCount→WindowCount rename, parseTmuxError centralization, context.Context threading, TestSendCommand_SpecialCharacters, TestCaptureOutput_ScrollbackMath). The current debate found no regressions against prior council findings. The remaining tracked items (Defects 2, 3, 4) are net-new observations specific to this PR's additions.

### Arbiter Recommendation

**FOR**

All three parties converged on FOR: advocates unconditionally, CRITIC-1 conditionally with tracked follow-up items that all parties agree are not merge conditions. The three prior council conditions are satisfied at verifiable file:line locations. The only substantive new challenge — CRITIC-1's LINE-401 arithmetic argument — was resolved on the merits when CRITIC-1 accepted the corrected model of `capture-pane -S` semantics and fully withdrew Defect 1. The remaining three items (Defects 2, 3, 4) are test quality and architectural design concerns that belong in follow-up tasks, not as merge conditions: the failure modes are conservative, the validation logic is correct, and none introduces data loss, silent corruption, or API contract violation.

### Suggested Fixes

#### In-PR Improvements

**Defect 2 — Add `ErrNoServer` sentinel to `parseTmuxError`; remove secondary string matching from `ListSessions`**

`parseTmuxError` was introduced as the single translation layer for tmux stderr, but `ListSessions` performs a second layer of string matching on the already-wrapped error. This violates the design intent and creates a fragile coupling to the default-case format string (`"tmux: %s"`). The correct fix is to add an `ErrNoServer` sentinel case to `parseTmuxError`, handle it in `ListSessions` via `errors.Is`, and remove the raw string check.

CITE: `internal/terminal/tmux_layout.go` L:19-24
CITE: `internal/terminal/tmux_errors.go` L:38-53

**Defect 4 — Remove tmux dependency from `TestCaptureOutput_InvalidLines`**

`ErrInvalidLines` is checked at `tmux_capture.go:14-16` before any tmux subprocess invocation. The test at `tmux_capture_test.go:11-24` calls `NewTmuxSubstrate()` and skips if tmux is absent, despite having no runtime dependency on tmux. The test should be restructured to exercise the validation path directly, either by constructing the substrate differently or by extracting the validation into a sub-test that doesn't require the constructor.

CITE: `internal/terminal/tmux_capture_test.go` L:11-24
CITE: `internal/terminal/tmux_capture.go` L:14-16

#### PR Description Amendments

**Defect 3 — Add doc comment to `TestSendCommand_SpecialCharacters` clarifying API-layer scope**

The test name implies shell-side verification of special-character handling. The test actually verifies that the tmux `send-keys` API accepts the keystroke sequences without returning a non-zero exit code — correct for the tmux API layer, but silent about shell-side interpretation. A brief comment clarifying the scope boundary would prevent future contributors from treating the test as sufficient proof that special characters are handled correctly end-to-end.

CITE: `internal/terminal/tmux_command_test.go` L:38-65

### Verification Results

| # | Finding | Citations | Verdict | Action |
|---|---------|-----------|---------|--------|
| 1 | ListSessions secondary string matching after parseTmuxError | `tmux_layout.go` L:19-24, `tmux_errors.go` L:38-53 | VERIFIED | Retained |
| 2 | TestCaptureOutput_InvalidLines unnecessary tmux skip | `tmux_capture_test.go` L:11-24, `tmux_capture.go` L:14-16 | VERIFIED | Retained |
| 3 | TestSendCommand_SpecialCharacters scope clarification | `tmux_command_test.go` L:38-65 | VERIFIED | Retained |

Verification: 5/5 citations verified, 0 phantom (purged), 0 unverified. All findings verified against codebase.
