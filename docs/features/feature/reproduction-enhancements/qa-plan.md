---
status: shipped
scope: feature/reproduction-enhancements/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-reproduction-enhancements
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Reproduction Enhancements

> QA plan for the Reproduction Enhancements feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification. This feature adds screenshot capture, visual assertions, data fixture generation, and bug report generation to reproduction scripts and test generation.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Reproduction Enhancements capture SCREENSHOTS of page state (which may show sensitive content), generate DATA FIXTURES from observed API traffic (which may contain PII/credentials), and produce BUG REPORTS with embedded screenshots. All three outputs are high-risk for data leakage.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Screenshots capture password fields with visible values | Screenshot taken while a password field has focus or shows characters; verify screenshots are stored in memory only and returned via MCP only | critical |
| DL-2 | Screenshots capture sensitive page content (PII, financial data) | Screenshot of a bank statement or health record page; verify screenshots stay in server memory buffer only (not written to disk) | critical |
| DL-3 | Screenshots embedded in bug report as base64 persist beyond session | Bug report with embedded base64 screenshots is returned as text; if the AI writes it to a file, the screenshots persist. Verify Kaboom does not write bug reports to disk | high |
| DL-4 | Data fixtures contain real user credentials | Fixture generation from observed API POST to `/api/login` with `{ email, password }` in request body; verify sensitive fields are replaced with test-safe values | critical |
| DL-5 | Data fixtures contain real API keys in request headers | Fixture setup calls include Authorization headers from observed traffic; verify headers are stripped per existing redaction rules | critical |
| DL-6 | Data fixtures contain PII from response bodies | Response bodies with names, emails, phone numbers; verify fixture simplification removes or anonymizes non-essential PII | high |
| DL-7 | Screenshot buffer in memory accessible via HTTP endpoint | Verify no HTTP endpoint exposes the screenshot buffer; screenshots are only accessible via MCP tool responses | high |
| DL-8 | `chrome.debugger` attachment exposes page to extension | When debugger is attached for full-page screenshots, verify it does not grant additional capabilities beyond screenshot capture | medium |
| DL-9 | Auto-screenshot on error captures sensitive error states | Error page showing stack trace with database connection strings or environment variables; screenshots may capture these | high |
| DL-10 | Thumbnail generation preserves sensitive content at lower resolution | 320px thumbnails still readable enough to expose text content; verify thumbnails are treated with same security as full screenshots | medium |
| DL-11 | GraphQL fixture generation includes sensitive query variables | GraphQL mutations with `password` or `token` variables; verify variable values are sanitized | high |
| DL-12 | Auth-dependent fixtures include plaintext credentials | Login step in fixture includes hardcoded username/password from observed session; verify these are replaced with placeholder values | critical |

