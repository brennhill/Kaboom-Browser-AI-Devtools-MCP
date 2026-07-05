---
status: proposed
scope: feature/causal-diffing/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-causal-diffing
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Causal Diffing

> QA plan for the Causal Diffing feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Resource URLs expose internal infrastructure | Verify resource URLs in diff output are browser-visible paths, not internal CDN or build-system URLs with tokens | high |
| DL-2 | Query parameters in resource URLs contain secrets | Verify URL normalization strips query params (`main.chunk.js?v=abc123` -> `main.chunk.js`) and removes auth tokens | critical |
| DL-3 | Probable cause text reveals internal architecture | Verify `probable_cause` summary does not expose server-side details beyond what the browser observes | medium |
| DL-4 | Recommendations leak private dependency names | Verify `recommendations` array references public resource paths, not private package names or internal module structure | medium |
| DL-5 | API endpoint paths reveal internal routing | Verify retimed API paths like `/api/dashboard/data` are the same URLs visible in the Network tab, not internal service meshes | medium |
| DL-6 | Baseline resource fingerprint stores response bodies | Verify fingerprint stores only `{ url, type, transferSize, duration, renderBlocking }`, never response content | critical |
| DL-7 | Dynamic API paths with user data | Verify dynamic paths like `/api/user/123/data` are grouped by prefix, not exposing individual user IDs in diff output | high |
| DL-8 | Render-blocking determination leaks DOM structure | Verify render-blocking detection uses Performance API fields, not full DOM traversal results | low |

### Negative Tests (must NOT leak)
- [ ] No query parameters with auth tokens in any resource URL in diff output
- [ ] No response body content in resource fingerprints (sizes and durations only)
- [ ] No internal hostnames or IP addresses in CDN resource entries
- [ ] No user-specific IDs in dynamic API path groupings
- [ ] No build system paths or source map URLs in resource entries
- [ ] No private npm package names in recommendations text

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Four diff categories are distinct | AI understands added/removed/resized/retimed as mutually exclusive categories | [ ] |
| CL-2 | "render_blocking" meaning | AI understands this means "blocks first contentful paint", not "blocks all rendering forever" | [ ] |
| CL-3 | Probable cause vs confirmed cause | AI understands `probable_cause` is an inference, not a guaranteed diagnosis | [ ] |
| CL-4 | Timing delta vs resource changes | AI distinguishes overall timing regression from individual resource-level changes | [ ] |
| CL-5 | "Resource comparison unavailable" | AI understands this means the baseline predates causal diffing, not that comparison failed | [ ] |
| CL-6 | Size in bytes vs kilobytes | AI correctly interprets `size_bytes: 287000` as ~287KB, not 287 bytes | [ ] |
| CL-7 | delta_bytes sign convention | AI understands positive delta_bytes means growth, not improvement | [ ] |
| CL-8 | Retimed vs resized distinction | AI understands retimed resources have similar size but different duration (backend regression), while resized have different size (frontend change) | [ ] |
| CL-9 | Recommendations are actionable suggestions | AI treats recommendations as starting points, not commands | [ ] |
| CL-10 | "No resource changes" message | AI understands this means regression cause is not frontend resources (could be backend/DOM/throttling) | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might assume all added resources are bad -- verify added resources could be legitimate features
- [ ] AI might confuse `delta_pct` (percentage change) with `delta_ms` (absolute change) -- verify units are clear
- [ ] AI might not realize render-blocking applies only to resources loaded before FCP -- verify context is provided
- [ ] AI might think "resource comparison unavailable" means there is a bug -- verify messaging explains legacy baselines
- [ ] AI might assume recommendations are ordered by priority -- verify whether ordering is intentional or arbitrary

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Get causal diff for latest regression | 1 step: `get_causal_diff()` (auto-selects most recent regression) | No -- already minimal |
| Get causal diff for specific URL | 1 step: `get_causal_diff(url: "/dashboard")` | No -- already minimal |
| Full regression diagnosis workflow | 3 steps: detect regression (automatic), call causal diff, read recommendations | No -- inherently requires detection first |
| Compare against specific baseline | 1 step: `get_causal_diff(baseline_id: "...")` | No -- already minimal |

