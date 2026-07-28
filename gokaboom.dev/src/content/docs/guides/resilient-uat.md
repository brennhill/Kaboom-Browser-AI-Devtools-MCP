---
title: Write UAT Scripts That Survive UI Changes
description: Create reusable acceptance tests using natural language and semantic selectors — scripts that keep working even when your UI gets redesigned.
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
normalized_tags: ['guides', 'resilient', 'uat']
---

## The Selenium Problem

You write a Playwright or Selenium test. It clicks `#app > div.main-content > div.form-wrapper > button.btn-primary`. The designer changes the class to `btn-accent`. The test breaks. No one fixes it. Now you have a test suite that's 40% red and everyone ignores it.

This happens because traditional test scripts are coupled to **structure**, not **intent**.

KaBOOM takes a different approach. You write your UAT in natural language. The AI uses semantic selectors — `text=Submit`, `label=Email`, `role=button` — that target what elements _mean_, not where they _sit_. The UI can be completely redesigned and the test still works.

## Step 1: Write the Test in Plain English

Start with what a human tester would do. No code:

```text
UAT: User Registration Flow

Precondition: User is on the homepage, not logged in.

1. Click "Sign Up"
2. Fill in "Full Name" with "Jane Doe"
3. Fill in "Email" with "jane@example.com"
4. Fill in "Password" with "SecurePass123!"
5. Check the "I agree to the Terms of Service" checkbox
6. Click "Create Account"
7. Verify the page shows "Welcome, Jane"
8. Verify the URL contains "/dashboard"
```

That's your test. Save it as a text file, a Notion doc, a comment in your issue tracker — wherever your team already works.

## Step 2: Let the AI Run It

Hand the script to your AI (Claude Code, Cursor, etc.) with KaBOOM connected. The AI translates each step into `interact` calls using semantic selectors:

```js
interact({what: "click", selector: "text=Sign Up"})

interact({what: "type", selector: "label=Full Name", text: "Jane Doe"})

interact({what: "type", selector: "label=Email", text: "jane@example.com"})

interact({what: "type", selector: "label=Password", text: "SecurePass123!"})

interact({what: "check", selector: "text=I agree to the Terms of Service"})

interact({what: "click", selector: "text=Create Account"})
```

For verification, the AI observes the page:

```js
observe({what: "page"})
// -> {url: "https://app.example.com/dashboard", title: "Dashboard - Welcome, Jane"}
```

Or uses `get_text` on specific elements:

```js
interact({what: "get_text", selector: "text=Welcome"})
// -> "Welcome, Jane"
```

## Why This Doesn't Break

**Semantic selectors target meaning, not structure.** Here's what survives:

| Change | Brittle selector breaks? | Semantic selector breaks? |
|---|---|---|
| Button class renamed | Yes | No — `text=Sign Up` still works |
| Form restructured into tabs | Yes | No — `label=Email` finds it anywhere |
| CSS framework swapped | Yes | No — `role=button` is framework-agnostic |
| Element ID removed | Yes | No — not using IDs |
| Component library upgraded | Yes | No — text and labels are stable |
| Button text changed to "Register" | No | Yes — but _intentionally_ (you update the one word) |

The only time a semantic selector breaks is when the _meaning_ changes — and that's when you _want_ the test to break, because the product behavior actually changed.

## Step 3: Make It Re-Runnable

**Save starting state** so you can reset between runs:

```js
interact({what: "save_state", snapshot_name: "logged-out-homepage"})
```

**Start each run from the checkpoint:**

```js
interact({what: "load_state", snapshot_name: "logged-out-homepage", include_url: true})
```

**Use `list_interactive` when the UI changes significantly.** If a page redesign moves things around, the AI can discover what's available:

```js
interact({what: "list_interactive"})
```

This returns every clickable, typeable, and selectable element on the page with suggested selectors. The AI adapts its approach based on what's actually there — not what was there last week.

## Step 4: Add Verification That Matters

Don't just check that clicks succeed. Verify the _outcomes_:

**Check for errors after key actions:**

```js
observe({what: "errors"})
```

If the error list is empty after "Create Account," the registration didn't throw. That's a stronger signal than checking if a success message appeared.

**Check network responses:**

```js
observe({what: "network_bodies", url: "/api/register", status_min: 200, status_max: 299})
```

Verify the API actually returned a success response, not just that the UI showed a green banner.

**Check performance hasn't regressed:**

```js
observe({what: "vitals"})
```

If LCP jumped from 1.2s to 4.8s after the last deploy, your UAT catches it — even though it's not a functional bug.

## Full Example: E-Commerce Checkout UAT

Here's a complete UAT script that a human tester could write and an AI could run:

```text
UAT: Guest Checkout Flow

1. Navigate to https://shop.example.com
2. Search for "wireless headphones"
3. Click the first product in results
4. Click "Add to Cart"
5. Click the cart icon
6. Verify cart shows 1 item
7. Click "Checkout"
8. Fill in shipping: name "Jane Doe", address "123 Main St",
   city "Portland", state "OR", zip "97201"
9. Click "Continue to Payment"
10. Verify no errors on the page
11. Verify the URL contains "/checkout/payment"
12. Check that the order total is greater than $0
```

No selectors. No waits. No framework assumptions. The AI handles all of that. When the design team replaces the cart icon with a slide-out panel next sprint, this script still works — because "Click the cart icon" uses `aria-label=Cart` or `text=Cart` under the hood, not `#header > div.nav-right > button:nth-child(3)`.

## Tips for Writing Good UAT Scripts

- **Use the words on the screen.** "Click Sign Up" is better than "Click the registration button" — the AI matches visible text.
- **One action per step.** Keep steps atomic so failures are easy to locate.
- **State your preconditions.** "User is logged in" or "Cart is empty" — the AI can set up state before running.
- **Verify outcomes, not mechanics.** "Verify the dashboard loads" is better than "Verify the spinner disappears and the div is visible."
- **Keep scripts in version control.** They're plain text. They diff cleanly. They belong next to your code.
