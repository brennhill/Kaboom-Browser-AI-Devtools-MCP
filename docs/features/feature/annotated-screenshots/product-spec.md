---
feature: annotated-screenshots
status: proposed
version: null
tool: observe
mode: page
authors: []
created: 2026-01-28
updated: 2026-01-28
doc_type: product-spec
feature_id: feature-annotated-screenshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Annotated Screenshots

> Capture a screenshot of the visible browser tab with numbered labels, bounding boxes, and interaction hints overlaid on interactive elements, enabling AI vision models to map visual understanding to programmatic action.

## Problem

AI coding agents using vision models (GPT-4o, Claude, Gemini) can see a screenshot but cannot reliably determine which elements are interactive, what their selectors are, or how to target them programmatically. A raw screenshot is visually informative but operationally useless -- the AI sees a button but has no way to click it.

Today, Kaboom's `observe({what: "page"})` returns structured metadata (headings, form count, interactive element count) but no visual representation. The `observe({what: "accessibility"})` mode returns an accessibility tree but without visual spatial context. Neither helps a vision model correlate what it sees in a screenshot with how to act on it.

This gap is critical for AI web agents. To autonomously navigate, test, or debug a web application, an agent must bridge visual perception ("I see a blue button labeled Submit in the lower right") with programmatic action (`document.querySelector('button[type="submit"]')`). Annotated screenshots create that bridge.

## Solution

Extend the existing `observe({what: "page"})` mode with an `annotate_screenshot` boolean option. When enabled, Kaboom:

1. **Discovers interactive elements** in the viewport using the extension's inject.js (running in page context).
2. **Assigns each element a numeric label** (1, 2, 3...) and collects its bounding box, best-available selector, accessible name, element type, and visible text.
3. **Captures a screenshot** using `chrome.tabs.captureVisibleTab()` (existing infrastructure).
4. **Renders annotations** onto the screenshot using an OffscreenCanvas in the extension's background service worker -- drawing numbered labels and bounding boxes over each discovered element.
5. **Returns** the annotated screenshot as base64 alongside a structured annotation map that links each label number to its selector, role, and element metadata.

The AI receives both the visual (annotated image) and the structured data (annotation map), allowing it to say: "Element [3] is the Submit button. I'll click it using `button[type='submit']`."

## User Stories

- As an AI coding agent, I want to see a screenshot with numbered interactive elements so that I can identify which element to interact with and immediately know its selector.
- As an AI coding agent, I want annotation metadata alongside the image so that I can map a visual label number to a CSS selector, ARIA role, or accessible name without additional tool calls.
- As a developer using Kaboom, I want to ask the AI "what do you see on this page?" and receive a response that references specific labeled elements, not vague descriptions like "the button near the top."
- As an AI coding agent performing E2E testing, I want annotated screenshots at each test step so that I can visually verify the page state and identify the next interaction target in a single observation.

## MCP Interface

**Tool:** `observe`
**Mode/Action:** `page` (extended with `annotate_screenshot` option)

### Request

```json
{
  "tool": "observe",
  "arguments": {
    "what": "page",
    "annotate_screenshot": true,
    "annotation_target": "interactive",
    "max_annotations": 50
  }
}
```

#### Parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `what` | string | (required) | Must be `"page"` |
| `annotate_screenshot` | boolean | `false` | When true, capture and annotate a screenshot |
| `annotation_target` | string | `"interactive"` | Which elements to annotate: `"interactive"` (buttons, inputs, links, selects), `"all_visible"` (all visible elements with text/role), or `"custom"` (use `annotation_selector`) |
| `annotation_selector` | string | `null` | CSS selector for custom annotation targets (only when `annotation_target: "custom"`) |
| `max_annotations` | number | `50` | Maximum number of elements to annotate (1-100). Elements are prioritized by viewport position (top-to-bottom, left-to-right) |

### Response

When `annotate_screenshot: false` (default), the response is unchanged from the current `page` mode -- returning page metadata (URL, title, viewport, headings, forms, interactive element count).

When `annotate_screenshot: true`:

```json
{
  "content": [
    {
      "type": "image",
      "data": "<base64-encoded-jpeg>",
      "mimeType": "image/jpeg"
    },
    {
      "type": "text",
      "text": "{\"page\":{\"url\":\"...\",\"title\":\"...\",\"viewport\":{\"width\":1280,\"height\":720}},\"annotations\":[{\"label\":1,\"selector\":\"button[type='submit']\",\"tag\":\"button\",\"role\":\"button\",\"name\":\"Submit Order\",\"text\":\"Submit Order\",\"bounds\":{\"x\":540,\"y\":620,\"width\":200,\"height\":44},\"interactionHint\":\"clickable\"},{\"label\":2,\"selector\":\"input#email\",\"tag\":\"input\",\"role\":\"textbox\",\"name\":\"Email address\",\"text\":\"\",\"bounds\":{\"x\":320,\"y\":280,\"width\":400,\"height\":36},\"interactionHint\":\"editable\"}]}"
    }
  ]
}
```

