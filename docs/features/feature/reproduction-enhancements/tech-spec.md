---
status: shipped
scope: feature/reproduction-enhancements/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-reproduction-enhancements
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-reproduction-enhancements.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Reproduction Enhancements Review](reproduction-enhancements-review.md).

# Technical Spec: Reproduction Script Enhancements

## Purpose

Kaboom's existing `get_reproduction_script` and `generate_test` tools produce basic Playwright scripts: they replay user actions (clicks, inputs, navigations) and optionally assert on network responses. This is useful for bug reproduction but limited in three ways:

1. **No visual verification**: The script clicks buttons and types text, but can't verify that the page LOOKS correct. A visual regression (broken layout, missing element, wrong color) passes the test because the assertions only check network responses.

2. **No data setup**: The script assumes the app starts in the right state. If the bug depends on specific data (a user with 50 items in their cart, a dashboard with 3 months of data), the test fails on a fresh environment because that data doesn't exist.

3. **No screenshots as evidence**: When the AI reproduces a bug, the human wants to SEE it. A screenshot in the reproduction script proves the bug exists visually — it's evidence the human can verify in seconds without running the script.

These enhancements make reproduction scripts self-contained (they set up their own data), verifiable (they include visual assertions), and evidential (they embed screenshots of the observed state).

---

## Opportunity & Business Value

**Visual regression testing**: The generated Playwright scripts can include `expect(page).toHaveScreenshot()` assertions. When the AI fixes a CSS bug, the test captures a "golden" screenshot. Future regressions are caught automatically by Playwright's visual comparison. This turns AI bug fixes into permanent regression guards.

**Self-contained test fixtures**: A test that calls `POST /api/users` to create test data before running assertions is portable — it works on any environment without manual data setup. The AI observes which API calls populated the page during the session and generates corresponding setup calls in the test. The test is self-documenting: "this is the state the bug requires."

**Evidence-based bug reports**: When the AI encounters a bug, it can generate a Markdown report with embedded screenshots: "Step 1: Navigate to /dashboard [screenshot]. Step 2: Click 'Export' [screenshot]. Step 3: Error appears [screenshot showing the error]." This is a complete bug report that any developer can verify without running code.

**Playwright Test integration**: Playwright's test runner supports screenshot assertions natively. Generated tests that include visual assertions integrate directly into the project's existing Playwright test suite — no new tools, no new CI configuration.

**Interoperability with visual testing services**: Screenshots can be uploaded to Percy, Chromatic, Applitools, or BackstopJS for cloud-based visual regression tracking. The generated test produces screenshots in a standard format these tools consume.

**Data fixture portability**: The generated fixtures are standard HTTP requests (fetch calls in the test). They work with any backend — REST, GraphQL, gRPC-Web. Teams can extract the fixtures into Postman collections, seed scripts, or factory functions.

---

## How It Works

### Screenshot Insertion

The extension can capture screenshots at key moments during the session using the Chrome DevTools Protocol's `Page.captureScreenshot` API (accessible from the background script via `chrome.debugger`).

**When screenshots are captured**:
1. After each navigation completes (page load + 1s settling)
2. After each user action that changes page content (click on a button, form submission)
3. When an error occurs (console error or unhandled exception)
4. On explicit request via a new `capture_screenshot` message from the AI

**Screenshot storage**:
- Stored as PNG, compressed, in the server's memory buffer
- Max 20 screenshots per session (FIFO eviction)
- Max 500KB per screenshot (resized to 1280px wide if larger)
- Thumbnail (320px) generated for embedding in markdown reports

**Integration with reproduction script**:
The `get_reproduction_script` tool gains a new parameter `include_screenshots: true`. When enabled, the Playwright script includes `await page.screenshot({ path: 'step-N.png' })` calls at each action point. The response also includes the actual screenshot data (base64-encoded) so the AI can embed them in bug reports or PR descriptions.

### Visual Assertions

When `generate_test` is called with `assert_visual: true`, the generated test includes Playwright's visual comparison assertions:

```javascript
// After navigation
await expect(page).toHaveScreenshot('dashboard-loaded.png', { maxDiffPixels: 100 });

// After clicking "Add Item"
await page.getByRole('button', { name: 'Add Item' }).click();
await expect(page).toHaveScreenshot('after-add-item.png', { maxDiffPixels: 100 });
```

