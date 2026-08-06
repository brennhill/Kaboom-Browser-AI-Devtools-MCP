---
feature: kaboom-ci
status: proposed
version: null
tool: observe, generate
mode: observe(errors, logs, network_waterfall, network_bodies, websocket_events, performance, timeline), generate(har, sarif)
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-kaboom-ci
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Kaboom CI Infrastructure (v6)

> Enables Kaboom browser telemetry capture inside CI/CD pipelines so that automated test runs produce the same observability artifacts available during local development -- console logs, network bodies, WebSocket events, performance metrics -- without requiring the Chrome extension.

## Problem

Today Kaboom requires a running Chrome extension connected to a developer's browser. This means:

1. **CI is blind.** When a Playwright or Cypress test fails in GitHub Actions, the developer sees "test failed" but not _why_. No console errors, no network request bodies, no WebSocket state. The developer must pull the branch, reproduce locally, open DevTools, and investigate -- a process that takes 15-60 minutes per failure and succeeds only ~60% of the time (many CI failures are environment-specific and do not reproduce locally).

2. **AI agents cannot diagnose CI failures.** The Agentic CI/CD features (self-healing tests, agentic E2E repair, PR preview exploration) all require browser telemetry at the point of failure. Without Kaboom running in CI, these features are limited to local development.

3. **No regression baseline.** Performance budgets, security audits, and accessibility checks run locally but not in CI. Regressions slip through because the gate only exists on one developer's machine.

4. **Artifact gap.** HAR exports, SARIF reports, and CSP generation are available locally but cannot be produced as part of a CI pipeline. Teams cannot attach browser-level evidence to PRs or archive it alongside test results.

## Solution

Kaboom CI provides a layered integration that enables full browser telemetry capture in headless CI environments:

**Layer 1: Capture Script** (`@anthropic/kaboom-ci`) -- A standalone JavaScript file that replicates the Chrome extension's capture capabilities (console, network, WebSocket, exceptions) without any browser extension APIs. Injected via Playwright's `addInitScript()` or Puppeteer's `evaluateOnNewDocument()`. Sends telemetry directly to the Kaboom server via HTTP POST, matching the same wire format the extension uses.

**Layer 2: Server Endpoints** -- Three HTTP endpoints on the existing Kaboom server enable CI-specific workflows: `/snapshot` (retrieve all captured state), `/clear` (reset buffers between tests), and `/test-boundary` (correlate telemetry to specific test cases).

**Layer 3: Test Framework Fixture** (`@anthropic/kaboom-playwright`) -- A Playwright test fixture that automates the lifecycle: inject the capture script, mark test boundaries, capture snapshots on failure, attach context to Playwright reports, and clear between tests.

**Layer 4: Reporting** -- After tests complete, captured telemetry feeds into existing Kaboom artifact generators (HAR export, SARIF export, PR summary) to produce CI-compatible reports. GitHub Actions integration uploads SARIF for Code Scanning annotations and attaches HAR files to workflow artifacts.

The Kaboom server itself runs as a sidecar process in CI -- started before tests, kept alive throughout, queried after. It requires no Chrome extension, no display server, and no additional infrastructure. The `npx kaboom-mcp` command that developers use locally works identically in CI.

## User Stories

- As a developer, I want to see console errors, network failures, and WebSocket state alongside my CI test results so that I can diagnose failures without reproducing locally.
- As an AI coding agent, I want to capture browser telemetry during CI test runs so that I can diagnose and fix test failures autonomously (self-healing tests).
- As a team lead, I want performance budgets and accessibility audits enforced in CI so that regressions are caught before merge, not after deployment.
- As a developer, I want a HAR file attached to every failed test so that I can replay the exact network traffic in Charles Proxy or Postman without any manual capture.
- As a security engineer, I want SARIF reports generated from CI test runs so that security findings appear as GitHub Code Scanning annotations on every PR.
- As a developer using Playwright, I want Kaboom integration to require only two lines of setup (import the fixture, use it in tests) so that adoption has near-zero friction.

## MCP Interface

Kaboom CI does not introduce new MCP tools. It relies entirely on existing tools and HTTP endpoints. The integration points are:

### Existing MCP Tools (used from CI context)

