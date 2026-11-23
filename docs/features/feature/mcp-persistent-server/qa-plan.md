---
doc_type: qa-plan
feature_id: feature-mcp-persistent-server
status: shipped
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# MCP Persistent Server QA Plan

## Automated Coverage
- `cmd/browser-agent/integration/bridge/startup_contention_test.go`
- `cmd/browser-agent/integration/bridge/faststart_extended_test.go`
- `internal/lifecycle/bridge/bridge_startup_test.go` (single-leader election, stale-lock reclaim, respawn)
- `internal/lifecycle/bridge/startuplock/lock_test.go`
- `internal/lifecycle/bridge/bridge_coverage_extra_test.go` (respawn peer signals, restart path)

## Required Scenarios
1. Multiple concurrent clients converge to one daemon.
2. Followers connect to leader-started daemon within bounded wait.
3. Stale startup lock is detected and safely reclaimed.
4. Restart path recovers from non-responsive daemon process.
