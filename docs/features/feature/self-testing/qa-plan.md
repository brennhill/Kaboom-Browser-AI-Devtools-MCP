---
status: proposed
scope: feature/self-testing/qa
ai-priority: medium
tags: [testing, qa]
relates-to: [product-spec.md, tech-spec.md]
last-verified: 2026-03-05
doc_type: qa-plan
feature_id: feature-self-testing
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Self-Testing Infrastructure

> QA plan for the Self-Testing Infrastructure feature (Features 32, 33, 34). Covers data leak analysis, LLM clarity, simplicity assessment, code-level testing, and step-by-step UAT verification. This feature exposes HTTP GET endpoints for captured data, an HTTP MCP endpoint, and a Playwright-based UAT harness.

---

## 1. Data Leak Analysis

**Goal:** Verify the feature does NOT expose data it shouldn't. Self-testing exposes HTTP GET endpoints (`/captured/*`) and an HTTP MCP endpoint (`/mcp`) that provide READ access to captured telemetry. These endpoints create a new attack surface beyond the existing POST-only ingestion endpoints.

| # | Data Leak Risk | What to Check | Severity |
|---|---------------|---------------|----------|
| DL-1 | `/captured/*` endpoints expose data over HTTP without authentication | Verify all `/captured/*` endpoints are localhost-only (127.0.0.1 binding); not accessible from external IPs | critical |
| DL-2 | `/captured/network` exposes request/response bodies with auth headers | Verify network entries returned by GET have the same header stripping as MCP `observe` responses (Authorization, Cookie, etc.) | critical |
| DL-3 | `/captured/logs` exposes console output containing secrets | Console log entries may contain `console.log("token:", secret)` — verify entries are returned as-is (same as observe) but endpoint is localhost-only | high |
| DL-4 | `/mcp` endpoint allows unauthenticated tool execution | Verify `/mcp` is localhost-only; anyone on localhost can invoke MCP tools via HTTP including `execute_js` | critical |
| DL-5 | `/mcp` exposes execute_js without AI Web Pilot toggle check | Verify that calling `execute_js` via `/mcp` still requires the AI Web Pilot toggle to be enabled in the extension | critical |
| DL-6 | `/captured/errors` exposes error stack traces with file paths | Error entries may include stack traces revealing server-side file paths; verify entries match what observe() returns | medium |
| DL-7 | `/captured/websocket` exposes WebSocket message payloads | If ws_mode is "messages", GET endpoint returns payloads; verify same redaction as observe() | high |
| DL-8 | JSONL format output bypasses redaction | When `format=jsonl` is used, verify each line has the same redaction applied as JSON format | high |
| DL-9 | Playwright harness stores credentials in test output | UAT runner output JSON should not contain captured auth tokens or credentials | medium |
| DL-10 | Test fixture page (`test-page.html`) accessible externally | If the test fixture is served by a local server, verify it is only on localhost | low |
| DL-11 | `/mcp` endpoint accepts requests from any origin (CORS) | Verify no CORS headers are set (or they restrict to localhost) — a browser on any page could POST to localhost:7890/mcp | high |
| DL-12 | `limit` parameter allows extracting all captured data | Large `limit` value (e.g., 999999) could dump entire ring buffer; verify the endpoint caps at a reasonable maximum | medium |

### Negative Tests (must NOT leak)
- [ ] `/captured/*` endpoints must NOT be accessible from external IP addresses
- [ ] `/captured/network` must NOT include raw Authorization or Cookie headers in entries
- [ ] `/mcp` must NOT bypass security checks (AI Web Pilot toggle, etc.)
- [ ] `/captured/*` must NOT return data when server binds to 127.0.0.1 and request comes from non-localhost
- [ ] Playwright harness output must NOT embed auth tokens from captured traffic
- [ ] `/mcp` must NOT allow `tools/call` with `initialize` method to leak server internals beyond what MCP spec allows

---

## 2. LLM Clarity Assessment

**Goal:** Verify an AI agent reading the HTTP endpoint responses can unambiguously understand the data without misinterpretation.

