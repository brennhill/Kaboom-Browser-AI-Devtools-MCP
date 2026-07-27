---
doc_type: feature_index
feature_id: feature-annotated-screenshots
status: active
feature_type: feature
owners: []
last_reviewed: 2026-07-27
code_paths:
  - extension/content/draw-mode.js
  - internal/annotation/store.go
  - cmd/browser-agent/tools_analyze_annotations_handlers.go
  - cmd/browser-agent/tools_analyze_annotations_hints.go
  - cmd/browser-agent/server_routes_media_draw_mode.go
  - cmd/browser-agent/tools_generate.go
  - cmd/browser-agent/internal/toolgenerate/annotations/handlers.go
  - cmd/browser-agent/internal/toolgenerate/annotations/visual.go
  - cmd/browser-agent/internal/toolgenerate/annotations/report.go
  - cmd/browser-agent/internal/toolgenerate/annotations/issues.go
  - cmd/browser-agent/internal/toolgenerate/annotations/builder.go
  - cmd/browser-agent/annotation_store.go
  - internal/schema/analyze.go
  - internal/tools/configure/capabilities/modespecs_analyze.go
  - scripts/smoke-tests/31-annotation-parity.sh
  - scripts/smoke-tests/annotation-parity-benchmark.sh
  - scripts/smoke-test.sh
  - package.json
test_paths:
  - tests/extension/draw-mode.test.js
  - internal/annotation/store_test.go
  - cmd/browser-agent/tools_analyze_annotations_test.go
  - cmd/browser-agent/tools_generate_annotations_test.go
  - cmd/browser-agent/internal/toolgenerate/annotations/annotations_test.go
  - scripts/smoke-tests/31-annotation-parity.sh
  - tests/extension/entry-point-parity.test.js
  - scripts/smoke-tests/31-annotation-parity.sh
  - scripts/smoke-tests/annotation-parity-benchmark.sh
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Annotated Screenshots

## TL;DR

- Status: active
- Tool: analyze
- Mode/Action: annotations, annotation_detail
- Location: `docs/features/feature/annotated-screenshots`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_ANNOTATED_SCREENSHOTS_001
- FEATURE_ANNOTATED_SCREENSHOTS_002
- FEATURE_ANNOTATED_SCREENSHOTS_003

## Code and Tests

### Extension (DOM capture)
- `extension/content/draw-mode.js` — `buildElementDetail()`, `detectCSSFramework()`, `collectSelectorCandidates()`, parent_context, siblings, `js_framework`, `component`

### Go (store + handler)
- `internal/annotation/store.go` — `Detail` struct with ParentContext, Siblings, CSSFramework fields; session TTL = 2 hours
- `internal/annotation/store_clear.go` — `ClearAll()` resets anonymous sessions, named sessions, details, and waiters (used by `configure(what:"clear", buffer:"all")` to prevent stale replay)
- `cmd/browser-agent/tools_analyze_annotations_handlers.go` — detail response enrichment, error correlation, LLM hints, and cross-project scope safety metadata (`projects`, `scope_ambiguous`, `scope_warning`, `filter_applied`)
- `cmd/browser-agent/tools_analyze_annotations_hints.go` — framework-aware detail hints (`design_system`, `runtime_framework`, `error_context`)
- `cmd/browser-agent/internal/toolgenerate/annotations/visual.go` — resilient visual test generation via locator fallback candidates (`css`, `testid`, `role`, `label`, `placeholder`, `text`)
- `cmd/browser-agent/internal/toolgenerate/annotations/handlers.go` — the three MCP entry points (`visual_test`, `annotation_report`, `annotation_issues`) and session resolution
- `cmd/browser-agent/internal/toolgenerate/annotations/report.go` / `issues.go` / `builder.go` — Markdown report rendering, structured issue payloads, and the shared line builder
- `internal/schema/analyze.go` + `internal/tools/configure/capabilities/modespecs_analyze.go` — analyze annotations schema/capability metadata for `url` / `url_pattern` filters

### Tests
- `tests/extension/draw-mode.test.js` — "Element Detail Enrichment" describe block
- `internal/annotation/store_test.go` — `TestStore_SessionTTL_Is2Hours`
- `cmd/browser-agent/tools_analyze_annotations_test.go` — enrichment fields (`selector_candidates`, `js_framework`, `component`), error correlation, hints tests
- `cmd/browser-agent/tools_generate_annotations_test.go` — resilient locator fallback generation tests
- `cmd/browser-agent/internal/toolgenerate/annotations/annotations_test.go` — generator unit tests (JS escaping, locator candidates, Markdown report, issue list, Playwright script) and handler no-data/named-session paths
- `scripts/smoke-tests/31-annotation-parity.sh` — deterministic end-to-end ingest/retrieval/generation gate with bounded retries for transient startup/no_data windows
- `scripts/smoke-tests/annotation-parity-benchmark.sh` — repeated pass-rate benchmark with threshold enforcement
- `scripts/smoke-test.sh` — resume-mode daemon version parity guard prevents stale-daemon false negatives in `--only/--start-from` runs
