---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Selector Strategies in Kaboom

The Kaboom extension uses a **multi-strategy selector approach** to create robust, maintainable selectors for test automation and element identification. This document explains how selectors are prioritized and when each strategy is used.

## Overview

When the extension captures user interactions or generates test scripts, it computes multiple selector strategies for each element and prioritizes them. The best strategy is chosen based on stability, semantics, and developer intent.

## Selection Priority

Selectors are evaluated in this order (highest to lowest priority):

```
1. testId (data-testid, data-test-id, data-cy)
   ↓
2. ariaLabel (aria-label attribute)
   ↓
3. Role + accessible name (implicit or explicit role)
   ↓
4. Element ID (id attribute)
   ↓
5. Text content (visible text for clickable elements)
   ↓
6. CSS path (fallback, most brittle)
```

## Strategy Details

### 1. Test ID (Highest Priority)

**Selector:** `data-testid`, `data-test-id`, or `data-cy` attributes

**Stability:** ⭐⭐⭐⭐⭐ (Most stable)

**Use Case:** Developer-controlled identifier explicitly meant for testing

**Examples:**

```html
<!-- React Testing Library -->
<button data-testid="submit-button">Submit</button>

<!-- Cypress -->
<button data-cy="checkout-btn">Pay Now</button>

<!-- Custom -->
<button data-test-id="payment-submit">Pay Now</button>
```

**Playwright Code Generated:**
```javascript
await page.getByTestId('submit-button').click();
```

**Advantages:**
- ✅ Not affected by UI changes
- ✅ Independent of text/styling
- ✅ Developer intent is explicit
- ✅ Works across frameworks

**Disadvantages:**
- ❌ Requires developers to add attributes
- ❌ Not semantic (accessibility-neutral)

**When Used:**
- Always checked first
- If present, no other strategies needed
- Modern test suites typically include this

---

### 2. Aria Label

**Selector:** `aria-label` attribute

**Stability:** ⭐⭐⭐⭐ (Very stable)

**Use Case:** Accessible label for icon buttons or unlabeled interactive elements

**Examples:**

```html
<!-- Icon button with accessible label -->
<button aria-label="Close dialog">✕</button>

<!-- Search icon -->
<button aria-label="Search"><i class="icon-search"></i></button>

<!-- Expandable menu -->
<button aria-label="Open menu">☰</button>
```

**Playwright Code Generated:**
```javascript
await page.getByLabel('Close dialog').click();
```

**Advantages:**
- ✅ Semantic (accessibility-compliant)
- ✅ Stable across UI changes
- ✅ Natural language readable

**Disadvantages:**
- ❌ Not always present (only where needed for a11y)
- ❌ May not be unique

**When Used:**
- Present AND no testId
- Useful for icon buttons, unlabeled interactive elements
- Should be used for accessibility (primary benefit)

---

### 3. Role + Accessible Name (Second Priority)

**Selector:** ARIA role + accessible name

**Stability:** ⭐⭐⭐⭐ (Very stable)

**Use Case:** Semantic interaction with named element

**Role Examples:**

```typescript
// Implicit roles (HTML semantics)
<button>Submit</button>           // role: 'button'
<a href="#">Link</a>              // role: 'link'
<input type="checkbox" />          // role: 'checkbox'
<input type="text" />              // role: 'textbox'
<select><option></select>          // role: 'combobox'
<textarea></textarea>              // role: 'textbox'
<input type="radio" />             // role: 'radio'

// Explicit roles (ARIA)
<div role="button">Custom Button</div>  // role: 'button'
<div role="tablist">                    // role: 'tablist'
  <div role="tab">Tab 1</div>
</div>
```

**Accessible Name Sources (in priority order):**

1. `aria-label` — Explicit label
2. `aria-labelledby` — Reference to labeling element
3. Text content — For buttons, links, headings
4. `label` element — For form inputs
5. `alt` attribute — For images
6. `title` attribute — As fallback

**Examples:**

```html
<!-- Button with text content -->
<button>Submit Order</button>
<!-- Accessible name: "Submit Order" -->

<!-- Input with label -->
<label for="email">Email</label>
<input id="email" type="email" />
<!-- Accessible name: "Email" -->

<!-- Button with aria-label -->
<button aria-label="Close">✕</button>
<!-- Accessible name: "Close" -->

<!-- Tab with aria-labelledby -->
<span id="tab-1-label">Profile</span>
<div role="tab" aria-labelledby="tab-1-label"></div>
<!-- Accessible name: "Profile" -->
```

