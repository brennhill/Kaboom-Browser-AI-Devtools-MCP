---
feature: kaboom-ci
status: proposed
doc_type: tech-spec
feature_id: feature-kaboom-ci
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Kaboom CI Infrastructure

> Plain language only. No code. Describes HOW the Kaboom CI infrastructure works at a high level.

## Architecture Overview

Kaboom CI enables browser telemetry capture in headless CI environments without a Chrome extension. The architecture has four layers:

1. **Capture Script** — JavaScript file injected into page context that captures console, network, WebSocket, exceptions
2. **Server Endpoints** — HTTP endpoints (/snapshot, /clear, /test-boundary) for CI-specific operations
3. **Test Framework Fixture** — Playwright/Puppeteer integration that automates capture lifecycle
4. **Reporting Integration** — Generate artifacts (HAR, SARIF) from captured telemetry for CI outputs

The server runs as a sidecar process in CI (started before tests, kept alive throughout). Tests connect to localhost:7890 (same as local development). The capture script sends data to the server via HTTP POST, using the exact same endpoints the Chrome extension uses (/logs, /network-bodies, /websocket-events).

## Key Components

### 1. CI Capture Script (kaboom-ci.js)

**Purpose:** Replicate Chrome extension capture without extension APIs.

#### What it captures:
- Console logs (console.log, console.error, console.warn)
- Unhandled exceptions (window.onerror, unhandledrejection events)
- Network requests and responses (fetch/XHR interception)
- WebSocket connections (WebSocket constructor patching)
- Performance metrics (PerformanceObserver)

#### How it works:
- Injected into page context via `page.addInitScript()` (Playwright) or `page.evaluateOnNewDocument()` (Puppeteer)
- Runs before page JavaScript executes (earliest possible interception)
- Patches global objects (window.fetch, XMLHttpRequest, WebSocket)
- Batches events and sends via navigator.sendBeacon() or fetch() POST to server
- Uses same HTTP endpoints as extension (/logs, /network-bodies, /websocket-events)
- Includes double-initialization guard (window.__KABOOM_CI_INITIALIZED) to prevent duplicates on page navigation

#### Configuration:
- Server host/port configurable via window globals (window.__KABOOM_HOST, window.__KABOOM_PORT)
- Defaults to 127.0.0.1:7890 (same as local)
- Debug mode via window.__KABOOM_DEBUG (logs batch sends to console)
- Capture version sent with each batch for compatibility tracking

#### Differences from extension:
- No chrome.runtime APIs (uses fetch/sendBeacon for transport)
- No background service worker (all logic in page context)
- No tab ID (CI typically runs single-tab tests)
- Simplified error handling (fail silently if server unreachable)

### 2. Server CI Endpoints

#### GET /snapshot
**Purpose:** Return all captured state in a single response.

**When used:** Called by fixture after test failure or at end of test to grab full context.

##### Query parameters:
- `since` (ISO 8601 timestamp) — Return only data after this time
- `test_id` (string) — Filter to specific test (if test boundaries used)

##### Response:
- logs array (all captured console/exception entries)
- websocket_events array
- network_bodies array (for failed requests or if body capture enabled)
- enhanced_actions array (if AI Web Pilot used)
- stats object (error_count, warning_count, network_failures, ws_connections)
- timestamp (server time when snapshot taken)
- test_id (if filtering applied)

**Performance:** Snapshot is in-memory read from ring buffers. Target < 200ms for typical capture (1000 log entries, 100 network bodies).

#### POST /clear
**Purpose:** Reset all ring buffers to empty state.

**When used:** Called by fixture between tests to ensure test isolation (test B doesn't see test A's data).

**Request body:** None (or optional confirmation flag)

##### Response:
- cleared (boolean, always true)
- entries_removed (integer, total entries purged across all buffers)
- timestamp (server time when clear occurred)

##### What clears:
- Log entries buffer
- Extension logs buffer (not relevant for CI but cleared anyway)
- Network bodies buffer
- WebSocket events buffer
- Network waterfall buffer
- Enhanced actions buffer
- Pending queries (not relevant for CI but cleared)

**Performance:** Clear is O(1) (reset pointers, let GC reclaim). Target < 10ms.

#### POST /test-boundary
**Purpose:** Mark start/end of a test case for correlation.

