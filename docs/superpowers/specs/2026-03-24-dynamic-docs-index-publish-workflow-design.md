# Dynamic Docs Index & Publish Workflow

**Date**: 2026-03-24
**Status**: Approved
**Scope**: Manifest-driven index, GitHub Actions publish workflow, animation pass

---

## Overview

Replace the static hardcoded `index.html` on the docs branch with a dynamic, manifest-driven page. Add a reusable GitHub Actions workflow that publishes visual-explainer HTML files to the docs branch and upserts entries in the manifest. Apply scroll-triggered animations and micro-interactions for visual polish.

## Goals

1. New visual-explainer pages appear in the index automatically — no manual HTML editing
2. Epics and features display in a nested hierarchy with spec/impl tag pills
3. Publishing is automated on PR events and available as manual dispatch
4. Animations enhance the experience without compromising accessibility

## Non-Goals

- Server-side rendering or static site generators
- Automated generation of visual-explainer content (that's the `/visual-explainer` skill's job)
- Changing the editorial design language (Instrument Serif, DM Sans, warm palette preserved)

---

## 1. Manifest Schema

`docs-manifest.json` on the docs branch:

```json
{
  "epics": [
    {
      "number": 1,
      "title": "Go Orchestrator Core",
      "desc": "Supervisor, agent state machine, DAG engine, gRPC service, Redis state",
      "color": "navy",
      "docs": [
        { "title": "Architecture Spec", "type": "spec", "href": "visual-explainer/epic-1-go-orchestrator-core.html" }
      ],
      "features": [
        {
          "number": 15,
          "title": "CompareAndSetAgentState",
          "desc": "Atomic state transitions with optimistic concurrency",
          "docs": [
            { "title": "Feature Spec", "type": "spec", "href": "visual-explainer/feature-15-cas-agent-state-spec.html" }
          ]
        }
      ]
    }
  ],
  "decisions": [
    { "tag": "#1", "title": "Orchestration Pattern", "desc": "Supervisor/Worker + DAG over LangGraph, CrewAI, or AutoGen", "href": "decisions/orchestration-pattern.html" }
  ],
  "roadmap": [
    { "title": "Project Roadmap", "desc": "Interactive timeline — four epics to pixel office", "href": "visual-explainer/roadmap.html" }
  ]
}
```

### Field definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `epics[].number` | number | yes | Epic issue number |
| `epics[].title` | string | yes | Display title |
| `epics[].desc` | string | yes | One-line description |
| `epics[].color` | string | yes | CSS color token mapped to `--{color}` CSS variable: `navy` → `--navy`, `slate` → `--slate`, `teal` → `--teal`, `amber` → `--amber` |
| `epics[].docs[]` | array | yes | Doc entries (can be empty) |
| `epics[].features[]` | array | yes | Nested features (can be empty) |
| `docs[].title` | string | yes | Link text |
| `docs[].type` | `"spec"` \| `"impl"` | yes | Determines tag pill color |
| `docs[].href` | string | yes | Relative path from docs root |
| `features[].number` | number | yes | Feature issue number |
| `decisions[].tag` | string | yes | Display tag (e.g. `#1`) |
| `roadmap[].title` | string | yes | Display title |

---

## 2. Index.html — Dynamic Rendering

### Fetch and render

1. `fetch('docs-manifest.json')` on `DOMContentLoaded`
2. Parse JSON, render each section into the DOM
3. Skeleton shimmer placeholders visible during load, crossfade to real content

### Card hierarchy

```
§ Visual Explainers
  ┌─ Epic card ───────────────────────┐
  │ [1] Go Orchestrator Core          │
  │     desc text                     │
  │     SPEC  IMPL  (tag pill links)  │
  │  ┌─ Feature sub-card ──────────┐  │
  │  │ CompareAndSetAgentState     │  │
  │  │ desc text                   │  │
  │  │ SPEC  (tag pill link)       │  │
  │  └─────────────────────────────┘  │
  └───────────────────────────────────┘
```