**Playwright Code Generated:**
```javascript
// Role with name
await page.getByRole('button', { name: 'Submit Order' }).click();
await page.getByRole('textbox', { name: 'Email' }).fill('test@example.com');

// Role only (when name not needed)
await page.getByRole('checkbox').check();
```

**Advantages:**
- ✅ Semantic and accessible
- ✅ Natural language readable
- ✅ Works across frameworks
- ✅ Resilient to style/layout changes

**Disadvantages:**
- ❌ Requires proper ARIA/HTML semantics
- ❌ Name may not be unique
- ❌ Text-based names break if copy changes

**When Used:**
- When testId missing but role present
- Primary strategy for modern accessible applications
- Most reliable for semantic queries

---

### 4. Element ID

**Selector:** `id` attribute

**Stability:** ⭐⭐⭐⭐ (Very stable)

**Use Case:** Unique element identifier assigned by developer

**Examples:**

```html
<button id="checkout-submit">Submit</button>
<input id="email-field" type="email" />
<div id="main-content">...</div>
```

**Playwright Code Generated:**
```javascript
await page.locator('#checkout-submit').click();
```

**Advantages:**
- ✅ Should be unique on page
- ✅ CSS selector is simple and fast
- ✅ Not affected by framework updates

**Disadvantages:**
- ❌ Not semantic (no meaning conveyed)
- ❌ Often auto-generated or missing
- ❌ Many apps don't use IDs on interactive elements

**When Used:**
- testId, ariaLabel, and role not available
- Element has a meaningful ID
- Last semantic option before CSS path

---

### 5. Text Content

**Selector:** Visible text of clickable elements

**Stability:** ⭐⭐⭐ (Moderate)

**Use Case:** User-visible text for buttons, links, and labeled controls

**Examples:**

```html
<button>Submit Order</button>
<a href="/profile">View Profile</a>
<span role="button">Save</span>
<li><a href="/home">Home</a></li>
```

**Supported Elements:**
- `<button>`, `<a>`, `<label>`
- Elements with `role="button"` or `role="link"`
- Most interactive HTML elements

**Playwright Code Generated:**
```javascript
await page.getByText('Submit Order').click();
await page.getByRole('link', { name: 'View Profile' }).click();
```

**Advantages:**
- ✅ Human-readable
- ✅ Matches user perspective
- ✅ Works for most interactive elements

**Disadvantages:**
- ❌ Breaks if button text changes
- ❌ May not be unique (multiple "OK" buttons)
- ❌ Doesn't work for unlabeled elements (icons)
- ❌ Whitespace variations can break matching

**When Used:**
- Only if element is clickable or labeled
- Earlier strategies unavailable
- Text is stable and unique
- Not for non-interactive content

**Text Length Limits:**
- Maximum 256 characters captured
- Truncated for very long text
- Whitespace normalized

---

### 6. CSS Path (Fallback)

**Selector:** CSS path from element to root or nearest anchor

**Stability:** ⭐⭐ (Brittle)

**Use Case:** Last-resort fallback when no other selectors work

**Structure:**

```
tag.class > tag.class > #id
```

**Examples:**

```html
<main>
  <section class="checkout">
    <form>
      <button class="primary">Submit</button>
    </form>
  </section>
</main>
```

**Generated CSS path:**
```
main > section.checkout > form > button.primary
```

**Playwright Code Generated:**
```javascript
await page.locator('main > section.checkout > form > button.primary').click();
```

**CSS Path Algorithm:**

1. Build selector upward from element to root
2. Use `tag` alone if no distinguishing features
3. Add stable, non-dynamic classes (max 2)
4. Add element `id` and stop climbing
5. Skip dynamic/generated classes (CSS-in-JS)

**Dynamic Class Detection:**
Classes matching these patterns are skipped:

```javascript
// CSS-in-JS prefixes
/^(css|sc|emotion|styled|chakra)-/
// Random hash classes (5-8 lowercase)
/^[a-z]{5,8}$/
```

