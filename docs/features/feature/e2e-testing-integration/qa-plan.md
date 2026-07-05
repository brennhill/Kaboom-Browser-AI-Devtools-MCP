---
status: proposed
scope: feature/e2e-testing-integration/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-e2e-testing-integration
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: E2E Testing Integration

> QA plan for the E2E Testing Integration feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. The E2E Testing Integration exports captured Kaboom state as Playwright test fixtures, CI configuration YAML, and failure snapshots. Generated artifacts are intended to be committed to version control, posted in PRs, and uploaded as CI artifacts -- making data leak prevention especially critical since these files persist beyond the ephemeral Kaboom session.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | Fixture data contains API tokens in response bodies | Verify that `generate({format: "playwright_fixture", artifact: "fixture_data"})` applies redaction to captured response bodies, replacing tokens (Bearer, JWT, session tokens) with `[REDACTED]` | critical |
| DL-2 | Fixture data exposes auth header values | Verify that when `include_headers: true` is set, the exported fixture data strips Authorization, Cookie, Set-Cookie, and token headers (same list as `inject.js`) | critical |
| DL-3 | Test harness hardcodes credentials | Verify that `test_harness` output does not include hardcoded passwords, API keys, or tokens in the generated test script. Input field values should use placeholders like `[user-provided]` | critical |
| DL-4 | CI config YAML embeds secrets | Verify that `ci_config` output does not include any repository tokens, deployment keys, environment-specific secrets, or credentials. Only standard CI constructs (actions/checkout, npx commands) are used | critical |
| DL-5 | Failure snapshot exposes raw network bodies with secrets | Verify that `failure_snapshot` applies the same redaction rules as `observe({what: "network_bodies"})` before returning data | critical |
| DL-6 | URL query parameters with secrets in fixture data | Verify that captured network URLs in fixture data have sensitive query parameters stripped (e.g., `?api_key=`, `?token=`, `?secret=`) | high |
| DL-7 | Fixture JSON exceeds size cap and includes truncated secrets | Verify that the 500KB output cap (R16) does not result in partial redaction where a token is split across a truncation boundary | high |
| DL-8 | Fixture-loader.js helper does not add additional data | Verify that the generated `fixture-loader.js` is a pure routing helper and does not include, log, or transmit fixture data beyond the `page.route()` fulfillment | medium |
| DL-9 | Generated test file references real user data | Verify that test harness field values (email addresses, names, phone numbers) use placeholder/test data, not real user data captured from the developer's browser session | high |
| DL-10 | GitLab CI YAML output differs in security from GitHub Actions | Verify that `ci_config` with `ci_provider: "gitlab_ci"` also contains no secrets, no internal URLs, and follows the same security constraints as GitHub Actions output | medium |

