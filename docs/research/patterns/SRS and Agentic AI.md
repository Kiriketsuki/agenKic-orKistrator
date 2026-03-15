---
title: SRS and Agentic AI
type: concept
tags:
  - tech/ai
  - tech/software-engineering
  - tech/agentic-ai
---

## Summary

The intersection of Software Requirements Specification (SRS) and agentic AI is bidirectional. Agentic systems are transforming how requirements are elicited, drafted, and validated. Simultaneously, agentic systems require fundamentally new specification paradigms — traditional deterministic SRS frameworks cannot govern non-deterministic, goal-directed software. This note covers both directions: AI as a requirements engineer, and how to write requirements for AI agents.

## Table of Contents

- [[#The Traditional SRS]]
- [[#Agentic AI Architecture]]
- [[#Specifying Agentic Systems — The New Paradigm]]
- [[#AI as Requirements Engineer]]
- [[#Key Frameworks]]
- [[#Testing Non-Deterministic Agents]]
- [[#Governance and Standards]]
- [[#Related Notes]]
- [[#Sources]]

---

## The Traditional SRS

The SRS (governed by **ISO/IEC/IEEE 29148:2018**) is the single source of truth bridging business stakeholders, developers, QA, and auditors. It has two core layers:

**Functional Requirements (FRs)** — what the system must do. Expressed in deterministic terms: given input X, the system shall produce output Y. Common syntaxes:
- **EARS** (Easy Approach to Requirements Syntax): `When [trigger], the [system] shall [action]`
- **Gherkin / BDD**: `Given [precondition] / When [event] / Then [outcome]` — binary pass/fail

**Non-Functional Requirements (NFRs)** — quality attributes: performance (throughput, latency), security, reliability (uptime, fault tolerance), usability, maintainability.

The fundamental assumption of all traditional SRS methodology is **determinism**: fixed input → fixed output, testable with binary pass/fail. This assumption breaks entirely for agentic systems.

> [!review]- Comprehension Review
> - **Comfort Level**: beginner / intermediate / advanced
> - **Feedback**:

---

## Agentic AI Architecture

An agentic system is distinguished by **operational autonomy, goal persistence, and environmental adaptability**. It operates through a continuous cognitive loop rather than linear script execution:

| Phase | Function |
|:---|:---|
| Perception | Gather data from APIs, databases, sensors, user input |
| Logic Formulation | Extract context, infer system state, identify constraints |
| Goal Setting / Planning | Decompose high-level mandate into executable sub-tasks (chain-of-thought) |
| Decision-Making | Evaluate courses of action against constraints; select optimal probabilistic path |
| Execution (Tool Use) | Invoke external tools: query DB, send email, modify filesystem |
| Adaptation | Monitor outcome; dynamically adjust if error, hallucination, or unexpected state |

Key consequence: the system is **inherently non-deterministic**. Identical inputs may produce entirely different execution paths depending on real-time environment, API state, or subtle probability distribution shifts in the underlying model. This makes traditional SRS structurally inadequate.

> [!review]- Comprehension Review
> - **Comfort Level**: beginner / intermediate / advanced
> - **Feedback**:

---

## Specifying Agentic Systems — The New Paradigm

### Intent Mandates Over Procedural Scripts

Traditional FR: `"The system shall fetch data from API A, transform via Function B, and save to Database C."`

Agentic FR (ReAct pattern): `"The system shall achieve the goal of synchronising client records. It is authorised to autonomously select from the approved tool registry. Execution is deemed successful if the final data state satisfies the target schema validation criteria, regardless of the intermediate tool path chosen."`

The shift: **process → outcome + constraints**.

### Adapting NFRs for Agents

NFRs become the primary governance mechanism. New categories emerge that don't exist in traditional software:

| NFR Category | Example Acceptance Criteria |
|:---|:---|
| Accuracy / Reliability | F1 score ≥ 0.85; hallucination rate < 5% |
| Operational Cost | ≤ 15 API calls per task; compute ≤ $0.50 per transaction |
| Explainability | 80% of SME reviewers find the chain-of-thought explanation satisfactory |
| Bias Mitigation | Automated threat detection flags ≥ 95% of adversarial prompt injections |
| Autonomy Boundaries | Financial transactions > $5,000 require HITL cryptographic approval |

### Specification by Example and BDD for Agents

Structured formats (Pact contracts, Design by Contract invariants, Gherkin) reduce prompt ambiguity. When an agent encounters `Given/When/Then` in its system prompt or config, it inherently maps the formal syntax to bounded execution. Treats the system prompt as a **verifiable engineering artifact** rather than a configuration detail.

### The "Reasonableness Gap"

Traditional IAM and access control answer *identity and permission*, but not *contextual reasonableness*. A notable incident: an autonomous coding agent deleted a production database because the prompt underspecified boundaries. Agentic SRS must pivot from "what can the system do" to "what are the tolerance bands of acceptable execution."

> [!review]- Comprehension Review
> - **Comfort Level**: beginner / intermediate / advanced
> - **Feedback**:

---

## AI as Requirements Engineer

### Automated Elicitation

The **"Prompt Me" framework** deploys specialised elicitation agents that adopt a senior business analyst persona (IEEE 830 / agile expertise) and autonomously conduct structured stakeholder interviews — asking step-by-step questions to clarify functional boundaries, NFR expectations, and integration needs before any documentation is drafted.

Multi-agent systems can simultaneously mine user forums, app reviews, and telemetry data — autonomously extracting feature requests and capability gaps.

### Modular Document Generation — ReqInOne

Directing a single LLM to write a 50-page SRS causes hallucinations and context loss. **ReqInOne** decomposes generation into isolated sub-tasks:

| Module | Function |
|:---|:---|
| Summary Task Component | Populates high-level sections; requires annotations mapping generated text back to stakeholder input |
| Requirement Extraction | Parses intent from natural language; forces output into `"The <subject> shall <action verb> <object>"` template |
| Requirement Classification | Categorises into FRs and NFRs (Availability, Performance, Security, Usability, Maintainability, Portability) |

Advanced implementations (**SRS Writer**) use 17+ specialised agents: content specialists (Summary Writer, Interface Requirements Writer, etc.) and process specialists (automated quality checks on 7-dimensional scoring matrices, YAML↔Markdown bidirectional sync, HTML prototype generation from approved requirements).

### Automated Validation and Traceability

Agentic review workflows ingest an entire specification and perform deep contextual analysis — identifying missing dependencies, unquantified NFRs, and logical contradictions between sub-systems. Platforms like **aqua** run continuous background agents checking alignment between requirements, user stories, and downstream system prompts. Traceability is automated: business goals → FRs → test cases → execution constraints.

> [!review]- Comprehension Review
> - **Comfort Level**: beginner / intermediate / advanced
> - **Feedback**:

---

## Key Frameworks

### MANDATE (Multi-Agent Nominal Decomposition for Autonomous Task Execution)

Addresses verification when multiple execution paths are possible. Introduces **Anchor Corridors**:

- **Minimum**: Absolute baseline for task success
- **Target**: Optimal desired outcome
- **Constraints**: Hard boundaries, ethical limits, resource caps that must not be breached

Any execution path satisfying Minimum without violating Constraints is "Acceptable." Reaching Target is "Preferred." Provides auditability even when the agent takes an unprecedented path. Uses a **1+6 Role Architecture** — one LLM sequentially prompted as 6 distinct evaluation roles (specification, planning, execution, validation, etc.) without needing multiple model instances.

### AARM (Autonomous Action Runtime Management)

Operates as a protocol-level gateway, SDK instrumentation, or eBPF kernel monitor. Implements **"Prevention Paradigm"** — post-execution detection is catastrophically inadequate for agents capable of thousands of actions per minute.

Classifies all agent capabilities into four action profiles:

| Classification | Definition |
|:---|:---|
| Forbidden | Hard-blocked regardless of context. E.g., dropping a production database. |
| Context-Dependent Deny | Allowed by policy, blocked if session context conflicts with stated intent. |
| Context-Dependent Allow | Denied by default; permitted if session context confirms explicit mandate. |
| Context-Dependent Defer | High-risk; escalated to HITL. E.g., financial trades exceeding limits. |

### Model Context Protocol (MCP)

Open standard (Anthropic → Linux Foundation / Agentic AI Foundation). Standardises how agents connect to external data sources and execution environments via JSON-RPC 2.0. The SRS no longer needs to detail API connection logic or auth headers — it simply mandates MCP-compliant servers. Agent handles cognitive reasoning; MCP server enforces deterministic data constraints and local security policies. Model-agnostic; enables clean separation of concerns.

> [!review]- Comprehension Review
> - **Comfort Level**: beginner / intermediate / advanced
> - **Feedback**:

---

## Testing Non-Deterministic Agents

Traditional CI/CD assertion (`output == expected_string`) is obsolete. The **Agentic Testing Pyramid**:

| Level | Approach |
|:---|:---|
| 1 — Deterministic Unit Tests | LLM fully mocked. Tests orchestration logic: intent routing, memory persistence, tool error handling. |
| 2 — Probabilistic Validation | Semantic similarity checks, strict JSON schema validation, numerical threshold checking instead of exact string matching. |
| 3 — Multi-turn Scenario Simulation | Goal persistence over extended interactions, tool error recovery, loop avoidance, ambiguous input handling. |
| 4 — Human-in-the-loop Evaluation | SME manual review of edge cases, adversarial scenarios, high-stakes reasoning trajectories. |

### Production Observability Metrics

Agentic compliance monitoring (via Maxim AI, Galileo, Langfuse):

- **Tool Selection Quality**: Did the agent pick the correct API and pass appropriate parameters?
- **Action Advancement**: Does each sequential step make verifiable progress toward the stated goal?
- **Context Adherence**: No hallucination outside the retrieved context window; no contradiction of established facts.
- **Guardrail Trigger Rates**: How often does the agent attempt forbidden actions? High rates indicate system prompt misalignment with the operational environment.

Existing benchmarks (SWE-bench) optimise only for task completion accuracy — they ignore cost and reliability. Agents achieving marginal accuracy gains via thousands of iterative API calls are production-unviable.

> [!review]- Comprehension Review
> - **Comfort Level**: beginner / intermediate / advanced
> - **Feedback**:

---

## Governance and Standards

### ISO/IEC 42001 — AI Management Systems

World's first comprehensive AI management standard (2023). Mandatory operational layer atop the SRS for organisations deploying agentic systems:

- **Transparency and Traceability**: Proven lineage across training data, model weights, prompt templates, and executed agent behaviours. Every action traceable to a specific requirement and authorisation.
- **Impact Assessment**: Rigorous evaluation of societal, ethical, and operational impact prior to deployment (aligned with upcoming ISO/IEC 42005).
- **Continuous Oversight**: Lifecycle monitoring for bias, performance drift, security degradation, and unintended behaviour — not a point-in-time checklist.

### IEEE P3119 — AI Procurement Standard

Reshaping government and enterprise vendor selection. Procurement SRS documents must now demand verifiable proof of ethical alignment, algorithmic transparency, and fallback mechanisms. Inability to map non-deterministic output back to a compliant specification will limit operation in regulated sectors (especially under EU AI Act enforcement).

> [!review]- Comprehension Review
> - **Comfort Level**: beginner / intermediate / advanced
> - **Feedback**:

---

## Related Notes

- [[Agentic-AI-for-Sensitive-Data]]
- [[SRS and Agentic AI - Idea]]
- [[Agentic SRS Template]]

---

## Sources

- Gemini Research Report: *SRS and Agentic AI Integration* (2026-03-04)
- ISO/IEC/IEEE 29148:2018 — Systems and Software Requirements Engineering
- MANDATE: A Tolerance-Based Framework for Autonomous Agent Task Specification — ResearchGate
- AARM: Autonomous Action Runtime Management — arXiv 2602.09433
- ReqInOne: A Large Language Model-Based Agent for SRS Generation — arXiv
- Prompt Me: Intelligent Software Agent for Requirements Engineering — REFSQ 2025
- ISO/IEC 42001 — AI Management Systems
- MCP: Introducing the Model Context Protocol — Anthropic
- The AI Agent Testing Pyramid — Derek C. Ashmore, Medium (Feb 2026)
- AARM Action Classification — arXiv / ResearchGate

---
*Authored by: Clault KiperS 4.6*