**When used:** Called by fixture at beginning of test (action: "start") and end of test (action: "end").

##### Request body:
- test_id (string) — Unique identifier for test (e.g., "login-flow" or full path "tests/e2e/login.spec.ts > should login")
- action (enum: "start" or "end")
- timestamp (optional, defaults to server time)

##### Response:
- acknowledged (boolean)
- test_id (echo)
- action (echo)
- timestamp (server time)

**Usage:** Allows /snapshot to filter by test_id. Enables multi-test correlation (which telemetry belongs to which test).

**Current limitation:** Single-tab tracking means concurrent tests (Playwright parallel workers) share telemetry. Test boundaries help but don't fully isolate. Future enhancement: per-worker server instances or tab ID correlation.

### 3. Playwright Test Fixture (@anthropic/kaboom-playwright)

**Purpose:** Automate capture lifecycle in Playwright tests.

#### What it provides:
- Inject capture script before every page navigation
- Call /test-boundary at test start and end
- Capture snapshot on test failure
- Attach snapshot to Playwright test report (JSON and human-readable text)
- Call /clear between tests (fixture teardown)

#### Usage:
```javascript
// In test file
import { test } from '@anthropic/kaboom-playwright';

test('should login', async ({ page }) => {
  // Capture script automatically injected
  await page.goto('http://localhost:3000');
  // Test logic here
  // Snapshot automatically captured on failure
});
```

#### Fixture lifecycle:
1. Setup: Inject capture script via page.addInitScript()
2. Test start: POST /test-boundary with action: "start"
3. Test runs: Page events captured and sent to server
4. Test end: POST /test-boundary with action: "end"
5. On failure: GET /snapshot → attach to report
6. Teardown: POST /clear → reset buffers

#### Configuration options:
- kaboomHost (default: "127.0.0.1")
- kaboomPort (default: 7890)
- kaboomAttachOnFailure (default: true)
- kaboomAutoStart (reserved for future auto-start server feature)

### 4. CI Reporting Integration

**Purpose:** Generate CI-compatible artifacts from captured telemetry.

#### Artifacts:

#### HAR Export
- Call `generate({what: "har"})` via MCP after tests complete
- Produces standard HAR 1.2 file from network_bodies and network_waterfall buffers
- Upload as GitHub Actions artifact or attach to CI report
- Developers can download and import into Charles Proxy, Postman, DevTools

#### SARIF Export
- Call `generate({what: "sarif"})` after security or accessibility audits
- Produces SARIF 2.1.0 file with findings as results
- Upload to GitHub Code Scanning API for PR annotations
- Findings appear inline on changed files

#### PR Summary
- Call `generate({what: "pr_summary"})` after tests
- Produces markdown summary of errors, network failures, performance metrics
- Post as PR comment or attach to CI report
- Gives reviewer high-level test health without diving into logs

##### GitHub Actions Integration:
```yaml
- name: Start Kaboom
  run: npx kaboom-mcp &
- name: Run Tests
  run: npx playwright test
- name: Generate HAR
  run: npx kaboom-cli generate --format har --output ./har-export.har
- name: Upload Artifacts
  uses: actions/upload-artifact@v3
  with:
    name: kaboom-telemetry
    path: ./har-export.har
```

## Data Flows

### Capture Flow (CI Script to Server)
```
Page loads → Capture script injected via addInitScript()
→ Script initializes: patch fetch, XHR, WebSocket, console
→ Page executes → console.error("fail") → Captured by script
→ Script batches event → POST /logs to server (127.0.0.1:7890)
→ Server receives → stores in logs ring buffer
→ (Same for network, WebSocket, etc.)
```

### Test Failure Flow (Fixture to Report)
```
Test runs → Assertion fails → Playwright test fails
→ Fixture teardown: GET /snapshot from server
→ Server returns: {logs, network_bodies, websocket_events, stats}
→ Fixture serializes to JSON → attach to Playwright report as "kaboom-snapshot.json"
→ Fixture formats human-readable summary → attach as "kaboom-summary.txt"
→ CI uploads Playwright report → Developers can download attachments
```

