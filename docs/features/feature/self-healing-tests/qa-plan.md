---
status: proposed
scope: feature/self-healing-tests/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-self-healing-tests
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Self-Healing Tests

> QA plan for the Self-Healing Tests feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification. This feature adds `observe({what: "test_diagnosis"})` and `generate({format: "test_fix"})` modes to existing tools.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Self-Healing Tests ingests test failure messages (which may contain secrets) and correlates them against browser telemetry. The diagnosis response aggregates data from multiple sources (console, network, DOM), creating a combined view that must be redacted.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Test failure message contains hardcoded API keys | Failure message like "Expected header Authorization: Bearer sk-..." is passed in; verify the diagnosis response redacts known secret patterns | critical |
| DL-2 | Diagnosis evidence includes unredacted network headers | `evidence.network_failures` array may include headers from failed requests; verify same stripping as `observe({what: "network_waterfall"})` | critical |
| DL-3 | Diagnosis evidence includes response bodies with PII | `evidence.network_failures` entries may reference response bodies; verify body content follows opt-in rules | high |
| DL-4 | DOM candidate selectors expose sensitive attribute values | `evidence.dom.candidates` shows selectors and text content; verify no password field values, hidden input values, or data-secret attributes leak | high |
| DL-5 | Console error evidence contains secrets from application logs | `evidence.console_errors` may include application log lines with tokens or PII; verify same handling as `observe({what: "errors"})` | high |
| DL-6 | Fix proposal includes sensitive test data | `fix.changes[].old_value` or `new_value` could echo back sensitive data from the failure message; verify redaction | medium |
| DL-7 | `root_cause` and `summary` fields contain raw sensitive data | These human-readable fields may interpolate data from telemetry; verify secrets are not embedded in explanatory text | high |
| DL-8 | `test_file` parameter path traversal | AI passes `test_file: "/etc/passwd"` — verify the server does NOT read or access the file system (test_file is informational only) | medium |
| DL-9 | `related_changes` array leaks detailed DOM diffs with sensitive content | Changes like "input value changed from 'password123' to ''" should not include the actual values | high |
| DL-10 | Diagnosis for cross-origin iframe exposes cross-origin content | DOM candidate search in cross-origin iframes should fail gracefully without exposing cross-origin data | medium |

### Negative Tests (must NOT leak)
- [ ] Diagnosis `evidence.network_failures` must NOT include Authorization, Cookie, or API key headers
- [ ] Diagnosis `evidence.dom.candidates` must NOT include password field values or hidden input values
- [ ] Fix proposal `changes[].old_value` and `new_value` must NOT contain unredacted secrets from the failure message
- [ ] `root_cause` and `summary` strings must apply the same noise/redaction rules as observe() output
- [ ] Server must NOT access `test_file` on disk — it is metadata only, never read

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the diagnosis and fix responses can unambiguously understand the data and take correct action.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Diagnosis category is unambiguous | `category` is one of 9 defined enums; AI can match directly | [ ] |
| CL-2 | Confidence level guides AI behavior | `high` = apply fix automatically; `medium` = apply with warning; `low` = ask human — verify these semantics are clear from the response | [ ] |
| CL-3 | `recommended_action` drives next step | `fix_test` vs `fix_code` vs `mark_flaky` vs `investigate` — verify AI knows exactly what to do for each | [ ] |
| CL-4 | `selector_stale` includes candidate selectors | `evidence.dom.candidates` array with similarity scores helps AI choose replacement | [ ] |
| CL-5 | `api_contract_changed` includes schema diff | Evidence shows expected vs actual field names/types so AI can update assertions | [ ] |
| CL-6 | `timing_issue` includes timing data | `evidence.timing.page_load_ms` and `dom_ready_ms` help AI choose wait strategy | [ ] |
| CL-7 | `true_regression` explicitly says "do NOT fix the test" | Response with `recommended_action: "fix_code"` makes it clear the test is correct | [ ] |
| CL-8 | `unknown` explains what data is missing | When diagnosis is `unknown`, explanation says "No browser telemetry found" or "Extension disconnected" — actionable next step | [ ] |
| CL-9 | Fix proposal `framework_hint` is code-ready | AI can copy the framework_hint directly into a test file (correct syntax for the specified framework) | [ ] |
| CL-10 | Fix proposal `warnings` prevent blind application | When multiple candidates exist or confidence is low, warnings explain why manual review is needed | [ ] |
| CL-11 | Fix `strategy` matches diagnosis `category` | `selector_stale` -> `selector_update`; `timing_issue` -> `wait_adjustment`; etc. — mapping is consistent | [ ] |
| CL-12 | Fix `changes` array is ordered | Changes should be applied in array order (dependency ordering if multiple changes needed) | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI may apply a `fix_test` fix when the diagnosis is `true_regression` — verify the AI checks `recommended_action` BEFORE reading `fix.changes`
- [ ] AI may confuse `selector_stale` (element exists with different selector) with `element_removed` (element is gone) — verify the summary makes this distinction explicit
- [ ] AI may apply fixes from a `low` confidence diagnosis without human review — verify warnings are present for low-confidence fixes
- [ ] AI may call `generate({format: "test_fix"})` without calling `observe({what: "test_diagnosis"})` first — verify test_fix works with manually constructed diagnosis objects (not just piped from test_diagnosis)
- [ ] AI may assume the diagnosis is deterministic — verify that timing-dependent diagnoses (flaky tests) include a note about non-determinism

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Diagnose a test failure | 1 step: `observe({what: "test_diagnosis", failure: {...}})` | No — already a single call |
| Generate a fix proposal | 1 step: `generate({format: "test_fix", diagnosis: {...}})` | No — already a single call |
| Full self-healing workflow | 2 steps: diagnose, then fix | Could combine into 1 call, but separation allows AI to review diagnosis first (intentional) |
| Diagnose + fix + verify | 4 steps: diagnose, fix, apply code change, re-run test | Steps 3-4 are outside Kaboom; cannot simplify |
| Handle "unknown" diagnosis | 3 steps: check observe(page), verify extension connected, retry with broader time window | Could add automatic fallback, but explicit control is better for AI agents |

