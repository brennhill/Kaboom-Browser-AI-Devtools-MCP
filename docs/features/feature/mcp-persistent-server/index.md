---
doc_type: feature_index
feature_id: feature-mcp-persistent-server
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-11-23
code_paths:
  - internal/lifecycle/listenport/store.go
  - internal/lifecycle/runtimeflags/flags.go
  - internal/platform/defaults/defaults.go
  - internal/platform/warningqueue/queue.go
  - internal/composition/http/openapi/openapi.go
  - internal/tools/mcp/response.go
  - internal/tools/mcp/response_content.go
  - internal/tools/mcp/protocol.go
  - internal/tools/mcp/types.go
  - internal/tools/asynccommand/handler.go
  - internal/platform/types/wire_log.go
  - internal/platform/identity/mcp.go
  - internal/tools/response/rate_limiter.go
  - internal/tools/response/toolresp.go
  - internal/tools/guard/guards.go
  - internal/tools/routing/routing.go
  - internal/tools/configure/handler/dispatcher.go
  - internal/tools/mcpserver/endpoint/handler.go
  - internal/composition/appruntime/runtime.go
  - cmd/browser-agent/main.go
  - cmd/browser-agent/config.go
  - internal/lifecycle/startupconfig/paths.go
  - internal/lifecycle/startupconfig/runtime.go
  - internal/lifecycle/runtimeconfig/parallel.go
  - internal/composition/toolruntime/tools_core.go
  - internal/composition/toolruntime/tools_configure.go
  - internal/composition/toolruntime/tools_interact_dispatch.go
  - internal/composition/toolruntime/server_state.go
  - internal/composition/toolruntime/modes.go
  - internal/tools/usage/key.go
  - internal/tools/postprocess/postprocess.go
  - internal/analysis/snapshots/snapshot-manager.go
  - internal/tools/module/registry.go
  - internal/tools/catalog/catalog.go
  - cmd/browser-agent/server.go
  - internal/lifecycle/doctor/incident/projections.go
  - internal/tools/commandhealth/response_builders.go
  - internal/tools/commandhealth/response_types.go
  - internal/tools/commandhealth/doctor_live_checks.go
  - internal/tools/playbooks/resources/catalog.go
  - internal/tools/playbooks/resources/resolver.go
  - internal/tools/playbooks/resources/audits.go
  - internal/lifecycle/bridge/bridge.go
  - internal/lifecycle/bridge/bridge_startup.go
  - internal/lifecycle/bridge/healthprobe/probe.go
  - internal/lifecycle/bridge/daemoncmd/command.go
  - internal/lifecycle/bridge/bridge_transport.go
  - internal/lifecycle/bridge/stdioisolate/isolation.go
  - internal/lifecycle/daemon/lifecycle.go
  - internal/lifecycle/process/lock/proclock.go
  - internal/lifecycle/instance/gov/runtime.go
  - internal/lifecycle/instance/gov/instancegov.go
  - internal/lifecycle/instance/registry/registry.go
  - internal/lifecycle/daemon/install_epoch.go
  - internal/lifecycle/daemon/startup_throttle.go
  - internal/lifecycle/daemon/deps.go
  - internal/lifecycle/daemon/recovery/primitives.go
  - internal/lifecycle/doctor/statediag/collector.go
  - internal/platform/state/fault/fault.go
  - internal/platform/state/file/statefile.go
  - internal/lifecycle/daemon/recovery/reclaimer.go
  - internal/composition/http/listener/server.go
  - internal/composition/testsupport/integration/harness.go
  - internal/lifecycle/procctl/stop.go
  - cmd/browser-agent/main_connection_mcp.go
  - internal/lifecycle/procctl/pidfile.go
  - internal/lifecycle/procctl/port.go
  - internal/lifecycle/procctl/argv0.go
  - internal/composition/http/operational/handler.go
  - internal/composition/http/dashboard/handler.go
  - internal/composition/http/dashboard/dashboard.html
  - internal/composition/http/dashboard/diagnostics.html
  - internal/composition/http/dashboard/logs.html
  - internal/composition/http/dashboard/setup.html
  - internal/composition/http/dashboard/docs.html
  - internal/observation/daemonlog/store.go
  - internal/observation/daemonlog/async.go
  - internal/observation/daemonlog/accessors.go
  - internal/observation/daemonlog/persistence.go
  - internal/observation/daemonlog/validate.go
  - internal/observation/daemonlog/seed.go
  - internal/lifecycle/exitdiag/recorder.go
  - internal/platform/httpguard/middleware.go
  - internal/composition/http/openapi/response.go
  - internal/tools/mcpserver/http/handler.go
  - internal/tools/mcpserver/call/handler.go
  - internal/tools/mcpserver/router/router.go
  - internal/tools/mcpserver/protocol/responses.go
  - internal/observation/adapters/mcptelemetry/owner.go
  - internal/tools/mcpserver/response/owner.go
  - internal/lifecycle/connectmode/runner.go
  - internal/lifecycle/upgrade/versioncheck/checker.go
  - internal/platform/diag/output.go
  - internal/platform/diag/debug_file.go
  - internal/annotation/adapters/http/screenshots.go
  - internal/platform/identity/mcp.go
  - internal/lifecycle/process/detach/detach_unix.go
  - internal/lifecycle/process/detach/detach_windows.go
