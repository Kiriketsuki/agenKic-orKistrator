# CI/CD Guide — kTemplate

## Overview

Calendar-based versioning (`YY.MM.Major.Minor`) with automated version bumping, issue-to-branch automation, and GitHub release management.

## Workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `version-validation.yml` | PR to `main` or `release` | Validate `VERSION` file format and uniqueness vs latest tag |
| `version-bump.yml` | PR merged to `main` / direct push with `hotfix:` | Auto-increment version in `VERSION` file |
| `manual-version-bump.yml` | `workflow_dispatch` | Monthly calendar rollover |
| `release.yml` | PR merged to `release` / push to `release` | Sync VERSION from main, create Git tag + GitHub Release |
| `issue-branch-handler.yml` | Issue labeled `task`, `feature`, or `bug` | Create branch + draft PR + sub-issue parent tracking |

## VERSION File

Located at the repository root. Single source of truth for the version.

```text
26.03.0.0
```

**Format**: `YY.MM.Major.Minor[suffix]`
- YY = two-digit year
- MM = two-digit month (01–12)
- Major = feature/task counter (accumulates through the year)
- Minor = bug fix counter (resets to 0 when Major increments)
- suffix = hotfix letters (a, b, … z, A …) — rare

## Branch Strategy

```
main          ← protected
release       ← production releases
task/{n}-...  ← large work units (branch from main)
feature/{n}-… ← features (branch from task or main)
bug/{n}-…     ← bug fixes (branch from feature, task, or main)
```

Sub-issues automatically branch from their parent issue's branch via `issue-branch-handler.yml`.

## Version Bumping Rules

| Branch type merged → main | Bump |
|---------------------------|------|
| `task/*` or `feature/*` | Major +1, Minor → 0 |
| `bug/*` or `hotfix/*` | Minor +1 |
| Direct push with `hotfix:` in message | Suffix (a, b, … z, A …) |

## Issue Hierarchy

```
Task (task label)
└── Feature (feature label, sub-issue of Task)
    └── Bug (bug label, sub-issue of Feature or Task)
```

- Create a Task → branch `task/{n}-...` auto-created from `main`, draft PR opened
- Add Feature as sub-issue of Task → branch `feature/{n}-...` auto-created from `task/*`
- Add Bug as sub-issue of Feature → branch `bug/{n}-...` auto-created from `feature/*`

## Monthly Version Rollover

Run **Manual Monthly Version Bump** from Actions → workflow_dispatch:
- New month (same year): preserves Major, resets Minor
- New year: resets to `YY.01.0.0`

## Release Process

1. Create PR from `main` → `release` (use **Rebase**)
2. `version-validation.yml` checks format and uniqueness vs latest tag
3. After merge, `release.yml` creates tag `v{VERSION}` and a GitHub Release

## Setting Up the Release Branch

```bash
git checkout main
git pull origin main
git checkout -b release
git push origin release
```

## Branch Protection (configure via GitHub settings)

### main
- Require PR before merging (1 approval)
- Dismiss stale reviews
- Required status checks: `validate-version`
- Require linear history
- No force pushes, no deletions

### release
- Require PR before merging (1 approval)
- Required status checks: `validate-version`
- Require linear history
- No force pushes, no deletions

## Labels to Create

Create these labels in the repo for the workflows to trigger correctly:

| Label | Color | Purpose |
|-------|-------|---------|
| `task` | `#0075ca` | Top-level work unit |
| `feature` | `#a2eeef` | Feature within a task |
| `bug` | `#d73a4a` | Bug fix |
| `implementation` | `#e4e669` | Auto-added to task PRs |
| `addition` | `#0e8a16` | Auto-added to feature PRs |
| `fix` | `#ee0701` | Auto-added to bug PRs |
