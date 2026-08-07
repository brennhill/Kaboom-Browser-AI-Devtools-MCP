---
doc_type: feature_index
feature_id: feature-push-alerts
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - cmd/browser-agent/tools_configure.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher.go
  - cmd/browser-agent/internal/toolobserve/dispatcher.go
  - cmd/browser-agent/internal/toolobserve/deps.go
  - cmd/browser-agent/internal/toolobserve/inbox.go
  - cmd/browser-agent/internal/toolconfigure/clear.go
  - cmd/browser-agent/internal/toolconfigure/runtime_modes.go
  - internal/streaming/stream.go
  - internal/streaming/stream_emit.go
  - internal/streaming/types.go
  - internal/streaming/stream_filters.go
  - internal/streaming/alertbuf/types.go
  - internal/streaming/alertbuf/buffer.go
  - internal/streaming/alertbuf/process.go
  - internal/identity/mcp.go
  - internal/push/inbox.go
  - internal/types/alert.go
test_paths:
  - internal/capture/resetter/resetter_test.go
  - cmd/browser-agent/internal/toolconfigure/dispatcher_test.go
  - internal/streaming/stream_test.go
  - internal/streaming/alertbuf/alertbuf_test.go
  - internal/push/inbox_test.go
  - cmd/browser-agent/tools_observe_inbox_test.go
  - cmd/browser-agent/tools_observe_unit_test.go
  - cmd/browser-agent/internal/toolconfigure/handlers_coverage_test.go
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
