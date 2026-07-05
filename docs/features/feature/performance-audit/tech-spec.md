---
feature: performance-audit
status: proposed
doc_type: tech-spec
feature_id: feature-performance-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Performance Audit

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Performance Audit adds `observe({what: "performance_audit"})` -- a new observation mode that analyzes the current page for performance issues and returns categorized findings with severity levels and recommendations. Implemented as extension-side analysis (inject.js) using Performance API, Resource Timing API, and DOM inspection.

Unlike passive performance monitoring (Web Vitals, network waterfall), this is an active audit that computes recommendations based on detected anti-patterns.

## Key Components

**Audit Engine**: Runs five audit categories in parallel:

1. **Render-blocking resources**: Identifies scripts without `async`/`defer` and stylesheets in `<head>` that block rendering. Uses `performance.getEntriesByType('resource')` and cross-references with DOM to check attributes.

2. **Bundle size**: Flags JavaScript bundles > 250KB and total payload > 2MB. Uses `transferSize` from Resource Timing entries.

3. **DOM bloat**: Counts DOM nodes. Flags if > 1500 nodes (moderate) or > 3000 nodes (severe). Uses `document.querySelectorAll('*').length`.

4. **Unoptimized images**: Identifies images served without compression (`Content-Encoding` header missing) or excessively large (> 500KB). Cross-references network waterfall with `<img>` tags.

5. **Duplicate resources**: Detects same resource loaded multiple times (same URL path, different query params or hosts). Groups by normalized URL.

**Severity Classification**: Each finding assigned severity: `critical` (blocks FCP/LCP), `high` (significant performance impact), `medium` (noticeable degradation), `low` (optimization opportunity).

**Recommendation Generator**: For each finding, generates actionable recommendation (e.g., "Add async attribute to script.js", "Use responsive images with srcset", "Bundle duplicate lodash libraries").

## Data Flows

```
AI calls observe({what: "performance_audit"})
  |
  v
Server creates PendingQuery{Type: "performance_audit"}
  |
  v
Extension dispatches to inject.js
  |
  v
inject.js: runPerformanceAudit() executes
  -> Query Resource Timing API for all resources
  -> Query DOM for render-blocking detection
  -> Count DOM nodes
  -> Identify images and check sizes
  -> Detect duplicates
  -> Classify severity for each finding
  -> Generate recommendations
  |
  v
Result posted to server
  |
  v
Server returns audit report to AI
```

## Implementation Strategy

**Extension files**:
- `extension/lib/performance-audit.js` (new): Audit engine, severity classifier, recommendation generator
- `extension/inject.js` (modified): Import and invoke performance audit on query

**Server files**:
- `cmd/browser-agent/queries.go`: Add `toolObservePerformanceAudit()` handler

**Trade-offs**:
- Extension-side analysis (not server-side) because requires DOM access and Resource Timing API. Server only receives final audit report.
- Synchronous execution (not async) acceptable because audit completes in < 200ms on typical pages.

## Edge Cases & Assumptions

- **No resources loaded**: Returns empty findings with note "no resources to audit."
- **Resource Timing unavailable**: Falls back to DOM-only audits (render-blocking, DOM bloat).
- **Large resource lists (>200)**: Audits top 50 by transferSize, summarizes rest.

## Risks & Mitigations

**Risk**: Audit overhead degrades page performance.
**Mitigation**: Audit runs on-demand only (not continuous). Completes in < 200ms. No DOM modifications.

## Dependencies

- Performance API (shipped in all modern browsers)
- Resource Timing API (shipped)
- Existing query dispatch infrastructure

## Performance Considerations

| Metric | Target |
|--------|--------|
| Audit execution time | < 200ms |
| Memory impact | < 1MB |
| Output size | < 20KB |

## Security Considerations

- Read-only DOM and Performance API access
- No network requests initiated
- Resource URLs may contain path params but query strings stripped (existing privacy layer)