- Epic cards: large number (Instrument Serif), title, desc, inline doc pills
- Feature sub-cards: nested inside epic, left indent + left border in epic's accent color, smaller type
- Tag pills: `SPEC` in teal (`--teal-dim` bg, `--teal` text), `IMPL` in amber (`--amber-dim` bg, `--amber` text). Monospace, uppercase, 9px. Each pill is an `<a>` to the doc.
- Empty docs array: card renders with "coming soon" in dimmed text
- Manifest fetch failure: graceful fallback message

### CSS variables

The new `index.html` inherits the full variable set from the existing static `index.html` on the docs branch. The complete set includes:

**Layout/typography**: `--font-display`, `--font-body`, `--font-mono`, `--ease-out-quart`
**Surfaces**: `--bg`, `--surface`, `--border`, `--border-strong`, `--text`, `--text-dim`
**Accent colors** (light + dark mode): `--navy`, `--navy-dim`, `--teal`, `--teal-dim`, `--amber`, `--amber-dim`, `--slate`, `--slate-dim`, `--rose`, `--rose-dim`

The existing `index.html` CSS `:root` block is the authoritative source — copy it verbatim into the new dynamic version.

### Sections rendered

1. **Visual Explainers** — epics with nested features
2. **Architecture Decisions** — flat list (as today)
3. **Roadmap** — accent-striped card (as today)
4. **Footer** — GitHub link, version

---

## 3. Animation Design

### Load sequence

- 3 skeleton card placeholders with CSS gradient shimmer animation
- On manifest load: skeleton fades out, real content fades up (200ms crossfade)

### Scroll entrance

- IntersectionObserver (threshold 0.1) on each section label and card group
- `fadeUp` keyframe: `opacity 0 → 1`, `translateY(12px) → 0`
- Staggered by `--i` CSS custom property (0.1s per item)

### Hover micro-interactions

- **Card**: `translateY(-3px)`, border → `--border-strong`, `box-shadow: 0 4px 20px rgba(0,0,0,0.07)`, arrow slides right 4px. All 200ms `ease-out-quart`.
- **Tag pill**: `scale(1.05)`, background opacity increase. 150ms.
- **Section dot**: `dotPulse` keyframe on scroll-in (existing pattern).

### Accessibility

- `prefers-reduced-motion: reduce` → all `animation-duration` and `transition-duration` collapse to `0.01ms`
- No content hidden behind animations — everything is visible immediately in reduced-motion mode

---

## 4. GitHub Actions Workflows

### `publish-doc.yml` — Reusable workflow

**Location**: `.github/workflows/publish-doc.yml` on `main`

**Trigger**: `workflow_call` + `workflow_dispatch`

**Permissions**: `contents: write` (required for pushing to docs branch)

**Inputs**:

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| `html-file` | string | yes | Path to HTML file in the source branch (e.g. `docs/visual-explainer/epic-1-go-orchestrator-core-spec.html`) |
| `target-path` | string | yes | Destination path on docs branch (e.g. `visual-explainer/epic-1-go-orchestrator-core-spec.html`) |
| `manifest-entry` | string | yes | JSON string with metadata for the manifest upsert |
| `source-ref` | string | no | Git ref to pull the HTML file from. Defaults to the calling workflow's `github.sha`. For `workflow_dispatch`, defaults to `github.ref`. |

**`manifest-entry` JSON format** (for epic-level docs):
```json
{
  "section": "epics",
  "epicNumber": 1,
  "doc": { "title": "Architecture Spec", "type": "spec" }
}
```

For feature-level docs (add `featureNumber`):
```json
{
  "section": "epics",
  "epicNumber": 1,
  "featureNumber": 15,
  "doc": { "title": "Feature Spec", "type": "spec" }
}
```

