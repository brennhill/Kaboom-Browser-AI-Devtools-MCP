---
doc_type: feature_index
feature_id: feature-request-session-correlation
status: active
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/observation/clientstore/owner.go
  - internal/observation/store/perf/store.go
  - internal/composition/capture/capture.go
  - cmd/browser-agent/main_connection_mcp.go
  - cmd/browser-agent/server.go
  - internal/tools/clientreg/http/handler.go
  - internal/tools/clientreg/registry.go
  - internal/tools/clientreg/state.go
  - internal/analysis/snapshots/snapshot-manager.go
  - internal/analysis/snapshots/comparison.go
  - internal/analysis/snapshots/snapdiff/types.go
  - internal/analysis/snapshots/snapdiff/network.go
  - internal/analysis/snapshots/snapdiff/performance.go
  - internal/observation/performance/types.go
  - internal/analysis/snapshots/runtime_reader.go
  - internal/analysis/snapshots/tool-handler.go
  - internal/tools/schema/configure/properties_runtime.go
  - internal/platform/types/snapshot.go
  - internal/platform/util/media.go
test_paths:
  - cmd/browser-agent/testdata/mcp-tools-list.golden.json
  - internal/observation/clientstore/owner_test.go
  - internal/observation/store/perf/store_test.go
  - internal/tools/clientreg/http/handler_test.go
  - internal/tools/clientreg/clientreg_test.go
  - internal/analysis/snapshots/snapshot_manager_test.go
  - internal/analysis/snapshots/sessions_test.go
  - internal/analysis/snapshots/sessions_compare_regression_test.go
  - internal/analysis/snapshots/tool_handler_test.go
  - internal/tools/schema/configure/schema_test.go
  - internal/analysis/snapshots/comparison_test.go
  - internal/analysis/snapshots/snapdiff/errors_test.go
  - internal/analysis/snapshots/snapdiff/network_test.go
  - internal/analysis/snapshots/snapdiff/performance_test.go
  - internal/observation/performance/wire_performance_test.go
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

> **2026-07-26:** the `verify_fix` half of this feature was removed as dead code.
> `internal/analysis/snapshots/verify` had zero importers outside its own tests and `verify_fix`
> was never registered as an MCP action, so the before/after verification loop was
> unreachable at runtime. The session-correlation paths below are live and unaffected.


# Request Session Correlation

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/protocol/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
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

`internal/platform/types/snapshot.go` owns the snapshot contract. Session capture and
diff modules consume those types directly; they do not re-export package-local
aliases. Snapshot manager, comparison, and diff tests exercise the same
canonical contract. Network diffing likewise consumes URL-path normalization
directly from `internal/platform/util`, without routing through capture.
The session package is bounded at ten files. `snapshot-manager.go` owns its
aggregate state, reader boundary, validation constants, and CRUD helpers;
runtime projection, comparison, and MCP dispatch retain distinct owners.
Client-session registry installation is owned by the focused
`internal/observation/clientstore` package. `Capture.Clients()` exposes that
canonical owner directly and stores the concrete `clientreg.ClientRegistry`.
Runtime routes and terminal CWD detection consume that registry without
interfaces returning `any`, adapters, JSON round trips, Capture forwarding
methods, or root-package contract aliases.
The canonical `internal/tools/clientreg/http` owner installs and handles `/clients`
routes. It exposes only the snake_case `ClientInfo` projection, never internal
cursor or checkpoint state, and the root server contains no client-route
helpers or compatibility facade.
Each client state owns a private activity clock used by `Touch`; tests advance
that source explicitly. Concurrent register/unregister coverage starts all
workers through a barrier rather than delaying between operations.

The runtime retains a bounded chronological history independently from its
latest-by-URL index. Named snapshots retain up to 20 recent performance
samples for the same page, and Doctor exposes sample pressure and drops.
The capture composition root installs the focused performance store directly.
That owner retains bounded pre-action baselines by correlation ID and consumes
them on read, preventing stale baselines from being reused across actions.
Comparisons report Core Web Vitals, transfer and execution cost, duplicate
requests, critical-path changes, median/p75 distributions, sample sufficiency,
and optional absolute regression-budget verdicts.
Performance-budget names come from one sorted canonical metric registry. The
configure schema constrains object property names to that registry, runtime
validation rejects unknown names precisely, and parity tests prove every
documented metric is accepted by both boundaries.