All existing `observe` and `generate` modes work identically whether telemetry was captured by the Chrome extension or by the CI capture script. The server does not distinguish between data sources.

**Tool:** `observe`
**Relevant modes:** `errors`, `logs`, `network_waterfall`, `network_bodies`, `websocket_events`, `performance`, `timeline`, `security_audit`, `accessibility`

**Tool:** `generate`
**Relevant modes:** `har`, `sarif`, `csp`, `pr_summary`

### CI-Specific HTTP Endpoints (not MCP tools)

These endpoints are called directly by the test framework fixture, not by MCP clients. They exist because test frameworks operate outside the MCP protocol.

#### GET /snapshot

Returns all captured state in a single response. Used by the fixture after test failure to grab everything at once.

Request:
```json
{
  "method": "GET",
  "url": "/snapshot",
  "query": {
    "since": "2026-01-28T10:00:00Z",
    "test_id": "login-flow"
  }
}
```

Response:
```json
{
  "timestamp": "2026-01-28T10:05:30.123Z",
  "test_id": "login-flow",
  "logs": [
    { "level": "error", "message": "TypeError: Cannot read property 'map' of undefined", "source": "exception", "url": "http://localhost:3000/dashboard" }
  ],
  "websocket_events": [],
  "network_bodies": [
    { "url": "http://localhost:3000/api/users", "method": "GET", "status": 500, "responseBody": "{\"error\":\"Internal Server Error\"}" }
  ],
  "enhanced_actions": [],
  "stats": {
    "total_logs": 1,
    "error_count": 1,
    "warning_count": 0,
    "network_failures": 1,
    "ws_connections": 0
  }
}
```

#### POST /clear

Resets all server buffers. Called between tests to prevent cross-contamination.

Request:
```json
{
  "method": "POST",
  "url": "/clear"
}
```

Response:
```json
{
  "cleared": true,
  "entries_removed": 47
}
```

#### POST /test-boundary

Marks the start or end of a test case for telemetry correlation.

Request:
```json
{
  "method": "POST",
  "url": "/test-boundary",
  "body": {
    "test_id": "login-flow > should redirect to dashboard",
    "action": "start"
  }
}
```

Response:
```json
{
  "test_id": "login-flow > should redirect to dashboard",
  "action": "start",
  "timestamp": "2026-01-28T10:05:00.000Z"
}
```

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Capture script (`kaboom-ci.js`) captures console logs, errors, unhandled exceptions, unhandled rejections, fetch/XHR responses (status >= 400), and WebSocket lifecycle events identically to the Chrome extension | must |
| R2 | Capture script posts telemetry to the Kaboom server using the same wire format as the Chrome extension (`/logs`, `/websocket-events`, `/network-bodies`) | must |
| R3 | Capture script batches entries and flushes every 100ms with a maximum batch buffer of 1000 entries per type to prevent unbounded memory growth | must |
| R4 | Capture script strips sensitive headers (Authorization, Cookie, etc.) using the same list as `inject.js` | must |
| R5 | Capture script has a guard against double initialization (`window.__KABOOM_CI_INITIALIZED`) | must |
| R6 | `/snapshot` endpoint returns all logs, WebSocket events, network bodies, enhanced actions, and computed stats in a single GET response | must |
| R7 | `/snapshot` supports `since` query parameter (ISO 8601) to return only entries after a given timestamp | must |
| R8 | `/snapshot` supports `test_id` query parameter to label the snapshot; falls back to the current test ID set via `/test-boundary` | must |
| R9 | `/clear` endpoint resets all server and capture buffers atomically | must |
| R10 | `/test-boundary` endpoint accepts `start` and `end` actions with a `test_id` to set/clear the current test context | must |
| R11 | Playwright fixture (`@anthropic/kaboom-playwright`) injects the capture script via `addInitScript`, manages test boundaries, and clears between tests automatically | must |
| R12 | Playwright fixture captures a snapshot and attaches it as a JSON artifact when a test fails (configurable via `kaboomAttachOnFailure` option) | must |
| R13 | Playwright fixture produces a human-readable failure summary (text format) in addition to the raw JSON snapshot | must |
| R14 | Capture script skips requests to the Kaboom server itself to avoid infinite capture loops | must |
| R15 | Capture script is configurable via `window.__KABOOM_HOST`, `window.__KABOOM_PORT`, and `window.__KABOOM_TEST_ID` globals | must |
| R16 | The Kaboom server starts and operates identically in CI and local environments -- no special `--ci` flag or mode switch required | must |
| R17 | `/snapshot` supports pagination via `offset` and `limit` query parameters for large result sets | should |
| R18 | Capture script includes debug logging when `window.__KABOOM_DEBUG` is set, printing send failures and batch sizes to console.warn | should |
| R19 | Capture script implements retry with exponential backoff for critical data (errors, test boundaries) when server is temporarily unreachable | should |
| R20 | Server restricts CORS to localhost origins only when running in CI mode (detected via `CI` environment variable) | should |
| R21 | Playwright fixture redacts sensitive data in snapshots before attaching to test reports | should |
| R22 | Capture script negotiates version with server via a `captureVersion` field in the initialization log entry | should |
| R23 | Cypress integration fixture with equivalent functionality to the Playwright fixture | could |
| R24 | GitHub Action wrapper that starts the Kaboom server, runs tests, generates reports, and uploads artifacts | could |
| R25 | JUnit XML report integration that embeds Kaboom snapshot URLs in test failure output | could |

