---
status: proposed
scope: feature/kaboom-ci/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-kaboom-ci
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Kaboom CI Infrastructure

> QA plan for the Kaboom CI Infrastructure feature. Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Kaboom CI runs inside CI/CD pipelines where environment variables contain repository secrets, deployment keys, API tokens, and infrastructure credentials. The capture script, server endpoints, and Playwright fixtures must ensure no sensitive data escapes through snapshots, test reports, or CI artifacts.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | CI environment variables in console captures | Verify that console.log statements referencing `process.env.GITHUB_TOKEN`, `CI_DEPLOY_KEY`, `AWS_SECRET_ACCESS_KEY` etc. captured by `kaboom-ci.js` do not appear unredacted in `/snapshot` responses | critical |
| DL-2 | Sensitive headers not stripped by CI capture script | Verify that `kaboom-ci.js` strips the exact same header list as `inject.js`: Authorization, Cookie, Set-Cookie, X-Auth-Token, X-API-Key, X-CSRF-Token, Proxy-Authorization | critical |
| DL-3 | Request bodies with API keys in fetch captures | Verify that POST/PUT request bodies containing API keys (e.g., `{"api_key": "sk-..."}`) captured by the fetch interceptor are subject to redaction patterns | critical |
| DL-4 | Response bodies with tokens in `/snapshot` | Verify that network response bodies containing Bearer tokens, JWT tokens, or session tokens are redacted in the snapshot response | critical |
| DL-5 | Playwright test report artifacts expose secrets | Verify that snapshot JSON attached to Playwright reports via `kaboomAttachOnFailure` has sensitive data redacted before attachment (not just at capture time) | critical |
| DL-6 | `/snapshot` accessible beyond localhost in CI | Verify that the server binds to 127.0.0.1 only and the `/snapshot`, `/clear`, `/test-boundary` endpoints are not accessible from other containers or services on the CI runner | critical |
| DL-7 | CORS allows exfiltration in CI environment | Verify that in CI mode (when `CI` env var is set), CORS is restricted to localhost origins only, preventing other processes on the CI runner from reading captured data | high |
| DL-8 | Capture script self-capture creates infinite loop with data exposure | Verify that `kaboom-ci.js` skips capturing its own requests to the Kaboom server (requests to `__KABOOM_HOST:__KABOOM_PORT`) to prevent infinite capture loops that could expose internal transport data | high |
| DL-9 | Test boundary markers expose test names with secrets | Verify that `test_id` values sent to `/test-boundary` do not contain secrets (edge case: test name includes a fixture value like `"should authenticate with token sk-abc123"`) | medium |
| DL-10 | Human-readable failure summary exposes raw bodies | Verify that the text-format failure summary attached to Playwright reports does not include full network response bodies (should summarize, not dump) | high |
| DL-11 | Debug logging in CI script leaks capture data | Verify that when `__KABOOM_DEBUG` is enabled, `console.warn` output from the capture script does not include sensitive payload content, only metadata (endpoint, batch size, error message) | medium |
| DL-12 | GitHub Actions artifacts publicly downloadable on public repos | Verify documentation warns that snapshot attachments in Playwright reports stored as GitHub Actions artifacts are publicly downloadable for 90 days on public repos, and recommends reviewing redaction settings | medium |

