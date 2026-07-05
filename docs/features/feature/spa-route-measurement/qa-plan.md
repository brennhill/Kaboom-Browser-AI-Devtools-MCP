---
status: proposed
scope: feature/spa-route-measurement/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-spa-route-measurement
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: SPA Route Measurement

> QA plan for the SPA Route Measurement feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Route paths expose user-specific data | Verify route normalization replaces user IDs (`/user/123` -> `/user/:id`) before storage and reporting | high |
| DL-2 | Fetch request URLs contain auth tokens | Verify network requests collected during transitions strip query-string auth tokens (e.g., `?token=abc`) | critical |
| DL-3 | Route transition history reveals user navigation patterns | Verify per-route metrics aggregate data (averages), not individual user sessions or click sequences | medium |
| DL-4 | Source/destination routes reveal internal app structure | Verify route paths are the same URLs visible in the browser address bar, not internal router state | low |
| DL-5 | Fetch request bodies during transition | Verify that network request bodies captured during SPA transitions follow the same body-capture opt-in rules | critical |
| DL-6 | Component render time data from React DevTools | Verify component names from React fiber data do not expose internal business logic naming | medium |
| DL-7 | Slowest URL in network data leaks API secrets | Verify `slowest_url` field in transition data is a normalized URL path without query parameters containing secrets | high |
| DL-8 | DOM mutation counts reveal content patterns | Verify only counts are stored (nodes added/removed), not actual DOM content or text | medium |

### Negative Tests (must NOT leak)
- [ ] No user IDs, UUIDs, or slugs appear in stored route paths (must be normalized to `:id`, `:uuid`, `:slug`)
- [ ] No auth tokens in fetch request URLs collected during transitions
- [ ] No request/response bodies in route transition data (only counts and sizes)
- [ ] No actual DOM text content in mutation tracking (only node counts)
- [ ] No individual user navigation sequences reconstructable from aggregate metrics
- [ ] No React component source code paths beyond component names

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | TTI meaning is clear | AI understands "time-to-interactive" is a composite metric (network + render + main thread quiescence), not just "page loaded" | [ ] |
| CL-2 | "timed out" vs "slow" distinction | AI understands `status: "timed_out"` at 10s means measurement gave up, not that the route took exactly 10s | [ ] |
| CL-3 | Route normalization is transparent | AI understands `/user/:id` represents all `/user/123`, `/user/456` etc., not a literal path with a colon | [ ] |
| CL-4 | avg_tti_ms vs last_tti_ms | AI knows `avg_tti_ms` is the average across observations, `last_tti_ms` is the most recent single measurement | [ ] |
| CL-5 | transitions_observed count | AI understands this counts how many times the route was visited, not the number of routes in the app | [ ] |
| CL-6 | "No routes measured" response | AI understands empty routes means "this is a traditional MPA, not an SPA" or "no navigation occurred yet" | [ ] |
| CL-7 | CLS during transition scope | AI understands CLS is scoped to the transition window, not the entire page lifetime CLS | [ ] |
| CL-8 | "cancelled" transition meaning | AI understands cancelled means "user navigated away before route finished loading" not "navigation failed" | [ ] |
| CL-9 | slowest_route identification | AI correctly interprets the `slowest_route` field as the route with highest avg_tti_ms | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might think `avg_tti_ms: 0` means instant -- verify this would not occur (minimum measurement is non-zero)
- [ ] AI might confuse route transitions with page loads -- verify summary language distinguishes them
- [ ] AI might not understand that `:id` in route paths is a normalization placeholder -- verify it is explained in summary or docs
- [ ] AI might interpret `network_requests: 3` as "3 unique endpoints" when it means "3 fetch calls during transition" -- verify field description
- [ ] AI might assume background tab measurements are included in averages -- verify they are excluded

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| View all route metrics | 1 step: `get_route_metrics()` | No -- already minimal |
| View specific route | 1 step: `get_route_metrics(route: "/dashboard")` | No -- already minimal |
| View route history | 1 step: `get_route_metrics(include_history: true)` | No -- already minimal |
| Detect slowest route | 0 steps: `slowest_route` field included automatically | No -- already zero-config |
| Activate measurement | 0 steps: automatic when SPA navigations occur | No -- fully automatic |

