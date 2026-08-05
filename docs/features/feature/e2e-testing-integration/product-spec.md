---
feature: e2e-testing-integration
status: proposed
version: null
tool: generate
mode: playwright_fixture
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-e2e-testing-integration
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# E2E Testing Integration (CI/CD)

> Export captured Kaboom state as Playwright test fixtures, enabling AI agents and developers to generate production-ready test artifacts from live browser telemetry.

## Problem

Kaboom already captures rich browser telemetry -- console logs, network request/response bodies, WebSocket events, user actions, and DOM state. The existing `generate({what: "reproduction"})` and `generate({what: "test"})` modes produce Playwright scripts from this data, including basic fixture generation via the `generate_fixtures` option. However, there is a significant gap between these generated scripts and what teams need for CI/CD integration:

1. **No standalone fixture export.** The current `generate_fixtures` option in `reproduction.go` embeds API response fixtures inline with the test script. There is no way to export just the fixture data in a structured format that can be committed to a test suite, versioned independently, and reused across multiple tests.

2. **No CI-aware artifact packaging.** Generated scripts lack the metadata, configuration, and project scaffolding that CI pipelines expect: `playwright.config.ts` snippets, GitHub Actions workflow fragments, environment variable references, and artifact upload configuration.

3. **No bridge between local capture and CI execution.** A developer captures telemetry locally with the Kaboom extension, generates a test, then must manually adapt it to work in CI where the extension is not available. The Kaboom CI Infrastructure spec (v6) solves the runtime capture side, but there is no tool to export locally-captured state as fixtures that CI tests can consume.

4. **No failure context export.** When a test fails in CI, the failure context (what the browser was doing at the time) is lost. Teams want to export Kaboom snapshots as fixture files that can be attached to test reports, shared in PRs, and used for debugging without re-running the test.

## Solution

Add a new `playwright_fixture` mode to the existing `generate` tool that exports captured Kaboom state as structured Playwright-compatible artifacts. This mode transforms in-memory telemetry into files and configuration fragments that integrate directly into a team's test infrastructure.

The mode produces four artifact types, selectable via the `artifact` parameter:

- **`fixture_data`** -- API response fixtures extracted from captured network bodies, formatted for use with `page.route()`. This extends the existing `generateFixtures()` function in `reproduction.go` with richer metadata (HTTP method, status, headers, content type).

- **`test_harness`** -- A complete Playwright test file that uses the Kaboom CI fixture (`@anthropic/kaboom-playwright`) for runtime capture, with pre-configured route handlers for the exported fixtures. Builds on the existing `generateTestScript()` and `generateEnhancedPlaywrightScript()` infrastructure.

- **`ci_config`** -- CI pipeline configuration fragments (GitHub Actions YAML, GitLab CI YAML) that start the Kaboom server, run the generated tests, and upload artifacts.

- **`failure_snapshot`** -- Exports the current server snapshot (logs, network, WebSocket, actions) as a self-contained JSON fixture file suitable for attaching to test reports or committing as test reference data.

