---
doc_type: feature_index
feature_id: feature-link-health
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-05
code_paths:
  - cmd/browser-agent/tools_analyze_dispatch.go
  - src/lib/link-health.ts
  - src/background/pending-queries.ts
  - src/content/message-handlers.ts
  - src/inject/message-handlers.ts
test_paths:
  - cmd/browser-agent/tools_analyze_validation_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Link Health

## TL;DR
- Status: shipped
- Tool: `analyze`
- Modes: `what:"link_health"` and `what:"link_validation"`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
