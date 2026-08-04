---
doc_type: feature_index
feature_id: feature-mcp-persistent-server
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-04
code_paths:
  - internal/mcp/response.go
  - internal/mcp/response_content.go
  - internal/mcp/protocol.go
  - internal/mcp/types.go
  - cmd/browser-agent/internal/asynccommand/handler.go
  - internal/types/log.go
  - internal/identity/mcp.go
  - cmd/browser-agent/internal/toolresp/rate_limiter.go
  - cmd/browser-agent/internal/toolresp/toolresp.go
  - cmd/browser-agent/internal/toolguard/guards.go
  - cmd/browser-agent/internal/toolrouting/routing.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/handler.go
  - cmd/browser-agent/main.go
  - cmd/browser-agent/config.go
  - cmd/browser-agent/tools_core.go
  - internal/session/snapshot-manager.go
  - cmd/browser-agent/internal/toolmodule/registry.go
  - cmd/browser-agent/internal/toolcatalog/catalog.go
  - cmd/browser-agent/server.go
  - internal/incident/projections.go
  - cmd/browser-agent/internal/health/response_builders.go
  - cmd/browser-agent/internal/health/response_types.go
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
  - cmd/browser-agent/internal/daemonlife/deps.go
  - internal/statediag/collector.go
  - internal/statefault/fault.go
  - internal/statefile/statefile.go
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
  - cmd/browser-agent/noise_doctor_test.go
  - cmd/browser-agent/internal/daemonlife/startup_throttle_test.go
  - scripts/release/install-upgrade-regression.contract.test.mjs
  - scripts/release/install-upgrade-regression.mjs
  - tests/architecture/user-state-loaders.test.cjs
  - internal/statediag/collector_test.go
  - cmd/browser-agent/cli_modes_subprocess_test.go
  - cmd/browser-agent/main_connection_adapters_test.go
  - cmd/browser-agent/main_connection_recovery_primitives_test.go
  - cmd/browser-agent/connection_lifecycle_helpers_test.go
  - cmd/browser-agent/server_core_unit_test.go
  - cmd/browser-agent/internal/asynccommand/handler_test.go
  - cmd/browser-agent/server_telemetry_contract_test.go
  - internal/capture/health_reader_owner_test.go
  - cmd/browser-agent/internal/toolrouting/routing_test.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/tools_configure_handler_test.go
  - cmd/browser-agent/tools_configure_session_actions_test.go
  - internal/mcp/response_test.go
  - cmd/browser-agent/internal/toolresp/toolresp_test.go
  - cmd/browser-agent/tools_errors_test.go
  - cmd/browser-agent/health_unit_test.go
  - cmd/browser-agent/command_execution_readiness_test.go
  - cmd/browser-agent/handler_unit_test.go
  - cmd/browser-agent/handler_unit_telemetry_test.go
  - cmd/browser-agent/handler_tools_call_usage_test.go
  - cmd/browser-agent/internal/procctl/stop_parse_test.go
  - cmd/browser-agent/internal/procctl/stop_test.go
  - cmd/browser-agent/main_flags_test.go
  - cmd/browser-agent/test_daemon_cleanup_test.go
  - cmd/browser-agent/main_connection_pid_contract_test.go
  - internal/diag/output_test.go
  - internal/diag/debug_file_test.go
  - cmd/browser-agent/internal/playbooks/resource_catalog_test.go
  - cmd/browser-agent/internal/playbooks/playbooks_resolver_test.go
  - cmd/browser-agent/stdout_protocol_boundary_test.go
  - cmd/browser-agent/mcp_protocol_test.go
  - cmd/browser-agent/mcp_initialize_test.go
  - cmd/browser-agent/mcp_transport_handler_test.go
  - cmd/browser-agent/stdout_sync_unit_test.go
  - cmd/browser-agent/tools_registry_test.go
  - cmd/browser-agent/internal/toolcatalog/catalog_test.go
  - cmd/browser-agent/terminal_availability_test.go
  - cmd/browser-agent/reclaim_port_identity_test.go
  - cmd/browser-agent/reclaim_port_test.go
  - cmd/browser-agent/bridge_startup_contention_test.go
  - cmd/browser-agent/bridge_faststart_extended_test.go
  - cmd/browser-agent/start_timeout_norace_test.go
  - cmd/browser-agent/start_timeout_race_test.go
  - cmd/browser-agent/internal/mcphttp/handler_test.go
  - cmd/browser-agent/internal/httpguard/middleware_test.go
  - cmd/browser-agent/internal/connectmode/runner_test.go
  - cmd/browser-agent/internal/versioncheck/checker_test.go
  - scripts/check-bridge-stdout-invariant.sh
  - cmd/browser-agent/handler_consistency_test.go
  - cmd/browser-agent/lint_hardening_test.go
  - cmd/browser-agent/tools_core_sync_test.go
  - cmd/browser-agent/server_routes_unit_test.go
  - cmd/browser-agent/internal/dashboard/branding_test.go
  - cmd/browser-agent/openapi_branding_test.go
  - cmd/browser-agent/internal/operationalapi/debug_test.go
  - cmd/browser-agent/internal/operationalapi/health_test.go
  - cmd/browser-agent/internal/operationalapi/coverage_contract_test.go
  - cmd/browser-agent/internal/dashboard/handler_test.go
  - cmd/browser-agent/internal/exitdiag/recorder_test.go

  - cmd/browser-agent/internal/bridge/bridge_fastpath_unit_test.go
  - cmd/browser-agent/internal/bridge/bridge_detach_stdio_test.go
  - cmd/browser-agent/internal/bridge/bridge_detach_contract_test.go
  - cmd/browser-agent/internal/bridge/bridge_context_contract_test.go
  - cmd/browser-agent/internal/bridge/health_metadata_test.go
  - cmd/browser-agent/internal/bridge/runner_isolation_test.go
  - cmd/browser-agent/internal/bridge/stdioisolate/isolation_unix_test.go
  - cmd/browser-agent/internal/daemonlife/lifecycle_takeover_test.go
  - cmd/browser-agent/internal/daemonlife/lifecycle_policy_test.go
  - cmd/browser-agent/internal/daemonlife/install_epoch_test.go
  - cmd/browser-agent/internal/daemonlife/startup_throttle_test.go
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

