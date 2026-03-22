---
## Adversarial Council — Merge PR #43 (Substrate Interface & Core F1) into Epic 2

> Convened: 2026-03-22T18:15:00Z | Advocates: 2 | Critics: 2 | Rounds: 3/4

### Motion
Should PR #43 (feat: Substrate Interface & Core Implementation F1) be merged into epic/2-implement-terminal-substrate-layer?

---

### Advocate Positions

**ADVOCATE-1**: This PR delivers exactly what spec tasks T1 and T2 authorize. The spec's task breakdown explicitly sequences work into discrete tasks — T1 (interface+types) and T2 (SpawnSession, DestroySession) are done; T2.1/T2.2/T3/T5 are future. The `run()` helper correctly uses `exec.CommandContext` for cancellation support; `SpawnSession` validates the name via `ValidateSessionName` before touching tmux; `parseTmuxError` is case-insensitive and covers three tmux error families; all six interface methods carry `context.Context`. Context threading was a prior-council remediation and was applied correctly. The merge criterion is not "all 6 methods implemented" — it is "T1 and T2 are correct." They are.

**ADVOCATE-2**: The `Substrate` interface at `substrate.go:9-32` defines a six-method contract that is entirely substrate-agnostic — no tmux imports, no tmux types, no tmux-specific anything. Context propagation on all six methods was added at the right time (interface definition), before any callers exist. The error translation layer in `tmux_errors.go:38-53` centralizes all tmux stderr → typed sentinel conversion; callers can `errors.Is(err, terminal.ErrSessionNotFound)` without knowing which substrate produced the error. The partial implementation is safe for a feature branch — Go's type system enforces completeness at the assignment site, not at the point of incomplete method set.

---

### Critic Positions

**CRITIC-1**: Four issues raised; two survived to final position. (1) Integration tests: `SpawnSession` at `tmux.go:41` and `DestroySession` at `tmux.go:64` have zero test coverage. A basic spawn + list-sessions + destroy smoke test was achievable within T2 scope without any T3 dependency. (2) ErrSessionNotFound at `tmux_errors.go:41-44` conflates "session not found", "can't find window", and "can't find pane" into one sentinel. This is semantically incorrect — a missing pane is not a missing session — and cheaper to fix before T3 (SplitPane) lands than after, because T3 callers will need to distinguish pane errors from session errors. Missing `var _ Substrate = &TmuxSubstrate{}` interface guard: withdrawn — cannot be added mid-implementation without breaking CI; belongs at T3 completion. "Compile failure" framing: fully conceded.

**CRITIC-2**: (1) `DestroySession` at `tmux.go:65-70` skips `ValidateSessionName` while `SpawnSession` calls it at `tmux.go:42`. tmux's `-t` flag accepts `session:window.pane` colon syntax — passing `"my:session"` to `DestroySession` bypasses validation and forwards it as a tmux target, which resolves to window "session" in session "my": a session the substrate never created. This is live in implemented code, not dormant future concern. (2) `cmd` at `tmux.go:47-49` is passed as a single unvalidated argument while `name` is validated at the same call site; no doc comment establishes the trusted-caller contract. ARBITER challenge on the `/bin/sh -c` mechanism accepted — retracted; risk reframed as asymmetric validation and absent caller documentation. (3) `SendCommand` pattern-precedent concern: reframed — not a defect against unimplemented code, but the interface at `substrate.go:19` commits the method with no security documentation despite the spec's Risks table flagging send-keys handling as a known concern. (4) Stderr verbatim forwarding at `tmux_errors.go:51`: acknowledged as below Critical Discovery threshold.

---

### Key Conflicts

- **Compile-time failure** — CRITIC-1 initially claimed "broken abstraction / compile-time failure"; ADVOCATE-1 and ADVOCATE-2 demonstrated no assignment of `*TmuxSubstrate` to `Substrate` exists in this diff and no compile error occurs. CRITIC-1 fully conceded. RESOLVED — argument dropped.

