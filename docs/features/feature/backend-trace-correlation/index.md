---
doc_type: feature_index
feature_id: feature-backend-trace-correlation
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-04
code_paths:
  - internal/performance/tracecorr/correlate.go
  - internal/tools/observe/session.go
  - src/lib/net/network.ts
  - cmd/browser-agent/internal/cli/parser/observe_analyze.go
test_paths:
  - internal/performance/tracecorr/correlate_test.go
  - tests/extension/network-http/network-waterfall.test.js
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# Backend Trace Correlation

## TL;DR

Pass `trace_source` to `analyze({what:"performance"})` to correlate captured
browser `traceparent` headers with a local OpenTelemetry JSON export. Trace data
is read locally and is never sent to product telemetry or any external service.
Category breakdowns use exclusive time so nested spans are not double-counted.
Malformed identifiers, timestamps, and unsupported export shapes fail loudly.

## Specs

- [Product specification](./product-spec.md)
- [Technical specification](./tech-spec.md)
- [QA plan](./qa-plan.md)
