# Feature: Dynamic Docs Index & Publish Workflow

## Overview

**User Story**: As a developer working on agenKic-orKistrator, I want visual-explainer docs to automatically appear in the GitHub Pages index when I create specs or finish implementations, so that the docs site stays current without manual HTML editing.

**Problem**: The current `index.html` on the docs branch is fully static — every card is hardcoded. Adding a new visual explainer requires manually editing HTML and pushing to the docs branch. This is error-prone, tedious, and disconnected from the development workflow.

**Out of Scope**:
- Server-side rendering or static site generators
- Automated generation of visual-explainer content (that is the `/visual-explainer` skill's job)
- Changing the editorial design language (Instrument Serif, DM Sans, warm palette preserved)
- Automated triggers for decision or roadmap docs (manual dispatch only)

---

## Success Condition

> This feature is complete when a developer can commit a visual-explainer HTML file to an epic or feature branch, and it automatically appears in the GitHub Pages index with correct hierarchy and tag pills — without editing `index.html` or `docs-manifest.json` by hand.

---

## Open Questions

| # | Question | Raised By | Resolved |
|:--|:---------|:----------|:---------|
| — | None — all questions resolved during design phase | — | [x] |

---

## Scope

### Must-Have
- **M1 — Manifest schema**: `docs-manifest.json` on docs branch with nested epics > features > docs structure: acceptance = manifest validates against the schema in the design spec (Section 1)
- **M2 — Dynamic index.html**: Fetches manifest on load, renders epic cards with nested feature sub-cards and spec/impl tag pills: acceptance = all 12 existing docs + 1 placeholder render correctly with no visual regression from current static page
- **M3 — Skeleton + crossfade loading**: Shimmer placeholders visible during manifest fetch, crossfade to real content: acceptance = no flash of empty content on page load
- **M4 — Scroll-triggered animations**: IntersectionObserver fade-up on sections and cards, staggered entrance, hover micro-interactions: acceptance = animations fire on scroll, `prefers-reduced-motion` collapses all durations
- **M5 — `publish-doc.yml` reusable workflow**: `workflow_call` + `workflow_dispatch`, accepts html-file/target-path/manifest-entry/source-ref, upserts manifest and pushes to docs branch: acceptance = manual dispatch successfully publishes a doc and updates manifest
- **M6 — `docs-on-pr.yml` automated trigger**: Fires on PR opened/ready_for_review (spec) and labeled `ready-to-merge` (impl) for epic/feature branches, discovers HTML files, calls publish-doc.yml: acceptance = opening a PR with a spec HTML auto-publishes it
- **M7 — Meta tag convention**: New visual-explainer HTML files include `<meta name="doc-title">`, `<meta name="doc-desc">`, `<meta name="doc-epic">`, optional `<meta name="doc-feature">`: acceptance = workflow extracts metadata from meta tags successfully
- **M8 — Migration**: One-time commit seeds manifest with 12 existing files + 1 placeholder, replaces static index.html with dynamic version: acceptance = GitHub Pages renders identically to current site after migration

### Should-Have
- **S1 — Manifest fetch error fallback**: Graceful message if `docs-manifest.json` fails to load
- **S2 — Epic stub auto-creation**: If epicNumber not in manifest, workflow creates a stub entry from meta tags or placeholders

### Nice-to-Have
- **N1 — Tag pill hover glow**: Subtle scale + opacity on tag pill hover

---

## Technical Plan

**Affected Components**:
- `index.html` on docs branch (full rewrite to dynamic)
- `docs-manifest.json` on docs branch (new file)
- `.github/workflows/publish-doc.yml` on main (new workflow)
- `.github/workflows/docs-on-pr.yml` on main (new workflow)

**Data Model Changes**:
- New `docs-manifest.json` — schema defined in design spec Section 1
- No database, no backend — pure static files + GitHub Actions

**API Contracts**: N/A (no server-side APIs)

**Dependencies**:
- GitHub Pages legacy build from docs branch root (already configured)
- GitHub Actions `actions/checkout@v6`, `actions/github-script@v8`
- `ready-to-merge` label must exist in the repo (create if missing)

**Risks**:
| Risk | Likelihood | Mitigation |
|:-----|:-----------|:-----------|
| Race condition: two PRs publish simultaneously, one overwrites the other's manifest changes | Low | publish-doc.yml uses sequential commits; GitHub rejects push if docs branch moved — workflow retries with pull + rebase |
| Manifest JSON corruption from malformed `manifest-entry` input | Medium | Validate JSON parse in the upsert step; fail loudly with error message rather than writing corrupt file |
| Legacy file discovery: automated pattern misses legacy filenames | N/A (by design) | Legacy files are seeded in manifest during migration and never re-published via automation |
| Sparse checkout misses files outside `docs/visual-explainer/` | Low | Document that `html-file` must be under `docs/visual-explainer/`; expand sparse-checkout if needed later |

---

## Acceptance Scenarios

```gherkin
Feature: Dynamic Docs Index & Publish Workflow
  As a developer on agenKic-orKistrator
  I want docs to auto-publish from PR branches to the GitHub Pages index
  So that the docs site stays current without manual HTML editing

  Background:
    Given the docs branch has a valid docs-manifest.json
    And the index.html fetches and renders from docs-manifest.json

  Rule: Index renders all manifest entries with correct hierarchy

    Scenario: Epic with nested features renders correctly
      Given the manifest contains epic 1 with 2 docs and 1 feature with 1 doc
      When the index page loads
      Then epic 1 card shows with number, title, description
      And 2 tag pills (SPEC, IMPL) link to the correct HTML files
      And 1 feature sub-card is nested inside the epic card with its own tag pill

    Scenario: Empty epic renders as placeholder
      Given the manifest contains epic 4 with empty docs array and no features
      When the index page loads
      Then epic 4 card shows with "coming soon" in dimmed text

    Scenario: Manifest fetch fails gracefully
      Given docs-manifest.json returns a network error
      When the index page loads
      Then a fallback message is displayed instead of cards

  Rule: Animations respect accessibility preferences

    Scenario: Scroll-triggered animations fire on viewport entry
      Given prefers-reduced-motion is not set
      When a section scrolls into the viewport
      Then cards fade up with staggered delay

    Scenario: Reduced motion disables all animations
      Given prefers-reduced-motion is set to reduce
      When the page loads and user scrolls
      Then no animations or transitions are visible

  Rule: publish-doc.yml publishes a doc and upserts the manifest

    Scenario: Manual dispatch publishes a new spec doc
      Given a visual-explainer HTML file exists in the source branch
      When publish-doc.yml is dispatched with valid inputs
      Then the HTML file appears on the docs branch at target-path
      And docs-manifest.json contains a new entry with the correct href and type

    Scenario: Upsert updates existing entry without duplication
      Given a doc with the same href already exists in the manifest
      When publish-doc.yml runs with updated metadata
      Then the existing entry is updated in place
      And no duplicate entry is created

  Rule: docs-on-pr.yml auto-publishes on PR events

    Scenario: PR opened with spec HTML triggers auto-publish
      Given a PR is opened from an epic/* branch
      And the branch contains docs/visual-explainer/epic-1-go-orchestrator-core-spec.html
      When the pull_request opened event fires
      Then publish-doc.yml is called with the spec file and correct manifest-entry

    Scenario: ready-to-merge label triggers impl publish
      Given a PR exists from a feature/* branch
      And the branch contains docs/visual-explainer/feature-15-cas-agent-state-impl.html
      When the ready-to-merge label is added
      Then publish-doc.yml is called with the impl file and correct manifest-entry

    Scenario: PR with no visual-explainer files is a no-op
      Given a PR is opened from a feature/* branch
      And the branch contains no files matching the discovery pattern
      When the pull_request opened event fires
      Then no workflow_call to publish-doc.yml is made

  Rule: Migration preserves existing content

    Scenario: All legacy docs appear in migrated index
      Given the migration commit has been applied to the docs branch
      When the index page loads
      Then all 12 existing HTML files are reachable via cards
      And epic 4 shows as a placeholder
      And the visual design matches the previous static index
```

---

## Task Breakdown

| ID | Task | Priority | Dependencies | Status |
|:---|:-----|:---------|:-------------|:-------|
| T1 | Create `docs-manifest.json` seeded with 12 existing files + epic 4 placeholder | High | None | pending |
| T2 | Build dynamic `index.html` — manifest fetch, card rendering, nested hierarchy, tag pills | High | T1 | pending |
| T3 | Add skeleton shimmer + crossfade loading animation | High | T2 | pending |
| T4 | Add scroll-triggered fade-up animations (IntersectionObserver) + hover micro-interactions | High | T2 | pending |
| T5 | Add `prefers-reduced-motion` support | High | T3, T4 | pending |
| T6 | Add manifest fetch error fallback UI | Med | T2 | pending |
| T7 | Migration commit: deploy T1-T6 to docs branch, verify GitHub Pages renders correctly | High | T1-T6 | pending |
| T8 | Create `.github/workflows/publish-doc.yml` — reusable workflow with dual checkout, manifest upsert, push | High | T1 | pending |
| T9 | Create `.github/workflows/docs-on-pr.yml` — automated trigger with file discovery, meta extraction, workflow_call | High | T8 | pending |
| T10 | Create `ready-to-merge` label in the repo if missing | Low | None | pending |
| T11 | End-to-end test: manual dispatch of publish-doc.yml with a test HTML file | High | T7, T8 | pending |
| T12 | End-to-end test: open a PR with a spec HTML file and verify auto-publish | High | T9, T11 | pending |

---

## Exit Criteria

- [ ] All Must-Have scenarios pass (manual verification on GitHub Pages)
- [ ] All 12 existing docs + epic 4 placeholder render in the migrated index
- [ ] No visual regression from the current static index (side-by-side comparison)
- [ ] `prefers-reduced-motion` disables all animations
- [ ] Manual dispatch of `publish-doc.yml` successfully publishes and upserts
- [ ] `docs-on-pr.yml` fires on PR opened and on `ready-to-merge` label
- [ ] No broken links in the index
- [ ] Light and dark mode both render correctly

---

## References

- Design spec: `docs/superpowers/specs/2026-03-24-dynamic-docs-index-publish-workflow-design.md`
- Current static index.html: `git show origin/docs:index.html`
- GitHub Pages config: docs branch root, legacy build
- Existing workflows: `.github/workflows/issue-branch-handler.yml` (reference for conventions)

---
*Authored by: Clault KiperS 4.6*