**Examples of Classes Skipped:**
```html
<!-- CSS-in-JS generated -->
<button class="css-a1b2c3">Submit</button>  <!-- ❌ skipped -->
<div class="emotion-jxk">...</div>          <!-- ❌ skipped -->
<div class="sc-a1b">...</div>               <!-- ❌ skipped -->

<!-- Custom static classes (included) -->
<button class="primary checkout-btn">...</button>  <!-- ✅ included -->
<div class="container header">...</div>            <!-- ✅ included -->
```

**Advantages:**
- ✅ Works for any element
- ✅ Generally unique (at element level)
- ✅ No semantic requirements

**Disadvantages:**
- ❌ Breaks with DOM structure changes
- ❌ Brittle across refactors
- ❌ Not maintainable long-term
- ❌ Hard to read and understand
- ❌ Fails if elements added/removed in hierarchy

**When Used:**
- Always computed as ultimate fallback
- Only used if all other strategies fail
- Should trigger review of testId/aria-label usage

---

## Decision Tree

```
┌─────────────────────────────────────────┐
│ Element to select for test              │
└────────────────────┬────────────────────┘
                     │
        ┌────────────▼────────────┐
        │ Has data-testid or     │
        │ data-test-id or        │
        │ data-cy?               │
        └────────────┬────────────┘
                    YES ─────► Use testId ✅
                     │
                    NO
                     │
        ┌────────────▼────────────┐
        │ Has aria-label?         │
        └────────────┬────────────┘
                    YES ─────► Use ariaLabel ✅
                     │
                    NO
                     │
        ┌────────────▼────────────────────┐
        │ Has ARIA role (implicit or      │
        │ explicit) AND accessible name?  │
        └────────────┬────────────────────┘
                    YES ─────► Use role + name ✅
                     │
                    NO
                     │
        ┌────────────▼────────────┐
        │ Has id attribute?       │
        └────────────┬────────────┘
                    YES ─────► Use id ✅
                     │
                    NO
                     │
        ┌────────────▼─────────────────┐
        │ Is clickable and has        │
        │ visible text content?       │
        └────────────┬─────────────────┘
                    YES ─────► Use text ✅
                     │
                    NO
                     │
        ┌────────────▼────────────┐
        │ Use CSS path            │
        │ (fallback)              │
        └────────────────────────┘
                  ⚠️  Warning: brittle selector
```

## Real-World Examples

### Example 1: Login Form

```html
<form id="login-form">
  <div class="form-group">
    <label for="username">Username</label>
    <input
      id="username"
      type="text"
      name="username"
      data-testid="login-username"
      aria-label="Username"
    />
  </div>
  <div class="form-group">
    <label for="password">Password</label>
    <input
      id="password"
      type="password"
      name="password"
      data-testid="login-password"
      aria-label="Password"
    />
  </div>
  <button type="submit" data-testid="login-submit">
    Sign In
  </button>
</form>
```

**Selectors Computed:**

| Element | testId | ariaLabel | role | id | text | cssPath |
|---------|--------|-----------|------|----|----|---------|
| Username input | `login-username` | `Username` | textbox | `username` | — | `form#login-form > div > input` |
| Submit button | `login-submit` | — | button | — | `Sign In` | `form#login-form > button` |

**Selected Selectors (in priority):**
- Username: `testId` → `getByTestId('login-username')`
- Submit: `testId` → `getByTestId('login-submit')`

---

### Example 2: Icon Button with Tooltip

```html
<button
  aria-label="Close dialog"
  class="icon-button"
  onclick="closeDialog()"
>
  <svg viewBox="0 0 24 24">
    <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12 19 6.41z"/>
  </svg>
</button>
```

**Selectors Computed:**

| Property | Value |
|----------|-------|
| testId | — |
| ariaLabel | `Close dialog` |
| role | `button` |
| id | — |
| text | (svg, no text) |
| cssPath | `button.icon-button` |

**Selected Selector:**
- `ariaLabel` → `getByLabel('Close dialog')`

---

### Example 3: Shopping Cart Item

```html
<div class="cart-item" data-testid="cart-item-12345">
  <img src="product.jpg" alt="Blue Widget" />
  <div class="product-info">
    <h3>Blue Widget</h3>
    <p class="price">$49.99</p>
    <button aria-label="Remove from cart">
      <span>Remove</span>
    </button>
  </div>
</div>
```

