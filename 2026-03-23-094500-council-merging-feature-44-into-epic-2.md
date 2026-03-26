## Adversarial Council -- Merge PR #46: Feature #44 (F2) into Epic #2

> Convened: 2026-03-23T09:45:00 | Advocates: 2 | Critics: 1 | Rounds: 2/4 | Motion type: CODE

### Motion

Merge PR #46 — feature/44 (Command Injection & Output Capture, F2) into epic/2 (Implement Terminal Substrate Layer). PR adds 11 files (825 insertions, 0 deletions) implementing `SendCommand` and `CaptureOutput` methods on `TmuxSubstrate`, along with the `Substrate` interface, shared types, error handling, and comprehensive tests. Current merge status: CONFLICTING.

---

### Advocate Positions

**ADVOCATE-1**: The implementation satisfies every must-have F2 exit criterion with precise, verifiable citations. `SendCommand` passes Enter as a discrete sixth argument to `run()` (`tmux_command.go:14`), preventing special-character misinterpretation. `CaptureOutput` guards against invalid line counts before invoking tmux (`tmux_capture.go:14-16`) and computes `-S -{lines}` correctly (`tmux_capture.go:20`). Error sentinels propagate through `fmt.Errorf(...%w...)` preserving `errors.Is` unwrapping. All six spec acceptance scenarios are covered. The 4/6 interface state is correct for a feature branch targeting an epic accumulator — Go does not require interface satisfaction at type declaration time. The compile-time assertion is physically impossible in F2 (would break the build for unimplemented F3 methods) and belongs in the F3 PR. Final recommendation: FOR.

**ADVOCATE-2**: Every spec exit criterion is met. `ListSessions` and `SplitPane` are explicitly assigned to F3 per three spec files. The compile-time assertion mechanically cannot be added to a 4/6 branch without breaking the build — it belongs on epic/2 after F3 lands. The scrollback test gap (LINE-401 boundary) is a real test quality improvement but not a correctness defect. Conceded: Round 1 factual error ("all six methods"), compile-time assertion as a future safeguard, scrollback test gap. Final recommendation: CONDITIONAL FOR — merge after conflict resolution with two tracked follow-up tasks.

---

### Critic Positions

