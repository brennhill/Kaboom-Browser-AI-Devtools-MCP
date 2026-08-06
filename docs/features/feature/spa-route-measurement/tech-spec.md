---
status: proposed
scope: feature/spa-route-measurement/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-spa-route-measurement
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-spa-route-measurement.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Spa Route Measurement Review](./index.md).

# Technical Spec: SPA Route Measurement

## Purpose

Modern web applications are single-page apps. After the initial page load, navigation between routes happens entirely on the client — React Router, Next.js, Vue Router, and similar libraries call `history.pushState()` or `history.replaceState()`, render new components, fetch data, and update the URL without a full page reload.

Today, Kaboom's performance measurement captures the initial page load but is blind to subsequent route transitions. For an app with 20 routes, the AI sees performance data for one of them (whichever was loaded first). The other 19 are invisible.

SPA route measurement tracks each client-side navigation as a discrete performance event: time-to-interactive for the new route, network requests triggered by the transition, layout shifts during the render, and long tasks that block interaction. The AI gets per-route performance data and can identify which routes are fast and which are slow — even though they're all "the same page" to the browser.

---

## Opportunity & Business Value

**Complete app coverage**: Without SPA measurement, the AI only optimizes the landing page. With it, the AI can identify "the /settings page takes 3 seconds to render because it loads a 2MB user preferences blob on mount." Per-route data turns the AI into a full-app performance consultant.

**Framework-aligned mental model**: Developers think in routes, not page loads. When they say "the checkout page is slow," they mean the `/checkout` route, which might be one component in a massive SPA. SPA measurement speaks the developer's language.

**Regression attribution**: "After adding the analytics dashboard route, the /dashboard → /analytics transition takes 4 seconds" is much more actionable than "the app feels slow sometimes." Per-route baselines make regressions attributable to specific code changes.

**Interoperability with React/Next.js/Vue tooling**: Framework-specific performance tools (React Profiler, Next.js Analytics, Vue Performance Devtool) measure component-level rendering. SPA route measurement provides the higher-level "time to usable" metric that frameworks don't track directly. These are complementary data sources.

**Soft Navigation API alignment**: Chrome's experimental Soft Navigation API (behind a flag since Chrome 110) aims to standardize SPA navigation measurement. Kaboom's implementation can use this API when available and fall back to heuristics when it's not, providing consistent measurement regardless of browser support.

---

## How It Works

### Navigation Detection

The extension intercepts client-side navigations by wrapping two browser APIs:

1. **`history.pushState`**: Wrapped to detect programmatic navigations (most SPA routers use this).
2. **`history.replaceState`**: Wrapped to detect URL updates without new history entries.
3. **`popstate` event**: Listened to detect back/forward browser navigation.

When any of these fire, the extension records the transition start time and the new URL.

### Time-to-Interactive Measurement

After detecting a navigation, the extension measures how long the new route takes to become interactive:

1. **Mark navigation start**: `performance.now()` when pushState/popstate fires
2. **Wait for network quiescence**: No new fetch requests started for 500ms (the route has finished loading its data)
3. **Wait for render quiescence**: No layout shifts or DOM mutations for 200ms (the route has finished rendering)
4. **Wait for main thread quiescence**: No long tasks (>50ms) for 200ms (the route is responsive to interaction)
5. **Mark navigation complete**: `performance.now()` when all three conditions are met simultaneously

The time from start to complete is the route's "time-to-interactive" (TTI).

### Timeout and Fallback

If the quiescence conditions aren't met within 10 seconds, the navigation is marked as "timed out" with the TTI recorded as 10000ms. This prevents infinite waiting for routes that have persistent polling, WebSocket streams, or animation loops.

A more lenient mode ignores ongoing WebSocket activity (which is background communication, not navigation-related rendering).

### Per-Route Data Collection

During the navigation window (start to TTI), the extension collects:

- **Fetch requests**: URLs, methods, sizes, and durations of requests triggered by the navigation (started after navigation start, before network quiescence)
- **Layout shifts**: CLS accumulated during the transition
- **Long tasks**: Tasks that blocked the main thread during the transition
- **DOM mutations**: Count of DOM nodes added/removed (via MutationObserver, sampled at 100ms intervals)
- **Component render time**: If React DevTools fiber data is accessible, the render time of the destination component

### MCP Tool: `get_route_metrics`

**Parameters**:
- `route` (optional): Specific route path to query. If omitted, returns metrics for all observed routes.
- `include_history` (optional, boolean): Include previous transition timings (up to 10 per route).

