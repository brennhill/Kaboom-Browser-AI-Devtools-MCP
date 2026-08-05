---
doc_type: feature_index
feature_id: feature-pagination
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - internal/pagination/pagination.go
  - internal/pagination/pagination_actions.go
  - internal/pagination/pagination_websocket.go
  - internal/pagination/pagination_logs.go
  - internal/pagination/cursor.go
  - internal/types/wire_log.go
  - Makefile
test_paths:
  - internal/pagination/pagination_test.go
  - internal/pagination/pagination_actions_test.go
  - internal/pagination/pagination_websocket_test.go
  - internal/pagination/pagination_coverage_test.go
  - internal/pagination/pagination_bench_test.go
  - internal/pagination/test_helpers_test.go
  - internal/pagination/cursor_slo_test.go
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Pagination

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/pagination`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_PAGINATION_001
- FEATURE_PAGINATION_002
- FEATURE_PAGINATION_003

## Code and Tests

- `internal/pagination/test_helpers_test.go` centralizes cursor test scenario runners shared by action and websocket pagination suites.
- `internal/pagination/pagination_actions_test.go` validates action cursor slicing, before/after cursors, and eviction restart behavior.
- `internal/pagination/pagination_websocket_test.go` validates websocket cursor slicing and eviction restart behavior using the shared runner.
- `internal/pagination/pagination_test.go` now reuses shared before/after cursor runners and common log-entry fixture builders.
- Log pagination consumes `internal/types.LogEntry` directly; the package-local
  compatibility alias has been removed.
- Action and WebSocket pagination tests consume `internal/types.EnhancedAction`
  and `internal/types.WebSocketEvent` directly; pagination does not re-export
  canonical entry contracts.
- Cursor construction uses direct integer formatting rather than general
  formatting, keeping the hot path below its 500 ns budget. Wall-clock cursor
  SLOs run through `make test-performance` with package and test parallelism
  disabled; the normal parallel unit lane skips only those timing assertions.