**CRITIC-1**: Six defects identified at opening. Conceded three outright (#3 sanitization, #5 time.Sleep, #6 session validation). Conceded ListSessions/SplitPane are correctly assigned to F3. Conceded compile-time assertion is mechanically impossible in F2 after ADVOCATE-2 demonstrated the build-break. Reclassified scrollback test gap as a follow-up. Final remaining concerns: (1) spec exit criterion wording at `command-injection-output-capture-spec.md:216` implies full interface satisfaction when only F2 methods are verified — should be clarified; (2) LINE-401 lower-boundary test assertion is absent. Final recommendation: CONDITIONAL FOR — withdraw all opposition if three conditions are tracked (see Conditions).

---

### Questioner Findings

QUESTIONER did not post findings during the debate (excluded per team-lead directive).

**ARBITER independently verified three claims:**
1. ADVOCATE-2's Round 1 assertion "TmuxSubstrate now implements all six methods" — **UNSUBSTANTIATED**. Grep confirms `ListSessions` and `SplitPane` have zero implementations across all 9 package files; only interface declarations exist.
2. CRITIC-1's claim that ListSessions has no scope assignment — **NOT SUBSTANTIATED**. `specs/substrate-interface-core-spec.md:15` explicitly states "Layout management (`ListSessions`, `SplitPane`) — covered by F3." `specs/layout-management-error-handling-spec.md` is the dedicated F3 spec for both methods. `specs/terminal-substrate-spec.md:140` assigns T3 as "Implement ListSessions and SplitPane."
3. ADVOCATE-2's technical claim that the compile-time assertion cannot compile in a 4/6 branch — **SUBSTANTIATED**. `var _ Substrate = (*TmuxSubstrate)(nil)` would produce a compile error for `ListSessions` and `SplitPane` until F3 completes.

---

### Key Conflicts

- **Interface completeness vs. feature-branch scope**: CRITIC-1 argued missing methods violate the exit criterion; advocates argued the criterion applies to F2's methods only. RESOLVED in favor of advocates — `substrate-interface-core-spec.md:15` assigns both missing methods to F3; the F2 task breakdown has no ListSessions task. The exit criterion refers to F2's two methods.

- **Compile-time assertion timing**: CRITIC-1 argued it belongs in this PR; advocates argued it cannot compile here. RESOLVED in favor of advocates — the assertion is mechanically impossible in a partial-interface branch; CRITIC-1 conceded after ADVOCATE-2's Round 3 argument.

- **Scrollback test precision**: Both sides agreed the implementation math is correct; both agreed the test at `tmux_capture_test.go:97-104` does not verify LINE-401 as the first captured line per the Gherkin scenario. RESOLVED as a tracked follow-up — not a correctness defect.

---

### Concessions

- **ADVOCATE-2** conceded Round 1 factual error: "all six methods implemented" — incorrect, 4/6 actual.
- **ADVOCATE-2** conceded compile-time assertion is a valuable safeguard (non-blocking, post-F3 timing).
- **ADVOCATE-2** conceded Issue #4 scrollback boundary test gap (LINE-401 assertion missing).
- **ADVOCATE-1** conceded compile-time assertion should be added before full-epic completion (future task, not F2 blocker).
- **ADVOCATE-1** conceded Issue #4 scrollback test gap as "a test quality improvement."
- **CRITIC-1** conceded Issue #3 (sanitization is correctly deferred as Should-Have per `specs/command-injection-output-capture-spec.md:212-219`).
- **CRITIC-1** conceded Issue #5 (time.Sleep is spec-endorsed mitigation per risk table `specs/command-injection-output-capture-spec.md:114-115`).
- **CRITIC-1** conceded Issue #6 (SendCommand validation gap is benign — `ValidateSessionName` at `tmux.go:42` prevents invalid session names from entering via the public API).
- **CRITIC-1** conceded ListSessions/SplitPane are correctly assigned to F3 per `specs/substrate-interface-core-spec.md:15`.
- **CRITIC-1** conceded compile-time assertion is mechanically impossible in F2 (Round 3, final summary) — withdrew all opposition.
- **CRITIC-1** reclassified Issue #4 from blocking objection to tracked follow-up.

---

### Regression Lineage

No regression lineage — no prior fix involvement.

---

### Arbiter Recommendation

**CONDITIONAL FOR**

All three debating parties converged on CONDITIONAL FOR in their final summaries. The F2 implementation is correct and spec-compliant: `SendCommand` (`tmux_command.go:14`) passes Enter as a separate argument; `CaptureOutput` (`tmux_capture.go:13-25`) correctly guards against invalid input and computes the `-S -{lines}` scrollback flag. Error sentinels propagate correctly. Context flows from all public methods through `exec.CommandContext`. `ListSessions` and `SplitPane` are confirmed F3 scope per three spec files and are not F2 defects. The compile-time assertion is mechanically impossible in a partial-interface branch and correctly belongs on epic/2 after F3. No correctness defect was established by any party. Merge after conflict resolution, with three tracked items.

---

### Conditions

1. **Resolve merge conflicts** before merging — prerequisite. Both sides agree the PR cannot merge in its current CONFLICTING state.

2. **Track follow-up task on epic/2**: Add `var _ Substrate = (*TmuxSubstrate)(nil)` to `internal/terminal/tmux.go` after F3 (`ListSessions` + `SplitPane`) lands. Assign an owner when F3 work begins. This makes the full interface contract machine-verifiable.
   CITE: `internal/terminal/tmux.go` L:10 (insertion point after package declaration)

3. **Track follow-up task on epic/2**: Strengthen `TestCaptureOutput_LargeScrollback` to assert `strings.Contains(out, "SCROLLTEST-LINE-401")` as the lower boundary, fully satisfying the Gherkin acceptance scenario.
   CITE: `internal/terminal/tmux_capture_test.go` L:97-105
   CITE: `specs/command-injection-output-capture-spec.md` L:162-163

---

### Suggested Fixes

#### Bug Fixes

No bugs found. All identified issues are either scope-correct omissions (F3 items), spec-approved deferments (Should-Have), or test precision gaps with correct underlying implementations.

#### In-PR Improvements

None required. Both follow-up tasks are intentionally deferred to the epic branch after F3 lands.

#### PR Description Amendments

- Amend exit criterion at `specs/command-injection-output-capture-spec.md:218` from "Methods satisfy the `Substrate` interface contract defined in `substrate.go`" to "SendCommand and CaptureOutput satisfy their `Substrate` interface contracts defined in `substrate.go`" — prevents future reviewers from inferring full 6/6 interface satisfaction from F2 alone (ADVOCATE-2's Round 1 factual error demonstrates this is a real reviewer risk, not theoretical).
  CITE: `specs/command-injection-output-capture-spec.md` L:218

#### Critical Discoveries

None. No OWASP Top 10, data loss, or compliance issues were identified or raised during the debate.

---

*Recommendation authored by: ARBITER | Council closed: 2026-03-23 | Unanimous CONDITIONAL FOR*