**Response**:
```
{
  "routes": [
    {
      "path": "/dashboard",
      "transitions_observed": 5,
      "avg_tti_ms": 1200,
      "last_tti_ms": 1150,
      "p95_tti_ms": 1800,
      "network_requests": 3,
      "avg_data_fetched_bytes": 45000,
      "avg_cls_during_transition": 0.02,
      "avg_long_task_ms": 180,
      "status": "measured",
      "last_transition_from": "/settings"
    },
    {
      "path": "/settings",
      "transitions_observed": 2,
      "avg_tti_ms": 800,
      "last_tti_ms": 750,
      "network_requests": 1,
      "avg_data_fetched_bytes": 12000,
      "avg_cls_during_transition": 0,
      "avg_long_task_ms": 50,
      "status": "measured",
      "last_transition_from": "/dashboard"
    }
  ],
  "total_routes_observed": 2,
  "slowest_route": "/dashboard",
  "summary": "2 routes measured. /dashboard averages 1.2s TTI (3 API calls). /settings averages 0.8s TTI."
}
```

---

## Data Model

### Route Transition Entry

Each observed navigation produces:
- Source route (where the user was)
- Destination route (where they're going)
- Transition start timestamp
- TTI (time-to-interactive in ms, or null if timed out)
- Timed out flag
- Network requests during transition: count, total bytes, slowest URL
- CLS during transition
- Long tasks during transition: count, total blocking time
- DOM mutations: nodes added, nodes removed

### Route Baseline

After 3+ observations of the same route, a baseline is computed:
- Average TTI
- P95 TTI
- Average network bytes
- Average CLS
- Average long task blocking time

This baseline feeds into the regression detection system — a route that was 500ms and becomes 2000ms generates a push notification.

### Route Normalization

Dynamic route segments are normalized:
- `/user/123` → `/user/:id`
- `/post/abc-my-title` → `/post/:slug`
- `/api/v2/items/456/comments` → `/api/v2/items/:id/comments`

Normalization uses heuristics: segments that are purely numeric are replaced with `:id`, UUIDs with `:uuid`, and segments that vary between observations of the same route prefix with `:param`.

---

## Edge Cases

- **Hash-based routing** (`/#/dashboard`): Detected via `hashchange` event in addition to pushState/popstate.
- **Same-route navigation** (e.g., `/user/1` → `/user/2`): Treated as a new transition (different data, potentially different render time). Normalized to the same route for baseline purposes.
- **Redirect chains** (pushState → immediate replaceState): The intermediate URL is ignored; only the final destination is recorded.
- **Background tab**: If the tab is hidden during navigation, the transition is marked "background" and excluded from baseline calculations (background tabs are throttled).
- **Concurrent navigations** (rapid route switching before previous finishes): Only the last navigation is measured. Earlier ones are marked "cancelled."
- **Page without SPA routing** (traditional multi-page app): No pushState/popstate events fire, no route metrics are generated. The feature is invisible.
- **Server-side rendered transitions** (Next.js prefetch + navigate): The prefetch fetch is excluded from the transition's network requests (it happened before navigation start).
- **Infinite scroll / pagination via URL update**: replaceState calls that only change query params (`?page=2`) are ignored as route transitions unless the path also changes.

---

## Performance Constraints

- pushState/popstate interception: under 0.01ms per call (record timestamp, post message)
- MutationObserver sampling: 100ms intervals, under 0.1ms per sample (count nodes)
- Quiescence detection: passive (setTimeout-based checks), zero CPU when idle
- Route metrics storage: max 50 routes × 10 history entries = 500 entries, under 100KB
- No impact on navigation speed (observation is asynchronous and non-blocking)

---

## Test Scenarios

1. pushState to new route → transition detected with correct source/destination
2. popstate (back button) → transition detected
3. TTI measured correctly (network + render + main thread quiescence)
4. Navigation timeout at 10s → timed out flag set
5. Fetch requests during transition counted and sized
6. CLS during transition accumulated correctly
7. Long tasks during transition detected
8. Route normalization: `/user/123` → `/user/:id`
9. Hash-based routing detected
10. Same-route navigation treated as new transition
11. Cancelled navigation (rapid switching) → only last measured
12. Background tab → excluded from baseline
13. Baseline computed after 3+ observations
14. `get_route_metrics` returns all routes with averages
15. `get_route_metrics` with specific route filter works
16. Slowest route identified in response
17. No SPA routing on page → no route metrics generated
18. Query-param-only changes ignored

---

## File Locations

Extension implementation: `extension/inject.js` (pushState/popstate interception, quiescence detection, metric collection).

Server implementation: `cmd/browser-agent/performance.go` (route metrics storage, baseline computation, MCP tool handler).

Tests: `extension-tests/spa-routes.test.js` (extension-side), `cmd/browser-agent/performance_test.go` (server-side).
