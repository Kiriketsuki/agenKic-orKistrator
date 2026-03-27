## Adversarial Council — PR #20 F2-F3 Integration (Round 4)

> Convened: 2026-03-15T15:52:00Z | Advocates: 1 | Critics: 2 | Rounds: 4/4 | Motion type: CODE

### Motion
PR #20 — F2-F3 Integration: Wire DAG Executor into gRPC Handlers — should be merged into task/1-task-implement-go-orchestrator-core. Round 4: post-fix validation after Round 3 conditions were applied.

### Round 3 Fix Validation

Both Round 3 conditions were implemented in commit 602b8c8. ARBITER verified each against the codebase before debate began:

| Condition | Required Fix | Commit 602b8c8 | Verified |
|-----------|-------------|-----------------|----------|
| Nil TaskSpec validation | `ErrMissingTaskSpec` sentinel + nil check + handler mapping + tests | `errors.go:17-18`, `graph.go:32-34`, `handlers.go:93`, `graph_test.go:49-58`, `server_test.go:250-272` | Yes |
| Shutdown ordering | GracefulStop -> Shutdown -> Stop -> cancel | `main.go:49-52` | Yes |

All 58 tests pass (`go test ./...`). The fixes are correctly scoped (6 files, +44/-3 lines) with no unrelated modifications.

### Advocate Positions
**ADVOCATE-1**: Both Round 3 fixes are correctly implemented and tested. Conceded two items as pre-merge fixes: (C1-2) non-deterministic node ordering in `recordToSnapshot` (`status.go:199-208`) — 2-line `sort.Slice`; (C1-1) Graph godoc correction from "immutable, validated DAG" to "validated DAG" at `graph.go:9` — 1-line edit making the contract honest. All other critic findings are either re-litigation of Round 3 deferrals, pre-existing proto design from F1, standard DAG engine behavior, or theoretical concerns with no reachable code path. Factually corrected CRITIC-2's IPC coverage attribution (PR improved coverage from 70.0% to 74.2%, not worsened) and DAGSpec.Edges attribution (field from F1 commit 2571e0b, not this PR). Acknowledged Airflow `upstream_failed` correction from CRITIC-2.

### Critic Positions
**CRITIC-1**: Conceded Objections 3 (StoreSubmitter silent drop — Round 3 scope) and 4 (unused ctx — Go interface convention). Maintains two new merge blockers: (C1-1) Graph at `graph.go:9` claims "immutable, validated DAG" but stores mutable proto pointers at `graph.go:35`, falsifying the documented invariant — proposed fix: extract domain struct or correct godoc; (C1-2) non-deterministic node ordering at `status.go:199-208` due to Go map iteration — proposed fix: 2-line `sort.Slice`. Noted CRITIC-2's factual correction on Airflow's `upstream_failed` state but maintained own downgrade of C2-3 per concession discipline.

**CRITIC-2**: Made five concessions (Findings 1, 2, 4, 5, 6) after ARBITER's re-litigation ruling and ADVOCATE-1's factual corrections. Downgraded Finding 3 (orphaned PENDING nodes) from blocker to follow-up. Endorsed CRITIC-1's two remaining blockers (C1-1, C1-2). Proposed concrete ~5-line fix for C2-3 using existing `FAILED` state with descriptive error message, but accepted it is not a merge gate.

### Questioner Findings
QUESTIONER did not submit probes. Per council protocol, no claims are marked unsubstantiated — no QUESTIONER-based constraints bind this recommendation. Key factual claims were independently verified by the ARBITER:

