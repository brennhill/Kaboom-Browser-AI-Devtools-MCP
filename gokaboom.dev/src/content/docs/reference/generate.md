---
title: "Generate — Create Artifacts"
description: "Complete reference for the generate tool. 13 formats for producing Playwright tests, reproduction scripts, HAR exports, SARIF reports, CSP headers, SRI hashes, PR summaries, visual tests, annotation reports, test healing, and failure classification."
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['reference', 'generate']
---

The `generate` tool creates production-ready artifacts from captured browser data. Tests, reproduction scripts, reports, and exports — all generated from real browser sessions.

Need one runnable call + response shape + failure fix for every mode? See [Generate Executable Examples](/reference/examples/generate-examples/).

## Quick Reference

```js
generate({what: "test"})                               // Playwright regression test
generate({what: "reproduction"})                        // Bug reproduction script
generate({what: "har", url: "/api"})                    // HTTP Archive export
generate({what: "sarif"})                               // Accessibility SARIF report
generate({what: "csp", mode: "strict"})                 // Content Security Policy
generate({what: "sri"})                                 // Subresource Integrity hashes
generate({what: "pr_summary"})                          // PR performance summary
generate({what: "visual_test", annot_session: "checkout"})   // Visual test from annotations
generate({what: "test_from_context", context: "error"}) // Test from error context
generate({what: "test_heal", action: "analyze", test_file: "tests/login.spec.ts"})  // Heal broken selectors
generate({what: "test_classify", action: "failure", failure: {error: "timeout"}})    // Classify failure
```

## Common Parameters

These parameters are used across multiple generate modes:

| Parameter | Type | Description |
|-----------|------|-------------|
| `what` | string (required) | Artifact type to generate |
| `telemetry_mode` | string | Telemetry metadata mode: `off`, `auto`, `full` |
| `save_to` | string | Output file path when writing artifacts to disk |
| `test_name` | string | Optional name for generated test artifacts |

---

## test — Playwright Regression Tests

Generates a complete Playwright test from the current browser session. Captures user actions, correlates them with API calls, and produces tests with real assertions — not just click replay.

```js
generate({what:"test"})

generate({what:"test",
          test_name: "guest-checkout",
          base_url: "http://localhost:3000",
          assert_network: true,
          assert_no_errors: true,
          assert_response_shape: true})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `test_name` | string | Derived from URL | Name for the `test()` block |
| `base_url` | string | Captured origin | Replace origin in URLs for portability |
| `assert_network` | boolean | — | Include `waitForResponse` + status code assertions |
| `assert_no_errors` | boolean | — | Assert `consoleErrors.length === 0` |
| `assert_response_shape` | boolean | — | Assert response body structure matches (types only, never values) |

### What the output includes

- **User actions** translated to Playwright commands (`click`, `fill`, `getByRole`, etc.)
- **Multi-strategy selectors** prioritized: `data-testid` > ARIA role > label > text > ID > CSS
- **Network assertions** with `waitForResponse` and status code checks
- **Response shape validation** — field names and types, never actual values
- **Console error collection** — asserts zero errors during the flow
- **Password redaction** — passwords replaced with `[user-provided]`

### Example output

```js
import { test, expect } from '@playwright/test';

test('submit-form flow', async ({ page }) => {
  const consoleErrors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });

  await page.goto('http://localhost:3000/login');
  await page.getByLabel('Email').fill('user@example.com');
  await page.getByLabel('Password').fill('[user-provided]');

  const loginResp = page.waitForResponse(r => r.url().includes('/api/auth/login'));
  await page.getByRole('button', { name: 'Sign In' }).click();
  const resp = await loginResp;
  expect(resp.status()).toBe(200);

  expect(consoleErrors).toHaveLength(0);
});
```

<!-- Screenshot: Generated Playwright test output in a terminal or editor -->

---

## reproduction — Bug Reproduction Scripts

Generates a Playwright script that reproduces the user's actions leading up to a bug. Unlike `test`, this focuses on replaying the exact sequence rather than asserting outcomes.

```js
generate({what:"reproduction"})

generate({what:"reproduction",
          error_message: "TypeError: Cannot read property 'id' of undefined",
          last_n: 10,
          base_url: "http://localhost:3000",
          include_screenshots: true})
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `error_message` | string | — | Error message to include as context in the script |
| `last_n` | number | All | Use only the last N recorded actions |
| `base_url` | string | Captured origin | Replace origin in URLs |
| `include_screenshots` | boolean | — | Insert `page.screenshot()` calls between steps |
| `generate_fixtures` | boolean | — | Generate fixture files from captured network data |
| `visual_assertions` | boolean | — | Add `toHaveScreenshot()` assertions |

---

## har — HTTP Archive Export

Exports captured network traffic in HAR 1.2 format. HAR files can be imported into Chrome DevTools, Charles Proxy, or any HAR viewer for analysis.

```js
generate({what:"har"})

generate({what:"har",
          url: "/api",
          method: "POST",
          status_min: 400,
          save_to: "/tmp/debug.har"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `url` | string | Filter by URL substring |
| `method` | string | Filter by HTTP method |
| `status_min` | number | Minimum status code |
| `status_max` | number | Maximum status code |
| `save_to` | string | File path to save the HAR file |

---

## sarif — Accessibility SARIF Report

Exports accessibility audit results in SARIF format (Static Analysis Results Interchange Format). SARIF files integrate with GitHub Code Scanning, VS Code, and CI/CD pipelines.

```js
generate({what:"sarif"})