The `maxDiffPixels` threshold accounts for anti-aliasing differences, timestamps, and other dynamic content. The default of 100 pixels is conservative — the developer can tune it.

For elements with dynamic content (timestamps, counters, random data), the generated test includes mask annotations:

```javascript
await expect(page).toHaveScreenshot('dashboard.png', {
  mask: [page.locator('.timestamp'), page.locator('.random-id')]
});
```

The AI identifies dynamic elements by observing which DOM nodes change between observations of the same page (using the checkpoint diff system).

### Data Fixture Generation

The AI observes API responses that populate the page. When generating a test, it can include a `beforeAll` block that recreates this data:

```javascript
test.beforeAll(async ({ request }) => {
  // Create the user that appears on the dashboard
  await request.post('/api/users', {
    data: { name: 'Test User', email: 'test@example.com', role: 'admin' }
  });

  // Create the items that appear in the list
  for (const item of testItems) {
    await request.post('/api/items', { data: item });
  }
});
```

**How fixtures are derived**:
1. From network body captures: API responses that returned 200 and contain JSON arrays or objects are candidates for fixture data
2. From request bodies: POST/PUT requests the user made during the session represent data mutations — these become fixture setup calls
3. The AI decides which requests are "setup" (happened before the bug) vs. "test actions" (the bug reproduction steps)

**Fixture simplification**:
- Only fields that affect the test are included (IDs, names, statuses — not timestamps, metadata)
- Arrays are trimmed to the minimum needed to reproduce the behavior (if the bug requires "more than 10 items," the fixture creates 11, not 500)
- Sensitive data (emails, passwords) are replaced with test-safe values

### MCP Tool Enhancements

**`get_reproduction_script`** — new parameters:
- `include_screenshots` (boolean): Embed screenshot capture points in the script. Returns screenshots as base64 in metadata.
- `include_fixtures` (boolean): Generate a `beforeAll` block with data setup from observed API calls.

**`generate_test`** — new parameters:
- `assert_visual` (boolean): Include `toHaveScreenshot` assertions at key points.
- `include_fixtures` (boolean): Same as above.
- `mask_dynamic` (boolean, default true): Automatically mask elements that changed between observations.

**New tool: `capture_screenshot`**:
- Captures a screenshot of the current page state
- Returns the screenshot as base64 and stores it in the buffer
- Parameters: `selector` (optional, capture specific element), `full_page` (boolean, capture below the fold)

### Bug Report Generation

A new tool `generate_bug_report` combines screenshots, actions, and errors into a Markdown document:

```markdown
## Bug Report: Export button throws TypeError

**URL**: http://localhost:3000/dashboard
**Observed**: 2026-01-24T10:30:00Z
**Severity**: Error (unhandled exception)

### Steps to Reproduce

1. Navigate to /dashboard
   ![Step 1](data:image/png;base64,...)

2. Click "Export" button
   ![Step 2](data:image/png;base64,...)

3. Error: `TypeError: Cannot read property 'map' of undefined` at dashboard.js:142
   ![Step 3: Error state](data:image/png;base64,...)

### Expected Behavior
Export dialog should open with data format options.

### Actual Behavior
Unhandled TypeError thrown. Export button becomes unresponsive.

### Environment
- URL: http://localhost:3000/dashboard
- Browser: Chrome 120
- Kaboom capture session: 2026-01-24

### Reproduction Script
\`\`\`javascript
// Playwright script...
\`\`\`
```

---

## Data Model

### Screenshot Entry

Stored in the server's screenshot buffer:
- ID (monotonic counter)
- Capture timestamp
- URL at capture time
- Trigger: "navigation", "action", "error", or "explicit"
- PNG data (compressed)
- Thumbnail PNG (320px wide)
- Associated action ID (if triggered by a user action)
- Size in bytes

### Fixture Entry

Derived from network body captures:
- Request method and URL
- Request body (sanitized)
- Response status
- Response body summary (fields that matter for the test)
- Classification: "setup" (before bug), "action" (during bug), "irrelevant"

---

## Extension Changes

### Screenshot Capture

