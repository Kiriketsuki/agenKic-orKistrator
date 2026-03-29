# agenKic-orKistrator

![Version](https://img.shields.io/badge/version-26.03.1.0-blue)
![Go](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)
![Godot](https://img.shields.io/badge/godot-4.x-478CBF?logo=godotengine)
![Tests](https://img.shields.io/badge/tests-none%20yet-lightgrey)
![Build](https://img.shields.io/github/actions/workflow/status/Kiriketsuki/agenKic-orKistrator/version-bump.yml?label=ci)
![License](https://img.shields.io/badge/license-MIT-green)
![Status](https://img.shields.io/badge/status-research%20%2F%20spike-orange)

A magic tower AI orchestrator. AI agents appear as fantasy character classes working inside a vertical tower. Each floor is a project workspace that grows under load. Users see agent activity in real time, route tasks across models (Claude, Gemini, Ollama, OpenAI, DeepSeek), and coordinate multi-agent workflows from a fantasy-themed desktop interface.

> Currently in the **research and spike phase.** No application code exists yet — all decisions are documented in [`docs/research/`](docs/research/).

---

## Stack

| Layer | Technology |
|-------|-----------|
| Desktop UI | Godot 4 + godot-xterm |
| UI-Orchestrator | HTTP/SSE bridge |
| Orchestrator | Go + gRPC |
| IPC | gRPC agent-to-agent + Redis Streams |
| State | Redis (Streams + Hashes) |
| Terminal substrate | tmux |
| Model gateway | LiteLLM + Judge-Router |

## Architecture

Read [`docs/research/Agentic-Orchestrator-MOC.md`](docs/research/Agentic-Orchestrator-MOC.md) for the full architectural decision table, competitive landscape, and rationale before touching any design.

## Versioning

Format: `YY.MM.Major.Minor`

| Event | Effect |
|-------|--------|
| `task/*` or `feature/*` merged → main | Major +1, Minor → 0 |
| `bug/*` or `hotfix/*` merged → main | Minor +1 |
| Monthly rollover | Run **Manual Monthly Version Bump** via Actions |

Current version: `26.03.1.0`

## Development

All work goes through issues. **Never push directly to `main`.**

1. Create an issue using a template — a branch and draft PR are auto-created
2. Work on the branch
3. PR is squash-merged into main (or parent branch for sub-issues)

See [`.github/CI-CD-Guide.md`](.github/CI-CD-Guide.md) for full workflow details.

## Feature Specs

| Spec | Component |
|------|-----------|
| [`go-orchestrator-core-spec.md`](go-orchestrator-core-spec.md) | Supervisor, agent state machine, DAG engine |
| [`terminal-substrate-spec.md`](terminal-substrate-spec.md) | tmux session management, command injection |
| [`model-gateway-spec.md`](model-gateway-spec.md) | LiteLLM proxy, Judge-Router, cost tracking |
| [`pixel-office-ui-spec.md`](pixel-office-ui-spec.md) | Godot 4 magic tower, character class agents, HTTP/SSE bridge |