## Non-Goals

- **This feature does NOT create a cloud-hosted Kaboom service.** Kaboom CI runs the same localhost server that developers use. There is no hosted SaaS component, no data leaves the CI runner, and no account or API key is required.

- **This feature does NOT require a headed browser or display server.** The capture script runs in any browser context (headed or headless). No xvfb or virtual display is needed for telemetry capture (though the test framework itself may require one for screenshot capture).

- **This feature does NOT replace Playwright's built-in tracing.** Playwright traces capture screenshots, DOM snapshots, and network HAR files natively. Kaboom CI captures _runtime_ telemetry that Playwright does not: console errors with full arguments, WebSocket message content, unhandled exceptions with serialized error objects, and cross-request network body correlation. The two are complementary.

- **This feature does NOT add new MCP tools.** The 5-tool model is preserved. CI integration uses existing HTTP endpoints and existing MCP tool modes. See the architecture reference in [360-observability-architecture.md](../../../core/architecture/360-observability-architecture.md).

- **Out of scope: multi-worker test isolation by worker ID.** The current `/test-boundary` endpoint tracks a single `currentTestID` on the server. Parallel test workers that share a single Kaboom server instance will overwrite each other's test boundary. Full per-worker isolation requires either separate server instances per worker or client-side tagging of every entry with a `test_id`. This is tracked as OI-1.

- **Out of scope: Puppeteer fixture package.** Only Playwright is supported in the initial release. Puppeteer users can manually use `page.evaluateOnNewDocument()` with the capture script file.

## Performance SLOs

| Metric | Target | Rationale |
|--------|--------|-----------|
| Capture script initialization | < 5ms | Must not delay page load in test |
| Per-event capture overhead | < 0.1ms for console, < 0.5ms for fetch intercept | Same SLOs as the Chrome extension |
| Batch flush (HTTP POST) | < 10ms for 50 entries | Fire-and-forget; must not block test execution |
| `/snapshot` response time | < 200ms for 1000 log entries + 100 network bodies | Acceptable test teardown overhead |
| `/clear` response time | < 10ms | Buffer reset is O(1) via nil assignment |
| Capture script memory footprint | < 5MB | Must not cause browser OOM in long test suites |
| Server memory (CI session) | < 50MB | Ring buffers cap growth; same limits as local mode |

## Security Considerations

- **Sensitive header stripping.** The capture script strips the same headers as the Chrome extension: Authorization, Cookie, Set-Cookie, X-Auth-Token, X-API-Key, X-CSRF-Token, Proxy-Authorization. Header values are replaced with `[REDACTED]`.

- **Network body content.** Request and response bodies may contain secrets (API keys in JSON payloads, tokens in response bodies). Kaboom's existing redaction patterns apply. The `/snapshot` response includes raw bodies -- the Playwright fixture should apply additional redaction before attaching to reports (R21).

- **CI runner isolation.** The Kaboom server binds to 127.0.0.1 only. In shared CI environments (e.g., self-hosted runners running multiple jobs), port conflicts are possible. The fixture's `kaboomPort` option allows configuration to avoid conflicts.

