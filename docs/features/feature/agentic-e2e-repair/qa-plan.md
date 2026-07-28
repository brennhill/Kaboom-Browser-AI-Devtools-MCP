---
status: shipped
scope: feature/agentic-e2e-repair/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-agentic-e2e-repair
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Agentic E2E Repair

> QA plan for the Agentic E2E Repair feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification. This is a WORKFLOW PATTERN that orchestrates existing Kaboom MCP tools — it does NOT introduce new tools or server features. QA focuses on the correctness and safety of the multi-tool orchestration sequence.

---

## 1. Data Leak Analysis

**Goal:** Verify the workflow does NOT expose data it shouldn't. Agentic E2E Repair aggregates data from multiple tool calls (errors, network bodies, DOM state, API schemas) into a diagnosis. The combined view is higher-risk than individual tool calls because the agent reads and correlates data across multiple sources, potentially building a composite picture that leaks more than any single observation.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Agent reads network bodies with auth tokens during diagnosis | `observe({what: "network_bodies"})` may return response bodies containing session tokens or API keys; verify the agent does not include these in PR descriptions or commit messages | critical |
| DL-2 | Agent includes real API response data in generated test fixtures | `generate({what: "test", include_fixtures: true})` may create fixtures from observed traffic containing PII; verify fixture sanitization applies | critical |
| DL-3 | Agent exposes DOM state with sensitive content | `analyze({what: "dom"})` returns DOM structure including input values; verify password fields and hidden inputs are not exposed | high |
| DL-4 | Agent writes reproduction script with embedded credentials | `generate({what: "reproduction"})` may include URLs with tokens in query params; verify URL redaction | high |
| DL-5 | Agent includes observed error stack traces with file paths in PR | Error clusters may contain server-side file paths; verify the agent's output follows existing redaction rules | medium |
| DL-6 | Agent uses `execute_js` to inspect sensitive runtime state | During verification, agent might run `localStorage.getItem('token')` to check state; verify this stays within MCP response (localhost-only) | high |
| DL-7 | Agent saves/loads browser state containing auth cookies | `interact({what: "save_state"})` captures cookies and localStorage; verify snapshots are not persisted beyond the session | medium |
| DL-8 | Batch repair correlates data across multiple test runs | Agent combines observations from multiple test failures; composite diagnosis may reveal more about the system than individual observations | medium |
| DL-9 | Agent commit message contains sensitive diagnostic data | Agent generates PR summary or commit message from diagnosis; verify no auth tokens, PII, or raw API responses in the text | high |
| DL-10 | Circuit breaker failure leaves diagnostic data in memory | After 3 failed attempts, diagnostic data from all attempts remains in conversation context; verify no persistence to disk | medium |
| DL-11 | `validate_api` response exposes internal API schema details | Schema analysis may reveal internal field names, types, and relationships; verify this is acceptable for localhost-only usage | low |
| DL-12 | `diff_sessions` exposes differences between passing and failing states | Session diff may show state changes that include sensitive configuration changes; verify redaction applies | medium |

### Negative Tests (must NOT leak)
- [ ] Generated test fixes must NOT contain hardcoded auth tokens from observed traffic
- [ ] Generated reproduction scripts must NOT include URLs with unredacted sensitive query parameters
- [ ] PR descriptions/commit messages must NOT include raw API response bodies or error stack traces with secrets
- [ ] Browser state snapshots used during repair must NOT be persisted to disk
- [ ] Diagnostic data from failed repair attempts must NOT be logged to any file
- [ ] Fixture data in generated tests must NOT contain real PII from observed responses

---

## 2. LLM Clarity Assessment

