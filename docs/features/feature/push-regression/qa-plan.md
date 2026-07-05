---
status: proposed
scope: feature/push-regression/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-push-regression
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Push Notification on Regression

> QA plan for the Push Regression feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom runs on localhost and data must never leave the machine. Pay particular attention to sensitive data flowing through MCP tool responses.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Regression alert URL field exposes internal paths | Verify `url` in performance_alerts uses the same URL path visible in the browser address bar, not internal routing | high |
| DL-2 | Metric baseline/current values reveal app state | Verify `metrics` object contains only numeric timing/size values (ms, bytes, percentages), no content data | medium |
| DL-3 | Alert summary text contains user-specific data | Verify `summary` field uses generic descriptions ("Load time regressed by 847ms") not user-identifying context | medium |
| DL-4 | Recommendation field references internal tools | Verify `recommendation` references only public Kaboom tools (`check_performance`, `causal_diff`), not internal endpoints | low |
| DL-5 | Multiple tab alerts reveal user browsing patterns | Verify `performance_alerts` array does not expose tab IDs or browsing sequences across multiple tabs | high |
| DL-6 | Stale baseline from persistent memory leaks old session data | Verify baseline values in alerts are numeric only (no old session context like URLs visited, errors seen) | medium |
| DL-7 | Pending alert accumulation reveals monitoring scope | Verify pending alerts (up to 10) do not collectively reveal which pages the user has been visiting | medium |
| DL-8 | Checkpoint IDs reveal session activity patterns | Verify checkpoint IDs are opaque monotonic counters, not timestamps or encodings of user actions | low |

### Negative Tests (must NOT leak)
- [ ] No auth tokens or query parameters in alert URL fields
- [ ] No request/response body content in metric data
- [ ] No user-identifying information in alert summaries
- [ ] No tab IDs or browsing sequence data in alert arrays
- [ ] No old session context beyond numeric baseline values
- [ ] No internal server endpoint URLs in recommendation text
- [ ] No checkpoint timing data that reveals when user was active

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Alert appears once, not repeatedly | AI understands alerts appear on the first `get_changes_since` poll after detection, then are cleared | [ ] |
| CL-2 | "regression" type meaning | AI understands this means performance got worse vs baseline, not that code has a bug | [ ] |
| CL-3 | Delta values are clear | AI correctly interprets `delta_ms: 847` as "847ms slower" and `delta_pct: 70.6` as "70.6% slower" | [ ] |
| CL-4 | Only regressed metrics shown | AI understands metrics within threshold are intentionally omitted, not missing due to error | [ ] |
| CL-5 | Baseline vs current semantics | AI understands `baseline` is the known-good state and `current` is what was just measured | [ ] |
| CL-6 | TTFB higher tolerance | AI understands TTFB allows >50% regression before alerting (network variance expected) | [ ] |
| CL-7 | CLS absolute vs percentage | AI understands CLS uses absolute threshold (>0.1 increase), not percentage, because CLS is already a ratio | [ ] |
| CL-8 | "No alerts" is expected | AI understands absence of `performance_alerts` key means no regressions, not a feature failure | [ ] |
| CL-9 | Self-resolving alerts | AI understands alerts auto-clear when next snapshot shows regression resolved | [ ] |
| CL-10 | Recommendation is a next-action hint | AI uses `recommendation` to decide what tool to call next, not as a diagnostic conclusion | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might think `delta_pct: 70.6` means the page loads at 70.6% of original speed -- verify it means "70.6% slower"
- [ ] AI might think alerts persist across polls -- verify they appear exactly once
- [ ] AI might confuse `transfer_bytes` regression with a security issue -- verify it is about payload size, not content
- [ ] AI might not understand that first snapshot creates baseline (no alert) -- verify this edge case is clear
- [ ] AI might think `performance_alerts: []` (empty array) is different from missing key -- verify both mean "no regressions"

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (fully automatic)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Detect regression | 0 steps: automatic, embedded in `get_changes_since` | No -- already zero-config |
| See regression details | 0 steps: alert includes metrics, summary, recommendation | No -- data inline |
| Act on regression | 1 step: follow recommendation (call `check_performance` or `causal_diff`) | No -- already one action |
| Acknowledge regression | 0 steps: automatic via checkpoint advancement | No -- already zero-config |
| Detect self-healing fix | 0 steps: alert auto-clears on next good snapshot | No -- already automatic |

