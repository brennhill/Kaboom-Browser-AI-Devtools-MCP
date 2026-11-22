---
doc_type: feature_index
feature_id: feature-bridge-restart
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-09-05
code_paths:
  - internal/lifecycle/bridge/bridge.go
  - internal/lifecycle/bridge/bridge_startup_state.go
  - internal/lifecycle/bridge/bridge_startup.go
  - internal/lifecycle/bridge/bridge_transport.go
  - internal/lifecycle/bridge/processsignal/resume_unix.go
  - internal/lifecycle/bridge/processsignal/resume_windows.go
  - internal/lifecycle/bridge/fingerprint/fingerprint.go
  - internal/lifecycle/bridge/pushrelay/relay.go
  - internal/lifecycle/bridge/stdioisolate/isolation.go
  - internal/lifecycle/bridge/stdioisolate/isolation_unix.go
  - internal/lifecycle/bridge/stdioisolate/isolation_windows.go
  - internal/lifecycle/bridge/stdioisolate/dup2_linux.go
  - internal/lifecycle/bridge/stdioisolate/dup2_unix_nonlinux.go
  - cmd/browser-agent/internal/toolruntime/tools_configure.go
  - internal/tools/configure/handler/dispatcher.go
  - internal/tools/schema/configure/tool.go
test_paths:
  - internal/lifecycle/bridge/bridge_unit_test.go
  - internal/lifecycle/bridge/bridge_test_support_test.go
  - internal/tools/configure/handler/dispatcher_test.go
  - cmd/browser-agent/integration/bridge/startup_contention_test.go
  - cmd/browser-agent/integration/bridge/faststart_extended_test.go
  - internal/lifecycle/bridge/bridge_startup_test.go
  - internal/lifecycle/bridge/bridge_fastpath_unit_test.go
  - internal/lifecycle/bridge/processsignal/resume_test.go
  - internal/lifecycle/bridge/fingerprint/fingerprint_test.go
  - internal/lifecycle/bridge/pushrelay/relay_test.go
  - internal/lifecycle/bridge/stdioisolate/isolation_test.go
  - internal/lifecycle/bridge/stdioisolate/isolation_unix_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Bridge Restart

## TL;DR

- Status: implemented
- Tool: `configure`
- Action: `restart`
- Location: `docs/features/feature/bridge-restart`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- Test Plan: [test-plan.md](./test-plan.md)

## Requirement IDs

- FEATURE_BRIDGE_RESTART_001: Force-restart daemon from bridge when unresponsive
- FEATURE_BRIDGE_RESTART_002: Daemon-side restart via self-SIGTERM when responsive
- FEATURE_BRIDGE_RESTART_003: Recovery from frozen (SIGSTOP'd) daemon processes

## Code and Tests

| File | Purpose |
|------|---------|
| `internal/lifecycle/bridge/bridge.go` | Startup-aware forwarding for `tools/call` during daemon warm-up |
| `internal/lifecycle/bridge/bridge_startup.go` | Startup coordinator, leadership, follower wait, and stale-lock takeover |
| `internal/lifecycle/bridge/bridge_startup_state.go` | Daemon readiness/failed signaling, bounded respawn peer-wait, and stale-wait leadership reclaim |
| `internal/lifecycle/bridge/processsignal/` | Platform-specific suspended-daemon resume signaling |
| `internal/lifecycle/bridge/fingerprint/fingerprint.go` | Immutable executable identity used by launch diagnostics |
| `cmd/browser-agent/tools_configure.go` | `handleConfigureRestart()` daemon-side handler |
| `internal/tools/schema/configure/tool.go` | Schema: `restart` in configure action enum + oneOf |
| `internal/lifecycle/bridge/bridge_unit_test.go` | Unit tests for bridge request routing |
| `internal/lifecycle/bridge/bridge_startup_test.go` | Startup, respawn, and multi-client convergence tests |
| `internal/lifecycle/bridge/bridge_fastpath_unit_test.go` | Fast-path + startup fallback regression tests (no indefinite wait on startup state drift) |
| `internal/lifecycle/bridge/fingerprint/fingerprint_test.go` | Binary identity and diagnostic error-path tests |

Bridge transport tests synchronize delayed response bodies with a header/body
barrier, and startup-grace tests deliver readiness through the canonical signal
channel. They verify cancellation ownership and readiness consumption without
scheduler delays.
The bridge runner owns its retry delay as a private runtime dependency.
Peer-startup tests advance the first retry synchronously and start the fixture
server at that transition; server-shutdown tests await `Serve` completion rather
than sleeping before probing the closed listener.
