---
doc_type: feature_index
feature_id: feature-deployment-watchdog
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/lifecycle/upgrade/binarywatch/watcher.go
  - cmd/browser-agent/config.go
  - internal/lifecycle/startupconfig/runtime.go
  - internal/composition/toolruntime/tools_configure.go
  - internal/tools/configure/handler/dispatcher.go
  - internal/composition/http/operational/handler.go
  - internal/tools/mcpserver/endpoint/handler.go
  - cmd/browser-agent/server.go
  - cmd/browser-agent/openapi.json
  - internal/composition/http/dashboard/diagnostics.html
  - src/generated/openapi-types.ts
test_paths:
  - internal/observation/healthreader/reader_test.go
  - internal/tools/configure/handler/dispatcher_test.go
  - internal/lifecycle/upgrade/binarywatch/watcher_test.go
  - internal/lifecycle/startupconfig/runtime_test.go
  - internal/composition/http/operational/health_test.go
  - internal/tools/mcpserver/endpoint/handler_test.go
  - cmd/browser-agent/composition_test.go
  - tests/extension/contracts/no-compatibility-facades.test.js
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

`GET /health` exposes `name` as the sole daemon identity field. Diagnostics
JSON is served only by `GET /diagnostics`; the dashboard uses that canonical
route, and generated TypeScript contracts come directly from
`cmd/browser-agent/openapi.json`.
Operational health handlers consume the canonical `healthreader.Reader`;
`Capture` no longer exposes a health forwarding method.
Binary-upgrade polling, baseline capture, cancellation, and grace-period waits
have explicit private runtime signals in tests. Upgrade detection and watcher
shutdown are therefore verified without wall-clock sleeps while production
continues to use real tickers and timers.