test_paths:
  - internal/composition/toolruntime/composition_test.go
  - internal/tools/observe/handler/dispatcher_commands_test.go
  - internal/lifecycle/daemon/recovery/primitives_test.go
  - internal/lifecycle/daemon/recovery/reclaimer_test.go
  - internal/composition/http/listener/server_test.go
  - internal/composition/testsupport/integration/harness_test.go
  - internal/lifecycle/daemon/helpers_test.go
  - internal/lifecycle/listenport/store_test.go
  - internal/platform/warningqueue/queue_test.go
  - internal/composition/http/openapi/openapi_test.go
  - internal/lifecycle/runtimeconfig/parallel_test.go
  - internal/lifecycle/doctor/support/projections_test.go
  - internal/lifecycle/daemon/startup_throttle_test.go
  - scripts/release/install-upgrade-regression.contract.test.mjs
  - scripts/release/install-upgrade-regression.mjs
  - tests/architecture/user-state-loaders.test.cjs
  - internal/lifecycle/doctor/statediag/collector_test.go
  - cmd/browser-agent/integration/cli/modes_test.go
  - cmd/browser-agent/integration/runtime/persistence_test.go
  - cmd/browser-agent/integration/runtime/reliability_test.go
  - cmd/browser-agent/integration/runtime/reliability_lifecycle_test.go
  - internal/lifecycle/startupconfig/paths_test.go
  - internal/lifecycle/startupconfig/runtime_test.go
  - internal/tools/asynccommand/handler_test.go
  - internal/observation/healthreader/reader_test.go
  - internal/tools/routing/routing_test.go
  - internal/tools/configure/handler/dispatcher_test.go
  - internal/tools/configure/boundaries_test.go
  - internal/tools/mcp/response_test.go
  - internal/tools/response/toolresp_test.go
  - internal/tools/mcp/errors_test.go
  - internal/tools/postprocess/postprocess_test.go
  - internal/tools/guard/guards_test.go
  - internal/tools/commandhealth/health_test.go
  - internal/tools/mcpserver/response/owner_test.go
  - internal/tools/mcpserver/endpoint/handler_test.go
  - internal/composition/appruntime/runtime_test.go
  - internal/observation/adapters/mcptelemetry/owner_test.go
  - internal/tools/usage/key_test.go
  - cmd/browser-agent/composition_test.go
  - internal/lifecycle/procctl/stop_parse_test.go
  - internal/lifecycle/procctl/stop_test.go
  - internal/lifecycle/runtimeflags/parsing_test.go
  - internal/lifecycle/runtimeflags/repeatable_test.go
  - internal/platform/diag/output_test.go
  - internal/platform/diag/debug_file_test.go
  - internal/tools/playbooks/resources/catalog_test.go
  - internal/tools/playbooks/resources/resolver_test.go
  - internal/tools/playbooks/resources/content_test.go
  - scripts/contracts/stdout_protocol_test.go
  - cmd/browser-agent/integration/bridge/stdio_silence_test.go
  - scripts/uat/protocol/smoke-mcp-transport.sh
  - internal/tools/mcpserver/protocol/responses_test.go
  - internal/lifecycle/bridge/bridge_unit_test.go
  - internal/tools/module/registry_test.go
  - internal/tools/catalog/catalog_test.go
  - internal/terminal/adapters/status/status_test.go
  - cmd/browser-agent/integration/bridge/startup_contention_test.go
  - cmd/browser-agent/integration/bridge/faststart_extended_test.go
  - internal/tools/mcpserver/http/handler_test.go
  - internal/tools/mcpserver/call/handler_test.go
  - internal/tools/mcpserver/router/router_test.go
  - internal/platform/httpguard/middleware_test.go
  - internal/lifecycle/connectmode/runner_test.go
  - internal/lifecycle/upgrade/versioncheck/checker_test.go
  - scripts/quality/contracts/check-bridge-stdout-invariant.sh
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - internal/composition/http/dashboard/branding_test.go
  - scripts/contracts/openapibranding/branding_test.go
  - internal/composition/http/operational/debug_test.go
  - internal/composition/http/operational/health_test.go
  - internal/composition/http/operational/coverage_contract_test.go
  - internal/composition/http/dashboard/handler_test.go
  - internal/lifecycle/exitdiag/recorder_test.go

  - internal/lifecycle/bridge/bridge_fastpath_unit_test.go
  - internal/lifecycle/bridge/daemoncmd/command_test.go
  - internal/lifecycle/bridge/contracts/source_contract_test.go
  - internal/lifecycle/bridge/healthprobe/probe_test.go
  - internal/lifecycle/bridge/bridge_test_support_test.go
  - internal/lifecycle/bridge/stdioisolate/isolation_unix_test.go
  - internal/lifecycle/process/lock/proclock_test.go
  - internal/lifecycle/instance/gov/instancegov_test.go
  - internal/lifecycle/instance/gov/policy_test.go
  - internal/lifecycle/daemon/install_epoch_test.go
  - internal/lifecycle/daemon/startup_throttle_test.go
  - internal/lifecycle/daemon/helpers_test.go
  - internal/lifecycle/procctl/pidfile_test.go
  - internal/lifecycle/procctl/port_test.go
  - internal/lifecycle/procctl/port_netstat_test.go
  - internal/lifecycle/procctl/argv0_test.go
  - internal/observation/daemonlog/store_test.go
  - internal/observation/daemonlog/async_test.go
  - internal/observation/daemonlog/validate_test.go
  - tests/regression/08-fast-start/test-fast-start.sh