### Negative Tests (must NOT leak)
- [ ] Screenshots must NOT be written to disk by the Kaboom server (memory buffer only)
- [ ] Screenshots must NOT be accessible via any HTTP endpoint (MCP tool response only)
- [ ] Data fixtures must NOT contain real passwords, API keys, or auth tokens from observed traffic
- [ ] Data fixtures must NOT contain real PII (emails, phone numbers) unless they are essential to the test (and even then, anonymized)
- [ ] Bug report generation must NOT write the report to disk (returned as MCP response text only)
- [ ] `chrome.debugger` must be detached after screenshot capture completes
- [ ] Auto-screenshot configuration must NOT be persisted (session-scoped)

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data and use it effectively.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Screenshot data format is clear | Response includes base64 PNG data with clear field name (`screenshot_base64` or similar); AI knows how to embed it | [ ] |
| CL-2 | Screenshot metadata identifies context | Each screenshot has trigger type (`navigation`, `action`, `error`, `explicit`), timestamp, and URL | [ ] |
| CL-3 | Visual assertion code is syntactically valid | `toHaveScreenshot()` calls in generated tests are valid Playwright syntax with correct parameters | [ ] |
| CL-4 | Fixture code is executable | Generated `beforeAll` block with API calls uses valid `request.post()` syntax and correct endpoints | [ ] |
| CL-5 | Bug report is valid Markdown | Generated bug report uses proper Markdown headers, image syntax, and code blocks | [ ] |
| CL-6 | Mask annotations identify dynamic elements | Generated visual assertions include `mask` arrays for elements identified as dynamic (timestamps, counters) | [ ] |
| CL-7 | Fixture distinguishes setup vs action | Generated fixtures clearly separate "setup" (beforeAll) from "test actions" (test body) with comments | [ ] |
| CL-8 | `capture_screenshot` response confirms capture | Response includes success, base64 data, size, and capture metadata | [ ] |
| CL-9 | Screenshot buffer limit communicated | When buffer is full (20 entries), response indicates eviction happened and oldest screenshot details | [ ] |
| CL-10 | `include_screenshots` effect is visible in generated script | Reproduction script with `include_screenshots: true` contains `page.screenshot()` calls at each action point; without it, no screenshot calls | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI may try to use screenshot base64 data directly in a PR description — verify the data is clearly labeled as base64 PNG, not a URL
- [ ] AI may assume `assert_visual: true` works without first establishing golden screenshots — verify the generated test includes a comment about first-run baseline creation
- [ ] AI may interpret `mask_dynamic: true` as masking ALL dynamic content — verify only elements that changed between observations are masked
- [ ] AI may confuse `include_screenshots` (adds screenshot calls to script) with `capture_screenshot` (captures a screenshot now) — verify these are distinct actions
- [ ] AI may not realize `include_fixtures` requires network body capture to be enabled — verify error message when no network bodies are available

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Medium

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Capture a screenshot | 1 step: `capture_screenshot` tool call | No — already minimal |
| Generate reproduction with screenshots | 1 step: `generate({what: "reproduction", include_screenshots: true})` | No — single parameter adds screenshots |
| Generate test with visual assertions | 1 step: `generate({what: "test", assert_visual: true})` | No — single parameter adds assertions |
| Generate test with fixtures | 1 step: `generate({what: "test", include_fixtures: true})` | No — single parameter adds fixtures |
| Generate full bug report | 1 step: `generate({what: "bug_report"})` | No — already minimal |
| Configure auto-screenshot | 1 step: configure auto_screenshot settings | No — already minimal |
| Full workflow: browse, capture, generate report | 3+ steps: browse site, trigger screenshots, generate report | Inherently multi-step; each step is minimal |