- **CORS in CI.** The existing CORS middleware allows `*` origins. In CI environments, tightening to localhost-only reduces the attack surface (R20). This can be automatically triggered when the `CI` environment variable is set (standard across GitHub Actions, GitLab CI, CircleCI, Jenkins).

- **Artifact sensitivity.** Snapshot attachments in Playwright reports may be stored as GitHub Actions artifacts (publicly downloadable for 90 days on public repos). Teams should review redaction settings before enabling `kaboomAttachOnFailure` on public repositories.

- **No auth headers on CI endpoints.** The `/snapshot`, `/clear`, and `/test-boundary` endpoints have no authentication. This is acceptable because the server binds to localhost only, but in CI environments with multiple processes, any local process can read/clear captured data.

## Edge Cases

- **Kaboom server not running when tests start.** The capture script sends telemetry via `sendBeacon` and `fetch` with swallowed errors. Data is silently lost. The Playwright fixture catches server-unreachable errors silently. Tests run normally but without observability. The fixture could optionally fail-fast with a health check (OI-3).

- **Server crashes mid-test.** In-flight batches in the capture script are lost. The Playwright fixture's `getSnapshot()` returns a fallback empty snapshot. The test result is unaffected; only the observability attachment is missing.

- **Very long test suite (1000+ tests).** Each test clears buffers, so memory stays bounded. However, if the fixture accumulates snapshot attachments, Playwright report size may grow. The snapshot JSON for a single test is typically < 100KB.

- **Page navigates during test.** The capture script re-initializes on each new page because `addInitScript` runs on every navigation. The double-initialization guard prevents duplicate capture setup. Telemetry from all pages within a test lands in the same server buffers and is captured by the end-of-test snapshot.

- **Browser context with multiple pages.** Each page in a Playwright browser context gets its own capture script instance. All instances POST to the same server. The snapshot aggregates data from all pages. There is no per-page filtering in the snapshot (data is interleaved chronologically).

- **Test timeout.** If a test times out, Playwright calls the fixture teardown. The fixture still attempts `getSnapshot()` and attachment. The `markTest(testId, 'end')` call may not complete if the server is also unresponsive.

- **Concurrent test workers (parallel execution).** Multiple workers share a single Kaboom server. Test boundary markers from different workers overwrite each other. Data from all workers is interleaved in server buffers. This is the primary limitation of the current design -- see OI-1.

- **Large network response bodies.** The capture script truncates response bodies at 5120 characters (`MAX_RESPONSE_LENGTH`). Binary responses are not captured (they bypass the text extraction). The snapshot includes truncated content with no explicit truncation marker (unlike HAR export which adds a comment).

## Dependencies

- **Depends on:**
  - Kaboom server (existing) -- provides HTTP endpoints, ring buffers, MCP tools
  - Existing capture logic in `inject.js` -- the CI script replicates key behaviors
  - HAR export (shipped) -- used for generating HAR artifacts from CI captures
  - SARIF export (shipped) -- used for generating SARIF reports from CI captures
  - Performance budgets (shipped) -- used for CI performance gating

- **Depended on by:**
  - Self-Healing Tests (proposed) -- requires CI capture to diagnose failures
  - Agentic E2E Repair (proposed) -- requires CI capture for API contract analysis
  - PR Preview Exploration (proposed) -- requires CI capture for exploratory testing
  - Deployment Watchdog (proposed) -- requires CI capture for post-deploy monitoring

## Components

### Component 1: Capture Script (`@anthropic/kaboom-ci`)

A single self-contained JavaScript file (`kaboom-ci.js`) that runs in any browser page context. It is the CI equivalent of `inject.js` + `content.js` + `background.js` combined into one file with a direct HTTP transport instead of the Chrome extension message-passing chain.

#### What it captures:
- Console output (log, warn, error, info, debug) with serialized arguments
- Uncaught exceptions (window.onerror) with stack traces
- Unhandled promise rejections with reason serialization
- Fetch responses with status >= 400 (URL, method, status, request/response bodies, headers)
- WebSocket lifecycle events (connecting, open, message, close, error) with message content
- Lifecycle signals (initialization log entry with capture version and test ID)