last_verified_version: 0.8.1
last_verified_date: 2026-03-29
---

# MCP Persistent Server

The canonical `internal/tools/mcpserver/call` owner contains the complete `tools/call`
pipeline: parameter validation, unknown-argument diagnostics, rate limiting,
five-tool execution, response redaction, response policy, and passive usage
telemetry. The root MCP handler only routes requests and composes this owner;
it defines no duplicate backend contracts or post-processing facade.

The adjacent `internal/tools/mcpserver/router` owner validates JSON-RPC envelopes, handles
notification semantics, routes protocol and static methods, and clamps dynamic
results. Its configuration exposes only immutable protocol data and one
tool-call callback, keeping transport routing independent of application state.
HTTP notification/framing assertions live with `internal/tools/mcpserver/http`, while bridge
stdout-forwarding contracts live with the bridge transport they protect; no
mixed root transport test duplicates these owners.
Existing-daemon adoption is likewise tested by `internal/lifecycle/bridge/framing`, while the
real-process exit wait is local to the stdio integration suite. Test-only
process-global helpers are not shared across unrelated feature suites.
Security-mode response warning and metadata coverage likewise lives directly
with `internal/tools/mcpserver/response`, independent of full tool-handler composition.
Passive delta/mode coverage lives with `internal/observation/adapters/mcptelemetry`; envelope and
negotiation coverage lives with `internal/tools/mcpserver/router`; malformed input,
content-type, read-failure, and framing coverage lives with `internal/tools/mcpserver/http`.
Stateless resource and initialization contracts live with `internal/tools/mcpserver/protocol`,
and warning queues, schema-argument diagnostics, redaction, and rate limiting
live with `internal/tools/mcpserver/response` and `internal/tools/mcpserver/call`.

MCP documentation resources are owned by the canonical
`internal/tools/playbooks/resources` package: catalog, URI resolution, guides,
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

The obsolete `internal/tools/mcp/deps.go` provider contracts were deleted after all
consumers migrated to explicit owner functions. Asynchronous command lifecycle
behavior now belongs to `internal/tools/asynccommand.Handler`; MCP transport owns no
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
are constructed once in `internal/tools/catalog.Catalog`; the composition root
does not maintain parallel lazy registries.

> **2026-07-27:** Deleted the package-main type facade. MCP wire contracts and
> protocol negotiation now come directly from `internal/tools/mcp`; server identity,
> annotations, and tool-call limiting come directly from their canonical owner
> packages. Root handlers no longer re-export these APIs.

> **2026-07-28:** Daemon lifecycle state, logs, PID files, and exit diagnostics
> resolve only through `internal/platform/state/paths`. Process control no longer reads,
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
> per-client owner in `internal/observation/adapters/mcptelemetry`. The handler supplies aggregate
> sources during composition and delegates augmentation directly; cursor
> synchronization, mode parsing, delta calculation, and expiry no longer live
> in the root handler or expose mutable test seams.

> **2026-08-07:** Stateless MCP initialization, tool catalog, bundled-resource,
> and resource-template responses now live in `internal/tools/mcpserver/protocol`. The same
> canonical instructions feed initialize responses and bridge identity, and
> response encoding failures return a redacted JSON-RPC internal error instead
> of being silently discarded.

> **2026-07-27:** Removed the unreachable `main_connection_diag.go`
> connection-probing path and its dedicated tests. No production caller invoked
> it. Two unrelated stop-mode tests in the same file were also obsolete because
> they asserted human diagnostics on protocol stdout.

## Where the tool runtime lives

The five-tool dispatcher is `internal/composition/toolruntime`, not package
`main`. It receives its dependencies as a `ServerState` value — logs, warnings,
incidents, push inbox, active codebase, annotation runtime, listen port, upload
security, terminal status, intent store, and state recovery — which `*Server`
fills in via `toolRuntimeState()`. There is no interface between them and no
forwarding shim in `main`: the move was atomic, and `NewToolHandler` is the only
constructor.

The consequence is testability. While `ToolHandler` lived in `main`, nothing
outside `cmd/browser-agent` could construct it, so the response-contract sweep
could only reach `observe`. It now drives all five dispatchers through the same
`HandleToolCall` an MCP client reaches, with no daemon and no browser.

## TL;DR
- Status: shipped
- Scope: long-lived daemon lifecycle across client reconnects

## Specs
- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Related Architecture
