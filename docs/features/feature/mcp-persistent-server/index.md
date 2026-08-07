---
doc_type: feature_index
feature_id: feature-mcp-persistent-server
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - internal/listenport/store.go
  - cmd/browser-agent/internal/runtimeflags/flags.go
  - internal/serverdefaults/defaults.go
  - internal/warningqueue/queue.go
  - cmd/browser-agent/internal/httpapi/openapi.go
  - internal/mcp/response.go
  - internal/mcp/response_content.go
  - internal/mcp/protocol.go
  - internal/mcp/types.go
  - cmd/browser-agent/internal/asynccommand/handler.go
  - internal/types/wire_log.go
  - internal/identity/mcp.go
  - cmd/browser-agent/internal/toolresp/rate_limiter.go
  - cmd/browser-agent/internal/toolresp/toolresp.go
  - cmd/browser-agent/internal/toolguard/guards.go
  - cmd/browser-agent/internal/toolrouting/routing.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/internal/mcpendpoint/handler.go
  - cmd/browser-agent/internal/appruntime/runtime.go
  - cmd/browser-agent/main.go
  - cmd/browser-agent/config.go
  - cmd/browser-agent/internal/startupconfig/paths.go
  - cmd/browser-agent/internal/startupconfig/runtime.go
  - cmd/browser-agent/internal/runtimeconfig/parallel.go
  - cmd/browser-agent/tools_core.go
  - cmd/browser-agent/internal/toolusage/key.go
  - cmd/browser-agent/internal/toolpostprocess/postprocess.go
  - internal/session/snapshot-manager.go
  - cmd/browser-agent/internal/toolmodule/registry.go
  - cmd/browser-agent/internal/toolcatalog/catalog.go
  - cmd/browser-agent/server.go
  - internal/incident/projections.go
  - cmd/browser-agent/internal/health/response_builders.go
  - cmd/browser-agent/internal/health/response_types.go
  - cmd/browser-agent/internal/health/doctor_live_checks.go
  - cmd/browser-agent/internal/playbooks/resources/catalog.go
  - cmd/browser-agent/internal/playbooks/resources/resolver.go
  - cmd/browser-agent/internal/playbooks/resources/audits.go
  - cmd/browser-agent/internal/bridge/bridge.go
  - cmd/browser-agent/internal/bridge/bridge_startup.go
  - cmd/browser-agent/internal/bridge/healthprobe/probe.go
  - cmd/browser-agent/internal/bridge/daemoncmd/command.go
  - cmd/browser-agent/internal/bridge/bridge_transport.go
  - cmd/browser-agent/internal/bridge/stdioisolate/isolation.go
  - cmd/browser-agent/internal/daemonlife/lifecycle.go
  - cmd/browser-agent/internal/daemonlife/lock_file.go
  - cmd/browser-agent/internal/daemonlife/install_epoch.go
  - cmd/browser-agent/internal/daemonlife/startup_throttle.go
  - cmd/browser-agent/internal/daemonlife/deps.go
  - cmd/browser-agent/internal/daemonrecovery/primitives.go
  - internal/statediag/collector.go
  - internal/statefault/fault.go
  - internal/statefile/statefile.go
  - cmd/browser-agent/internal/daemonrecovery/reclaimer.go
  - cmd/browser-agent/internal/daemonhttp/server.go
  - cmd/browser-agent/internal/integrationtest/harness.go
  - cmd/browser-agent/internal/procctl/stop.go
  - cmd/browser-agent/main_connection_mcp.go
  - cmd/browser-agent/internal/procctl/pidfile.go
  - cmd/browser-agent/internal/procctl/port.go
  - cmd/browser-agent/internal/procctl/argv0.go
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
  - cmd/browser-agent/internal/mcpcall/handler.go
  - cmd/browser-agent/internal/mcprouter/router.go
  - cmd/browser-agent/internal/mcpprotocol/responses.go
  - cmd/browser-agent/internal/mcptelemetry/owner.go
  - cmd/browser-agent/internal/mcpresponse/owner.go
  - cmd/browser-agent/internal/connectmode/runner.go
  - cmd/browser-agent/internal/versioncheck/checker.go
  - internal/diag/output.go
  - internal/diag/debug_file.go
  - cmd/browser-agent/internal/mediaapi/screenshots.go
  - internal/identity/mcp.go
  - internal/util/proc_unix.go
  - internal/util/proc_windows.go
