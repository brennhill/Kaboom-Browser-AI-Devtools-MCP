---
doc_type: feature_index
feature_id: feature-environment-manipulation
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - cmd/browser-agent/internal/toolinteract/interact_storage.go
test_paths:
  - cmd/browser-agent/internal/toolinteract/interact_storage_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Environment Manipulation

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/environment-manipulation`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ENVIRONMENT_MANIPULATION_001
- FEATURE_ENVIRONMENT_MANIPULATION_002
- FEATURE_ENVIRONMENT_MANIPULATION_003

## Code and Tests

- Storage and cookie mutation handlers share one canonical execution-target
  contract for tab, timeout, and JavaScript world selection.
- Characterization tests verify that every storage mutation preserves that
  contract through the queued extension command.