The background script uses `chrome.debugger.sendCommand` to capture screenshots:

```javascript
chrome.debugger.attach({ tabId }, '1.3', () => {
  chrome.debugger.sendCommand({ tabId }, 'Page.captureScreenshot', {
    format: 'png',
    quality: 80,
    clip: { x: 0, y: 0, width: viewportWidth, height: viewportHeight, scale: 1 }
  }, (result) => {
    // result.data is base64 PNG
    sendScreenshotToServer(result.data)
  })
})
```

The debugger is attached only when screenshot capture is enabled (default: disabled until the AI requests it or the user enables it in options). This avoids the "Chrome is being controlled by automated test software" banner that `chrome.debugger` triggers.

Alternative approach: Use `chrome.tabs.captureVisibleTab` which doesn't require debugger attachment but only captures the visible viewport (no full-page, no element-specific captures). This is the default; debugger mode is opt-in for advanced captures.

### Auto-Capture Configuration

The AI can configure when screenshots are automatically captured:

```
// Via MCP tool or extension settings:
{ "auto_screenshot": { "on_navigation": true, "on_error": true, "on_action": false } }
```

By default, only `on_error` is true (screenshots on errors are the highest-value use case). `on_action` is expensive (many actions per session) and only enabled when the AI is actively debugging a visual issue.

---

## Edge Cases

- **Debugger permission denied**: Falls back to `chrome.tabs.captureVisibleTab`. Full-page and element captures are unavailable.
- **Headless Chrome**: Screenshots work normally (Playwright runs headless by default).
- **Very long pages** (full_page mode): Limited to 16384px height (Chrome's max surface size). Taller pages are cropped with a note.
- **Dynamic content in screenshots** (animations, video): Single frame captured. Animation state is non-deterministic — visual assertions should mask animated regions.
- **Screenshot buffer full** (20 entries): Oldest screenshot evicted. The evicted screenshot's metadata (timestamp, trigger, URL) is preserved for the timeline.
- **Large response bodies for fixtures**: Only the first 10KB of each response is used for fixture generation. Large datasets are represented as "generate N items with these fields."
- **GraphQL requests**: Fixture generation includes the query/mutation string, not just the URL. The generated fixture uses the same GraphQL operation.
- **Authentication-dependent data**: Fixture generation includes a login step if the session started with an auth request.
- **Cross-origin screenshots**: Only the visible tab content is captured. Cross-origin iframes are rendered but their internal content may be blank (browser security).

---

## Performance Constraints

- Screenshot capture: under 200ms (browser compositing + PNG encoding)
- Thumbnail generation: under 50ms (resize from full screenshot)
- Screenshot memory: max 10MB (20 × 500KB)
- Fixture derivation: under 20ms (scanning network body buffer)
- Bug report generation: under 50ms (markdown formatting + base64 embedding)
- No impact on page rendering (screenshots use existing compositor output)

---

## Test Scenarios

1. `capture_screenshot` returns valid base64 PNG
2. `capture_screenshot` with selector captures specific element
3. `capture_screenshot` full_page captures below the fold
4. Auto-screenshot on error captures page state
5. Auto-screenshot on navigation captures loaded page
6. Screenshot buffer evicts oldest at 20 entries
7. `get_reproduction_script` with `include_screenshots` adds screenshot calls
8. `generate_test` with `assert_visual` adds toHaveScreenshot assertions
9. Dynamic elements masked in visual assertions
10. `include_fixtures` generates beforeAll with API calls
11. Fixture sanitizes sensitive data
12. Fixture arrays trimmed to minimum needed
13. `generate_bug_report` produces valid Markdown with embedded images
14. Bug report includes steps, error, and reproduction script
15. Fallback to captureVisibleTab when debugger unavailable
16. Screenshot size under 500KB (resized if needed)
17. GraphQL fixtures include query string
18. Auth-dependent fixtures include login step
19. Thumbnail generated at 320px width

---

## File Locations

Server implementation: `cmd/browser-agent/codegen.go` (enhanced script generation, fixture derivation, bug report).

Extension implementation: `extension/background.js` (screenshot capture, auto-capture configuration).

Tests: `cmd/browser-agent/codegen_test.go` (server-side), `extension-tests/screenshot.test.js` (extension-side).