#### Annotation object fields:

| Field | Type | Description |
|-------|------|-------------|
| `label` | number | The numeric label drawn on the screenshot (1-indexed) |
| `selector` | string | Best CSS selector for this element (uses the locator priority: `[data-testid]` > `#id` > `[aria-label]` > tag+class path) |
| `tag` | string | HTML tag name (e.g., `button`, `input`, `a`) |
| `role` | string | ARIA role (explicit or implicit) |
| `name` | string | Accessible name (from aria-label, aria-labelledby, or label element) |
| `text` | string | Visible text content (truncated to 100 chars) |
| `bounds` | object | Bounding box `{x, y, width, height}` in viewport coordinates |
| `interactionHint` | string | One of: `clickable`, `editable`, `selectable`, `toggleable`, `navigable` |

## Requirements

| # | Requirement | Priority |
|---|-------------|----------|
| R1 | Capture screenshot via `chrome.tabs.captureVisibleTab()` using existing rate-limited infrastructure | must |
| R2 | Discover interactive elements (button, input, select, textarea, a[href], [role="button"], [onclick], [tabindex]) in the current viewport | must |
| R3 | Assign sequential numeric labels (1, 2, 3...) to discovered elements, ordered top-to-bottom then left-to-right | must |
| R4 | Draw numbered labels and bounding boxes on the screenshot using OffscreenCanvas | must |
| R5 | Generate best-available CSS selector for each annotated element using locator priority chain | must |
| R6 | Return annotated image as base64 JPEG in MCP image content block | must |
| R7 | Return structured annotation map as MCP text content block alongside the image | must |
| R8 | Respect `max_annotations` limit (default 50, max 100) to control image density and response size | must |
| R9 | Support `annotation_target: "interactive"` to annotate only interactive elements (default) | must |
| R10 | Support `annotation_target: "all_visible"` to annotate all visible elements with meaningful text or ARIA roles | should |
| R11 | Support `annotation_target: "custom"` with a user-supplied CSS selector | should |
| R12 | Include `interactionHint` for each element describing how it can be interacted with | should |
| R13 | Extract accessible name from aria-label, aria-labelledby, associated label, or placeholder | should |
| R14 | Skip elements that are fully obscured by other elements (z-index occlusion) | could |
| R15 | When `annotate_screenshot: false` or omitted, behavior is identical to current `page` mode (backward compatible) | must |

## Non-Goals

- This feature does NOT perform OCR or text recognition on the screenshot. It uses DOM metadata, not pixel analysis.
- This feature does NOT annotate elements outside the visible viewport. Scroll-and-capture is a separate future concern.
- This feature does NOT modify the live DOM. Annotations are drawn on a canvas copy of the screenshot, not injected into the page.
- Out of scope: Full-page stitched screenshots (scrolling capture). This captures only the visible viewport.
- Out of scope: Video or animation capture. This is a single-frame snapshot.
- Out of scope: Annotation persistence or diffing between captures. Each call is stateless.

## Performance SLOs

| Metric | Target |
|--------|--------|
| Element discovery (inject.js) | < 50ms for up to 100 elements |
| Canvas annotation rendering | < 100ms |
| Total end-to-end (capture + annotate + encode) | < 500ms |
| Base64 response size (annotated JPEG) | < 500KB typical (JPEG quality 80) |
| Memory impact during annotation | < 5MB transient (canvas + image decode) |
| No browsing degradation | Canvas work runs in service worker OffscreenCanvas, never blocks page main thread |

## Security Considerations

- **Data captured:** Screenshot pixel data (same as existing screenshot feature) plus DOM metadata (selectors, text, bounds). No new data categories introduced.
- **Sensitive content in screenshots:** Screenshots may contain sensitive on-screen content (passwords in plain text fields, PII, etc.). This is the same risk as the existing screenshot feature. Mitigation: screenshots are localhost-only, never transmitted externally.
- **Selector exposure:** CSS selectors and ARIA labels in the annotation map could reveal internal application structure. Acceptable for localhost developer tooling.
- **No new attack surface:** Uses existing `captureVisibleTab` permission (already granted), existing HTTP endpoint (`/screenshots`), and existing async query infrastructure.
- **Rate limiting:** Inherits existing screenshot rate limits (5s cooldown, 10/session max) to prevent abuse.

## Edge Cases

