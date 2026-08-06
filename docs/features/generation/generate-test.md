---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Generate Test: Turn Browser Sessions into Regression Tests

## Your AI assistant watched you use the app. Now it writes the test.

---

## The Problem

You fixed a bug. You manually verified it works. But without a regression test, it'll break again next sprint. Writing that test is tedious — you have to remember what you clicked, what API calls fired, and what the responses looked like.

Meanwhile, Kaboom already captured all of it.

## The Solution

`generate_test` converts your real browser session into a Playwright test with *actual assertions* — not just a replay script. It knows what API responses looked like, what status codes came back, and whether console errors occurred.

```
You click around your app fixing a bug
         ↓
Kaboom captures actions + network + console
         ↓
AI calls generate_test
         ↓
Playwright test with assertions, ready for CI
```

## What Makes This Different

Most record-and-replay tools produce fragile scripts that just click buttons. `generate_test` produces a **regression test**:

| Replay Script | Kaboom `generate_test` |
|---------------|--------------------------|
| Clicks buttons | Clicks buttons |
| No network assertions | Asserts API status codes |
| No response validation | Validates response structure |
| No error checking | Asserts zero console errors |
| Hardcoded URLs | Configurable base URL |
| Fragile CSS selectors | Multi-strategy selectors (data-testid > aria > role > CSS) |

## Example Output

You log in, submit a form, and see the result. `generate_test` produces:

```javascript
import { test, expect } from '@playwright/test';

test('submit-form flow', async ({ page }) => {
  // Collect console errors
  const consoleErrors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });

  await page.goto('http://localhost:3000/login');

  // Login
  await page.getByLabel('Email').fill('user@example.com');
  await page.getByLabel('Password').fill('[user-provided]');

  const resp1Promise = page.waitForResponse(r => r.url().includes('/api/auth/login'));
  await page.getByRole('button', { name: 'Sign In' }).click();
  const resp1 = await resp1Promise;
  expect(resp1.status()).toBe(200);
  const resp1Body = await resp1.json();
  expect(resp1Body).toHaveProperty('token');
  expect(resp1Body).toHaveProperty('user');

  // Navigate to form
  await expect(page).toHaveURL(/\/dashboard/);

  // Submit
  await page.getByTestId('name-input').fill('Test Project');
  const resp2Promise = page.waitForResponse(r => r.url().includes('/api/projects'));
  await page.getByRole('button', { name: 'Create' }).click();
  const resp2 = await resp2Promise;
  expect(resp2.status()).toBe(201);

  // Assert: no console errors during flow
  expect(consoleErrors).toHaveLength(0);
});
```

Passwords are automatically redacted. Selectors prefer `data-testid` and ARIA roles over brittle CSS paths.

## How It Works

### 1. Session Timeline

Kaboom correlates three data sources into a unified timeline:

- **User actions** — clicks, inputs, keypresses, navigation, scrolls
- **Network requests** — URL, method, status, response body, timing
- **Console events** — errors and warnings with messages

Each network request is attributed to the preceding user action that triggered it.

### 2. Response Shape Extraction

For JSON API responses, Kaboom extracts the *structural type signature* — field names and types, never values:

```
Response: {"id": 42, "name": "Project", "tasks": [{"title": "Todo"}]}
   ↓
Shape: {"id": "number", "name": "string", "tasks": [{"title": "string"}]}
```

This means your test asserts the API contract, not specific data that changes between runs.

### 3. Assertion Generation

| Signal | Assertion |
|--------|-----------|
| Click triggers API call | `waitForResponse` + `expect(status).toBe(...)` |
| Navigation occurs | `expect(page).toHaveURL(/pattern/)` |
| JSON response received | `expect(body).toHaveProperty('field')` |
| No errors in session | `expect(consoleErrors).toHaveLength(0)` |
| Errors present in session | Commented-out assertion + listed known errors |

### 4. Selector Priority

Kaboom computes multiple selector strategies for every interaction and picks the most resilient:

1. `data-testid` — `getByTestId('submit-btn')`
2. ARIA role + name — `getByRole('button', { name: 'Submit' })`
3. ARIA label — `getByLabel('Email')`
4. Visible text — `getByText('Sign In')`
5. Element ID — `locator('#submit')`
6. CSS path — `locator('form > button.primary')` (last resort)

## MCP Tool Reference

### `get_session_timeline`

Get the correlated timeline without generating a test. Useful for understanding cause-and-effect:

```json
{
  "last_n_actions": 5,
  "url": "checkout",
  "include": ["actions", "network"]
}
```

Returns a sorted timeline with summary stats (action count, network requests, console errors, duration).

### `generate_test`

Generate the full Playwright test:

```json
{
  "test_name": "checkout completes successfully",
  "base_url": "http://localhost:3000",
  "assert_network": true,
  "assert_no_errors": true,
  "assert_response_shape": true,
  "last_n_actions": 10
}
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `test_name` | Derived from URL | Name for the `test()` block |
| `base_url` | Captured origin | Replace origin for portability |
| `assert_network` | `true` | Assert API status codes |
| `assert_no_errors` | `true` | Assert zero console errors |
| `assert_response_shape` | `false` | Assert response body structure |
| `last_n_actions` | All | Scope to recent N actions |

## Use Cases

### After Fixing a Bug

> "I just fixed the login timeout issue. Generate a regression test from my session."

Your AI calls `generate_test` and produces a test that verifies login works — with the actual API assertions that would catch a regression.

### Documenting a Flow

> "Generate a test for the checkout flow I just walked through."

Get a complete Playwright test covering navigation, form fills, API calls, and error checks — no manual test authoring.

### API Contract Testing

> "Generate a test with response shape assertions for the user profile flow."

With `assert_response_shape: true`, the test validates that API responses maintain their structure — catching breaking changes even when values differ.

### Portable Tests

> "Generate a test using <http://localhost:3000> as the base URL."

The `base_url` parameter rewrites captured URLs (e.g., from `https://staging.example.com`) to your local dev server.

## Privacy & Security

- Passwords are redacted to `[user-provided]` before test generation
- Response shapes contain types only, never actual values
- All processing is local — no data leaves your machine
- Generated tests never contain auth tokens or secrets

## Getting Started

1. Install [Kaboom](https://github.com/brennhill/kaboom-agentic-browser-devtools-mcp) (extension + server)
2. Use your app normally — Kaboom captures actions in the background
3. Ask your AI assistant: *"Generate a regression test from my session"*
4. Save the output as a `.spec.ts` file and run with `npx playwright test`

No configuration needed. No test code to write. Just use your app and get tests.
