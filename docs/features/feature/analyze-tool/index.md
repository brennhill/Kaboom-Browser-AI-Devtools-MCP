---
doc_type: feature_index
feature_id: feature-analyze-tool
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - cmd/browser-agent/tools_analyze_dispatch.go
  - cmd/browser-agent/tools_analyze_annotations_handlers.go
  - internal/analysis/apicontract/runtime_handler.go
  - cmd/browser-agent/internal/toolanalyze/inspect/forms.go
  - cmd/browser-agent/internal/toolanalyze/inspect/dom.go
  - cmd/browser-agent/internal/toolanalyze/visual/handler.go
  - cmd/browser-agent/internal/toolanalyze/pageissues/handler.go
  - internal/annotation/draw_sessions_handler.go
  - cmd/browser-agent/tools_async_completion.go
  - cmd/browser-agent/tools_observe.go
  - cmd/browser-agent/tools_async_completion.go
  - cmd/browser-agent/internal/toolanalyze/combinedaudit/handler.go
  - internal/annotation/store.go
  - internal/annotation/store_results.go
  - internal/annotation/store_wait.go
  - internal/tools/analyze/args_parse.go
  - internal/tools/analyze/computed_styles.go
  - internal/tools/analyze/forms.go
  - internal/tools/analyze/link_validation.go
  - internal/tools/analyze/visual_diff.go
  - internal/tools/analyze/imagediff/imagediff.go
  - internal/tools/analyze/imagediff/grid.go
  - internal/tools/analyze/imagediff/regions.go
  - internal/tools/analyze/imagediff/imageio.go
  - internal/schema/analyze.go
  - src/background/commands/analyze.ts
  - src/background/exec/frame-targeting.ts
  - src/background/dom/primitives/dom-frame-probe.ts
  - src/background/commands/helpers.ts
  - src/content/message-handlers.ts
  - src/content/runtime-message-listener.ts
  - src/inject/data-table.ts
  - src/inject/message-handlers.ts
  - src/types/runtime-messages.ts
test_paths:
  - internal/analysis/apicontract/runtime_handler_test.go
  - cmd/browser-agent/tools_analyze_annotations_test.go
  - cmd/browser-agent/tools_analyze_inspect_test.go
  - cmd/browser-agent/internal/toolanalyze/inspect/forms_test.go
  - cmd/browser-agent/internal/toolanalyze/inspect/dom_test.go
  - cmd/browser-agent/internal/toolanalyze/visual/handler_test.go
  - cmd/browser-agent/internal/toolanalyze/pageissues/handler_test.go
  - cmd/browser-agent/internal/toolanalyze/pageissues/summary_test.go
  - internal/annotation/draw_sessions_handler_test.go
  - cmd/browser-agent/tools_analyze_annotations_draw_test.go
  - cmd/browser-agent/tools_analyze_structured_extraction_test.go
  - cmd/browser-agent/internal/toolanalyze/combinedaudit/handler_test.go
  - cmd/browser-agent/tools_analyze_handler_test.go
  - cmd/browser-agent/tools_pending_query_enqueue_test.go
  - internal/annotation/store_test.go
  - internal/tools/analyze/computed_styles_test.go
  - internal/tools/analyze/forms_test.go
  - internal/tools/analyze/link_validation_test.go
  - internal/tools/analyze/visual_diff_test.go
  - internal/tools/analyze/imagediff/imagediff_test.go
  - tests/extension/data-table.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Analyze Tool

## TL;DR
- Status: shipped
- Tool: `analyze`
- Mode key: `what`
- Contract source: `internal/schema/analyze.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map: `flow-map.md`

## Canonical Note
`analyze` is the active analysis surface. `analyze({what:"dom"})` is the canonical DOM query API.
The mode registry, alias policy, and narrow `toolanalyze.Deps` adaptation are
colocated in `tools_analyze_dispatch.go`; feature implementations remain in
their dedicated modules.

Structured extraction modes:
- `analyze({what:"form_state"})` returns current form values and field metadata.
- `analyze({what:"data_table"})` returns parsed table headers/rows without `execute_js` string parsing.

Aliases:
- `history` → `navigation_patterns` (quiet alias, dispatches correctly but hidden from schema enum).

Queue saturation for extension-dispatched analyze actions now fails fast with a structured `queue_full` response (via shared enqueue helper), instead of entering async wait/poll flow.
