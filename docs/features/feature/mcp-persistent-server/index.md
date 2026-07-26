---
doc_type: feature_index
feature_id: feature-mcp-persistent-server
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-26
code_paths:
  - cmd/browser-agent/mcp_identity.go
  - cmd/browser-agent/bridge_adapter.go
  - cmd/browser-agent/internal/bridge/bridge.go
  - cmd/browser-agent/internal/bridge/bridge_startup_orchestration.go
  - cmd/browser-agent/internal/daemonlife/lifecycle.go
  - cmd/browser-agent/internal/daemonlife/lock_file.go
  - cmd/browser-agent/internal/daemonlife/install_epoch.go
  - cmd/browser-agent/internal/daemonlife/startup_throttle.go
  - cmd/browser-agent/internal/daemonlife/version_compare.go
  - cmd/browser-agent/internal/daemonlife/deps.go
  - cmd/browser-agent/daemon_lifecycle_wiring.go
  - cmd/browser-agent/main_connection_mcp.go
  - cmd/browser-agent/main_connection_mcp_bootstrap.go
  - cmd/browser-agent/main_connection_recovery.go
  - cmd/browser-agent/server_routes_health_diagnostics.go
  - cmd/browser-agent/main_connection_mcp_shutdown.go
  - cmd/browser-agent/server_middleware.go
  - cmd/browser-agent/handler_http.go
  - cmd/browser-agent/connect_mode.go
  - cmd/browser-agent/server_routes_media_screenshots.go
  - internal/identity/mcp.go
  - internal/util/proc_unix.go
  - internal/util/proc_windows.go
test_paths:
  - cmd/browser-agent/terminal_availability_test.go
  - cmd/browser-agent/reclaim_port_identity_test.go
  - cmd/browser-agent/reclaim_port_test.go
  - cmd/browser-agent/bridge_startup_contention_test.go
  - cmd/browser-agent/bridge_faststart_extended_test.go
  - cmd/browser-agent/handler_http_headers_test.go
  - cmd/browser-agent/server_middleware_test.go
  - cmd/browser-agent/connect_mode_run_test.go
  - cmd/browser-agent/handler_consistency_test.go
  - cmd/browser-agent/server_routes_unit_test.go
  - cmd/browser-agent/main_connection_diag_test.go
  - cmd/browser-agent/internal/bridge/bridge_fastpath_unit_test.go
  - cmd/browser-agent/internal/bridge/bridge_detach_stdio_test.go
  - cmd/browser-agent/internal/bridge/bridge_deps_isolation_test.go
  - cmd/browser-agent/internal/daemonlife/lifecycle_takeover_test.go
  - cmd/browser-agent/internal/daemonlife/lifecycle_policy_test.go
  - cmd/browser-agent/internal/daemonlife/install_epoch_test.go
  - cmd/browser-agent/internal/daemonlife/startup_throttle_test.go
  - cmd/browser-agent/internal/daemonlife/version_compare_test.go
  - cmd/browser-agent/internal/daemonlife/helpers_test.go
  - cmd/browser-agent/daemon_lifecycle_policy_test.go
  - cmd/browser-agent/daemon_lifecycle_wiring_test.go
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

## Related Architecture
- [MCP Daemon Lifecycle](../../../architecture/mcp-daemon-lifecycle.md)
