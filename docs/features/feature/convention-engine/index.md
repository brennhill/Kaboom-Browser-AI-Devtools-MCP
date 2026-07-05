---
doc_type: feature_index
feature_id: feature-convention-engine
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - internal/hook/convention_discover.go
  - internal/hook/convention_detect.go
  - internal/hook/quality_gate.go
test_paths:
  - internal/hook/convention_discover_test.go
  - internal/hook/convention_detect_test.go
  - internal/hook/quality_gate_test.go
  - internal/hook/eval/testdata/quality-gate/
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

- Call-site discovery engine built (`convention_discover.go`)
- Convention summary injected on every Edit/Write
- Discovered probes integrated into existing convention detection
- 5-minute cache per project root + language
- Noise filtering for Go (90+ stdlib patterns) and TS/JS (25+ patterns)
- Eval fixtures validate discovery against kaboom codebase itself
