---
doc_type: feature_index
feature_id: feature-cursor-pagination
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - internal/pagination/pagination.go
  - internal/pagination/entries.go
test_paths:
  - internal/pagination/cursor_test.go
  - internal/pagination/pagination_test.go
  - internal/pagination/entries_actions_test.go
  - internal/pagination/entries_websocket_test.go
  - internal/pagination/pagination_bench_test.go
  - internal/pagination/test_helpers_test.go
last_verified_version: 0.9.0
last_verified_date: 2026-08-05
---

# Cursor Pagination

## TL;DR

- Status: shipped
- Tool: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/cursor-pagination`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_CURSOR_PAGINATION_001
- FEATURE_CURSOR_PAGINATION_002
- FEATURE_CURSOR_PAGINATION_003

## Code and Tests

`internal/pagination/pagination.go` is the single generic cursor engine. Domain
adaptation is isolated in `internal/pagination/entries.go`, which consumes the
canonical telemetry wire types without re-exporting them. Correctness,
property, fuzz, performance, and adapter suites exercise the same public
boundary, while a package regression test enforces the ten-file architecture
limit.