### Default Behavior Verification
- [ ] Auto-screenshot is off by default (only `on_error: true` is default for auto-capture)
- [ ] `include_screenshots` defaults to false in reproduction scripts
- [ ] `assert_visual` defaults to false in test generation
- [ ] `include_fixtures` defaults to false
- [ ] `mask_dynamic` defaults to true when `assert_visual` is true
- [ ] Screenshot buffer starts empty (no pre-capture)
- [ ] `maxDiffPixels` defaults to 100 in generated visual assertions

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | `capture_screenshot` returns valid base64 PNG | Trigger capture on a page | Response with `success: true`, base64 data starting with PNG header, size < 500KB | must |
| UT-2 | `capture_screenshot` with selector | `{ selector: "#specific-element" }` | Screenshot of specific element only (cropped) | must |
| UT-3 | `capture_screenshot` with `full_page: true` | Full page capture | Screenshot includes below-the-fold content | should |
| UT-4 | Screenshot buffer FIFO at 20 entries | Capture 21 screenshots | Buffer has 20; oldest evicted; metadata preserved | must |
| UT-5 | Screenshot resized if over 500KB | Capture of high-resolution page | Resulting PNG <= 500KB; width capped at 1280px | must |
| UT-6 | Thumbnail generated at 320px width | Any screenshot capture | Thumbnail available alongside full screenshot | should |
| UT-7 | `get_reproduction_script` with `include_screenshots` | Actions captured + `include_screenshots: true` | Script contains `page.screenshot()` calls at each action point | must |
| UT-8 | `get_reproduction_script` without `include_screenshots` | Actions captured + `include_screenshots: false` | Script has NO screenshot calls | must |
| UT-9 | `generate_test` with `assert_visual` | Actions captured + `assert_visual: true` | Test contains `toHaveScreenshot()` assertions | must |
| UT-10 | `generate_test` with `mask_dynamic` | Dynamic elements detected + `assert_visual: true` + `mask_dynamic: true` | Test contains `mask: [page.locator(...)]` arrays | must |
| UT-11 | `generate_test` with `include_fixtures` | Network bodies captured + `include_fixtures: true` | Test contains `beforeAll` block with API setup calls | must |
| UT-12 | Fixture sanitizes passwords | POST /api/login with password in body | Fixture replaces password with "test-password" or similar | must |
| UT-13 | Fixture sanitizes email addresses | Response body with real email | Fixture replaces with "<test@example.com>" | should |
| UT-14 | Fixture trims arrays to minimum | Response with 500-item array | Fixture creates minimum needed (e.g., 11 for "more than 10" test) | should |
| UT-15 | Fixture includes only relevant fields | Complex response object | Only fields affecting the test are included (IDs, names, statuses) | should |
| UT-16 | `generate_bug_report` produces valid Markdown | Screenshots + actions + errors captured | Markdown with headers, embedded images, steps, error details | must |
| UT-17 | Bug report includes reproduction script | Full session captured | Bug report ends with a code block containing the Playwright script | should |
| UT-18 | Auto-screenshot on error captures page state | Console error triggered + `on_error: true` | Screenshot automatically captured and stored in buffer | must |
| UT-19 | Auto-screenshot on navigation | Page navigation + `on_navigation: true` | Screenshot captured after page load + 1s settling | should |
| UT-20 | GraphQL fixture includes query string | POST with GraphQL body | Fixture includes the mutation/query string, not just URL | should |
| UT-21 | Auth-dependent fixture includes login step | Session started with POST /api/login | Fixture includes sanitized login call in setup | should |
| UT-22 | Fallback to `captureVisibleTab` when debugger unavailable | Debugger permission denied | Capture uses `chrome.tabs.captureVisibleTab`; full_page and selector captures unavailable; error explains limitation | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Screenshot capture round-trip | Extension background.js (debugger API), server MCP handler, screenshot buffer | Extension captures screenshot, sends to server, AI retrieves via tool call | must |
| IT-2 | Reproduction script with screenshots end-to-end | Action capture, screenshot buffer, codegen engine | Captured actions + screenshots produce executable Playwright script with screenshots | must |
| IT-3 | Test generation with visual assertions and fixtures | Network body buffer, action buffer, screenshot buffer, codegen engine | Generated test includes beforeAll fixtures, test actions, and toHaveScreenshot assertions | must |
| IT-4 | Bug report generation with all components | All buffers, codegen engine | Bug report includes screenshots, steps, errors, and reproduction script | must |
| IT-5 | Auto-screenshot triggers on error | Extension error capture, screenshot trigger, server buffer | Console error triggers automatic screenshot; both error and screenshot stored | should |
| IT-6 | Fixture generation from GraphQL traffic | Extension network capture, server body buffer, fixture generator | GraphQL POST captured with body; fixture uses correct query/mutation | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Screenshot capture latency | Time from capture request to base64 response | < 200ms | must |
| PT-2 | Thumbnail generation time | Time to resize from full screenshot | < 50ms | should |
| PT-3 | Screenshot buffer memory | Memory for 20 screenshots at max size | < 10MB (20 x 500KB) | must |
| PT-4 | Fixture derivation time | Time to scan network body buffer | < 20ms | must |
| PT-5 | Bug report generation time | Time for markdown + base64 embedding | < 50ms | must |
| PT-6 | Page rendering impact | CLS and LCP during screenshot capture | No degradation (using compositor output) | must |
| PT-7 | Reproduction script generation with 20 screenshots | Time to embed all screenshots | < 200ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Screenshot of blank/empty page | Capture on about:blank | Valid (likely white) PNG returned; no error | should |
| EC-2 | Screenshot with debugger already attached | Another extension has debugger attached | Fallback to captureVisibleTab; note in response | should |
| EC-3 | Very long page (full_page > 16384px) | Full page capture of extremely tall page | Cropped at 16384px with note about truncation | should |
| EC-4 | Screenshot of page with animations | Page has CSS animations or video | Single frame captured; note about non-deterministic animation state | should |
| EC-5 | Fixture from response with circular JSON | API response that cannot be JSON-serialized | Skip this response or sanitize; no crash | must |
| EC-6 | Fixture from very large response body (>10KB) | API returns 1MB JSON response | Only first 10KB used; fixture notes "generate N items with these fields" | must |
| EC-7 | Fixture from file upload request | POST with multipart/form-data | Skip file content; fixture notes "file upload required" | should |
| EC-8 | Bug report with no screenshots | No screenshots captured, but errors exist | Bug report generated without images; text-only steps and errors | must |
| EC-9 | Bug report with no errors | Screenshots captured but no errors | Bug report includes screenshots and actions but no error section | should |
| EC-10 | Cross-origin iframe in screenshot | Page has cross-origin iframe | Iframe renders but may show blank/white content; note in response | should |
| EC-11 | Screenshot capture during page navigation | Capture requested while page is loading | Either captures current state or returns error; no crash | must |
| EC-12 | `include_fixtures` with no network bodies captured | Network body capture is off | Error or warning: "Enable network body capture for fixture generation" | must |
| EC-13 | Multiple simultaneous capture requests | 5 capture_screenshot calls at once | Each returns a screenshot; buffer updates correctly; no race condition | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test web application running (e.g., localhost:3000) with a form, buttons, and API calls
- [ ] Network body capture enabled (if testing fixtures)
- [ ] Browser DevTools open for verification

