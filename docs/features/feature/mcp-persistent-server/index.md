---
doc_type: feature_index
feature_id: feature-mcp-persistent-server
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/handler.go
  - cmd/browser-agent/handler_tools_call.go
  - cmd/browser-agent/bridge_adapter.go
  - cmd/browser-agent/tools_core.go
  - internal/session/snapshot-manager.go
  - cmd/browser-agent/tools_registry.go
  - cmd/browser-agent/types.go
  - cmd/browser-agent/server.go
  - cmd/browser-agent/server_routes.go
  - cmd/browser-agent/internal/playbooks/resource_catalog.go
  - cmd/browser-agent/internal/playbooks/playbooks_resolver.go
  - cmd/browser-agent/bridge_adapter.go
  - cmd/browser-agent/internal/bridge/bridge.go
  - cmd/browser-agent/internal/bridge/bridge_startup.go
  - cmd/browser-agent/internal/bridge/bridge_transport.go
  - cmd/browser-agent/internal/bridge/stdioisolate/isolation.go
  - cmd/browser-agent/internal/daemonlife/lifecycle.go
  - cmd/browser-agent/internal/daemonlife/lock_file.go
  - cmd/browser-agent/internal/daemonlife/install_epoch.go
  - cmd/browser-agent/internal/daemonlife/startup_throttle.go
  - cmd/browser-agent/internal/daemonlife/version_compare.go
  - cmd/browser-agent/internal/daemonlife/deps.go
  - cmd/browser-agent/main_connection_recovery.go
  - cmd/browser-agent/main_connection_stop.go
  - cmd/browser-agent/main_connection_mcp.go
  - cmd/browser-agent/internal/procctl/pidfile.go
  - cmd/browser-agent/internal/procctl/port.go
  - cmd/browser-agent/internal/procctl/argv0.go
  - cmd/browser-agent/main_connection_recovery.go
  - cmd/browser-agent/server_routes_diagnostics.go
  - cmd/browser-agent/dashboard.go
  - cmd/browser-agent/internal/logstore/store.go
  - cmd/browser-agent/internal/logstore/async.go
  - cmd/browser-agent/internal/logstore/accessors.go
  - cmd/browser-agent/internal/logstore/persistence.go
  - cmd/browser-agent/internal/logstore/validate.go
  - cmd/browser-agent/internal/logstore/seed.go
  - cmd/browser-agent/main_connection_mcp_shutdown.go
  - cmd/browser-agent/exit_diagnostics.go
  - cmd/browser-agent/internal/httpguard/middleware.go
  - cmd/browser-agent/handler_http.go
  - cmd/browser-agent/connect_mode.go
  - internal/diag/output.go
  - internal/diag/debug_file.go
  - cmd/browser-agent/server_routes_media_screenshots.go
  - internal/identity/mcp.go
  - internal/util/proc_unix.go
  - internal/util/proc_windows.go
test_paths:
  - cmd/browser-agent/health_unit_test.go
  - cmd/browser-agent/command_execution_readiness_test.go
  - cmd/browser-agent/handler_unit_test.go
  - cmd/browser-agent/handler_unit_telemetry_test.go
  - cmd/browser-agent/main_connection_stop_test.go
  - cmd/browser-agent/test_daemon_cleanup_test.go
  - cmd/browser-agent/main_connection_pid_contract_test.go
  - internal/diag/output_test.go
  - internal/diag/debug_file_test.go
  - cmd/browser-agent/internal/playbooks/resource_catalog_test.go
  - cmd/browser-agent/stdout_protocol_boundary_test.go
  - cmd/browser-agent/mcp_protocol_test.go
  - cmd/browser-agent/stdout_sync_unit_test.go
  - cmd/browser-agent/tools_registry_test.go
  - cmd/browser-agent/terminal_availability_test.go
  - cmd/browser-agent/reclaim_port_identity_test.go
  - cmd/browser-agent/reclaim_port_test.go
  - cmd/browser-agent/bridge_startup_contention_test.go
  - cmd/browser-agent/bridge_faststart_extended_test.go
  - cmd/browser-agent/handler_http_headers_test.go
  - cmd/browser-agent/internal/httpguard/middleware_test.go
  - cmd/browser-agent/connect_mode_run_test.go
  - cmd/browser-agent/handler_consistency_test.go
  - cmd/browser-agent/server_routes_unit_test.go
  - cmd/browser-agent/server_routes_debug_usage_test.go
  - cmd/browser-agent/dashboard_test.go
  - cmd/browser-agent/exit_diagnostics_test.go
  - cmd/browser-agent/internal/bridge/bridge_fastpath_unit_test.go
  - cmd/browser-agent/internal/bridge/bridge_detach_stdio_test.go
  - cmd/browser-agent/internal/bridge/bridge_detach_contract_test.go
  - cmd/browser-agent/internal/bridge/bridge_context_contract_test.go
  - cmd/browser-agent/internal/bridge/health_metadata_test.go
  - cmd/browser-agent/internal/bridge/bridge_deps_isolation_test.go
  - cmd/browser-agent/internal/bridge/stdioisolate/isolation_unix_test.go
  - cmd/browser-agent/internal/daemonlife/lifecycle_takeover_test.go
  - cmd/browser-agent/internal/daemonlife/lifecycle_policy_test.go
  - cmd/browser-agent/internal/daemonlife/install_epoch_test.go
  - cmd/browser-agent/internal/daemonlife/startup_throttle_test.go
  - cmd/browser-agent/internal/daemonlife/version_compare_test.go
  - cmd/browser-agent/internal/daemonlife/helpers_test.go
  - cmd/browser-agent/daemon_lifecycle_policy_test.go
  - cmd/browser-agent/daemon_lifecycle_wiring_test.go
  - cmd/browser-agent/server_core_unit_test.go
  - cmd/browser-agent/internal/procctl/pidfile_test.go
  - cmd/browser-agent/internal/procctl/port_test.go
  - cmd/browser-agent/internal/procctl/port_netstat_test.go
  - cmd/browser-agent/internal/procctl/argv0_test.go
  - cmd/browser-agent/internal/logstore/store_test.go
  - cmd/browser-agent/internal/logstore/async_test.go
  - cmd/browser-agent/internal/logstore/validate_test.go
  - tests/regression/08-fast-start/test-fast-start.sh
last_verified_version: 0.8.1
last_verified_date: 2026-03-29
---

# MCP Persistent Server

> **2026-07-27:** Removed the unreachable `main_connection_diag.go`
> connection-probing path and its dedicated tests. No production caller invoked
> it. Two unrelated stop-mode tests in the same file were also obsolete because
> they asserted human diagnostics on protocol stdout.

## TL;DR
- Status: shipped
- Scope: long-lived daemon lifecycle across client reconnects

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Related Architecture
