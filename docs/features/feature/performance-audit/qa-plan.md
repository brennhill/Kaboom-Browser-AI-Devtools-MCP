---
status: proposed
scope: feature/performance-audit/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-performance-audit
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Performance Audit

> QA plan for the Performance Audit feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

**Note:** No tech-spec.md is available for this feature. This QA plan is based solely on the product-spec.md.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Resource URLs in findings may contain sensitive query parameters (tokens, session IDs) | Verify resource URLs in audit findings are stripped of sensitive query parameters by the extension's privacy layer. If not stripped, document this as a known exposure (same as network waterfall). | high |
| DL-2 | DOM queries reveal page structure and content | Verify DOM queries collect only structural metrics (node counts, element types, image src attributes) -- not text content, form values, or user data. | medium |
| DL-3 | Third-party script URLs and origins expose technology stack | Verify third-party identification only reveals what is already visible in the network waterfall. This is acceptable for localhost dev tooling. Document the exposure. | low |
| DL-4 | Cache-Control header values expose server configuration | Verify header values are shown as-is. This is the same data already available in network bodies. Acceptable for localhost. | low |
| DL-5 | Image analysis query reads image src URLs | Verify image analysis collects only `src`, `width`, `height`, `loading` attributes -- not image content or alt text containing sensitive info. | medium |
| DL-6 | Head element analysis reads script/link tags | Verify head analysis collects only structural data (tag types, src URLs, async/defer attributes) -- not inline script content that could contain API keys. | high |
| DL-7 | Audit recommendations reference specific resource paths | Verify recommendation text is generated from templates with URL references, not raw page content. Recommendations should reference resource paths but not request/response bodies. | medium |
| DL-8 | DOM query JavaScript runs in page context | Verify DOM query scripts are read-only (querySelectorAll, getElementsByTagName). No cookie/localStorage access, no DOM modification, no user script execution. | high |

### Negative Tests (must NOT leak)
- [ ] DOM queries must NOT access `document.cookie`, `localStorage`, or `sessionStorage`
- [ ] DOM queries must NOT read inline `<script>` content (which may contain API keys or secrets)
- [ ] DOM queries must NOT read form field values
- [ ] Resource URLs in findings must NOT contain authorization tokens (should be stripped by privacy layer)
- [ ] Audit response must NOT contain request/response body content
- [ ] DOM queries must NOT modify the page DOM in any way

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Overall score is a number 0-100 | `overall_score` is an integer, not a fraction or percentage string. | [ ] |
| CL-2 | Per-category scores are 0-100 | Each category has a `score` field that is an integer 0-100. | [ ] |
| CL-3 | Severity values are consistent | All findings use `"critical"`, `"high"`, `"moderate"`, or `"low"`. No other values. | [ ] |
| CL-4 | estimated_impact_ms is clearly an approximation | Verify `estimated_impact_ms` is documented as heuristic. LLM should not treat it as precise measurement. | [ ] |
| CL-5 | Resource URLs in findings are actionable | URLs point to specific resources the LLM can reference in code fixes. | [ ] |
| CL-6 | Recommendations are concrete and actionable | Each recommendation references specific files/elements and suggests specific code changes (e.g., "add defer attribute"). | [ ] |
| CL-7 | Category scores of null indicate unavailable data | When DOM queries timeout, category `score` is `null` with an `error` explanation string. | [ ] |
| CL-8 | Web Vitals assessments are consistent | Each vital includes `value`, `assessment` ("good"/"needs-improvement"/"poor"), and `target`. | [ ] |
| CL-9 | top_opportunities is sorted by impact | Array is ordered by `estimated_impact_ms` descending. | [ ] |
| CL-10 | Effort estimates are meaningful | `effort` is one of `"low"`, `"medium"`, `"high"`. LLM can use for prioritization. | [ ] |
| CL-11 | Finding IDs are unique within the response | Each finding has a unique `id` (e.g., "rb-1", "img-1"). | [ ] |
| CL-12 | Category weights are documented in response or are well-known | Scoring methodology is transparent so LLM can explain scores. | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM may treat `estimated_impact_ms` as precise measurements rather than heuristic approximations. Verify the response or documentation notes these are estimates.
- [ ] LLM may assume a 100-score category has zero performance issues. Verify this means "no findings detected," not "perfect performance."
- [ ] LLM may confuse category `severity` (overall category assessment) with individual finding `severity`. Verify both are clearly scoped.
- [ ] LLM may not understand that caching findings have `estimated_impact_ms: 0` because impact is on repeat visits. Verify the explanation is in the finding description.
- [ ] LLM may assume DOM-dependent categories (dom_size, images, render_blocking) always have data. Verify null-score handling when DOM queries fail.
- [ ] LLM may treat unused JavaScript estimation as precise Coverage API data. Verify "heuristic" is explicit in the finding description.

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Run full performance audit | 1 step: `generate({format: "performance_audit"})` | No -- already minimal |
| Run category-specific audit | 1 step with `categories` parameter | No -- single call |
| Audit specific URL | 1 step with `url` parameter | No -- single call |
| Audit then fix then verify | 3 steps: audit + fix + observe(performance/vitals) | Natural workflow; performance audit is diagnosis, observe is verification |
| Metrics-only report (no recommendations) | 1 step with `include_recommendations: false` | No -- single parameter |