### Multi-Test Flow (Isolation)
```
Test 1 starts → POST /test-boundary {test_id: "test1", action: "start"}
→ Test 1 runs → events captured → tagged with test_id
→ Test 1 ends → POST /test-boundary {test_id: "test1", action: "end"}
→ Fixture teardown: POST /clear → buffers reset
→ Test 2 starts → POST /test-boundary {test_id: "test2", action: "start"}
→ Test 2 runs → events captured → no data from test 1
```

### Artifact Generation Flow (Telemetry to CI Output)
```
All tests complete → CI job still running
→ Call MCP tool: generate({what: "har"})
→ Server reads network_bodies + network_waterfall buffers
→ Generates HAR 1.2 JSON
→ Returns to MCP client (or via HTTP if using CLI)
→ Write to file: har-export.har
→ GitHub Actions: upload-artifact with path: har-export.har
→ Artifact downloadable from Actions UI for 90 days
```

## Implementation Strategy

### Phase 1: Capture Script (kaboom-ci.js)
1. Implement console capture (patch console.log, .error, .warn)
2. Implement exception capture (window.onerror, unhandledrejection)
3. Implement fetch/XHR interception (patch global fetch and XMLHttpRequest.prototype)
4. Implement WebSocket interception (patch WebSocket constructor)
5. Implement batching and send (navigator.sendBeacon with fallback to fetch)
6. Add double-initialization guard
7. Add configuration via window globals
8. Test on Playwright and Puppeteer

### Phase 2: Server Endpoints
1. Implement GET /snapshot (read from existing ring buffers, serialize all)
2. Implement POST /clear (reset all ring buffers)
3. Implement POST /test-boundary (store current test_id, mark timestamps)
4. Update existing POST endpoints (/logs, /network-bodies) to accept CI script data (already compatible, verify)
5. Add CORS headers for localhost (already present, verify)

### Phase 3: Playwright Fixture
1. Create NPM package @anthropic/kaboom-playwright
2. Implement fixture with kaboom page option
3. Inject capture script via page.addInitScript()
4. Implement test boundary calls (beforeEach, afterEach)
5. Implement snapshot capture on failure (test.afterEach with testInfo.status)
6. Implement report attachments (testInfo.attach)
7. Implement /clear call in teardown
8. Add configuration options (host, port, attachOnFailure)

### Phase 4: Puppeteer Integration (Manual, No Fixture)
1. Document manual injection via page.evaluateOnNewDocument()
2. Provide example script for reading kaboom-ci.js file
3. Document manual /snapshot and /clear calls
4. Note: No fixture abstraction for Puppeteer (lower priority, can be added later)

### Phase 5: CI Pipeline Examples
1. Create GitHub Actions workflow example
2. Create GitLab CI example
3. Create CircleCI example
4. Document artifact upload patterns
5. Document SARIF upload to GitHub Code Scanning

### Phase 6: Reporting Integration
1. Verify existing MCP generate tool works with CI-captured data
2. Add CLI wrapper for non-MCP usage: `npx kaboom-cli generate --format har`
3. Document artifact generation in CI
4. Add GitHub Actions examples for SARIF upload

## Edge Cases & Assumptions

### Edge Case 1: Server Not Running When Tests Start
**Handling:** Capture script sends data but receives no response (server unreachable). Script fails silently (logs to console if debug mode). Tests proceed unaffected. Fixture's /snapshot call returns empty or errors; attach empty snapshot.

### Edge Case 2: Page Navigation Re-Initializes Script
**Handling:** Double-initialization guard (window.__KABOOM_CI_INITIALIZED) prevents duplicate patching. Script checks guard, skips if already initialized.

### Edge Case 3: Very Long Test Suite (1000+ tests)
**Handling:** /clear between tests prevents memory growth. Ring buffers have size limits (1000 logs, 100 network bodies). Oldest entries evicted. Server memory remains bounded.

### Edge Case 4: Concurrent Test Workers (Playwright Parallel)
**Handling:** Known limitation. All workers send to same server instance. Telemetry interleaved. Test boundary markers help but don't fully isolate. Future enhancement: per-worker server instances on different ports or tab ID correlation.