### Negative Tests (must NOT leak)
- [ ] `GITHUB_TOKEN` value must not appear in any `/snapshot` response, even if logged to console by the application under test
- [ ] Authorization header values must show `[REDACTED]` in captured network bodies
- [ ] Cookie header values must show `[REDACTED]` in captured network bodies
- [ ] JWT tokens in response bodies must be redacted if redaction patterns are configured
- [ ] Requests to `127.0.0.1:7890` (Kaboom server) must not appear in captured network bodies (self-capture prevention)
- [ ] Playwright report attachments must not contain unredacted auth tokens
- [ ] `console.warn` debug output from capture script must not include request/response body content
- [ ] `/snapshot` must not be accessible from `http://172.17.0.1:7890` (Docker bridge network) in CI

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the tool responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | Snapshot stats clearly summarize test health | Verify that `stats.error_count`, `stats.warning_count`, `stats.network_failures` in the snapshot response give the AI a clear picture of test health without requiring it to parse individual log entries | [ ] |
| CL-2 | Test boundary correlation | Verify that when a `test_id` is set, the AI can clearly associate snapshot data with the specific test case (not data from a different test or inter-test noise) | [ ] |
| CL-3 | Clear distinction between extension and CI data | Verify that the AI cannot and does not need to distinguish whether data came from the extension or the CI capture script (server treats both identically, as specified) | [ ] |
| CL-4 | `/clear` response confirms scope | Verify that `entries_removed: 47` clearly tells the AI how much data was cleared, and the AI understands all buffer types were reset atomically | [ ] |
| CL-5 | Snapshot `since` filter semantics | Verify that the AI understands `since` returns entries AFTER the given timestamp (exclusive), not AT or before | [ ] |
| CL-6 | Network failure classification | Verify that `network_failures` count in stats clearly represents HTTP 4xx/5xx responses, not network connectivity errors or DNS failures | [ ] |
| CL-7 | Log level semantics | Verify that log entries with `level: "error"` and `source: "exception"` are clearly unhandled exceptions, distinguishable from `console.error()` calls | [ ] |
| CL-8 | Empty snapshot meaning | Verify that an empty snapshot (no logs, no network bodies) clearly means "no telemetry captured" not "everything is fine" -- the AI should suggest checking the capture script injection | [ ] |
| CL-9 | Fixture teardown vs test failure | Verify that the AI can distinguish between a test that failed (assertion error) and a fixture teardown that failed (Kaboom server unreachable) | [ ] |
| CL-10 | Batch flush timing | Verify that the AI understands there may be a ~100ms delay between an event occurring and it appearing in the snapshot (batch flush interval) | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI might interpret `stats.error_count: 0` as "no issues" when there are network failures (`network_failures: 5`) -- verify stats present all failure types
- [ ] AI might assume snapshot contains ALL data when it was filtered by `since` parameter -- verify response includes filter metadata
- [ ] AI might interpret `ws_connections: 0` as "WebSocket broken" when the app simply does not use WebSockets -- verify context-dependent interpretation
- [ ] AI might retry `/clear` thinking it failed when `entries_removed: 0` (empty buffers) -- verify this is a success case
- [ ] AI might misinterpret the test boundary `action: "end"` as "test passed" when it just marks the temporal boundary -- verify boundary is neutral to test outcome

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (for minimal setup) / Medium (for full CI pipeline)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Minimal Playwright setup | 2 steps: (1) `npm install @anthropic/kaboom-playwright`, (2) import fixture in test file | No -- already at minimum viable |
| GitHub Actions setup | 5 steps: (1) add npm install, (2) install Playwright, (3) start Kaboom server, (4) run tests, (5) upload artifacts | Yes -- could provide a GitHub Action that handles steps 3 and 5 |
| Manual capture with Puppeteer | 3 steps: (1) read `kaboom-ci.js` file, (2) inject via `evaluateOnNewDocument`, (3) manually call `/snapshot` after test | No -- Puppeteer lacks fixture abstraction; this is inherently manual |
| Get snapshot after test failure | 1 step (automatic): fixture captures snapshot on failure | No -- already zero-effort for the developer |
| Clear buffers between tests | 0 steps (automatic): fixture calls `/clear` between tests | No -- already zero-effort |
| Generate HAR from CI capture | 2 steps: (1) run tests with capture, (2) call `generate({format: "har"})` | Yes -- could auto-generate HAR as part of fixture teardown |