- **Interface guard (`var _ Substrate = &TmuxSubstrate{}`)** — CRITIC-1 argued it should be in this PR; ADVOCATE-2 demonstrated it would immediately produce 4 compile errors listing missing methods. CRITIC-1 conceded it belongs at T3 completion. RESOLVED — not a this-PR requirement; should be tracked via TODO comment.

- **DestroySession missing ValidateSessionName** — CRITIC-2 raised it (`tmux.go:65-70`); both ADVOCATE-1 and ADVOCATE-2 conceded it is a real inconsistency and a one-line fix. The tmux `-t` colon-syntax targeting risk is the substantive mechanism. RESOLVED — confirmed as an in-PR bug fix.

- **Integration tests for SpawnSession/DestroySession** — CRITIC-1 argued no tests exist for implemented operations; ADVOCATE-1 initially defended with spec T5 sequencing then partially conceded: a basic spawn+destroy smoke test is achievable without ListSessions or T3 dependency. RESOLVED — gap is real; partially addressable within T2 scope.

- **Hardcoded 200×50 dimensions** (`tmux.go:46`) — CRITIC-1 raised as potentially breaking; ADVOCATE-2 proposed a `SessionOptions` struct extension path that avoids signature breakage. RESOLVED — CRITIC-1's "breaking change" framing substantially weakened; accepted as enhancement path.

- **ErrSessionNotFound conflating pane errors** (`tmux_errors.go:41-44`) — CRITIC-1 argued semantic defect; ADVOCATE-1 initially defended as dormant then partially conceded: cheaper to fix before T3. UNRESOLVED as a merge blocker — neither side reached consensus on whether this must block this PR or be a tracked pre-T3 requirement. Arbiter adjudication below.

- **cmd unvalidated in SpawnSession** (`tmux.go:47-49`) — CRITIC-2 raised; CRITIC-2 retracted the `/bin/sh -c` mechanism claim under ARBITER challenge. Reframed as asymmetric validation + absent caller documentation. ADVOCATE-1 and ADVOCATE-2 argued this is a design note for an internal API (orchestrator controls cmd), not an OWASP-class vulnerability. RESOLVED — in-PR improvement (doc comment), not a bug fix.

---

### Concessions

- CRITIC-1 conceded: "compile-time failure" framing was incorrect; code compiles as-is
- CRITIC-1 conceded: interface guard cannot go in this PR without breaking CI; belongs at T3
- CRITIC-1 conceded: Width/Height comment qualifier "returned by ListSessions" scopes the zero-value rule to ListSessions context; SpawnSession returning known dimensions is consistent
- CRITIC-2 conceded: `/bin/sh -c` mechanism claim was unsubstantiated; retracted under ARBITER challenge
- CRITIC-2 conceded: SendCommand scaffolding concern is a pattern-precedent observation, not a defect against unimplemented code
- CRITIC-2 conceded: stderr verbatim forwarding does not meet Critical Discovery threshold for this architecture
- ADVOCATE-1 conceded: DestroySession missing ValidateSessionName is a real inconsistency with live tmux targeting risk
- ADVOCATE-1 conceded: basic smoke test for SpawnSession/DestroySession was achievable within T2 scope without ListSessions dependency
- ADVOCATE-2 conceded: DestroySession validation gap is a real one-line defect
- ADVOCATE-2 conceded: integration test gap is real, correctly deferred per spec T5 for full lifecycle; basic smoke test achievability acknowledged

---

### Arbiter Recommendation

**CONDITIONAL FOR**

The PR delivers T1 and T2 as spec-sequenced. The interface is architecturally correct, context propagation is properly threaded, error mapping is centralized, and the two implemented operations are logically sound. One confirmed live defect exists — `DestroySession` at `tmux.go:65-70` skips the same `ValidateSessionName` guard that `SpawnSession` applies at `tmux.go:42`, creating a real tmux `-t` colon-syntax targeting inconsistency in implemented, callable code. Both advocates conceded this finding. The fix is one line. Merge is conditional on that fix being applied before the PR is merged to the epic branch.

