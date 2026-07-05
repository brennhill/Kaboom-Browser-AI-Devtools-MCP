---
doc_type: feature_index
feature_id: feature-bridge-restart
status: implemented
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - cmd/browser-agent/bridge.go
  - cmd/browser-agent/bridge_startup.go
  - cmd/browser-agent/bridge_startup_orchestration.go
  - cmd/browser-agent/bridge_startup_lock.go
  - cmd/browser-agent/bridge_startup_state.go
  - cmd/browser-agent/bridge_startup_status.go
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/tools_schema.go
test_paths:
  - cmd/browser-agent/bridge_test.go
  - cmd/browser-agent/bridge_spawn_race_test.go
  - cmd/browser-agent/bridge_startup_lock_test.go
  - cmd/browser-agent/bridge_startup_contention_test.go
  - cmd/browser-agent/bridge_faststart_extended_test.go
  - cmd/browser-agent/bridge_fastpath_unit_test.go
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
- Flow Map Pointer: [flow-map.md](./flow-map.md)
- Canonical Startup Flow: [Bridge Startup Contention and Convergence](../../../architecture/flow-maps/bridge-startup-contention-and-convergence.md)

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
| `cmd/browser-agent/tools_configure.go` | `toolConfigureRestart()` daemon-side handler |
| `cmd/browser-agent/tools_schema.go` | Schema: `restart` in configure action enum + oneOf |
| `cmd/browser-agent/bridge_test.go` | Unit tests for `extractToolAction()` |
| `cmd/browser-agent/bridge_startup_contention_test.go` | Multi-client startup convergence integration test |
| `cmd/browser-agent/bridge_fastpath_unit_test.go` | Fast-path + startup fallback regression tests (no indefinite wait on startup state drift) |
