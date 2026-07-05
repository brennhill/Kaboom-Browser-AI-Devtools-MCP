---
status: proposed
scope: feature/performance-budget/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-performance-budget
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Performance Budget

> QA plan for the Performance Budget feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Performance budget monitoring collects page load metrics, resource timing data, baseline averages, and regression alerts. Resource URLs and page URLs may contain sensitive information. Alert history persists across sessions.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Page URLs in baselines containing PII | Verify baseline URLs (e.g., `/admin/user/john@example.com/profile`) do not expose PII when baselines are queried or persisted | medium |
| DL-2 | Resource URLs in performance reports | Verify resource entries (JS, CSS, images) do not expose internal CDN tokens or signed URLs with auth parameters | high |
| DL-3 | Resource fingerprints revealing internal paths | Verify resource fingerprints stored in baselines do not expose full server-side file paths | medium |
| DL-4 | Regression alert messages with sensitive context | Verify alert messages (e.g., "Load time regressed on /admin/secret-panel") do not expose sensitive route names inappropriately | medium |
| DL-5 | Baseline persistence containing raw timing data | Verify persisted baselines store only aggregated averages, not individual request timings that could fingerprint sessions | low |
| DL-6 | Causal diff exposing individual resource details | Verify causal diffing (identifying what changed between snapshots) does not expose individual resource content or headers | medium |
| DL-7 | Push notification content | Verify push notifications for regressions contain summary info, not detailed resource URLs or timing data | medium |
| DL-8 | Transfer size data revealing payload structure | Verify transfer size metrics do not reveal enough about response sizes to infer data content | low |
| DL-9 | LRU eviction data accessible | Verify evicted baselines/snapshots are fully removed from memory (not just dereferenced) | low |
| DL-10 | Session summary including sensitive URLs | Verify PR summaries and session summaries generated from performance data sanitize page URLs | medium |

### Negative Tests (must NOT leak)
- [ ] Baseline URLs with PII are not exposed in performance reports without sanitization
- [ ] Resource URLs with authentication tokens (signed CDN URLs) are not persisted in baselines
- [ ] Regression alert messages do not expose sensitive admin-only route names
- [ ] Persisted baselines contain only aggregated metrics (averages, counts), not raw per-request data
- [ ] Causal diff output shows resource type changes (e.g., "new script added, 50KB"), not full resource URLs with auth params
- [ ] Push notifications for regressions contain only metric name, old value, new value -- not full page context
- [ ] Evicted LRU entries are not accessible through any API or persistence mechanism

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Regression is clearly defined | "2x baseline" threshold is documented in response or tool description | [ ] |
| CL-2 | Metric names are standard | Load time, FCP, LCP, TTFB, CLS, INP, transfer size, request count, TBT use standard names | [ ] |
| CL-3 | Baseline sample count is visible | `sample_count` tells AI how confident the baseline is (more samples = more reliable) | [ ] |
| CL-4 | Regression alerts are actionable | Alert includes metric name, current value, baseline value, and recommendation | [ ] |
| CL-5 | "No baseline" is distinguishable | First page load (no baseline yet) is clearly different from "baseline exists, no regression" | [ ] |
| CL-6 | Weighted averaging is transparent | Baseline uses 80/20 weighted average after stabilization (~5 samples) | [ ] |
| CL-7 | Resource breakdown is clear | Performance report shows resources by type (scripts, styles, images, fonts) with counts and sizes | [ ] |
| CL-8 | Causal diff is actionable | Diff shows what changed (new resources, removed resources, size changes) between snapshots | [ ] |
| CL-9 | Alert resolution is visible | Resolved alerts are distinguishable from active alerts | [ ] |
| CL-10 | Performance report format | Report is formatted as readable text, not just raw JSON | [ ] |
| CL-11 | Timing units are consistent | All timing values in milliseconds, transfer sizes in bytes | [ ] |
| CL-12 | Max alerts cap is noted | When alert cap is reached, response notes that additional alerts may be suppressed | [ ] |