#### What it does NOT capture (compared to Chrome extension):
- DOM structure and accessibility tree (requires extension APIs or Playwright's built-in methods)
- Resource timing waterfall (available via Performance API but not currently extracted)
- User actions (clicks, inputs) -- the extension's action tracking uses content script injection
- Web Vitals (LCP, CLS, INP) -- requires PerformanceObserver setup not yet in the CI script

#### Transport:
- Batches entries by type (logs, WebSocket, network bodies)
- Flushes every 100ms or on page unload (`beforeunload` event)
- Maximum 50 entries per flush, maximum 1000 buffered per type
- Uses `navigator.sendBeacon` for reliability (falls back to `fetch` with `keepalive: true`)
- Silently swallows all transport errors -- never interferes with the application under test

#### Distribution:
- npm package: `@anthropic/kaboom-ci`
- Single file, zero dependencies
- Compatible with `page.addInitScript({ path })` (Playwright) and `page.evaluateOnNewDocument(fs.readFileSync(...))` (Puppeteer)

### Component 2: CI Server Endpoints

Three HTTP endpoints registered on the existing Kaboom server (already implemented in `ci.go`):

- `GET /snapshot` -- Aggregates all server and capture buffers into a single JSON response. Supports `since` (timestamp filter) and `test_id` (labeling) query parameters.
- `POST /clear` (also `DELETE /clear`) -- Atomically resets all buffers (logs, network bodies, WebSocket events, enhanced actions, test ID).
- `POST /test-boundary` -- Sets or clears the current test ID for correlation. Accepts `{ test_id, action: "start"|"end" }`.

These endpoints are already implemented and tested (`cmd/browser-agent/ci.go`, `cmd/browser-agent/ci_test.go`).

### Component 3: Playwright Fixture (`@anthropic/kaboom-playwright`)

A Playwright test fixture that automates the full lifecycle:

1. **Setup:** Injects `kaboom-ci.js` via `addInitScript`. Posts `start` to `/test-boundary`.
2. **During test:** The `kaboom` fixture object exposes `getSnapshot(since?)`, `clear()`, and `markTest(id, action)` for manual use within tests.
3. **Teardown:** If the test failed and `kaboomAttachOnFailure` is true, captures a snapshot and attaches it to the Playwright report as both JSON and a human-readable text summary. Posts `end` to `/test-boundary`. Calls `/clear`.

#### Configuration options:
- `kaboomPort` (default: 7890) -- Server port
- `kaboomAutoStart` (default: true) -- Reserved for future auto-start capability
- `kaboomAttachOnFailure` (default: true) -- Whether to grab and attach snapshots on test failure

#### Distribution:
- npm package: `@anthropic/kaboom-playwright`
- Peer dependency: `@playwright/test >= 1.40.0`
- Dependency: `@anthropic/kaboom-ci`

### Component 4: Reporting Integration (Future)

After tests complete, the captured telemetry can be used to generate reports. This layer composes existing Kaboom features:

- **HAR export:** Call `generate({what: "har", save_to: "./artifacts/test-run.har"})` via MCP to produce a HAR file from all captured network traffic. Attach as a CI artifact.
- **SARIF export:** Call `generate({what: "sarif", save_to: "./artifacts/a11y-report.sarif"})` via MCP to produce a SARIF file. Upload to GitHub Code Scanning.
- **PR summary:** Call `generate({what: "pr_summary"})` via MCP to produce a markdown summary of the test run's telemetry. Post as a PR comment via `gh pr comment`.

This layer is future work and depends on having an MCP client available in the CI context (either the Kaboom CLI or a lightweight report command).

## Workflow: CI Pipeline Integration

### Minimal Setup (Playwright)

A developer adds Kaboom CI to their Playwright tests in two steps:

Step 1: Install packages.
```
npm install --save-dev @anthropic/kaboom-playwright
```

Step 2: Use the fixture in tests.
```javascript
// tests/example.spec.js
const { test } = require('@anthropic/kaboom-playwright')

test('login flow', async ({ page, kaboom }) => {
  await page.goto('http://localhost:3000/login')
  await page.fill('[name=email]', 'test@example.com')
  await page.fill('[name=password]', 'password123')
  await page.click('button[type=submit]')
  await page.waitForURL('/dashboard')
  // If this test fails, Kaboom snapshot is automatically attached
})
```

### GitHub Actions Setup

```yaml
# .github/workflows/test.yml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
      - run: npm ci
      - run: npx playwright install --with-deps

      # Start Kaboom server as background process
      - run: npx kaboom-mcp &
      - run: sleep 2 # Wait for server to start

      # Run tests (Kaboom fixture handles injection/capture)
      - run: npx playwright test

      # Upload Playwright report (includes Kaboom snapshots on failure)
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: playwright-report
          path: playwright-report/
```

### What Happens on Test Failure

1. Test starts. Fixture injects capture script, marks test boundary.
2. Page loads. Console errors, network failures, WebSocket events are captured and batched to the server.
3. Test assertion fails (or page throws exception).
4. Fixture teardown fires. Fixture calls `GET /snapshot` to retrieve all captured state.
5. Fixture attaches two artifacts to the Playwright report:
   - `kaboom-snapshot` (JSON): Full telemetry -- every log, network body, and WebSocket event captured during the test.
   - `kaboom-summary` (text): Human-readable summary showing error count, warning count, network failures, and the first few error messages and failed requests.
6. Fixture calls `POST /test-boundary` (end) and `POST /clear`.
7. Developer opens the Playwright HTML report, clicks on the failed test, and sees the Kaboom context alongside the Playwright trace.

## Assumptions

- A1: The Kaboom server is started before tests run and remains available throughout the test suite. The fixture does not auto-start the server.
- A2: The test framework supports `addInitScript` or equivalent for injecting JavaScript before page load.
- A3: The browser under test has access to `127.0.0.1:7890` (or configured port). Network isolation in CI containers may need adjustment.
- A4: Tests run sequentially within a single worker. Parallel workers sharing a single server instance will have interleaved data (see OI-1).
- A5: The capture script's wire format matches what the Kaboom server's existing endpoints expect. Format drift between `inject.js` and `kaboom-ci.js` is a maintenance risk (see OI-2).

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Multi-worker test isolation | open | The `/test-boundary` endpoint sets a single `currentTestID` on the server. Parallel workers overwrite each other. Options: (a) require one Kaboom server per worker (different ports), (b) require client-side `test_id` tagging on every POST payload, (c) accept interleaved data and filter client-side. Option (b) was recommended by the principal engineer review but requires changes to all POST endpoints. |
| OI-2 | Capture script parity with inject.js | open | `kaboom-ci.js` is a separate codebase from `inject.js`. Functions like `safeSerialize`, `filterHeaders`, and console/fetch/WS interception are duplicated. The principal engineer review flagged this as a critical divergence risk. Options: (a) use a build tool (esbuild) to bundle inject.js with a CI transport shim, (b) accept duplication with a shared test suite verifying parity, (c) maintain manually with lint rules. |
| OI-3 | Fail-fast vs silent degradation | open | Should the Playwright fixture fail tests if the Kaboom server is unreachable? Current behavior: silent degradation (tests run, no observability). Alternative: health check on setup, fail if server is down. Could be a `kaboomRequired` option (default false). |
| OI-4 | Snapshot pagination for large test suites | open | The principal engineer review flagged that `/snapshot` returns all data in one response, which can timeout on large captures. Pagination via `offset`/`limit` parameters would solve this (R17), but the fixture currently expects a single response. |
| OI-5 | GitHub Action packaging | open | Should Kaboom provide a first-party GitHub Action (`uses: kaboom/ci-action@v1`) that wraps server startup + report generation + artifact upload? Or is the two-line `npx` approach sufficient? The Action adds convenience but also a maintenance surface. |
| OI-6 | Capture script Web Vitals support | open | The Chrome extension captures LCP, CLS, INP via PerformanceObserver. The CI capture script does not. Adding this would make CI and local capture more symmetric but increases script complexity. |
| OI-7 | Version negotiation | open | The capture script sends a `captureVersion` field in its initialization log entry, but the server does not validate it. If the wire format changes, old scripts silently produce incompatible data. Should the server reject unsupported versions? |
| OI-8 | Reporting CLI command | open | After tests run, how does the developer generate HAR/SARIF reports from CI? The MCP tools require an MCP client. Options: (a) add a `kaboom report` CLI subcommand that calls the server's HTTP API directly, (b) use an MCP client in the GitHub Action, (c) expose report generation as additional HTTP endpoints. |
