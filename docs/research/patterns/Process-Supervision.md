---
title: Agent Process Supervision
tags: [ai, agents, supervision, erlang, otp, supervisord, pm2, health-checks]
type: reference
---

# Agent Process Supervision

## Philosophy: "Let It Crash"

Borrowed from Erlang/OTP (35 years proven): **isolation + restart** beats defensive coding. When an AI agent hallucinates, loops, or runs out of memory — crash it cleanly and restart rather than adding increasingly complex error handlers.

This works because:
1. Agents are stateless between tasks (state is external)
2. Restart is cheap compared to debugging corrupted internal state
3. Repeated crashes signal a systemic bug worth investigating

---

## Supervision Tree Structure

```
Root Supervisor
├── Agent Group A Supervisor (one-for-one)
│   ├── Agent A1
│   ├── Agent A2
│   └── Agent A3
├── Agent Group B Supervisor (rest-for-one)
│   ├── Agent B1 (coordinator)
│   └── Agent B2 (depends on B1)
└── Infrastructure Supervisor (one-for-all)
    ├── Redis connection
    └── gRPC server
```

**Strategies:**
- `one-for-one` — restart only the failed child
- `one-for-all` — restart all children if one fails (tightly coupled group)
- `rest-for-one` — restart the failed child and all children started after it (ordered dependencies)

---

## Three Health Probes (All Required)

| Probe | Question | Action on Failure |
|-------|---------|------------------|
| **Liveness** | Is the process alive? | Restart |
| **Readiness** | Is the model loaded / ready for tasks? | Don't route new tasks |
| **Progress** | Is it making forward progress (not looping)? | Restart after timeout |

A single health endpoint misses AI-specific failure modes (hung inference, infinite tool loops).

---

## Restart Policy

```
attempt 1: restart after 1s
attempt 2: restart after 2s
attempt 3: restart after 4s
attempt 4: restart after 8s
attempt N: restart after min(2^N, 300)s + jitter
circuit breaker: after 5 restarts in 60s → stop + alert human
```

**Never** restart immediately in a tight loop — it thrashes the CPU and masks the root cause.

---

## Tools

| Tool | Best For |
|------|---------|
| **supervisord** | Config-based, simple, process groups, XML-RPC control |
| **PM2** | Node.js-native, cluster mode, real-time dashboard |
| **systemd** | OS-level, socket activation (lazy agent startup) |
| **Erlang/OTP** | Gold standard if using Elixir/Erlang runtime |

**Socket activation** (systemd): agent sleeps when idle; wakes on first incoming connection. Saves memory for sporadic agents.

---

## Log Aggregation

All agents should emit structured logs (JSON) to a central collector:
- **journald** + **Lazyjournal** (TUI) for local development
- **Loki** + **Grafana** for production fleet visibility
- Log format: `{ timestamp, agent_id, task_id, level, message, tool_call?, duration_ms? }`

---

## Graceful Shutdown

```
1. Send SIGTERM to process group (not just parent)
2. Agent: finish current tool call, checkpoint state, close connections
3. After grace period (e.g. 30s): escalate to SIGKILL
4. Never SIGKILL without grace period — causes checkpoint corruption
```

---

*Authored by: Clault KiperS 4.6*