generate({what:"sarif",
          scope: "#main-content",
          include_passes: true,
          save_to: "/tmp/a11y.sarif"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `scope` | string | CSS selector to limit audit scope |
| `include_passes` | boolean | Include passing rules (not just violations) |
| `save_to` | string | File path to save the SARIF file |

---

## csp — Content Security Policy Generation

Generates a Content Security Policy header from observed network traffic. KaBOOM sees which origins your page loads resources from and produces a CSP that allows exactly those origins.

```js
generate({what:"csp"})

generate({what:"csp",
          mode: "strict",
          exclude_origins: ["https://analytics.google.com"],
          include_report_uri: true})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `mode` | string | Strictness: `strict`, `moderate`, or `report_only` |
| `exclude_origins` | array | Origins to exclude from the generated CSP |
| `include_report_uri` | boolean | Include a `report-uri` directive |

---

## sri — Subresource Integrity Hashes

Generates SRI hashes for external scripts and stylesheets. SRI ensures that fetched resources haven't been tampered with.

```js
generate({what:"sri"})

generate({what:"sri",
          resource_types: ["script"],
          origins: ["https://cdn.example.com"]})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `resource_types` | array | Filter: `script`, `stylesheet` |
| `origins` | array | Filter by specific CDN origins |

---

## pr_summary — PR Performance Summary

Generates a performance impact summary suitable for pull request descriptions. Compares before/after metrics and highlights regressions or improvements.

```js
generate({what:"pr_summary"})
```

No additional parameters. Uses the current performance snapshot data.

Output includes:
- Web Vitals comparison (before/after)
- Regression/improvement verdicts
- Network request count changes
- Bundle size impact (if measurable)

---

## visual_test — Visual Regression Test from Annotations

Generates a Playwright visual regression test from a draw mode annotation session. Each annotation becomes a visual assertion.

```js
generate({what:"visual_test", annot_session: "checkout-flow"})

generate({what:"visual_test",
          session: "checkout-flow",
          test_name: "checkout-visual-regression"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `annot_session` | string | Named annotation session from draw mode |
| `test_name` | string | Name for the generated test |

---

## annotation_report — Annotation Report

Generates a report from draw mode annotations — summarizes all user feedback from an annotation session.

```js
generate({what:"annotation_report", annot_session: "homepage-review"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `annot_session` | string | Named annotation session |

---

## annotation_issues — Extract Issues from Annotations

Extracts structured issues from draw mode annotations, suitable for issue tracker integration.

```js
generate({what:"annotation_issues", annot_session: "homepage-review"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `annot_session` | string | Named annotation session |

---

## test_from_context — Context-Aware Test Generation

Generates a Playwright test from a specific error, interaction, or regression context. More targeted than `test` — focuses on reproducing a specific scenario.

```js
generate({what:"test_from_context", context: "error"})
generate({what:"test_from_context", context: "interaction"})
generate({what:"test_from_context", context: "regression", include_mocks: true})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `context` | string | Test context: `error`, `interaction`, or `regression` |
| `error_id` | string | Specific error ID (for `error` context) |
| `include_mocks` | boolean | Include network mocks in the generated test |
| `output_format` | string | Output format: `file` or `inline` |

---

## test_heal — Repair Broken Selectors

Self-healing for Playwright tests — analyzes test files for broken selectors and suggests or auto-applies fixes using the current DOM state.

### Analyze a test file

```js
generate({what:"test_heal", action: "analyze", test_file: "tests/login.spec.ts"})
```

### Repair broken selectors

```js
generate({what:"test_heal", action: "repair",
          broken_selectors: ["#old-submit-btn", ".deprecated-class"],
          auto_apply: true})
```

### Batch heal a directory

```js
generate({what:"test_heal", action: "batch", test_dir: "tests/"})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `action` | string | `analyze` (find broken selectors), `repair` (fix them), `batch` (process directory) |
| `test_file` | string | Test file path (for `analyze`) |
| `test_dir` | string | Test directory (for `batch`) |
| `broken_selectors` | array | Selectors to repair (for `repair`) |
| `auto_apply` | boolean | Auto-apply high-confidence fixes (for `repair`) |

---

## test_classify — Classify Test Failures

Classifies test failures by root cause — infrastructure, flaky, regression, environment, etc. Useful for batch triage of CI failures.

### Classify a single failure

```js
generate({what:"test_classify", action: "failure",
          failure: {test_name: "login-flow", error: "Timeout 30000ms exceeded", duration_ms: 30500}})
```

### Batch classify

```js
generate({what:"test_classify", action: "batch",
          failures: [
            {error: "Timeout 30000ms exceeded", test_name: "login"},
            {error: "Element not found: #submit", test_name: "checkout"}
          ]})
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `action` | string | `failure` (single) or `batch` (multiple) |
| `failure` | object | Single test failure: `{error, test_name?, screenshot?, trace?, duration_ms?}` |
| `failures` | array | Array of failure objects (for `batch`) |
