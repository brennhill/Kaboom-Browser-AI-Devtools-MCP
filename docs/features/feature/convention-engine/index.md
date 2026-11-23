---
doc_type: feature_index
feature_id: feature-convention-engine
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - internal/hooks/conventions/conventions.go
  - internal/hooks/projectscan/projectscan.go
  - internal/hooks/hookdiag/hookdiag.go
  - internal/platform/state/paths/paths.go
  - internal/hooks/hook_policy.go
test_paths:
  - internal/hooks/conventions/conventions_test.go
  - internal/hooks/conventions/discovery_test.go
  - internal/hooks/projectscan/projectscan_test.go
  - internal/hooks/hookdiag/hookdiag_test.go
  - internal/hooks/hook_policy_test.go
  - internal/hooks/eval/testdata/quality-gate/
---

# Convention Engine

| Field         | Value                                    |
|---------------|------------------------------------------|
| **Status**    | proposed (phase 1 built)                 |
| **Extends**   | quality-gates                            |
| **Issue**     | TBD                                      |

## Specs

- [Product Spec](./product-spec.md) — 10 universal principles, plugin architecture, 4-step cycle, monetization

## Summary

Automatic convention discovery and enforcement via plugins. The engine discovers what a codebase does (call-site patterns, structural patterns), suggests what it should do (pattern catalog assessment), enforces settled standards (approved conventions), and handles re-architecture without thrash (migration declarations).

Three plugin tiers: universal (10 principles, always active, free), language base (Go, TS, Python, C# — auto-activated), framework (Gin, React, FastAPI — import-detected, paid).

## Current State (Phase 1)

- Call-site discovery engine and edited-file detection share the canonical
  `internal/hooks/conventions` package (`Detect`, `Discover`, `Summary`,
  `Format`, `Probes`). Repository-walk pruning lives in
  `internal/hooks/projectscan` so the convention scanner and the blast-radius
  scanner cannot disagree about which files the project contains.
- The injected summary never splits a tie: when the 10th and 11th patterns
  appear in the same number of files the cut extends over the whole tie group,
  so adding one file anywhere in the repository can no longer swap which
  convention a reviewer is told the project uses.
- Convention summary injected on every Edit/Write
- Discovered probes integrated into existing convention detection
- 5-minute cache per project root + language
- Deterministic ranking: frequency descending, then pattern text ascending for
  ties, so concurrent cache fills return identical ordered results
- Detection and discovery share one canonical source-file filter, keeping
  extension, generated-file, and size exclusions identical.
- Noise filtering for Go (90+ stdlib patterns) and TS/JS (25+ patterns)
- Eval fixtures validate discovery against kaboom codebase itself
