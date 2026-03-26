---
## Adversarial Council — PR #24: Epic Merge to Main

> Convened: 2026-03-26 | Advocates: 1 | Critics: 2 | Rounds: 3/4 | Motion type: CODE

### Motion
Merge PR #24 (epic: implement go orchestrator core) into main. This is an epic branch integrating all sub-features (#9 E2E lifecycle tests, #15 CompareAndSetAgentState for optimistic locking, #17 Event stream consumption methods, #19 Wire DAG Executor into gRPC handlers) into the main branch. 15,407 additions across 120 files.

### Advocate Positions
**ADVOCATE-1**: The epic delivers a complete, well-tested Go orchestrator core. Key strengths: stateless agent machine with CAS-based transitions, per-agent mutexes in the supervisor, 6 explicit re-enqueue paths with dedicated E2E tests (no silent task loss), 23 E2E + ~90 unit tests with race detection, clean interface boundaries designed for future `TaskSubmitter` swap, and a well-defined gRPC contract with 7 RPCs. CI green (one transient protoc download failure). Conceded build tag fix, tracking issues for MVP deferrals, and handler routing documentation.

### Critic Positions
**SCOPE-CRITIC**: Identified 5 concerns. Withdrew 2 (StreamOutput stub, StoreSubmitter field dropping) after accepting ADVOCATE-1's documented-MVP-deferral defense. Downgraded flaky test from hard blocker to "fix before merge." Narrowed heartbeat RPC and E2E gRPC bypass to spec ambiguities requiring tracking issues. Final position: CONDITIONAL MERGE with build tag fix and tracking issues.

**ARCH-CRITIC**: Identified 4 concerns. Withdrew 2 (god interface — narrow consumer interfaces already exist; dual write paths — defensible producer-consumer separation). Conceded duplicate Clock as trivial. Narrowed DAG false-completion to a documentation fix: `StoreSubmitter` must document that it provides enqueue-only semantics, not completion-tracking. Final position: CONDITIONAL with documentation fix.

### Questioner Findings
QUESTIONER did not submit probes. The ARBITER independently verified all cited claims against source code:
- `StateStore` interface: 16 methods at `internal/state/store.go:68-116` — confirmed
- Dual write paths in `handlers.go:35` (store-direct) vs `handlers.go:107` (supervisor) — confirmed
- DAG `executeNode` marks completed at enqueue: `executor.go:139` after `storesubmitter.go:25-27` — confirmed
- Duplicate `Clock`: `restart.go:9` and `status.go:12` — confirmed
- No heartbeat RPC in `proto/orchestrator.proto` — confirmed (zero matches)
- `executor_test.go` missing `//go:build testenv` tag — confirmed (line 1 is `package dag_test`)
- Flaky test: ARBITER ran `TestExecutor_ParallelFork` — passed at 49ms vs 50ms threshold (1ms margin)
- `export_e2e.go` test backdoors gated behind `//go:build testenv` — confirmed
- **Executor blocking compatibility**: ARBITER verified `executor.go:100-106` spawns per-node goroutines with `wg.Wait()`. Mock submitter at `executor_test.go:38-43` demonstrates blocking via `time.After(delay)`. A blocking `TaskSubmitter` requires zero executor changes. ARCH-CRITIC's claim that this "requires a design change to the executor's core loop" is **not supported** by the code.

### Key Conflicts
- **DAG "completed = enqueued" semantics** — resolved. Both sides agree the semantic gap exists. ADVOCATE-1 demonstrated the executor already supports blocking `TaskSubmitter` implementations (verified by ARBITER). ARCH-CRITIC narrowed to requiring documentation, not code change. All parties agree a tracking issue is needed.
- **Heartbeat RPC absence** — resolved. Spec's API contract section (`go-orchestrator-core-spec.md:58-63`) does not list a heartbeat RPC. Spec's acceptance scenario (`go-orchestrator-core-spec.md:96-100`) implies agents can heartbeat. This is a spec ambiguity, not a code defect. All parties accept a tracking issue.
- **Flaky timing test** — resolved. Not deterministic (1ms margin), but a flaky test on main erodes CI trust. All parties agree: fix before merge.
- **God interface / dual authority** — resolved. ARCH-CRITIC withdrew both objections after ADVOCATE-1 demonstrated narrow consumer interfaces exist (`dag.TaskEnqueuer`) and the store-direct vs supervisor routing is a defensible producer-consumer pattern.

