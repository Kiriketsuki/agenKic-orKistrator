---
## Adversarial Council -- Feature 33 T2/T3 Council 4: Final Merge Readiness

> Convened: 2026-03-23 20:00 | Advocates: 1 | Critics: 2 | Questioner: 1 | Rounds: 2/4 | Motion type: CODE

### Motion

"Feature 33 (T2/T3) has resolved all conditions from three prior councils and is ready to squash-merge into epic/3 via PR #49."

---

### Advocate Positions

**ADVOCATE-1** (final position: CONDITIONAL FOR): All five Council 1 code conditions, both Council 2 conditions, all Council 3 conditions, and both advisory items are resolved in the current codebase. Tests pass (43 PASS, 0 FAIL). The code is architecturally sound: dependency inversion via `AdapterResolver` (`gateway.go:248-251`), immutable `FormatRequest` implementations (`anthropic.go:25`, `openai.go:29`, `ollama.go:24`), credential redaction via `ProviderConfig.Format()` (`gateway.go:196-209`), response size limiting via `io.LimitReader` (`litellm.go:14,149`), and URL scheme validation (`litellm.go:70-78`). Conceded one new finding: exported type name `Option` (spec line 128) vs `LiteLLMOption` (`litellm.go:66`) is a real spec-code divergence that should be fixed before merge. Accepted this as a condition. Partially conceded the Gherkin scenario at spec lines 303-307 is imprecise about which layer performs temperature omission, but argues advisory (system behavior is correct). Wire type naming divergence (Objection 2) conceded as advisory. Mutation safety test (Objection 4) rebutted — no current bug, not in exit criteria.

---

### Critic Positions

**CRITIC-1** (final position: CONDITIONAL): Confirmed all four Council 3 conditions are independently resolved. Identified four new findings: (1) exported type `Option` vs `LiteLLMOption` — a compilation-breaking spec-code divergence on an exported type that C2.1 should have caught, verified by ARBITER at `litellm-client-provider-adapters-spec.md:125,128` vs `litellm.go:66,105`; (2) unexported wire type naming divergence (`chatCompletionRequest` vs `liteLLMRequest` etc.) — conceded as LOW/advisory; (3) Gherkin scenario at spec lines 303-307 attributes temperature omission to `FormatRequest` when the actual mechanism is `buildRequest` serialization at `litellm.go:202-205` — accepted ARBITER's LOW ruling, proposed as advisory; (4) no mutation safety test for `Metadata` map — accepted ARBITER's DROP ruling (previously triaged in Council 3). Final condition: sync spec to use `LiteLLMOption`. Advisories: wire type names, Gherkin scenario clarity.

**CRITIC-2** (final position: CONDITIONAL aligning with CRITIC-1): Conceded in Round 1 that the WithBaseURL test tautology (Council 3 condition) is resolved — the `recordingTransport` at `litellm_test.go:16-23` correctly isolates the guard from downstream `http.Transport` rejection. Raised four objections in Round 1: (1) scheme-only SSRF protection, (2) silent rejection anti-pattern, (3) `WithHTTPClient` bypasses defenses, (4) test asymmetry between valid and invalid URL paths. Withdrew Objections 1-3 in Round 2 after ADVOCATE-1 cited verbatim prior council records showing all three were previously conceded and triaged (Objections 1-2 as advisory deferred to T5, Objection 3 as by-design). Maintained Objection 4 as LOW/advisory. Aligned with CRITIC-1's single condition (`Option` → `LiteLLMOption`). No security-specific merge blockers.

---

### Questioner Findings

| Probe | Target | Finding | Status |
|:------|:-------|:--------|:-------|
| 1 | ADVOCATE-1 (prior council conditions) | Asked for exact council condition texts, not just commit hashes. ARBITER provided partial resolution by confirming all conditions against source files in pre-debate verification. | SATISFIED by ARBITER verification |
| 2 | ADVOCATE-1 (test output) | Asked for verbatim test output. ARBITER ran tests at debate open: `go test ./internal/gateway/...` — all pass (0.008s + 0.003s). | SATISFIED by ARBITER verification |
| 3 | ADVOCATE-1 (WithBaseURL assertion) | Asked what specific assertion proves guard rejection vs transport rejection. ARBITER confirmed: `litellm_test.go:289` asserts `strings.HasPrefix(rt.lastURL, "http://localhost:8000")` — if guard had accepted bad URL, `rt.lastURL` would start with the bad URL instead. | SATISFIED |

---

### Key Conflicts

