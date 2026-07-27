---
doc_type: feature_index
feature_id: feature-query-dom
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/internal/toolanalyze/inspect/dom.go
  - cmd/browser-agent/tools_analyze_dispatch.go
  - src/background/pending-queries.ts
  - src/content/message-handlers.ts
  - src/inject/message-handlers.ts
  - src/lib/analysis/dom-queries.ts
test_paths:
  - cmd/browser-agent/internal/toolanalyze/inspect/dom_test.go
  - cmd/browser-agent/tools_analyze_handler_test.go
  - cmd/browser-agent/tools_pending_query_enqueue_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Query DOM

## TL;DR
- Status: shipped
- Tool: `analyze`
- Mode: `what:"dom"`
- Legacy note: `analyze({what:"dom"})` is non-canonical

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