### Default Behavior Verification
- [ ] `generate({format: "performance_audit"})` with no optional params runs all 8 categories
- [ ] Default `include_recommendations` is `true`
- [ ] Default `url` uses the most recent performance snapshot URL
- [ ] Default `categories` includes all 8 categories
- [ ] Missing performance snapshot returns clear error message

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Parse performance_audit format | `{format: "performance_audit"}` | Routes to performance audit handler | must |
| UT-2 | Validate categories parameter | `["render_blocking", "images"]` | Only specified categories analyzed | must |
| UT-3 | Reject invalid category | `["render_blocking", "fonts"]` | "fonts" rejected with valid category list | must |
| UT-4 | Render-blocking CSS detection | Network entries with CSS in `<head>` without preload | Finding identifies blocking stylesheets with timing | must |
| UT-5 | Render-blocking JS detection | Script tags in `<head>` without async/defer | Finding identifies blocking scripts | must |
| UT-6 | DOM size analysis: node count | DOM query returns 2847 nodes | Finding with total_nodes, severity based on threshold | must |
| UT-7 | DOM size analysis: max depth | DOM query returns max_depth=18 | Finding includes max_depth and deepest_path | must |
| UT-8 | DOM size analysis: max children | DOM query returns max_children=142 | Finding identifies element with most children | must |
| UT-9 | Image: missing dimensions | Images without width/height attributes | Finding lists affected image URLs | must |
| UT-10 | Image: non-modern format | PNG/JPEG images (not WebP/AVIF) | Finding with potential savings calculation | must |
| UT-11 | JavaScript: large bundle detection | Bundle > 200KB | Finding with per-bundle size breakdown | must |
| UT-12 | JavaScript: unused estimation | Total JS > 500KB | Heuristic unused percentage estimate | should |
| UT-13 | CSS: large payload | Total CSS > 100KB | Finding with per-stylesheet sizes | should |
| UT-14 | Third-party: blocking time | Third-party scripts with main thread blocking | Finding with per-origin blocking time | should |
| UT-15 | Caching: missing Cache-Control | Static resources without caching headers | Finding lists uncached resources | should |
| UT-16 | Caching: short max-age | Cache-Control: max-age=300 on static asset | Finding flags short cache duration | should |
| UT-17 | Compression: uncompressed text | Text resource where transfer size = decoded size | Finding with potential compressed size | should |
| UT-18 | Overall score calculation | Category scores: [35, 72, 45, 58, 70, 80, 55, 85] | Weighted average using defined weights | must |
| UT-19 | Per-category score: no findings | Category with 0 findings | Score: 100 | must |
| UT-20 | Per-category score: critical finding | Category with critical-severity finding | Score: 0-49 | must |
| UT-21 | Web Vitals integration | FCP=1850, LCP=3200, CLS=0.18, INP=180 | Correct assessments (good/needs-improvement/poor) | must |
| UT-22 | Top opportunities ranking | Multiple findings across categories | Sorted by estimated_impact_ms descending | must |
| UT-23 | include_recommendations=false | Flag set to false | Findings present but no recommendation text | must |
| UT-24 | URL filter | `url: "https://example.com/app"` | Only data matching URL analyzed | should |
| UT-25 | No performance snapshot available | Empty ring buffer | Error: "No performance data available" | must |
| UT-26 | DOM query timeout (categories degrade) | DOM queries timeout | DOM-dependent categories show null score with error explanation | must |
| UT-27 | Empty network waterfall | No waterfall entries | Waterfall-dependent categories marked unavailable | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Full performance audit end-to-end | Go server + Extension + DOM queries + ring buffers | Complete audit with all 8 categories, scores, findings, recommendations | must |
| IT-2 | Audit with DOM queries | Go server async commands + Extension DOM query execution | DOM metrics collected and analyzed (node count, image analysis, head analysis) | must |
| IT-3 | Audit without extension (DOM queries fail) | Go server with extension disconnected | Non-DOM categories complete; DOM categories show null score with error | must |
| IT-4 | Audit after page load | Extension captures performance snapshot + waterfall | Audit references current snapshot data accurately | must |
| IT-5 | Category filtering | `categories: ["images", "javascript"]` | Only images and javascript categories in response | must |
| IT-6 | Concurrent audit requests | Two simultaneous audit requests | Both complete independently (stateless computation, read locks) | should |
| IT-7 | Audit then observe performance | Audit + subsequent observe({what: "performance"}) | Both tools work independently; audit does not modify snapshot data | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Server-side analysis time | Computation time without DOM queries | < 50ms | must |
| PT-2 | DOM query round-trip | Extension DOM query latency | < 2s | must |
| PT-3 | Total audit response time | Including DOM queries | < 5s | must |
| PT-4 | Memory during computation | Transient allocations | < 500KB | must |
| PT-5 | DOM query page-thread impact | Main thread blocking in page context | < 5ms | must |
| PT-6 | Audit with large waterfall (1000 entries) | Analysis time for 1000 resources | < 100ms server-side | should |
| PT-7 | DOM node count query on large page (10K nodes) | Query execution time | < 50ms (uses getElementsByTagName) | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No performance snapshot | Empty snapshot buffer | Error: "No performance data available. Navigate to a page and wait for it to load." | must |
| EC-2 | Extension disconnected during DOM queries | DOM queries timeout | DOM-dependent categories (dom_size, images, render_blocking) show null score; others complete | must |
| EC-3 | Empty network waterfall | No waterfall entries | Waterfall-dependent categories (caching, compression, third_party) unavailable; snapshot categories still work | must |
| EC-4 | SPA with stale snapshot | Page loaded minutes ago, SPA navigation since | Audit includes snapshot timestamp; AI can decide freshness | should |
| EC-5 | Page with no images | No `<img>` elements | Images category: score 100, zero findings | must |
| EC-6 | Very large DOM (10K+ nodes) | Complex page | DOM query uses efficient counting (getElementsByTagName); max depth bounded to 50 | must |
| EC-7 | Invalid category parameter | `categories: ["invalid"]` | Error listing valid categories | must |
| EC-8 | Concurrent audits | Two audit requests at same time | Both complete independently; stateless computation | should |
| EC-9 | All resources cached | Every resource has strong Cache-Control | Caching category: score 100, zero findings | should |
| EC-10 | All resources compressed | Every text resource has gzip/brotli | Compression category: score 100, zero findings | should |
| EC-11 | No third-party resources | Only first-party resources loaded | Third-party category: score 100, zero findings | should |
| EC-12 | No JavaScript on page | Static HTML page with no scripts | JavaScript category: score 100, zero findings | should |
| EC-13 | URL parameter matches no snapshot | `url: "https://nonexistent.com"` | Error or empty audit noting no matching data | must |
| EC-14 | 4K images on page | Multiple large images | Image findings include potential savings in bytes | should |
| EC-15 | Web Vitals not available | No CWV data in snapshot | web_vitals section shows null values or "unavailable" | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web page with known performance issues loaded (e.g., large images, render-blocking scripts, no caching headers)
- [ ] Page fully loaded (wait for load event) to ensure performance snapshot exists

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "generate", "arguments": {"format": "performance_audit"}}` | Server processes audit (may take up to 5s for DOM queries) | Response with all 8 categories, overall_score, web_vitals, top_opportunities | [ ] |
| UAT-2 | Inspect overall_score | Compare to per-category scores | Overall score is weighted average of category scores (weights per spec) | [ ] |
| UAT-3 | Inspect render_blocking category | Check actual `<head>` of page | Render-blocking CSS and JS correctly identified with URLs and blocking times | [ ] |
| UAT-4 | Inspect images category | Check actual images on page | Missing dimensions, non-modern formats, and oversized images correctly flagged | [ ] |
| UAT-5 | Inspect javascript category | Check JS bundles in Network tab | Bundle sizes match; large bundles correctly identified | [ ] |
| UAT-6 | Inspect caching category | Check Cache-Control headers in Network tab | Resources without effective caching correctly flagged | [ ] |
| UAT-7 | Inspect top_opportunities | Review ordering | Sorted by estimated_impact_ms descending; finding IDs reference actual findings | [ ] |
| UAT-8 | Inspect web_vitals | Compare to Chrome DevTools Performance | Values match or are close to DevTools readings; assessments correct | [ ] |
| UAT-9 | `{"tool": "generate", "arguments": {"format": "performance_audit", "categories": ["images", "javascript"]}}` | Only 2 categories | Response contains only images and javascript categories | [ ] |
| UAT-10 | `{"tool": "generate", "arguments": {"format": "performance_audit", "include_recommendations": false}}` | No recommendations | Findings present but no recommendation text in any finding | [ ] |
| UAT-11 | Fix an issue (e.g., add defer to script), reload, then re-audit | Performance improves | Previously flagged issue resolved or severity reduced; score improves | [ ] |
| UAT-12 | Disconnect extension, run audit | Extension offline | DOM-dependent categories show null score; other categories complete normally | [ ] |
| UAT-13 | Run audit before navigating to any page | No performance snapshot | Error: "No performance data available. Navigate to a page and wait for it to load." | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Resource URLs do not contain auth tokens | Inspect all URLs in findings | URLs contain paths but sensitive query params stripped | [ ] |
| DL-UAT-2 | DOM queries are read-only | Check page state before and after audit | No DOM modifications, no form submissions, no navigation | [ ] |
| DL-UAT-3 | No inline script content in response | Inspect head analysis findings | Only script tag attributes (src, async, defer) -- not inline JS content | [ ] |
| DL-UAT-4 | No request/response bodies in findings | Inspect all findings | Only URLs, sizes, headers, and timing data -- no body content | [ ] |
| DL-UAT-5 | No external requests during audit | Monitor network | All analysis is local; no external API calls | [ ] |

### Regression Checks
- [ ] Existing `observe({what: "performance"})` still works independently
- [ ] Existing `observe({what: "vitals"})` still works independently
- [ ] Existing `observe({what: "network_waterfall"})` still works independently
- [ ] Performance snapshot ring buffer not modified by audit
- [ ] Network waterfall buffer not modified by audit
- [ ] Other generate modes (csp, test, reproduction, har, sarif) still work
- [ ] DOM query infrastructure not affected for other features (a11y, page)

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
