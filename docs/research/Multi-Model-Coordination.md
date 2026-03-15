---
title: Multi-Model AI Coordination — Claude, Codex, Ollama, Gemini
tags: [research, orchestration, llm, litellm, ollama, multi-model]
date: 2026-03-14
type: research
---

# Multi-Model AI Coordination

## Overview

A production orchestrator routes tasks to the *right* model — not just one model. The Judge-Router-Agent pattern achieves **60-90%+ cost savings** on real workloads (RouteLLM research; exact figure depends heavily on workload composition) by pairing a cheap classifier with specialized execution models.

## The Judge-Router-Agent Pattern

```
[Task] → [Judge/Classifier (Haiku, $0.0002/query)] → routing decision
         ↓              ↓              ↓
  [Cheap model]  [Mid model]  [Frontier model]
  (bulk/simple)  (reasoning)  (critical/complex)
```

Actual production breakdown: 800 cheap queries + 150 medium + 50 expensive = **$1.74/day** vs $15/day naive all-frontier.

## Gateway Layer: LiteLLM & OpenRouter

### LiteLLM
- OpenAI-compatible proxy for 100+ LLMs
- ~10ms overhead per call
- Features: fallback chains, load balancing, cost tracking, retry logic
- Single unified API: `litellm.completion(model="claude-3-5-haiku", ...)`

### OpenRouter
- Aggregates 290+ models at pass-through pricing (+5.5% fee)
- Good for model comparison and routing by capability
- Useful for provider redundancy

## Ollama: Local Model Integration

Ollama now (Jan 2026) supports Claude's Messages API natively, eliminating custom context translation proxies. Integration pattern:

```
Orchestrator → LiteLLM proxy → {
  cloud: Anthropic API / OpenAI API / Google API
  local: Ollama (http://localhost:11434)
}
```

Use Ollama for: sensitive/private data, offline capability, zero marginal cost at inference time.

## Model Capability Matrix

| Model | Best For | Context | Cost Tier |
|-------|---------|---------|-----------|
| Claude 3.5 Haiku | Fast reasoning, bulk tasks | 200K | Low |
| Claude Sonnet 4.6 | Complex coding, orchestration | 200K | Mid |
| Claude Opus 4.6 | Deepest reasoning, architecture | 200K | High |
| GPT-4o | General, multimodal | 128K | Mid-High |
| Codex / o3 | Pure code generation | 128K | Mid |
| Gemini 3.1 Pro | Long-context, multimodal | 1M | Mid |
| Ollama (local) | Private data, offline | Varies | Zero |
| DeepSeek V3.2 | Frontier reasoning | 128K | Very Low ($0.028 cache-hit / $0.28 cache-miss / $0.42 output per 1M tokens) |

## Context Format Translation

Each provider has a different format. LiteLLM handles most of this, but for custom orchestrators:

| Provider | Format |
|---------|--------|
| Anthropic | `system` string + `messages` array, XML encouraged in system |
| OpenAI | `messages` array with `system` role |
| Ollama | modelfile system prompt + messages array (OpenAI-compatible) |
| Gemini | `contents` array with `parts` |

**Recommendation**: Maintain a canonical `{system, messages[]}` format internally, translate at the provider adapter layer.

## Parallel Inference & Evaluator Models

For high-stakes decisions, send to multiple models and evaluate:

- **Majority voting**: cheap but naive (doesn't account for model confidence)
- **Evaluator/LatentRM**: a dedicated judge model scores responses, picks best (arXiv 2510.07745, 2509.26314 — supports the technique; outperforms majority voting in studied benchmarks, though not universally so)
- Cost: 2-3× single model — worthwhile for critical decisions

## Context Window Strategy

- **Gemini 3.1 Pro (1M)** → whole-codebase analysis, massive documents
- **Claude (200K)** → large context with better instruction following
- **GPT-4o (128K)** → moderate context tasks

Warning: All models suffer "lost in the middle" — recall drops to 76-82% for info in the middle of long contexts vs 85-95% at edges. Load critical info at start/end.

## Streaming vs Batching

| Mode | Perceived Latency | Cost | Use When |
|------|------------------|------|---------|
| Streaming | ~500ms first token | Same | Interactive, user-facing |
| Batching | 10-30s total | -30-50% | Background, bulk processing |

## Recommended Stack

1. **LiteLLM** as unified gateway with cost tracking
2. **Haiku-class judge** for task classification/routing
3. **Ollama** for local/private workloads
4. **Evaluator model** for critical decisions
5. **Gemini** for anything requiring >200K context

## References

- LiteLLM docs: proxy, routing, fallback chains
- OpenRouter: model catalogue and pricing
- Ollama Claude integration (Jan 2026)
- LatentRM papers: arXiv 2510.07745, 2509.26314 (evaluator models vs majority voting)
- RouteLLM (2024): cost-routing research, 60-90%+ savings data

---

*Authored by: Clault KiperS 4.6*
