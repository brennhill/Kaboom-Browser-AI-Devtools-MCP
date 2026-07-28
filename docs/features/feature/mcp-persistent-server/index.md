---
doc_type: feature_index
feature_id: feature-mcp-persistent-server
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-28
code_paths:
  - internal/mcp/response.go
  - internal/mcp/response_content.go
  - internal/mcp/protocol.go
  - internal/mcp/types.go
  - internal/mcp/deps.go
  - internal/types/log.go
  - internal/identity/mcp.go
  - cmd/browser-agent/internal/toolresp/rate_limiter.go
  - cmd/browser-agent/internal/toolresp/toolresp.go
  - cmd/browser-agent/internal/toolguard/guards.go
  - cmd/browser-agent/internal/toolrouting/routing.go
  - cmd/browser-agent/handler.go
  - cmd/browser-agent/main.go
  - cmd/browser-agent/config.go
  - cmd/browser-agent/tools_core.go
  - internal/session/snapshot-manager.go
  - cmd/browser-agent/internal/toolmodule/registry.go
  - cmd/browser-agent/server.go
  - cmd/browser-agent/internal/playbooks/resource_catalog.go
  - cmd/browser-agent/internal/playbooks/playbooks_resolver.go
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
  - cmd/browser-agent/internal/procctl/stop.go
  - cmd/browser-agent/main_connection_mcp.go
  - cmd/browser-agent/internal/procctl/pidfile.go
  - cmd/browser-agent/internal/procctl/port.go
  - cmd/browser-agent/internal/procctl/argv0.go
  - cmd/browser-agent/main_connection_recovery.go
  - cmd/browser-agent/internal/operationalapi/handler.go
  - cmd/browser-agent/internal/dashboard/handler.go
  - cmd/browser-agent/internal/dashboard/dashboard.html
  - cmd/browser-agent/internal/dashboard/diagnostics.html
  - cmd/browser-agent/internal/dashboard/logs.html
  - cmd/browser-agent/internal/dashboard/setup.html
  - cmd/browser-agent/internal/dashboard/docs.html
  - cmd/browser-agent/internal/logstore/store.go
  - cmd/browser-agent/internal/logstore/async.go
  - cmd/browser-agent/internal/logstore/accessors.go
  - cmd/browser-agent/internal/logstore/persistence.go
  - cmd/browser-agent/internal/logstore/validate.go
  - cmd/browser-agent/internal/logstore/seed.go
  - cmd/browser-agent/internal/exitdiag/recorder.go
  - cmd/browser-agent/internal/httpguard/middleware.go
  - cmd/browser-agent/internal/httpapi/response.go
  - cmd/browser-agent/internal/mcphttp/handler.go
  - cmd/browser-agent/internal/connectmode/runner.go
  - cmd/browser-agent/internal/versioncheck/checker.go
  - internal/diag/output.go
  - internal/diag/debug_file.go
  - cmd/browser-agent/internal/mediaapi/screenshots.go
  - internal/identity/mcp.go
  - internal/util/proc_unix.go
  - internal/util/proc_windows.go
test_paths:
  - cmd/browser-agent/internal/toolrouting/routing_test.go
  - internal/mcp/response_test.go
  - cmd/browser-agent/internal/toolresp/toolresp_test.go
  - cmd/browser-agent/tools_errors_test.go
  - cmd/browser-agent/health_unit_test.go
  - cmd/browser-agent/command_execution_readiness_test.go
  - cmd/browser-agent/handler_unit_test.go
  - cmd/browser-agent/handler_unit_telemetry_test.go
  - cmd/browser-agent/internal/procctl/stop_parse_test.go
  - cmd/browser-agent/internal/procctl/stop_test.go
  - cmd/browser-agent/main_flags_test.go
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
  - cmd/browser-agent/internal/mcphttp/handler_test.go
  - cmd/browser-agent/internal/httpguard/middleware_test.go
  - cmd/browser-agent/internal/connectmode/runner_test.go
  - cmd/browser-agent/internal/versioncheck/checker_test.go
  - scripts/check-bridge-stdout-invariant.sh
  - cmd/browser-agent/handler_consistency_test.go
  - cmd/browser-agent/server_routes_unit_test.go
  - cmd/browser-agent/internal/dashboard/branding_test.go
  - cmd/browser-agent/openapi_branding_test.go
  - cmd/browser-agent/internal/operationalapi/debug_test.go
  - cmd/browser-agent/internal/operationalapi/health_test.go
  - cmd/browser-agent/internal/dashboard/handler_test.go
  - cmd/browser-agent/internal/exitdiag/recorder_test.go
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

`internal/mcp/deps.go` contains only live diagnostic and asynchronous-command
contracts. Obsolete capture, log-buffer, accessibility, and noise provider
interfaces were removed after their consumers migrated to explicit composition.

> **2026-07-27:** Deleted the package-main type facade. MCP wire contracts and
> protocol negotiation now come directly from `internal/mcp`; server identity,
> annotations, and tool-call limiting come directly from their canonical owner
> packages. Root handlers no longer re-export these APIs.

> **2026-07-28:** Daemon lifecycle state, logs, PID files, and exit diagnostics
> resolve only through `internal/state`. Process control no longer reads,
> removes, or writes fallback filesystem locations.

> **2026-07-28:** Route construction now treats a missing capture runtime as an
> unavailable optional dependency. Guard diagnostics and injected query,
> performance, and session readers remain nil-safe, allowing middleware-only
> route tests without inventing a fake Capture.

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