### Negative Tests (must NOT leak)
- [ ] Bearer tokens in API response bodies must show `[REDACTED]` in fixture_data output
- [ ] Authorization header values must not appear even when `include_headers: true`
- [ ] Generated test scripts must use `[user-provided]` for password fields, not captured values
- [ ] CI config YAML must not contain `GITHUB_TOKEN`, `DEPLOY_KEY`, or any `${{ secrets.* }}` references
- [ ] Failure snapshot must not contain unredacted auth tokens in network body content
- [ ] URL query parameters `?token=`, `?api_key=`, `?secret=` must be stripped from fixture URLs
- [ ] Real email addresses captured from form inputs must be replaced with test data in generated tests

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Artifact type selection | Verify that the AI clearly understands the four `artifact` options (`fixture_data`, `test_harness`, `ci_config`, `failure_snapshot`) and knows which to use for each task | [ ] |
| CL-2 | Fixture data is JSON, not executable | Verify that `fixture_data` output is clearly a JSON data file (for use with `page.route()`), not an executable test -- the AI should save it as `.json`, not `.spec.js` | [ ] |
| CL-3 | Test harness vs existing test generation | Verify the AI understands that `playwright_fixture` + `test_harness` is for CI-integrated tests (with Kaboom CI fixture), while `generate({format: "test"})` is for standalone scripts | [ ] |
| CL-4 | CI config is a fragment, not a complete file | Verify that the AI understands `ci_config` produces a workflow YAML that may need customization (e.g., adding environment variables, adjusting Node version) | [ ] |
| CL-5 | Failure snapshot vs live observation | Verify the AI understands that `failure_snapshot` is a point-in-time export of server state, not a live stream -- it captures what is currently in buffers | [ ] |
| CL-6 | filter_url is a substring match | Verify the AI understands `filter_url: "/api/"` matches any URL containing "/api/" as a substring, not an exact path match or regex | [ ] |
| CL-7 | Generated code needs file saving | Verify the AI understands that all `generate` output is returned as MCP response content and must be explicitly saved to files -- Kaboom does not write to the project directory | [ ] |
| CL-8 | Multiple content blocks in response | Verify the AI correctly handles responses with multiple `content` blocks (e.g., `fixture_data` returns both the JSON file and the `fixture-loader.js` helper as separate blocks) | [ ] |
| CL-9 | Redacted values in fixtures | Verify the AI understands that `[REDACTED]` values in fixture data need to be replaced with test-appropriate values before the test can run | [ ] |
| CL-10 | base_url replacement scope | Verify the AI understands that `base_url` replaces the origin in generated URLs but does not affect the fixture data's captured URLs | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might try to execute fixture_data as a test file -- verify it is clearly labeled as a JSON data file
- [ ] AI might assume ci_config is complete and ready to commit without review -- verify it includes comments suggesting customization
- [ ] AI might use failure_snapshot as fixture_data -- verify these serve different purposes (debugging vs testing)
- [ ] AI might not realize that generated test requires `@anthropic/kaboom-playwright` to be installed -- verify the import is clear
- [ ] AI might set `include_kaboom_fixture: false` without understanding the test will lack runtime capture capability -- verify the trade-off is documented in output
- [ ] AI might interpret empty fixture_data `{}` as an error -- verify it is clearly marked as "no JSON API responses found"

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Export API fixtures from captured traffic | 1 step: `generate({format: "playwright_fixture", artifact: "fixture_data"})` | No -- already minimal |
| Generate CI-ready test file | 1 step: `generate({format: "playwright_fixture", artifact: "test_harness"})` | No -- already minimal |
| Generate GitHub Actions config | 1 step: `generate({format: "playwright_fixture", artifact: "ci_config"})` | No -- already minimal |
| Export failure snapshot | 1 step: `generate({format: "playwright_fixture", artifact: "failure_snapshot"})` | No -- already minimal |
| Full CI setup from captured session | 3 steps: (1) generate fixture_data, (2) generate test_harness, (3) generate ci_config, then save all files | Yes -- could provide a "generate all" option that returns all artifacts at once |
| Filter fixtures to specific API paths | 1 step: add `filter_url: "/api/"` to options | No -- already a single parameter |

