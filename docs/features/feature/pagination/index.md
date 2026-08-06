---
doc_type: feature_index
feature_id: feature-pagination
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - internal/pagination/pagination.go
  - internal/pagination/entries.go
  - internal/types/wire_log.go
  - internal/capture/actionstore/store.go
  - Makefile
test_paths:
  - internal/pagination/pagination_test.go
  - internal/pagination/cursor_test.go
  - internal/pagination/entries_actions_test.go
  - internal/pagination/entries_websocket_test.go
  - internal/pagination/pagination_bench_test.go
  - internal/pagination/test_helpers_test.go
  - internal/capture/actionstore/store_test.go
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Pagination

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
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

- `internal/pagination/pagination.go` owns the generic cursor model, parsing, and slicing engine.
- `internal/pagination/entries.go` owns the change-coupled action, log, and WebSocket adapters to the canonical telemetry wire types.
- `internal/pagination/test_helpers_test.go` centralizes cursor test scenario runners shared by action and WebSocket pagination suites.
- `internal/pagination/entries_actions_test.go` and `entries_websocket_test.go` validate adapter-specific slicing, eviction recovery, and serialization.
- `internal/pagination/pagination_test.go` now reuses shared before/after cursor runners and common log-entry fixture builders.
- Cursor properties live with cursor correctness tests; fuzz, benchmark, and isolated wall-clock SLO coverage live together in `pagination_bench_test.go`.
- The wall-clock assertions require the explicit marker supplied by
  `make test-performance`; concurrent coverage and race lanes cannot
  accidentally evaluate latency budgets under unrelated load.
- A package boundary regression test enforces the ten-file limit. Every file remains below 800 lines while keeping code that changes together under one owner.
- Log pagination consumes `internal/types.LogEntry` directly; the package-local
  compatibility alias has been removed.
- Action and WebSocket pagination tests consume `internal/types.EnhancedAction`
  and `internal/types.WebSocketEvent` directly; pagination does not re-export
  canonical entry contracts.
- Action pagination receives one coherent action snapshot and monotonic total
  from `capture.Telemetry().Actions()` so evidence and sequence metadata cannot
  be read through separate compatibility surfaces.
- Cursor construction uses direct integer formatting rather than general
  formatting, keeping the hot path below its 500 ns budget. Wall-clock cursor
  SLOs run through `make test-performance` with package and test parallelism
  disabled; the normal parallel unit lane skips only those timing assertions.
