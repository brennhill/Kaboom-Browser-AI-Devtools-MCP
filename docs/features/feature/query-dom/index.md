---
doc_type: feature_index
feature_id: feature-query-dom
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-08-06
code_paths:
  - cmd/browser-agent/internal/toolanalyze/inspect/dom.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - src/background/pending-queries.ts
  - src/content/message-handlers.ts
  - src/inject.ts
  - src/inject/message-handlers.ts
  - src/lib/analysis/dom-queries.ts
test_paths:
  - cmd/browser-agent/internal/toolanalyze/inspect/dom_test.go
  - cmd/browser-agent/tools_analyze_handler_test.go
  - cmd/browser-agent/tools_pending_query_enqueue_test.go
  - tests/extension/a11y/a11y-runtime-error.test.js
  - tests/extension/a11y/on-demand-a11y-runtime.test.js
  - tests/extension/a11y/on-demand-fixture.js
  - tests/extension/a11y/on-demand.test.js
  - tests/extension/contracts/no-compatibility-facades.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Query DOM

The injected runtime entrypoint performs startup only. DOM query consumers use
`src/lib/analysis/dom-queries.ts` directly rather than an injected-bundle facade.

## TL;DR
- Status: shipped
- Tool: `analyze`
- Mode: `what:"dom"`
- Legacy note: `analyze({what:"dom"})` is non-canonical

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
