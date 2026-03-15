---
title: Adversarial Council — Factual Verification of Orchestration Research Notes
tags: [research, council, verification, orchestration, ai-agents]
date: 2026-03-14
type: council-report
---

# Adversarial Council: Factual Verification of Orchestration Research Notes

**Session Date:** 2026-03-14
**Scope:** 11 research notes on agentic AI orchestration
**Method:** Full adversarial council — two advocates, one critic, one questioner, one arbiter — played by single agent with web verification on each key claim
**Verdict at a Glance:** Notes are broadly trustworthy with important caveats. Three claims require correction, five require softening or caveat additions. Core architectural guidance is sound.

---

## Council Roles

| Role | Position |
|------|----------|
| Advocate A | These notes synthesise well-established patterns accurately; the architecture guidance is solid. |
| Advocate B | The competitive data is fresh; the tooling recommendations are practical and grounded. |
| Critic | Several numerical claims lack citations or are demonstrably wrong. Recommend-as-gospel stance on some tools ignores real limitations. |
| Questioner | Where exactly do the star counts and percentages come from? Who is the LatentRM claim sourcing? Is "gold standard" a factual descriptor or marketing language? |
| Arbiter | Weighs evidence, issues final per-claim verdicts. |

---

## Claim-by-Claim Findings

---

### Claim 1: "tmux is the gold standard orchestration substrate"

**Source note:** `Terminal-Multiplexing-Tmux.md`

**Advocate A:** tmux is 15+ years old, runs on every Unix system, has a complete programmatic CLI, and is explicitly used by Anthropic's own Claude Code agent teams. The NTM (Named Tmux Manager) project and Claude Code Agent Farm both validate it as production substrate.

**Advocate B:** Multiple orchestration frameworks (Claude Code, Agent Farm, TmuxAI) independently converge on tmux. That convergence is the signal.

