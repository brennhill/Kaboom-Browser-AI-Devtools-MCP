---
status: proposed
scope: feature/web-vitals/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-web-vitals
last_reviewed: 2026-08-04
last_verified_version: 0.9.0
last_verified_date: 2026-08-04
---

# QA Plan: Web Vitals Capture

Automated contract coverage verifies bounded LCP, CLS, INP, and long-task
attribution; content and resource URLs are excluded; and unsupported source
stacks are explicitly labeled unavailable.

> QA plan for the Web Vitals Capture feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Web vitals data is primarily performance metrics (timing values, layout shift scores), which are inherently low-risk. However, the INP attribution data includes CSS selectors of interacted elements and page URLs, which could expose internal application structure.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Page URLs in vitals history | Verify page URLs in vitals entries do not expose sensitive routes (e.g., `/admin/users/john@example.com`) | medium |
| DL-2 | CSS selectors revealing PII | Verify `worst_target` CSS selector (e.g., `input#email-john-doe`) does not contain user-specific data | medium |
| DL-3 | INP attribution exposing form content | Verify INP event target attribution does not include form field values or typed content | high |
| DL-4 | Vitals history exposing browsing patterns | Verify history (up to 10 entries per URL) does not create a detailed browsing timeline that could identify a user | low |
| DL-5 | Extension page detection | Verify `chrome://` and extension URLs are noted as unsupported, not exposed in vitals data | low |
| DL-6 | SPA navigation tracking | Verify soft navigation URLs tracked in vitals do not expose sensitive client-side route parameters | medium |
| DL-7 | Performance timing data granularity | Verify timing values (ms) do not inadvertently fingerprint the user's hardware or network | low |
| DL-8 | Summary text containing URL paths | Verify `summary` field does not include full page URLs with sensitive path segments | medium |

