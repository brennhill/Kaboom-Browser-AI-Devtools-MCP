---
doc_type: feature_index
feature_id: draw-mode
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-29
code_paths:
  - extension/content/draw-mode.js
test_paths:
  - tests/extension/draw-mode/draw-mode.test.js
  - tests/extension/draw-mode/draw-mode-drawing.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Draw Mode

## TL;DR

- Status: proposed
- Tool: interact, analyze
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/draw-mode`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- DRAW_MODE_001
- DRAW_MODE_002
- DRAW_MODE_003

## Code and Tests

Draw mode uses standard modal keyboard semantics: Enter saves the active
annotation, a subsequent Enter submits completed annotations, and Escape
cancels the session without delivering results.