### Default Behavior Verification
- [ ] `context.since` defaults to last 60 seconds if not provided
- [ ] `framework` parameter is optional — generic output when omitted
- [ ] `fix_strategy` auto-selects based on category when omitted
- [ ] Diagnosis works with only `failure.message` (all other fields optional)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Diagnosis with matching console error | Failure message + console error in buffer matching the message | `category: "js_error"`, evidence includes the matching console error | must |
| UT-2 | Diagnosis with stale selector (element renamed) | Failure "element not found: .submit-btn" + DOM has `.submit-button` | `category: "selector_stale"`, candidates include `.submit-button` with similarity score | must |
| UT-3 | Diagnosis with element removed | Failure "element not found: .old-widget" + DOM has no similar element | `category: "element_removed"` (escalated from selector_stale) | must |
| UT-4 | Diagnosis with network failure | Failure + 500 error in network buffer for related URL | `category: "network_failure"`, evidence includes the 500 response | must |
| UT-5 | Diagnosis with API contract change | Failure about field name + network body showing renamed field | `category: "api_contract_changed"`, evidence shows old vs new field | must |
| UT-6 | Diagnosis with timing issue | Failure with "timeout" in message + slow network in buffer | `category: "timing_issue"`, evidence includes timing data | must |
| UT-7 | Diagnosis with no telemetry | Failure message + empty buffers | `category: "unknown"`, `confidence: "low"`, message about missing telemetry | must |
| UT-8 | Diagnosis with extension disconnected | Failure message + server knows extension is disconnected | `category: "unknown"`, note about extension disconnected | must |
| UT-9 | Diagnosis with `since` filter | Telemetry from 5 minutes ago + `since` 60 seconds ago | Only recent telemetry considered; old data ignored | must |
| UT-10 | Diagnosis default `since` window | No `since` provided | Defaults to last 60 seconds | must |
| UT-11 | Fix for selector_stale | Diagnosis with stale selector + candidate | `strategy: "selector_update"`, changes with old/new selector | must |
| UT-12 | Fix for timing_issue | Diagnosis with timing issue | `strategy: "wait_adjustment"`, changes with wait condition | must |
| UT-13 | Fix for api_contract_changed | Diagnosis with API field rename | `strategy: "api_mock_update"`, changes with old/new field | must |
| UT-14 | Fix for true_regression | Diagnosis with `recommended_action: "fix_code"` | `strategy: "code_fix_needed"`, NO test changes proposed | must |
| UT-15 | Fix for unknown category | Diagnosis with `category: "unknown"` | `strategy: "no_fix_available"`, recommendation to investigate | must |
| UT-16 | Fix with framework hint (Playwright) | `framework: "playwright"` | `framework_hint` contains Playwright-specific code | should |
| UT-17 | Fix with framework hint (Cypress) | `framework: "cypress"` | `framework_hint` contains Cypress-specific code | should |
| UT-18 | Fix with unrecognized framework | `framework: "unknown_fw"` | `framework_hint` contains generic JS suggestion | should |
| UT-19 | Fix with fix_strategy override | `fix_strategy: "wait"` for a selector_stale diagnosis | Strategy overridden to `wait_adjustment` instead of auto-selected `selector_update` | should |
| UT-20 | Fix with multiple candidates | Diagnosis with 3 candidate selectors | Fix uses highest-scoring candidate; warning about multiple matches | must |
| UT-21 | Diagnosis confidence assignment | Various evidence combinations | High confidence: clear single cause. Medium: multiple possibilities. Low: insufficient evidence | must |
| UT-22 | Diagnosis `related_changes` population | Changes in buffer during test window | `related_changes` array populated with relevant state changes | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Diagnosis correlates across buffer types | Log buffer, network buffer, DOM query | Diagnosis reads from multiple buffers to build composite evidence | must |
| IT-2 | Diagnosis uses existing analyze({what: "dom"}) | MCP handler, DOM query system, extension | Selector candidate search delegates to existing analyze({what: "dom"}) infrastructure | must |
| IT-3 | Diagnosis uses error_clusters when available | Error clustering system, diagnosis handler | Grouped errors referenced in diagnosis to avoid duplicate investigation | should |
| IT-4 | Fix integrates with existing test generation | generate handler, test template system | test_fix shares framework-specific template code with `generate({format: "test"})` | should |
| IT-5 | Full workflow: diagnose then fix | observe handler, generate handler | Diagnosis output is valid input for test_fix; round-trip produces actionable fix | must |
| IT-6 | Concurrent diagnosis requests | Multiple MCP requests | Each operates on snapshot of buffer contents; no interference | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | test_diagnosis response time | End-to-end from MCP call to response | < 500ms | must |
| PT-2 | test_fix response time | End-to-end from MCP call to response | < 200ms | must |
| PT-3 | Diagnosis memory impact | Transient memory for copying telemetry during analysis | < 2MB | must |
| PT-4 | DOM candidate search time | Time to find candidates using analyze({what: "dom"}) | < 100ms | must |
| PT-5 | Diagnosis with full 60-second buffer | 1000+ entries in buffers, 60s window scan | Still under 500ms | should |
| PT-6 | Diagnosis with minimal data (empty buffers) | Zero entries, fast-path to "unknown" | < 50ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Failure message with no matching telemetry | Error message about a completely unrelated component | `category: "unknown"`, no false correlation | must |
| EC-2 | Multiple matching candidates for stale selector | DOM has 5 elements with similar selectors | All returned in candidates array, sorted by similarity; warning about ambiguity | must |
| EC-3 | Zero candidates for stale selector | No similar element exists | Category escalates to `element_removed` | must |
| EC-4 | Very large failure message (10KB+) | Extremely verbose test runner output | Truncated or handled without OOM; diagnosis still works | should |
| EC-5 | Failure message in non-English | Error message in Japanese or Chinese characters | String comparison still works; similarity scoring may be less accurate; no crash | should |
| EC-6 | Test ran in iframe or shadow DOM | Selector from shadow DOM context | DOM candidate search handles shadow DOM via extension; notes limitations if cross-origin | should |
| EC-7 | Telemetry from wrong tab | Multi-tab scenario where test ran in tab A but telemetry is from tab B | Diagnosis may correlate incorrectly; no crash. Notes this as a limitation if detected | should |
| EC-8 | Simultaneous `selector_stale` AND `network_failure` | Both selector and network issues in the same test | Both appear in evidence; diagnosis picks primary cause (network failure takes precedence) | should |
| EC-9 | Fix for flaky test | `category: "flaky"` | `strategy: "mark_flaky"` or adds retry logic; does NOT blindly change selectors | must |
| EC-10 | Diagnosis called repeatedly for same failure | Same failure message within seconds | Each call operates on current buffer state; no caching (OI-3 documents this decision) | should |
| EC-11 | `context.since` with very old timestamp (hours) | `since: "2026-01-28T01:00:00Z"` when it is 10:00 | Enforces maximum scan window; returns data within allowed range | should |
| EC-12 | Fix with empty diagnosis (no evidence) | Diagnosis with category unknown and empty evidence | `strategy: "no_fix_available"` with clear explanation | must |
| EC-13 | Framework parameter affects only output format | Same diagnosis with framework: "playwright" vs "cypress" | Different `framework_hint` text but same `changes` structure | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web application running with known elements (e.g., a button with `data-testid="submit-button"`)
- [ ] Browser DevTools open to verify DOM state

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Human triggers a console error: `throw new Error("Test element missing")` | Error in DevTools console | Error captured by extension | [ ] |
| UAT-2 | `{"tool": "observe", "arguments": {"what": "test_diagnosis", "failure": {"message": "Element not found: [data-testid='submit-btn']", "test_name": "Submit form test", "framework": "playwright"}}}` | No visual change | AI receives diagnosis with `category` (likely `selector_stale` or `unknown` depending on DOM state), `confidence`, `summary`, and `evidence` | [ ] |
| UAT-3 | Verify diagnosis evidence | Human checks DOM for actual element | If page has `[data-testid='submit-button']`, diagnosis should show it as a candidate with similarity score | [ ] |
| UAT-4 | `{"tool": "generate", "arguments": {"format": "test_fix", "diagnosis": {"category": "selector_stale", "expected_selector": "[data-testid='submit-btn']", "candidates": [{"selector": "[data-testid='submit-button']", "similarity": 0.92}]}, "framework": "playwright"}}` | No visual change | AI receives fix proposal with `strategy: "selector_update"`, `changes` array with old/new selectors, and Playwright-specific `framework_hint` | [ ] |
| UAT-5 | Verify fix proposal is correct | Human reviews proposed selector change | `old_value` is `[data-testid='submit-btn']`, `new_value` is `[data-testid='submit-button']`, framework_hint shows Playwright locator syntax | [ ] |
| UAT-6 | `{"tool": "observe", "arguments": {"what": "test_diagnosis", "failure": {"message": "Request failed: POST /api/users returned 500"}, "context": {"since": "2026-01-28T00:00:00Z"}}}` | No visual change | AI receives diagnosis — if server has 500 errors in buffer, `category: "network_failure"`; otherwise `category: "unknown"` | [ ] |
| UAT-7 | `{"tool": "observe", "arguments": {"what": "test_diagnosis", "failure": {"message": "Timeout waiting for selector .loading-spinner to be hidden"}}}` | No visual change | AI receives diagnosis with `category: "timing_issue"` (if timing evidence exists) or `category: "unknown"` | [ ] |
| UAT-8 | `{"tool": "generate", "arguments": {"format": "test_fix", "diagnosis": {"category": "timing_issue"}, "framework": "playwright"}}` | No visual change | Fix proposal with `strategy: "wait_adjustment"`, `framework_hint` showing Playwright waitForSelector or waitForLoadState | [ ] |
| UAT-9 | `{"tool": "observe", "arguments": {"what": "test_diagnosis", "failure": {"message": "Some error"}, "context": {"since": "2099-01-01T00:00:00Z"}}}` | No visual change | Diagnosis: `category: "unknown"`, `confidence: "low"` because no telemetry in that future window | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Diagnosis does not expose auth headers | Trigger a failed request with Authorization header, then diagnose | `evidence.network_failures` entries do NOT include the Authorization header value | [ ] |
| DL-UAT-2 | DOM candidates do not expose password values | Page has a password input, run diagnosis with selector matching the password field | Candidates list shows the selector and tag but NOT the password value | [ ] |
| DL-UAT-3 | Fix proposal does not echo back secrets | Pass a failure message containing "Bearer sk-12345..." and generate a fix | Fix changes, summary, and framework_hint do not contain the token string | [ ] |

### Regression Checks
- [ ] Existing `observe({what: "errors"})` still works normally
- [ ] Existing `generate({format: "test"})` still works normally
- [ ] `analyze({what: "dom"})` still works as a standalone action
- [ ] `observe({what: "changes"})` still works independently
- [ ] No new MCP tools created (still exactly 5 tools)

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