**Goal:** Verify the AI agent can follow the repair workflow correctly using existing tool responses, without misinterpreting data or taking wrong actions.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Error output from test runner is parseable | Agent can extract test name, error message, and stack trace from common test runner formats (Playwright, Cypress, Jest) | [ ] |
| CL-2 | `observe({what: "errors"})` distinguishes app errors from test errors | Console errors from the application are distinguishable from test framework errors; verify error source is clear | [ ] |
| CL-3 | `observe({what: "network_bodies"})` shows actual vs expected | Agent can compare observed API response shape against what the test asserts; verify field names and types are clear | [ ] |
| CL-4 | `analyze({what: "dom"})` confirms selector existence | Response clearly shows whether a selector matched (count > 0) or not (count = 0); agent can determine selector drift | [ ] |
| CL-5 | `configure({what: "validate_api"})` classifies contract changes | Violations are categorized (shape_change, type_change, new_field, null_field); agent can map these to fix strategies | [ ] |
| CL-6 | `observe({what: "error_clusters"})` groups related failures | Clusters show multiple errors with a single root cause; agent can fix the root cause once instead of each error | [ ] |
| CL-7 | `generate({what: "test"})` produces framework-correct code | Generated test code uses correct Playwright/Cypress/Selenium syntax based on detected or specified framework | [ ] |
| CL-8 | Root cause category is actionable | Each of 5 categories (selector drift, API contract drift, timing fragility, mock staleness, true regression) maps to a clear fix strategy | [ ] |
| CL-9 | "True regression" stops the agent from modifying tests | When root cause is a true regression, the agent reports rather than fixes; verify the diagnostic evidence makes this clear | [ ] |
| CL-10 | Circuit breaker message is unambiguous | After 3 failed attempts, the agent knows to stop and escalate; verify error patterns are distinguishable from transient issues | [ ] |
| CL-11 | `diff_sessions` output shows what changed | Session diff clearly identifies which DOM elements, network responses, or error patterns differ between passing and failing | [ ] |
| CL-12 | Batch repair grouping is clear | When multiple tests fail from the same root cause, error clusters help the agent identify the common cause | [ ] |

### Common LLM Misinterpretation Risks
- [ ] Agent may fix a test when the application has a true regression — verify the diagnosis step distinguishes "test is wrong" from "app is broken" using error_clusters and API validation
- [ ] Agent may apply the same fix to multiple files when a batch fix is needed — verify error_clusters guide the agent to a single root cause fix
- [ ] Agent may enter an infinite fix-break loop — verify the circuit breaker (3 attempts) is enforced and the agent escalates
- [ ] Agent may confuse "API returned 500" (true regression) with "API returned 200 with different fields" (contract drift) — verify network_waterfall status codes guide the classification
- [ ] Agent may generate fixes for a framework it does not recognize — verify the agent checks framework detection before generating code
- [ ] Agent may not clear Kaboom buffers between test runs — verify `configure({what: "clear"})` is called before re-running tests to avoid stale data

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** High

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Diagnose a single test failure | 3-5 tool calls: observe errors, observe network, query DOM, optionally validate_api, observe error_clusters | Could bundle into a single "diagnose" meta-call, but composability is more flexible |
| Generate a fix for selector drift | 1-2 tool calls: analyze({what: "dom"}) to find candidates, generate test with updated selector | No — already minimal for this specific case |
| Full repair cycle (single test) | 6-8 tool calls: observe (multiple), query DOM, validate API, generate fix, verify | This is inherently a multi-step workflow; each step provides valuable information |
| Batch repair (multiple tests) | 10-15 tool calls: run all tests, collect error clusters, group, fix root cause, re-run all | Batch grouping via error_clusters reduces this vs fixing each test independently |
| Verify fix | 2 steps: re-run test, observe results | Cannot simplify — verification requires execution |

### Default Behavior Verification
- [ ] All tools used in the workflow have their default settings (no special configuration needed to start)
- [ ] `observe({what: "errors"})` works without prior configuration
- [ ] `analyze({what: "dom"})` works without prior configuration
- [ ] `generate({what: "test"})` auto-detects framework when not specified
- [ ] No pre-enablement required for the diagnosis workflow (unlike AI Web Pilot which requires a toggle)
- [ ] Network body capture must be enabled for API contract drift detection — this is a prerequisite the agent must check

