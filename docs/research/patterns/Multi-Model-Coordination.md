---
title: Multi-Model AI Coordination
tags: [ai, agents, litellm, ollama, openrouter, routing, multi-model]
type: reference
---

# Multi-Model AI Coordination

## Core Principle

Don't pick one model — route each task to the *right* model. A cheap classifier routing to specialised executors saves 60-90%+ of inference cost (RouteLLM research) with no quality loss on the routed subset.

---

## The Judge-Router-Agent Pattern

```
[Task]
  ↓
[Judge model — cheap, fast e.g. Haiku]
  ↓ classifies task complexity/type
  ↓
[Route to:]
  Cheap model    → bulk/simple tasks
  Mid model      → reasoning tasks
  Frontier model → critical/complex tasks
```

---

## Gateway Layer

### LiteLLM (recommended)
- OpenAI-compatible proxy for 100+ LLMs
- ~10ms overhead per call
- Fallback chains, load balancing, cost tracking
- `litellm.completion(model="claude-3-5-haiku", ...)` — one API for all providers

### OpenRouter
- 290+ models, pass-through pricing (+5.5%)
- Good for provider redundancy and model comparison

---

## Model Capability Matrix

| Tier | Models | Best For |
|------|--------|---------|
| Fast/cheap | Claude Haiku, Gemini Flash, GPT-4o-mini | Classification, bulk, simple Q&A |
| Mid | Claude Sonnet 4.6, GPT-4o | Coding, reasoning, orchestration |
| Frontier | Claude Opus 4.6, o3 | Architecture, deep reasoning |
| Local | Ollama (any model) | Private data, offline, zero cost |
| Long-context | Gemini 3.1 Pro (1M tokens) | Whole-codebase, massive docs |

---

## Local Models: Ollama

```
Orchestrator → LiteLLM → {
  cloud:  Anthropic / OpenAI / Google APIs
  local:  Ollama at http://localhost:11434
}
```

Ollama (v0.14.0+, Jan 2026) supports Claude's Messages API natively. Use for: sensitive data, air-gapped environments, zero marginal cost.

---

## Context Format Translation

LiteLLM handles this automatically, but for custom adapters:

| Provider | Format |
|---------|--------|
| Anthropic | `system` string + `messages[]`, XML encouraged |
| OpenAI | `messages[]` with `system` role |
| Ollama | OpenAI-compatible messages array |
| Gemini | `contents[]` with `parts` |

Keep a canonical `{system, messages[]}` internally; translate at the provider adapter boundary.

---

## Parallel Inference & Evaluation

For high-stakes decisions — send to N models, evaluate with a judge:
- **Majority voting**: simple but ignores confidence
- **Evaluator model (LatentRM)**: judge model scores all responses, picks best — outperforms voting in studied benchmarks (arXiv 2510.07745, 2509.26314)
- Cost: 2-3× single model; worthwhile for critical decisions only

---

## Context Window Strategy

All models suffer "lost in the middle" (76-82% recall mid-context vs 85-95% at edges). Mitigations:
- Put critical info at the **start and end** of context
- Use Gemini 3.1 Pro (1M) for truly massive inputs
- Chunk large inputs and synthesize results across chunks

---

## Pricing Note (as of early 2026)

DeepSeek V3.2 pricing varies significantly by access pattern:
- Cache-hit input: $0.028/1M tokens
- Cache-miss input: $0.28/1M tokens
- Output: $0.42/1M tokens

Don't use a single "cost per 1M tokens" figure for budget planning — always model input vs output split and cache hit rate.

---

## Streaming vs Batching

| Mode | First Token | Total Cost | Use When |
|------|------------|-----------|---------|
| Streaming | ~500ms | Same | Interactive / user-facing |
| Batching | 10-30s | -30-50% | Background / bulk |

---

*Authored by: Clault KiperS 4.6*