`featureNumber` is optional — omit it entirely for epic-level docs. When present, the doc is nested under the feature within the epic.

**Epic stub creation**: If `epicNumber` does not exist in the manifest, the upsert creates a stub entry with `title`, `desc`, and `color` pulled from the `<meta>` tags in the HTML file. If meta tags are missing, it uses placeholder values (`title`: "Epic {n}", `desc`: "", `color`: "navy") that should be manually corrected in a follow-up dispatch.

**Steps**:

1. Checkout docs branch into `./docs-branch/` using `actions/checkout` with `path: docs-branch`
2. Checkout source ref into `./source/` using `actions/checkout` with `ref: ${{ inputs.source-ref }}`, `path: source`, `sparse-checkout: docs/visual-explainer`
3. Copy `source/${{ inputs.html-file }}` to `docs-branch/${{ inputs.target-path }}`
4. Run manifest upsert via `actions/github-script`:
   - Read and parse `docs-branch/docs-manifest.json`
   - Find or create epic entry by `epicNumber` (create stub if missing — see above)
   - If `featureNumber` is present: find or create feature under that epic
   - Upsert into the `docs` array by `href` match (update if exists, append if new)
   - `href` is derived from `target-path` (used as-is, no transformation needed)
   - Write back with `JSON.stringify(manifest, null, 2)` to `docs-branch/docs-manifest.json`
5. Commit in `docs-branch/`: `docs: publish {target-path}`
6. Push to docs branch

### `docs-on-pr.yml` — Automated trigger

**Location**: `.github/workflows/docs-on-pr.yml` on `main`

**Trigger**: `pull_request` events on `epic/*` and `feature/*` branches

**Permissions**: `contents: write`, `pull-requests: read`

| Event | Condition | Behavior |
|-------|-----------|----------|
| `opened`, `ready_for_review` | PR branch contains `docs/visual-explainer/*-spec.html` | Extract metadata from `<meta>` tags, call `publish-doc.yml` with `type: "spec"` |
| `labeled` with `ready-to-merge` | PR branch contains `docs/visual-explainer/*-impl.html` | Extract metadata from `<meta>` tags, call `publish-doc.yml` with `type: "impl"` |

The `ready-to-merge` label is applied manually by the developer when the PR is ready for final merge. This is the human gate before the impl explainer is published.

**Git ref for file discovery**: Use `github.event.pull_request.head.sha` as the ref for `git ls-tree` and for `source-ref` when calling `publish-doc.yml`. This ensures we inspect the actual PR head, not the merge commit.

**File discovery**: `git ls-tree -r --name-only $HEAD_SHA -- docs/visual-explainer/` then filter with grep for files matching `(epic|feature)-[0-9]+-.*-(spec|impl)\.html$`.

**Multiple files in one PR**: If multiple matching files are found (e.g., a spec and impl, or multiple features), the workflow iterates and calls `publish-doc.yml` once per file. Each call is independent.

**Metadata extraction from HTML** (via `grep` or `actions/github-script` reading the file):
```html
<meta name="doc-title" content="Architecture Spec">
<meta name="doc-desc" content="Supervisor, agent state machine, DAG engine">
<meta name="doc-epic" content="1">
<meta name="doc-feature" content="15">  <!-- optional, omit for epic-level docs -->
```

Fallback if meta tags missing: infer from filename pattern `{type}-{number}-{slug}-{spec|impl}.html` and pull title from the GitHub issue linked to the PR.

**Decisions and roadmap**: The automated trigger only handles `epics` and `features`. Decision and roadmap docs are published via `workflow_dispatch` (manual fallback) only, since they are created infrequently and don't follow the PR lifecycle.

---

## 5. File Naming Convention

**New files** (going forward):
```
docs/visual-explainer/epic-{n}-{slug}-spec.html
docs/visual-explainer/epic-{n}-{slug}-impl.html
docs/visual-explainer/feature-{n}-{slug}-spec.html
docs/visual-explainer/feature-{n}-{slug}-impl.html
```