### Step-by-Step Verification

#### Screenshot Capture

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | `{"tool": "interact", "arguments": {"what": "capture_screenshot"}}` | Brief flash or no visible change (captureVisibleTab) | AI receives response with `success: true`, base64 PNG data, size in bytes, trigger: "explicit" | [ ] |
| UAT-2 | `{"tool": "interact", "arguments": {"what": "capture_screenshot", "selector": "h1"}}` | No visible change | AI receives cropped screenshot of h1 element only | [ ] |
| UAT-3 | Human triggers `throw new Error("screenshot test")` in DevTools (with on_error auto-screenshot enabled) | Error in console | AI calls `observe({what: "errors"})` and sees the error; screenshot buffer has an auto-captured screenshot with trigger: "error" | [ ] |

#### Reproduction Script with Screenshots

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-4 | Human performs actions on the page: click button, type in input, navigate | Actions visible on page | Actions captured by extension | [ ] |
| UAT-5 | `{"tool": "generate", "arguments": {"type": "reproduction", "include_screenshots": true}}` | No visual change | AI receives Playwright script with `page.screenshot()` calls at each action point; response includes base64 screenshots as metadata | [ ] |
| UAT-6 | `{"tool": "generate", "arguments": {"type": "reproduction", "include_screenshots": false}}` | No visual change | AI receives Playwright script WITHOUT screenshot calls | [ ] |

#### Test Generation with Visual Assertions and Fixtures

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-7 | `{"tool": "generate", "arguments": {"type": "test", "assert_visual": true}}` | No visual change | AI receives test with `toHaveScreenshot()` assertions at key points | [ ] |
| UAT-8 | Verify generated test has mask annotations | Human reviews generated test code | Dynamic elements (timestamps, counters) have `mask` arrays in screenshot assertions | [ ] |
| UAT-9 | `{"tool": "generate", "arguments": {"type": "test", "include_fixtures": true}}` | No visual change | AI receives test with `beforeAll` block containing API setup calls derived from observed network traffic | [ ] |
| UAT-10 | Verify fixtures are sanitized | Human reviews generated fixture code | No real passwords, API keys, or PII in fixture data; replaced with test-safe values | [ ] |

#### Bug Report Generation

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-11 | Human triggers an error on the page after performing some actions | Error visible in DevTools | Error and actions captured | [ ] |
| UAT-12 | `{"tool": "generate", "arguments": {"type": "bug_report"}}` | No visual change | AI receives Markdown bug report with: steps to reproduce (with screenshots if available), error details, environment info, and reproduction script | [ ] |
| UAT-13 | Verify bug report Markdown is valid | Human reviews Markdown | Proper headers, image syntax (`![](data:image/png;base64,...)`), code blocks | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Screenshots not on disk | Check `~/.kaboom/` and server working directory | No PNG files saved to disk by Kaboom | [ ] |
| DL-UAT-2 | Fixtures sanitize credentials | Generate test with fixtures from a session that included login | Generated fixture replaces real password with placeholder (e.g., "test-password") | [ ] |
| DL-UAT-3 | Fixtures strip auth headers | Generate test with fixtures from API calls that had Authorization headers | Generated fixture API calls do NOT include Authorization headers | [ ] |
| DL-UAT-4 | Screenshot not accessible via HTTP | `curl http://localhost:7890/screenshots` or any path | 404 Not Found — no HTTP endpoint for screenshots | [ ] |

### Regression Checks
- [ ] Existing `generate({what: "reproduction"})` without new parameters still works identically
- [ ] Existing `generate({what: "test"})` without new parameters still works identically
- [ ] Network body capture pipeline unchanged
- [ ] Action capture pipeline unchanged
- [ ] Extension performance not degraded when auto-screenshot is disabled (default)
- [ ] No new MCP tools created (screenshot features added as modes to existing tools)

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
