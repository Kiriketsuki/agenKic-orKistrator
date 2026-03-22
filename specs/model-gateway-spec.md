# Feature: Model Gateway / Router

## Overview

**User Story**: As an orchestrator operator, I want a unified model gateway that routes tasks to the right AI provider (Claude, Gemini, Ollama) based on task complexity so that I get optimal cost-performance tradeoffs across workloads.

**Problem**: Calling each provider's API directly requires per-provider code, manual model selection, and no cost optimization. Without a gateway, every task goes to the most expensive model regardless of complexity.

**Out of Scope**: Agent lifecycle management (orchestrator core), terminal execution (terminal substrate), UI visualization (Godot pixel UI).

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| 1 | | | [ ] |

---

## Scope

### Must-Have
- **LiteLLM proxy integration** — unified `completion()` call across providers: same Go client code works for Claude, Gemini, and Ollama
- **Judge-Router pattern** — Haiku-class model classifies task complexity and routes to appropriate tier: simple task routes to cheap model, complex task routes to frontier model
- **Three routing tiers** (cheap/mid/frontier) with configurable model assignments: tiers defined in config, operator can reassign models without code changes
- **Provider adapters** — context format translation at the boundary (Anthropic, OpenAI, Ollama, Gemini formats): each provider receives correctly formatted requests
- **Cost tracking** — log per-request model, token count, and estimated cost: daily spend breakdown by tier is queryable
- **Fallback chains** — if primary model fails, retry with fallback: when Claude API returns 500, request falls through to Gemini

### Should-Have
- Batch mode for background tasks (30-50% cost reduction)
- Ollama local model support for private/offline workloads

### Nice-to-Have
- Parallel inference + evaluator model for high-stakes decisions
- Streaming vs batching auto-selection based on task type

---

## Technical Plan

**Affected Components**:
- `internal/gateway/` — gateway interface, router logic
- `internal/gateway/router.go` — judge-router implementation
- `internal/gateway/litellm.go` — LiteLLM proxy client
- `internal/gateway/providers/` — per-provider format adapters
- `internal/gateway/cost.go` — cost tracking and reporting
- `config/models.yaml` — tier definitions, model assignments, fallback chains

**API Contracts**:
```go
type Gateway interface {
    Route(ctx context.Context, task TaskSpec) (RoutingDecision, error)
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    GetCostReport(ctx context.Context, period TimePeriod) (CostReport, error)
}
```

**Dependencies**: LiteLLM (deployed as sidecar or external service), `net/http` (stdlib)

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Judge model misclassifies task complexity | Medium | Log routing decisions; add human override flag; tune with feedback loop |
| LiteLLM proxy adds ~10ms latency per call | Low | Acceptable for agent workloads; batch mode amortizes for bulk |
| DeepSeek pricing varies by cache hit/miss (10x difference) | Medium | Track cache-hit rate separately; alert on unexpected cost spikes |

---

## Acceptance Scenarios

```gherkin
Feature: Model Gateway / Router
  As an orchestrator operator
  I want unified model routing with cost optimization
  So that tasks go to the right model at the right cost

  Background:
    Given LiteLLM proxy is running
    And models.yaml defines three tiers: cheap (Haiku), mid (Sonnet), frontier (Opus)

  Rule: Judge-Router classifies and routes tasks

    Scenario: Simple task routes to cheap tier
      Given a task with description "Summarize this 3-line error message"
      When the judge model classifies the task
      Then the task is routed to the cheap tier (Haiku)
      And the routing decision is logged with reason

    Scenario: Complex task routes to frontier tier
      Given a task with description "Design a distributed caching architecture"
      When the judge model classifies the task
      Then the task is routed to the frontier tier (Opus)

  Rule: Unified completion across providers

    Scenario: Complete via Claude provider
      Given a completion request targeting "claude-sonnet-4-6"
      When Complete is called
      Then LiteLLM translates to Anthropic message format
      And returns a CompletionResponse with content and token counts

    Scenario: Complete via Ollama local provider
      Given a completion request targeting "ollama/llama3"
      When Complete is called
      Then the request is sent to localhost:11434
      And returns a CompletionResponse in the same format

  Rule: Fallback chains on provider failure

    Scenario: Primary provider fails, fallback succeeds
      Given Claude API returns HTTP 500
      And the fallback chain for mid tier is [claude-sonnet, gemini-pro]
      When Complete is called
      Then the request falls through to Gemini
      And the response includes a "fallback_used" flag

    Scenario: All providers in chain fail
      Given all providers in the fallback chain return errors
      When Complete is called
      Then an error is returned with all provider errors aggregated

  Rule: Cost tracking

    Scenario: Daily cost report by tier
      Given 800 cheap, 150 mid, and 50 frontier requests were made today
      When GetCostReport is called for today
      Then the report shows per-tier token counts and estimated costs
      And the total is less than the naive all-frontier baseline
```

---

## Task Breakdown

| ID   | Task | Priority | Dependencies | Status  |
|:-----|:-----|:---------|:-------------|:--------|
| T1   | Define Gateway interface and types (tiers, requests, responses) | High | None | pending |
| T2   | LiteLLM proxy client: unified completion call | High | T1 | pending |
| T3   | Provider format adapters (Anthropic, OpenAI, Ollama, Gemini) | High | T2 | pending |
| T4   | Judge-Router: Haiku classifies task -> routes to tier | High | T2 | pending |
| T5   | Config: `models.yaml` for tier definitions, fallback chains | High | T1 | pending |
| T6   | Fallback chain logic: retry with next provider on failure | High | T2, T5 | pending |
| T7   | Cost tracking: per-request logging, daily/period reports | High | T2 | pending |
| T8   | Wire into orchestrator core: supervisor routes tasks through gateway | High | T7 | pending |
| T9   | Integration tests: routing, fallback, cost tracking | High | T8 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass in CI
- [ ] No regressions on related features
- [ ] Judge-Router correctly classifies at least 3 task complexity levels
- [ ] Fallback chains recover from single-provider outage
- [ ] Cost report shows measurable savings vs all-frontier baseline

---

## References

- Research MOC: `docs/research/Agentic-Orchestrator-MOC.md`
- Multi-model coordination: `docs/research/Multi-Model-Coordination.md`
- Coordination patterns: `docs/research/patterns/Multi-Model-Coordination.md`

---
*Authored by: Clault KiperS 4.6*
