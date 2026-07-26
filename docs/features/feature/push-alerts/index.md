---
doc_type: feature_index
feature_id: feature-push-alerts
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-05
code_paths:
  - cmd/browser-agent/alerts.go
  - cmd/browser-agent/streaming.go
  - cmd/browser-agent/tools_configure_runtime_impl.go
  - cmd/browser-agent/tools_observe_inbox.go
  - cmd/browser-agent/tools_configure_state_impl.go
  - internal/streaming/stream.go
  - internal/streaming/stream_emit.go
  - internal/streaming/types.go
  - internal/streaming/alerts_buffer.go
  - internal/identity/mcp.go
  - internal/push/inbox.go
test_paths:
  - internal/streaming/stream_test.go
  - internal/streaming/alerts_test.go
  - cmd/browser-agent/alerts_unit_test.go
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
