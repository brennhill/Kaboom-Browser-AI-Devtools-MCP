---
doc_type: feature_index
feature_id: feature-buffer-clearing
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/toolconfigure/clear.go
  - cmd/browser-agent/tools_configure.go
test_paths:
  - cmd/browser-agent/tools_configure_coverage_test.go
  - cmd/browser-agent/tools_configure_handler_test.go
  - cmd/browser-agent/tools_configure_clear_annotations_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Buffer Clearing

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/buffer-clearing`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_BUFFER_CLEARING_001
- FEATURE_BUFFER_CLEARING_002
- FEATURE_BUFFER_CLEARING_003

## Code and Tests

`internal/toolconfigure/clear.go` owns request parsing and the clearing policy
for capture, log, push inbox, and annotation stores. The root configure registry
supplies those stores explicitly through `ClearTargets`.