### Default Behavior Verification
- [ ] `fixture_data` works without any options (exports all JSON API responses)
- [ ] `test_harness` defaults to including `@anthropic/kaboom-playwright` fixture import
- [ ] `ci_config` defaults to `github_actions` provider
- [ ] `include_headers` defaults to `false` (safer default -- no headers exported)
- [ ] `filter_url` defaults to unset (all network bodies included)
- [ ] `include_kaboom_fixture` defaults to `true` (CI-integrated by default)
- [ ] Non-JSON responses automatically excluded without explicit filtering
- [ ] Duplicate API endpoints deduplicated (most recent response kept) without configuration

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | fixture_data from captured JSON responses | 3 JSON API responses captured | JSON object with 3 entries keyed by URL, each with method, status, contentType, body | must |
| UT-2 | fixture_data applies redaction | Response body contains `{"token": "Bearer xyz"}` | Token value replaced with `[REDACTED]` in fixture output | must |
| UT-3 | fixture_data with include_headers: true | Responses with Content-Type and X-Request-Id headers | Headers included in fixture (minus sensitive headers which are stripped) | must |
| UT-4 | fixture_data with filter_url | 5 responses, 3 matching `/api/`, 2 not | Only 3 matching responses in output | must |
| UT-5 | fixture_data deduplicates by URL+method | 3 captures for `GET /api/users` at different times | Only the most recent response included | should |
| UT-6 | fixture_data skips non-JSON responses | HTML response, CSS response, JSON response | Only JSON response appears in output | must |
| UT-7 | fixture_data skips malformed JSON | Response body is `{invalid json` | Entry skipped, no crash | must |
| UT-8 | fixture_data 500KB cap | 200 large JSON responses totaling 2MB | Output truncated at 500KB with `[truncated]` marker | should |
| UT-9 | fixture_data includes fixture-loader.js | Any fixture_data request | Response contains 2 content blocks: JSON data + loader helper | must |
| UT-10 | fixture_data empty (no captures) | No network bodies captured | Empty JSON `{}` with comment "no JSON API responses found" | must |
| UT-11 | test_harness generates valid Playwright test | User actions captured (click, type, navigate) | Complete `.spec.js` file with imports, test function, action replay | must |
| UT-12 | test_harness imports @anthropic/kaboom-playwright | Default options | First line: `import { test, expect } from '@anthropic/kaboom-playwright'` | must |
| UT-13 | test_harness imports @playwright/test when kaboom fixture disabled | `include_kaboom_fixture: false` | First line: `import { test, expect } from '@playwright/test'` | should |
| UT-14 | test_harness uses correct locator priority | Actions with testId, role, ariaLabel, text, id, cssPath | Locators follow priority: testId > role > ariaLabel > text > id > cssPath | must |
| UT-15 | test_harness includes fixture loading | fixture_data available | Test includes `loadFixtures(page, fixtures)` call | must |
| UT-16 | test_harness with no actions | No user actions captured | Minimal test skeleton with comment "no user actions available" | must |
| UT-17 | test_harness with base_url replacement | `base_url: "http://localhost:3000"`, captured URLs from `http://localhost:8080` | All URLs replaced with `http://localhost:3000` | should |
| UT-18 | test_harness with test_name | `test_name: "user login flow"` | Test function named `test('user login flow', ...)` | must |
| UT-19 | ci_config generates valid GitHub Actions YAML | Default options | Valid YAML with checkout, setup-node, npm ci, Playwright install, Kaboom start, test, upload steps | must |
| UT-20 | ci_config generates valid GitLab CI YAML | `ci_provider: "gitlab_ci"` | Valid GitLab CI YAML with equivalent stages | should |
| UT-21 | ci_config contains no secrets | Any options | Output has no `${{ secrets.* }}`, no hardcoded tokens, no credentials | must |
| UT-22 | failure_snapshot matches SnapshotResponse structure | Server has captured data | JSON output matches `SnapshotResponse` struct with logs, websocket_events, network_bodies, enhanced_actions, stats | must |
| UT-23 | failure_snapshot reuses computeSnapshotStats() | 10 log entries, 3 errors, 2 network failures | Stats computed identically to `/snapshot` endpoint | must |
| UT-24 | failure_snapshot with since filter | 20 entries, 10 before timestamp | Only 10 entries after timestamp in output | should |
| UT-25 | Response body truncation at 10KB | Single response body of 50KB | Body truncated to 10KB with `[response truncated]` comment | should |
| UT-26 | base_url trailing slash normalization | `base_url: "http://localhost:3000/"` | Trailing slash stripped, URLs generated without double slashes | should |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | fixture_data uses generateFixtures() infrastructure | `reproduction.go` + `tools_core.go` dispatch | Fixture data extends existing generateFixtures() with method/status/contentType metadata | must |
| IT-2 | test_harness uses generateEnhancedPlaywrightScript() | `reproduction.go` + `codegen.go` + tools_core.go dispatch | Test harness wraps existing script generation with Kaboom CI fixture imports | must |
| IT-3 | failure_snapshot uses computeSnapshotStats() from ci.go | `ci.go` + tools_core.go dispatch | Snapshot stats match what `/snapshot` endpoint would return | must |
| IT-4 | filter_url consistent with NetworkBodyFilter | filter_url pattern + existing network body filtering | Same URL matching behavior as other network filtering across Kaboom | must |
| IT-5 | since filter consistent with filterLogsSince() from ci.go | since parameter + existing log filtering | Same timestamp filtering behavior as `/snapshot?since=` endpoint | should |
| IT-6 | base_url uses replaceOrigin() from codegen.go | base_url option + existing origin replacement | Same origin replacement behavior as existing test generation | should |
| IT-7 | Tool dispatch in tools_core.go | `format: "playwright_fixture"` in generate tool | Correctly dispatched to `toolGeneratePlaywrightFixture()` handler | must |
| IT-8 | All 4 artifact types from same session | Capture data, then generate all 4 artifacts | All artifacts contain consistent data from the same session | must |
| IT-9 | Redaction consistent with observe({what: "network_bodies"}) | Same captured data viewed via observe and via fixture_data | Same redaction applied to both | must |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | fixture_data generation (100 bodies) | Wall clock time | < 200ms | must |
| PT-2 | test_harness generation | Wall clock time | < 200ms | must |
| PT-3 | ci_config generation | Wall clock time | < 50ms | must |
| PT-4 | failure_snapshot generation | Wall clock time for full snapshot | < 200ms | must |
| PT-5 | Output size within cap | Maximum output size per artifact | < 500KB | must |
| PT-6 | fixture_data with 500 network bodies | Wall clock time for large dataset | < 500ms | should |
| PT-7 | Concurrent generation requests | Two artifact generations in parallel | Both complete without deadlock, no data corruption | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | No network bodies captured | Empty server buffers | fixture_data returns `{}`, test_harness omits route handlers | must |
| EC-2 | No actions captured | No user interactions recorded | test_harness returns minimal skeleton with comment | must |
| EC-3 | Only non-JSON responses | HTML pages, CSS files, images | fixture_data returns `{}` with comment noting no JSON responses found | must |
| EC-4 | Very large response bodies | 50KB JSON response bodies | Bodies truncated at 10KB per entry, total output capped at 500KB | should |
| EC-5 | Duplicate API endpoints | 5 captures for GET /api/users | Most recent response used (deduplication) | should |
| EC-6 | Malformed JSON response body | Body is `{invalid` | Entry skipped, other valid entries included | must |
| EC-7 | Extension disconnected | No extension connection | failure_snapshot returns whatever is in server buffers (may be empty) | must |
| EC-8 | Concurrent reads during generation | Extension pushing data while generate is called | Generation reads from ring buffers under RLock, no blocking | should |
| EC-9 | base_url with trailing slash | `base_url: "http://localhost:3000/"` | Trailing slash normalized, no double slashes in URLs | should |
| EC-10 | Empty test_name option | `test_name: ""` | Default test name used (e.g., "generated test") | should |
| EC-11 | Unknown ci_provider | `ci_provider: "jenkins"` | Error: unsupported CI provider, suggest github_actions or gitlab_ci | should |
| EC-12 | Unknown artifact type | `artifact: "unknown"` | Error: unsupported artifact type, list valid options | must |
| EC-13 | Since filter with future timestamp | `since: "2099-01-01T00:00:00Z"` | No entries match, empty output returned | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A web application loaded with the developer performing a user flow (login, navigate, interact)
- [ ] At least 3 API requests captured in network bodies
- [ ] At least 5 user actions captured (clicks, typing, navigation)

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | AI generates fixture data: `{"tool":"generate","arguments":{"type":"playwright_fixture","artifact":"fixture_data","options":{"filter_url":"/api/"}}}` | N/A | JSON response with API endpoint fixtures including method, status, contentType, and body | [ ] |
| UAT-2 | Human verifies fixture JSON structure | Inspect returned JSON | Each entry has method (GET/POST), status (200/404), contentType, and parsed body | [ ] |
| UAT-3 | Human verifies no secrets in fixture data | Search output for tokens, passwords, API keys | All sensitive values show `[REDACTED]` | [ ] |
| UAT-4 | AI generates fixture data with headers: `{"tool":"generate","arguments":{"type":"playwright_fixture","artifact":"fixture_data","options":{"include_headers":true}}}` | N/A | Fixtures include response headers, but Authorization/Cookie headers stripped | [ ] |
| UAT-5 | AI generates test harness: `{"tool":"generate","arguments":{"type":"playwright_fixture","artifact":"test_harness","options":{"test_name":"user login flow","base_url":"http://localhost:3000"}}}` | N/A | Complete Playwright test file with imports, fixture loading, and action replay | [ ] |
| UAT-6 | Human verifies test harness is valid JS | Read generated code | Valid JavaScript/TypeScript syntax, correct Playwright API usage | [ ] |
| UAT-7 | Human verifies test harness imports Kaboom fixture | Check import statement | `import { test, expect } from '@anthropic/kaboom-playwright'` | [ ] |
| UAT-8 | Human verifies password fields use placeholders | Check form fill actions | Password fields show `[user-provided]`, not actual captured values | [ ] |
| UAT-9 | AI generates CI config: `{"tool":"generate","arguments":{"type":"playwright_fixture","artifact":"ci_config"}}` | N/A | Valid GitHub Actions YAML workflow | [ ] |
| UAT-10 | Human verifies CI config has no secrets | Read YAML | No `${{ secrets.* }}`, no hardcoded tokens, no credentials | [ ] |
| UAT-11 | Human verifies CI config is valid YAML | Parse YAML | Valid YAML structure with correct GitHub Actions syntax | [ ] |
| UAT-12 | AI generates failure snapshot: `{"tool":"generate","arguments":{"type":"playwright_fixture","artifact":"failure_snapshot"}}` | N/A | JSON snapshot with logs, network_bodies, websocket_events, actions, stats | [ ] |
| UAT-13 | Human verifies snapshot stats accuracy | Compare stats to known captured data | error_count, warning_count, network_failures match expected values | [ ] |
| UAT-14 | AI generates test harness without Kaboom fixture: `{"tool":"generate","arguments":{"type":"playwright_fixture","artifact":"test_harness","options":{"include_kaboom_fixture":false}}}` | N/A | Test imports from `@playwright/test` instead of `@anthropic/kaboom-playwright` | [ ] |
| UAT-15 | AI generates GitLab CI config: `{"tool":"generate","arguments":{"type":"playwright_fixture","artifact":"ci_config","options":{"ci_provider":"gitlab_ci"}}}` | N/A | Valid GitLab CI YAML configuration | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | Fixture data redacts auth tokens | Make authenticated API call, generate fixture_data | Response body tokens show `[REDACTED]` | [ ] |
| DL-UAT-2 | Headers stripped even with include_headers | Set `include_headers: true`, inspect output | Authorization, Cookie headers not present in fixture | [ ] |
| DL-UAT-3 | Test harness uses placeholders for passwords | Perform login flow, generate test_harness | Password field value shows `[user-provided]` | [ ] |
| DL-UAT-4 | CI config is secret-free | Generate ci_config, search for common secret patterns | No matches for `token`, `secret`, `key`, `password`, `credential` in values | [ ] |
| DL-UAT-5 | Failure snapshot applies same redaction as observe | Compare `failure_snapshot` output to `observe({what: "network_bodies"})` output | Same redaction applied to both | [ ] |

### Regression Checks
- [ ] Existing `generate({format: "reproduction"})` works independently and produces unchanged output
- [ ] Existing `generate({format: "test"})` works independently and produces unchanged output
- [ ] Existing `generate({format: "har"})` and `generate({format: "sarif"})` work independently
- [ ] 5-tool model maintained -- no new tools added, only new mode under `generate`
- [ ] `generateFixtures()` in `reproduction.go` still works correctly for reproduction scripts
- [ ] `getPlaywrightLocator()` selector priority unchanged for all generation modes

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