**Legacy files** (existing, pre-convention):
```
visual-explainer/epic-1-go-orchestrator-core.html     (no -spec suffix)
visual-explainer/epic-2-terminal-substrate.html        (no -spec suffix)
visual-explainer/epic-3-model-gateway.html             (no -spec suffix)
```

Legacy files are NOT renamed. They are referenced by their actual filenames in the seeded manifest. The automated file discovery pattern (`*-(spec|impl).html`) will not match legacy files — this is intentional. Legacy files are already in the manifest from the migration seed and do not need re-publishing.

Published to docs branch at the same relative path without the `docs/` prefix:
```
visual-explainer/epic-{n}-{slug}-spec.html
```

---

## 6. Migration Plan

One-time commit to docs branch:

1. Create `docs-manifest.json` seeded with all 12 existing content files plus 1 placeholder:
   - 3 epic spec explainers (epics 1-3, using their legacy filenames without `-spec` suffix)
   - Epic 4 placeholder (empty docs array, no file yet)
   - 8 decision explainers
   - 1 roadmap
2. Replace `index.html` with the dynamic manifest-driven version
3. Existing HTML files unchanged — no renames, no broken links
4. Commit message: `docs: migrate index to manifest-driven dynamic rendering`

---

## 7. Developer Experience

### Spec phase
1. Finish writing spec → run `/visual-explainer` → run `/animate`
2. Save to `docs/visual-explainer/{type}-{n}-{slug}-spec.html` with `<meta>` tags
3. Commit, push → PR opened → auto-published to docs branch

### Implementation phase
1. Implementation done → run `/visual-explainer` → run `/animate`
2. Save to `docs/visual-explainer/{type}-{n}-{slug}-impl.html`
3. Commit, push → add `ready-to-merge` label → auto-published
4. Squash merge the PR

### Manual fallback
```bash
gh workflow run publish-doc.yml \
  -f html-file=docs/visual-explainer/epic-1-go-orchestrator-core-spec.html \
  -f target-path=visual-explainer/epic-1-go-orchestrator-core-spec.html \
  -f manifest-entry='{"section":"epics","epicNumber":1,"doc":{"title":"Architecture Spec","type":"spec"}}' \
  -f source-ref=epic/1-implement-go-orchestrator-core
```

### Invariants
- `index.html` is never manually edited after migration
- `docs-manifest.json` is only edited by the workflow (or manual dispatch) after the initial migration seed commit
- All **new** visual-explainer HTML files include `<meta name="doc-title">` and `<meta name="doc-desc">` (legacy files are grandfathered without meta tags)
- `prefers-reduced-motion` respected in all animations

---

## 8. Existing Content Mapping

| Current file | Manifest location | Type |
|-------------|-------------------|------|
| `visual-explainer/epic-1-go-orchestrator-core.html` | `epics[0].docs[0]` | spec |
| `visual-explainer/epic-2-terminal-substrate.html` | `epics[1].docs[0]` | spec |
| `visual-explainer/epic-3-model-gateway.html` | `epics[2].docs[0]` | spec |
| Epic 4 (no file yet) | `epics[3]` | placeholder |
| `decisions/orchestration-pattern.html` | `decisions[0]` | decision |
| `decisions/ipc-communication.html` | `decisions[1]` | decision |
| `decisions/state-management.html` | `decisions[2]` | decision |
| `decisions/process-supervision.html` | `decisions[3]` | decision |
| `decisions/terminal-substrate.html` | `decisions[4]` | decision |
| `decisions/desktop-rendering.html` | `decisions[5]` | decision |
| `decisions/model-gateway.html` | `decisions[6]` | decision |
| `decisions/tui-alternative.html` | `decisions[7]` | decision |
| `visual-explainer/roadmap.html` | `roadmap[0]` | roadmap |
