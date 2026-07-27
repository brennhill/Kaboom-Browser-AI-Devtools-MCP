---
doc_type: feature_index
feature_id: feature-push-alerts
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/alerts.go
  - cmd/browser-agent/streaming.go
  - cmd/browser-agent/tools_configure_runtime_impl.go
  - cmd/browser-agent/tools_observe.go
  - cmd/browser-agent/internal/toolobserve/inbox.go
  - cmd/browser-agent/tools_configure_clear.go
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
  - internal/streaming/stream_test.go
  - internal/streaming/alertbuf/alertbuf_test.go
  - internal/push/inbox_test.go
  - cmd/browser-agent/tools_observe_inbox_test.go
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

Add concrete implementation and test links here as this feature evolves.
