---
title: Replace Selenium and Playwright with Natural Language
description: Stop writing brittle test code. Write what you want tested in plain English and let AI do the clicking — no engineers, no special tools, no selectors to maintain.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'replace', 'selenium']
---

## The Script Graveyard

Every team has one. A folder full of Selenium or Playwright scripts that nobody touches because:

- They break every time the UI changes
- They require an engineer to write and maintain
- They need a local dev environment, Node.js, Python, package managers
- They use selectors like `#root > div:nth-child(3) > form > button.submit-btn` that nobody can read
- When they fail, the error message tells you _what_ broke, not _why_

Product managers know what needs testing. They wrote the acceptance criteria. But there's a wall between "user can complete checkout as a guest" and a 200-line Playwright test that implements it.

KaBOOM removes that wall.

## What Replaces the Code

A text file. That's it.

Instead of this:

```js
// 47 lines of Playwright that break next sprint
const { test, expect } = require('@playwright/test');

test('guest checkout', async ({ page }) => {
  await page.goto('https://shop.example.com');
  await page.click('[data-testid="search-input"]');
  await page.fill('[data-testid="search-input"]', 'wireless headphones');
  await page.press('[data-testid="search-input"]', 'Enter');
  await page.click('.product-card:first-child .product-link');
  await page.click('#add-to-cart-button');
  await page.click('.cart-icon-wrapper > button');
  await expect(page.locator('.cart-count')).toHaveText('1');
  await page.click('[data-testid="checkout-btn"]');
  await page.fill('#shipping-name', 'Jane Doe');
  await page.fill('#shipping-address', '123 Main St');
  // ... 30 more lines
});
```

You write this:

```text
Test: Guest Checkout

1. Go to the shop homepage
2. Search for "wireless headphones"
3. Click the first product in the results
4. Click "Add to Cart"
5. Open the cart
6. Verify the cart shows 1 item
7. Click "Checkout"
8. Fill in the shipping form: Jane Doe, 123 Main St, Portland, OR, 97201
9. Click "Continue to Payment"
10. Verify no errors on the page
11. Verify the URL contains "/checkout/payment"
```

Hand that to your AI (Claude Code, Cursor, Windsurf — any MCP-compatible tool) with KaBOOM connected. The AI reads it, drives the browser, and reports the results.

## Why Product Managers Can Do This

**No dev environment.** You need a browser with the KaBOOM extension and an AI tool. No `npm install`, no `pip install`, no `package.json`.

**No selectors.** You write "Click the Add to Cart button." The AI figures out the selector. KaBOOM supports semantic selectors — `text=Add to Cart`, `label=Email`, `aria-label=Close` — that target what elements _mean_. The AI picks the right one.

**No maintenance.** When the design team renames a button from "Checkout" to "Proceed to Checkout," your test still works — the AI adapts. If the text changes so much that the AI can't find the element, it tells you in plain English: "I couldn't find a button matching 'Checkout'. I see 'Proceed to Checkout' instead. Should I use that?"

**No debugging.** When a traditional test fails, you get `TimeoutError: locator.click: Timeout 30000ms exceeded`. When a natural language test fails, the AI tells you: "Step 7 failed — when I clicked Checkout, the page showed an error: 'Your cart is empty.' It looks like the Add to Cart action didn't persist after navigation."

## How to Run a Test

**Step 1:** Open your AI tool with KaBOOM configured.

**Step 2:** Paste your test script and ask the AI to run it:

```text
Run this acceptance test against our staging environment and report results:

Test: Guest Checkout
1. Go to https://staging.shop.example.com
...
```

**Step 3:** Watch the browser. The AI navigates, clicks, types, and verifies — you see every action happen live.

**Step 4:** Read the report. The AI tells you which steps passed, which failed, and why.

## How to Make Tests Repeatable

**Save your scripts in version control.** They're plain text files — they diff cleanly, they're reviewable in PRs, and they live next to the code.

**Use state checkpoints.** Before running a test, save the browser state:

```text
Before running: save the current browser state as "pre-test"
After the test: report results and restore "pre-test" state
```

The AI calls `save_state` and `load_state` behind the scenes.

**Add verification steps.** Don't just check that clicks work — verify outcomes:

```text
7. Click "Checkout"
8. Verify no console errors appeared
9. Verify the API call to /api/checkout returned 200
10. Verify the page shows the order confirmation number
```

The AI uses `observe({what: "errors"})` and `observe({what: "network_bodies"})` to verify these — deeper than any Selenium assertion.

## Selenium vs. KaBOOM: Side by Side

| | Selenium/Playwright | KaBOOM + Natural Language |
|---|---|---|
| **Who writes tests** | Engineers | Anyone who can describe the workflow |
| **Language** | JavaScript, Python, Java | English |
| **Maintenance** | Constant (selectors break) | Rare (semantic selectors adapt) |
| **Setup** | Node/Python + browser driver + CI config | Browser extension + AI tool |
| **Error messages** | `TimeoutError: selector not found` | "The submit button isn't visible because the form has a validation error on the email field" |
| **Network verification** | Requires intercept setup | `observe({what: "network_bodies"})` built in |
| **WebSocket testing** | Not supported | `observe({what: "websocket_events"})` built in |
| **Visual verification** | Screenshot comparison libraries | AI sees the screenshot and reasons about it |
| **Accessibility checks** | Separate library (axe) | `analyze({what: "accessibility"})` built in |

## Getting Started

1. Install KaBOOM ([Getting Started](/getting-started/))
2. Enable AI Web Pilot in the extension popup
3. Write your first test — start with something you already test manually
4. Paste it into your AI tool and say "run this test"
5. Watch it work

You've been writing acceptance criteria in Jira your whole career. Now those criteria _are_ the tests.
