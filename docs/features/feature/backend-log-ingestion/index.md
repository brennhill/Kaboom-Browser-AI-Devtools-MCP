---
doc_type: feature_index
feature_id: feature-backend-log-ingestion
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-29
code_paths:
  - cmd/browser-agent/internal/logstore/async.go
  - cmd/browser-agent/internal/logstore/store.go
test_paths:
  - cmd/browser-agent/internal/logstore/ring_window_test.go
  - cmd/browser-agent/internal/logstore/ring_window_bench_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Log Ingestion

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/backend-log-ingestion`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_BACKEND_LOG_INGESTION_001
- FEATURE_BACKEND_LOG_INGESTION_002
- FEATURE_BACKEND_LOG_INGESTION_003

## Code and Tests

The daemon's in-memory log window uses fixed-capacity circular storage in
`cmd/browser-agent/internal/logstore`. HTTP ingestion overwrites the oldest slot in place after
capacity is reached, while detached snapshots preserve chronological order for
readers and persistence compaction.
