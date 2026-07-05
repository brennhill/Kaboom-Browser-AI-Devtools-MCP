---
status: shipped
scope: feature/causal-diffing/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-causal-diffing
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-causal-diffing.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Causal Diffing Review](causal-diffing-review.md).

# Technical Spec: Causal Diffing

## Purpose

When the performance budget monitor detects a regression, the AI knows THAT something got slower but not WHY. A 500ms load time increase could be caused by a new 300KB bundle, a slow API response, a render-blocking stylesheet, or an unoptimized image. Without causal attribution, the AI's only recourse is to guess or ask the human.

Causal diffing answers "why did performance change?" by comparing the resource waterfall between the baseline state and the current state. It identifies what's new, what's gone, what got bigger, and what got slower — then summarizes the probable cause in a form the AI can act on directly.

This turns the AI from a performance alarm ("something is slow") into a performance diagnostician ("your new analytics script added 400KB and blocked rendering for 200ms").

---

## Opportunity & Business Value

**Closes the diagnosis gap**: The performance budget monitor detects regressions. Push notification delivers them. Causal diffing explains them. Together, these three features form a complete performance feedback loop: detect → alert → diagnose → fix.

**Eliminates human investigation**: Without causal diffing, the developer receives "load time regressed by 500ms" and must open Chrome DevTools, compare waterfalls manually, and identify the culprit. With causal diffing, the AI identifies the culprit and proposes a fix in the same session — no human tool-switching needed.

**Actionable AI output**: An AI that says "performance regressed" is marginally useful. An AI that says "the lodash bundle you added in the last commit is 280KB uncompressed and blocks FCP — consider dynamic import" is immediately actionable. Causal diffing provides the data to generate that second message.

**Interoperability with Lighthouse/WebPageTest workflows**: Developers familiar with Lighthouse audits expect explanations like "Avoid enormous network payloads" or "Eliminate render-blocking resources." Causal diffing produces similar explanations but scoped to the delta between two known states, which is more useful than a static audit because it tells you what CHANGED.

---

## How It Works

### Resource Fingerprinting

Each performance snapshot includes a list of loaded resources (from `performance.getEntriesByType('resource')`). Each resource is fingerprinted by its URL path (ignoring query parameters that represent cache-busters or version hashes).

The server stores the resource fingerprint of the baseline alongside the timing metrics. When a new snapshot arrives and a regression is detected, the server compares the current resource list against the baseline's resource list.

### Diff Categories

The comparison produces four categories of changes:

**Added resources**: URLs present in the current snapshot but not in the baseline. These are the most likely culprits for regressions — new scripts, stylesheets, fonts, or images that weren't part of the original load.

**Removed resources**: URLs present in the baseline but not in the current snapshot. These explain improvements (or indicate broken functionality if load time got worse despite removing resources).

**Resized resources**: URLs present in both, but with a transfer size difference exceeding 10% or 10KB (whichever is smaller). A bundle that grew from 100KB to 250KB after adding a dependency appears here.

**Retimed resources**: URLs present in both with similar size, but with a duration difference exceeding 100ms. An API endpoint that was responding in 50ms and now takes 500ms appears here (likely a backend regression, not a frontend change).

### Causal Attribution

After categorizing changes, the server computes a probable cause by analyzing the impact of each change on the overall regression:

1. Sum the transfer sizes of all added resources → "New resources added X KB"
2. Check if any added resources are render-blocking (scripts without `async`/`defer`, stylesheets in `<head>`) → "Render-blocking: [list]"
3. Check if any retimed resources are on the critical path (initiated before FCP) → "Slow critical-path resource: [url] (+Nms)"
4. Compare total transfer size change → "Total payload increased by X%"

The output is a structured summary the AI can consume directly.

### MCP Tool: `get_causal_diff`

A new MCP tool that provides the resource-level comparison.

**Parameters**:
- `url` (optional): URL path to analyze. If omitted, uses the most recently regressed URL (from push notification).
- `baseline_id` (optional): Specific baseline to compare against. If omitted, uses the current stored baseline.

