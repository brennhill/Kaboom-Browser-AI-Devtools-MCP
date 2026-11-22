---
doc_type: feature_index
feature_id: feature-push-alerts
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/composition/toolruntime/tools_configure.go
  - internal/tools/configure/handler/dispatcher.go
  - internal/tools/observe/handler/dispatcher.go
  - internal/tools/observe/handler/deps.go
  - internal/tools/observe/handler/inbox.go
  - internal/tools/configure/handler/clear.go
  - internal/tools/configure/handler/runtime_modes.go
  - internal/observation/streaming/stream.go
  - internal/observation/streaming/stream_emit.go
  - internal/observation/streaming/types.go
  - internal/observation/streaming/stream_filters.go
  - internal/observation/streaming/alertbuf/types.go
  - internal/observation/streaming/alertbuf/buffer.go
  - internal/observation/streaming/alertbuf/process.go
  - internal/platform/identity/mcp.go
  - internal/push/inbox.go
  - internal/platform/types/alert.go
test_paths:
  - internal/observation/resetter/resetter_test.go
  - internal/tools/configure/handler/dispatcher_test.go
  - internal/observation/streaming/stream_test.go
  - internal/observation/streaming/alertbuf/alertbuf_test.go
  - internal/push/inbox_test.go
  - internal/tools/observe/handler/toolobserve_coverage_test.go
  - internal/tools/observe/handler/toolobserve_test.go
  - internal/tools/configure/handler/handlers_coverage_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Push Alerts

## TL;DR

- Status: shipped
- Tool: observe
- Mode/Action: alert system
- Location: `docs/features/feature/push-alerts`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Related Architecture


## Requirement IDs

- FEATURE_PUSH_ALERTS_001
- FEATURE_PUSH_ALERTS_002
- FEATURE_PUSH_ALERTS_003

## Code and Tests

Observe response post-processing receives connectivity and alert-drain
operations explicitly; it does not dispatch through a catch-all host.
Rate-limited notifications use a fixed-capacity pending queue whose size,
capacity, cumulative drops, oldest age, and saturation transition are exposed
through the stream owner rather than silently discarding overload.
