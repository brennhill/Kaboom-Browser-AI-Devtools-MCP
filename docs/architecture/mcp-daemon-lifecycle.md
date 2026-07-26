---
doc_type: flow_map
flow_id: mcp-daemon-lifecycle
status: active
last_reviewed: 2026-03-29
owners:
  - Brenn
entrypoints:
  - cmd/browser-agent/main.go:dispatchMode
  - cmd/browser-agent/main_connection_mcp.go:runMCPMode
code_paths:
  - cmd/browser-agent/mcp_identity.go
  - cmd/browser-agent/main_connection_mcp.go
  - cmd/browser-agent/main_connection_mcp_bootstrap.go
  - cmd/browser-agent/main_connection_mcp_upgrade.go
  - cmd/browser-agent/main_connection_mcp_shutdown.go
  - cmd/browser-agent/daemon_lifecycle.go
  - cmd/browser-agent/daemon_lock_file.go
  - cmd/browser-agent/server_routes.go
  - cmd/browser-agent/server_middleware.go
  - cmd/browser-agent/handler_http.go
  - cmd/browser-agent/connect_mode.go
  - cmd/browser-agent/server_routes_media_screenshots.go
  - internal/identity/mcp.go
test_paths:
  - cmd/browser-agent/main_connection_coverage_test.go
  - cmd/browser-agent/main_connection_diag_test.go
  - cmd/browser-agent/main_connection_pid_contract_test.go
  - cmd/browser-agent/handler_http_headers_test.go
  - cmd/browser-agent/server_middleware_test.go
  - cmd/browser-agent/connect_mode_run_test.go
  - cmd/browser-agent/handler_consistency_test.go
  - cmd/browser-agent/server_routes_unit_test.go
  - cmd/browser-agent/daemon_lifecycle_policy_test.go
  - cmd/browser-agent/runtime_mode_test.go
last_verified_version: 0.8.1
last_verified_date: 2026-03-29
---

# MCP Daemon Lifecycle

## Scope

Covers daemon startup, HTTP bind, PID/lock persistence, upgrade watcher wiring, and shutdown semantics.

## Entrypoints

- `dispatchMode` selects daemon vs bridge runtime.
- `runMCPMode` orchestrates daemon boot and lifecycle.

## Primary Flow

1. `dispatchMode` calls `runMCPMode` in daemon mode.
2. `runMCPMode` initializes capture and routes.
3. Startup loops begin (`startVersionCheckLoop`, screenshot limiter cleanup).
4. Upgrade monitoring is configured (`configureBinaryUpgradeMonitoring`).
5. Launch mode is classified (`persistent` vs `likely_transient`) and one-shot warning/strict policy is applied.
6. Startup policy + stale PID + port preflight checks run.
7. HTTP server is created and bound (`startHTTPServer`).
8. PID file and daemon lock are persisted.
9. Runtime blocks on `awaitShutdownSignal`.
10. Runtime identity surfaces resolve canonical server naming from shared identity constants.
11. Extension-facing HTTP routes and connect-mode forwarding use the `X-Kaboom-*` header family for per-client routing, diagnostics, and screenshot rate limiting.

## Error and Recovery Paths

- Port bind failures return early with lifecycle logging.
- Stale PID with owner mismatch is removed before bind retry.
- Unexpected HTTP listener exit triggers synthetic shutdown to avoid zombie daemon.
- Upgrade detection writes marker and triggers SIGTERM for controlled restart.

## State and Contracts

- `binaryUpgradeState` is shared read state for health/handler warnings.
- PID file + daemon lock reflect active owner and are cleared on shutdown.
- `httpDone` channel is the listener liveness signal.
- Launch-mode classification is surfaced in health/diagnostics for operator visibility.
- Canonical server name (`kaboom-browser-devtools`) is sourced from `internal/identity` to avoid cross-package drift.
- Extension traffic is keyed by `X-Kaboom-Client`; HTTP debug context also records `X-Kaboom-Ext-Session` and `X-Kaboom-Extension-Version`.

## Code Paths

- `cmd/browser-agent/mcp_identity.go`
- `cmd/browser-agent/main_connection_mcp.go`
- `cmd/browser-agent/main_connection_mcp_bootstrap.go`
- `cmd/browser-agent/main_connection_mcp_upgrade.go`
- `cmd/browser-agent/main_connection_mcp_shutdown.go`
- `cmd/browser-agent/daemon_lifecycle.go`
- `cmd/browser-agent/daemon_lock_file.go`
- `cmd/browser-agent/server_middleware.go`
- `cmd/browser-agent/handler_http.go`
- `cmd/browser-agent/connect_mode.go`
- `cmd/browser-agent/server_routes_media_screenshots.go`
- `internal/identity/mcp.go`

## Test Paths

- `cmd/browser-agent/main_connection_coverage_test.go`
- `cmd/browser-agent/main_connection_diag_test.go`
- `cmd/browser-agent/main_connection_pid_contract_test.go`
- `cmd/browser-agent/handler_http_headers_test.go`
- `cmd/browser-agent/server_middleware_test.go`
- `cmd/browser-agent/connect_mode_run_test.go`
- `cmd/browser-agent/handler_consistency_test.go`
- `cmd/browser-agent/server_routes_unit_test.go`
- `cmd/browser-agent/daemon_lifecycle_policy_test.go`
- `cmd/browser-agent/runtime_mode_test.go`

## Edit Guardrails

- Keep `runMCPMode` orchestration-only; push details into helper files.
- Any change in lifecycle ordering must update this flow map and tests above.
- Preserve structured lifecycle event names for diagnostics compatibility.
