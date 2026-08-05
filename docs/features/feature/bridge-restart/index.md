---
doc_type: feature_index
feature_id: feature-bridge-restart
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-08-05
code_paths:
  - cmd/browser-agent/internal/bridge/runner.go
  - cmd/browser-agent/internal/bridge/bridge.go
  - cmd/browser-agent/internal/bridge/bridge_startup_state.go
  - cmd/browser-agent/internal/bridge/bridge_startup.go
  - cmd/browser-agent/internal/bridge/bridge_transport.go
  - cmd/browser-agent/internal/bridge/bridge_fingerprint.go
  - cmd/browser-agent/internal/bridge/stdioisolate/isolation.go
  - cmd/browser-agent/internal/bridge/stdioisolate/isolation_unix.go
  - cmd/browser-agent/internal/bridge/stdioisolate/isolation_windows.go
  - cmd/browser-agent/internal/bridge/stdioisolate/dup2_linux.go
  - cmd/browser-agent/internal/bridge/stdioisolate/dup2_unix_nonlinux.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - internal/schema/configure/tool.go
test_paths:
  - cmd/browser-agent/internal/bridge/bridge_unit_test.go
  - cmd/browser-agent/internal/bridge/runner_isolation_test.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - cmd/browser-agent/bridge_test.go
  - cmd/browser-agent/bridge_startup_contention_test.go
  - cmd/browser-agent/bridge_faststart_extended_test.go
  - cmd/browser-agent/internal/bridge/bridge_spawn_race_test.go
  - cmd/browser-agent/internal/bridge/lazy_server_start_test.go
  - cmd/browser-agent/internal/bridge/bridge_fastpath_unit_test.go
  - cmd/browser-agent/internal/bridge/bridge_respawn_backoff_test.go
  - cmd/browser-agent/internal/bridge/bridge_fingerprint_test.go
  - cmd/browser-agent/internal/bridge/stdioisolate/isolation_test.go
  - cmd/browser-agent/internal/bridge/stdioisolate/isolation_unix_test.go
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
| `cmd/browser-agent/bridge.go` | Startup-aware forwarding for `tools/call` during daemon warm-up |
| `cmd/browser-agent/bridge_startup_orchestration.go` | Startup coordinator: leader election, follower wait, stale-lock takeover |
| `cmd/browser-agent/bridge_startup_lock.go` | Lock-file startup leadership (`bridge-startup-<port>.lock.json`) |
| `cmd/browser-agent/bridge_startup_state.go` | Daemon readiness/failed signaling, bounded respawn peer-wait, and stale-wait leadership reclaim |
| `cmd/browser-agent/tools_configure.go` | `handleConfigureRestart()` daemon-side handler |
| `internal/schema/configure/tool.go` | Schema: `restart` in configure action enum + oneOf |
| `cmd/browser-agent/bridge_test.go` | Unit tests for `extractToolAction()` |
| `cmd/browser-agent/bridge_startup_contention_test.go` | Multi-client startup convergence integration test |
| `cmd/browser-agent/bridge_fastpath_unit_test.go` | Fast-path + startup fallback regression tests (no indefinite wait on startup state drift) |

Bridge transport tests synchronize delayed response bodies with a header/body
barrier, and startup-grace tests deliver readiness through the canonical signal
channel. They verify cancellation ownership and readiness consumption without
scheduler delays.
The bridge runner owns its retry delay as a private runtime dependency.
Peer-startup tests advance the first retry synchronously and start the fixture
server at that transition; server-shutdown tests await `Serve` completion rather
than sleeping before probing the closed listener.
