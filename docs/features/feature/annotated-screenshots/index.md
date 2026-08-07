---
doc_type: feature_index
feature_id: feature-annotated-screenshots
status: active
feature_type: feature
owners: []
last_reviewed: 2026-08-07
code_paths:
  - src/content/draw-mode/lifecycle-overlay.js
  - src/content/draw-mode/input-rendering.js
  - src/content/draw-mode/element-capture.js
  - src/content/draw-mode/element-analysis.js
  - src/content/draw-mode/persistence-submission.js
  - src/content/draw-mode/geometry-context.js
  - scripts/build/generate-draw-mode.js
  - extension/content/draw-mode.js
  - internal/annotation/store.go
  - internal/annotation/runtime/owner.go
  - internal/annotation/store_details.go
  - internal/annotation/store_named.go
  - internal/annotation/store_sessions.go
  - internal/annotation/store_results.go
  - internal/annotation/draw_sessions_handler.go
  - cmd/browser-agent/internal/toolanalyze/annotationanalysis/handler.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/dispatcher.go
  - cmd/browser-agent/internal/mediaapi/draw_mode.go
  - cmd/browser-agent/internal/mediaapi/screenshots.go
  - cmd/browser-agent/internal/mediaapi/handler.go
  - cmd/browser-agent/internal/toolgenerate/dispatcher.go
  - cmd/browser-agent/internal/toolgenerate/deps.go
  - cmd/browser-agent/internal/toolgenerate/annotations/handlers.go
  - cmd/browser-agent/internal/toolgenerate/annotations/visual.go
  - cmd/browser-agent/internal/toolgenerate/annotations/report.go
  - cmd/browser-agent/internal/toolgenerate/annotations/issues.go
  - cmd/browser-agent/internal/toolgenerate/annotations/builder.go
  - internal/schema/analyze.go
  - internal/tools/configure/capabilities/modespecs_analyze.go
  - scripts/smoke-tests/media/31-annotation-parity.sh
  - scripts/smoke-tests/media/annotation-parity-benchmark.sh
  - scripts/uat/runners/smoke-test.sh
  - package.json
test_paths:
  - internal/annotation/runtime/owner_test.go
  - tests/extension/draw-mode/draw-mode-generation.test.js
  - tests/extension/draw-mode/draw-mode-drawing.test.js
  - tests/extension/draw-mode/draw-mode-enrichment.test.js
  - tests/extension/draw-mode/draw-mode-fixture.js
  - tests/extension/draw-mode/draw-mode-routing.test.js
  - internal/schema/invariants_test.go
  - tests/extension/draw-mode/draw-mode.test.js
  - internal/annotation/store_test.go
  - internal/annotation/named_test.go
  - internal/annotation/store_lifecycle_test.go
  - internal/annotation/store_maintenance_test.go
  - cmd/browser-agent/internal/toolanalyze/analyzedispatch/draw_sessions_test.go
  - cmd/browser-agent/internal/mediaapi/annotation_store_test.go
  - cmd/browser-agent/internal/mediaapi/draw_mode_http_test.go
  - cmd/browser-agent/internal/mediaapi/handler_test.go
  - cmd/browser-agent/server_routes_unit_test.go
  - cmd/browser-agent/internal/toolanalyze/annotationanalysis/handler_test.go
  - cmd/browser-agent/internal/toolanalyze/annotationanalysis/sessions_test.go
  - cmd/browser-agent/internal/toolanalyze/annotationanalysis/wait_test.go
  - cmd/browser-agent/internal/toolanalyze/annotationanalysis/detail_test.go
  - cmd/browser-agent/internal/toolanalyze/annotationanalysis/hints_test.go
  - cmd/browser-agent/internal/toolgenerate/annotations/annotations_test.go
  - scripts/contracts/goarchitecturetests/contracts_test.go
  - tests/extension/contracts/entry-point-parity.test.js
  - scripts/smoke-tests/media/31-annotation-parity.sh
  - scripts/smoke-tests/media/annotation-parity-benchmark.sh
last_verified_version: 0.9.0
last_verified_date: 2026-08-05
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
- `src/content/draw-mode/` is the canonical change-coupled implementation:
  lifecycle/overlay, input/rendering, DOM capture, element analysis,
  persistence/submission, and geometry/context. `scripts/build/generate-draw-mode.js`
  assembles the MV3-loadable `extension/content/draw-mode.js` artifact and
  `--check` fails on drift.
- The persistent action bar exposes **Undo**, a live
  **Submit N annotations** action, and **Cancel**. Escape always cancels,
  Enter saves the active annotation, and the next Enter submits.