**Selectors for Remove Button:**

| Strategy | Value | Status |
|----------|-------|--------|
| testId | — | ❌ missing |
| ariaLabel | `Remove from cart` | ✅ found |
| role | `button` | ✅ button |
| text | `Remove` | ✅ found |
| cssPath | `div.cart-item > div > button` | ✅ found |

**Selected Selector:**
- `ariaLabel` → `getByLabel('Remove from cart')`

---

### Example 4: Dynamic React App

```html
<div class="emotion-jhk2">
  <button class="chakra-button-abc123 primary">
    Click Me
  </button>
</div>
```

**Selectors Computed:**

| Property | Value | Notes |
|----------|-------|-------|
| testId | — | not set |
| role | button | implicit |
| text | `Click Me` | clickable |
| cssPath | `button.primary` | CSS-in-JS classes skipped |

**Selected Selector:**
- Text content → `getByText('Click Me')`
- Or (better): CSS path → `locator('button.primary')`

---

## Best Practices

### For Developers (Adding Selectors)

**✅ DO:**

```html
<!-- Explicit testId -->
<button data-testid="checkout-submit">Pay Now</button>

<!-- Semantic roles -->
<button>Delete Item</button>

<!-- Accessible labels -->
<button aria-label="Close">✕</button>

<!-- Form labels -->
<label for="email">Email</label>
<input id="email" type="email" />
```

**❌ DON'T:**

```html
<!-- Avoid aria-label for text-based buttons -->
<button aria-label="Click this button">Click this button</button>

<!-- Avoid divs instead of buttons -->
<div onclick="submit()">Submit</div>

<!-- Avoid random classes -->
<button class="abc123 xyz789">Submit</button>

<!-- Avoid ID-only inputs without labels -->
<input id="f1" type="text" />
```

### For Test Automation

**✅ Selector Priority:**
1. Use testId when available
2. Fall back to role + name for semantic queries
3. Use ariaLabel for icon buttons
4. Use text content for buttons/links
5. Avoid CSS path in final tests

**Playwright Best Practices:**

```javascript
// ✅ GOOD: Semantic, stable
await page.getByRole('button', { name: 'Submit' }).click();
await page.getByLabel('Email').fill('test@example.com');
await page.getByTestId('checkout-btn').click();

// ⚠️  ACCEPTABLE: Works but less semantic
await page.getByText('Submit Order').click();

// ❌ AVOID: Brittle, maintenance nightmare
await page.locator('main > section > form > button.primary').click();
```

---

## Limitations & Edge Cases

### Shadow DOM

Selectors don't cross shadow DOM boundaries. For elements inside shadow roots:
- CSS path will stop at the shadow host
- Role/text may not match if hidden by shadow
- Consider using testId for shadow DOM elements

### iframes

- Selectors are computed per-frame
- CSS path doesn't cross iframe boundaries
- Frame context required to query inside iframe

### Dynamic Content

- Text-based selectors fail if content changes
- CSS path breaks if DOM restructured
- testId and role+name more resilient

### Rapidly Changing IDs

If element IDs change frequently:
- Avoid using id selector
- Prefer testId or role+name
- Use data attributes for stability

### Missing Accessible Names

For elements without testId and no semantic name:
- Falls back to CSS path
- Consider adding aria-label or data-testid
- Review accessibility compliance

---

## Performance Notes

Selector computation is optimized for speed:

- **Implicit role detection:** O(1) lookup table by tag
- **CSS path generation:** O(n) where n = depth
- **Dynamic class detection:** Regex match on class names
- **Text extraction:** O(1) with length limit

No DOM traversal is required beyond the element hierarchy.

## Debugging

To see all computed selectors for an element:

```javascript
if (window.__kaboom) {
  const element = document.querySelector('button');
  const selectors = window.__kaboom.getSelectors(element);
  console.log('All selectors:', selectors);
  // {
  //   testId: 'submit-btn',
  //   ariaLabel: undefined,
  //   role: { role: 'button', name: 'Submit' },
  //   id: undefined,
  //   text: 'Submit',
  //   cssPath: 'main > form > button'
  // }
}
```
