---
## Adversarial Council — PR #24: Epic Merge to Main (Round 2)

> Convened: 2026-03-26 | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion
Merge PR #24 (epic: implement go orchestrator core) into main. This is an epic branch integrating all sub-features (#9 E2E lifecycle tests, #15 CompareAndSetAgentState for optimistic locking, #17 Event stream consumption methods, #19 Wire DAG Executor into gRPC handlers) into the main branch. 15,518 additions across 121 files. CI green (test x2 pass, validate-version pass).

### Advocate Positions
**ADVOCATE-1**: The epic delivers a complete, well-tested Go orchestrator core. Key strengths cited with evidence: stateless agent machine with CAS-based transitions (`agent/machine.go:47-73`), per-agent mutexes in supervisor, correct DAG topological execution with fail-fast semantics (`executor.go:108-115`), comprehensive health aggregation with graceful degradation (`health/aggregator.go:71-136`), proper interface segregation at consumer boundaries (`dag.TaskEnqueuer`, `ipc.DAGEngine`), and CI validated against real Redis service container. All prior council conditions confirmed satisfied in commit `410b8f8`. Shutdown ordering structurally sound via `sync.WaitGroup` (`executor.go:20,69`) and context cancellation chain (`main.go:81-93`), though runtime leak detection (goleak) is absent. Successfully rebutted SCOPE-CRITIC's open-issues claim by demonstrating 5 of 6 cited issues were already implemented on the branch.

### Critic Positions
**SCOPE-CRITIC**: Opened with 5 concerns (E2E monolith, MockStore complexity, gRPC-bypass labeling gaps, DAG completion semantics, 10+ open issues). After factual corrections from ADVOCATE-1 and ARBITER verification, withdrew or downgraded all 5 to post-merge tracking. Final position: unconditional FOR. Key withdrawals: Point 5 (open issues) was factually incorrect — 5 of 6 cited issues already resolved on branch; Point 3 (gRPC bypass) overstated the count (44 not 70) and conflated precondition setup with test scope; Point 1 (E2E split) conceded as style preference after ARCH-CRITIC confirmed no architectural warrant.

**ARCH-CRITIC**: Identified 2 architectural concerns plus 1 informational note: (1) proto types (`pb.DAGExecutionState`) leaked into DAG domain layer at `status.go:23,33,50` — inconsistent with the agent package's boundary pattern at `agent/state.go:14-18`; (2) `OrchestratorServer` takes concrete `*supervisor.Supervisor` at `server.go:30` instead of a 2-method interface, while the same file correctly abstracts `DAGEngine` via interface at `dagengine.go:11-14`; (3) `StatusTracker.records` at `status.go:42` is an unbounded map with no eviction — raised in two prior councils but still untracked in #71-74. All three characterized as post-merge tracking items, not merge blockers. CONDITIONAL FOR with conditions limited to appending notes to existing tracking issues.

### Questioner Findings
QUESTIONER probed 3 claims, all resolved:

1. **SCOPE-CRITIC's "10+ open issues"**: Probed whether #57, #58, #60, #61, #64, #65 were code quality or feature work. ADVOCATE-1 demonstrated 5 of 6 were already implemented on the branch. ARBITER verified via commit history. **Resolved: claim was factually incorrect.**

2. **ADVOCATE-1's "no goroutine leaks"**: Probed for runtime evidence. ADVOCATE-1 provided structural evidence chain (WaitGroup, context cancellation) but conceded no runtime leak detector test exists. **Resolved: qualified as code-inspection confidence, acceptable for MVP.**

3. **ARCH-CRITIC's StatusTracker unbounded memory**: Probed whether already tracked. ARCH-CRITIC confirmed NOT in #71-74 despite being raised in two prior councils. **Resolved: new tracking item, append to #72.**

All claims substantiated or appropriately qualified. No unsubstantiated claims remain.

### Key Conflicts
- **gRPC-bypass labeling scope** — resolved. SCOPE-CRITIC initially argued 70 shim calls across 23 scenarios with only 2 labeled. ARBITER corrected to 44 `ApplyEventForTest` calls (total shim surface 70 across 3 functions). ADVOCATE-1 distinguished precondition setup (42 uses) from scenario bypass (2 labeled scenarios + 1 stress test). SCOPE-CRITIC accepted ADVOCATE-1's alternative: update the package comment to name all three shim functions and clarify the distinction, rather than per-function annotations (which would mark 21 of 23 functions). All parties agreed.

