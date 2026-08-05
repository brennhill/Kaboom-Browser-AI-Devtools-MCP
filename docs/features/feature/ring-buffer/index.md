---
doc_type: feature_index
feature_id: feature-ring-buffer
status: active
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - internal/buffers/ring_buffer.go
  - internal/buffers/ring_buffer_filter.go
  - internal/buffers/filter.go
test_paths:
  - internal/buffers/ring_buffer_test.go
  - internal/buffers/ring_buffer_stress_test.go
  - internal/buffers/ring_buffer_property_test.go
  - internal/buffers/ring_buffer_slo_test.go
  - internal/buffers/filter_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Ring Buffer

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/ring-buffer`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_RING_BUFFER_001
- FEATURE_RING_BUFFER_002
- FEATURE_RING_BUFFER_003

## Code and Tests

- `internal/buffers/ring_buffer.go` owns fixed-capacity storage, monotonic
  cursors, timestamp lookup, wraparound, and clear semantics.
- The buffer owns a private clock boundary. Production uses `time.Now`; package
  tests provide a fixed clock so timestamp lookup has exact before/after
  boundaries without sleeping.
- Concurrency and stress tests release readers, writers, and clearers from a
  shared start barrier. Race coverage therefore exercises the same operation
  set without depending on scheduler delays.
- The package is intentionally bounded to ten Go files. Cursor-resolution
  cases live with core buffer behavior, while benchmarks live with the SLO
  tests they measure; historical migration smoke tests are not retained as a
  parallel behavioral suite.
