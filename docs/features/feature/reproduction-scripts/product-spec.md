---
feature: Reproduction Scripts
status: proposed
tool: generate
format: reproduction
version: 0.7.12
doc_type: product-spec
feature_id: feature-reproduction-scripts
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Reproduction Scripts

## Problem Statement

When a user discovers a bug or wants to demonstrate a flow, they need a way to **export what just happened** as a replayable script. Today the `generate({what: "reproduction"})` tool is a stub that returns an empty string.

Users need two very different export formats:

1. **Playwright script** — for CI, automated testing, and developers who want deterministic replay in a test runner.
2. **Natural language script** — for demos, bug reports, and AI-powered replay via the Kaboom Pilot.

The Playwright format already exists in the TypeScript extension (`reproduction.ts`) but is **not available on the server side** where the MCP generate tool runs. The natural language format does not exist anywhere.

---

## Solution

Implement `generate({what: "reproduction"})` to produce scripts from captured `EnhancedAction` data in two output formats:

- **`playwright`** — Playwright test code using multi-strategy selectors (testId > role > ariaLabel > text > id > cssPath)
- **`kaboom`** — Human-readable natural language steps that describe what to do, usable as instructions for the AI Pilot or as documentation

### Kaboom Format Example

```
# Reproduction: User checkout flow
# Captured: 2026-02-07T10:30:00Z | 8 actions | https://shop.example.com

1. Navigate to: https://shop.example.com/products
2. Click: "Add to Cart" button
3. Click: "Cart" link in the navigation
4. Type "2" into: Quantity field (input#quantity)
5. Select "Express" from: Shipping Method dropdown
6. [2.3s pause]
7. Type "SUMMER20" into: Coupon Code field
8. Click: "Checkout" button
```

### Playwright Format Example

```typescript
import { test, expect } from '@playwright/test';

test('reproduction: captured user actions', async ({ page }) => {
  await page.goto('https://shop.example.com/products');
  await page.getByRole('button', { name: 'Add to Cart' }).click();
  await page.getByRole('link', { name: 'Cart' }).click();
  await page.locator('#quantity').fill('2');
  await page.locator('#shipping-method').selectOption('Express');
  // [2s pause]
  await page.getByTestId('coupon-input').fill('SUMMER20');
  await page.getByRole('button', { name: 'Checkout' }).click();
});
```

---

## User Workflows

### Workflow 1: Bug Report (Natural Language)

```
1. Developer reproduces a bug in the browser
2. LLM calls: generate({what: "reproduction", output_format: "kaboom"})
3. Gets human-readable steps
4. Pastes into bug report / Slack / PR description
5. Anyone can read and manually follow the steps
```

### Workflow 2: AI Replay (Natural Language)

```
1. User records a flow (captured as EnhancedActions)
2. LLM calls: generate({what: "reproduction", output_format: "kaboom"})
3. Gets natural language steps
4. LLM feeds steps to the Pilot via interact() tool
5. Pilot re-executes the flow using semantic selectors
```

### Workflow 3: Automated Test (Playwright)

```
1. QA reproduces a flow in the browser
2. LLM calls: generate({what: "reproduction", output_format: "playwright"})
3. Gets Playwright test code
4. Saves to test suite, runs in CI
```

### Workflow 4: Demo Recording

```
1. User performs a demo flow (1000 action buffer captures it)
2. LLM calls: generate({what: "reproduction", output_format: "kaboom"})
3. Gets natural language steps for the demo
4. Steps can be replayed later via Pilot for live demos
```

---

## MCP API

### Request

```javascript
generate({
  format: "reproduction",
  output_format: "kaboom",  // "kaboom" (default) | "playwright"
  last_n: 20,                 // Optional: only last N actions
  base_url: "http://localhost:3000",  // Optional: rewrite URLs
  include_screenshots: false,  // Optional: insert screenshot steps (playwright only)
  error_message: "Cannot read property 'x' of undefined"  // Optional: context
})
```

### Response

```json
{
  "script": "# Reproduction: captured user actions\n...",
  "format": "kaboom",
  "action_count": 8,
  "duration_ms": 45200,
  "start_url": "https://shop.example.com/products",
  "metadata": {
    "generated_at": "2026-02-07T10:35:00Z",
    "selectors_used": ["testId", "role", "text", "cssPath"],
    "actions_available": 42,
    "actions_included": 8
  }
}
```

---

## Kaboom Script Format Specification

The Kaboom format is a numbered list of human-readable steps. Each step maps to one `EnhancedAction`.

### Action Type Mapping

| Action Type | Kaboom Format | Example |
|-------------|----------------|---------|
| `navigate` | `Navigate to: {url}` | `Navigate to: https://example.com/login` |
| `click` | `Click: {element description}` | `Click: "Submit" button` |
| `input` | `Type "{value}" into: {element description}` | `Type "alice@example.com" into: Email field` |
| `select` | `Select "{text}" from: {element description}` | `Select "Express" from: Shipping dropdown` |
| `keypress` | `Press: {key}` | `Press: Enter` |
| `scroll` | `Scroll to: y={position}` | `Scroll to: y=500` |

### Element Description Priority

The element description uses the most human-readable selector available, in priority order:

1. **text + role** — `"Submit" button`, `"Cart" link`
2. **ariaLabel + role** — `"Close dialog" button`
3. **testId** — `[data-testid="coupon-input"]`
4. **role + name** — `"Email" textbox`
5. **id** — `#quantity`
6. **text alone** — `"Add to Cart"`
7. **cssPath** — `form > div.input-group > input` (last resort)

### Timing

Pauses > 2 seconds between actions are shown as `[{N}s pause]` to communicate pacing.

### Header

Every script starts with a metadata header:
```
# Reproduction: {description}
# Captured: {timestamp} | {action_count} actions | {start_url}
```

---

## Requirements

### Functional
- [ ] `generate({what: "reproduction"})` returns a non-empty script
- [ ] Default `output_format` is `"kaboom"` (natural language)
- [ ] Playwright format uses multi-strategy selectors (testId > role > ariaLabel > text > id > cssPath)
- [ ] Kaboom format uses human-readable element descriptions
- [ ] `last_n` parameter filters to last N actions
- [ ] `base_url` parameter rewrites URLs in both formats
- [ ] `error_message` adds context annotation
- [ ] Pauses > 2s between actions are annotated
- [ ] Response includes metadata (action count, duration, selectors used)

### Non-Functional
- [ ] Generation completes in < 50ms for 1000 actions
- [ ] Output size capped at 200KB
- [ ] Zero new dependencies

---

## Out of Scope

- Screenshot insertion (deferred, already specced in repro-v2.md)
- Data fixture generation (deferred)
- Visual assertions (deferred)
- Replay execution (separate feature: Pilot reads the script and replays via interact())

---

## Success Criteria

- `generate({what: "reproduction"})` returns useful, human-readable output
- Kaboom format can be copy-pasted into a bug report and understood by anyone
- Kaboom format can be fed to Claude + Pilot for AI replay
- Playwright format produces valid, runnable Playwright tests
- Both formats use the best available selector for each action