### Default Behavior Verification
- [ ] Playwright fixture works with zero configuration beyond the import statement
- [ ] Default port (7890) is used when `kaboomPort` is not specified
- [ ] `kaboomAttachOnFailure` defaults to `true` -- snapshots attached automatically on test failure
- [ ] `kaboomAutoStart` defaults to `true` (reserved for future auto-start)
- [ ] Capture script auto-detects server at `127.0.0.1:7890` without explicit configuration
- [ ] Double-initialization guard (`__KABOOM_CI_INITIALIZED`) prevents duplicate capture on page navigation
- [ ] Capture script silently degrades if server is unreachable -- tests are not blocked

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | Capture script initializes and sets guard | `kaboom-ci.js` injected into page | `window.__KABOOM_CI_INITIALIZED === true` | must |
| UT-2 | Double initialization prevented | `kaboom-ci.js` injected twice | Second injection is no-op, no duplicate event listeners | must |
| UT-3 | Console.log captured | `console.log("test message")` | Log entry with `level: "log"`, `message: "test message"` in batch | must |
| UT-4 | Console.error captured | `console.error("failure")` | Log entry with `level: "error"`, `message: "failure"` | must |
| UT-5 | Unhandled exception captured | `window.onerror` fires with TypeError | Log entry with `source: "exception"`, stack trace included | must |
| UT-6 | Unhandled promise rejection captured | `Promise.reject(new Error("async fail"))` | Log entry with rejection reason serialized | must |
| UT-7 | Fetch 400+ response captured | `fetch("/api/fail")` returns 500 | Network body entry with `status: 500`, response body included | must |
| UT-8 | Fetch 200 response NOT captured | `fetch("/api/success")` returns 200 | No network body entry for this request | must |
| UT-9 | Sensitive headers stripped | Fetch with `Authorization: Bearer xyz` | Header value replaced with `[REDACTED]` in capture | must |
| UT-10 | Self-capture prevention | Fetch to Kaboom server `127.0.0.1:7890/logs` | Request NOT captured in network bodies | must |
| UT-11 | WebSocket open event captured | `new WebSocket("ws://localhost:3000")` | WS event with `type: "open"` | must |
| UT-12 | WebSocket message captured | WebSocket receives message | WS event with message content | must |
| UT-13 | WebSocket close event captured | WebSocket connection closed | WS event with `type: "close"`, close code | must |
| UT-14 | Batch flush at 100ms interval | 10 log entries within 50ms | All 10 entries sent in a single batch after flush interval | must |
| UT-15 | Batch size limit (50 per flush) | 60 entries accumulated before flush | First 50 sent, remaining 10 in next flush | must |
| UT-16 | Buffer limit (1000 per type) | 1100 log entries accumulated | Buffer capped at 1000, oldest entries dropped | must |
| UT-17 | Response body truncation | 10KB response body | Truncated at 5120 characters (`MAX_RESPONSE_LENGTH`) | must |
| UT-18 | `/snapshot` returns all captured state | Various telemetry captured | Response includes logs, websocket_events, network_bodies, enhanced_actions, stats | must |
| UT-19 | `/snapshot` with `since` filter | 10 entries, 5 before timestamp, 5 after | Only 5 entries after timestamp returned | must |
| UT-20 | `/snapshot` with `test_id` label | `test_id=login-flow` | Snapshot labeled with `test_id: "login-flow"` | must |
| UT-21 | `/clear` resets all buffers | Buffers have 100 entries | All buffers empty, response `entries_removed: 100` | must |
| UT-22 | `/test-boundary` start action | `{test_id: "my-test", action: "start"}` | Current test ID set, timestamp recorded | must |
| UT-23 | `/test-boundary` end action | `{test_id: "my-test", action: "end"}` | Current test ID cleared, timestamp recorded | must |
| UT-24 | `computeSnapshotStats()` accuracy | 5 logs (2 errors, 1 warning), 3 network bodies (1 failure) | `{total_logs: 5, error_count: 2, warning_count: 1, network_failures: 1, ws_connections: 0}` | must |
| UT-25 | Capture script configurable globals | `window.__KABOOM_HOST = "10.0.0.1"`, `__KABOOM_PORT = 9999` | Capture script sends to `10.0.0.1:9999` | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | End-to-end: inject -> capture -> snapshot | `kaboom-ci.js` + Kaboom server + `/snapshot` | Console error in page appears in snapshot response | must |
| IT-2 | Playwright fixture lifecycle | Fixture setup + test execution + teardown | Capture script injected, test boundary set, snapshot attached on failure, buffers cleared | must |
| IT-3 | Test isolation via /clear | Two tests in sequence with different errors | Second test's snapshot contains only its own errors, not first test's | must |
| IT-4 | Page navigation re-initialization | Test navigates to new page | Capture script re-initializes, guard prevents duplicates, telemetry continues | must |
| IT-5 | Server starts before tests, survives entire suite | `npx kaboom-mcp` + 20 sequential Playwright tests | Server stays alive, no memory leak, all tests get snapshots | must |
| IT-6 | Snapshot attachment on test failure | Playwright test assertion fails | Two attachments: JSON snapshot and human-readable text summary | must |
| IT-7 | No attachment on test pass | Playwright test passes | No Kaboom attachments added to report | must |
| IT-8 | Multiple browser contexts | Test opens 2 pages in same context | Both pages' telemetry appears in single snapshot (interleaved chronologically) | should |
| IT-9 | Test timeout handling | Test times out (Playwright timeout) | Fixture teardown still fires, snapshot captured (possibly partial) | should |
| IT-10 | Server unreachable graceful degradation | Kaboom server not started, run tests | Tests run normally, no Kaboom attachments, no test failures due to Kaboom | must |
| IT-11 | MCP tools work with CI-captured data | Capture via `kaboom-ci.js`, then `observe({what: "errors"})` via MCP | MCP response includes errors captured by CI script (server treats identically) | must |
| IT-12 | HAR generation from CI capture | Capture via CI script, call `generate({format: "har"})` | Valid HAR file produced from CI-captured network data | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | Capture script initialization time | Time from injection to `__KABOOM_CI_INITIALIZED = true` | < 5ms | must |
| PT-2 | Console log capture overhead | Time per `console.log()` call with capture active vs without | < 0.1ms overhead | must |
| PT-3 | Fetch intercept overhead | Time per `fetch()` call with capture active vs without | < 0.5ms overhead | must |
| PT-4 | Batch flush latency | Time for HTTP POST of 50 entries | < 10ms | must |
| PT-5 | `/snapshot` response time (1000 entries) | Server response time for full snapshot | < 200ms | must |
| PT-6 | `/clear` response time | Server response time for buffer reset | < 10ms | must |
| PT-7 | Capture script memory footprint | Browser memory with capture active after 100 tests | < 5MB | must |
| PT-8 | Server memory in CI session | Server memory after 500 tests with captures | < 50MB (ring buffers cap growth) | must |
| PT-9 | sendBeacon reliability | Percentage of batches successfully delivered | > 99% when server is reachable | should |
| PT-10 | Large snapshot serialization | JSON serialization time for 10,000 log entries | < 500ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Server not running when tests start | `kaboom-ci.js` sends to unreachable server | Errors silently swallowed, no test impact, no console errors from capture script | must |
| EC-2 | Server crashes mid-test | Server killed during test execution | In-flight batches lost, fixture teardown returns empty snapshot, test unaffected | must |
| EC-3 | Very long test suite (1000+ tests) | 1000 tests with `/clear` between each | Memory stays bounded, no growth over time | must |
| EC-4 | Page navigates during test | Test does `page.goto()` twice | Capture script re-initialized via `addInitScript`, guard prevents duplicates | must |
| EC-5 | Browser context with multiple pages | 3 pages open simultaneously posting telemetry | All telemetry aggregated in server, no data loss or race conditions | should |
| EC-6 | Test timeout fires | Playwright test exceeds timeout | Fixture teardown attempts snapshot capture; may get partial data | should |
| EC-7 | Concurrent test workers sharing server | 4 parallel workers posting to same server | Data interleaved (known limitation); `/test-boundary` markers may conflict | should |
| EC-8 | Large response bodies (>5KB) | API response of 100KB | Body truncated at 5120 characters in capture | must |
| EC-9 | Binary response body | Image or PDF response | Binary content not captured (bypassed), no crash | must |
| EC-10 | Port conflict in CI | Another process using port 7890 | Server fails to start with clear error; fixture's `kaboomPort` option allows alternative | should |
| EC-11 | `beforeunload` flush | Page navigates away during batch accumulation | `navigator.sendBeacon` fires to flush remaining batches | should |
| EC-12 | Debug mode logging | `window.__KABOOM_DEBUG = true` | `console.warn` messages show batch sizes and send failures without leaking payload content | should |
| EC-13 | Capture version mismatch | CI script sends `captureVersion: "2.0.0"`, server expects "1.0.0" | Server accepts data (current behavior) or rejects with clear error (if version negotiation implemented) | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed (for comparison testing) OR headless browser with Playwright
- [ ] Node.js installed with `@anthropic/kaboom-playwright` available (or simulated)
- [ ] A sample Playwright test project with at least one passing and one intentionally failing test