1. **`Option` vs `LiteLLMOption` severity** — ADVOCATE-1 argued advisory (no downstream consumers, 1-line fix); CRITIC-1 and CRITIC-2 argued blocking condition (spec is a contract for future consumers, trivial fix has no reason to defer). **Resolved by ARBITER**: classified as CONDITION — C2.1 required spec sync, and an exported type name mismatch means C2.1 is incomplete. The fix is trivial, which is precisely why it should be done before merge, not deferred.

2. **CRITIC-2 re-litigation** — CRITIC-2 re-raised SSRF (Objection 1) and silent rejection (Objection 2), both previously conceded in Councils 1 and 3. ADVOCATE-1 cited verbatim council records. CRITIC-2 withdrew both after verification. **Resolved**: concession discipline enforced.

3. **Gherkin scenario accuracy** — CRITIC-1 argued spec lines 303-307 are misleading; ADVOCATE-1 argued Gherkin describes system behavior. **Resolved**: classified as ADVISORY — the scenario is imprecise about mechanism but the system behavior is correct. No code bug.

---

### Concessions

**ADVOCATE-1 conceded:**
- Exported type name `Option` vs `LiteLLMOption` is a real spec-code divergence (accepted as condition)
- Wire type naming (`chatCompletionRequest` vs `liteLLMRequest`) is a real divergence (advisory)
- Gherkin scenario at lines 303-307 is imprecise about which layer performs omission (advisory)
- Original claim of "precise" spec-code alignment was overstated on naming
- Test asymmetry between valid/invalid URL paths is a real coverage gap (LOW)

**CRITIC-1 conceded:**
- All four Council 3 conditions are resolved — confirmed independently
- Tests pass, architecture is clean, credential redaction is sound
- Wire type naming (Objection 2): advisory, unexported types
- Mutation safety test (Objection 4): dropped, previously triaged
- Gherkin scenario (Objection 3): advisory, accepted ARBITER's LOW ruling
- `WithHTTPClient` is by-design, not a defect

**CRITIC-2 conceded:**
- WithBaseURL test tautology is resolved (Round 1)
- SSRF (Objection 1): withdrawn — concession discipline, previously triaged as advisory deferred to T5
- Silent rejection (Objection 2): withdrawn — concession discipline, previously triaged as advisory deferred to T5
- `WithHTTPClient` bypass (Objection 3): withdrawn — by-design construction-time option, not Critical Discovery
- Test asymmetry (Objection 4): maintained as LOW, not merge-blocking

---

### Regression Lineage

- **`Option` vs `LiteLLMOption` divergence**: This is NOT a regression from a prior council fix. The spec was committed in `65e8e0e` and the API contract section was updated in `8f23f7a` (C2.1 fix), but the `Option` type name at lines 125/128 was not caught during the C2.1 update. This is a gap in the C2.1 fix scope — the same category as Council 3's spec text divergences.

- **Gherkin scenario (lines 303-307)**: NOT a regression. This scenario was in the original spec commit (`65e8e0e`) and was never flagged by prior councils. It is a pre-existing spec accuracy issue, not caused by any fix.

---

### Scope Audit

| Finding | Relevance | Pre-existence | Critical Discovery? | Ruling |
|:--------|:----------|:--------------|:--------------------|:-------|
| CRITIC-1 Obj 1: `Option` vs `LiteLLMOption` | YES — spec sync (C2.1) | YES — specific to this PR | N/A | **IN SCOPE — CONDITION** |
| CRITIC-1 Obj 2: Wire type naming | YES | YES | N/A | **IN SCOPE — ADVISORY** |
| CRITIC-1 Obj 3: Gherkin scenario | YES | YES | N/A | **IN SCOPE — ADVISORY (LOW)** |
| CRITIC-1 Obj 4: Mutation safety test | YES | NO — previously triaged | N/A | **DROPPED** |
| CRITIC-2 Obj 1: SSRF scheme-only | Fails pre-existence | Already triaged C1+C3 | SSRF is OWASP Top 10 but NOT a new discovery | **DROPPED** |
| CRITIC-2 Obj 2: Silent rejection | Fails pre-existence | Already triaged C3 | No | **DROPPED** |
| CRITIC-2 Obj 3: WithHTTPClient bypass | Fails relevance | By-design feature | No | **DROPPED** |
| CRITIC-2 Obj 4: Test asymmetry | YES | YES | No | **ADVISORY** |

---

### Arbiter Recommendation

**CONDITIONAL**

All conditions from three prior councils are resolved in the current codebase. The Go implementation is correct, well-tested (43 PASS, 0 FAIL), and architecturally sound. No code changes are needed.

One new finding emerged during this council: the spec uses the exported type name `Option` while the code exports `LiteLLMOption`. This is a C2.1 spec sync gap — the same class of issue that Councils 2 and 3 addressed. I classify this as a **condition** (not advisory) for two reasons:

1. **The motion's first clause fails without it.** The motion claims "has resolved all conditions from three prior councils." C2.1 required spec sync. An exported type name mismatch on the public API means C2.1 is demonstrably incomplete.

2. **The fix is trivial.** CRITIC-1's argument is persuasive: if a fix takes 2 minutes, there is no reason to defer it. Advisory classification is appropriate for items requiring significant effort or judgment. A find-and-replace on `Option` → `LiteLLMOption` in the spec requires neither.

The Gherkin scenario (CRITIC-1 Objection 3) is imprecise but not incorrect at the system level. I classify it as advisory — it should be clarified but does not block merge.

This council converged in 2 rounds with all three participants reaching CONDITIONAL. The trajectory across four councils — from 5 code conditions (Council 1) to 2 doc/test conditions (Council 2) to 4 spec text fixes (Council 3) to 1 type name fix (Council 4) — confirms the implementation has been stable and the remaining work is exclusively spec documentation.

---

### Conditions (blocking)

1. **Sync exported type name in spec** — Update `litellm-client-provider-adapters-spec.md` lines 125, 128, 130, 131, 132 to use `LiteLLMOption` instead of `Option`, matching the code at `litellm.go:66,105`.

   CITE: `litellm-client-provider-adapters-spec.md` L:125, L:128, L:130-132
   CITE: `litellm.go` L:66, L:105

---

### Suggested Fixes

#### Bug Fixes
None. All Go implementation code is correct.

#### In-PR Improvements

1. **Exported type name sync** (Condition 1): Find-and-replace `Option` → `LiteLLMOption` in the spec's LiteLLM client API Contract section (5 occurrences at lines 125, 128, 130, 131, 132).
   CITE: `litellm-client-provider-adapters-spec.md` L:125-132

#### PR Description Amendments
- Note that Council 4 identified one residual spec-code divergence (exported type name) missed by C2.1 and resolved in this commit.

#### Critical Discoveries
None. No OWASP Top 10, data loss, or compliance issues were identified. The SSRF scheme validation is defense-in-depth with full host allowlist deferred to T5 (documented at `gateway.go:182-184`). This was triaged in Council 1 and reconfirmed in Councils 3 and 4.

---

### Advisory Items (non-blocking, tracked for future work)

| # | Item | Source | Recommended Timing |
|:--|:-----|:-------|:-------------------|
| 1 | Sync unexported wire type names in spec (`chatCompletionRequest` → `liteLLMRequest`, etc.) | CRITIC-1 Obj 2 | Same commit as Condition 1 (if convenient) |
| 2 | Clarify Gherkin scenario at spec lines 303-307 to note temperature omission occurs at serialization layer, not adapter layer | CRITIC-1 Obj 3 | Same commit or next PR |
| 3 | Add symmetric `recordingTransport` assertion for valid URL path in WithBaseURL test | CRITIC-2 Obj 4 | Next PR touching this file |
| 4 | Add `slog.Warn` for rejected schemes in `WithBaseURL` (`litellm.go:74`) | Council 3 Advisory #1 | T5 (config loading) |
| 5 | Test HTTPS acceptance via `httptest.NewTLSServer` | Council 3 Advisory #2 | Next PR touching this file |
| 6 | Consider `maps.Clone` for Metadata in `Registry.Resolve` | Council 3 Advisory #3 | When a future adapter mutates Metadata |

---

### Prior Council Comparison

| Council | Date | Outcome | Conditions | Status |
|:--------|:-----|:--------|:-----------|:-------|
| Council 1 | 2026-03-23 14:30 | CONDITIONAL | 5 code conditions + 2 advisory | All 7 resolved |
| Council 2 | 2026-03-23 18:00 | CONDITIONAL | C2.1 (spec sync) + C2.2 (test fix) | Resolved, with 1 gap in C2.1 scope |
| Council 3 | 2026-03-23 19:00 | CONDITIONAL | 3 spec text + 1 test tautology | All 4 resolved |
| Council 4 (this) | 2026-03-23 20:00 | CONDITIONAL | 1 spec type name | Pending |

**Trajectory**: Each council has narrowed the gap. Council 1 found 5 code defects; Council 2 reduced to 2 documentation/test issues; Council 3 found 4 spec text divergences; this council finds 1 type name divergence. The implementation code has been stable since Council 1 — all remaining work is a single spec text fix. All four council participants converged to CONDITIONAL in 2 rounds.

---

### Verification Checklist (populated by verifier)

- [ ] Condition 1: Spec updated to use `LiteLLMOption` at lines 125, 128, 130-132
- [ ] All tests pass: `go test ./internal/gateway/...`
- [ ] `go vet ./internal/gateway/...` passes clean
---