- Persisted annotations restored after an interrupted extension lifecycle
  immediately update the action count and remain undoable.
- Daemon-correlated screenshot queries bypass the unsolicited-upload throttle,
  allowing visual baseline and diff capture to run back-to-back while
  preserving rate limiting for uncorrelated browser uploads.

### Go (store + handler)
- `internal/annotation/store.go` — canonical store state, lifecycle, cleanup,
  reset, and wait coordination; `Detail` includes ParentContext, Siblings, and
  CSSFramework fields; session TTL = 2 hours.
- Store time ownership is injected through its private clock boundary. Production
  uses the wall clock, while package tests advance a controlled clock for TTL,
  draw-generation, and eviction behavior without sleeps or timing races.
- `Store.ClearAll()` resets anonymous sessions, named sessions, details, and
  waiters (used by `configure(what:"clear", buffer:"all")` to prevent stale
  replay).
- `internal/annotation/draw_sessions_handler.go` — persisted draw history, traversal-safe loading, and annotation-store hydration
- `cmd/browser-agent/internal/toolanalyze/annotationanalysis/handler.go` — detail response enrichment, error correlation, LLM hints, and cross-project scope safety metadata (`projects`, `scope_ambiguous`, `scope_warning`, `filter_applied`)
- `cmd/browser-agent/internal/toolanalyze/annotationanalysis/handler.go` — annotation retrieval, error correlation, and detail response shaping
- `internal/annotation/store_results.go` owns the canonical filtered named-session
  page projection shared by async completion and analysis enrichment.
- The package is exactly ten files, every file is below 800 lines, and
  `TestPackageFileBoundary` prevents either boundary from regressing. Tests are
  grouped by store behavior, maintenance, lifecycle/persistence, and result
  projection—the same families that change together.
- `cmd/browser-agent/internal/toolgenerate/annotations/visual.go` — resilient visual test generation via locator fallback candidates (`css`, `testid`, `role`, `label`, `placeholder`, `text`)
- `cmd/browser-agent/internal/toolgenerate/annotations/handlers.go` — the three MCP entry points (`visual_test`, `annotation_report`, `annotation_issues`) and session resolution
- Annotation artifact handlers accept the canonical `*annotation.Store`
  directly. There is no annotation host interface or ToolHandler store accessor.
- `cmd/browser-agent/internal/toolgenerate/annotations/report.go` / `issues.go` / `builder.go` — Markdown report rendering, structured issue payloads, and the shared line builder
- `internal/schema/analyze.go` + `internal/tools/configure/capabilities/modespecs_analyze.go` — analyze annotations schema/capability metadata for the canonical `url` filter

### Tests
- `tests/extension/draw-mode/draw-mode-enrichment.test.js` — element detail, framework, and selector enrichment
- `tests/extension/draw-mode/draw-mode-drawing.test.js` — pointer mechanics and annotation context capture
- `tests/extension/draw-mode/draw-mode-routing.test.js` — message routing, lifecycle, accessibility, and re-entry
- `tests/extension/draw-mode/draw-mode.test.js` — activation, annotation CRUD, persistence, and export
- `tests/extension/draw-mode/draw-mode-fixture.js` — shared DOM, Chrome, timer, and module fixtures
- `internal/annotation/store_maintenance_test.go` — `TestStore_SessionTTL_Is2Hours`
- Annotation store tests synchronize waiters through store notifications and use
  the controlled clock for timestamp ordering and expiration; no test relies on
  `time.Sleep` for correctness.
- `internal/annotation/store_lifecycle_test.go` and `cmd/browser-agent/internal/toolanalyze/analyzedispatch/draw_sessions_test.go` — safe persisted-session loading and end-to-end store hydration
- `cmd/browser-agent/internal/toolanalyze/annotationanalysis/` tests — session retrieval, waiting, enrichment fields (`selector_candidates`, `js_framework`, `component`), error correlation, and hints
- `internal/schema/invariants_test.go` — ensures annotations expose only the canonical `url` scope filter and never restore `url_pattern`
- `cmd/browser-agent/internal/toolgenerate/annotations/annotations_test.go` — generator and handler contracts for JS escaping, locator fallbacks, expired detail recovery, named sessions, Markdown reports, structured issues, and Playwright output
- `scripts/smoke-tests/media/31-annotation-parity.sh` — deterministic end-to-end ingest/retrieval/generation gate with bounded retries for transient startup/no_data windows
- `scripts/smoke-tests/media/annotation-parity-benchmark.sh` — repeated pass-rate benchmark with threshold enforcement
- `scripts/uat/runners/smoke-test.sh` — resume-mode daemon version parity guard prevents stale-daemon false negatives in `--only/--start-from` runs