### Step-by-Step Verification

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Start Kaboom server: `./dist/kaboom --port 7890` | Server starts, logs show "listening on :7890" | Server running and ready | [ ] |
| UAT-2 | AI checks server health: `GET http://127.0.0.1:7890/health` | N/A | HTTP 200 response | [ ] |
| UAT-3 | Human runs a Playwright test with Kaboom fixture that navigates to a page with a console error | Test output shows failure | Test runs, console error captured | [ ] |
| UAT-4 | AI gets snapshot: `GET http://127.0.0.1:7890/snapshot` | N/A | Response contains the console error in `logs` array, `stats.error_count >= 1` | [ ] |
| UAT-5 | AI verifies sensitive headers stripped: `observe({what: "network_bodies"})` | N/A | Any Authorization/Cookie headers show `[REDACTED]` | [ ] |
| UAT-6 | AI clears buffers: `POST http://127.0.0.1:7890/clear` | N/A | Response: `{"cleared": true, "entries_removed": N}` where N > 0 | [ ] |
| UAT-7 | AI verifies clear worked: `GET http://127.0.0.1:7890/snapshot` | N/A | Response has empty logs, network_bodies, stats show all zeros | [ ] |
| UAT-8 | Human runs a passing Playwright test with Kaboom fixture | Test passes | No Kaboom snapshot attached to report (attachment is failure-only) | [ ] |
| UAT-9 | Human runs a failing Playwright test with Kaboom fixture | Test fails | Kaboom snapshot JSON and text summary attached to Playwright report | [ ] |
| UAT-10 | AI marks test boundary: `POST http://127.0.0.1:7890/test-boundary {"test_id": "login-test", "action": "start"}` | N/A | Response confirms test boundary set with timestamp | [ ] |
| UAT-11 | AI gets snapshot with test_id: `GET http://127.0.0.1:7890/snapshot?test_id=login-test` | N/A | Snapshot labeled with `test_id: "login-test"` | [ ] |
| UAT-12 | AI uses existing MCP tool with CI-captured data: `{"tool":"observe","arguments":{"what":"errors"}}` | N/A | Errors captured by CI script appear in MCP response identically to extension-captured errors | [ ] |
| UAT-13 | AI generates HAR from CI data: `{"tool":"generate","arguments":{"format":"har"}}` | N/A | Valid HAR file produced from CI-captured network data | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | CI env vars not in snapshot | Set `GITHUB_TOKEN=ghp_secret123`, app logs it via `console.log`, get snapshot | Console log entry exists but value is the raw log (Kaboom does not redact app console output -- this is documented behavior; redaction is the app's responsibility) | [ ] |
| DL-UAT-2 | Auth headers stripped in CI capture | Make fetch with `Authorization: Bearer token123`, get snapshot | Header value shows `[REDACTED]` | [ ] |
| DL-UAT-3 | Self-capture prevention | Verify no entries for `127.0.0.1:7890/*` in network bodies | No Kaboom server requests captured | [ ] |
| DL-UAT-4 | Playwright attachment redacted | Inspect test report attachment for a failed test | Snapshot JSON has headers redacted, response bodies subject to configured redaction rules | [ ] |
| DL-UAT-5 | Server bound to localhost only | Try `curl http://<external-ip>:7890/snapshot` | Connection refused | [ ] |
| DL-UAT-6 | CORS restricted in CI mode | Set `CI=true`, send request with `Origin: http://attacker.com` | CORS response does not include `Access-Control-Allow-Origin: http://attacker.com` | [ ] |

### Regression Checks
- [ ] Extension-based capture still works when CI endpoints are registered
- [ ] Existing `/logs`, `/websocket-events`, `/network-bodies` POST endpoints accept data identically
- [ ] Existing MCP tools (observe, generate, configure, interact) function identically
- [ ] Server starts and operates the same with or without the `CI` environment variable
- [ ] Ring buffer sizes and memory limits unchanged
- [ ] No performance degradation for extension capture when CI endpoints are active

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