### Common LLM Misinterpretation Risks
- [ ] LLM might interpret "no baseline" as "performance is fine" rather than "not enough data to judge" -- verify messaging is clear
- [ ] LLM might not understand the 80/20 weighted average and expect simple averages -- verify baseline description
- [ ] LLM might confuse TBT (Total Blocking Time) with load time -- verify metrics are clearly named
- [ ] LLM might treat a resolved regression alert as still active -- verify resolution status is prominent
- [ ] LLM might compare baselines across different URLs without realizing each URL has its own baseline -- verify URL association is clear
- [ ] LLM might not realize the 2x threshold means "double the baseline" -- verify the comparison is explicit

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium (configuration via `configure` tool, observation via `observe` tool)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Check current performance | 1 step: call `observe(what: "performance")` | No -- already minimal |
| Check session health | 1 step: call `configure(action: "health")` | No -- already minimal |
| View regression alerts | Included in health check or changes response | No -- already integrated |
| Set custom budget thresholds | Via `configure` tool | Could be simpler with presets (e.g., "strict", "relaxed") |
| Compare across page loads | Automatic via baseline system | No -- baselines build automatically |
| Reset baselines | Via `configure` tool | No -- explicit action is correct for destructive operation |

### Default Behavior Verification
- [ ] Feature works with zero configuration (baselines build automatically from observed traffic)
- [ ] Default regression threshold is 2x baseline (sensible for catching real regressions)
- [ ] Performance snapshots are automatically captured by the extension
- [ ] Baseline starts building from the first page load (no manual "start tracking" step)
- [ ] Alerts fire automatically when regressions are detected (push notifications)
- [ ] LRU eviction keeps memory bounded without user intervention

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Performance snapshot storage | Valid snapshot with URL, timing, network data | Snapshot stored, retrievable by URL | must |
| UT-2 | LRU eviction for snapshots | More snapshots than maxPerfSnapshots | Oldest evicted, newest retained | must |
| UT-3 | First baseline creation | First snapshot for a URL | Baseline with sample_count=1, values match snapshot | must |
| UT-4 | Baseline averaging (simple, n<5) | Second snapshot for same URL | Baseline updated with simple running average | must |
| UT-5 | Baseline weighted averaging (n>=5) | 6th snapshot | Baseline uses 80/20 weighted average | must |
| UT-6 | avgOptionalFloat with nil baseline | Baseline nil, snapshot has value | Returns snapshot value | must |
| UT-7 | avgOptionalFloat with nil snapshot | Baseline has value, snapshot nil | Returns baseline value | must |
| UT-8 | weightedOptionalFloat with nil baseline | Baseline nil, snapshot has value | Returns snapshot value | must |
| UT-9 | weightedOptionalFloat zero baseline | Baseline is 0.0, snapshot has value | Returns weighted result (not NaN or panic) | must |
| UT-10 | Load time regression detection | Baseline load=1000ms, snapshot load=2500ms | Regression detected (>2x) | must |
| UT-11 | Load time no false positive | Baseline load=1000ms, snapshot load=1800ms | No regression (<2x) | must |
| UT-12 | FCP regression detection | Baseline FCP=800ms, snapshot FCP=2000ms | Regression detected | must |
| UT-13 | LCP regression detection | Baseline LCP=2000ms, snapshot LCP=5000ms | Regression detected | must |
| UT-14 | TTFB regression detection | Baseline TTFB=200ms, snapshot TTFB=500ms | Regression detected (>2x) | must |
| UT-15 | CLS regression detection | Baseline CLS=0.05, snapshot CLS=0.15 | Regression detected | must |
| UT-16 | CLS regression with zero baseline | Baseline CLS=0, snapshot CLS=0.12 | Regression detected (absolute threshold) | must |
| UT-17 | Transfer size regression | Baseline transfer=100KB, snapshot transfer=250KB | Regression detected (>2x) | must |
| UT-18 | Request count regression | Baseline requests=20, snapshot requests=50 | Regression detected (>2x) | must |
| UT-19 | Long tasks regression from zero | Baseline long_tasks=0, snapshot long_tasks=3 | Regression detected | must |
| UT-20 | Long tasks regression >100% increase | Baseline long_tasks=2, snapshot long_tasks=5 | Regression detected | must |
| UT-21 | TBT regression | Baseline TBT=50ms, snapshot TBT=200ms | Regression detected | must |
| UT-22 | TBT from zero | Baseline TBT=0, snapshot TBT=100ms | Regression detected (absolute threshold) | must |
| UT-23 | No regression on all metrics | All metrics within threshold | Empty regression list | must |
| UT-24 | INP timing field | Snapshot with INP value | INP included in performance report and baseline | must |
| UT-25 | INP omitted when nil | Snapshot without INP | INP field absent from JSON output | must |
| UT-26 | Performance report formatting | Snapshot + baseline | Readable text report with timing, network, resource sections | must |
| UT-27 | Performance report no baseline | First snapshot, no baseline | Report notes "no baseline established" | must |
| UT-28 | Performance report with regressions | Snapshot with regressions vs baseline | Report includes regression warnings | must |
| UT-29 | Performance report with slowest requests | Snapshot with resource timing | Top slowest requests listed | should |
| UT-30 | GetLatestPerformanceSnapshot | Multiple URLs stored | Returns most recently added | must |
| UT-31 | Baseline resource update | New resources in snapshot | Baseline resources updated with weighted average | should |
| UT-32 | Baseline resource first sample | First snapshot with resources | Resources stored directly | should |
| UT-33 | Baseline resource refiltering | Too many resources | Resources filtered to stay within limits | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end performance via MCP | Extension -> performance snapshot POST -> server -> `observe(what: "performance")` | Full performance report with metrics | must |
| IT-2 | Baseline build across page loads | 3 page loads -> query performance | Baseline reflects averaged metrics from all 3 loads | must |
| IT-3 | Regression alert via push notification | Baseline established -> intentional regression -> alert | Alert pushed to changes/health response | must |
| IT-4 | Alert resolution | Regression -> fix -> next snapshot within threshold | Alert marked as resolved | should |
| IT-5 | Multiple batched snapshots | Extension POSTs array of snapshots | All processed, baselines updated for each | must |
| IT-6 | URL-filtered performance check | `observe(what: "performance", url: "/dashboard")` | Only dashboard performance shown | must |
| IT-7 | Health check integration | `configure(action: "health")` | Includes performance regression status | should |
| IT-8 | Concurrent snapshot processing | Multiple snapshots arriving simultaneously | No race conditions, baselines consistent | must |
| IT-9 | Session summary with performance | Generate PR summary with performance data | Performance section includes baseline comparison | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Snapshot storage | Wall clock time | Under 1ms per snapshot | must |
| PT-2 | Baseline update | Wall clock time | Under 1ms per update | must |
| PT-3 | Regression detection | Wall clock time | Under 2ms for all metrics | must |
| PT-4 | Performance report formatting | Wall clock time | Under 5ms | should |
| PT-5 | Memory for snapshots + baselines | Memory footprint | Within maxPerfSnapshots/maxPerfBaselines bounds | must |
| PT-6 | HandlePerformanceSnapshots HTTP | Wall clock time for batch processing | Under 10ms for 5 snapshots | should |
| PT-7 | Alert detection and storage | Wall clock time | Under 5ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No snapshot data | Query performance with no snapshots | Clear "no performance data captured" message | must |
| EC-2 | LRU eviction under pressure | maxPerfSnapshots + 1 snapshots | Oldest evicted, newest stored, no crash | must |
| EC-3 | LRU baseline eviction | maxPerfBaselines + 1 baselines | Oldest baseline evicted | must |
| EC-4 | Snapshot with nil FCP/LCP | Page where FCP/LCP not captured | Handled gracefully, nil fields omitted | must |
| EC-5 | Very fast page (all zeros) | Load=0ms, FCP=0ms | No regression detected, reported correctly | must |
| EC-6 | Very slow page | Load=60000ms | Captured correctly, regression detected if baseline is lower | must |
| EC-7 | Max alerts capped | Many simultaneous regressions | Alert count bounded, note about suppression | must |
| EC-8 | Alert not repeated | Same regression detected on consecutive snapshots | Alert fires once, not on every snapshot | must |
| EC-9 | Concurrent baseline access | Read baseline while another goroutine updates it | RWMutex ensures safe access | must |
| EC-10 | Empty performance snapshot array | POST `[]` to performance endpoint | 200 OK, no processing | must |
| EC-11 | Bad JSON in performance POST | Malformed JSON body | 400 error, no crash | must |
| EC-12 | GET to performance endpoint | GET instead of POST | Method not allowed or appropriate response | must |
| EC-13 | Baseline with zero sample count | Edge case in averaging | No division by zero | must |
| EC-14 | Resource baseline empty snapshot | Snapshot with no resources | Baseline resources unchanged | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web page loaded (preferably one where you can control load time, e.g., by adding network throttling or heavy resources)
- [ ] Page has been loaded at least 3 times to build a baseline

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "observe", "arguments": {"what": "performance"}}` | Page has loaded normally | Performance report with timing (Load, FCP, LCP, TTFB), network (request count, transfer size), and resource breakdown | [ ] |
| UAT-2 | Reload page 3 more times, then: `{"tool": "observe", "arguments": {"what": "performance"}}` | Multiple loads completed | Report shows baseline with sample count, comparison to baseline | [ ] |
| UAT-3 | Add a large (2MB) image to the page, reload, then query performance | Page loads slower with large image | Transfer size regression detected, alert in response | [ ] |
| UAT-4 | Remove the large image, reload, then query performance | Page load normalizes | Previous regression resolves | [ ] |
| UAT-5 | `{"tool": "observe", "arguments": {"what": "performance", "url": "/specific-page"}}` | Check filtering | Only performance data for the specified URL | [ ] |
| UAT-6 | `{"tool": "configure", "arguments": {"action": "health"}}` | Session health check | Performance section included, shows regression status (if any) | [ ] |
| UAT-7 | `{"tool": "observe", "arguments": {"what": "changes"}}` | After performance regression | Changes response includes performance alert in its summary | [ ] |
| UAT-8 | Add a slow synchronous script (blocking main thread for 500ms), reload | Page feels janky | Long tasks and/or TBT regression detected | [ ] |
| UAT-9 | Check the performance report includes resource breakdown | Look at resource types in report | Scripts, stylesheets, images, fonts listed with counts and sizes | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Baseline URLs clean | Query performance for pages with sensitive paths | URLs shown but no PII in query params (params stripped) | [ ] |
| DL-UAT-2 | Resource URLs clean | Check resource entries in performance report | No signed CDN URLs with auth tokens in resource list | [ ] |
| DL-UAT-3 | Alert messages appropriate | Trigger a regression, check alert text | Alert contains metric name, values, recommendation -- no sensitive context | [ ] |
| DL-UAT-4 | Persisted data safe | Inspect baseline persistence (if enabled) | Only aggregated averages and counts, no raw per-request data | [ ] |

### Regression Checks
- [ ] Existing `observe(what: "vitals")` still works alongside performance budget
- [ ] Web vitals (FCP, LCP, CLS) values in performance report match dedicated vitals tool
- [ ] Performance snapshot HTTP endpoint (`POST /performance-snapshots`) accepts both single and batched payloads
- [ ] Extension perf-snapshot.js continues to capture and send snapshots on page load
- [ ] Server memory stays bounded under continuous page reloads (LRU eviction working)
- [ ] Concurrent MCP queries and incoming snapshots do not cause races (verified with `-race` flag)

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