- What happens when the page has no interactive elements? Expected behavior: Return screenshot with no annotations and an empty `annotations` array. The `page` metadata is still returned.
- What happens when there are more than `max_annotations` elements? Expected behavior: Annotate only the first N elements by viewport position (top-left to bottom-right), include `total_found` count in the response so the AI knows elements were omitted.
- What happens when an element has no usable selector? Expected behavior: Generate a CSS path fallback (e.g., `body > div:nth-child(2) > button`). Every annotated element must have a selector.
- What happens when the extension is disconnected? Expected behavior: Return standard timeout error, same as current `observe({what: "page"})`.
- What happens when the tab is a chrome:// or extension page? Expected behavior: `captureVisibleTab` may fail on restricted pages. Return error with hint: "Cannot capture screenshots of browser internal pages."
- What happens when elements overlap? Expected behavior: Annotate all matching elements regardless of overlap. Label positioning should avoid label-on-label overlap by offsetting when bounding boxes are within 20px.
- What happens when `annotation_target: "custom"` is used with an invalid selector? Expected behavior: Return error with the invalid selector string and a hint to check CSS selector syntax.
- What happens when `annotate_screenshot` is used during an existing screenshot rate limit? Expected behavior: Return rate limit error with `nextAllowedIn` field, same as manual screenshot capture.
- What happens when the page is still loading? Expected behavior: Annotate whatever elements are currently in the DOM. Include `document.readyState` in the response so the AI can decide whether to wait and retry.

## Dependencies

- Depends on: Existing screenshot capture infrastructure (`captureScreenshot`, `canTakeScreenshot`, `recordScreenshot` in background.js)
- Depends on: Existing async query polling mechanism (pending queries, `WaitForResult` in Go server)
- Depends on: DOM query capabilities in inject.js (element discovery, bounding box extraction)
- Depends on: OffscreenCanvas API (available in Chrome service workers)
- Depended on by: Future "click by label" feature in `interact` tool (AI says "click element 3" referencing annotation labels)

## Assumptions

- A1: The extension is connected and tracking a tab with a loaded page.
- A2: The tracked tab is a standard web page (not chrome://, devtools://, or other restricted URL).
- A3: `chrome.tabs.captureVisibleTab()` returns a JPEG data URL (existing behavior).
- A4: OffscreenCanvas is available in Chrome extension service workers (Chrome 105+, MV3 baseline).
- A5: Interactive elements are identifiable via standard HTML semantics and ARIA roles. Custom JavaScript-driven interactivity (e.g., `div` with click handler but no role) may be missed unless `annotation_target: "all_visible"` or `"custom"` is used.
- A6: The MCP response format supports mixed content blocks (image + text) per the MCP specification.

## Design Decisions

### Why extend `page` mode instead of a new `what` value?

The `page` mode already represents "show me what's on the page." Adding `annotate_screenshot: true` is a natural enhancement of the same intent -- it just adds a visual representation. A separate `what: "annotated_screenshot"` would fragment page observation into two modes that return overlapping data. By extending `page`, the AI can get metadata-only (fast, text-only) or metadata+visual (richer, slower) from the same mode.

### Why number labels instead of text labels?

Numbered labels (1, 2, 3) are compact, unambiguous, and don't occlude the element they annotate. Text labels ("Submit Button", "Email Input") would overlap content and be harder to reference in conversation. The AI says "click [3]" not "click the element labeled 'Submit Button'." The structured annotation map provides the full details for each number.

### Why top-to-bottom, left-to-right ordering?

This matches natural reading order (in LTR languages), making the numbered labels predictable. When the AI describes "elements 1-5 are the navigation bar, 6-12 are the form fields," the spatial grouping is intuitive. This also matches how screen readers traverse the page, creating consistency with accessibility patterns.

### Why OffscreenCanvas in the service worker?

Drawing annotations on the page DOM (injecting overlay divs) would modify the page state, potentially breaking layouts and triggering observers. Using OffscreenCanvas in the service worker isolates rendering completely -- the page is never touched. The screenshot is captured, annotated in-memory, and returned. This also avoids content script permission issues on restricted pages.

### Selector priority chain

The selector generation follows Playwright's locator priority, which represents industry best practices for test resilience:

1. `[data-testid="value"]` -- Explicit test hook, most stable
2. `#id` -- Unique identifier, very stable
3. `[aria-label="value"]` -- Accessibility-first, meaningful
4. `role + name` combination -- Semantic, resilient to layout changes
5. CSS path fallback -- Always works, least stable

This ensures the AI gets the most maintainable selector possible for each element.

## Open Items

| # | Item | Status | Notes |
|---|------|--------|-------|
| OI-1 | Label visual style (color, size, shape) | open | Needs UX decision: bright red circles with white numbers? Semi-transparent badges? Must be visible on both light and dark backgrounds |
| OI-2 | OffscreenCanvas availability in all target Chrome versions | open | Need to verify minimum Chrome version requirement; fallback if unavailable (return unannotated screenshot + metadata only) |
| OI-3 | Response size limits for base64 images in MCP | open | Large annotated screenshots could exceed 1MB base64. May need quality/resolution controls or server-side file storage with path reference |
| OI-4 | Interaction with existing screenshot rate limiting | open | Should annotated screenshots count toward the same rate limit as manual screenshots, or have a separate budget? |
| OI-5 | Shadow DOM element discovery | open | Elements inside shadow roots may not be discoverable via standard `querySelectorAll`. Need to decide if shadow DOM traversal is in scope |