| # | Clarity Check | What to Verify | Status |
|---|--------------|----------------|--------|
| CL-1 | `/captured/*` response includes count and truncation flag | Response has `count` (actual entries returned) and `truncated` (whether more exist) so AI knows if data was clipped | [ ] |
| CL-2 | `/captured/*` response includes time range | `oldest_ts` and `newest_ts` help AI understand the data window | [ ] |
| CL-3 | `/mcp` response follows JSON-RPC 2.0 | Response includes `jsonrpc`, `id`, and `result` or `error` fields per spec | [ ] |
| CL-4 | `/mcp` error responses include MCP error codes | Parse errors return -32700; method not found returns -32601; etc. | [ ] |
| CL-5 | Method not allowed returns 405 | GET on /mcp returns 405 (not 404); POST on /captured/* returns 405 | [ ] |
| CL-6 | Empty results are explicit | `/captured/actions` with no captured actions returns `{ entries: [], count: 0, truncated: false }` — not a 404 or empty body | [ ] |
| CL-7 | Playwright harness output is structured JSON | UAT runner outputs machine-readable JSON with phases, test names, statuses, and durations | [ ] |
| CL-8 | Harness failure messages are actionable | When a test fails, the output includes the test name, expected vs actual, and duration — enough to diagnose | [ ] |

### Common LLM Misinterpretation Risks
- [ ] AI may confuse `/captured/network` (HTTP GET for raw entries) with `observe({what: "network_waterfall"})` (MCP tool with analysis) — verify different response shapes make them distinguishable
- [ ] AI may try to use `/mcp` endpoint instead of stdio MCP in normal operation — verify /mcp is documented as a testing/scripting interface
- [ ] AI may assume `/captured/*` data is the same as `observe()` output — verify it is raw ring buffer data, not the processed/analyzed output
- [ ] AI may not realize `since` parameter is in Unix milliseconds, not seconds or ISO string — verify error messages help with format

---

## 3. Simplicity Assessment

**Goal:** Count steps and evaluate cognitive load for both human and AI users.

**Complexity Score:** Low (Feature 32), Medium (Feature 33), High (Feature 34)

| Workflow | Steps Required | Can Be Simplified? |
|----------|---------------|-------------------|
| Check captured errors via HTTP | 1 step: `curl localhost:7890/captured/errors` | No — already minimal |
| Call MCP tool via HTTP | 1 step: POST JSON-RPC to `/mcp` | No — already minimal |
| Run full UAT suite | 1 step: `node scripts/uat-runner.js` | No — harness handles setup/teardown |
| Run specific UAT phase | 1 step: `node scripts/uat-runner.js --phase 1` | No — already minimal |
| Verify specific capture pipeline | 2 steps: trigger action, then GET /captured/* | Could combine into a single harness test, but manual verification is inherently 2 steps |

### Default Behavior Verification
- [ ] `/captured/*` endpoints work with no configuration (default limit=100, since=0, format=json)
- [ ] `/mcp` endpoint uses same tool handling as stdio MCP (no separate configuration)
- [ ] Playwright harness auto-builds server before running (`make dev`)
- [ ] Harness cleans up processes on exit (no zombie kaboom servers or browser instances)

---

## 4. Code Test Plan

### 4.1 Unit Tests

| # | Test Case | Input | Expected Output | Priority |
|---|-----------|-------|-----------------|----------|
| UT-1 | GET /captured/actions returns captured actions | Populate action buffer, GET /captured/actions | JSON with entries array, count, truncated flag | must |
| UT-2 | GET /captured/network returns network entries | Populate network buffer, GET /captured/network | JSON with network entries | must |
| UT-3 | GET /captured/websocket returns WS events | Populate WS buffer, GET /captured/websocket | JSON with WebSocket entries | must |
| UT-4 | GET /captured/logs returns console logs | Populate log buffer, GET /captured/logs | JSON with log entries | must |
| UT-5 | GET /captured/errors returns errors only | Populate with mix of logs and errors, GET /captured/errors | JSON with error entries only | must |
| UT-6 | `limit` parameter caps results | 50 entries in buffer, `limit=10` | 10 entries returned, `truncated: true` | must |
| UT-7 | `since` parameter filters by time | Entries from different times, `since=<recent>` | Only entries after the timestamp | must |
| UT-8 | `format=jsonl` returns newline-delimited JSON | GET with `format=jsonl` | Each entry on its own line, no wrapping object | should |
| UT-9 | POST on /captured/* returns 405 | POST /captured/actions | HTTP 405 Method Not Allowed | must |
| UT-10 | GET /mcp returns 405 | GET /mcp | HTTP 405 Method Not Allowed | must |
| UT-11 | POST /mcp with valid tools/list | JSON-RPC tools/list request | Response with tools array matching stdio MCP | must |
| UT-12 | POST /mcp with valid tools/call observe | `{ method: "tools/call", params: { name: "observe", arguments: { what: "errors" } } }` | Same response as stdio observe | must |
| UT-13 | POST /mcp with invalid JSON | Malformed JSON body | JSON-RPC error -32700 Parse error | must |
| UT-14 | POST /mcp with unknown method | `{ method: "unknown/method" }` | JSON-RPC error -32601 Method not found | must |
| UT-15 | POST /mcp with initialize | `{ method: "initialize" }` | Server capabilities response | should |
| UT-16 | Empty buffer returns empty array | GET /captured/actions with no actions captured | `{ entries: [], count: 0, truncated: false }` | must |
| UT-17 | `since` with future timestamp | `since=9999999999999` | `{ entries: [], count: 0, truncated: false }` | should |
| UT-18 | `limit` with negative value | `limit=-1` | Uses default (100) or returns error | should |
| UT-19 | parseIntParam helper handles invalid input | `limit=abc` | Uses default value (100) | must |
| UT-20 | parseInt64Param helper handles invalid input | `since=notanumber` | Uses default value (0) | must |

### 4.2 Integration Tests

| # | Test Case | Components Involved | Expected Behavior | Priority |
|---|-----------|--------------------|--------------------|----------|
| IT-1 | Extension POSTs data, GET /captured returns it | Extension capture pipeline, server HTTP handlers, ring buffers | Extension sends log data via POST /logs; GET /captured/logs returns the same entries | must |
| IT-2 | /mcp endpoint invokes observe and returns results | HTTP handler, MCP dispatch, ring buffers | POST /mcp with observe tool call returns captured data | must |
| IT-3 | /mcp endpoint invokes interact tools with pilot check | HTTP handler, MCP dispatch, pilot toggle | POST /mcp with execute_js returns pilot-disabled error when toggle is off | must |
| IT-4 | Playwright harness launches browser with extension | uat-runner.js, Playwright, Chrome, extension, server | Harness builds and starts server, launches Chrome with extension, verifies connection | must |
| IT-5 | Playwright harness runs phase 1 tests | uat-runner.js, all components | Console error capture, log capture, network capture all pass | must |
| IT-6 | Playwright harness cleanup | uat-runner.js, process management | Server process killed, browser closed, no zombies | must |
| IT-7 | /captured and /mcp share the same data source | Extension, server buffers, both endpoints | Data captured by extension is accessible via both /captured/* and /mcp observe | should |

### 4.3 Performance Tests

| # | Test Case | Metric | Target | Priority |
|---|-----------|--------|--------|----------|
| PT-1 | GET /captured/actions with 1000 entries | Response time | < 50ms | must |
| PT-2 | GET /captured/network with limit=10 from 1000 | Response time | < 10ms | must |
| PT-3 | POST /mcp tools/call observe | Response time | < 100ms (matches stdio MCP) | must |
| PT-4 | POST /mcp tools/list | Response time | < 20ms | should |
| PT-5 | Playwright harness total execution time | Full UAT suite runtime | < 120s for all phases | should |
| PT-6 | JSONL format streaming performance | Time to serialize 1000 entries as JSONL | < 100ms | should |

### 4.4 Edge Case Tests

| # | Edge Case | Input/Scenario | Expected Behavior | Priority |
|---|-----------|---------------|-------------------|----------|
| EC-1 | Concurrent GET /captured during buffer write | Extension POSTing while AI GETs | RWMutex ensures consistent read; no partial entries | must |
| EC-2 | /mcp called with no Content-Type header | POST without Content-Type | Attempts JSON parse; returns -32700 if invalid | should |
| EC-3 | /mcp called with very large body | 10MB JSON body | Server rejects or handles gracefully (not OOM) | should |
| EC-4 | /captured with all parameters | `?limit=5&since=1234567890000&format=jsonl` | Correct filtering, limiting, and formatting applied | must |
| EC-5 | Harness with server already running on port | Port conflict during uat-runner.js | Harness detects conflict and uses a different port or errors clearly | should |
| EC-6 | Harness with extension not built | Extension directory missing or incomplete | Harness errors with actionable message: "Build extension first" | should |
| EC-7 | /mcp with batch JSON-RPC (array of requests) | `[{...}, {...}]` as body | Either processes batch or returns unsupported-batch error | should |
| EC-8 | /captured/network with network_bodies disabled | Body capture off, GET /captured/network | Entries returned without body data (consistent with capture state) | must |
| EC-9 | Harness test failure output | One scenario fails in uat-runner.js | JSON output includes failed test with error message and duration | must |
| EC-10 | /mcp with missing `id` field | Notification-style request to /mcp | Returns error (HTTP endpoint requires request-response, not notification) | should |

---

## 5. UAT Checklist (Human + AI)

> Step-by-step verification for a human working with an AI assistant. The AI executes MCP tool calls and HTTP requests; the human observes browser behavior and confirms results.

### Prerequisites
- [ ] Kaboom server running: `./dist/kaboom --port 7890`
- [ ] Chrome extension installed and connected
- [ ] A test page open (e.g., localhost:3000 or any web page with JS console access)
- [ ] `curl` or equivalent HTTP client available
- [ ] Node.js installed (for Playwright harness tests)

### Step-by-Step Verification

#### Feature 32: HTTP GET for Captured Data

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-1 | Human triggers `console.error("test-error-32")` in DevTools | Error appears in console | Error captured by extension | [ ] |
| UAT-2 | `curl http://localhost:7890/captured/errors` | Terminal output | JSON response with `entries` array containing the test error, `count >= 1`, `truncated: false` | [ ] |
| UAT-3 | `curl http://localhost:7890/captured/logs` | Terminal output | JSON response with log entries (if log_level captures them) | [ ] |
| UAT-4 | `curl "http://localhost:7890/captured/errors?limit=1"` | Terminal output | Exactly 1 entry returned, `truncated` may be true | [ ] |
| UAT-5 | `curl "http://localhost:7890/captured/errors?format=jsonl"` | Terminal output | One JSON object per line, no wrapping object | [ ] |
| UAT-6 | `curl -X POST http://localhost:7890/captured/errors` | Terminal output | HTTP 405 Method Not Allowed | [ ] |

#### Feature 33: HTTP MCP Endpoint

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-7 | `curl -X POST http://localhost:7890/mcp -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'` | Terminal output | JSON-RPC response with tools array containing observe, generate, configure, interact | [ ] |
| UAT-8 | `curl -X POST http://localhost:7890/mcp -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}'` | Terminal output | JSON-RPC response with error data matching what MCP stdio would return | [ ] |
| UAT-9 | `curl -X POST http://localhost:7890/mcp -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"interact","arguments":{"action":"execute_js","script":"1+1"}}}'` | Check extension popup: AI Web Pilot toggle state | If toggle OFF: error response with `ai_web_pilot_disabled`. If toggle ON: `{ success: true, result: 2 }` | [ ] |
| UAT-10 | `curl -X POST http://localhost:7890/mcp -H "Content-Type: application/json" -d 'invalid json'` | Terminal output | JSON-RPC error with code -32700 (Parse error) | [ ] |
| UAT-11 | `curl -X GET http://localhost:7890/mcp` | Terminal output | HTTP 405 Method Not Allowed | [ ] |

#### Feature 34: Playwright Harness

| # | Step (AI executes) | Human Observes | Expected Result | Pass |
|---|-------------------|----------------|-----------------|------|
| UAT-12 | `node scripts/uat-runner.js --phase 1 --json` | Browser launches (if --headed), tests run | JSON output with phase1 results, all tests show status | [ ] |
| UAT-13 | After harness completes, check for zombie processes | `ps aux \| grep kaboom` | No orphaned kaboom or Chrome processes | [ ] |

### Data Leak UAT Verification

| # | Check | Method | Expected | Pass |
|---|-------|--------|----------|------|
| DL-UAT-1 | /captured/* not accessible externally | From another machine: `curl http://<server-ip>:7890/captured/errors` | Connection refused | [ ] |
| DL-UAT-2 | /captured/network strips auth headers | Trigger a request with Authorization header, then GET /captured/network | Entry does not contain Authorization header value | [ ] |
| DL-UAT-3 | /mcp respects pilot toggle | POST /mcp with execute_js when pilot is OFF | Error: ai_web_pilot_disabled | [ ] |
| DL-UAT-4 | Harness output contains no secrets | Run harness with test page that has auth data, check JSON output | No auth tokens, cookies, or API keys in output | [ ] |

### Regression Checks
- [ ] Existing POST endpoints (/logs, /network-bodies, etc.) still work for extension ingestion
- [ ] Existing MCP stdio interface unaffected by /mcp HTTP endpoint
- [ ] Server startup time not significantly increased by new routes
- [ ] Ring buffer behavior unchanged (eviction, sizing)

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
