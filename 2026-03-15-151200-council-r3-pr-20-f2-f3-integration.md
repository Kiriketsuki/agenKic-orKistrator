## Adversarial Council — PR #20 F2-F3 Integration (Round 3)

> Convened: 2026-03-15T15:12:00Z | Advocates: 1 | Critics: 2 | Rounds: 2/4 | Motion type: CODE

### Motion
PR #20 — F2-F3 Integration: Wire DAG Executor into gRPC Handlers — should be merged into task/1-task-implement-go-orchestrator-core.

Summary of changes:
- Wire F3's dag.Executor into F2's OrchestratorServer, implementing SubmitDAG and GetDAGStatus gRPC handlers
- Add StoreSubmitter adapter bridging dag.TaskSubmitter to state.TaskEnqueuer (MVP: prompt/modelTier dropped)
- Add ErrExecutionNotFound sentinel with proper codes.NotFound mapping
- Fix executor to detach background goroutine from caller's context (server-lifetime context)
- Add Shutdown() method with sync.WaitGroup for graceful drain

### Advocate Positions
**ADVOCATE-1**: The integration is architecturally sound — clean DAGEngine interface decoupling (`dagengine.go:11-14`), correct server-lifetime context pattern (`executor.go:62-67`), thorough gRPC error mapping (`handlers.go:82-116`), immutable data structures with defensive copies (`graph.go:71-86`, `status.go:159-168`), and 18 new tests across three packages (all 58 repo tests pass). The Round 1 blockers (server-lifetime context, dead test) are verified resolved. Both critic-raised bugs (nil TaskSpec, shutdown ordering) are conceded as genuine findings requiring surgical fixes — but neither indicates architectural unsoundness. The PR is merge-ready with two conditions.