---

## 4. Code Test Plan

### 4.1 Unit Tests

Since Agentic E2E Repair is a workflow pattern (not new server code), unit tests focus on the existing tools that the workflow depends on, ensuring they return the data the workflow needs.

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | `observe({what: "errors"})` returns errors with stack traces | Populate error buffer with errors | Errors with message, stack, timestamp, source | must |
| UT-2 | `observe({what: "network_waterfall"})` returns status codes | Populate network buffer | Entries with URL, status, method, timing | must |
| UT-3 | `observe({what: "network_bodies"})` returns response bodies | Populate body buffer | Entries with URL, status, response body (redacted) | must |
| UT-4 | `observe({what: "api"})` returns inferred schema | API traffic captured | Schema with field names, types, nullable | must |
| UT-5 | `analyze({what: "dom", selector: ".submit-btn"})` returns match count | DOM with matching/non-matching elements | `{ count: N, matches: [...] }` or `{ count: 0 }` | must |
| UT-6 | `configure({what: "validate_api"})` detects shape_change | Schema drift in traffic | Violations with category and details | must |
| UT-7 | `observe({what: "error_clusters"})` groups related errors | Multiple related errors in buffer | Clusters with representative error and count | must |
| UT-8 | `observe({what: "changes"})` shows recent changes | DOM or network changes captured | Change entries with type, description, timestamp | must |
| UT-9 | `configure({what: "diff_sessions"})` compares states | Two session snapshots available | Diff showing DOM, network, error differences | should |
| UT-10 | `generate({what: "test"})` produces valid Playwright code | Actions and assertions captured | Syntactically valid Playwright test | must |
| UT-11 | `generate({what: "test", include_fixtures: true})` adds fixtures | Network bodies available | Test with beforeAll API setup calls | must |
| UT-12 | `generate({what: "reproduction"})` produces reproduction steps | Actions captured | Playwright script reproducing the observed behavior | must |
| UT-13 | `configure({what: "clear"})` resets buffers | Populated buffers | All buffers empty after clear | must |
| UT-14 | `interact({what: "navigate", url: "..."})` navigates browser | Valid URL | Browser navigates to URL | must |
| UT-15 | `interact({what: "execute_js", script: "..."})` runs in page | Valid JS expression | Result returned from page context | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Selector drift detection workflow | analyze({what: "dom"}), error buffer, DOM query system | Agent queries DOM with test's selector -> no match -> queries with relaxed selector -> finds match -> classifies as selector drift | must |
| IT-2 | API contract drift detection workflow | network_bodies, validate_api, API schema | Agent observes network bodies -> validates API contract -> detects field rename -> classifies as API drift | must |
| IT-3 | Timing issue detection workflow | network_waterfall, error buffer | Agent observes slow network response overlapping with test action -> classifies as timing issue | should |
| IT-4 | True regression detection workflow | error_clusters, network_bodies | Agent observes application-level errors (not test errors) + new error cluster -> classifies as true regression -> does NOT generate test fix | must |
| IT-5 | Batch repair workflow | error_clusters across multiple runs | Agent runs multiple failing tests, groups by error cluster, fixes root cause once, verifies all tests pass | should |
| IT-6 | Circuit breaker workflow | Multiple generate + verify cycles | Agent generates fix, verifies, fix fails, retries 2 more times, escalates after 3rd failure | must |
| IT-7 | Buffer clear between test runs | configure clear, observe | Agent clears buffers before re-running test; new observations reflect only the current run | must |
| IT-8 | Full end-to-end repair: selector drift | All components | Agent detects selector drift, generates updated test, verifies fix passes | must |
| IT-9 | Full end-to-end repair: API contract drift | All components | Agent detects API change, generates updated test with new assertions, verifies | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Diagnosis phase (all observations) | Total time for 3-5 observe calls | < 30 seconds (spec SLO) | must |
| PT-2 | Fix generation time | Time for generate({what: "test"}) | < 10 seconds (spec SLO) | must |
| PT-3 | Server memory during repair workflow | Additional memory from tool calls | Zero additional (uses existing buffers) | must |
| PT-4 | Token efficiency for batch repair | Tokens used for N related failures vs N independent repairs | < 2x tokens of single repair | should |
| PT-5 | Buffer clear time | Time for configure({what: "clear"}) | < 10ms | should |
| PT-6 | DOM query response time | Time for analyze({what: "dom"}) with complex selector | < 500ms | must |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No Kaboom data captured during test run | Extension not connected or server not running | Agent reports "insufficient data"; suggests re-running with Kaboom capture | must |
| EC-2 | Test fails with no browser errors | No console errors, no network failures | Agent queries DOM for selector validity; if selectors valid, classifies as timing or unknown | must |
| EC-3 | Multiple root causes in single test | Both selector drift AND API contract change | Agent addresses in dependency order: API contract first, then selectors | should |
| EC-4 | Unrecognized test framework | Agent cannot detect whether test uses Playwright, Cypress, or Selenium | Generates generic JS fixes; reports framework detection failure | should |
| EC-5 | Extension disconnected mid-test | Partial telemetry captured | Agent works with partial data; notes that capture was incomplete | must |
| EC-6 | Ring buffer overflow during long test suite | 1000+ log entries evict oldest | Agent uses configure({what: "clear"}) between tests; diagnoses with available data | should |
| EC-7 | Test was already flaky | Test passes intermittently regardless of agent's changes | Agent runs test twice after fix; if inconsistent, classifies as flaky and reports | should |
| EC-8 | Fix breaks a different test | Agent's fix causes another test to fail | Circuit breaker activates; agent reports regression | must |
| EC-9 | API returns 500 error (server broken) vs 200 with changed fields | Different HTTP status codes | Agent distinguishes: 500 = true regression (report); 200 with different fields = contract drift (fix test) | must |
| EC-10 | Parallel test runners posting to same Kaboom | Multiple Playwright workers | Cross-contamination of telemetry; agent may misdiagnose. Notes limitation (OI-8) | should |
| EC-11 | Agent tries to modify more than 10 files | Batch repair across many test files | Agent should request human approval before modifying more than 10 files (OI-3) | should |
| EC-12 | Network body capture disabled | Agent tries API contract analysis | Agent detects missing data; suggests enabling body capture via configure({what: "capture", settings: {network_bodies: true}}) | must |
| EC-13 | Very stale test (months without update) | Many selectors, mocks, and assertions are outdated | Agent addresses one issue at a time; may need multiple repair cycles | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI orchestrates the repair workflow using MCP tool calls; the human observes browser behavior and reviews proposed fixes.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web application running (e.g., localhost:3000) with known UI elements
- [ ] A failing E2E test with known root cause (for controlled verification)
- [ ] Test runner installed (Playwright recommended)
- [ ] Network body capture enabled for API contract testing

