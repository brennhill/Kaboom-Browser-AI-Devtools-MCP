---
feature: analyze-tool
version: 0.7.12
doc_type: qa-plan
feature_id: feature-analyze-tool
last_reviewed: 2026-08-07
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Analyze QA Plan (TARGET)

## Automated Coverage
- `cmd/browser-agent/internal/toolanalyze/linkvalidation/handler_test.go`
- `cmd/browser-agent/tools_analyze_route_test.go`
- `cmd/browser-agent/tools_analyze_handler_test.go`
- `cmd/browser-agent/tools_analyze_page_summary_test.go`

## Required Scenarios
1. Mode dispatch
- Every `what` enum value routes to a valid handler.

2. Async control
- Default sync waits for completion.
- `background:true` returns queued state + correlation ID.
- `observe(command_result)` returns completion payload.

3. Mode-specific contracts
- `dom`: selector validation, frame behavior, response shape.
- `accessibility`: scope/tags/frame behavior.
- `link_health`: timeout/worker/domain options.
- `link_validation`: URL list validation + SSRF safety behavior.
- `api_validation`: `analyze`, `report`, `clear` operations.

4. Annotation flows
- Wait mode completion via draw mode.
- detail lookup success and expiry behavior.

## Manual UAT
1. Run `analyze(what:"dom", selector:"body")`.
2. Run `analyze(what:"accessibility")` and poll if queued.
3. Run `analyze(what:"link_health")` on a page with internal and external links.
4. Run `analyze(what:"api_validation", operation:"analyze")` after traffic capture.
