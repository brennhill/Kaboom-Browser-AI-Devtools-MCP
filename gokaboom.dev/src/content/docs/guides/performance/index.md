---
title: "Performance Measurement & Refinement"
description: "Use KaBOOM to measure Web Vitals, profile page loads, compare before/after performance, detect regressions, analyze resource loading, and generate PR performance summaries."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'performance']
---

KaBOOM captures Core Web Vitals, navigation timing, resource loading, and long tasks from every page your AI visits. Use this data to measure performance, identify bottlenecks, and verify optimizations — all without leaving your AI workflow.

## Web Vitals

Get a snapshot of Core Web Vitals for the current page:

```js
observe({what: "vitals"})
```

Returns the latest metrics with Google's standard ratings:

| Metric | What It Measures | Good | Needs Improvement | Poor |
|--------|-----------------|------|-------------------|------|
| **FCP** | First Contentful Paint — time to first visible content | ≤ 1.8s | 1.8–3.0s | > 3.0s |
| **LCP** | Largest Contentful Paint — time to main content visible | ≤ 2.5s | 2.5–4.0s | > 4.0s |
| **CLS** | Cumulative Layout Shift — visual stability | ≤ 0.1 | 0.1–0.25 | > 0.25 |
| **INP** | Interaction to Next Paint — input responsiveness | ≤ 200ms | 200–500ms | > 500ms |

Each metric includes the raw value and a `good` / `needs_improvement` / `poor` rating.

### When to use `vitals` vs `performance`

- **`vitals`** — quick check of the latest Core Web Vitals. Fast, focused, ideal for "how does this page perform right now?"
- **`performance`** (via `analyze`) — full performance snapshots including navigation timing, network breakdown, long tasks, and resource details. Use this for deep analysis.

---

## Full Performance Snapshots

```js
analyze({what: "performance"})
```

Returns all captured performance snapshots (up to 20 retained). Each snapshot includes:

### Navigation Timing

| Metric | Description |
|--------|-------------|
| `time_to_first_byte` | Server response time (TTFB) |
| `first_contentful_paint` | First visible content rendered |
| `largest_contentful_paint` | Main content visible |
| `dom_interactive` | DOM ready for interaction |
| `dom_content_loaded` | DOMContentLoaded event fired |
| `load` | Full page load event fired |
| `interaction_to_next_paint` | Worst-case input responsiveness |

### Network Summary

Aggregated resource loading data broken down by type:

| Category | What's Counted |
|----------|---------------|
| `script` | JavaScript files |
| `style` | CSS files |
| `image` | Images (PNG, JPEG, SVG, WebP, etc.) |
| `font` | Web fonts (WOFF2, WOFF, TTF) |
| `fetch` | XHR and Fetch API calls |
| `other` | Everything else |

Per category: request count, transfer size (compressed), decoded size (uncompressed).

Plus the **slowest requests** — the top resources by duration, so you immediately see what's dragging down the waterfall.

### Long Task Metrics

Tasks that block the main thread for more than 50ms are captured:

| Metric | Description |
|--------|-------------|
| `count` | Number of long tasks during page load |
| `total_blocking_time` | Sum of (task duration - 50ms) for each long task |
| `longest_task` | Duration of the single longest task |

High total blocking time directly impacts INP. If your page has 500ms of blocking time, user interactions will feel sluggish.

### User Timing

If your application uses `performance.mark()` and `performance.measure()`, KaBOOM captures those too:

```js
// Your application code
performance.mark('api-call-start');
await fetchData();
performance.mark('api-call-end');
performance.measure('api-call', 'api-call-start', 'api-call-end');
```

These custom marks and measures appear in the performance snapshot alongside browser metrics. Up to 200 entries retained per type.

---

## Before/After Comparison (perf_diff)

The most powerful performance feature: automatic before/after comparison when navigating or refreshing.

### How It Works

When you use `interact` to navigate or refresh:

```js
interact({what: "refresh"})
interact({what: "navigate", url: "https://myapp.com/dashboard"})
```

KaBOOM automatically:
1. Stashes a **before** performance snapshot
2. Executes the navigation or refresh
3. Captures an **after** performance snapshot
4. Computes a **perf_diff** — available in the async result

### Reading the Diff

The perf_diff includes:

**Verdict** — one of:
- `improved` — more metrics improved than regressed
- `regressed` — more metrics regressed than improved
- `mixed` — some improved, some regressed
- `unchanged` — no meaningful change

**Per-metric comparison**:

| Field | Description |
|-------|-------------|
| `before` | Value before the action |
| `after` | Value after |
| `delta` | Absolute change |
| `pct` | Percentage change ("+50%", "-10%") |
| `improved` | Whether the change is better |
| `rating` | Web Vitals rating (good/needs_improvement/poor) |
| `unit` | `ms`, `KB`, `count`, or empty for CLS |

**Resource changes**:
- **Added** — new resources not present before
- **Removed** — resources that disappeared
- **Resized** — same resource, size changed by >10% and >1KB
- **Retimed** — same resource, load time changed significantly

**Summary** — human-readable description under 200 characters:
> "LCP improved -25% (good); removed old-script.js; TRANSFER_KB improved -27%"

### Profiling DOM Actions

Add `analyze: true` to any DOM action to get performance profiling:

```js
interact({what: "click", selector: "text=Load Dashboard", analyze: true})
```

This captures before/after snapshots around the action, showing the performance impact of that specific interaction.

---

## Network Waterfall Analysis

See exactly how resources load:

```js
observe({what: "network_waterfall"})
observe({what: "network_waterfall", limit: 50, url: "/api"})
```

Per resource entry:

| Field | Description |
|-------|-------------|
| `name` / `url` | Full resource URL |
| `initiator_type` | What triggered the load (`script`, `style`, `fetch`, `image`, etc.) |
| `duration` | Total load time (ms) |
| `start_time` | When loading started (relative to page navigation) |
| `transfer_size` | Compressed bytes over the network |
| `decoded_body_size` | Uncompressed bytes |

Use this to identify:
- **Render-blocking resources** — scripts and styles with early `start_time` and long `duration`
- **Oversized assets** — large `transfer_size` relative to `decoded_body_size` (or vice versa — poor compression)
- **Waterfall bottlenecks** — sequential chains where one resource depends on another
- **Redundant requests** — same resource loaded multiple times

The waterfall data refreshes on demand — if it's more than 1 second stale, KaBOOM requests fresh data from the extension.

---

## Timeline View

See performance events in context with everything else that happened:

```js
observe({what: "timeline"})
observe({what: "timeline", include: ["network", "errors"]})
```

The timeline merges:
- **Network requests** with timing and size
- **User actions** (clicks, navigation, input)
- **Errors** (console errors, exceptions)
- **WebSocket events** (messages, connections)

Sorted chronologically, newest first. Default limit of 50 entries.

This is invaluable for understanding **what caused** a performance issue. If LCP spiked, the timeline shows what happened right before — maybe a failed API call triggered a retry cascade, or a user action loaded an expensive component.

---

## Performance Budgets

Configure thresholds in a `.kaboom.json` file at your project root:

```json
{
  "budgets": {
    "default": {
      "load_ms": 2000,
      "fcp_ms": 1800,
      "lcp_ms": 2500,
      "cls": 0.1,
      "inp_ms": 200,
      "ttfb_ms": 800,
      "total_transfer_kb": 500,
      "script_transfer_kb": 300
    },
    "routes": {
      "/login": { "load_ms": 1000 },
      "/dashboard": { "load_ms": 3000 }
    }
  }
}
```

When a performance snapshot exceeds a budget, KaBOOM reports the violation:

```json
{
  "type": "budget_exceeded",
  "url": "/dashboard",
  "violations": [
    {
      "metric": "total_transfer_kb",
      "budget": 500,
      "actual": 620,
      "over_by": "120KB (24%)"
    }
  ]
}
```

### Built-In Presets

| Preset | FCP | LCP | CLS | INP | Load | TTFB |
|--------|-----|-----|-----|-----|------|------|
| `web-vitals-good` | 1800ms | 2500ms | 0.1 | 200ms | — | — |
| `web-vitals-needs-improvement` | 3000ms | 4000ms | 0.25 | 500ms | — | — |
| `performance-budget-default` | — | — | — | — | 3000ms | 600ms |

Route-specific budgets override defaults using longest-prefix matching. The config file is reloaded every 30 seconds.

---

## Workflow: Performance Optimization Cycle

### 1. Measure the Baseline

```
"Navigate to the dashboard and show me the Web Vitals."
```

```js
interact({what: "navigate", url: "https://myapp.com/dashboard"})
observe({what: "vitals"})
```

### 2. Identify Bottlenecks

```
"Show me the full performance snapshot — I want to see what's slow."
```

```js
analyze({what: "performance"})
```

Look for:
- High TTFB → server-side issue
- Large gap between TTFB and FCP → render-blocking CSS/JS
- High LCP with low FCP → main content loads late (lazy loading too aggressive, or hero image too large)
- High CLS → layout shifts from dynamic content
- High total blocking time → heavy JavaScript execution

### 3. Drill Into the Waterfall

```
"Show me the network waterfall, especially JavaScript files."
```

```js
observe({what: "network_waterfall", url: ".js"})
```

### 4. Make Changes and Compare

```
"Refresh and compare performance."
```

```js
interact({what: "refresh"})
```

The perf_diff tells you exactly what improved, what regressed, and by how much.

### 5. Verify with the Timeline

```
"Show me the timeline around the page load."
```

```js
observe({what: "timeline", include: ["network"]})
```

Confirm that render-blocking resources are gone, API calls are parallelized, or whatever optimization you made is reflected.

### 6. Generate a PR Summary

```
"Generate a performance summary for the PR."
```

```js
generate({what: "pr_summary"})
```

Produces a before/after comparison table suitable for pull request descriptions — Web Vitals deltas, resource changes, and a regression/improvement verdict.

---

## What Gets Captured Automatically

KaBOOM collects performance data passively through browser Performance APIs:

| API | What It Captures | How |
|-----|-----------------|-----|
| `PerformanceNavigationTiming` | Page load timing (TTFB, DOM events, load) | Automatic on navigation |
| `PerformanceResourceTiming` | Per-resource load timing and size | Automatic for all resources |
| `PerformancePaintTiming` | FCP | PerformanceObserver |
| `LargestContentfulPaint` | LCP | PerformanceObserver (continuously updated until interaction) |
| `LayoutShift` | CLS | PerformanceObserver (excludes shifts within 500ms of user input) |
| `Event` timing | INP | PerformanceObserver (groups by interactionId, threshold 40ms) |
| `PerformanceLongTaskTiming` | Long tasks (>50ms) | PerformanceObserver |
| `performance.mark()` / `measure()` | Custom application timing | Wrapped + observed |

No configuration needed. As long as the extension is tracking a tab, performance data flows to the server automatically.

---

## Tips

**Compare across deploys**: Navigate to the same page before and after a deploy. The perf_diff shows exactly what changed.

**Use route-specific budgets**: Your landing page should be fast (1s load). Your admin dashboard can be slower (3s). Set different budgets for different routes.

**Watch total blocking time**: TBT is the most actionable metric for JavaScript performance. If it's high, find the long tasks and split or defer them.

**Check transfer vs decoded size**: If `transfer_size` is close to `decoded_body_size`, your server isn't compressing responses. Enable gzip or brotli.

**Profile specific interactions**: Use `analyze: true` on clicks that trigger expensive operations (opening modals, loading data tables, switching tabs) to measure their impact.