**Critic:** "Gold standard" is marketing language, not a factual descriptor. Zellij now has a scriptable CLI, WebAssembly plugin system, and an active feature request for native Claude Code agent team support (GitHub issue #31901). WezTerm has a Lua scripting API. The note correctly lists these as alternatives but then dismisses them too quickly. tmux is also documented to have platform limitations: the note itself acknowledges it does not work well in VS Code's integrated terminal, Windows Terminal, or Ghostty.

**Questioner:** Is there a study comparing tmux vs Zellij for orchestration throughput? No. The "gold standard" framing is asserted, not demonstrated.

**Web evidence:** Confirmed that Claude Code agent teams use tmux panes as their process substrate. Zellij described as "nowhere near as mature as tmux" by independent reviewers as of early 2026, but actively approaching 1.0 production release. tmux scripting superiority for SSH and remote workflows confirmed.

**Arbiter verdict:** The claim is accurate in substance but overstated in language. tmux is the most proven choice today for local orchestration. The framing as "gold standard" is editorial, not empirical. The note should acknowledge that Zellij is a credible emerging alternative with growing scripting support and that the recommendation is time-bounded.

**Rating: EXAGGERATED**
**Action: SOFTEN CLAIM** — Change "gold standard" to "most proven and widely-adopted substrate as of 2026" and explicitly note Zellij is on a trajectory to compete within 12–18 months.

---

### Claim 2: "Godot 4 + godot-xterm as recommended pixel-office rendering stack" — production-ready?

**Source note:** `research/pixelated-desktop-ui-framework-evaluation.md`

**Advocate A:** godot-xterm is in the Godot Asset Library, actively maintained (issues filed October 2025), documented, and the note correctly states it uses libtsm and node-pty — the same libraries used by production terminal emulators. Pixelorama (8.5k+ stars) proves Godot 4 is production-worthy for productivity tools.

**Advocate B:** The note does not actually claim godot-xterm is "production-ready" — it uses language like "mature terminal support" and "strongest overall choice for MVP." That is defensible framing.

**Critic:** godot-xterm has a documented critical limitation that the note understates: the PTY node (the part that actually runs shells and connects to agents) is **Linux and macOS only**. The Windows build provides Terminal UI rendering only — no PTY. For a project explicitly aiming at cross-platform deployment including Windows, this is not a caveat, it is a fundamental gap. The note mentions "partial Windows support" but buries it in the framework details section rather than flagging it prominently in the verdict and recommendation sections.

**Questioner:** The recommendation section gives godot-xterm two checkmarks and calls it "mature terminal integration." Does that description hold when it cannot spawn a shell on one of the three major desktop platforms?

**Web evidence:** Confirmed: PTY node is Linux and macOS only. Terminal display node has partial Windows 64-bit support. The editor plugin terminal panel is Linux/macOS only. godot-xterm is listed in the Godot Asset Library and is actively maintained. No "production-ready" declaration found in official sources; the project is functional but lacks a stable v1.0 release tag.

**Arbiter verdict:** The Godot 4 recommendation for pixel-art rendering is well-supported. The godot-xterm endorsement as "mature" for terminal integration is overstated given the PTY Windows gap. The note should prominently caveat that Windows agent deployment is not possible with godot-xterm's PTY node without ConPTY/WinPTY integration work that does not yet exist in the project. For Linux-first deployment (which the vault context suggests), the recommendation holds. For cross-platform deployment, it is a blocker.

**Rating: EXAGGERATED** (specifically the "mature terminal integration" and "multi-platform" framing)
**Action: ADD CAVEAT** — In the verdict box and recommendation section, prominently flag that godot-xterm PTY support is Linux/macOS only. Windows requires either ConPTY integration (not yet implemented) or an IPC workaround to an external terminal process.

---

### Claim 3: "Judge-Router-Agent pattern achieving 91% cost savings"

**Source notes:** `Multi-Model-Coordination.md`, `Agentic-Orchestrator-MOC.md`

**Advocate A:** The example calculation in the note (800 cheap + 150 medium + 50 expensive queries = $1.74/day vs $15/day naive all-frontier) is internally consistent and the math checks out at roughly 88% savings. The 91% figure is in the right ballpark for routing-heavy workloads.

**Advocate B:** The RouteLLM paper (LMSYS, July 2024) demonstrated routers requiring only 14% of GPT-4 calls to achieve 95% GPT-4 performance — that is an 86% reduction. The 91% figure is plausible.

**Critic:** The notes present 91% as a precise figure with no citation. It is neither sourced to a paper nor to a specific internal benchmark. The MOC note says "91% cost savings on production workloads" as a stated fact. The RouteLLM paper cited in web search does not report 91%; it reports a different figure for a different workload. The worked example in the note itself comes out to approximately 88%, not 91%. These are self-inconsistent.

**Questioner:** What is the source of the 91% figure? Is it from a paper, from a personal calculation, or is it interpolated? The note does not say.

**Web evidence:** RouteLLM (2024) reports ~86% GPT-4 reduction. Research confirms 30–70% cost reductions are typical; some workloads achieve 90%+ but these are highly workload-dependent. A COLM 2025 paper cites 91% in a specific router evaluation context but for upgrade rates, not cost savings per se. No single study establishes 91% as the canonical figure for this pattern.

**Arbiter verdict:** The Judge-Router pattern is real and effective. The directional claim (large cost savings from routing) is well-supported by the literature. The specific "91%" figure is unverified and self-inconsistent with the note's own example calculation (which yields ~88%). This is a case of false precision on a real phenomenon.

**Rating: UNVERIFIED** (figure lacks citation; plausible range is 70–90%+ depending on workload)
**Action: SOFTEN CLAIM** — Replace "~91% cost savings" with "60–90%+ cost savings depending on workload composition" and cite RouteLLM (LMSYS, 2024) as the supporting study. Remove or correct the internal inconsistency with the worked example.

---

### Claim 4: "90.2% performance improvement from multi-agent vs single-agent Claude"

**Source notes:** `Claude-Code-Multi-Agent-System.md`, `claude-code-multi-agent-research.md`

**Advocate A:** This is cited to a specific Anthropic engineering post: "How we built our multi-agent research system" (June 2025). The URL is given in both notes. It is a primary source from Anthropic itself.

**Advocate B:** The figure is specific, sourced, and consistent across both notes that cite it. Multiple secondary sources (Simon Willison, ByteByteGo, Medium) corroborate the figure from the same Anthropic post.

**Critic:** The notes consistently attribute this to "Claude Opus 4 lead + Sonnet 4 workers" but the Anthropic post was written in June 2025 — at that time the models were likely earlier versions. The model names in the notes appear updated to 2026 naming conventions but the study predates those model releases. This creates a misleading impression that the 90.2% figure applies to the current Opus 4.6 + Sonnet 4.6 stack when it was measured on earlier models.

**Questioner:** The web search confirms the figure and source. Does the model versioning matter for the architectural conclusion?

**Web evidence:** Confirmed. The Anthropic engineering blog post "How we built our multi-agent research system" is at https://www.anthropic.com/engineering/multi-agent-research-system. The 90.2% figure is verified. The 80% variance explained by token usage is also confirmed in the same post. The Anthropic blog confirms "agents typically use about 4× more tokens than chat interactions, and multi-agent systems use about 15× more tokens than chats."

**Arbiter verdict:** The 90.2% figure and the 80% token variance figure are both verified against a primary Anthropic source. The concern about model versioning is legitimate but minor — the architectural insight (lead + worker parallelism yields large gains) generalises across model versions. The notes should cite the June 2025 post date and note the specific model versions tested in the original study.

**Rating: VERIFIED** (with minor caveat on model versioning)
**Action: ADD CAVEAT** — Note that the 90.2% figure was measured in June 2025 on Claude Opus 4 + Sonnet 4 (not necessarily the current 4.6 versions) and may differ for the current model stack.

---

### Claim 5: "Token usage explains 80% of performance variance in multi-agent systems"

**Source notes:** `Claude-Code-Multi-Agent-System.md`, `claude-code-multi-agent-research.md`

**Advocate A:** Same sourcing as Claim 4 — the Anthropic engineering post is explicit: "Token usage by itself explains 80% of the variance."

**Critic:** The statement is from Anthropic's own internal evaluation system, on their own workloads, using their own evaluation metric. It is a specific finding from one study, not a general law of multi-agent systems. The notes present it as a universal principle without qualification.

**Web evidence:** Confirmed verbatim in the Anthropic engineering post. The full quote is: "Token usage by itself explains 80% of the variance, with the number of tool calls and the model choice as the two other explanatory factors."

**Arbiter verdict:** The figure is accurately cited from a verified primary source. The critic's concern about overgeneralisation is noted but the notes do attribute it to "Anthropic's own multi-agent research system" in the longer report. The condensed note states it as a principle, which is slightly stronger than the source warrants.

**Rating: VERIFIED** (from Anthropic's own system; note it is specific to their evaluation, not a universal law)
**Action: ADD CAVEAT** — Add "(from Anthropic's internal research system evaluation; may vary for other workloads)" to contextualise the figure.

---

### Claim 6: "Pixel Agents (4.4k stars) vs MetaGPT (65.1k stars)"

**Source notes:** `orchestration-research-findings.md`, `Agentic-Orchestrator-MOC.md`

**Advocate A:** The competitive research note (`orchestration-research-findings.md`) has detailed entries for both projects, cites the GitHub URLs, and notes specific last-updated dates. This level of detail is not hallucinated.

**Critic:** The web search for MetaGPT returns 64.1k stars in recent search results, not 65.1k as stated. The Pixel Agents note contains internally contradictory star counts: the executive summary and competitive table say "4,400 stars" but the detailed entry for Pixel Agents says "Nearly 2,800 GitHub stars (feature-complete)." These cannot both be correct.

**Questioner:** Which Pixel Agents number is right — 4,400 or 2,800?

**Web evidence:** MetaGPT search returns 64.1k, not 65.1k. Pixel Agents GitHub search did not return a definitive count but two different figures appear within the same note. This internal inconsistency is a data quality issue.

**Arbiter verdict:** MetaGPT's 65.1k figure is approximately correct (within ~1.5% of the 64.1k found in search) and consistent with stars growing over time — the note was written on 2026-03-14 and 65.1k is plausible for that date. The Pixel Agents internal contradiction (4,400 vs 2,800 within the same note) is a genuine data quality problem. One figure is likely stale from an earlier draft.

**Rating: PARTIALLY UNVERIFIED** (MetaGPT plausible; Pixel Agents self-contradictory)
**Action: CORRECT** — Reconcile the Pixel Agents star count. The 4,400 figure appears in the competitive summary table and is used consistently in the MOC; 2,800 appears in the detailed project entry with a note "feature-complete" which suggests it may be from an earlier date. Use the single figure from the detailed entry as the baseline and update or remove the inconsistent one.

---

### Claim 7: "DeepSeek V3.2 at $0.03/1M tokens"

**Source note:** `Multi-Model-Coordination.md`

**Advocate A:** The note says "Very Low ($0.03/1M)" which is plausible for a headline/cached-hit price.

**Critic:** The actual pricing is significantly more complex. Web search confirms: cache hit input is $0.028/1M (which rounds to ~$0.03), but cache miss input is $0.28/1M and output is $0.42/1M. The note presents a single "$0.03/1M tokens" figure that is only true for the best-case cached-input scenario. Most real-world usage involves cache misses and output tokens, putting the effective cost an order of magnitude higher. This is materially misleading for anyone doing cost planning.

**Web evidence:** Confirmed. Official DeepSeek pricing as of March 2026: cache hit $0.028/1M (input), cache miss $0.28/1M (input), output $0.42/1M. The $0.03 figure represents only the cached-hit input scenario.

**Arbiter verdict:** The $0.03 figure is technically accurate for cache-hit input but is misleading as a standalone cost descriptor. A user relying on this for budget planning would be off by an order of magnitude on output costs. This needs correction.

**Rating: EXAGGERATED / MISLEADING**
**Action: CORRECT** — Update the model matrix to show: "Input $0.028 (cached) / $0.28 (miss), Output $0.42 per 1M tokens." Remove the single-figure "$0.03/1M" characterisation.

---

### Claim 8: "Ollama now supports Claude's Messages API natively (Jan 2026)"

**Source note:** `Multi-Model-Coordination.md`

**Advocate A:** This is confirmed by Ollama's own blog post and documentation. Ollama v0.14.0 introduced Anthropic Messages API compatibility. The date (January 2026) aligns with the release.

**Advocate B:** Multiple independent sources corroborate: the Ollama official blog, the documentation at `docs.ollama.com/api/anthropic-compatibility`, and the Medium article from February 2026 all confirm the feature. The Akshay Pachaar X post from the release period confirms it was received as significant news.

**Critic:** The web search notes "Edge cases in streaming and tool calling are still being patched" — the note presents it as a clean, complete integration when it is still maturing.

**Web evidence:** Confirmed. Ollama v0.14.0 (January 2026) added Anthropic Messages API compatibility. Streaming and tool-calling edge cases acknowledged as still being resolved.

**Arbiter verdict:** The core claim is verified. The caveat about ongoing edge case resolution should be added.

**Rating: VERIFIED** (with minor incompleteness)
**Action: ADD CAVEAT** — Note that streaming and tool-calling edge cases are still being patched as of early 2026 and to test before production use.

---

### Claim 9: "LatentRM outperforms majority voting consistently — based on which papers?"

**Source note:** `Multi-Model-Coordination.md`

**Advocate A:** The note references "2025-2026 research" and the LatentRM paper search found two relevant papers on arxiv: "Parallel Test-Time Scaling for Latent Reasoning Models" (arXiv 2510.07745) and "Latent Thinking Optimization" (arXiv 2509.26314).

**Critic:** The note says "a dedicated judge model scores responses, picks best (2025-2026 research shows this outperforms majority voting consistently)" and then cites "LatentRM paper (2025): evaluator > majority voting for parallel inference" in its references. "LatentRM" does not appear to be the title of a single canonical paper — it is a technique name used in at least two distinct papers. The claim that it "outperforms majority voting consistently" is stronger than what the papers demonstrate: arxiv 2510.07745 shows LatentRM-guided scoring improves on greedy decoding and majority voting in specific latent reasoning model scenarios, but "consistently" across all evaluator+voting comparisons is not established in the literature.

**Questioner:** Is there a paper specifically titled "LatentRM" that can be cited? No — it is a technique described in multiple papers. The reference in the note is vague.

**Web evidence:** Found two papers that use LatentRM as a technique (arXiv 2509.26314 and 2510.07745). Neither establishes universal superiority over majority voting. The broader LLM-as-judge literature (RewardBench, etc.) shows evaluator models outperform majority voting in many but not all settings.

**Arbiter verdict:** The general principle that evaluator/judge models outperform naive majority voting is supported by the literature. The specific "LatentRM" citation is vague and the "consistently" qualifier is overclaimed. The reference section should be replaced with specific paper citations and the claim softened.

**Rating: UNVERIFIED** (principle plausible, citation vague, "consistently" overclaimed)
**Action: SOFTEN CLAIM + CORRECT REFERENCE** — Replace "LatentRM paper (2025)" with specific arXiv IDs (2510.07745, 2509.26314) and change "outperforms majority voting consistently" to "shows improved performance over majority voting in tested scenarios."

---

### Claim 10: "Bevy is overkill but best if agents animate"

**Source note:** `research/pixelated-desktop-ui-framework-evaluation.md`

**Advocate A:** The note is nuanced — it calls Bevy "high-performance" with the "best 2D rendering performance" and says it takes "6–8 weeks to MVP" vs Godot's 3–4 weeks. "Overkill" is the MOC's summary of a careful tradeoff analysis. The note's actual text is fair.

**Critic:** The word "overkill" appears in the MOC summary (`Agentic-Orchestrator-MOC.md`) but not in the detailed evaluation note itself, which presents Bevy as a legitimate fourth option. The MOC is reductive. Additionally, the Bevy 0.16 "~3x performance improvement over 0.15" claim in the framework evaluation note is stated without citation or version check.

**Questioner:** Is Bevy actually overkill for an animated pixel-art office, or is a 2D sprite-heavy animated scene precisely the workload Bevy excels at?

**Web evidence:** Bevy explicitly lists "visualizations, user interfaces, or other graphical applications" as supported use cases alongside games. Its ECS architecture does scale to millions of animated entities. The MOC summary framing of "overkill" is a judgment call on development speed, not on technical fit.

**Arbiter verdict:** The detailed evaluation note is balanced and accurate. The MOC summary oversimplifies Bevy's applicability. For a pixel-art office with many animated agents, Bevy's sprite batching and ECS are not "overkill" in the technical sense — they are appropriate. The "overkill" framing is a development-velocity argument (Rust + ECS takes longer to learn) which is fair but should be stated as such.

**Rating: EXAGGERATED** (in the MOC summary; the detailed note is fair)
**Action: SOFTEN CLAIM** — In the MOC, change "Bevy is overkill" to "Bevy's Rust/ECS learning curve makes it a slower-to-prototype but technically appropriate choice for agent-animation-heavy workloads."

---

## Supplementary Findings (Noted During Review)

### The "Gemini 3 Pro" Naming

`Multi-Model-Coordination.md` references "Gemini 3 Pro" with 1M context. The correct current model name is **Gemini 3.1 Pro** (released February 19, 2026). "Gemini 3 Pro" was a prior release; the research note was written the same day as Gemini 3.1 Pro's release, so this is an understandable slip but creates version confusion.

**Action: CORRECT** — Update "Gemini 3 Pro" to "Gemini 3.1 Pro" in the model matrix.

---

### The "Claude Opus 4.6" Model Name

`Multi-Model-Coordination.md` lists "Claude Opus 4.6" in the model matrix. Web search confirms this is correct — Claude Opus 4.6 was released February 5, 2026. However, the note also lists "Claude 3.7 Sonnet" which is an outdated version name (Claude Sonnet 4.6 was released February 17, 2026). The model matrix mixes version generations.

**Action: CORRECT** — Update model matrix to use current naming: Claude Opus 4.6, Claude Sonnet 4.6, Claude Haiku 3.5 (or current Haiku version).

---

### "Lost in the Middle" Recall Percentages (76–82% vs 85–95%)

`Multi-Model-Coordination.md` states recall drops to "76–82% for info in the middle of long contexts vs 85–95% at edges." The original paper (Liu et al., TACL 2024) does document the U-shaped performance curve. The specific percentages cited in the note are not verbatim from the paper (which reports task-specific accuracy, not raw recall %), but the pattern is accurate. These figures appear to be illustrative approximations rather than direct quotes from the paper.

**Action: ADD CAVEAT** — Note these are indicative figures based on the "Lost in the Middle" pattern (Liu et al., 2024, TACL) and link the paper rather than presenting them as precise measurements.

---

### IPC Note Quality

`IPC-Inter-Agent-Communication.md` is the strongest note in the collection. It cites specific URLs for every claim, links to man pages and benchmark posts, and makes no overclaiming. The latency figures (UDS ~130µs, TCP ~334µs, shared memory ~100ns, ZeroMQ ~50–100µs, gRPC ~5–20ms) are consistent with the cited benchmarks.

**Rating: FULLY VERIFIED — KEEP AS-IS**

---

### Agent State Management Notes Quality

`Agent-State-Management.md` and `agent-state-management-report.md` are well-sourced and conceptually accurate. The Blackboard (Hayes-Roth 1985), Linda/Tuple Spaces (Gelernter 1985), CRDT, and Event Sourcing material is textbook-accurate. Redis data structure recommendations are consistent with official Redis documentation.

The claim that "long-running tasks fail 20–30% of the time" and "recovery savings: 60%+ of reprocessing avoided" are presented without citation. These are plausible engineering estimates but should be flagged as such.

**Action: ADD CAVEAT** — Mark the "20–30% failure rate" and "60% recovery savings" figures as engineering estimates rather than cited measurements.

---

### Agentic Orchestration Patterns Note

`Agentic-Orchestration-Patterns.md` is accurate and conservative. The framework comparison (LangGraph, CrewAI, AutoGen, Swarm, MetaGPT) is fair. The statement "LangGraph is the canonical implementation (2025 standard)" is defensible given its adoption trajectory. The "10× cheaper than pure ReAct for structured workflows" for Plan-and-Execute is presented without citation — it is plausible but unverified.

**Action: ADD CAVEAT** — Note the 10× cost figure is an estimate without specific citation.

---

## Summary Verdict Table

| # | Claim | Status | Action |
|---|-------|--------|--------|
| 1 | "tmux is gold standard orchestration substrate" | EXAGGERATED | SOFTEN CLAIM |
| 2 | "Godot 4 + godot-xterm as recommended rendering stack" | EXAGGERATED | ADD CAVEAT (PTY Windows gap) |
| 3 | "Judge-Router-Agent achieving 91% cost savings" | UNVERIFIED | SOFTEN CLAIM |
| 4 | "90.2% performance improvement multi-agent vs single" | VERIFIED | ADD CAVEAT (model version) |
| 5 | "Token usage explains 80% of performance variance" | VERIFIED | ADD CAVEAT (context-specific) |
| 6 | "Pixel Agents 4.4k stars vs MetaGPT 65.1k stars" | PARTIALLY UNVERIFIED | CORRECT (internal inconsistency) |
| 7 | "DeepSeek V3.2 at $0.03/1M tokens" | MISLEADING | CORRECT |
| 8 | "Ollama supports Claude Messages API natively (Jan 2026)" | VERIFIED | ADD CAVEAT (edge cases) |
| 9 | "LatentRM outperforms majority voting consistently" | UNVERIFIED | SOFTEN + CORRECT REFERENCE |
| 10 | "Bevy is overkill but best if agents animate" | EXAGGERATED | SOFTEN CLAIM |
| S1 | "Gemini 3 Pro" model name | OUTDATED | CORRECT to Gemini 3.1 Pro |
| S2 | "Claude 3.7 Sonnet" in model matrix | OUTDATED | CORRECT to Sonnet 4.6 |
| S3 | "Lost in the middle" recall percentages | PLAUSIBLE/UNVERIFIED | ADD CAVEAT |
| S4 | "20–30% task failure rate" for checkpointing | PLAUSIBLE/UNVERIFIED | ADD CAVEAT |
| S5 | "10× cheaper than pure ReAct" for plan-and-execute | PLAUSIBLE/UNVERIFIED | ADD CAVEAT |
| S6 | IPC latency figures throughout | VERIFIED | KEEP AS-IS |
| S7 | Blackboard, Linda, CRDT, Event Sourcing patterns | VERIFIED | KEEP AS-IS |

---

## Overall Verdict

**Are these notes trustworthy for designing a real orchestrator?**

**Yes, with targeted corrections.** The architectural guidance is sound and the core patterns (supervisor/worker, event sourcing, Redis hybrid, gRPC + NATS IPC, tmux substrate) reflect genuine best practice. The IPC note in particular is excellent — thoroughly cited, precise, and actionable.

**Three issues to fix before using for design decisions:**

1. **DeepSeek pricing** (Claim 7) is materially misleading for cost planning. Fix it before doing any budget modelling.
2. **godot-xterm Windows PTY gap** (Claim 2) is a deployment blocker for cross-platform targets. Correct the recommendation before committing to Godot as the rendering stack if Windows support matters.
3. **Pixel Agents internal star-count contradiction** (Claim 6) signals a data quality issue in the competitive research — verify star counts directly before presenting competitive analysis to stakeholders.

**Two analytical patterns that weaken the notes generally:**

- Unattributed precision: claiming specific percentages (91%, 10×, 20–30%) without citations. These should be marked as estimates or given citations.
- Outdated model names: the model matrix mixes version-generation nomenclature. Update before using for capability comparisons.

**The notes are safe to use as an architectural starting point and design reference.** They should not be cited to external parties without the corrections noted above.

---

## Priority Correction Queue

Ordered by impact on design decisions:

1. **CORRECT** DeepSeek V3.2 pricing in `Multi-Model-Coordination.md`
2. **CORRECT** Model matrix names (Gemini 3.1 Pro, Claude Sonnet 4.6) in `Multi-Model-Coordination.md`
3. **ADD CAVEAT** godot-xterm Windows PTY limitation in `research/pixelated-desktop-ui-framework-evaluation.md` verdict section
4. **CORRECT** Pixel Agents star count inconsistency in `orchestration-research-findings.md`
5. **SOFTEN** "91% cost savings" to cited range in `Multi-Model-Coordination.md` and `Agentic-Orchestrator-MOC.md`
6. **CORRECT REFERENCE** LatentRM citation to specific arXiv IDs in `Multi-Model-Coordination.md`
7. **ADD CAVEAT** "gold standard" tmux framing in `Terminal-Multiplexing-Tmux.md`
8. **ADD CAVEAT** 90.2% figure with model version and date in `Claude-Code-Multi-Agent-System.md`

---

*Adversarial council conducted by Clault KiperS 4.6, 2026-03-14*

---

*Authored by: Clault KiperS 4.6*