**Response**:
```
{
  "url": "/dashboard",
  "timing_delta": {
    "load_ms": 847,
    "fcp_ms": 320,
    "lcp_ms": 650
  },
  "resource_changes": {
    "added": [
      { "url": "/static/js/analytics.chunk.js", "type": "script", "size_bytes": 287000, "duration_ms": 180, "render_blocking": false },
      { "url": "/static/js/chart-library.js", "type": "script", "size_bytes": 412000, "duration_ms": 250, "render_blocking": true }
    ],
    "removed": [],
    "resized": [
      { "url": "/static/js/main.chunk.js", "baseline_bytes": 145000, "current_bytes": 198000, "delta_bytes": 53000 }
    ],
    "retimed": [
      { "url": "/api/dashboard/data", "baseline_ms": 80, "current_ms": 340, "delta_ms": 260 }
    ]
  },
  "probable_cause": "Added 699KB in new scripts (chart-library.js is render-blocking). API response /api/dashboard/data slowed by 260ms. Total payload increased by 58%.",
  "recommendations": [
    "Consider lazy-loading chart-library.js (412KB, render-blocking)",
    "Investigate API regression on /api/dashboard/data (+260ms)",
    "main.chunk.js grew by 53KB — review recent imports"
  ]
}
```

---

## Data Model

### Baseline Resource Fingerprint

Stored alongside each `PerformanceBaseline`:
- List of resource entries: `{ url, type, transferSize, duration, renderBlocking }` normalized by URL path
- Total resource count
- Total transfer size
- Computed on first snapshot, updated with exponential moving average on subsequent snapshots (size and duration are averaged, not replaced)

### Resource Classification

Resources are classified by type using the `initiatorType` field from the Performance API:
- `script`: JavaScript files
- `css`: Stylesheets
- `img`: Images
- `font`: Web fonts
- `fetch`/`xmlhttprequest`: API calls
- `other`: Everything else

Render-blocking determination comes from two signals:
1. Scripts without `async` or `defer` attributes (detected from the resource timing `renderBlockingStatus` field, available in Chrome 107+)
2. Stylesheets loaded in the document `<head>` (heuristic: stylesheet loaded before FCP is likely render-blocking)

---

## Integration with Push Notification

When a regression alert is generated (from the push notification spec), the alert's `recommendation` field references causal diffing: "Use `get_causal_diff` for resource-level analysis."

The AI's natural workflow becomes:
1. Receive regression alert in `get_changes_since`
2. Call `get_causal_diff` to understand why
3. Make a code change to fix the cause
4. Watch `get_changes_since` for the next snapshot confirming the fix

---

## Edge Cases

- **No baseline resource list** (baseline created before causal diffing was implemented): The tool returns timing deltas but reports "resource comparison unavailable — baseline predates resource tracking."
- **URL normalization**: Query params are stripped for comparison (`main.chunk.js?v=abc123` → `main.chunk.js`). Hash fragments are preserved (they indicate code-split chunks in some bundlers).
- **CDN URLs**: Resources served from CDNs (different hostname) are matched by path only if the path is unique. If two CDNs serve `/jquery.min.js`, they're treated as separate resources.
- **Very large resource lists** (>200 entries): Only the top 50 by transfer size are stored in the baseline fingerprint. Small resources (<1KB) are aggregated as "N small resources totaling X KB."
- **Dynamic resources** (API calls with varying paths like `/api/user/123`): These are grouped by path prefix (first 2 segments). `/api/user/123` and `/api/user/456` are the same resource for diffing purposes.
- **Same resources, different order**: Order doesn't matter. Comparison is set-based on normalized URL.
- **Regression with no resource changes**: The probable cause says "No resource changes detected. Regression may be caused by slower backend responses, increased DOM complexity, or browser throttling."

---

## Performance Constraints

- Resource fingerprint storage: under 50KB per baseline (50 entries × ~1KB each)
- Diff computation: under 5ms (set intersection/difference on 50-entry lists)
- No additional network requests from the extension (uses existing performance API data)
- Resource list included in existing performance snapshot POST (no new endpoint)

---

## Test Scenarios

1. Added script appears in `resource_changes.added` with correct size
2. Removed stylesheet appears in `resource_changes.removed`
3. Bundle that grew 50KB appears in `resource_changes.resized`
4. API endpoint 200ms slower appears in `resource_changes.retimed`
5. Render-blocking script flagged with `render_blocking: true`
6. `probable_cause` summarizes total added size and blocking resources
7. `recommendations` array contains actionable items
8. URL normalization strips query params but preserves hash
9. No baseline → returns timing delta only with "unavailable" message
10. No resource changes → probable_cause mentions backend/DOM/throttling
11. More than 200 resources → only top 50 by size stored
12. Small resources (<1KB) aggregated in summary
13. Dynamic API paths grouped by prefix
14. `get_causal_diff` with explicit URL parameter overrides "most recent"
15. Resource fingerprint updated with moving average on subsequent snapshots

---

## File Locations

Server implementation: `cmd/browser-agent/performance.go` (resource fingerprinting, diff computation, MCP tool handler).

Extension changes: Minor addition to the performance snapshot — include `renderBlockingStatus` field from resource timing entries. File: `extension/inject.js`.

Tests: `cmd/browser-agent/performance_test.go`.
