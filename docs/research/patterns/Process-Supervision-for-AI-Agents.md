---
title: Process Supervision for AI Agents
tags: [ai, agents, supervision, otp, erlang, supervisord, pm2, health-checks]
date: 2026-03-14
type: evergreen
---

# Process Supervision for AI Agents

## The Core Principle: Let It Crash

Erlang/OTP proved over 35 years that **isolation + restart beats defensive coding**. For AI agents — which can hallucinate, loop, or exhaust resources in unpredictable ways — this is especially true. Design agents to crash cleanly and be restarted, not to handle every failure case internally.

## Supervision Tree Model (OTP-Inspired)

```
Root Supervisor
├── Agent Group A Supervisor
│   ├── Agent A1
│   ├── Agent A2
│   └── Agent A3
└── Agent Group B Supervisor
    ├── Agent B1
    └── Agent B2
```

**Restart strategies:**
- **One-for-one**: restart only the failed child (most common)
- **One-for-all**: restart all children when one fails (tightly coupled groups)
- **Rest-for-one**: restart the failed child and all children started after it (dependency chains)

Hierarchical trees mean a group supervisor can fail without taking down the whole fleet.

## Three Health Probes (Not One)

A single health endpoint misses AI-specific failure modes:

| Probe | Question | Action on Failure |
|-------|---------|------------------|
| **Liveness** | Is the process alive? | Restart |
| **Readiness** | Is the model loaded and ready for tasks? | Don't route tasks yet |
| **Progress** | Is the agent making forward progress? (not looping/stalled) | Restart or escalate |

Progress probes need a heartbeat or task-completion signal — a process can be alive and ready but stuck in an infinite reasoning loop.

## Exponential Backoff + Circuit Breaker

Naive restart (immediate) causes CPU thrashing when the failure is systemic:

```
Attempt 1: wait 1s
Attempt 2: wait 2s
Attempt 3: wait 4s
Attempt 4: wait 8s
...
After N failures: OPEN CIRCUIT → alert human, stop restarting
```

The circuit breaker prevents masking systemic bugs with endless restart loops.

## Practical Tools

| Tool | Strengths | Use When |
|------|-----------|---------|
| **supervisord** | Config-based, process groups, XML-RPC control | Simple fleets, Python agents |
| **PM2** | Node.js native, cluster mode, web dashboard | JS/TS agent servers |
| **systemd socket activation** | Lazy spawn on first connection, zero idle cost | Sporadic/infrequent agents |
| **OTP/Erlang** | Gold standard, battle-tested supervision | When using Elixir/Erlang |

For Python/Go/Rust agents on Linux, **supervisord** or **systemd** with socket activation is the pragmatic starting point.

## Graceful Shutdown Protocol

```
SIGTERM → agent.on_shutdown() → flush state → close connections → exit(0)
[grace period, e.g. 30s]
SIGKILL → force terminate
```

Send SIGTERM to the **entire process group** (not just the parent PID) to catch all child processes.

## Log Aggregation

Structured logging (JSON) to stdout → centralised collector (journald / Loki / CloudWatch). Each log line should include:
- `agent_id` — which agent
- `task_id` — which task
- `level` — info/warn/error
- `timestamp` — ISO 8601

This enables real-time dashboards showing all agents' activity in one view without grepping individual files.

## See Also

- [[Agentic-Orchestration-Patterns]] — supervisor/worker dispatch patterns
- [[Agent-State-Management]] — checkpointing before crash

---

*Authored by: Clault KiperS 4.6*