### Critic Positions
**CRITIC-1**: Entered with five issues (shutdown ordering, dead ctx parameter, silent field dropping, unbounded StatusTracker map, unbounded goroutine fan-out). After evidence-based exchange, conceded three (dead ctx is Go convention, unbounded map is premature to fix, goroutine cost is negligible) and softened one (silent field drop is not an API contract violation). Holds two merge conditions: nil TaskSpec validation (amplifying CRITIC-2's finding) and shutdown reorder. Credits CRITIC-2 for the novel nil TaskSpec discovery and correct sentinel naming.

**CRITIC-2**: Discovered the nil TaskSpec panic — a novel crashable bug missed by Rounds 1 and 2. Chain: `graph.go:28-34` (no nil check) → `graph.go:89-94` (returns nil Task) → `executor.go:120-122` (nil dereference panic). Also raised post-shutdown Execute concern, which was withdrawn after ADVOCATE-1 demonstrated it's derivative of the shutdown ordering fix. Contributed the correct sentinel naming (`ErrMissingTaskSpec` vs reusing `ErrNodeNotFound`), accepted by all parties.

### Questioner Findings
No claims were marked unsubstantiated. All key claims were independently verified:
- Nil TaskSpec panic chain verified by ARBITER via code inspection (`graph.go:28-34`, `graph.go:89-94`, `executor.go:120-122`).
- Shutdown ordering at `main.go:49-52` verified by ARBITER via code inspection.
- ADVOCATE-1's `sync.WaitGroup` semantics correction (Add/Done valid regardless of Wait state) is consistent with Go stdlib documentation.

### Key Conflicts
- **Shutdown ordering severity** (CRITIC-1 "correctness bug" vs ADVOCATE-1 "observable, not silent") — resolved. CRITIC-1 withdrew "silent" characterization; both agree the failure is observable via StatusTracker but still incorrect. Converged on merge condition.
- **Dead ctx parameter** (CRITIC-1 "API contract lie" vs ADVOCATE-1 "Go convention") — resolved. CRITIC-1 conceded the convention argument and withdrew "dead code" characterization. Downgraded to documentation suggestion.
- **Post-Shutdown Execute** (CRITIC-2 "independent blocker" vs ADVOCATE-1 "derivative of shutdown fix") — resolved. CRITIC-2 withdrew after accepting that fixing shutdown ordering eliminates the reachability via the only call site (`handlers.go:87`).
- **Sentinel naming** (ADVOCATE-1 proposed reusing `ErrNodeNotFound` vs CRITIC-2 proposed `ErrMissingTaskSpec`) — resolved. ADVOCATE-1 conceded that the errors are semantically distinct and withdrew the reuse suggestion.

### Concessions
- **ADVOCATE-1** conceded nil TaskSpec panic to **CRITIC-2** (genuine crashable bug)
- **ADVOCATE-1** conceded shutdown ordering to **CRITIC-1** (incorrect sequence in main.go)
- **ADVOCATE-1** conceded sentinel naming to **CRITIC-2** (ErrMissingTaskSpec, not ErrNodeNotFound)
- **CRITIC-1** conceded dead ctx parameter to **ADVOCATE-1** (standard Go convention)
- **CRITIC-1** conceded silent field dropping framing to **ADVOCATE-1** (internal boundary, not API contract)
- **CRITIC-1** conceded unbounded map to **ADVOCATE-1** (premature to fix without load data)
- **CRITIC-1** conceded goroutine fan-out to **ADVOCATE-1** (negligible cost, cap requires future data)
- **CRITIC-2** conceded post-shutdown Execute to **ADVOCATE-1** (derivative of shutdown ordering fix)
- **CRITIC-2** conceded WaitGroup semantics to **ADVOCATE-1** (Add after Wait is valid Go)

### Arbiter Recommendation
**CONDITIONAL FOR**

Unanimous convergence in 2 rounds (of 4 maximum). All three debaters independently arrived at the same two merge conditions. The PR's architecture — DAGEngine interface decoupling, server-lifetime context pattern, error mapping discipline, immutable data structures — is sound and well-tested. Round 3 surfaced one novel bug (nil TaskSpec panic) that Rounds 1 and 2 missed, plus confirmed the shutdown ordering issue from Round 2 as a required fix rather than a future improvement. Both fixes are surgical (< 10 lines combined) and do not require architectural changes.

**Relation to prior councils**: Round 2 recommended FOR (unanimous, 1 round) and flagged shutdown ordering as "future improvement, not blocker." Round 3 **contradicts** that assessment — the shutdown ordering is a merge condition, not a deferral. Round 3 **adds** the nil TaskSpec panic, which is a genuinely new finding. Round 3 **confirms** all other Round 2 assessments (StatusTracker unbounded map, DAG concurrency limits, raw error messages as future enhancements).

### Conditions (both required before merge)

1. **Nil TaskSpec validation**: Add `ErrMissingTaskSpec = errors.New("dag: node has no task spec")` to `internal/dag/errors.go`. Add nil check for `n.Task` in the `NewGraph` loop at `graph.go:28-34`. Add `errors.Is(err, dag.ErrMissingTaskSpec)` to the `InvalidArgument` mapping at `handlers.go:89-92`. Add a test covering the nil Task path.

2. **Shutdown ordering**: Reorder `cmd/orchestrator/main.go:49-52` from `sv.Stop()` → `executor.Shutdown()` → `server.GracefulStop()` → `cancel()` to `server.GracefulStop()` → `executor.Shutdown()` → `sv.Stop()` → `cancel()`.

### Suggested Fixes

#### Bug Fixes (always in-PR)
1. **Nil TaskSpec dereference** — Add `ErrMissingTaskSpec` sentinel to `internal/dag/errors.go`, nil check in `NewGraph` at `internal/dag/graph.go:28-34`, handler mapping update at `internal/ipc/handlers.go:89-92`, and a test for the nil Task path. (CRITIC-2 finding, verified by ARBITER and all debaters.)
2. **Shutdown ordering** — Reorder `cmd/orchestrator/main.go:49-52` to: `server.GracefulStop()` → `executor.Shutdown()` → `sv.Stop()` → `cancel()`. (CRITIC-1 finding, independently confirmed by CRITIC-2.)

#### In-PR Improvements
No issues identified beyond the two bug fixes above.

#### PR Description Amendments
- Note in the PR description that the StoreSubmitter MVP intentionally drops `prompt` and `modelTier` fields, to be addressed in model gateway integration.

#### New Issues (future features only -- confirm with human)
1. **StatusTracker eviction** — Add TTL-based pruning for completed executions in the `records` map at `internal/dag/status.go:41`. (Round 2 + Round 3 consensus: deferred until load data is available.)
2. **DAG concurrency limits** — Add a semaphore or worker pool to `executor.go:90-99` to cap per-level goroutine fan-out. (Deferred until model gateway rate limits are known.)
3. **Executor shutdown guard** — Add `atomic.Bool` to `Executor` to reject `Execute()` calls after `Shutdown()`. (Defensive measure for future non-gRPC callers.)
4. **DAGEngine.Execute godoc** — Document that the `ctx` parameter governs the synchronous validation phase only; background execution uses the executor's server-lifetime context.
5. **StoreSubmitter startup log** — Log once at construction that `prompt` and `modelTier` are not persisted in the MVP.
6. **Empty dag_id validation** — Validate that `dag_id` is non-empty in `NewGraph` or the handler.