- `graph.go:35` stores raw `*pb.DAGNode` pointers from caller's spec — confirmed by code inspection
- `status.go:199-208` iterates Go map producing non-deterministic order — confirmed by code inspection
- Protobuf `repeated` fields preserve insertion order at the wire level — confirmed (ADVOCATE-1's contrary claim was factually incorrect and withdrawn)
- DAGSpec.Edges field originates from F1 commit 2571e0b, not this PR — confirmed by `git show`
- IPC coverage: F2 baseline 70.0%, this PR improved to 74.2% — confirmed by `go test -cover` on both commits
- ADVOCATE-1's Airflow claim ("tasks show `no_status`") — challenged by CRITIC-2 who cited Airflow's `upstream_failed` state. ARBITER notes the correction is consistent with Airflow documentation. ADVOCATE-1's broader argument (FAILED execution + PENDING node is interpretable) remains defensible.

### Key Conflicts

- **C1-1: Mutable proto refs** (CRITIC-1 "falsifies immutability contract" vs. ADVOCATE-1 "no reachable mutation path, `internal/` scope") — **resolved by convergence in final summaries**. CRITIC-1 argued the exported `NewGraph` function's godoc creates a contract that the implementation doesn't enforce. ADVOCATE-1 demonstrated only one production caller exists, all within `internal/`, and no code path can trigger mutation. In final summaries, ADVOCATE-1 explicitly accepted CRITIC-1's Option B: change godoc from "immutable, validated DAG" to "validated DAG" (1-line edit). All parties agree on this resolution.

- **C1-2: Non-deterministic ordering** (CRITIC-1 "server-introduced randomness in API response" vs. ADVOCATE-1 "no client exists, fix later") — **resolved by concession**. ADVOCATE-1 conceded after CRITIC-1 and CRITIC-2 corrected the protobuf ordering fact. ADVOCATE-1 agreed the 2-line `sort.Slice` fix should be applied pre-merge.

- **C2-3: Orphaned PENDING nodes** (CRITIC-2 "observability gap, polling ambiguity" vs. ADVOCATE-1 "correct behavior, FAILED state would be less accurate") — **resolved by downgrade**. CRITIC-2 downgraded from blocker to follow-up after acknowledging the semantic point that PENDING accurately means "never attempted." ADVOCATE-1's Airflow precedent was factually inaccurate (Airflow uses `upstream_failed`), but the broader argument that FAILED + PENDING is interpretable was accepted by both critics. CRITIC-2's proposed fix (mark as FAILED with "skipped: upstream failure" error message) was noted as concrete and small.

### Concessions

**Round 4 concessions:**
- **ADVOCATE-1** conceded C1-2 (non-deterministic ordering) to **CRITIC-1** — server-introduced randomness, trivial fix
- **ADVOCATE-1** conceded C1-1 (immutability claim) to **CRITIC-1** — accepted Option B: godoc correction from "immutable, validated DAG" to "validated DAG"
- **ADVOCATE-1** withdrew the `http.Request` analogy for C1-1
- **ADVOCATE-1** acknowledged Airflow `upstream_failed` correction from CRITIC-2
- **ADVOCATE-1** acknowledged protobuf ordering claim was factually incorrect
- **CRITIC-1** conceded Objection 3 (StoreSubmitter) to **ADVOCATE-1** — Round 3 scope
- **CRITIC-1** conceded Objection 4 (unused ctx) to **ADVOCATE-1** — Go interface convention
- **CRITIC-1** downgraded C2-3 (orphaned PENDING) — ADVOCATE-1's industry precedent argument persuasive
- **CRITIC-2** conceded Finding 1 (memory leak) — re-litigation per ARBITER ruling
- **CRITIC-2** conceded Finding 2 (DAGSpec.Edges) — F1 attribution corrected by ADVOCATE-1
- **CRITIC-2** conceded Finding 4 (dag_id validation) — re-litigation per ARBITER ruling
- **CRITIC-2** conceded Finding 5 (unbounded concurrency) — re-litigation per ARBITER ruling
- **CRITIC-2** conceded Finding 6 (IPC coverage) — attribution corrected by ADVOCATE-1
- **CRITIC-2** downgraded Finding 3 (orphaned PENDING) to follow-up — semantic point accepted

### Arbiter Recommendation
**CONDITIONAL FOR**

Round 4 confirms that both Round 3 conditions were correctly implemented. The fixes are precisely scoped, well-tested, and introduce no regressions. The debate surfaced five genuinely new findings (C1-1, C1-2, C2-3, C2-2, C2-6) not raised in Rounds 1-3. After extensive rebuttal, the debate reached **unanimous convergence** on two conditions — all three debaters independently arrived at the same resolution in their final summaries.

**C1-2 (non-deterministic ordering)**: All parties agree. The fix is trivial (2-line `sort.Slice`), the defect is real (server-introduced non-determinism in an API response), and ADVOCATE-1 conceded it should be fixed pre-merge.

**C1-1 (mutable proto refs)**: All parties agree. ADVOCATE-1 explicitly accepted CRITIC-1's Option B in the final summary: correct the godoc at `graph.go:9` from "immutable, validated DAG" to "validated DAG" (1-line edit). This makes the contract honest about what the implementation enforces. Full immutability enforcement via domain struct extraction is recommended as a follow-up.

**Relation to prior councils**: Round 3 recommended CONDITIONAL FOR (unanimous, 2/4 rounds) with two conditions. Round 4 **confirms** both Round 3 conditions were correctly implemented. Round 4 **adds** two new conditions (C1-2 sort fix, C1-1 immutability resolution) and one recommended follow-up (C2-3 orphaned PENDING). Round 4 **upholds** all six Round 3 future enhancement deferrals — CRITIC-2's attempt to re-litigate three of them (StatusTracker eviction, dag_id validation, concurrency limits) was ruled out of scope by the ARBITER given no new evidence. Round 4 **corrects** ADVOCATE-1's protobuf ordering claim (repeated fields do preserve insertion order) and Airflow precedent claim (Airflow uses `upstream_failed`, not `no_status`).

### Conditions (both required before merge)

1. **Deterministic node ordering**: Add `sort.Slice(nodeStatuses, func(i, j int) bool { return nodeStatuses[i].NodeID < nodeStatuses[j].NodeID })` to `recordToSnapshot` at `internal/dag/status.go:207`, before the return statement. (ADVOCATE-1 concession, CRITIC-1 finding, all parties agree.)

2. **Resolve immutability claim**: Either (a) correct the godoc at `internal/dag/graph.go:9` from "immutable, validated DAG" to "validated DAG" (removing the unenforceable immutability claim), OR (b) clone `TaskSpec` in `NewGraph` after the nil check at `graph.go:32-34` to enforce the claim. Implementer's choice. (CRITIC-1 finding, all parties agree status quo should not persist.)

### Suggested Fixes

#### Bug Fixes (always in-PR)
1. **Non-deterministic node ordering** — `sort.Slice` in `recordToSnapshot` at `internal/dag/status.go:207`. (CRITIC-1 finding, ADVOCATE-1 conceded.)
2. **Immutability claim mismatch** — Godoc correction or TaskSpec clone at `internal/dag/graph.go:9,32-34`. (CRITIC-1 finding, disputed severity but all parties agree the mismatch should be resolved.)

#### In-PR Improvements
1. **Orphaned PENDING nodes** — After the loop break at `executor.go:103-105`, mark remaining unstarted nodes as FAILED with error message "skipped: upstream failure" using existing `MarkNodeFailed`. ~5 lines, no proto change. (CRITIC-2 finding, downgraded from blocker by CRITIC-2, ADVOCATE-1 disputes semantic accuracy but acknowledges the observability improvement.)

#### PR Description Amendments
- Note in the PR description that the StoreSubmitter MVP intentionally drops `prompt` and `modelTier` fields (carried forward from Round 3).

#### New Issues (future features only — confirm with human)
1. **Graph domain struct extraction** — Extract a `GraphNode` domain struct from proto `DAGNode` in `NewGraph`, severing all mutable proto references. Full immutability enforcement at the type level. (CRITIC-1 proposed, ADVOCATE-1 acknowledged as valid refactoring.)
2. **StatusTracker eviction** — Add TTL-based pruning for completed executions in `records` map at `internal/dag/status.go:41`. (Round 3 consensus, re-confirmed in Round 4.)
3. **DAG concurrency limits** — Add semaphore or worker pool to `executor.go:64-68` to cap concurrent DAG executions. (Round 3 consensus, re-confirmed in Round 4.)
4. **Executor shutdown guard** — Add `atomic.Bool` to reject `Execute()` calls after `Shutdown()`. (Round 3 consensus.)
5. **DAGEngine.Execute godoc** — Document that the `ctx` parameter governs synchronous validation only. (Round 3 consensus.)
6. **StoreSubmitter startup log** — Log once that `prompt` and `modelTier` are not persisted in the MVP. (Round 3 consensus.)
7. **Empty dag_id validation** — Validate `dag_id` is non-empty in `NewGraph` or the handler. (Round 3 consensus.)
8. **DAGSpec.Edges field** — Decide whether to validate, remove, or support the `edges` field alongside `depends_on`. Proto evolution question from F1. (CRITIC-2 finding, attributed to F1 commit 2571e0b.)
9. **Orphaned PENDING observability** — If in-PR improvement is not applied: add descriptive state for nodes skipped due to upstream failure, either via error message on FAILED state or a new SKIPPED proto state. (CRITIC-2 finding, downgraded from blocker.)
10. **IPC test coverage** — Bring `internal/ipc` coverage from 74.2% to 80%+ threshold. Gap inherited from F2, not introduced by this PR. (CRITIC-2 finding, attribution corrected.)