### Edge Case 5: Binary Response Bodies (Images, PDFs)
**Handling:** Capture script skips binary content (checks Content-Type header). Only text/*, application/json, application/xml captured. Prevents bloating network_bodies buffer with useless data.

### Edge Case 6: Large Response Bodies (> 5KB)
**Handling:** Capture script truncates at 5120 characters (MAX_RESPONSE_LENGTH). Sets truncated flag. Prevents memory exhaustion.

### Assumption 1: CI Environment Has Localhost Access
We assume the Kaboom server binds to 127.0.0.1 and tests run on the same machine. If tests run in separate containers (e.g., Docker Compose), networking must allow localhost access or use container name.

### Assumption 2: Server Survives Entire Test Suite
We assume `npx kaboom-mcp` started before tests and not killed until after. If server crashes, telemetry capture stops (tests still run but no data).

### Assumption 3: Single Browser Context
We assume tests run in one browser context at a time. Multiple contexts in parallel (Playwright sharding) may interleave telemetry. Test boundaries help but don't fully isolate.

## Risks & Mitigations

### Risk 1: Capture Script Breaks Application
**Mitigation:** Capture script patches globals carefully (use try/catch, preserve original behavior). Test on multiple applications. If patching fails, capture degrades gracefully (don't crash app).

### Risk 2: Performance Impact from Interception
**Mitigation:** Batching reduces HTTP overhead (flush every 100ms, max 50 entries per batch). navigator.sendBeacon is async (doesn't block). Target < 0.5ms overhead per request.

### Risk 3: Server Memory Exhaustion in Long CI Runs
**Mitigation:** Ring buffers enforce size limits (1000 logs, 100 network bodies, 8MB total). /clear between tests prevents accumulation. Server memory stays under 50MB.

### Risk 4: CI Artifact Storage Costs
**Mitigation:** HAR files and snapshots compressed (gzip). Typical snapshot: 50KB-500KB. GitHub Actions has 500MB artifact storage per run (plenty). Document cleanup policies.

### Risk 5: SARIF Upload Rate Limiting
**Mitigation:** GitHub Code Scanning API allows 20 SARIF uploads per commit per 3 hours. Kaboom generates one SARIF per CI run. Unlikely to hit limit unless running CI hundreds of times per hour.

## Dependencies

### Depends On (Existing Features)
- **Ring buffers** — /snapshot reads from existing logs, network_bodies, websocket_events buffers
- **POST /logs, /network-bodies, /websocket-events** — Capture script uses same endpoints as extension
- **generate tool** — HAR, SARIF, PR summary generation already implemented

### New Dependencies
- **CI capture script (kaboom-ci.js)** — New component, no Kaboom dependencies, pure JavaScript
- **Playwright fixture package** — Depends on Playwright test runner, Kaboom server running
- **GitHub Actions integration** — Optional, depends on GitHub Actions and gh CLI

## Performance Considerations

- Capture script initialization: < 5ms (script injection overhead)
- Console log capture overhead: < 0.1ms per log
- Fetch intercept overhead: < 0.5ms per request
- Batch flush latency: 100ms interval, < 10ms per POST
- /snapshot response time: < 200ms for 1000 entries
- /clear response time: < 10ms
- Server memory in CI: < 50MB (ring buffers capped)
- Artifact size: 50KB-500KB per test failure (snapshot JSON)

## Security Considerations

- **Localhost-only:** Server binds 127.0.0.1. Not accessible from other machines in CI environment (unless Docker networking misconfigured).
- **Sensitive headers redacted:** Capture script strips Authorization, Cookie, Set-Cookie (same as extension).
- **Body capture opt-in:** Network bodies only captured for 4xx/5xx by default (or if explicitly enabled). Prevents capturing sensitive request payloads by default.
- **No remote code loading:** Capture script bundled as static file. No CDN requests. Chrome Web Store policy compliant.
- **SARIF privacy:** SARIF reports may contain code snippets. Review before uploading to public repos. GitHub Code Scanning respects repo visibility (private repos = private findings).

## Test Plan Reference

See qa-plan.md for detailed testing strategy. Key test scenarios:
1. Capture script captures console errors in Playwright headless
2. Capture script captures network failures
3. /snapshot returns all captured data
4. /clear resets buffers between tests
5. Playwright fixture attaches snapshot on failure
6. HAR generation from CI-captured data
7. SARIF upload to GitHub Code Scanning
8. Server memory stays bounded after 100 tests
