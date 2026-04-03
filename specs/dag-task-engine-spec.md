# Feature: DAG Task Engine

## Overview

**Parent Task**: [Task #1: Implement Go Orchestrator Core](https://github.com/Kiriketsuki/agenKic-orKistrator/issues/1)

**User Story**: As an orchestrator operator, I want to submit multi-step workflows as directed acyclic graphs (DAGs) so that tasks with dependencies execute in topological order and independent tasks run in parallel, completing complex workflows efficiently.

**Problem**: Without a DAG engine, the orchestrator can only handle flat, independent tasks. Multi-step AI workflows — where one agent's output feeds another's input — have no mechanism for dependency resolution, ordering, or parallel execution.

**Out of Scope**: Complex graph scheduling beyond linear + single-fork DAGs (deferred), health probes (F4), E2E tests (F5). Depends on F2 (gRPC server, supervisor).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | Should DAG execution state be stored in Redis (for restart recovery) or only in-process at MVP? | — | [ ] |
| 2 | How are inter-node outputs passed — via Redis, direct gRPC, or stored payload in task spec? | — | [ ] |

---

## Scope

### Must-Have
- **DAG spec definition**: `DAGSpec` proto message with nodes (task specs) and edges (dependency pairs)
- **Topological sort**: validates DAG (no cycles), computes correct execution order
- **Parallel fork execution**: nodes with no unresolved dependencies execute concurrently using goroutines
- **SubmitDAG gRPC endpoint**: accepts a DAGSpec, returns a DAGExecutionID, begins execution
- **Fail-fast failure mode**: when any node fails, all pending and running sibling nodes are cancelled; DAG is marked failed; caller must retry the whole DAG

### Should-Have
- DAG execution status queryable via GetDAGStatus gRPC endpoint
- DAG state (node statuses, execution ID) stored in Redis for observability

### Nice-to-Have
- Partial DAG re-execution from a checkpoint (retry from failed node rather than full restart)
- Fan-in/fan-out patterns beyond single-level forks

---

## Technical Plan

**Affected Components**:
- `internal/dag/dag.go` — DAGSpec parsing, graph representation
- `internal/dag/topo.go` — topological sort (Kahn's algorithm)
- `internal/dag/executor.go` — parallel node execution, fan-out, fail-fast cancellation
- `internal/ipc/handlers.go` — SubmitDAG handler (added to existing gRPC server)
- `proto/orchestrator.proto` — DAGSpec, DAGNode, DAGEdge, DAGExecutionID messages (defined in F1)

**DAGSpec Structure** (proto):
```proto
message DAGSpec {
  repeated DAGNode nodes = 1;
  repeated DAGEdge edges = 2;
}

message DAGNode {
  string id = 1;
  TaskSpec task = 2;
}

message DAGEdge {
  string from_node_id = 1;  // dependency
  string to_node_id   = 2;  // dependent
}
```

**Execution Algorithm**:
1. Parse `DAGSpec` into adjacency list + in-degree map
2. Validate: detect cycles (topological sort fails if cycle exists)
3. Initialise ready queue with zero-in-degree nodes
4. For each ready node: submit task via supervisor (F2)
5. On node completion: decrement in-degree of dependents; enqueue newly-ready nodes
6. On node failure: cancel all in-flight goroutines via context; mark DAG failed

**Fail-Fast Cancellation**:
- Each node runs in a goroutine with a shared `context.Context`
- Root context is cancelled on first node failure
- Running nodes detect cancellation and stop; pending nodes are never started

**Dependencies**: F2 (supervisor task assignment, gRPC server)

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Cycle detection missed in complex graphs | Low | Kahn's algorithm is deterministic; add fuzz tests with cycle injection |
| Goroutine leak if cancel not propagated | Medium | All node goroutines receive shared context; executor waits on WaitGroup |
| Fan-out explosion on large DAGs at MVP | Low | MVP scope: linear + single-fork only; document limit explicitly |

---

## Acceptance Scenarios

```gherkin
Feature: DAG Task Engine
  As an orchestrator operator
  I want to submit task DAGs
  So that multi-step workflows execute in dependency order

  Background:
    Given the orchestrator gRPC server is started
    And 3 idle agent workers are available

  Rule: Topological execution

    Scenario: Linear DAG executes A → B → C
      Given a DAGSpec with nodes [A, B, C] and edges [A→B, B→C]
      When SubmitDAG is called
      Then task A executes first
      And task B executes only after A completes
      And task C executes only after B completes

    Scenario: Cyclic DAG is rejected
      Given a DAGSpec with nodes [A, B] and edges [A→B, B→A]
      When SubmitDAG is called
      Then the server returns an InvalidArgument error
      And no tasks are executed

  Rule: Parallel fork execution

    Scenario: Independent nodes run in parallel
      Given a DAGSpec with nodes [A, B, C, D] and edges [A→B, A→C, B→D, C→D]
      When task A completes
      Then tasks B and C begin execution concurrently
      And task D executes only after both B and C complete

    Scenario: Fork executes faster than sequential
      Given a DAGSpec [A → (B, C) → D] with B and C each taking 1 second
      When the DAG executes
      Then the total wall-clock time for B+C is ≤ 1.5 seconds (parallel)

  Rule: Fail-fast on node failure

    Scenario: Failed node cancels pending siblings
      Given a DAGSpec [A → (B, C) → D] where B fails
      When B returns an error
      Then C is cancelled before it starts (or mid-execution)
      And D is never started
      And the DAG is marked as "failed"

    Scenario: Whole-DAG retry after failure
      Given a failed DAGSpec execution
      When SubmitDAG is called again with the same DAGSpec
      Then execution starts fresh from node A
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T6 | DAGSpec proto messages (DAGNode, DAGEdge, DAGExecutionID) — already in proto from F1 | High | F1 | pending |
| T6.1 | `internal/dag/`: graph representation + Kahn's topological sort | High | F1 | pending |
| T6.2 | Parallel executor: goroutine fan-out, WaitGroup, context-based fail-fast | High | T6.1, F2 | pending |
| T6.3 | SubmitDAG gRPC handler wired into ipc/handlers.go | High | T6.2 | pending |
| T6.4 | DAG unit tests: linear order, parallel fork, cycle detection, fail-fast propagation | High | T6.3 | pending |

---

## Exit Criteria

- [ ] Linear DAG [A→B→C] executes in correct order
- [ ] Parallel fork [A→(B,C)→D] runs B and C concurrently
- [ ] Cyclic DAG returns InvalidArgument error
- [ ] Node failure cancels all pending/running siblings
- [ ] DAG marked "failed" after any node failure
- [ ] All DAG unit tests pass
- [ ] `go vet ./...` and `go build ./...` pass cleanly

---

## References

- Parent spec: `specs/go-orchestrator-core-spec.md`
- F2 dependency: `specs/grpc-supervisor-spec.md`
- Orchestration patterns: `docs/research/Agentic-Orchestration-Patterns.md`
- Process supervision: `docs/research/Process-Supervision.md`

---
*Sub-feature of Task #1 · Go Orchestrator Core*
