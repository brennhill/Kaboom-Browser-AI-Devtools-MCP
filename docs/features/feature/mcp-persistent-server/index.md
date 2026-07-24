---
doc_type: feature_index
feature_id: feature-mcp-persistent-server
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-29
code_paths:
  - cmd/browser-agent/mcp_identity.go
  - cmd/browser-agent/internal/bridge/bridge.go
  - cmd/browser-agent/internal/bridge/bridge_startup_orchestration.go
  - cmd/browser-agent/server_middleware.go
  - cmd/browser-agent/handler_http.go
  - cmd/browser-agent/connect_mode.go
  - cmd/browser-agent/server_routes_media_screenshots.go
  - internal/identity/mcp.go
  - internal/util/proc_unix.go
  - internal/util/proc_windows.go
test_paths:
  - cmd/browser-agent/bridge_startup_contention_test.go
  - cmd/browser-agent/bridge_faststart_extended_test.go
  - cmd/browser-agent/handler_http_headers_test.go
  - cmd/browser-agent/server_middleware_test.go
  - cmd/browser-agent/connect_mode_run_test.go
  - cmd/browser-agent/handler_consistency_test.go
  - cmd/browser-agent/server_routes_unit_test.go
  - cmd/browser-agent/main_connection_diag_test.go
  - cmd/browser-agent/internal/bridge/bridge_fastpath_unit_test.go
  - cmd/browser-agent/test_timing_test.go
  - cmd/browser-agent/bridge_faststart_test.go
  - tests/regression/08-fast-start/test-fast-start.sh
last_verified_version: 0.8.1
last_verified_date: 2026-03-29
---

# MCP Persistent Server

## TL;DR
- Status: shipped
- Scope: long-lived daemon lifecycle across client reconnects

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Fast-Start Test Timing

The fast-start tests spawn the real binary and assert latency, which makes them the repo's main flake surface. Two rules keep them deterministic; both live in `cmd/browser-agent/test_timing_test.go`.

**1. Budgets measure steady state, never cold start.** The first exec of a freshly built 16 MB coverage-instrumented binary costs **~520 ms** (page-in from disk + macOS code-signature validation); every exec after is **~0 ms**. That one-time cost used to land inside whichever test spawned first, which is why `TestFastStart_ClientCompatibilityMatrix/claude_code` — the first subtest — blew its read timeout under full-suite load while subtests 2–4 passed. `buildTestBinary` now runs the binary once (`--version`, a pure print-and-exit path) immediately after building, so no measurement pays it.

**2. A liveness timeout is a hang guard, not an assertion.** `testLivenessTimeout` (30s) sits far above every budget so a slow-but-alive process fails as a *budget miss reporting its measured elapsed time*, not as a bare "timeout waiting for response". The old code read `initialize` with a 5s timeout while asserting a 4s budget — one second apart — so a loaded machine produced the uninformative failure instead of the useful one. `TestLivenessTimeoutExceedsLatencyBudgets` enforces a 3x margin so this cannot silently regress.

| Constant | Value | Meaning |
|----------|-------|---------|
| `testLivenessTimeout` | 30s | hang guard; exceeding it means *no* response |
| `fastStartInitBudget` | 4s | `initialize` round-trip including process spawn |
| `fastStartResourceBudget` | 500ms | `resources/read` on an initialized bridge |
| `fastStartWarmBudget` | 100ms | any request on an already-initialized bridge |

## Related Architecture
- [Daemon Stop and Force Cleanup](../../../architecture/flow-maps/daemon-stop-and-force-cleanup.md)
- [MCP Daemon Lifecycle](../../../architecture/flow-maps/mcp-daemon-lifecycle.md)