test_paths:
  - cmd/browser-agent/internal/toolobserve/dispatcher_commands_test.go
  - cmd/browser-agent/internal/daemonrecovery/primitives_test.go
  - cmd/browser-agent/internal/daemonrecovery/reclaimer_test.go
  - cmd/browser-agent/internal/daemonhttp/server_test.go
  - cmd/browser-agent/internal/integrationtest/harness_test.go
  - cmd/browser-agent/internal/daemonlife/helpers_test.go
  - internal/listenport/store_test.go
  - internal/warningqueue/queue_test.go
  - cmd/browser-agent/internal/httpapi/openapi_test.go
  - cmd/browser-agent/internal/runtimeconfig/parallel_test.go
  - cmd/browser-agent/internal/doctorsupport/projections_test.go
  - cmd/browser-agent/internal/daemonlife/startup_throttle_test.go
  - scripts/release/install-upgrade-regression.contract.test.mjs
  - scripts/release/install-upgrade-regression.mjs
  - tests/architecture/user-state-loaders.test.cjs
  - internal/statediag/collector_test.go
  - cmd/browser-agent/integration/cli/modes_test.go
  - cmd/browser-agent/integration/runtime/persistence_test.go
  - cmd/browser-agent/integration/runtime/reliability_test.go
  - cmd/browser-agent/integration/runtime/reliability_lifecycle_test.go
  - cmd/browser-agent/connection_lifecycle_helpers_test.go
  - cmd/browser-agent/internal/startupconfig/paths_test.go
  - cmd/browser-agent/internal/startupconfig/runtime_test.go
  - cmd/browser-agent/internal/asynccommand/handler_test.go
  - internal/capture/healthreader/reader_test.go
  - cmd/browser-agent/internal/toolrouting/routing_test.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - internal/tools/configure/boundaries_test.go
  - internal/mcp/response_test.go
  - cmd/browser-agent/internal/toolresp/toolresp_test.go
  - internal/mcp/errors_test.go
  - cmd/browser-agent/internal/toolpostprocess/postprocess_test.go
  - cmd/browser-agent/internal/toolguard/guards_test.go
  - cmd/browser-agent/internal/health/health_test.go
  - cmd/browser-agent/internal/mcpresponse/owner_test.go
  - cmd/browser-agent/internal/mcpendpoint/handler_test.go
  - cmd/browser-agent/internal/appruntime/runtime_test.go
  - cmd/browser-agent/internal/mcptelemetry/owner_test.go
  - cmd/browser-agent/internal/toolusage/key_test.go
  - cmd/browser-agent/tools_core_unit_test.go
  - cmd/browser-agent/internal/procctl/stop_parse_test.go
  - cmd/browser-agent/internal/procctl/stop_test.go
  - cmd/browser-agent/internal/runtimeflags/parsing_test.go
  - cmd/browser-agent/internal/runtimeflags/repeatable_test.go
  - cmd/browser-agent/test_daemon_cleanup_test.go
  - internal/diag/output_test.go
  - internal/diag/debug_file_test.go
  - cmd/browser-agent/internal/playbooks/resources/catalog_test.go
  - cmd/browser-agent/internal/playbooks/resources/resolver_test.go
  - cmd/browser-agent/internal/playbooks/resources/content_test.go
  - scripts/contracts/stdout_protocol_test.go
  - cmd/browser-agent/integration/bridge/stdio_silence_test.go
  - scripts/uat/protocol/smoke-mcp-transport.sh
  - cmd/browser-agent/internal/mcpprotocol/responses_test.go
  - cmd/browser-agent/internal/bridge/bridge_unit_test.go
  - cmd/browser-agent/internal/toolmodule/registry_test.go
  - cmd/browser-agent/internal/toolcatalog/catalog_test.go
  - cmd/browser-agent/internal/terminal/status/status_test.go
  - cmd/browser-agent/integration/bridge/startup_contention_test.go
  - cmd/browser-agent/integration/bridge/faststart_extended_test.go
  - cmd/browser-agent/start_timeout_norace_test.go
  - cmd/browser-agent/start_timeout_race_test.go
  - cmd/browser-agent/internal/mcphttp/handler_test.go
  - cmd/browser-agent/internal/mcpcall/handler_test.go
  - cmd/browser-agent/internal/mcprouter/router_test.go
  - cmd/browser-agent/internal/httpguard/middleware_test.go
  - cmd/browser-agent/internal/connectmode/runner_test.go
  - cmd/browser-agent/internal/versioncheck/checker_test.go
  - scripts/quality/contracts/check-bridge-stdout-invariant.sh
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - cmd/browser-agent/server_routes_unit_test.go
  - cmd/browser-agent/internal/dashboard/branding_test.go
  - scripts/contracts/openapibranding/branding_test.go
  - cmd/browser-agent/internal/operationalapi/debug_test.go
  - cmd/browser-agent/internal/operationalapi/health_test.go
  - cmd/browser-agent/internal/operationalapi/coverage_contract_test.go
  - cmd/browser-agent/internal/dashboard/handler_test.go
  - cmd/browser-agent/internal/exitdiag/recorder_test.go

  - cmd/browser-agent/internal/bridge/bridge_fastpath_unit_test.go
  - cmd/browser-agent/internal/bridge/daemoncmd/command_test.go
  - cmd/browser-agent/internal/bridge/contracts/source_contract_test.go
  - cmd/browser-agent/internal/bridge/healthprobe/probe_test.go
  - cmd/browser-agent/internal/bridge/bridge_test_support_test.go
  - cmd/browser-agent/internal/bridge/stdioisolate/isolation_unix_test.go
  - cmd/browser-agent/internal/daemonlife/lifecycle_takeover_test.go
  - cmd/browser-agent/internal/daemonlife/lifecycle_policy_test.go
  - cmd/browser-agent/internal/daemonlife/install_epoch_test.go
  - cmd/browser-agent/internal/daemonlife/startup_throttle_test.go
  - cmd/browser-agent/internal/daemonlife/helpers_test.go
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