### Default Behavior Verification
- [ ] `get_causal_diff()` with no parameters automatically uses most recently regressed URL
- [ ] Resource fingerprinting happens automatically during performance snapshots (no opt-in)
- [ ] Recommendations generated automatically without configuration
- [ ] Probable cause computed automatically from resource diff
- [ ] Small resources (<1KB) aggregated automatically

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Added resource detection | Baseline: [main.js], Current: [main.js, analytics.js] | `added: [analytics.js]` with size and duration | must |
| UT-2 | Removed resource detection | Baseline: [main.js, old-lib.js], Current: [main.js] | `removed: [old-lib.js]` | must |
| UT-3 | Resized resource - above threshold | main.js: baseline 100KB, current 150KB (50% increase) | `resized: [{url: "main.js", delta_bytes: 50000}]` | must |
| UT-4 | Resized resource - below threshold | main.js: baseline 100KB, current 108KB (8% / 8KB) | NOT in resized (below both 10% and 10KB) | must |
| UT-5 | Retimed resource detection | API endpoint: baseline 50ms, current 500ms | `retimed: [{delta_ms: 450}]` | must |
| UT-6 | Retimed resource - below threshold | API endpoint: baseline 50ms, current 120ms (70ms delta) | NOT in retimed (below 100ms threshold) | must |
| UT-7 | Render-blocking flag - script without async | Script loaded without async/defer | `render_blocking: true` | must |
| UT-8 | Render-blocking flag - async script | Script with async attribute | `render_blocking: false` | must |
| UT-9 | Render-blocking flag - stylesheet before FCP | CSS loaded before First Contentful Paint | `render_blocking: true` (heuristic) | should |
| UT-10 | URL normalization - strip query params | `main.chunk.js?v=abc123` and `main.chunk.js?v=def456` | Treated as same resource | must |
| UT-11 | URL normalization - preserve hash | `vendor.chunk.abc123.js` (content hash in filename) | Hash preserved for matching | must |
| UT-12 | Probable cause - added resources summary | 3 added scripts totaling 500KB | `probable_cause` mentions "Added 500KB in new scripts" | must |
| UT-13 | Probable cause - render-blocking detection | Added render-blocking script | `probable_cause` mentions "render-blocking" | must |
| UT-14 | Probable cause - slow critical path | Retimed resource on critical path (before FCP) | `probable_cause` mentions slow critical-path resource | must |
| UT-15 | Probable cause - no resource changes | Same resources, same sizes, same durations | `probable_cause` mentions "backend responses, DOM complexity, or browser throttling" | must |
| UT-16 | Recommendations generation | Added 400KB render-blocking script | Recommendation: "Consider lazy-loading [script]" | must |
| UT-17 | Dynamic API path grouping | `/api/user/123` and `/api/user/456` | Grouped as same resource by first 2 path segments | must |
| UT-18 | Large resource list capping | 250 resources in snapshot | Only top 50 by transfer size stored | must |
| UT-19 | Small resource aggregation | 20 resources under 1KB each | Aggregated as "20 small resources totaling X KB" | should |
| UT-20 | Exponential moving average update | Baseline has resource at 100KB, new snapshot at 120KB | Baseline updated with EMA, not replaced with 120KB | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end causal diff after regression | Extension snapshot -> regression detected -> `get_causal_diff` called | Full diff output with added/removed/resized/retimed | must |
| IT-2 | Causal diff + push regression integration | Regression alert fires -> AI calls `get_causal_diff` | Diff available with correct URL from regression alert | must |
| IT-3 | Resource fingerprint stored with performance baseline | Performance baseline saved -> resource fingerprint included | Fingerprint persists and is available for comparison | must |
| IT-4 | Extension sends renderBlockingStatus | Extension performance snapshot includes renderBlockingStatus | Server uses the field for render-blocking determination | must |
| IT-5 | No baseline resource list (legacy baseline) | Old baseline without resource data -> new snapshot | Returns timing deltas with "resource comparison unavailable" message | must |
| IT-6 | Multiple sequential diffs | Regression -> diff -> fix -> new regression -> diff | Each diff compares against current baseline, not previous diff | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Resource fingerprint storage size | Memory for 50 resource entries | < 50KB per baseline | must |
| PT-2 | Diff computation speed | Time to diff two 50-entry resource lists | < 5ms | must |
| PT-3 | No additional network requests | Extension overhead for resource data | Zero extra requests (uses existing Performance API) | must |
| PT-4 | Resource list serialization | Size of resource fingerprint in performance snapshot POST | < 10KB for 50 entries | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No baseline exists | `get_causal_diff` called with no baseline | Error message: "No baseline for this URL" | must |
| EC-2 | Baseline without resource fingerprint | Legacy baseline (pre-causal-diffing) | Timing deltas returned, resources marked "unavailable" | must |
| EC-3 | Identical resources (no changes) | Same resource list, same sizes, same durations | All four categories empty, probable_cause mentions backend/DOM | must |
| EC-4 | CDN resources with different hostnames | `cdn1.example.com/jquery.min.js` vs `cdn2.example.com/jquery.min.js` | Treated as separate resources (different hostname) | must |
| EC-5 | Very large added resource (50MB) | 50MB video or data file added | Listed in added with correct size, flagged in recommendations | should |
| EC-6 | Regression with only retimed changes | All resources same, but API 5x slower | `retimed` populated, `probable_cause` focuses on backend regression | must |
| EC-7 | Resource removed AND regression worsened | Removed resource but load time increased | Removed resources listed, probable_cause notes inconsistency | should |
| EC-8 | Same URL, different resource types | CSS file replaced with JS file at same URL | Detected as both removed (CSS) and added (JS) | should |
| EC-9 | Concurrent diff requests | Two AI agents call `get_causal_diff` simultaneously | Both get correct results, no race condition | should |
| EC-10 | Resource with zero transfer size | Cached resource (304 response, 0 bytes transferred) | Handled correctly, not flagged as "removed" | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web app with controllable resource loading (can add/remove scripts, slow down API endpoints)
- [ ] Performance baseline established for the test page

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Load test page, wait for performance snapshot | Network tab shows resources loaded | Baseline performance snapshot captured | [ ] |
| UAT-2 | Establish performance baseline (via behavioral baselines or automatic) | AI confirms baseline stored | Baseline includes resource fingerprint | [ ] |
| UAT-3 | Add a new 200KB JavaScript file to the page | Human adds `<script src="new-lib.js">` to HTML | New resource loads in Network tab | [ ] |
| UAT-4 | Reload page, wait for performance snapshot | Page loads slower with new script | Regression detected (load time increased) | [ ] |
| UAT-5 | `{"tool": "observe", "arguments": {"what": "vitals", "mode": "causal_diff"}}` | AI receives causal diff | Response shows `added: [new-lib.js]` with size ~200KB | [ ] |
| UAT-6 | Verify probable cause | Check `probable_cause` field | Mentions added resource and total payload increase | [ ] |
| UAT-7 | Verify recommendations | Check `recommendations` array | Contains actionable suggestion about the new script | [ ] |
| UAT-8 | Remove the added script, add API latency instead | Human removes script, adds 500ms delay to API endpoint | Network tab shows slow API, no extra scripts | [ ] |
| UAT-9 | Reload page, call `get_causal_diff` | AI receives updated diff | `retimed: [api-endpoint]` with delta_ms ~500, `removed: [new-lib.js]` | [ ] |
| UAT-10 | Verify probable cause for backend regression | Check `probable_cause` field | Mentions slow API endpoint, not frontend resources | [ ] |
| UAT-11 | Restore normal API speed, reload | Human removes API delay | Page loads at baseline speed | [ ] |
| UAT-12 | `{"tool": "observe", "arguments": {"what": "vitals", "mode": "causal_diff"}}` | AI receives diff | No significant resource changes, timing deltas near zero | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No query params with tokens in resource URLs | Inspect `resource_changes` arrays for query strings | URLs stripped to path only (e.g., `main.chunk.js`, not `main.chunk.js?token=...`) | [ ] |
| DL-UAT-2 | No response body content in fingerprint | Inspect stored baseline resource data | Only url, type, transferSize, duration, renderBlocking fields | [ ] |
| DL-UAT-3 | Dynamic API paths grouped, not exposing user IDs | Make requests to `/api/user/123` and `/api/user/456` | Grouped as `/api/user/*` in diff output, no individual IDs | [ ] |
| DL-UAT-4 | Recommendations reference only public paths | Check recommendations text | No internal dependency names, private package refs, or build paths | [ ] |

### Regression Checks
- [ ] Existing performance monitoring still works without causal diff being called
- [ ] Performance baseline save/load unaffected by resource fingerprint addition
- [ ] Extension performance snapshot size not significantly increased by renderBlockingStatus field
- [ ] `get_causal_diff` gracefully handles missing baselines (no crash)
- [ ] Push regression alerts still fire independently of causal diff

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
