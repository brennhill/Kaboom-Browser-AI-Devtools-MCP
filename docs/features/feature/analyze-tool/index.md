---
doc_type: feature_index
feature_id: feature-analyze-tool
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-11-21
code_paths:
  - internal/composition/toolruntime/tools_core.go
  - internal/tools/analyze/handler/deps.go
  - internal/tools/playbooks/resources/guides.go
  - internal/tools/analyze/handler/analyzedispatch/dispatcher.go
  - internal/tools/analyze/handler/annotationanalysis/handler.go
  - internal/analysis/apicontract/runtime/handler.go
  - internal/tools/analyze/handler/inspect/forms.go
  - internal/tools/analyze/handler/inspect/dom.go
  - internal/tools/analyze/handler/visual/handler.go
  - internal/page/app/screenshot/screenshot.go
  - internal/tools/analyze/handler/pageissues/handler.go
  - internal/tools/analyze/handler/designdrift/handler.go
  - internal/tools/analyze/handler/verificationhandler/handler.go
  - internal/analysis/verification/contract.go
  - internal/analysis/verification/evidence.go
  - internal/analysis/verification/store.go
  - internal/annotation/draw_sessions_handler.go
  - internal/tools/observe/handler/dispatcher.go
  - internal/tools/observe/handler/deps.go
  - internal/tools/asynccommand/handler.go
  - internal/tools/analyze/handler/combinedaudit/handler.go
  - internal/tools/analyze/handler/navigation.go
  - internal/tools/analyze/handler/linkvalidation/handler.go
  - internal/tools/analyze/handler/securityaudit/handler.go
  - internal/tools/response/toolresp.go
  - internal/tools/mcp/response.go
  - internal/annotation/store.go
  - internal/annotation/store_results.go
  - internal/tools/analyze/args_parse.go
  - internal/tools/analyze/computed_styles.go
  - internal/tools/analyze/forms.go
  - internal/tools/analyze/link_validation.go
  - internal/tools/analyze/visual_diff.go
  - internal/tools/analyze/imagediff/imagediff.go
  - internal/tools/analyze/imagediff/grid.go
  - internal/tools/analyze/imagediff/regions.go
  - internal/tools/analyze/imagediff/imageio.go
  - internal/tools/schema/analyze.go
  - src/background.ts
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
  - internal/tools/analyze/handler/analyzedispatch/dispatcher_test.go
  - internal/tools/analyze/handler/analyzedispatch/draw_sessions_test.go
  - internal/tools/analyze/handler/annotationanalysis/handler_test.go
  - internal/tools/analyze/handler/annotationanalysis/detail_test.go
  - internal/tools/analyze/handler/annotationanalysis/hints_test.go
  - internal/tools/analyze/handler/annotationanalysis/sessions_test.go
  - internal/tools/analyze/handler/annotationanalysis/wait_test.go
  - internal/tools/analyze/handler/inspect/forms_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - internal/tools/analyze/handler/handlers_coverage_test.go
  - internal/analysis/apicontract/runtime/handler_test.go
  - internal/tools/analyze/handler/inspect/forms_test.go
  - internal/tools/analyze/handler/inspect/dom_test.go
  - internal/tools/analyze/handler/visual/handler_test.go
  - internal/tools/analyze/handler/pageissues/handler_test.go
  - internal/tools/analyze/handler/pageissues/summary_test.go
  - internal/tools/analyze/handler/verificationhandler/handler_test.go
  - internal/analysis/verification/contract_test.go
  - internal/analysis/verification/evidence_test.go
  - internal/analysis/verification/store_test.go
  - internal/annotation/store_lifecycle_test.go
  - internal/tools/analyze/handler/combinedaudit/handler_test.go
  - internal/tools/analyze/handler/linkvalidation/handler_test.go
  - internal/tools/analyze/handler/securityaudit/handler_test.go
  - internal/annotation/store_test.go
  - internal/tools/analyze/computed_styles_test.go
  - internal/tools/analyze/forms_test.go
  - internal/tools/analyze/link_validation_test.go
  - internal/tools/analyze/visual_diff_test.go
  - internal/tools/analyze/imagediff/imagediff_test.go
  - tests/extension/misc/data-table.test.js
  - tests/extension/dom/page-query-targeting.test.js
  - tests/extension/contracts/no-compatibility-facades.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Analyze Tool

Analyze modes that reuse observation logic receive the same explicit observation
read value. The unused `internal/tools/analyze` host declaration was deleted
rather than retained as a prospective interface.

## TL;DR

- Visual-diff thresholds are clamped before unsigned arithmetic, and rendered
  channel narrowing is explicitly proven safe for the security scanner.
- Status: shipped
- Tool: `analyze`
- Mode key: `what`
- Contract source: `internal/tools/schema/analyze.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map: `flow-map.md`

## Canonical Note
`analyze` is the active analysis surface. `analyze({what:"dom"})` is the canonical DOM query API.
The mode registry and alias policy are owned by
`internal/toolanalyze/analyzedispatch/dispatcher.go`; feature implementations
remain in their dedicated modules. The dispatcher closes over separate
analyze, inspect, observe, audit, and visual dependency groups. Analyze-owned
telemetry and security access uses explicit function fields, so there is no
catch-all host interface or root forwarding surface.
Analyze actions receive queueing, waiting, and accessibility execution directly
from `internal/tools/asynccommand.Handler`; `ToolHandler` no longer implements an
asynchronous host contract.
The background service-worker entrypoint owns startup only. Analysis tests and
runtime code import their focused owner modules rather than an entrypoint facade.
Synchronous analyze timeout coverage injects a terminal command-completion seam
after the real queue and correlation registration run. This proves the caller's
budget and exactly-once response without racing a short-lived queue entry.
Annotation-detail parsing has an owner-level malformed-JSON contract so invalid
requests fail before any store lookup or response enrichment.

Structured extraction modes:
- `analyze({what:"form_state"})` returns current form values and field metadata.
- `analyze({what:"data_table"})` returns parsed table headers/rows without `execute_js` string parsing.

Aliases:
- Navigation-pattern analysis uses the canonical `navigation_patterns` mode.

Tool dispatch uses only the canonical `what` selector and canonical mode names;
`mode`, `action`, `a11y`, and `history` routing shortcuts are not accepted.

Annotation execution uses only `background`: false blocks for the bounded wait,
while true returns immediately. The former inverse `wait` parameter is removed.
Annotation wait tests order completed sessions with explicit future timestamps
and coordinate concurrent delivery with channels. They do not depend on the
host clock or scheduler delays to make a session newer than draw activation.

Queue saturation for extension-dispatched analyze actions now fails fast with a structured `queue_full` response (via shared enqueue helper), instead of entering async wait/poll flow.