### Default Behavior Verification
- [ ] Feature works with zero configuration once baseline exists
- [ ] No opt-in or enable step required
- [ ] Alert automatically embedded in existing `get_changes_since` response
- [ ] Alert lifecycle (appear once, drain) works without explicit acknowledgment
- [ ] Self-resolving alerts clear automatically
- [ ] Pending alert cap (10) enforced automatically

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Snapshot within threshold | All metrics within 20% of baseline | No alert generated | must |
| UT-2 | Load time 30% over baseline | Baseline 1000ms, current 1300ms | Alert with `load.delta_ms: 300, delta_pct: 30.0` | must |
| UT-3 | FCP 25% over baseline | Baseline 500ms, current 625ms | Alert with `fcp.delta_ms: 125, delta_pct: 25.0` | must |
| UT-4 | LCP regression | Baseline 800ms, current 1200ms (50%) | Alert with LCP metrics | must |
| UT-5 | TTFB under 50% threshold | Baseline 100ms, current 140ms (40%) | No TTFB alert (higher tolerance) | must |
| UT-6 | TTFB over 50% threshold | Baseline 100ms, current 160ms (60%) | Alert with TTFB metrics | must |
| UT-7 | CLS absolute increase >0.1 | Baseline 0.05, current 0.20 (delta 0.15) | Alert with CLS metrics | must |
| UT-8 | CLS absolute increase <=0.1 | Baseline 0.05, current 0.12 (delta 0.07) | No CLS alert | must |
| UT-9 | Transfer size 30% increase | Baseline 200KB, current 260KB (30%) | Alert with transfer_bytes metrics | must |
| UT-10 | Transfer size under 25% threshold | Baseline 200KB, current 240KB (20%) | No transfer alert | must |
| UT-11 | Only regressed metrics in alert | Load and LCP regressed, FCP within threshold | Alert includes only load and LCP, not FCP | must |
| UT-12 | Alert includes in get_changes_since | Alert pending, checkpoint before detection | Alert in `performance_alerts` array | must |
| UT-13 | Alert not repeated | Alert already delivered, newer checkpoint | Alert NOT in response | must |
| UT-14 | Multiple pending alerts | 3 different URLs regressed | `performance_alerts` array has 3 entries | must |
| UT-15 | Alert cap at 10 | 11 regressions detected | Oldest alert dropped, 10 remain | must |
| UT-16 | Self-resolving alert | Regression detected, next snapshot within threshold | Alert cleared before delivery (or not delivered if not yet polled) | must |
| UT-17 | No baseline exists | First snapshot for a URL | No alert (baseline created, not compared) | must |
| UT-18 | Stale baseline from persistent memory | Baseline loaded from disk (previous session) | Alerts still fire against it (regression vs known-good) | must |
| UT-19 | Recommendation field content | Any regression alert | Contains "check_performance" or "causal_diff" suggestion | must |
| UT-20 | Alert ID monotonic | 3 alerts generated sequentially | IDs are monotonically increasing | must |
| UT-21 | Checkpoint tracking: alert before checkpoint | Alert at checkpoint 5, query from checkpoint 3 | Alert included | must |
| UT-22 | Checkpoint tracking: alert after checkpoint | Alert at checkpoint 5, query from checkpoint 6 | Alert NOT included | must |
| UT-23 | Multiple tabs same URL | Tab A loads fast, Tab B loads slow | Slow tab generates alert, fast tab does not | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end regression detection | Extension snapshot -> Server comparison -> Alert in get_changes_since | Complete flow from browser to AI | must |
| IT-2 | Regression + push alerts integration | Regression detected -> appears in both `get_changes_since` and passive alert system | Both delivery mechanisms surface the regression | must |
| IT-3 | Regression + causal diff integration | Regression alert -> AI calls `get_causal_diff` | Causal diff available for the regressed URL | should |
| IT-4 | Self-healing verification | Regression -> fix code -> reload -> next snapshot OK | Alert cleared, no regression in subsequent polls | must |
| IT-5 | Checkpoint-based lifecycle | Multiple reloads with regressions -> sequential `get_changes_since` calls | Each poll gets only new alerts since last checkpoint | must |
| IT-6 | Server restart clears pending | Alerts pending -> server restart -> query | No pending alerts (in-memory only) | must |
| IT-7 | Baseline from persistent memory | Server loads baseline from disk -> extension sends slow snapshot | Regression detected against disk-loaded baseline | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Regression detection speed | Time to compare 6 metrics against baseline | < 0.5ms | must |
| PT-2 | Alert inclusion speed | Time to scan 10 pending alerts for checkpoint | < 0.1ms | must |
| PT-3 | Memory for pending alerts | Memory for 10 alerts with full metric data | < 5KB | must |
| PT-4 | No additional goroutines | Thread count before/after feature | No new goroutines | must |
| PT-5 | Alert lifecycle overhead | Impact on get_changes_since response time | < 0.5ms additional | must |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No baseline for URL | First page load to a new URL | Baseline created, no alert | must |
| EC-2 | Agent never polls | 15 regressions detected, no get_changes_since calls | 10 alerts retained (cap), oldest 5 dropped | must |
| EC-3 | Multiple tabs, same URL | Tab A: 500ms, Tab B: 3000ms (both compare to 1000ms baseline) | Tab B generates alert, Tab A does not | must |
| EC-4 | Regression resolves before delivery | Regression at checkpoint 5, fix at checkpoint 6, poll at checkpoint 4 | Alert might be cleared (depends on timing) or delivered then cleared | must |
| EC-5 | Server restart between snapshot and poll | Regression detected -> server crash -> restart -> poll | Alert lost (in-memory only), no crash | must |
| EC-6 | Concurrent snapshot and poll | Snapshot processing at same time as get_changes_since | Alert stored after snapshot, appears on next poll if current poll missed it | must |
| EC-7 | Baseline is exactly at threshold | Current is exactly 20% over baseline | Alert generated (at threshold = regression) | should |
| EC-8 | Baseline is zero | Baseline load_ms: 0 (impossible but defensive) | No division by zero, handled gracefully | must |
| EC-9 | Very large regression | Baseline 100ms, current 30000ms (300x) | Alert with correct delta, no overflow | should |
| EC-10 | All metrics regressed simultaneously | All 6 metrics over threshold | Single alert with all 6 metrics listed | must |
| EC-11 | Rapid page reloads (10 in 5 seconds) | 10 snapshots arrive quickly, all regressed | Alerts accumulate up to cap (10), checkpoint tracking correct | must |
| EC-12 | CLS of 0.0 as baseline | Baseline CLS: 0.0, current CLS: 0.05 | No alert (0.05 < 0.1 absolute threshold) | must |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web app with controllable performance (can add/remove scripts to cause regressions)
- [ ] Performance baseline established for the test page (at least one prior load)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Load the test page normally (establish baseline) | Page loads at normal speed | Extension captures performance snapshot, baseline created | [ ] |
| UAT-2 | `{"tool": "observe", "arguments": {"what": "changes"}}` | AI receives changes | No `performance_alerts` key (first load creates baseline, not alert) | [ ] |
| UAT-3 | Add a 500KB render-blocking script to the page | Human adds `<script src="heavy.js">` (synchronous, blocking) | Script added to page source | [ ] |
| UAT-4 | Reload the page | Page loads noticeably slower | Extension captures slow performance snapshot | [ ] |
| UAT-5 | `{"tool": "observe", "arguments": {"what": "changes"}}` | AI receives changes | `performance_alerts` array with at least 1 entry showing regression | [ ] |
| UAT-6 | Verify alert content | Inspect alert fields | `type: "regression"`, `url: "/test-page"`, `summary` mentions load time increase, `metrics` shows delta | [ ] |
| UAT-7 | Verify only regressed metrics present | Inspect `metrics` object | Only metrics exceeding threshold included (not all 6) | [ ] |
| UAT-8 | Verify recommendation | Check `recommendation` field | Contains suggestion to use `check_performance` or `causal_diff` | [ ] |
| UAT-9 | `{"tool": "observe", "arguments": {"what": "changes"}}` again | AI receives changes | NO `performance_alerts` (alert was drained on previous call) | [ ] |
| UAT-10 | Remove the heavy script (fix the regression) | Human removes the blocking script | Page source restored to normal | [ ] |
| UAT-11 | Reload the page | Page loads at normal speed again | Extension captures fast performance snapshot | [ ] |
| UAT-12 | `{"tool": "observe", "arguments": {"what": "changes"}}` | AI receives changes | No regression alert (self-resolved) | [ ] |
| UAT-13 | Add API latency (slow backend endpoint by 500ms) | Human adds server-side delay | API response takes 500ms longer | [ ] |
| UAT-14 | Reload the page | Page loads slower due to API | TTFB or load time regression depending on threshold | [ ] |
| UAT-15 | `{"tool": "observe", "arguments": {"what": "changes"}}` | AI receives changes | Regression alert for TTFB/load time with API-related metrics | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | No query params in alert URL | Trigger regression on URL with `?token=abc` | Alert `url` shows clean path without query params | [ ] |
| DL-UAT-2 | Metric values are numeric only | Inspect `metrics` object in alert | Only numbers (ms, bytes, percentages), no strings with content | [ ] |
| DL-UAT-3 | Summary is generic | Read alert `summary` field | Generic description like "Load time regressed by Xms", no user-specific context | [ ] |
| DL-UAT-4 | Multiple alerts do not reveal browsing pattern | Trigger regressions on 3 different pages | Alerts show independent URL paths, no sequence/timing between them | [ ] |

### Regression Checks
- [ ] Existing `get_changes_since` response format unchanged when no regressions
- [ ] Existing performance monitoring (`check_performance`) still works independently
- [ ] Checkpoint-based diffing for console/network/WebSocket changes unaffected
- [ ] No additional HTTP requests or goroutines from this feature
- [ ] Server restart gracefully loses pending alerts (no crash, no stale data)
- [ ] Performance snapshot processing speed not degraded by alert detection

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