### Step-by-Step Verification: Selector Drift Repair

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Human creates a test that uses `.submit-btn` but the page has `.btn-submit` | Test fails with "element not found" | Failing test output available | [ ] |
| UAT-2 | Human navigates to the page with the mismatched selector | Page loads in Kaboom-tracked browser | Extension captures DOM state | [ ] |
| UAT-3 | `{"tool": "observe", "arguments": {"what": "errors"}}` | No visual change | AI receives any console errors from the test run | [ ] |
| UAT-4 | `{"tool": "analyze", "arguments": {"what": "dom", "selector": ".submit-btn"}}` | No visual change | AI receives `{ count: 0 }` — selector does not match | [ ] |
| UAT-5 | `{"tool": "analyze", "arguments": {"what": "dom", "selector": "button"}}` | No visual change | AI receives `{ count: N }` with button elements; can see `.btn-submit` exists | [ ] |
| UAT-6 | AI diagnoses: selector drift — `.submit-btn` renamed to `.btn-submit` | Human confirms diagnosis is correct | Agent identifies correct root cause category | [ ] |
| UAT-7 | `{"tool": "generate", "arguments": {"type": "test", "include_fixtures": false}}` | No visual change | AI receives generated test with updated selector `.btn-submit` instead of `.submit-btn` | [ ] |
| UAT-8 | Human reviews generated test code | Test code visible in AI output | Updated selector is correct; test structure matches original; no other unintended changes | [ ] |

