## Adversarial Council — PR #10: F1 Foundation Scaffold, Proto, Redis, Agent State Machine

> Convened: 2026-03-15 | Advocates: 2 | Critics: 1 | Rounds: 4/4 | Motion type: CODE

### Motion
PR #10 (Adding [Feature]: F1 — Foundation: Scaffold, Proto, Redis, Agent State Machine) is correct and ready to merge into task/1-task-implement-go-orchestrator-core.

### Advocate Positions
**ADVOCATE-1**: The PR delivers exactly what F1 promises with exemplary immutability (stateless `Machine` at `machine.go:15-17`, immutable `AgentSnapshot` at `snapshot.go:5-8`), a pure-function transition table (`transition.go:5-18`), exhaustive test coverage (all 12 invalid transitions tested at `transition_test.go:57-96`, full lifecycle roundtrip at `machine_test.go:18-58`), and correct Redis primitives (HSET/HGET, SADD/SMEMBERS, XADD, ZADD/ZPOPMIN). The remaining contested issues (#1 TOCTOU, #4 No Failed state) are F2 supervision concerns, not F1 foundation defects — the spec explicitly scopes supervisor/heartbeat to F2. The implementation matches the F1 spec's transition table exactly, and the "matches the spec" defense applies to architectural decisions (no CAS) while the additional CRUD methods are standard completeness extensions.

**ADVOCATE-2**: The StateStore abstraction is well-factored with a conformance test suite (`store_test.go:15-187`) guaranteeing behavioral equivalence between MockStore and RedisStore — a textbook interface testing approach. The Redis implementation handles subtle edge cases correctly: `redis.Nil` → `ErrAgentNotFound` mapping (`redis.go:101-103`), empty `HGetAll` → `ErrAgentNotFound` (`redis.go:132-135`), atomic `ZPOPMIN` for dequeue (`redis.go:207`), and connection validation at construction (`redis.go:57-63`). Proto and domain types being separate is standard Go gRPC layered architecture — conversion belongs in the handler layer (F2). The Machine's caller-publishes-events pattern is architecturally correct because the Machine lacks `TaskID`/`Payload` context needed for meaningful events.

### Critic Positions
**CRITIC-1**: Eight issues were raised across the debate. The strongest surviving concern is the TOCTOU race in `Machine.ApplyEvent` (`machine.go:31-51`): the read-validate-write sequence has no atomicity guarantee, and the code's statelessness comments actively imply concurrent safety without documenting the single-writer-per-agent constraint — a documentation fix is the minimum, but absence of enforcement means future developers can silently introduce races. Three code-level defects were universally conceded: silent error swallowing (`redis.go:137-138`), `Pipeline()` used where `TxPipeline()` is needed (`redis.go:89,113,149`), and fragile `fmt.Sprintf` conversion (`redis.go:214`). The spec's "logged" requirement (`spec:31`) is unmet by `ApplyEvent` which never calls `PublishEvent`, but this was resolved by consensus to enrich `AgentSnapshot` with `PreviousState` and `Event` so callers can publish complete events. Five issues were conceded: #2 Proto disconnect (documentation gap, not defect), #4 No Failed state (matches spec, F2 concern), #5 Write-only events (intentional scope), #6 Implicit creation (domain invariant enforced at correct layer), #8 logging (caller with full context should publish).

### Questioner Findings
QUESTIONER did not submit probes during the debate. All claim substantiation was driven by direct advocate-critic exchanges and arbiter verification.

### Key Conflicts
- **TOCTOU Race (#1)** — CRITIC-1 said CRITICAL ship-blocker requiring CAS; Advocates said F2 supervision serializes per-agent access, doc comment is sufficient for F1 — **resolved: doc comment before merge, CAS deferred to F2**
- **No Failed State (#4)** — CRITIC-1 said architecturally incompatible with supervision; Advocates cited exact spec match at `spec:73-79` and Erlang/OTP analogy (supervisor owns recovery, not the state machine) — **resolved: CRITIC-1 conceded, F2 design concern**
- **Spec "logged" violation (#8)** — CRITIC-1 said Machine must call PublishEvent per spec; Advocates showed Machine lacks TaskID/Payload context, and moving PublishEvent inside ApplyEvent creates the same atomicity gap — **resolved: enrich AgentSnapshot so callers can publish complete events**
- **AgentSnapshot enrichment timing** — CRITIC-1 and ADVOCATE-2 said pre-merge; ADVOCATE-1 initially said fast-follow — **resolved: ADVOCATE-1 conceded, all agree pre-merge**
- **"Selective spec" defense (#1 Counter A)** — CRITIC-1 argued 7 extra methods beyond spec undermine "matches spec" defense against CAS; Advocates argued CRUD extensions are categorically different from concurrency primitives — **unresolved but moot: doc comment accepted as sufficient**

### Concessions
- **ADVOCATE-1** conceded Issues #3, #7, Pipeline→TxPipeline, and AgentSnapshot enrichment timing (pre-merge) to **CRITIC-1**
- **ADVOCATE-2** conceded Issues #3, #7, Pipeline→TxPipeline, Pipeline "atomicity" characterization imprecision, and MockStore mutation characterization to **CRITIC-1**
- **CRITIC-1** conceded Issues #2 (documentation gap only), #4 (spec-compliant design), #5 (intentional scope boundary), #6 (correct layering), and #8 partially (caller logging architecturally defensible) to **ADVOCATE-1** and **ADVOCATE-2**

### Arbiter Recommendation
**CONDITIONAL**
The PR delivers a well-architected foundation that matches the F1 spec's intent and scope. The debate surfaced three genuine code-level defects (silent error swallowing, Pipeline vs TxPipeline, fragile type conversion), two documentation gaps (concurrency contract, event publishing responsibility), and one API ergonomics improvement (AgentSnapshot enrichment) — all universally agreed upon by the end of the debate. No architectural rework is needed. The TOCTOU concern (#1) is a valid design consideration but is correctly scoped to F2 where the supervisor's concurrency model will be designed; a documentation comment is sufficient for F1. Merge after the six-item fix list is applied on-branch.

### Conditions (if CONDITIONAL)
- All six fixes in the "Suggested Fixes" section below must be applied on the `feature/5-*` branch before squash-merge into `task/1-*`
- Fixes are estimated at ~22 lines total — no interface redesign or architectural rework required

### Suggested Fixes

#### Bug Fixes (always in-PR, regardless of original scope)
- **Handle `strconv.ParseInt` errors** — `internal/state/redis.go:137-138` — Parse errors are silently discarded with `_`, violating the project's "never silently swallow errors" rule. Check both errors and return a wrapped diagnostic.
- **Use `TxPipeline()` instead of `Pipeline()`** — `internal/state/redis.go:89`, `internal/state/redis.go:113`, `internal/state/redis.go:149` — `Pipeline()` is network batching, not transactional. `TxPipeline()` wraps commands in `MULTI/EXEC` for atomicity. Prevents partial-write states (e.g., hash exists without set membership) on connection failure.
- **Use type assertion instead of `fmt.Sprintf("%v")`** — `internal/state/redis.go:214` — `results[0].Member` is `interface{}`; `%v` formatting is fragile and type-unsafe. Use `member, ok := results[0].Member.(string)` with an explicit error on assertion failure.

#### In-PR Improvements (scoped, non-bug)
- **Add concurrency contract comment to `ApplyEvent`** — `internal/agent/machine.go:24-30` (doc comment area) — Document that callers must serialize `ApplyEvent` calls per agentID; concurrent calls for the same agent produce undefined results. The F2 supervisor will enforce this invariant.
- **Add caller event-publishing responsibility comment to `ApplyEvent`** — `internal/agent/machine.go:24-30` (doc comment area) — Document that the Machine handles only state transitions; callers are responsible for publishing domain events via `StateStore.PublishEvent` with full context (TaskID, Payload).
- **Enrich `AgentSnapshot` with `PreviousState` and `Event`** — `internal/agent/snapshot.go:5-8` and `internal/agent/machine.go:51` — The Machine reads `PreviousState` at `machine.go:32-37` and receives `Event` as a parameter, then discards both. Add `PreviousState AgentState` and `Event AgentEvent` fields to `AgentSnapshot` and populate them in the return statement. This enables callers to publish complete domain events without redundant store reads.

#### PR Description Amendments (update scope/intent)
- Note in the PR description that `Machine.ApplyEvent` follows a caller-publishes-events pattern — the Machine validates and persists state transitions; the supervisor (F2) will publish domain events with full business context.

#### New Issues (future features/enhancements only — confirm with human before creating)
> NEVER list bugs here. Confirm with team lead before filing.
- **Add `CompareAndSetAgentState` to StateStore for optimistic locking** — The TOCTOU race in `ApplyEvent` is mitigated by F2's single-writer-per-agent supervision model, but a CAS primitive would provide defense-in-depth. Design should be driven by the F2 supervisor's concurrency model. — Feature
- **Add event stream consumption methods to StateStore** — `ReadEvents`/`SubscribeEvents` using Redis `XREAD`/`XREADGROUP`. Design should be driven by actual consumer requirements in F2-F5. — Feature
