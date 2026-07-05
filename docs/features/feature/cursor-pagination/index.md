---
doc_type: feature_index
feature_id: feature-cursor-pagination
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - internal/pagination/cursor.go
  - internal/pagination/pagination.go
  - internal/pagination/pagination_actions.go
  - internal/pagination/pagination_websocket.go
  - internal/pagination/serialization.go
  - internal/pagination/test_helpers_test.go
test_paths:
  - internal/pagination/cursor_test.go
  - internal/pagination/pagination_test.go
  - internal/pagination/pagination_actions_test.go
  - internal/pagination/pagination_websocket_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Cursor Pagination

## TL;DR

- Status: shipped
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/cursor-pagination`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_CURSOR_PAGINATION_001
- FEATURE_CURSOR_PAGINATION_002
- FEATURE_CURSOR_PAGINATION_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
