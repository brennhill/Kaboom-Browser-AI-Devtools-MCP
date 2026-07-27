---
doc_type: feature_index
feature_id: feature-deployment-watchdog
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/main_connection_mcp_upgrade.go
  - cmd/browser-agent/binary_watcher.go
  - cmd/browser-agent/config_modes.go
  - cmd/browser-agent/health_tool_handler.go
  - cmd/browser-agent/server_routes_diagnostics.go
  - cmd/browser-agent/handler_tools_call.go
test_paths:
  - cmd/browser-agent/binary_watcher_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Deployment Watchdog

## TL;DR

- Status: proposed
- Tool: configure, observe
- Mode/Action: watchdog, deployment_status
- Location: `docs/features/feature/deployment-watchdog`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_DEPLOYMENT_WATCHDOG_001
- FEATURE_DEPLOYMENT_WATCHDOG_002
- FEATURE_DEPLOYMENT_WATCHDOG_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