The canonical `internal/mcpcall` owner contains the complete `tools/call`
pipeline: parameter validation, unknown-argument diagnostics, rate limiting,
five-tool execution, response redaction, response policy, and passive usage
telemetry. The root MCP handler only routes requests and composes this owner;
it defines no duplicate backend contracts or post-processing facade.

The adjacent `internal/mcprouter` owner validates JSON-RPC envelopes, handles
notification semantics, routes protocol and static methods, and clamps dynamic
results. Its configuration exposes only immutable protocol data and one
tool-call callback, keeping transport routing independent of application state.
HTTP notification/framing assertions live with `internal/mcphttp`, while bridge
stdout-forwarding contracts live with the bridge transport they protect; no
mixed root transport test duplicates these owners.
Existing-daemon adoption is likewise tested by `internal/bridge`, while the
real-process exit wait is local to the stdio integration suite. Test-only
process-global helpers are not shared across unrelated feature suites.
Security-mode response warning and metadata coverage likewise lives directly
with `internal/mcpresponse`, independent of full tool-handler composition.
Passive delta/mode coverage lives with `internal/mcptelemetry`; envelope and
negotiation coverage lives with `internal/mcprouter`; malformed input,
content-type, read-failure, and framing coverage lives with `internal/mcphttp`.
Stateless resource and initialization contracts live with `internal/mcpprotocol`,
and warning queues, schema-argument diagnostics, redaction, and rate limiting
live with `internal/mcpresponse` and `internal/mcpcall`.

MCP documentation resources are owned by the canonical
`internal/playbooks/resources` package: catalog, URI resolution, guides,
demos, automation, and audit playbooks evolve together without depending on
interact-failure recovery. Accessibility, performance, and security share one
audit-content owner, while the parent `playbooks` package contains only the
structured interact recovery contract. Both packages enforce the ten-file
boundary directly and callers import the resource owner without a facade.