### Step-by-Step Verification: API Contract Drift Repair

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-9 | Human creates a test asserting `response.userName` but API now returns `response.user_name` | Test fails with assertion error | Failing test output available | [ ] |
| UAT-10 | `{"tool": "observe", "arguments": {"what": "network_bodies", "url_filter": "/api/users"}}` | No visual change | AI receives actual API response showing `user_name` field | [ ] |
| UAT-11 | `{"tool": "configure", "arguments": {"what": "validate_api", "operation": "analyze"}}` | No visual change | AI receives contract violations showing `shape_change` for userName -> user_name | [ ] |
| UAT-12 | AI diagnoses: API contract drift — field renamed | Human confirms diagnosis | Agent correctly identifies API field rename as root cause | [ ] |
| UAT-13 | AI proposes fix: update test assertion from `userName` to `user_name` | Human reviews proposed fix | Fix is targeted to the assertion only; no other test changes | [ ] |

### Step-by-Step Verification: True Regression Detection

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-14 | Human introduces a real bug: application throws error on button click | Button click causes error | Error visible in DevTools | [ ] |
| UAT-15 | `{"tool": "observe", "arguments": {"what": "errors"}}` | No visual change | AI sees application error (not test error) | [ ] |
| UAT-16 | `{"tool": "observe", "arguments": {"what": "error_clusters"}}` | No visual change | AI sees new error cluster that did not exist before | [ ] |
| UAT-17 | AI diagnoses: true regression — application code is broken | Human confirms this is an app bug, not a test bug | Agent correctly identifies this as "fix_code" (not "fix_test") | [ ] |
| UAT-18 | AI does NOT propose a test fix; instead generates reproduction | No visual change | AI uses `generate({what: "reproduction"})` to create reproduction script | [ ] |

### Step-by-Step Verification: Circuit Breaker

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-19 | AI attempts fix #1, re-runs test, test still fails | Test fails | Agent notes first attempt failed | [ ] |
| UAT-20 | AI attempts fix #2, re-runs test, test still fails | Test fails | Agent notes second attempt failed | [ ] |
| UAT-21 | AI attempts fix #3, re-runs test, test still fails | Test fails | Agent stops, reports "3 fix attempts exhausted; escalating to human review" with all diagnostic evidence | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Generated test fix contains no auth tokens | Review generated test code after API contract repair | No Authorization headers, Bearer tokens, or API keys in the generated code | [ ] |
| DL-UAT-2 | Reproduction script has redacted URLs | Review generated reproduction script | URLs with `?token=` or `?api_key=` have values masked | [ ] |
| DL-UAT-3 | Agent commit message has no sensitive data | Review proposed commit message/PR description | No raw API responses, passwords, or PII in the text | [ ] |
| DL-UAT-4 | Browser state snapshots are session-only | Restart Kaboom server, check for persisted snapshots | No snapshot data survives server restart (stored in extension chrome.storage.local, not server disk) | [ ] |

### Regression Checks
- [ ] All individual MCP tools (observe, generate, configure, interact) work normally outside the repair workflow
- [ ] Extension capture behavior unaffected by the repair workflow
- [ ] No new MCP tools created (still exactly 5 tools — observe, generate, configure, interact, analyze)
- [ ] Server performance unaffected by the multi-tool orchestration pattern
- [ ] Existing test generation (`generate({what: "test"})`) still works for non-repair use cases

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