### Concessions
- **ADVOCATE-1** conceded to **SCOPE-CRITIC**: build tag fix needed; tracking issues needed for heartbeat RPC, StreamOutput, TaskSpec persistence, and agent-initiated RPCs
- **ADVOCATE-1** conceded to **ARCH-CRITIC**: DAG completion semantic gap is real; handler routing documentation needed; duplicate Clock is valid
- **SCOPE-CRITIC** conceded to **ADVOCATE-1**: error handling in supervisor is thorough; StreamOutput stub acceptable as MVP; StoreSubmitter field-dropping acceptable (interface correctly designed); E2E gRPC bypass acceptable (tests honest about limitations); test failure is flaky, not deterministic
- **ARCH-CRITIC** conceded to **ADVOCATE-1**: god interface withdrawn (narrow interfaces exist); dual authority withdrawn (defensible design); DAG concern narrowed to documentation only

### Regression Lineage
No regression lineage — no prior fix involvement. This is the initial epic merge.

### Arbiter Recommendation
**CONDITIONAL FOR**

The epic is well-engineered: stateless agent machine with CAS locking, comprehensive error re-enqueue paths, per-agent mutexes, and a clean interface boundary (`TaskSubmitter`) designed for future blocking implementations. The debate revealed no bugs or correctness defects in the current code. The DAG "completed = enqueued" semantic is an MVP limitation, not a defect — the executor's goroutine-per-node model already supports blocking `TaskSubmitter` implementations without code changes (verified against source). Both critics moved to CONDITIONAL after the advocate demonstrated these points. The two pre-merge fixes are trivial (one line + one doc comment), and the tracking issues formalize documented MVP deferrals.

### Conditions

#### Pre-merge fixes (2)
1. **Add `//go:build testenv` to `internal/dag/executor_test.go`** — matches pattern in `supervisor_test.go:1` and `e2e/lifecycle_test.go:1`. Prevents flaky timing test from failing `go test ./...` on main.
2. **Add doc comment to `internal/dag/storesubmitter.go`** clarifying that `StoreSubmitter` provides enqueue-only MVP semantics and does NOT provide the completion-tracking behavior that `MarkNodeCompleted` in the executor implies. Future `TaskSubmitter` implementations should block until actual task completion.

#### Pre-merge documentation (1)
3. **Add routing-rule comments to `internal/ipc/handlers.go`** explaining which handlers route to the supervisor (agent state transitions) vs. the store directly (stateless queue/read operations), and why.

#### Post-merge tracking issues (4)
4. **Heartbeat RPC** — Add server-side endpoint for agent liveness refresh. Supervisor monitors `LastHeartbeat` (`supervisor.go:143-184`) but no RPC exists to update it.
5. **StreamOutput business logic** — Current handler is echo-only (`handlers.go:62-79`). Integrate with task completion flow when terminal substrate provides output consumers.
6. **TaskSpec persistence** — `StoreSubmitter` discards `prompt` and `modelTier` (`storesubmitter.go:25-27`). Model gateway epic must provide a `TaskSubmitter` that persists full `TaskSpec`.
7. **Agent-initiated state transition RPCs** — No gRPC handlers for `assigned->working` (`EventWorkStarted`) or `working->reporting` (`EventOutputReady`). Design and add when agent client implementation begins.

### Suggested Fixes

#### Fixes (all in-PR)
- Add build tag `//go:build testenv` to executor_test.go — `internal/dag/executor_test.go` L:1 — LOW — prevents flaky test on main
  CITE: `internal/dag/executor_test.go` L:1
- Add StoreSubmitter doc comment about enqueue-only semantics — `internal/dag/storesubmitter.go` L:11-13 — LOW — prevents future misreading of completion semantics
  CITE: `internal/dag/storesubmitter.go` L:11
- Add handler routing comments — `internal/ipc/handlers.go` L:17 — LOW — documents store-direct vs supervisor routing rationale
  CITE: `internal/ipc/handlers.go` L:17

#### PR Description Amendments
- Add a "Known MVP Limitations" section listing: (1) DAG status reports enqueue, not execution completion; (2) StreamOutput is echo-only stub; (3) No heartbeat RPC; (4) No agent-initiated state transition RPCs. This sets expectations for downstream epic consumers.

#### Critical Discoveries (informational)
None identified. No security, data-loss, or compliance issues found.
---