### Negative Tests (must NOT leak)
- [ ] Page URLs with PII in path segments (e.g., `/users/john@example.com/profile`) must not appear without sanitization
- [ ] CSS selectors containing user-specific IDs or data attributes must not expose PII
- [ ] INP attribution must show only element selector, event type, and timing -- never form input values
- [ ] Vitals history must not expose authentication-gated page URLs if the user expects privacy
- [ ] No hardware fingerprinting data (CPU model, GPU info, device memory) in vitals output
- [ ] Extension-internal URLs (chrome-extension://) must not appear in vitals data

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Four metrics clearly named | FCP, LCP, CLS, INP are standard abbreviations, full names available | [ ] |
| CL-2 | Ratings use Google terminology | "good", "needs-improvement", "poor" match Google's classification | [ ] |
| CL-3 | Thresholds are included | Each metric includes `threshold` object with `good` and `poor` boundaries | [ ] |
| CL-4 | Units are unambiguous | FCP, LCP, INP in `value_ms` (milliseconds), CLS unitless `value` | [ ] |
| CL-5 | Overall rating semantics | `overall_rating` is the worst individual rating (clearly documented) | [ ] |
| CL-6 | INP null when no interactions | `inp: null` with explanation "no interactions measured" is clear | [ ] |
| CL-7 | INP attribution is actionable | `worst_target`, `worst_type`, processing/delay/presentation breakdown | [ ] |
| CL-8 | Summary highlights most actionable insight | One-sentence summary directs attention to the biggest issue | [ ] |
| CL-9 | History shows trends | `include_history: true` returns array of previous loads for comparison | [ ] |
| CL-10 | LCP "estimated" flag | LCP marked as estimated when page hidden before finalization is distinguishable from finalized LCP | [ ] |
| CL-11 | SPA navigation note | Response notes whether soft navigations occurred (affects FCP/LCP validity) | [ ] |
| CL-12 | Browser compatibility note | When INP is unavailable (Chrome <96), the null value has explanation | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM might confuse CLS (unitless score 0.0-1.0+) with a millisecond value -- verify units are clearly labeled
- [ ] LLM might interpret `overall_rating: "poor"` as "everything is bad" when only one metric is poor -- verify the summary explains which specific metric is the issue
- [ ] LLM might not realize INP null means "no interactions yet" rather than "INP is 0ms" -- verify the null explanation
- [ ] LLM might compare FCP across different pages without realizing page content affects FCP -- verify URL is associated with each entry
- [ ] LLM might not understand that LCP stops updating after user interaction -- verify the finalization semantics are noted
- [ ] LLM might treat `needs-improvement` as a binary fail rather than a middle ground -- verify three-tier classification is clear

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Get current page vitals | 1 step: call `observe(what: "vitals")` | No -- already minimal |
| Get vitals with history | 1 step: add `include_history: true` | No |
| Monitor vitals across reloads | Repeat `observe(what: "vitals")` after each reload | No -- polling is the correct pattern |
| Identify slow interactions | Read INP attribution from vitals response | No -- attribution is inline |

### Default Behavior Verification
- [ ] Feature works with zero configuration (vitals captured automatically via PerformanceObservers)
- [ ] Default response includes all four metrics (FCP, LCP, CLS, INP)
- [ ] `include_history` defaults to `false` (smaller response for typical queries)
- [ ] Thresholds are always included (no separate config step)
- [ ] INP observer starts automatically in extension inject.js
- [ ] LCP finalization happens automatically on visibility change or user input

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | FCP good classification | FCP = 1200ms | `rating: "good"` (threshold: <=1800ms) | must |
| UT-2 | FCP needs-improvement | FCP = 2500ms | `rating: "needs-improvement"` (1800-3000ms) | must |
| UT-3 | FCP poor classification | FCP = 3500ms | `rating: "poor"` (>3000ms) | must |
| UT-4 | LCP good classification | LCP = 2000ms | `rating: "good"` (threshold: <=2500ms) | must |
| UT-5 | LCP needs-improvement | LCP = 3200ms | `rating: "needs-improvement"` (2500-4000ms) | must |
| UT-6 | LCP poor classification | LCP = 4500ms | `rating: "poor"` (>4000ms) | must |
| UT-7 | CLS good classification | CLS = 0.05 | `rating: "good"` (threshold: <=0.1) | must |
| UT-8 | CLS needs-improvement | CLS = 0.15 | `rating: "needs-improvement"` (0.1-0.25) | must |
| UT-9 | CLS poor classification | CLS = 0.35 | `rating: "poor"` (>0.25) | must |
| UT-10 | INP good classification | INP = 150ms | `rating: "good"` (threshold: <=200ms) | must |
| UT-11 | INP needs-improvement | INP = 350ms | `rating: "needs-improvement"` (200-500ms) | must |
| UT-12 | INP poor classification | INP = 600ms | `rating: "poor"` (>500ms) | must |
| UT-13 | Overall rating worst-wins | FCP good, LCP good, CLS poor, INP good | `overall_rating: "poor"` | must |
| UT-14 | Overall rating all good | All four metrics good | `overall_rating: "good"` | must |
| UT-15 | INP null (no interactions) | No user interactions recorded | `inp: null` with explanation | must |
| UT-16 | INP worst interaction (<=50) | 30 interactions, worst = 180ms | INP = 180ms | must |
| UT-17 | INP 98th percentile (>50) | 100 interactions | INP = 98th percentile value | must |
| UT-18 | INP interactionId grouping | pointerdown + pointerup + click = 1 interaction | Longest duration used for the group | must |
| UT-19 | INP attribution | Slow click handler | `worst_target`, `worst_type`, processing/delay/presentation breakdown | must |
| UT-20 | LCP finalization on visibility change | Page hidden event | LCP uses last reported value, marked as finalized | must |
| UT-21 | LCP finalization on user input | First click event | LCP stops updating | must |
| UT-22 | LCP estimated flag | Page hidden before LCP stabilizes | LCP marked "estimated" | should |
| UT-23 | CLS excludes input-adjacent shifts | Layout shift within 500ms of user input | Not counted in CLS | must |
| UT-24 | Vitals history storage | 5 page loads to same URL | History contains all 5 entries | must |
| UT-25 | Vitals history cap | 11 page loads to same URL | History contains only last 10 | must |
| UT-26 | Include history parameter | `include_history: true` | Response includes previous loads | must |
| UT-27 | Summary highlights worst metric | LCP poor, others good | Summary mentions LCP specifically | must |
| UT-28 | Summary notes threshold proximity | INP = 190ms (close to 200ms good limit) | Summary mentions INP approaching threshold | should |
| UT-29 | SPA navigation note | pushState navigation occurred | Response notes soft navigation since load | should |
| UT-30 | Browser compatibility (no event observer) | INP observer unavailable | `inp: null` with "browser does not support INP measurement" | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end vitals via MCP | Extension captures vitals -> server stores -> `observe(what: "vitals")` | All four metrics with ratings and thresholds | must |
| IT-2 | INP capture during interactions | User clicks buttons on test page -> query vitals | INP populated with attribution data | must |
| IT-3 | LCP finalization on tab switch | Load page -> switch tabs -> query vitals | LCP finalized with correct value | must |
| IT-4 | Vitals history across reloads | Reload page 3 times -> query with history | History array has 3 entries | must |
| IT-5 | Performance budget integration | INP regresses -> push notification | Alert includes INP regression details | should |
| IT-6 | Performance snapshot enhancement | Extension POSTs snapshot with INP | Server stores INP alongside FCP/LCP/CLS | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | INP observer overhead per interaction | Wall clock time | Under 0.05ms | must |
| PT-2 | Vitals calculation | Wall clock time | Under 0.1ms | must |
| PT-3 | Memory for 200 interactions | Memory footprint | Under 20KB | must |
| PT-4 | Observer does not affect measured interactions | Input delay attributable to observer | Zero measurable impact | must |
| PT-5 | History storage memory | Memory for 10 entries per URL, 50 URLs | Under 500KB | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No interactions on page | Page loaded, immediately queried | INP null with explanation | must |
| EC-2 | Page hidden before LCP | Load page, immediately switch tabs | LCP estimated, CLS accumulated | must |
| EC-3 | Very many interactions (>200) | 250 interactions on page | Top 50 by duration stored, 98th percentile from sample | must |
| EC-4 | iframe interactions | Click inside cross-origin iframe | INP measures main-frame only, iframe excluded | must |
| EC-5 | SPA navigation (pushState) | Single-page app with client-side routing | FCP/LCP valid for initial load only, CLS/INP accumulate, note included | must |
| EC-6 | Chrome extension page | Navigate to `chrome://extensions` | Vitals null with "extension pages not supported" note | should |
| EC-7 | Very fast page (all metrics excellent) | Lightweight static page | All metrics "good", summary confirms | must |
| EC-8 | Very slow page (all metrics terrible) | Heavy page with layout shifts and slow handlers | All metrics "poor", summary actionable | must |
| EC-9 | CLS only from layout shifts without input | Automatic image resizing causing shifts | CLS counts these shifts (no input to exclude) | must |
| EC-10 | INP with very short interaction | 1ms click handler | Not captured if below 16ms durationThreshold | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page loaded (any page with measurable content -- images, text, interactive elements)
- [ ] Chrome version 96+ (required for INP measurement via `event` observer)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "observe", "arguments": {"what": "vitals"}}` | Page has finished loading | Response shows FCP, LCP, CLS with values, ratings, thresholds. INP may be null if no interactions yet | [ ] |
| UAT-2 | Click several buttons/links on the page, then: `{"tool": "observe", "arguments": {"what": "vitals"}}` | Interactions occurred | INP now has a value with `worst_target` and `worst_type` attribution | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "vitals", "include_history": true}}` | At least 2 page loads in session | History array with previous vitals entries | [ ] |
| UAT-4 | Reload page 3 times, querying vitals each time | Each reload produces new vitals | Values may vary slightly, all three captured in history | [ ] |
| UAT-5 | Open Chrome DevTools -> Lighthouse tab -> run audit | Compare Kaboom vitals to Lighthouse | FCP and LCP should be in the same ballpark (not identical due to different measurement conditions) | [ ] |
| UAT-6 | Switch to another tab for 5 seconds, then switch back and query vitals | Tab visibility changed | LCP shows finalized value (not still updating) | [ ] |
| UAT-7 | Verify overall rating | Check individual and overall ratings | `overall_rating` equals the worst individual metric rating | [ ] |
| UAT-8 | Navigate to a very simple page (e.g., blank HTML with one paragraph) | Fast page load | All metrics "good", summary confirms excellent performance | [ ] |
| UAT-9 | Add a slow click handler (`setTimeout(resolve, 1000)` on a button), click it, query vitals | Slow interaction | INP shows high value with attribution pointing to that button | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Page URLs in vitals | Query vitals for pages with sensitive paths | URLs shown are the page paths (expected -- necessary for metric association) | [ ] |
| DL-UAT-2 | INP target selector clean | Click on a form input with PII in name attribute | CSS selector shows element identifier, not input value | [ ] |
| DL-UAT-3 | No hardware fingerprinting | Inspect full vitals response | No CPU, GPU, device memory, or screen resolution data | [ ] |
| DL-UAT-4 | History does not create browsing timeline | Query with history for authenticated pages | History is per-URL (not a chronological timeline of all pages visited) | [ ] |

### Regression Checks
- [ ] Existing performance snapshot capture still works alongside web vitals
- [ ] Existing `observe(what: "performance")` tool still returns full performance data
- [ ] FCP and LCP values match those in performance snapshots (same source data)
- [ ] Extension inject.js does not measurably degrade page performance with new observers
- [ ] Performance budget regression detection integrates INP thresholds correctly

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
