---
## Adversarial Council — Merge PR #43 (F1 Substrate Interface) into Epic 2

> Convened: 2026-03-22T22:00:00Z | Advocates: 2 | Critics: 2 | Rounds: 2/4

### Motion
Should PR #43 (feature/42 — Substrate Interface & Core Implementation F1) be merged into epic/2-implement-terminal-substrate-layer?

---

### Pre-Debate Technical Investigation (ARBITER)

Before opening the debate, the ARBITER ran independent code-level verification. Key findings:

**Merge mechanics — the "zero conflicts" briefing claim is false.**

The briefing cited `git merge-tree <base> <branch1> <branch2>` (old trivial mode), which does not report add/add conflicts — it silently selects one side. The new-style `git merge-tree epic/2 feature/42` (which performs actual content merging) exits with **code 1** and reports 5 CONFLICT (add/add) entries:

| File | F1 blob | Epic/2 blob | Result |
|---|---|---|---|
| internal/terminal/substrate.go | 37525cc | d03d277 | add/add CONFLICT |
| internal/terminal/tmux.go | f71268c | 55dbcb2 | add/add CONFLICT |
| internal/terminal/types.go | a2dff7a | b56f8c2 | add/add CONFLICT |
| internal/terminal/tmux_errors.go | 86ee014 | e42d4a8 | add/add CONFLICT |
| internal/terminal/tmux_errors_test.go | 19b26d3 | d89486b | add/add CONFLICT |
| internal/terminal/tmux_integration_test.go | F1 only | — | no conflict (unique filename) |
| internal/terminal/integration_test.go | — | epic/2 only | no conflict |

Both branches created all five conflicting files from a merge base (`43e0532`) that had only `.gitkeep`. GitHub's squash-merge button will fail.

**Interface incompatibility verified at file:line:**

- F1's `substrate.go:9-34`: all 6 interface methods carry `ctx context.Context` as first arg; `SpawnSession` also takes `opts SessionOptions`
- Epic/2's `substrate.go:10-28`: no `context.Context` on any method; no `SessionOptions`
- Epic/2's `tmux_command.go:13`: `SendCommand(session string, cmd string) error` — no ctx
- Epic/2's `tmux_layout.go:12`: `t.run(...)` — no ctx; `ListSessions() ([]Session, error)` — no ctx at line 11
- Epic/2's `tmux_layout.go:56`: assigns `PaneCount: paneCount` where `paneCount` was parsed from `#{session_windows}` format field (line 14) — semantically wrong field name (windows ≠ panes)
- F1's `tmux_integration_test.go:33`: calls `sub.SpawnSession(ctx, name, "", SessionOptions{})` — 4-arg form that does not compile against epic/2's 2-arg implementation

**Epic/2 baseline bug confirmed (independent of F1 merge):**

`tmux_errors.go:41-45` on epic/2 maps `"can't find window"` and `"can't find pane"` to `ErrSessionNotFound` — conflating a missing-pane error with a missing-session error. F1's `parseTmuxError` introduces `ErrPaneNotFound` to fix this. This bug is also replicated inline in epic/2's `tmux_command.go:17-20`, `tmux_capture.go:22-26`, and `tmux_layout.go:26-29`.

**No go build CI gate exists:**

`.github/workflows/` contains only version, release, and branch-handler workflows. There is no automated compilation check on the epic/2 branch.

---

### Advocate Positions

**ADVOCATE-1**: F1's design is unambiguously correct for an agent orchestrator. `context.Context` on all 6 interface methods is mandatory — without it, any caller needing cancellation (DAG abort, supervisor shutdown, timeout) faces a breaking API change. `exec.CommandContext` in `run()` prevents orphaned tmux processes on context cancel. `ValidateSessionName` is a security boundary (injection protection) documented at the interface level. `parseTmuxError` centralizes what epic/2 duplicates inline across three files. `WindowCount` is the semantically correct name — `tmux_layout.go:14` uses `#{session_windows}` as the format field, but `tmux_layout.go:56` assigns it to `PaneCount`. The conflicts are resolvable with F1 winning every file; follow-on edits to thread context through F2-F4 are mechanical, bounded, and fully enumerated. CONDITIONAL FOR.

**ADVOCATE-2**: Design convergence was reached: CRITIC-1 conceded all design points. The remaining dispute is purely procedural. The 6 mechanical edits required for a clean merge — adding `ctx context.Context` to `SendCommand`, `CaptureOutput`, `ListSessions`, `SplitPane` signatures and threading it to `t.run(ctx, ...)`, renaming `PaneCount` → `WindowCount` at `tmux_layout.go:56`, and updating `integration_test.go` — follow an identical pattern across 4 files. CRITIC-2 raised a valid additional defect: `TmuxVersion()` at `tmux_errors.go:29` uses `exec.Command` without context, inconsistent with the rest of the codebase. This is a one-line fix. CONDITIONAL FOR with all defects enumerated.