The `ErrSessionNotFound` pane-conflation at `tmux_errors.go:41-44` is dormant at T2 scope — `SpawnSession` and `DestroySession` do not generate "can't find window" or "can't find pane" stderr — but it should be resolved before `SplitPane` (T3) lands. It is tracked below as a required pre-T3 issue rather than a this-PR blocker.

The test gap is real and partially addressable within T2 scope. A basic integration test for SpawnSession + DestroySession (spawn → verify via `tmux list-sessions` → destroy → verify gone) is achievable without any T3 dependency. This is logged as a conditional improvement rather than a hard blocker, as its omission does not create a live defect — but it should be addressed in this PR or the immediately following one before the epic branch accumulates further untested operations.

---

### Conditions (REQUIRED before merge)

1. **Add `ValidateSessionName(name)` to `DestroySession`** — `tmux.go:65` — add the same name validation `SpawnSession` applies at line 42; prevents tmux `-t` colon-syntax targeting of unrelated sessions

---

### Suggested Fixes

#### Bug Fixes (always in-PR)

- **DestroySession missing ValidateSessionName** — `tmux.go:65` — insert `if err := ValidateSessionName(name); err != nil { return err }` at the top of `DestroySession`, matching the pattern at `SpawnSession:42`. Both advocates conceded this defect. CRITIC-2 demonstrated the tmux `-t` colon-syntax targeting risk.

#### In-PR Improvements (scoped, non-bug)

- **TODO comment for interface guard** — `tmux.go`, near `TmuxSubstrate` struct definition — add `// TODO: add var _ Substrate = (*TmuxSubstrate)(nil) once all 6 methods are implemented` so the guard is not silently omitted across T2.1–T3 PRs. CRITIC-1 requested; ADVOCATE-2 conceded as legitimate; CRITIC-1 accepted this as the appropriate form.

- **Doc comment on `cmd` parameter** — `tmux.go:41` (SpawnSession godoc) — add a note that `cmd` must be a trusted binary path controlled by the orchestrator layer, not user-supplied input. CRITIC-2 raised; both advocates acknowledged the absent caller contract documentation.

- **Integration smoke test for SpawnSession/DestroySession** — `tmux_errors_test.go` or new `tmux_integration_test.go` — basic test: spawn a named session, verify it appears in `tmux list-sessions`, destroy it, verify it is gone. Skippable when tmux not available (pattern already established at `tmux_errors_test.go:10-17`). ADVOCATE-1 partially conceded this was achievable within T2 scope.

#### PR Description Amendments

- Explicitly state this PR covers T1 + T2 only; list T2.1 (SendCommand), T2.2 (CaptureOutput), T3 (ListSessions/SplitPane), T5 (integration tests) as planned follow-on PRs into the epic branch so reviewers understand the partial interface satisfaction is by design.

#### New Issues (future features only — confirm with human)

- **`ErrPaneNotFound` sentinel** — `tmux_errors.go:41-44` currently maps "can't find window" and "can't find pane" to `ErrSessionNotFound`. This is semantically incorrect; a missing pane is not a missing session. Introduce `ErrPaneNotFound` in `types.go` and a new `case` in `parseTmuxError` before T3 (SplitPane) lands. CRITIC-1 argued; ADVOCATE-1 partially conceded it is cheaper pre-T3. — **Task** (pre-T3 required)

- **Configurable session dimensions** — `tmux.go:46` hardcodes `-x 200 -y 50`. ADVOCATE-2 proposed a `SessionOptions` struct extension path. Implement when per-agent viewport requirements become concrete. — **Feature** (post-T3)

---

*Arbiter: ARBITER | Model: claude-sonnet-4-6 | 2026-03-22T18:15:00Z*
