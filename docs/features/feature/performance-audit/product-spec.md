---
feature: performance-audit
status: proposed
version: null
tool: generate
mode: performance_audit
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-performance-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Performance Audit

> Generates a comprehensive, Lighthouse-style performance audit with structured bottleneck analysis and actionable fix recommendations, designed for consumption by AI coding agents.

## Problem

AI coding agents working on web applications need to identify and fix performance problems, but Kaboom's existing performance tools only provide raw metrics, not diagnostic analysis:

1. **`observe({what: "performance"})`** returns a performance snapshot: navigation timing (TTFB, FCP, LCP, load), network summary (request count, transfer size), long tasks, and CLS. It tells the AI *what* the numbers are, but not *why* they are bad or *what to do about it*.

2. **`observe({what: "vitals"})`** returns Core Web Vitals with good/needs-improvement/poor assessments. This is a pass/fail grade, not a diagnosis.

3. **Causal diffing** (internal to `observe({what: "performance"})`) compares snapshots against baselines and identifies resource changes that may explain regressions. This is useful for "what changed?" but not for "what is wrong in absolute terms?"

None of these tools answer the fundamental question an AI coding agent needs answered: **"What specific performance problems does this page have, and what code changes would fix them?"**

The gap is the analysis layer between raw metrics and actionable code fixes. A developer running Lighthouse gets a categorized audit with specific recommendations ("Eliminate render-blocking resources", "Properly size images", "Reduce unused JavaScript"). An AI agent using Kaboom gets numbers and must independently reason about what they mean. This is wasteful -- the browser already has the data to produce these recommendations, and Kaboom should surface them in a structured format the AI can act on.

## Solution

Performance Audit is a new mode under the `generate` tool that produces a comprehensive, structured performance analysis. It collects data from multiple sources already available to Kaboom (performance snapshots, network waterfall, resource timing, DOM queries) and synthesizes them into a categorized audit report with scored sections, identified bottlenecks, and specific fix recommendations.

The audit is generated on-demand (not continuously), combining:

1. **Server-side analysis** of already-captured telemetry (performance snapshots, network waterfall entries, resource timing data from the extension).
2. **On-demand DOM queries** via the existing async command infrastructure to collect page-specific data not available from passive capture (DOM node counts, image dimensions, inline script sizes, render-blocking link/script elements in `<head>`).

The output is structured JSON designed for LLM consumption: each audit category contains a score, findings with severity, and recommendations that reference specific resources or DOM elements the AI can address in code.

This differs from existing tools in three ways:
- **Scope**: Analyzes 8+ categories vs. a single metrics snapshot.
- **Diagnosis**: Identifies specific bottlenecks (e.g., "bundle.js is 450KB of which ~60% appears unused") vs. reporting raw numbers.
- **Prescriptive**: Provides concrete recommendations (e.g., "Add `loading='lazy'` to images below the fold") vs. leaving interpretation to the AI.

## User Stories

- As an AI coding agent, I want to run a comprehensive performance audit so that I can identify all performance bottlenecks in a single call rather than manually correlating data from multiple observe calls.
- As an AI coding agent, I want each performance finding to include a specific fix recommendation so that I can generate code changes without additional research.
- As an AI coding agent, I want the audit results scored by category so that I can prioritize which performance issues to fix first.
- As an AI coding agent, I want to scope the audit to specific categories so that I can focus on the areas relevant to the current task (e.g., only image optimization after adding new images).
- As a developer using Kaboom, I want the AI to detect render-blocking resources, oversized bundles, and unoptimized images so that I get actionable performance improvements without running a separate Lighthouse audit.

## MCP Interface

**Tool:** `generate`
**Mode:** `performance_audit`

### Request

```json
{
  "tool": "generate",
  "arguments": {
    "format": "performance_audit",
    "url": "https://example.com/app",
    "categories": ["render_blocking", "dom_size", "images", "javascript", "css", "third_party", "caching", "compression"],
    "include_recommendations": true
  }
}
```

#### Parameters:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `format` | string | yes | -- | Must be `"performance_audit"` |
| `url` | string | no | latest snapshot URL | URL to audit. If omitted, audits the most recently captured page. |
| `categories` | string[] | no | all categories | Which audit categories to run. Omit for a full audit. |
| `include_recommendations` | bool | no | true | Whether to include fix recommendations in the output. Set to false for a metrics-only report. |