Persisted restart timestamps are validated before they become incident
generations. Invalid negative values take the existing fail-open corruption
recovery path and are surfaced through local Doctor diagnostics.

The MCP initialize instructions explicitly designate Kaboom as the live-browser
control surface. Agents must recover Kaboom connectivity instead of silently
switching to Chrome DevTools, Playwright, or a sandboxed browser; another browser
tool is appropriate only when the user requests it or Kaboom reports a concrete
capability gap.

Fast-start compatibility tests use the canonical server startup budget
(5 seconds normally, 30 seconds under the race detector or coverage
instrumentation). This wider setup ceiling only absorbs instrumentation
overhead; post-start MCP resource latency remains independently enforced at
500 milliseconds.

The obsolete `internal/mcp/deps.go` provider contracts were deleted after all
consumers migrated to explicit owner functions. Asynchronous command lifecycle
behavior now belongs to `internal/asynccommand.Handler`; MCP transport owns no
capture, accessibility, log-buffer, noise, or completion provider interface.
HTTP MCP responses are marshaled once at the transport boundary and those exact
validated bytes are written to the client. Upstream serialization failures are
converted into a valid `-32603` JSON-RPC error with the original request ID, so
HTTP 200 can never carry an empty or partially encoded protocol response.
`MCPHandler` owns its capture, tool schemas, limiter, redactor, usage tracker,
and execution backend through one `ToolBackend` value. The executor contract has
only `HandleToolCall`; no transport-policy getter remains on `ToolHandler`.
The execution modules, examples, and validation schemas used by that call path
are constructed once in `internal/toolcatalog.Catalog`; the composition root
does not maintain parallel lazy registries.

> **2026-07-27:** Deleted the package-main type facade. MCP wire contracts and
> protocol negotiation now come directly from `internal/mcp`; server identity,
> annotations, and tool-call limiting come directly from their canonical owner
> packages. Root handlers no longer re-export these APIs.

> **2026-07-28:** Daemon lifecycle state, logs, PID files, and exit diagnostics
> resolve only through `internal/state`. Process control no longer reads,
> removes, or writes fallback filesystem locations.

> **2026-07-30:** Malformed daemon locks no longer block startup, and malformed
> restart history cannot trigger false throttling. Lock, restart-history, and
> install-epoch recovery use safe defaults, structured lifecycle events, and
> the canonical System Doctor recovery collector.

> **2026-08-03:** Daemon lock and restart-throttle persistence now share one
> canonical lifecycle filesystem boundary with durable atomic writes. Read,
> write, quota, cancellation, corruption, partial-write, cleanup, and restart
> tests use deterministic fakes; lifecycle logs expose stable reasons instead
> of paths or raw errors. Partial install-epoch stamps can no longer be accepted
> as plausible nanosecond identities.

> **2026-07-28:** Route construction now treats a missing capture runtime as an
> unavailable optional dependency. Guard diagnostics and injected query,
> performance, and session readers remain nil-safe, allowing middleware-only
> route tests without inventing a fake Capture.

> **2026-08-03:** Health and Doctor expose one machine-readable pressure
> contract for console, browser telemetry, extension diagnostics, performance
> snapshots, pending commands, alerts, notification queues, recordings, and
> the correlated Doctor timeline. Active obligations remain distinct from
> recoverable history, and historical drops do not falsely degrade health.

> **2026-08-04:** The canonical restart-history owner now classifies an
> unfinished matching daemon run as `unclean_daemon_exit` on the next launch.
> Clean shutdown, version upgrades, install-epoch changes, and other ports do
> not create incidents. Clean shutdown is durably marked before best-effort
> history cleanup, so a failed removal cannot masquerade as a crash. Packaged
> lifecycle UAT kills the daemon, restarts it,
> and requires the correlated terminal Doctor incident before passing.

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
