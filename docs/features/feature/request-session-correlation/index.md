---
doc_type: feature_index
feature_id: feature-request-session-correlation
status: active
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - internal/capture/model.go
  - internal/capture/capture.go
  - cmd/browser-agent/main_connection_mcp.go
  - cmd/browser-agent/server.go
  - internal/session/clientreg/registry.go
  - internal/session/clientreg/state.go
  - internal/session/types.go
  - internal/session/snapshot-manager.go
  - internal/session/comparison.go
  - internal/session/snapdiff/types.go
  - internal/session/snapdiff/network.go
  - internal/types/snapshot.go
  - internal/util/url.go
test_paths:
  - cmd/browser-agent/server_routes_clients_test.go
  - internal/session/clientreg/clientreg_test.go
  - internal/session/snapshot_manager_test.go
  - internal/session/comparison_test.go
  - internal/session/snapdiff/errors_test.go
  - internal/session/snapdiff/network_test.go
  - internal/session/snapdiff/performance_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **2026-07-26:** the `verify_fix` half of this feature was removed as dead code.
> `internal/session/verify` had zero importers outside its own tests and `verify_fix`
> was never registered as an MCP action, so the before/after verification loop was
> unreachable at runtime. The session-correlation paths below are live and unaffected.


# Request Session Correlation

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/request-session-correlation`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_REQUEST_SESSION_CORRELATION_001
- FEATURE_REQUEST_SESSION_CORRELATION_002
- FEATURE_REQUEST_SESSION_CORRELATION_003

## Code and Tests

`internal/types/snapshot.go` owns the snapshot contract. Session capture and
diff modules consume those types directly; they do not re-export package-local
aliases. Snapshot manager, comparison, and diff tests exercise the same
canonical contract. Network diffing likewise consumes URL-path normalization
directly from `internal/util`, without routing through capture.