- **Open issues backlog** — resolved. SCOPE-CRITIC claimed 10+ unresolved implementation concerns. ADVOCATE-1 demonstrated 5 of 6 were already implemented (PRs #63, #67, #68 on branch; test exists for #58). SCOPE-CRITIC withdrew the claim, acknowledging it was a bookkeeping gap (GitHub auto-close limitation on non-main branches), not a code quality gap.

- **Pre-merge conditions scope** — resolved. SCOPE-CRITIC's 3 original pre-merge conditions (split E2E file, annotate bypassed tests, triage issues) were all withdrawn or downgraded to post-merge. ARCH-CRITIC's conditions narrowed to issue-note appends only (no code changes). ADVOCATE-1 accepted all tracking items.

### Concessions
- **SCOPE-CRITIC** conceded to **ADVOCATE-1**: Point 5 (open issues) was factually wrong — 5 of 6 already implemented; gRPC-bypass per-function annotation superseded by package comment alternative; E2E file split is style preference, not architectural warrant
- **SCOPE-CRITIC** conceded to **ARCH-CRITIC**: MockStore size is proportional to interface width, not a smell
- **ADVOCATE-1** conceded to **SCOPE-CRITIC**: stress test at `lifecycle_test.go:1460` should carry a bypass annotation for consistency; package comment should be updated to name shim functions
- **ADVOCATE-1** conceded to **ARCH-CRITIC**: proto-in-domain coupling and concrete Supervisor are valid architectural papercuts; both should be tracked
- **ADVOCATE-1** conceded to **QUESTIONER**: "no goroutine leaks" was code-inspection confidence, not runtime-verified
- **ARCH-CRITIC** conceded to **ADVOCATE-1**: E2E file size and MockStore size are not structural defects; SCOPE-CRITIC's pre-merge condition to split E2E was not architecturally warranted

### Regression Lineage
No regression lineage. The prior council's 3 pre-merge fixes (build tag, doc comment, routing comments — all in commit `410b8f8`) introduced no new issues. The current debate's findings are independent of the prior council's remediation.

### Arbiter Recommendation
**FOR**

This council confirms and strengthens the prior council's CONDITIONAL FOR. All 3 prior conditions were satisfied in commit `410b8f8`. The current debate surfaced no correctness defects, no security issues, and no structural problems that warrant blocking merge. SCOPE-CRITIC's strongest concerns (open issues backlog, gRPC-bypass coverage gaps) were factually rebutted — 5 of 6 cited issues were already resolved on the branch, and the precondition-vs-scenario distinction for test shims is valid. ARCH-CRITIC's architectural concerns (proto-in-domain, concrete Supervisor, unbounded StatusTracker) are genuine papercuts but explicitly non-blocking and correctly scoped as tracking items on existing issues. The epic delivers a well-tested MVP orchestrator with clean interface boundaries, CAS-based concurrency safety, and documented limitations with tracking issues. It is ready to merge.

### Suggested Fixes

#### Fixes (all in-PR)
- Update `e2e/lifecycle_test.go` package comment to explicitly name all three `export_e2e.go` shim functions (`ApplyEventForTest`, `CrashAgentForTest`, `CompleteAgentForTest`) and clarify precondition-setup vs. scenario-bypass usage — `e2e/lifecycle_test.go` L:6-8 — LOW — prevents future misreading of gRPC coverage boundaries
  CITE: `e2e/lifecycle_test.go` L:6-8

#### PR Description Amendments
None required. The prior council's "Known MVP Limitations" section is already present and accurate.

#### Post-Merge Issue Updates (recommended, not conditions)
- Append proto-in-domain coupling note to #72: `dag/status.go:23,33,50` uses `pb.DAGExecutionState` directly; should align with `agent/state.go:14-18` boundary pattern
  CITE: `internal/dag/status.go` L:23
- Append StatusTracker unbounded memory eviction note to #72: `status.go:42` `records` map has no TTL/LRU; raised in two prior councils (2026-03-15)
  CITE: `internal/dag/status.go` L:42
- Append concrete Supervisor interface extraction note to #73: `server.go:30` takes `*supervisor.Supervisor` instead of a 2-method `AgentLifecycler` interface; natural extraction point when agent-initiated RPCs are added
  CITE: `internal/ipc/server.go` L:30

#### Post-Merge Housekeeping (recommended)
- Split `e2e/lifecycle_test.go` into per-subsystem files before Epic 2 adds scenarios
- Add `goleak.VerifyNone(t)` to E2E test teardown for runtime goroutine leak detection
- Verify issues #58, #60, #61, #64, #65 auto-close on squash merge to main; manually close any that don't
---