### Default Behavior Verification
- [ ] Feature activates automatically when pushState/popstate fires (no opt-in)
- [ ] Route normalization happens automatically (no configuration)
- [ ] Baseline computed automatically after 3+ observations
- [ ] Hash-based routing detected automatically
- [ ] Background tab transitions excluded automatically
- [ ] Query-param-only changes ignored automatically

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | pushState navigation detected | `history.pushState({}, "", "/dashboard")` | Transition recorded: source=current, destination="/dashboard" | must |
| UT-2 | popstate navigation detected | Browser back button fires popstate event | Transition recorded with correct source/destination | must |
| UT-3 | replaceState navigation detected | `history.replaceState({}, "", "/settings")` | Transition recorded | must |
| UT-4 | hashchange navigation detected | URL changes from `/#/home` to `/#/dashboard` | Transition recorded via hashchange event | must |
| UT-5 | TTI measurement - normal case | Network quiescent after 300ms, render quiescent after 200ms, main thread clear | TTI recorded accurately as time from start to all three quiescent | must |
| UT-6 | TTI measurement - timeout | Route has persistent polling (never quiescent) | TTI = 10000ms, timed_out = true | must |
| UT-7 | Route normalization - numeric ID | `/user/123` and `/user/456` | Both normalized to `/user/:id` | must |
| UT-8 | Route normalization - UUID | `/item/550e8400-e29b-41d4-a716-446655440000` | Normalized to `/item/:uuid` | must |
| UT-9 | Route normalization - slug | `/post/my-blog-title` and `/post/another-title` | Both normalized to `/post/:slug` after observing variation | must |
| UT-10 | Route normalization - mixed segments | `/api/v2/items/456/comments` | `/api/v2/items/:id/comments` | must |
| UT-11 | Fetch requests during transition | 3 fetch calls started after nav start, before network quiescence | `network_requests: 3` with correct total bytes | must |
| UT-12 | CLS during transition | Layout shifts totaling 0.05 during render | `avg_cls_during_transition: 0.05` | must |
| UT-13 | Long tasks during transition | Two 80ms tasks blocking main thread | `avg_long_task_ms: 160` (total blocking time) | must |
| UT-14 | DOM mutations counting | 50 nodes added, 10 removed | Mutation counts recorded correctly | should |
| UT-15 | Baseline computation at 3 observations | 3 transitions to same route: 500ms, 700ms, 600ms | avg_tti_ms: 600, p95 computed | must |
| UT-16 | Cancelled navigation | pushState to /A, then immediately pushState to /B | /A transition marked "cancelled", only /B measured | must |
| UT-17 | Same-route navigation | `/user/1` -> `/user/2` (same normalized route) | Treated as new transition, baseline updated for `/user/:id` | should |
| UT-18 | Query-param-only change ignored | `/items?page=1` -> `/items?page=2` via replaceState | NOT treated as route transition | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Extension -> Server route data flow | Extension detects pushState -> sends data via WS -> Server stores route metrics | Route metrics available via `get_route_metrics` | must |
| IT-2 | Route metrics + performance baseline | Route measured -> baseline computed -> regression detected | Route regression triggers push notification | should |
| IT-3 | Multiple routes in single session | Navigate /home -> /dashboard -> /settings -> /home | All 3 routes have metrics, /home has 2 observations | must |
| IT-4 | Route metrics after server restart | Routes observed, server restarted | Route metrics cleared (session-scoped) | must |
| IT-5 | Concurrent tab route measurements | Tab A navigates /dashboard, Tab B navigates /settings simultaneously | Both routes measured independently, no cross-contamination | should |
| IT-6 | Route metrics + behavioral baseline | Save baseline with route perf data, degrade route, compare | Route performance regression detected in baseline comparison | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | pushState interception overhead | Time added to pushState call | < 0.01ms | must |
| PT-2 | MutationObserver sampling overhead | CPU time per 100ms sample interval | < 0.1ms | must |
| PT-3 | Route metrics storage | Memory for 50 routes x 10 history entries | < 100KB | must |
| PT-4 | Quiescence detection CPU usage | CPU usage during idle (no navigation in progress) | Zero (timeout-based, not polling) | must |
| PT-5 | Navigation speed impact | Page transition time with/without measurement | No measurable difference (async, non-blocking) | must |
| PT-6 | get_route_metrics response time | Time to compute and return metrics for 50 routes | < 5ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Traditional MPA (no SPA routing) | Page with no pushState/popstate events | No route metrics generated, feature invisible | must |
| EC-2 | Redirect chain | pushState("/temp") -> immediate replaceState("/final") | Only "/final" recorded as destination | must |
| EC-3 | Background tab navigation | Tab is hidden during route transition | Transition marked "background", excluded from baselines | must |
| EC-4 | Rapid route switching (5 routes in 1 second) | User clicks through 5 nav items quickly | Only last navigation measured, first 4 cancelled | must |
| EC-5 | Route with persistent WebSocket | Route has always-open WS connection | Lenient mode ignores WS for quiescence check | should |
| EC-6 | SSR prefetch navigation (Next.js) | Prefetch fetch happens before pushState | Prefetch excluded from transition network requests | should |
| EC-7 | Infinite scroll via URL update | replaceState changes `?page=2` only | NOT treated as route transition | must |
| EC-8 | Very long route path (100+ chars) | `/a/b/c/d/.../z/123/456` deeply nested route | Normalization handles correctly, no truncation | should |
| EC-9 | Route with animation loop | CSS animation running continuously post-navigation | Animation does not prevent quiescence (only blocking main thread tasks count) | should |
| EC-10 | Hash-based + pushState hybrid | App uses both `/#/old-route` and `/new-route` patterns | Both detected correctly | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A SPA test app running (React, Vue, or Next.js with client-side routing)
- [ ] At least 3 distinct routes available in the test app (e.g., /home, /dashboard, /settings)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Navigate to /home in the SPA | URL bar shows /home, page content renders | Initial page loaded | [ ] |
| UAT-2 | Click navigation link to /dashboard | URL changes to /dashboard via pushState, content updates | SPA navigation occurs | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "vitals", "mode": "route_metrics"}}` | AI receives route data | At least 1 route (/dashboard) with measured TTI | [ ] |
| UAT-4 | Navigate to /settings, then back to /dashboard | Two more SPA navigations | URL changes without page reload | [ ] |
| UAT-5 | `{"tool": "observe", "arguments": {"what": "vitals", "mode": "route_metrics"}}` | AI receives updated metrics | /dashboard has transitions_observed: 2, /settings has 1 | [ ] |
| UAT-6 | Navigate to /dashboard 2 more times | Additional route transitions | 4 total observations for /dashboard | [ ] |
| UAT-7 | `{"tool": "observe", "arguments": {"what": "vitals", "mode": "route_metrics", "include_history": true}}` | AI receives full history | /dashboard has baseline computed (3+ observations), history entries visible | [ ] |
| UAT-8 | Verify slowest route | Check `slowest_route` field in response | Correctly identifies the route with highest avg_tti_ms | [ ] |
| UAT-9 | Navigate to a route with user ID, e.g., /user/123 | URL shows /user/123 | Dynamic route visited | [ ] |
| UAT-10 | Navigate to /user/456 | URL shows /user/456 | Different user, same route pattern | [ ] |
| UAT-11 | `{"tool": "observe", "arguments": {"what": "vitals", "mode": "route_metrics"}}` | AI receives metrics | Both visits normalized to `/user/:id` with transitions_observed: 2 | [ ] |
| UAT-12 | Use browser back button | URL changes to previous route | popstate event fires | [ ] |
| UAT-13 | `{"tool": "observe", "arguments": {"what": "vitals", "mode": "route_metrics"}}` | AI receives metrics | Back navigation recorded as a transition | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Route paths normalized | Check route metrics for `/user/:id` instead of `/user/123` | No user-specific IDs in route paths | [ ] |
| DL-UAT-2 | No auth tokens in fetch URLs | Inspect network_requests data during transition | Query params with tokens stripped or absent | [ ] |
| DL-UAT-3 | No DOM content in mutations | Check transition data for mutation tracking | Only counts (nodes_added, nodes_removed), no text content | [ ] |
| DL-UAT-4 | Summary does not reveal user patterns | Inspect `summary` field | Aggregated metrics only, no individual navigation sequences | [ ] |

### Regression Checks
- [ ] Traditional (non-SPA) pages still work without errors (feature is invisible)
- [ ] Existing `observe(what: "vitals")` performance data still works alongside route metrics
- [ ] Extension does not degrade page navigation speed
- [ ] Extension memory usage stable after 100+ route transitions
- [ ] Server memory bounded at max 50 routes x 10 history entries

---

## Sign-Off

| Area | Tester | Date | Pass/Fail |
|------|--------|------|-----------|
| Data Leak Analysis | | | |
| LLM Clarity | | | |
| Simplicity | | | |
| Code Tests | | | |
| UAT | | | |
| **Overall** | | | |