---

### Critic Positions

**CRITIC-1**: F1's architectural improvements are correct — context threading, SessionOptions, WindowCount, ErrPaneNotFound, parseTmuxError should all land on epic/2. However, PR #43 as submitted cannot merge cleanly. The 5 conflicts, plus the unreviewed context-threading edits across 4 files, plus the absence of a go build CI gate, mean that approving the merge as-is would authorize unreviewed in-flight edits at merge time with no compilation verification. The correct path is: close PR #43, rebase on epic/2 tip, make the interface-threading changes on a clean branch where they can be reviewed and `go build ./internal/terminal/...` verified, then open a new PR. The code outcome is byte-for-byte identical to a CONDITIONAL FOR — the distinction is whether there is a reviewable, buildable artifact at the point of merge.

**CRITIC-2** (joined post-call): Confirmed the 5 add/add conflicts independently. Documented compile failures if either side wins the conflict resolution without also updating the other side's files. Identified `TmuxVersion()` in F1's `tmux_errors.go:29-35` as using `exec.Command` without context — a hanging `tmux -V` would block indefinitely, inconsistent with the rest of the codebase. Inline error duplication in `tmux_command.go` still maps "can't find pane" to `ErrSessionNotFound` even after the council-remediated `ErrPaneNotFound` sentinel exists.

---

### Key Conflicts

- **"Zero conflicts" claim** — Resolved. Both advocates conceded. Old trivial merge-tree does not report add/add conflicts. New-style confirms 5 real conflicts. Exit code 1.
- **Does epic/2 already have ErrPaneNotFound, SessionOptions, WindowCount?** — Resolved. CRITIC-1's claim that T4 added these was retracted after direct file verification. Epic/2's `types.go` has `PaneCount`, no `SessionOptions`, no `ErrPaneNotFound`.
- **Is F1's design architecturally superior?** — Resolved/Converged. All parties agree. context.Context, SessionOptions, WindowCount, ErrPaneNotFound, parseTmuxError are correct.
- **Is PaneCount a semantic error?** — Resolved (verified by ARBITER). `tmux_layout.go:14` uses `#{session_windows}`; `tmux_layout.go:56` assigns to `PaneCount`. The author's own comment at line 41 says the field is "windows." F1's `WindowCount` is correct.
- **Merge mechanism: conditional FOR vs. close-and-reopen** — Unresolved between sides. ADVOCATES argue CONDITIONAL FOR covers the rebase work. CRITIC-1 argues AGAINST (as-is) with close-and-new-PR producing identical code. Both paths lead to the same codebase state.
- **TmuxVersion() context gap** — Raised by CRITIC-2. Not contested. Valid defect.

---

### Concessions

- **ADVOCATE-1** conceded the "zero conflicts" claim is factually wrong and withdrew arguments depending on it.
- **ADVOCATE-2** conceded the "zero conflicts" claim is factually wrong.
- **CRITIC-1** conceded their claim that epic/2 already had `ErrPaneNotFound`, `SessionOptions`, and `WindowCount` (via T4) — this was false, verified against the actual blob on epic/2.
- **CRITIC-2** did not participate during the debate (submitted position after debate was called); no concessions recorded.

---

### Arbiter Recommendation

**CONDITIONAL**

F1's design is architecturally correct and should be integrated into epic/2. The council is unanimous on the design question: `context.Context` threading, `SessionOptions`, `ErrPaneNotFound`, `WindowCount`, `parseTmuxError`, and `ValidateSessionName` are genuine improvements over epic/2's current state. However, PR #43 in its current form cannot be merged — 5 add/add conflicts require manual resolution, and the resulting branch would fail to compile until epic/2's four F2-F4 implementation files are updated to match F1's interface signatures. Whether this work is done on the existing feature/42 branch (updated PR) or a new branch (closed and reopened) is a judgment call for the repo owner; both paths produce identical code.

**CRITIC-1's concern about unreviewed in-flight edits is valid and binding.** The merge work must produce a reviewable, buildable artifact — not edits performed at merge time. The PR must reach a state where `go build ./internal/terminal/...` passes before it is merged.

---

### Conditions (CONDITIONAL)

1. **Rebase** feature/42 on epic/2's current tip (`b46ee02`).
2. **Resolve all 5 add/add conflicts in F1's favor** — F1's versions of `substrate.go`, `tmux.go`, `tmux_errors.go`, `tmux_errors_test.go`, and `types.go` win.
3. **Thread `context.Context` through F2-F4 implementation files** (6 call-site edits):
   - `tmux_command.go:13` — `SendCommand(session, cmd)` → `SendCommand(ctx context.Context, session, cmd)`, pass `ctx` to `t.run(ctx, ...)`
   - `tmux_capture.go` — `CaptureOutput(session, lines)` → `CaptureOutput(ctx context.Context, session, lines)`, pass `ctx` to `t.run(ctx, ...)`
   - `tmux_layout.go:11` — `ListSessions()` → `ListSessions(ctx context.Context)`, pass `ctx` to `t.run(ctx, ...)`
   - `tmux_layout.go:63` — `SplitPane(session, direction)` → `SplitPane(ctx context.Context, session, direction)`, pass `ctx` to `t.run(ctx, ...)`
   - `tmux_layout.go:56` — `PaneCount: paneCount` → `WindowCount: paneCount`
   - `integration_test.go:~21,28` — thread `context.Background()` into `SpawnSession(ctx, name, "bash", SessionOptions{})` and `DestroySession(ctx, name)`