**Valid categories:** `render_blocking`, `dom_size`, `images`, `javascript`, `css`, `third_party`, `caching`, `compression`

### Response

```json
{
  "audit": {
    "url": "https://example.com/app",
    "timestamp": "2026-01-28T14:30:00Z",
    "overall_score": 62,
    "summary": "Page has 3 critical and 5 moderate performance issues. Largest opportunities: eliminating render-blocking resources (est. -800ms LCP) and optimizing images (est. -1.2MB transfer).",
    "categories": {
      "render_blocking": {
        "score": 35,
        "severity": "critical",
        "findings": [
          {
            "id": "rb-1",
            "severity": "critical",
            "title": "Render-blocking CSS in <head>",
            "description": "3 external stylesheets block rendering for 420ms before first paint.",
            "resources": [
              {"url": "/css/main.css", "size_bytes": 89000, "blocking_time_ms": 180},
              {"url": "/css/vendor.css", "size_bytes": 245000, "blocking_time_ms": 310},
              {"url": "/css/theme.css", "size_bytes": 12000, "blocking_time_ms": 95}
            ],
            "recommendation": "Inline critical CSS and load non-critical stylesheets asynchronously using `<link rel='preload' as='style' onload=\"this.rel='stylesheet'\">`. Consider extracting above-the-fold CSS (~15KB) and deferring the rest.",
            "estimated_impact_ms": 350
          },
          {
            "id": "rb-2",
            "severity": "high",
            "title": "Render-blocking JavaScript in <head>",
            "description": "2 scripts without async/defer block parsing for 280ms.",
            "resources": [
              {"url": "/js/analytics.js", "size_bytes": 45000, "blocking_time_ms": 120},
              {"url": "/js/polyfills.js", "size_bytes": 78000, "blocking_time_ms": 160}
            ],
            "recommendation": "Add `defer` attribute to scripts that do not need to execute before DOM parsing. analytics.js is a candidate for `async` loading.",
            "estimated_impact_ms": 250
          }
        ]
      },
      "dom_size": {
        "score": 72,
        "severity": "moderate",
        "findings": [
          {
            "id": "dom-1",
            "severity": "moderate",
            "title": "Excessive DOM size",
            "description": "DOM contains 2,847 nodes. Pages with more than 1,500 nodes may experience slower style recalculations and layout operations.",
            "metrics": {
              "total_nodes": 2847,
              "max_depth": 18,
              "max_children": 142,
              "deepest_path": "html > body > div#app > div.main > div.content > div.list > div.item:nth-child(142)"
            },
            "recommendation": "The element at div.list contains 142 children. Consider virtualizing this list (e.g., react-window, virtual scrolling) to render only visible items. Target: fewer than 1,500 DOM nodes.",
            "estimated_impact_ms": 50
          }
        ]
      },
      "images": {
        "score": 45,
        "severity": "critical",
        "findings": [
          {
            "id": "img-1",
            "severity": "critical",
            "title": "Images served without modern format",
            "description": "8 images are served as PNG/JPEG that could use WebP/AVIF for 40-60% size reduction.",
            "resources": [
              {"url": "/images/hero.png", "size_bytes": 890000, "dimensions": "1920x1080", "potential_savings_bytes": 534000},
              {"url": "/images/team.jpg", "size_bytes": 420000, "dimensions": "800x600", "potential_savings_bytes": 210000}
            ],
            "recommendation": "Convert images to WebP format using `<picture>` element with PNG/JPEG fallback. Estimated total savings: 1.2MB.",
            "estimated_impact_ms": 400
          },
          {
            "id": "img-2",
            "severity": "moderate",
            "title": "Images missing explicit dimensions",
            "description": "5 images lack width/height attributes, contributing to layout shift (CLS).",
            "resources": [
              {"url": "/images/avatar.png", "rendered_size": "48x48"},
              {"url": "/images/logo.svg", "rendered_size": "200x40"}
            ],
            "recommendation": "Add explicit `width` and `height` attributes to all `<img>` elements to prevent layout shift during loading.",
            "estimated_impact_ms": 0
          }
        ]
      },
      "javascript": {
        "score": 58,
        "severity": "high",
        "findings": [
          {
            "id": "js-1",
            "severity": "high",
            "title": "Large JavaScript bundles",
            "description": "Total JavaScript payload is 1.8MB (compressed). 2 bundles exceed 200KB each.",
            "resources": [
              {"url": "/js/vendor.bundle.js", "size_bytes": 680000, "compressed_bytes": 195000},
              {"url": "/js/app.bundle.js", "size_bytes": 520000, "compressed_bytes": 148000}
            ],
            "recommendation": "Implement code splitting to load JavaScript on demand. vendor.bundle.js (680KB) likely contains libraries that can be tree-shaken or lazy-loaded.",
            "estimated_impact_ms": 300
          },
          {
            "id": "js-2",
            "severity": "moderate",
            "title": "Unused JavaScript estimation",
            "description": "Based on resource coverage heuristics, approximately 45% of loaded JavaScript may be unused on initial load.",
            "metrics": {
              "total_js_bytes": 1800000,
              "estimated_unused_bytes": 810000,
              "estimated_unused_pct": 45
            },
            "recommendation": "Use dynamic `import()` for routes and heavy components. Consider analyzing with Chrome DevTools Coverage tab for precise unused code identification.",
            "estimated_impact_ms": 200
          }
        ]
      },
      "css": {
        "score": 70,
        "severity": "moderate",
        "findings": [
          {
            "id": "css-1",
            "severity": "moderate",
            "title": "Large CSS payload",
            "description": "Total CSS payload is 346KB. Stylesheets exceeding 100KB often contain significant unused rules.",
            "resources": [
              {"url": "/css/vendor.css", "size_bytes": 245000},
              {"url": "/css/main.css", "size_bytes": 89000}
            ],
            "recommendation": "Audit CSS for unused rules using PurgeCSS or similar tooling. Consider splitting vendor CSS and loading component-specific styles on demand.",
            "estimated_impact_ms": 100
          }
        ]
      },
      "third_party": {
        "score": 80,
        "severity": "low",
        "findings": [
          {
            "id": "tp-1",
            "severity": "moderate",
            "title": "Third-party script impact",
            "description": "4 third-party scripts contribute 320ms to main thread blocking time.",
            "resources": [
              {"url": "https://www.googletagmanager.com/gtag.js", "size_bytes": 89000, "blocking_time_ms": 120, "origin": "google"},
              {"url": "https://cdn.segment.com/analytics.js", "size_bytes": 67000, "blocking_time_ms": 95, "origin": "segment"},
              {"url": "https://js.intercomcdn.com/shim.js", "size_bytes": 145000, "blocking_time_ms": 105, "origin": "intercom"}
            ],
            "recommendation": "Load third-party scripts with `async` or `defer`. Consider delaying non-essential scripts (analytics, chat widgets) until after page interactive.",
            "estimated_impact_ms": 200
          }
        ]
      },
      "caching": {
        "score": 55,
        "severity": "high",
        "findings": [
          {
            "id": "cache-1",
            "severity": "high",
            "title": "Resources served without effective cache policy",
            "description": "12 static resources lack Cache-Control headers or use short max-age values (<3600s).",
            "resources": [
              {"url": "/js/app.bundle.js", "cache_control": "no-cache", "size_bytes": 520000},
              {"url": "/css/main.css", "cache_control": "max-age=300", "size_bytes": 89000}
            ],
            "recommendation": "Set `Cache-Control: max-age=31536000, immutable` for hashed/fingerprinted assets. Use `Cache-Control: no-cache` only for HTML documents.",
            "estimated_impact_ms": 0
          }
        ]
      },
      "compression": {
        "score": 85,
        "severity": "low",
        "findings": [
          {
            "id": "comp-1",
            "severity": "moderate",
            "title": "Uncompressed text resources",
            "description": "3 text resources are served without gzip/brotli compression.",
            "resources": [
              {"url": "/api/config.json", "size_bytes": 45000, "potential_compressed_bytes": 8100},
              {"url": "/data/translations.json", "size_bytes": 120000, "potential_compressed_bytes": 18000}
            ],
            "recommendation": "Enable gzip or brotli compression on the server for all text-based responses (application/json, text/html, text/css, application/javascript).",
            "estimated_impact_ms": 50
          }
        ]
      }
    },
    "web_vitals": {
      "fcp": {"value_ms": 1850, "assessment": "needs-improvement", "target_ms": 1800},
      "lcp": {"value_ms": 3200, "assessment": "needs-improvement", "target_ms": 2500},
      "cls": {"value": 0.18, "assessment": "needs-improvement", "target": 0.1},
      "inp": {"value_ms": 180, "assessment": "good", "target_ms": 200}
    },
    "top_opportunities": [
      {"finding_id": "rb-1", "category": "render_blocking", "estimated_impact_ms": 350, "effort": "medium"},
      {"finding_id": "img-1", "category": "images", "estimated_impact_ms": 400, "effort": "low"},
      {"finding_id": "js-1", "category": "javascript", "estimated_impact_ms": 300, "effort": "high"},
      {"finding_id": "rb-2", "category": "render_blocking", "estimated_impact_ms": 250, "effort": "low"},
      {"finding_id": "tp-1", "category": "third_party", "estimated_impact_ms": 200, "effort": "low"}
    ]
  }
}
```

## How It Works

### Data Collection

The audit synthesizes data from multiple sources, most of which are already captured by the extension:

| Data Source | Already Captured? | Used For |
|------------|-------------------|----------|
| Performance snapshots | Yes (ring buffer) | Timing metrics, Web Vitals, network summary |
| Network waterfall | Yes (ring buffer) | Resource sizes, compression, cache headers, third-party identification |
| Resource timing | Yes (in performance snapshot) | Transfer sizes, durations, render-blocking flags |
| DOM metrics | **New** (async query) | Node count, max depth, max children, deepest path |
| Image analysis | **New** (async query) | Missing dimensions, format detection, rendered vs intrinsic size |
| Head element analysis | **New** (async query) | Render-blocking scripts/styles in `<head>` |

The three "new" data points are collected via the existing async command infrastructure (`analyze({what: "dom"})` internally) -- the extension executes a JavaScript snippet in the page context that queries the DOM and returns structured metrics. This reuses the same async pattern as `interact({what: "execute_js"})` but is scoped to read-only DOM queries.

### Analysis Pipeline

When the AI calls `generate({what: "performance_audit"})`:

1. **Retrieve snapshot**: Get the latest (or URL-specific) performance snapshot from the ring buffer.
2. **Retrieve waterfall**: Get network waterfall entries for the same page URL.
3. **Dispatch DOM queries**: Send async DOM queries to the extension for DOM size, image analysis, and head element analysis.
4. **Wait for results**: Poll for DOM query results (uses existing async command timeout window).
5. **Analyze categories**: Run each requested category's analysis function against the collected data.
6. **Score and rank**: Compute per-category scores and rank findings by estimated impact.
7. **Generate response**: Assemble the structured JSON response with findings, recommendations, and top opportunities.

If DOM queries time out (extension disconnected, tab navigated away), the audit proceeds with the categories that do not require DOM data. Categories that require DOM data are returned with `"score": null` and a note explaining that DOM analysis was unavailable.

### Scoring

Each category is scored 0-100 based on the severity and count of findings:

- **100**: No findings. The category passes.
- **85-99**: Minor findings only (low severity).
- **50-84**: Moderate findings present.
- **0-49**: Critical findings present.

The overall score is the weighted average of category scores:

| Category | Weight | Rationale |
|----------|--------|-----------|
| render_blocking | 25% | Directly impacts FCP and LCP |
| javascript | 20% | Affects load time, interactivity, and TBT |
| images | 15% | Common source of large payload |
| caching | 15% | Repeat visit performance |
| dom_size | 10% | Runtime rendering performance |
| css | 5% | Typically smaller impact than JS |
| third_party | 5% | Outside developer's direct control |
| compression | 5% | Server configuration, usually easy to fix |

### Estimated Impact

Each finding includes an `estimated_impact_ms` field representing the approximate time savings if the issue is fixed. These are heuristic estimates based on:

- **Render-blocking resources**: Estimated from the resource's blocking time as measured by the browser's resource timing API.
- **Image optimization**: Estimated from transfer size reduction at typical compression ratios (WebP ~40% smaller than PNG, ~25% smaller than JPEG).
- **JavaScript bundles**: Estimated from the parse/compile time savings for reduced payload (approximately 1ms per 10KB on mobile, 0.5ms per 10KB on desktop).
- **Caching**: No direct time savings on first load (impact is on repeat visits), so reported as 0ms.

These estimates are clearly approximations. The audit does not run synthetic benchmarks.

## Relationship to Existing Tools

| Existing Tool | What It Does | How Performance Audit Differs |
|--------------|-------------|------------------------------|
| `observe({what: "performance"})` | Returns raw timing metrics and network summary | Audit analyzes *why* metrics are bad and recommends fixes |
| `observe({what: "vitals"})` | Returns Core Web Vitals with pass/fail grades | Audit explains *what causes* poor vitals and how to improve them |
| `observe({what: "network_waterfall"})` | Returns raw resource timing entries | Audit categorizes resources by impact (render-blocking, uncompressed, uncached) |
| `observe({what: "third_party_audit"})` | Lists third-party scripts by origin | Performance audit focuses on third-party *performance impact* (blocking time) rather than security/privacy |
| Causal diffing (performance regression) | Compares current vs. baseline to find regressions | Audit evaluates absolute quality regardless of baseline -- useful for new pages or first-time analysis |

The performance audit is complementary to these tools. A typical AI workflow would be:

1. `generate({what: "performance_audit"})` -- get a comprehensive diagnosis.
2. Fix the identified issues in code.
3. `observe({what: "performance"})` -- verify the fix improved the metrics.
4. `observe({what: "vitals"})` -- confirm Web Vitals are now in the "good" range.

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Analyze render-blocking resources (CSS and JS in `<head>` without async/defer) and report blocking time | must |
| R2 | Analyze DOM size (total nodes, max depth, max children, deepest path) | must |
| R3 | Analyze images (missing dimensions, format opportunities, oversized images) | must |
| R4 | Analyze JavaScript bundles (total size, per-bundle size, estimated unused percentage) | must |
| R5 | Analyze CSS payload (total size, per-stylesheet size) | should |
| R6 | Identify third-party script performance impact (blocking time per origin) | should |
| R7 | Evaluate cache policy effectiveness (missing/short Cache-Control on static assets) | should |
| R8 | Detect uncompressed text resources (missing gzip/brotli based on transfer vs decoded size) | should |
| R9 | Produce per-category scores (0-100) and an overall weighted score | must |
| R10 | Include actionable fix recommendations for each finding | must |
| R11 | Include estimated impact in milliseconds for each finding | should |
| R12 | Support filtering by category via the `categories` parameter | must |
| R13 | Rank top opportunities by estimated impact | must |
| R14 | Include current Web Vitals with assessments in the response | must |
| R15 | Gracefully degrade when DOM queries time out (return available categories with a note) | must |
| R16 | Support URL-specific audits via the `url` parameter | should |
| R17 | Use the existing async command infrastructure for DOM queries (no new HTTP endpoints) | must |

## Non-Goals

- This feature does NOT run synthetic benchmarks or Lighthouse itself. It performs static analysis of captured telemetry and DOM state. It does not simulate throttled network conditions or generate traffic.

- This feature does NOT provide JavaScript coverage data. Estimating unused JavaScript is heuristic-based (comparing bundle sizes to typical coverage ratios). Precise coverage requires Chrome DevTools Protocol integration, which is out of scope.

- This feature does NOT analyze server-side performance (database queries, API response generation time). It only analyzes what the browser observes. Backend latency appears indirectly via TTFB and API response times in the waterfall.

- This feature does NOT produce a visual report or HTML output. The output is structured JSON for AI consumption. Rendering a human-readable report is the AI's responsibility (or a future feature).

- This feature does NOT persist audit results. Each call generates a fresh audit from current data. The AI can use `configure({what: "store"})` to persist results if needed.

- Out of scope: font optimization analysis, preconnect/preload hint suggestions, HTTP/2 multiplexing analysis. These may be added as future findings within the existing category structure.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Audit generation (server-side analysis) | < 50ms |
| DOM query round-trip (extension) | < 2s (within existing async timeout) |
| Total audit response time (including DOM queries) | < 5s |
| Memory impact of audit computation | < 500KB (transient, freed after response) |
| Impact on browsing performance (DOM queries) | < 5ms page-thread time |

The server-side analysis is pure computation over data already in memory (ring buffers). The dominant latency is the DOM query round-trip to the extension. If DOM queries are not needed (categories that only use waterfall/snapshot data), the audit completes in under 50ms.

## Security Considerations

- **Data scope**: The audit reads the same data already captured by the extension (performance snapshots, network waterfall). DOM queries collect node counts and element selectors, not text content or user data.

- **DOM queries**: The JavaScript executed in page context for DOM analysis is read-only (querySelectorAll, document.getElementsByTagName, etc.). It does not modify the DOM, execute user scripts, or access cookies/storage.

- **URL exposure**: Resource URLs appear in the audit response. These may contain path parameters but sensitive query strings should already be stripped by the extension's privacy layer.

- **No new attack surface**: The audit uses existing data flows (ring buffers, async command infrastructure). No new HTTP endpoints or extension permissions are required.

- **Third-party identification**: Third-party resources are identified by comparing resource origins against the page origin. Origin information is already captured in the network waterfall.

## Edge Cases

- **No performance snapshot available**: If no performance snapshot exists (page not yet loaded or buffer evicted), the audit returns an error: `"No performance data available. Navigate to a page and wait for it to load."` This matches the behavior of `observe({what: "performance"})`.

- **Extension disconnected during DOM queries**: DOM queries time out. The audit returns results for categories that do not require DOM data (javascript, caching, compression, third_party) and marks DOM-dependent categories (dom_size, images, render_blocking) with `"score": null` and `"error": "DOM query timed out -- extension may be disconnected"`.

- **No network waterfall entries**: If the waterfall buffer is empty, categories that depend on waterfall data (caching, compression, third_party) are marked as unavailable. The audit still reports findings from the performance snapshot (timing, vitals).

- **SPA with no full page load**: Performance snapshots are captured on page load. For SPAs that never do a full reload, the snapshot may be stale. The audit includes the snapshot timestamp; the AI can decide whether the data is fresh enough.

- **Page with no images**: The images category returns `"score": 100` with zero findings. This is correct behavior, not an edge case to handle specially.

- **Very large DOM (10,000+ nodes)**: The DOM query script uses efficient counting (document.getElementsByTagName('*').length) rather than recursive traversal to avoid blocking the main thread. Max depth and deepest path calculations use bounded traversal (max depth 50).

- **Audit called with invalid category**: Returns a structured error listing valid categories, matching the pattern used by other generate modes.

- **Concurrent audit requests**: Each audit is a stateless computation. Multiple concurrent audits are safe because they read from ring buffers under RLock and produce independent responses.

## Dependencies

- **Depends on:**
  - Performance snapshots (shipped) -- Timing metrics, Web Vitals, network summary, resource entries.
  - Network waterfall (shipped) -- Resource-level timing, transfer sizes, cache headers.
  - Async command infrastructure (shipped) -- DOM query execution in page context.
  - Resource timing with render-blocking flag (shipped) -- Identifying render-blocking resources.

- **Optionally composes with:**
  - `observe({what: "performance"})` (shipped) -- AI uses this after fixing issues to verify improvement.
  - `observe({what: "third_party_audit"})` (shipped) -- Provides complementary security/privacy analysis of third parties.
  - Performance budgets via `configure({what: "health"})` (shipped) -- Budget thresholds can inform scoring (a category that violates a budget gets a harsher score).

- **Depended on by:**
  - None currently. This is a standalone analysis tool.

## Assumptions

- A1: The extension is connected and tracking a tab with a loaded page.
- A2: At least one performance snapshot exists in the ring buffer for the target URL.
- A3: The network waterfall contains entries for the same page (entries are correlated by pageURL).
- A4: The browser tab is still open and the page DOM is accessible for DOM queries (tab has not been closed or navigated away between snapshot capture and audit request).
- A5: The extension's inject.js is running in the page context and can execute read-only DOM queries.
- A6: Resource timing entries include the `renderBlocking` property (available in Chrome 107+, which is well within the extension's browser support range).

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Should the unused JavaScript estimation use Coverage API data if available, or is heuristic estimation sufficient? | open | Coverage API requires DevTools Protocol integration, which is a significant scope expansion. Heuristic estimation (based on bundle size vs. executed function count) may be inaccurate but is implementable within the current architecture. |
| OI-2 | Should the audit include a "font optimization" category (font-display, preloading, subsetting)? | open | Font optimization is a common performance recommendation but adds complexity. Could be added as a future category without changing the audit structure. |
| OI-3 | Should the DOM query for images check for lazy-loading attributes (`loading="lazy"`) on below-the-fold images? | open | Requires determining the viewport boundary, which adds complexity to the DOM query. Could be a valuable finding for LCP optimization. |
| OI-4 | How should the `estimated_impact_ms` be calibrated -- should it assume mobile or desktop conditions? | open | Lighthouse uses simulated mobile throttling. Kaboom observes real conditions. Estimates could be annotated with the assumed environment or provide both mobile/desktop estimates. |
| OI-5 | Should the audit response include a `diff` field comparing the current audit against a previous audit to show improvement over time? | open | Useful for iterative optimization workflows. Would require storing previous audit results, which conflicts with the stateless design. The AI could store results via `configure({what: "store"})` and diff manually. |
