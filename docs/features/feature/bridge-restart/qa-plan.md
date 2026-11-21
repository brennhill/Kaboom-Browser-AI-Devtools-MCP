---
doc_type: qa-plan
feature_id: feature-bridge-restart
status: implemented
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Bridge Restart QA Plan

## Canonical Source
Detailed coverage remains in [test-plan.md](./test-plan.md); this file is the normalized feature-bundle QA entrypoint.

## Automated Coverage
- `cmd/browser-agent/internal/bridge/bridge_unit_test.go` (`TestExtractToolAction`, `checkDaemonStatus` startup grace)
- `cmd/browser-agent/internal/bridge/bridge_coverage_extra_test.go` (`forceKillOnPort`, `handleBridgeRestart`, respawn peer signals)
- `cmd/browser-agent/internal/bridge/startuplock/lock_test.go`
- `internal/bridge/timeout_test.go` (`TestExtractToolAction`)
- `cmd/browser-agent/internal/bridge/bridge_startup_test.go`
- `cmd/browser-agent/integration/bridge/startup_contention_test.go`
- `cmd/browser-agent/integration/bridge/faststart_extended_test.go`
- `cmd/browser-agent/internal/bridge/bridge_fastpath_unit_test.go`

## Required Scenarios
1. Responsive daemon restart path.
2. Frozen daemon recovery path.
3. Contention convergence: only one startup leader, followers connect in bounded time.
4. Restart action extraction robustness for malformed/non-configure payloads.
