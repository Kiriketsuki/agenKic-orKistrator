---
title: Agent Process Management & Supervision
tags: [research, orchestration, supervision, erlang, otp, process-management]
date: 2026-03-14
type: research
---

# Agent Process Management & Supervision

## The Core Philosophy: "Let It Crash"

Erlang/OTP proved over 35 years that **isolation + automatic restart** is superior to defensive coding. For AI agents that may:
- Hallucinate and produce garbage output
- Enter infinite loops
- Exhaust memory or compute resources
- Crash on unexpected tool outputs

The right response is: **let it crash, restart cleanly** — not wrap everything in try/catch.

---

## Erlang/OTP Supervision Trees (The Gold Standard)

Supervisors monitor workers and restart them according to a **strategy**:

| Strategy | Behavior | Use When |
|----------|---------|---------|
| **one-for-one** | Restart only the failed child | Workers are independent |
| **one-for-all** | Restart all children if one fails | Workers share state |
| **rest-for-one** | Restart failed + all started after it | Ordered dependencies |

Supervisors can themselves be supervised — creating a **tree** that isolates failures at the appropriate level. A crashed leaf agent doesn't bring down its supervisor or siblings.

**Apply this pattern even outside Erlang**: design agent processes as independent, restartable units with a supervisor process that owns the restart logic.

---

## Health Probes (Three Required)

Single "is it alive?" checks are insufficient for AI agents. Use three probes:

| Probe | Question | Action on Failure |
|-------|---------|------------------|
| **Liveness** | Is the process alive? (heartbeat) | Restart |
| **Readiness** | Is the model loaded? Is it accepting tasks? | Don't route new tasks |
| **Progress** | Is it making forward progress? (task completion rate) | Restart if stalled |

AI-specific failure modes that single probes miss:
- Hung inference (alive, not progressing)
- Infinite reasoning loop (alive, consuming tokens, no output)
- Model loaded but degraded (hallucinating on every response)

---

## Restart Policies

### Naive Restart (BAD)
Immediate restart on failure → CPU thrashing + supervisor bottleneck when agents fail in rapid succession.

### Exponential Backoff + Jitter (GOOD)
```
1s → 2s → 4s → 8s → 16s → (cap at 60s)
+ jitter: each interval ± 20% to prevent synchronized restart storms
```

### Circuit Breaker
After N consecutive restarts within T seconds:
1. **Stop restarting** — mark agent as failed
2. **Alert human** — don't silently mask systemic bugs
3. **Wait for manual intervention** before accepting tasks again

---

## Process Managers

| Tool | Best For | Key Features |
|------|---------|-------------|
| **supervisord** | Simple Python-based supervision | Config-driven, process groups, event listeners, XML-RPC control |
| **PM2** | Node.js ecosystems | Cluster mode, ecosystem.config.js, real-time monitoring |
| **systemd** | System-level services | Socket activation, journal logging, cgroup resource limits |
| **Custom supervisor** | Full control | Implement OTP-style strategies in Go/Rust/Python |

---

## systemd Socket Activation

Lazily spawn agents only when their socket receives a connection:
1. systemd listens on the socket
2. First connection causes systemd to spawn the agent process
3. Agent inherits the socket fd, processes requests
4. Agent can exit when idle — next connection spawns it again

Ideal for **sporadic analysis agents** that run infrequently — saves memory and startup latency.

---

## Log Aggregation for Agent Fleets

| Approach | Use When |
|---------|---------|
| **journald** | systemd-managed agents, binary indexed logs |
| **Structured JSON logging** | Custom supervisor, log forwarding to centralized store |
| **Redis Streams** | Agent output as a stream, orchestrator reads in real time |
| **TUI dashboard** | Live monitoring of N agents without SSH/grep |

Key: each agent should emit structured log events with `agent_id`, `task_id`, `timestamp`, `level`, `message`. This makes fleet-wide monitoring tractable.

---

## Graceful Shutdown Protocol

```
1. Send SIGTERM to process group (not just parent)
2. Wait for grace period (e.g., 30s)
3. Agent: checkpoint current state, close connections cleanly
4. Escalate to SIGKILL only after timeout
```

Process groups ensure the entire agent subprocess tree receives the signal — not just the parent process leaving orphaned children.

---

## Applying Kubernetes Concepts Locally

| k8s concept | Local equivalent |
|------------|----------------|
| Pod lifecycle | Agent process lifecycle |
| Init containers | Pre-flight checks before agent starts |
| Readiness gates | Health probe before routing tasks |
| Resource limits | cgroup limits via systemd or Docker |
| Restart policies | Exponential backoff in supervisor |

---

## References

- Armstrong, J. "Programming Erlang" — OTP supervision trees
- "Let It Crash" philosophy — Erlang community
- systemd socket activation documentation
- supervisord docs: event listeners, XML-RPC control interface
- PM2 ecosystem.config.js patterns

---

*Authored by: Clault KiperS 4.6*