4. **Fix `TmuxVersion()` context gap** — `tmux_errors.go:29`: replace `exec.Command(path, "-V").Output()` with `exec.CommandContext` using a bounded timeout, consistent with the rest of the codebase.
5. **Activate compile-time interface assertion** — `tmux.go:11` TODO: uncomment `var _ Substrate = (*TmuxSubstrate)(nil)` after all 6 methods have context-bearing signatures.
6. **Verify `go build ./internal/terminal/...` and `go vet ./internal/terminal/...` pass** on the resulting branch before merge.

---

### Suggested Fixes

#### Bug Fixes (always in-PR)

1. **`tmux_errors.go:29` (`TmuxVersion`)** — Uses `exec.Command` without context. A hanging `tmux -V` blocks the calling goroutine indefinitely with no cancellation path. Fix: use `exec.CommandContext` with a short timeout (e.g., 5 seconds). This is the sole context-free shell invocation in the package and is inconsistent with all other calls routed through `run()`.

2. **`tmux_command.go:17-20`, `tmux_capture.go:22-26`, `tmux_layout.go:26-29` (epic/2 inline error checks)** — All three files map `"can't find window"` and `"can't find pane"` to `ErrSessionNotFound`. This is semantically wrong: a missing-pane error is distinct from a missing-session error. F1's `parseTmuxError` is called inside `run()` and already returns typed sentinels to callers. The correct fix is therefore NOT to call `parseTmuxError` again on the returned error — it is to replace all inline `strings.Contains(err.Error(), ...)` checks with `errors.Is(err, ErrSessionNotFound)` / `errors.Is(err, ErrPaneNotFound)` semantics. Note: `tmux_layout.go:ListSessions` has an additional error path for `"no server running"` / `"no sessions"` that `parseTmuxError` does not cover and must be preserved as a separate branch. This migration is additional scope beyond the 6 call-site edits enumerated for context threading — CRITIC-1 is correct that the total scope is larger than the advocates initially claimed.

#### In-PR Improvements

3. **Activate interface assertion** — After all 6 methods satisfy F1's interface, add `var _ Substrate = (*TmuxSubstrate)(nil)` to `tmux.go` (removing the TODO comment). This makes future interface-drift a compile error.

#### PR Description Amendments

4. **Scope expansion** — PR #43 was originally scoped to "F1 — Substrate Interface & Core Implementation (SpawnSession, DestroySession)." After the rebase and call-site updates, the PR also migrates the F2-F4 implementation files to context-bearing signatures. The PR description should explicitly state this expanded scope so reviewers know what changed.

#### New Issues (future features only)

*None identified. All blocking items above must be resolved before merge.*

---

### QUESTIONER Notes (ARBITER operating in parallel)

- **CRITIC-1's claim** that epic/2 already has `ErrPaneNotFound`, `SessionOptions`, `WindowCount` — challenged and retracted. Marked as unsubstantiated in early rounds; concession recorded.
- **ADVOCATE-1's claim** that `tmux_layout.go` assigns window-count to `PaneCount` via `#{session_windows}` — challenged for file:line citation; citation provided (`tmux_layout.go:14`, `tmux_layout.go:50`, `tmux_layout.go:56`) and independently verified by ARBITER. Marked substantiated.
- **ADVOCATE-1/2 claim** that conflict resolution is "15 lines" — CRITIC-1 correctly expanded this to ~16+ discrete edits across 4 files. The advocates ultimately enumerated all 6 edit sites explicitly (ADVOCATE-2 closing), confirming CRITIC-1's scope count was accurate. Both characterizations are consistent.
- **CRITIC-2's `TmuxVersion` finding** — raised after debate was called; not contested by any party. Treated as a valid late finding; classified as Bug Fix above.
---
**Scope note (from final summaries):** CRITIC-2 confirmed that `tmux_layout.go:ListSessions` has an error path for `"no server running"` / `"no sessions"` not covered by `parseTmuxError`, requiring a preserved separate branch in the `errors.Is()` migration. CRITIC-1's final summary is that the full pre-merge work covers rebase + 6 context edits + errors.Is migration across 3 files + TmuxVersion fix = more than the advocates' initial "15 lines" estimate. The conditions above reflect the full enumerated scope.

*Council convened and arbitrated 2026-03-22. Recommendation written by ARBITER. Final summaries incorporated post-file-creation.*
