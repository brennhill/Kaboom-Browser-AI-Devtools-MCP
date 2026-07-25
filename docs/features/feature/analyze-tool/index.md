---
doc_type: feature_index
feature_id: feature-analyze-tool
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-25
code_paths:
  - cmd/browser-agent/tools_analyze_dispatch.go
  - cmd/browser-agent/tools_analyze_annotations_handlers.go
  - cmd/browser-agent/tools_analyze_api_validation.go
  - cmd/browser-agent/tools_analyze_inspect_forms.go
  - cmd/browser-agent/tools_pending_query_enqueue.go
  - cmd/browser-agent/tools_async_observe_commands.go
  - cmd/browser-agent/tools_async_formatting.go
  - cmd/browser-agent/tools_security_audit.go
  - internal/annotation/store.go
  - internal/annotation/store_results.go
  - internal/annotation/store_wait.go
  - internal/tools/analyze/args_parse.go
  - internal/tools/analyze/computed_styles.go
  - internal/tools/analyze/forms.go
  - internal/tools/analyze/visual_diff.go
  - internal/schema/analyze.go
  - src/background/commands/analyze.ts
  - src/background/frame-targeting.ts
  - src/background/dom/dom-frame-probe.ts
  - src/background/commands/helpers.ts
  - src/content/message-handlers.ts
  - src/content/runtime-message-listener.ts
  - src/inject/data-table.ts
  - src/inject/message-handlers.ts
  - src/types/runtime-messages.ts
test_paths:
  - cmd/browser-agent/tools_analyze_annotations_test.go
  - cmd/browser-agent/tools_analyze_inspect_test.go
  - cmd/browser-agent/tools_analyze_structured_extraction_test.go
  - cmd/browser-agent/tools_analyze_handler_test.go
  - cmd/browser-agent/tools_pending_query_enqueue_test.go
  - internal/annotation/store_test.go
  - internal/tools/analyze/computed_styles_test.go
  - internal/tools/analyze/forms_test.go
  - internal/tools/analyze/visual_diff_test.go
  - tests/extension/data-table.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Analyze Tool

## TL;DR
- Status: shipped
- Tool: `analyze`
- Mode key: `what`
- Contract source: `cmd/browser-agent/tools_schema.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map: `flow-map.md`

## Canonical Note
`analyze` is the active analysis surface. `analyze({what:"dom"})` is the canonical DOM query API.

Structured extraction modes:
- `analyze({what:"form_state"})` returns current form values and field metadata.
- `analyze({what:"data_table"})` returns parsed table headers/rows without `execute_js` string parsing.

Aliases:
- `history` → `navigation_patterns` (quiet alias, dispatches correctly but hidden from schema enum).

Queue saturation for extension-dispatched analyze actions now fails fast with a structured `queue_full` response (via shared enqueue helper), instead of entering async wait/poll flow.