Each server owns an application runtime for its start epoch, release checker,
binary-upgrade provider, exit diagnostics, bridge runner, and update-warning cooldown. These collaborators are
never shared between server instances, so parallel tests and multiple composed
runtimes cannot suppress or overwrite one another's lifecycle state.
Validated upload policy, OS-automation permission, and startup warnings flow
from parsed configuration into the target server instance. Flag parsing no
longer mutates package state later consumed by unrelated server instances.
Daemon process discovery, liveness, shutdown, termination, and port-release
collaborators are owned by each server. Recovery-policy tests inject one
server's host boundary and can run without swapping process-wide function
variables or affecting concurrent instances.
Doctor CLI discovery and authentication probes use an explicit command runtime.
Tests inject per-call lookup and subprocess functions, so provider diagnostics
are deterministic and cannot race through mutable health-package globals.

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
The canonical `mcpendpoint.Handler` owns JSON-RPC routing and response policy
through one explicit backend. `ToolHandler` is the composed runtime returned to
the server directly; startup, health, telemetry, and shutdown never recover it
through a backend type assertion. The executor contract has only
`HandleToolCall`; no transport-policy getter remains on `ToolHandler`.
Parsed flags are converted into a validated `startupconfig.Runtime` without
process exits. Root startup retains only explicit early-exit and launch policy;
port validation precedes all network/process modes, while path, parallel-state,
log fallback, and upload-boundary failures are owner-tested.
The source root contains no compiled executables. A deterministic architecture
gate rejects Mach-O, ELF, and PE signatures there, preventing local build
artifacts from silently inflating releases or introducing platform-specific
repository state.
Terminal intent routes receive explicit live relay/store callbacks from root
composition. The obsolete server adapter interface and its root-only test were
deleted; missing runtime resources remain a typed service-unavailable state.

Stdio isolation tests exercise the built bridge rather than the Go test binary.
They close stdin and await the bridge's process-exit barrier, so transport
purity is proven without wall-clock sleeps or forced termination. The focused
smoke runner explicitly enables the integration build tag and therefore cannot
silently report success after running zero transport tests.
Health-metric uptime tests position the private start timestamp directly and
assert the resulting duration; they do not wait for wall-clock time to pass.
Synchronous MCP completion tests establish connection first, then deliver the
result without assuming whether completion or waiter subscription wins.
QA fixture command tests await the canonical pending-query notification before
returning the simulated extension result; they do not poll for enqueue.
Subprocess-mode tests use the bridge readiness helper instead of a duplicate
poll loop. Test-state cleanup retries yield only between failed filesystem
operations, and suite-level daemon cleanup uses definitive termination rather
than a fixed grace sleep.
Release persistence and reliability suites express genuine observation windows
with tickers or bounded timers and fixed check counts; scheduler sleeps no
longer coordinate state. Upgrade coverage waits for the old child process to
exit before binding its replacement. The permanently skipped goroutine-leak
test was deleted because it measured the test process, not the daemon it claimed
to validate.
The asynchronous log worker captures the exact store created during server
construction. Replacing a server's store in an isolated test cannot redirect a
late-starting worker or race with its startup closure.
Log destination creation, writability fallback, persistence disablement, and
existing-entry loading are one `internal/logstore` startup operation. Its fault
fixture uses an invalid parent shape rather than platform permission behavior;
the server composition root only supplies the local warning sink.
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

> **2026-08-07:** Passive MCP telemetry metadata now has one bounded,
> per-client owner in `internal/mcptelemetry`. The handler supplies aggregate
> sources during composition and delegates augmentation directly; cursor
> synchronization, mode parsing, delta calculation, and expiry no longer live
> in the root handler or expose mutable test seams.

> **2026-08-07:** Stateless MCP initialization, tool catalog, bundled-resource,
> and resource-template responses now live in `internal/mcpprotocol`. The same
> canonical instructions feed initialize responses and bridge identity, and
> response encoding failures return a redacted JSON-RPC internal error instead
> of being silently discarded.

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
