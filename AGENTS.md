# AGENTS.md

## Overview

agenKic-orKistrator is a pixel-art AI office orchestrator: a native desktop app where AI agents appear as workers at desks, expose real-time activity, and can be routed across multiple model providers.

This repository is currently in the research and spike phase. There is no application code yet. Architectural decisions live in `docs/research/`, especially `docs/research/Agentic-Orchestrator-MOC.md`.

## Stack

- Godot 4 with `godot-xterm` for the desktop UI
- Go with gRPC for orchestration and IPC
- Redis Streams and Hashes for durable state
- tmux or WezTerm Lua for terminal/pane control
- LiteLLM with a judge-router pattern for multi-model routing

## Commands

No build, run, or test commands exist yet. Update this file once the Go module and Godot project are scaffolded.

## Architecture

Planned architecture:

- `cmd/orchestrator/main.go`: supervisor entrypoint
- `internal/supervisor/`: restart strategies, health probes, supervision tree
- `internal/agent/`: agent lifecycle and state machine
- `internal/dag/`: task DAG execution
- `internal/ipc/`: gRPC services and transport
- `internal/state/`: Redis-backed state and event storage
- `internal/gateway/`: LiteLLM gateway, routing, provider adapters
- `internal/terminal/`: tmux / WezTerm substrate
- `proto/orchestrator.proto`: API contract
- `godot/`: Godot desktop client

Key decisions from the research MOC:

- Supervisor/worker plus DAG orchestration
- gRPC for typed IPC, Redis for durable coordination
- "Context window = RAM"; persistent state externalized
- OTP-style supervision over defensive long-lived processes
- Godot 4 plus `godot-xterm` for the pixel-office UI
- tmux as the primary terminal substrate

## Active Work

Current work is research-driven:

- Validate Godot 4 plus `godot-xterm` as the desktop UI path
- Prototype tmux-based multi-agent orchestration with shared Redis state
- Design the agent state machine and routing algorithm
- Evaluate Anthropic Agent SDK fit for structured agent-to-agent calls

## Git Workflow

All work is issue-driven. Never push directly to `main`.

- Create an issue first, then work from the generated issue branch
- Branch types: `epic/*`, `feature/*`, `task/*`, `bug/*`, `hotfix/*`
- All PRs are squash-merged