This mode does NOT implement any CI runtime infrastructure (that is the Kaboom CI spec's scope). It generates static artifacts that CI pipelines consume.

## User Stories

- As an AI coding agent, I want to export captured API responses as Playwright fixtures so that I can generate tests with pre-configured mock data that runs in CI without a live backend.
- As a developer using Kaboom, I want to generate a complete Playwright test file that includes both the user action replay and the Kaboom CI fixture integration so that I do not have to manually wire them together.
- As an AI coding agent, I want to generate a GitHub Actions workflow snippet for running Kaboom-instrumented tests so that I can add CI observability to a project in one step.
- As a developer using Kaboom, I want to export the current browser state snapshot as a fixture file so that I can attach it to a bug report or use it as test reference data.
- As an AI coding agent, I want to generate test fixtures from the captured network traffic of a specific user flow so that the generated test is self-contained and does not depend on a running API server.

## MCP Interface

**Tool:** `generate`
**Mode:** `playwright_fixture`

### Request

```json
{
  "tool": "generate",
  "arguments": {
    "type": "playwright_fixture",
    "artifact": "fixture_data",
    "options": {
      "include_headers": false,
      "filter_url": "/api/",
      "test_name": "user login flow",
      "base_url": "http://localhost:3000",
      "ci_provider": "github_actions"
    }
  }
}
```

#### Parameters:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | yes | Must be `"playwright_fixture"` |
| `artifact` | string | yes | One of: `fixture_data`, `test_harness`, `ci_config`, `failure_snapshot` |
| `options.include_headers` | bool | no | Include response headers in fixture data (default: false) |
| `options.filter_url` | string | no | Only include network entries matching this URL substring |
| `options.test_name` | string | no | Name for the generated test (used in `test_harness` and `fixture_data`) |
| `options.base_url` | string | no | Base URL for generated tests (replaces origins in captured URLs) |
| `options.ci_provider` | string | no | Target CI provider for `ci_config`: `github_actions` (default), `gitlab_ci` |
| `options.include_kaboom_fixture` | bool | no | Whether `test_harness` should import the Kaboom CI Playwright fixture (default: true) |
| `options.last_n` | int | no | Only use the last N captured actions (passed through to existing action filtering) |
| `options.since` | string (ISO 8601) | no | Only include telemetry captured after this timestamp |

### Response: `fixture_data`

```json
{
  "content": [
    {
      "type": "text",
      "text": "// fixtures/api-responses.json\n{\n  \"api/users\": {\n    \"method\": \"GET\",\n    \"status\": 200,\n    \"contentType\": \"application/json\",\n    \"body\": {\"users\": [{\"id\": 1, \"name\": \"Alice\"}]}\n  },\n  \"api/auth/login\": {\n    \"method\": \"POST\",\n    \"status\": 200,\n    \"contentType\": \"application/json\",\n    \"body\": {\"token\": \"[REDACTED]\", \"user\": {\"id\": 1}}\n  }\n}"
    },
    {
      "type": "text",
      "text": "// fixture-loader.js\n// Helper to apply fixtures as Playwright route handlers\nmodule.exports = function loadFixtures(page, fixtures) {\n  for (const [path, fixture] of Object.entries(fixtures)) {\n    page.route(`**/${path}`, route => {\n      route.fulfill({\n        status: fixture.status,\n        contentType: fixture.contentType,\n        body: JSON.stringify(fixture.body)\n      });\n    });\n  }\n};"
    }
  ]
}
```

### Response: `test_harness`

```json
{
  "content": [
    {
      "type": "text",
      "text": "import { test, expect } from '@anthropic/kaboom-playwright';\nconst fixtures = require('./fixtures/api-responses.json');\nconst loadFixtures = require('./fixtures/fixture-loader');\n\ntest('user login flow', async ({ page, kaboom }) => {\n  loadFixtures(page, fixtures);\n\n  await page.goto('http://localhost:3000/login');\n  await page.getByTestId('email').fill('test@example.com');\n  await page.getByTestId('password').fill('[user-provided]');\n  await page.getByRole('button', { name: 'Sign In' }).click();\n  await page.waitForURL('/dashboard');\n\n  // If this test fails, Kaboom snapshot is automatically attached\n});\n"
    }
  ]
}
```

### Response: `ci_config`

```json
{
  "content": [
    {
      "type": "text",
      "text": "# .github/workflows/e2e-tests.yml\nname: E2E Tests\non: [push, pull_request]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - uses: actions/setup-node@v4\n        with:\n          node-version: '20'\n      - run: npm ci\n      - run: npx playwright install --with-deps\n      - name: Start Kaboom server\n        run: npx kaboom-agentic-browser &\n      - name: Wait for server\n        run: sleep 2\n      - name: Run E2E tests\n        run: npx playwright test\n      - name: Upload test report\n        uses: actions/upload-artifact@v4\n        if: always()\n        with:\n          name: playwright-report\n          path: playwright-report/\n"
    }
  ]
}
```

### Response: `failure_snapshot`

```json
{
  "content": [
    {
      "type": "text",
      "text": "{\n  \"timestamp\": \"2026-01-28T10:05:30.123Z\",\n  \"test_id\": \"\",\n  \"logs\": [...],\n  \"websocket_events\": [...],\n  \"network_bodies\": [...],\n  \"enhanced_actions\": [...],\n  \"stats\": {\n    \"total_logs\": 5,\n    \"error_count\": 2,\n    \"warning_count\": 1,\n    \"network_failures\": 1,\n    \"ws_connections\": 0\n  }\n}"
    }
  ]
}
```

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | `generate({what: "playwright_fixture", artifact: "fixture_data"})` extracts JSON API response fixtures from captured network bodies, including method, status, content type, and parsed response body | must |
| R2 | Fixture data applies the same sensitive header stripping and redaction rules as the existing network body capture (Authorization, Cookie, tokens replaced with `[REDACTED]`) | must |
| R3 | Fixture data includes a companion `fixture-loader.js` helper that applies fixtures as `page.route()` handlers in a single function call | must |
| R4 | `generate({what: "playwright_fixture", artifact: "test_harness"})` produces a complete Playwright test file that imports from `@anthropic/kaboom-playwright` and uses the `kaboom` fixture | must |
| R5 | Test harness generation reuses the existing `getPlaywrightLocator()` selector priority (testId > role > ariaLabel > text > id > cssPath) from `codegen.go` | must |
| R6 | Test harness generation reuses the existing `generateEnhancedPlaywrightScript()` action replay logic from `reproduction.go`, adding Kaboom CI fixture integration | must |
| R7 | `generate({what: "playwright_fixture", artifact: "ci_config"})` produces a valid GitHub Actions workflow YAML for running Kaboom-instrumented Playwright tests | must |
| R8 | `generate({what: "playwright_fixture", artifact: "failure_snapshot"})` exports the current server state (logs, network bodies, WebSocket events, actions, stats) as a structured JSON fixture | must |
| R9 | Failure snapshot reuses the existing `SnapshotResponse` structure and `computeSnapshotStats()` from `ci.go` | must |
| R10 | The `filter_url` option filters network bodies by URL substring before generating fixtures (consistent with existing `NetworkBodyFilter` patterns) | must |
| R11 | The `since` option filters all telemetry by timestamp before generating any artifact (consistent with existing `filterLogsSince()` from `ci.go`) | should |
| R12 | The `base_url` option replaces origins in generated test URLs using the existing `replaceOrigin()` function from `codegen.go` | should |
| R13 | CI config generation supports GitLab CI YAML output when `ci_provider` is set to `gitlab_ci` | should |
| R14 | Test harness conditionally imports from `@playwright/test` instead of `@anthropic/kaboom-playwright` when `include_kaboom_fixture` is false, producing a standalone test without Kaboom CI dependency | should |
| R15 | Fixture data deduplicates API responses by URL+method, keeping the most recent response when multiple captures exist for the same endpoint | should |
| R16 | Generated fixture JSON is capped at 500KB to prevent oversized test artifacts; responses exceeding the cap are truncated with a `"[truncated]"` marker | should |
| R17 | Test harness includes inline comments explaining each generated step for readability | could |
| R18 | CI config includes optional SARIF upload step for security scanning integration when the user has SARIF generation enabled | could |

## Non-Goals

- **This feature does NOT implement CI runtime capture.** The Kaboom CI Infrastructure spec (v6) handles the capture script (`kaboom-ci.js`), CI server endpoints (`/snapshot`, `/clear`, `/test-boundary`), and the Playwright test fixture (`@anthropic/kaboom-playwright`). This spec only generates static artifacts that reference those components.

- **This feature does NOT run tests.** It generates test files, fixture data, and CI configuration. The AI agent or developer is responsible for saving the generated files and executing the test suite.

- **This feature does NOT create a 5th MCP tool.** It adds a new `playwright_fixture` mode to the existing `generate` tool, respecting the 5-tool model. The mode dispatches via the existing `type` parameter in `tools_core.go`.

- **This feature does NOT implement self-healing or test diagnosis.** The Self-Healing Tests spec covers `observe({what: "test_diagnosis"})` and `generate({what: "test_fix"})`. This spec covers fixture export and CI artifact generation -- distinct concerns that complement self-healing.

- **This feature does NOT persist fixtures to disk.** Like all Kaboom `generate` modes, output is returned as MCP response content. The AI agent or developer saves the generated content to the appropriate file paths. Kaboom does not write files to the user's project directory.

- **This feature does NOT support Cypress fixture generation.** Initial scope is Playwright only. Cypress uses a different fixture mechanism (`cy.intercept()`) that would require separate generation templates. Cypress support is tracked as a future enhancement.

- **Out of scope: fixture versioning or diffing.** Comparing current API responses against previously-exported fixtures to detect contract drift is part of the Agentic E2E Repair spec, not this one.

## Performance SLOs

| Metric | Target | Rationale |
|--------|--------|-----------|
| `fixture_data` generation | < 200ms for 100 network bodies | Iterates captured bodies, parses JSON, applies redaction -- all in-memory |
| `test_harness` generation | < 200ms | Reuses existing script generation pipeline |
| `ci_config` generation | < 50ms | Template string assembly, no data processing |
| `failure_snapshot` generation | < 200ms for full snapshot | Reuses existing `/snapshot` aggregation logic |
| Output size cap | < 500KB per artifact | Prevents oversized MCP responses; consistent with existing 50KB script cap extended for fixture data |

## Security Considerations

- **Sensitive data in fixtures.** Captured network response bodies may contain API keys, tokens, PII, or other secrets. Fixture generation applies the same redaction rules as `observe({what: "network_bodies"})`: Authorization, Cookie, Set-Cookie, and token headers are stripped. Response body redaction uses the patterns configured via `configure({what: "noise_rule"})`. The `fixture-loader.js` helper does NOT add additional redaction -- the data is cleaned at generation time.

- **CI configuration secrets.** Generated CI config YAML does not include any secrets, API keys, or environment-specific values. It references standard GitHub Actions / GitLab CI constructs only (`actions/checkout`, `actions/setup-node`, `npx` commands). No repository tokens, deployment keys, or credentials are embedded.

- **Fixture data scope.** The `filter_url` option allows narrowing fixture export to specific API paths (e.g., `/api/`). This prevents accidental export of telemetry from unrelated third-party requests. By default, all captured JSON network bodies are included -- the developer should review fixture content before committing to version control.

- **Localhost binding.** All data used by this mode comes from the existing Kaboom server's in-memory ring buffers. No new network endpoints, capture mechanisms, or data sources are introduced. The localhost-only binding constraint is unchanged.

## Edge Cases

- **No network bodies captured.** Expected behavior: `fixture_data` returns an empty fixtures object `{}` with a comment noting that no JSON API responses were found. `test_harness` generates a test without route handlers. Both are valid Playwright files.

- **No actions captured.** Expected behavior: `test_harness` returns a minimal test skeleton with a comment explaining that no user actions were available. The test compiles but contains no steps. `fixture_data` and `ci_config` are unaffected (they do not depend on actions).

- **Non-JSON response bodies.** Expected behavior: Fixture generation skips non-JSON responses (HTML pages, images, CSS, etc.) silently. Only responses with `Content-Type` containing `json` are included, consistent with the existing `generateFixtures()` function in `reproduction.go`.

- **Very large response bodies.** Expected behavior: Individual response bodies exceeding 10KB are truncated to 10KB with a `"[response truncated]"` comment. The total fixture output is capped at 500KB (R16). This prevents MCP response bloat.

- **Duplicate API endpoints.** Expected behavior: When multiple captures exist for the same URL+method combination, the most recent response is used (R15). If `filter_url` is set, deduplication applies only within the filtered set.

- **Malformed JSON response bodies.** Expected behavior: Bodies that fail `json.Unmarshal` are skipped, consistent with the existing `generateFixtures()` behavior in `reproduction.go`. A comment in the output notes the skipped entries.

- **Extension disconnected.** Expected behavior: `failure_snapshot` returns whatever is currently in the server buffers (which may be empty or stale). The snapshot includes a `stats` object so the consumer can judge data freshness. No error is returned -- partial data is better than no data.

- **Concurrent requests.** Expected behavior: Generation reads from ring buffers under `RLock` (consistent with all existing `generate` modes). Concurrent writes do not block generation; generation does not block writes.

- **`base_url` with trailing slash.** Expected behavior: The existing `replaceOrigin()` function in `codegen.go` already handles trailing slash normalization via `strings.TrimRight(baseURL, "/")`. No additional handling needed.

## Dependencies

- **Depends on:**
  - **`generate({what: "reproduction"})` (shipped)** -- Reuses `generateEnhancedPlaywrightScript()`, `generateFixtures()`, `getPlaywrightLocator()`, `replaceOrigin()`, `escapeJSString()` from `codegen.go` and `reproduction.go`.
  - **`generate({what: "test"})` (shipped)** -- Reuses `generateTestScript()` and `TestGenerationOptions` from `codegen.go` for action-to-test conversion.
  - **CI endpoints (shipped)** -- Reuses `SnapshotResponse`, `computeSnapshotStats()`, `filterLogsSince()` from `ci.go` for failure snapshot generation.
  - **Network body capture (shipped)** -- Reads detached evidence from the canonical bounded body store via `NetworkBodies().Snapshot().Bodies`.
  - **Redaction (shipped)** -- Applies existing header stripping and body redaction from `redaction.go`.
  - **Kaboom CI Infrastructure (proposed, v6)** -- The `test_harness` artifact references `@anthropic/kaboom-playwright` which is defined in the CI Infrastructure spec. The test harness is functional without it when `include_kaboom_fixture` is set to false.

- **Depended on by:**
  - **Self-Healing Tests (proposed)** -- May use fixture export to generate updated mock data when API contracts change.
  - **Agentic E2E Repair (proposed)** -- May use fixture diffing to detect API contract drift.

## Implementation Notes

### Integration with existing `generate` tool dispatch

The new mode plugs into the existing switch statement in `tools_core.go` (line ~1309):

```go
case "playwright_fixture":
    return h.toolGeneratePlaywrightFixture(req, args)
```

### Code reuse strategy

The implementation should maximize reuse of existing functions:

| Existing function | File | Reuse in `playwright_fixture` |
|---|---|---|
| `generateFixtures()` | `reproduction.go` | Core of `fixture_data` -- extend with method/status/contentType metadata |
| `generateEnhancedPlaywrightScript()` | `reproduction.go` | Core of `test_harness` -- wrap with Kaboom CI fixture import |
| `getPlaywrightLocator()` | `codegen.go` | Selector generation in `test_harness` |
| `replaceOrigin()` | `codegen.go` | Base URL substitution across all artifacts |
| `escapeJSString()` | `codegen.go` | String escaping in generated JavaScript |
| `computeSnapshotStats()` | `ci.go` | Stats computation in `failure_snapshot` |
| `filterLogsSince()` | `ci.go` | Timestamp filtering for `since` option |
| `NetworkBodies().Snapshot().Bodies` | `capture` methods | Data source for `fixture_data` |
| `GetEnhancedActions()` | `capture` methods | Data source for `test_harness` |

### Estimated effort

~500 lines total:
- ~150 lines: `fixture_data` artifact generation (extend `generateFixtures()` + fixture-loader template)
- ~100 lines: `test_harness` artifact generation (compose existing script gen + Kaboom CI imports)
- ~80 lines: `ci_config` artifact generation (YAML templates for GitHub Actions + GitLab CI)
- ~50 lines: `failure_snapshot` artifact generation (wrap existing snapshot aggregation)
- ~50 lines: parameter parsing, validation, dispatch in tool handler
- ~70 lines: tests

## Assumptions

- A1: The Kaboom extension is connected and has captured at least some network traffic or user actions before this tool is called. Without captured data, the tool returns valid but empty artifacts.
- A2: The `@anthropic/kaboom-playwright` npm package exists (or will exist per the Kaboom CI spec) when the developer runs the generated `test_harness`. The generated code imports it by name. If the package is not installed, the Playwright test will fail with a module-not-found error at runtime, not at generation time.
- A3: The developer's project uses Playwright as the E2E test framework. Generated test files use Playwright syntax exclusively. Other frameworks (Cypress, Puppeteer) are out of scope.
- A4: Captured network bodies contain JSON responses. Non-JSON responses are silently excluded from fixture data. Binary responses (images, fonts, etc.) are never included.
- A5: The `generate` tool's existing dispatch pattern (switch on `type`/`format` parameter) continues to be the extension point for new generation modes.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Fixture format: flat JSON vs structured per-endpoint | open | Current `generateFixtures()` uses API path as key with response body as value. The enriched format adds method/status/contentType. Should each endpoint be a top-level key, or should fixtures be an array of `{url, method, status, body}` objects? Array is more expressive (supports multiple methods per path) but harder to look up. Propose: object keyed by `"METHOD path"` (e.g., `"GET api/users"`). |
| OI-2 | Fixture-loader as separate file vs inline in test | open | The fixture-loader helper could be (a) a separate `.js` file returned as a second content block, (b) inlined in the test harness, or (c) published as part of `@anthropic/kaboom-playwright`. Option (a) is currently specified. Option (c) would reduce generated code but couples fixture loading to the Kaboom CI package. |
| OI-3 | CI config: minimal vs comprehensive | open | Should `ci_config` generate a minimal workflow (just server + tests + report upload) or a comprehensive one (matrix testing, caching, SARIF upload, Slack notification)? Propose: minimal by default with an `options.comprehensive` flag for the full version. |
| OI-4 | Fixture data request body capture | open | Current fixture export only captures response bodies. Should request bodies also be exported for POST/PUT/PATCH endpoints? This would enable generating tests that assert the correct request payload is sent. Propose: add `options.include_request_bodies` (default false) for v2. |
| OI-5 | Relationship to `generate({what: "test"})` | open | There is overlap between `test_harness` and the existing `generate({what: "test"})` mode. The key difference is that `test_harness` adds Kaboom CI fixture integration and uses exported fixtures. Should `test` mode gain an option to include Kaboom CI fixtures, or should they remain separate modes with distinct purposes (standalone test vs CI-integrated test)? Propose: keep separate for clarity -- `test` is a quick standalone script, `playwright_fixture` + `test_harness` is a CI-ready package. |
