---
status: proposed
scope: feature/dom-fingerprinting/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-dom-fingerprinting
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

> **[MIGRATION NOTICE]**
> Canonical location for this tech spec. Migrated from `/docs/ai-first/tech-spec-dom-fingerprinting.md` on 2026-01-26.
> See also: [Product Spec](product-spec.md) and [Dom Fingerprinting Review](./index.md).

# Technical Spec: DOM Fingerprinting

## Purpose

AI agents often need to verify that a page looks correct after making changes. The current approach — taking a screenshot and sending it to a vision model — costs roughly $0.03 per check and takes 2-3 seconds. For an agent that checks the page 20+ times per session, this adds up to $0.60+ and nearly a minute of waiting.

DOM fingerprinting provides a 300x cheaper, 10x faster alternative. Instead of a pixel image, it extracts the page's semantic structure: what landmarks exist, what interactive elements are visible, what content sections are populated, and whether any error/loading/empty states are showing. This structural snapshot uses about 500 tokens (vs. 1500+ for a vision-model screenshot description) and takes under 30ms to extract.

The trade-off: fingerprinting catches structural problems (missing nav, empty list, error alert visible, button disabled) but not visual problems (wrong color, misaligned layout, clipped text). For AI coding workflows, structural verification covers the majority of regressions.

---

## How It Works

### Extraction (Extension-Side)

When the agent calls `get_dom_fingerprint`, the server creates a pending query (using the same mechanism as `analyze({what: "dom"})`). The extension picks it up via polling, routes it to the content script, which relays it to the injected page script.

The page script runs `extractDOMFingerprint()` which walks the DOM and extracts:

1. **Landmarks**: Checks for standard page regions — header, main, nav, footer, aside — by tag name and ARIA role. For each, records whether it's present and what it contains.

2. **Content**: Counts and catalogs headings (with level and text), lists (with item count), forms (with field names and button labels), tables (with row/column counts), and images (with alt text and broken counts).

3. **Interactive elements**: Finds all buttons, links, inputs, selects, textareas, and elements with `role="button"` or `tabindex="0"`. For each, records the type, accessible name, visibility, enabled state, and (for links) the href path.

4. **Page state**: Searches for UI state indicators using common selector patterns:
   - Errors: `[role="alert"]`, `.error`, `.alert-danger`, `[data-testid*="error"]`
   - Loading: `[aria-busy="true"]`, `.loading`, `.spinner`, `.skeleton`
   - Empty states: `.empty-state`, `.no-results`, `.no-data`
   - Modals: `[role="dialog"]` that isn't hidden
   - Notifications: `[role="status"]`, `.toast`, `.notification`

The result flows back through the message chain (inject → content → background → server) and the server returns it to the agent.

### Comparison (Server-Side)

The `compare_dom_fingerprint` tool takes a current fingerprint and compares it against a stored baseline or previous fingerprint. It detects:

- **Error appeared**: An error element is now visible that wasn't before
- **Landmark missing**: A page region (header, main, nav, etc.) is no longer present
- **List empty**: A list that had items now has zero items
- **Element missing/added**: Interactive elements that disappeared or appeared

Each change gets a severity level (error, warning, info) and a human-readable description.

### Hash for Quick Comparison

Each fingerprint gets an 8-character hash derived from its structure (ignoring timestamps). If the hash matches a previous one, the page structure hasn't changed — no detailed comparison needed.

---

## Data Model

### Fingerprint

A DOM fingerprint contains:
- URL and page title
- Viewport dimensions (width × height)
- Capture timestamp
- Structure object (landmarks, content, interactive, state)
- Hash (8 hex chars, computed from structure)
- Token count estimate

### Landmarks

A map of region name → landmark info:
- `present`: boolean
- `contains`: list of notable child elements (nav, form, section tags, roles, ids)
- `interactive`: labels of the first few buttons/links within this region
- `role`: ARIA role

### Content

- `headings`: list of {level, text} (capped at 50)
- `lists`: list of {selector, items count, type} (capped at 20)
- `forms`: list of {selector, field labels, button labels} (capped at 10)
- `tables`: list of {selector, rows, columns} (capped at 10, detailed depth only)
- `images`: {count, with_alt count, broken count}

### Interactive Elements

List of up to 100 elements (50 for standard depth), each with:
- `type`: button, link, input, select, textarea, or interactive
- `text`: accessible name (aria-label > aria-labelledby > textContent > title > placeholder)
- `visible`: always true (hidden elements are skipped)
- `enabled`: whether the element is not disabled
- `href`: path (for links only)
- `value`: "[has value]" or "" (for inputs only, never the actual value)

### Page State

Lists of state indicator elements found on the page:
- `error_elements`: visible error indicators
- `loading_indicators`: busy/loading/spinner elements
- `empty_states`: empty/no-results indicators
- `modals_open`: visible dialog/modal elements
- `notifications`: toast/status messages

Each has a selector, text content (truncated to 200 chars), and role.

### Comparison Result

- `status`: "unchanged", "changed", or "error"
- `severity`: highest severity among changes ("clean", "info", "warning", "error")
- `changes`: list of specific changes with type, severity, element, description
- `unchanged`: list of categories that didn't change
- `summary`: human-readable one-liner

---

## Tool Interface

### `get_dom_fingerprint`

**Parameters** (all optional):
- `scope`: "full" (entire page, default), "above_fold" (visible viewport only), or a CSS selector (only within that element)
- `depth`: "minimal" (landmarks only), "standard" (landmarks + interactive + content), or "detailed" (everything including tables)
- `baseline_name`: If provided, automatically compares against this baseline's stored fingerprint and returns both the fingerprint and the comparison

**Returns**: The fingerprint object, optionally with a comparison result attached.

### `compare_dom_fingerprint`

**Parameters**:
- `against` (required): Baseline name or fingerprint hash to compare against
- `severity_threshold`: Only report changes at or above this severity: "info", "warning", or "error" (default: "warning")

**Returns**: The comparison result.

---

## Accessibility Name Resolution

The system determines an element's accessible name using this priority order:
1. `aria-label` attribute
2. `aria-labelledby` → text of the referenced element
3. `textContent` (if under 100 characters)
4. `title` attribute
5. `placeholder` attribute
6. `name` attribute

Elements without any accessible name (except inputs) are skipped — they're not useful for structural fingerprinting.

---

## Visibility Detection

An element is considered visible if:
- It has an `offsetParent` (or has `position: fixed`)
- Its computed `display` is not "none"
- Its computed `visibility` is not "hidden"
- Its computed `opacity` is not "0"

Hidden elements are excluded from the fingerprint entirely.

---

## Selector Generation

When the fingerprint needs to identify an element (for lists, forms, etc.), it generates a simple selector using this priority:
1. `#id` if the element has an ID
2. `[data-testid="value"]` if it has a test ID
3. `.first-class-name` if it has CSS classes (skipping internal/generated ones starting with _)
4. The tag name as fallback

---

## Extension Message Flow

This uses the existing pending query infrastructure:

1. Server creates a pending query with type "dom_fingerprint" and the scope/depth params
2. Extension polls `/pending-queries`, picks up the query
3. background.js sends it to content.js via `chrome.tabs.sendMessage`
4. content.js relays to inject.js via `window.postMessage`
5. inject.js runs `extractDOMFingerprint()` and posts the result back
6. Result flows back: inject → content → background → POST `/query-result`
7. Server receives the result and returns it to the MCP caller

The query has a 5-second timeout. If no response arrives (extension not connected, page not loaded), the tool returns an error.

---

## Depth Modes

- **minimal**: Only extracts landmarks. Fastest, smallest output. Good for quick "is the page structure intact?" checks.
- **standard** (default): Landmarks + interactive elements (up to 50) + content (headings, lists, forms, images). Good balance of detail and token cost.
- **detailed**: Everything including tables, up to 100 interactive elements. Use when you need to verify specific data table contents.

---

## Edge Cases

- **Scope element not found**: If a CSS selector scope doesn't match any element, returns an error "Scope element not found."
- **No extension connected**: Query times out after 5 seconds, returns error.
- **Massive DOM**: All element lists are capped (100 interactive, 50 headings, 20 lists, 10 forms, 10 tables). The budget is enforced at extraction time.
- **Dynamic content**: The fingerprint is a point-in-time snapshot. If content is still loading (spinner visible), the fingerprint will show the loading state — which is useful information for the agent.
- **Shadow DOM**: Not currently traversed. Only light DOM elements are extracted.
- **iframes**: Not traversed. Only the main document is fingerprinted.

---

## Performance Constraints (Extension-Side)

- Total `extractDOMFingerprint`: under 30ms on the main thread
- `extractLandmarks`: under 2ms (fixed set of queries)
- `extractInteractive`: under 15ms (capped element count)
- `extractContent`: under 10ms (capped lists)
- `extractPageState`: under 3ms (fixed selector set)
- `isVisible` per element: under 0.1ms (single getComputedStyle)
- Response size: under 5KB typical

A performance guard warns (console.warn) if extraction exceeds the 30ms budget.

---

## Test Scenarios

### Server Tests

1. No extension connected → timeout error after 5 seconds
2. Valid fingerprint response → parsed with hash and token count
3. With baseline_name → response includes fingerprint AND comparison
4. No changes between fingerprints → status "unchanged", severity "clean"
5. Error element appeared → change type "error_appeared", severity "error"
6. Landmark missing → change type "landmark_missing", severity "error"
7. List empty → change type "list_empty", severity "warning"
8. Severity threshold filters lower-severity changes
9. Hash is deterministic for same structure
10. Hash changes when structure changes
11. filterBySeverity correctly filters change list

### Extension Tests

12. Full extraction returns correct structure shape
13. Semantic HTML landmarks found by tag name
14. ARIA role landmarks found
15. Buttons and links found with correct type/text
16. Max elements cap respected
17. Hidden elements skipped
18. Error elements detected by role="alert" and class patterns
19. Loading indicators detected
20. Headings extracted with correct level and text
21. List items counted correctly
22. Minimal depth skips lists and forms
23. visibility:hidden elements detected as hidden
24. display:none elements detected as hidden
25. Accessible name priority order correct
26. Selector prefers ID, then data-testid, then class
27. Message handler responds to fingerprint query
28. Scope "above_fold" limits to viewport
29. CSS selector scope limits extraction to that subtree

---

## File Locations

- Server implementation: `cmd/browser-agent/ai_fingerprint.go`
- Server tests: `cmd/browser-agent/ai_fingerprint_test.go`
- Extension extraction logic: additions to `extension/inject.js`
- Extension tests: `extension-tests/fingerprint.test.js`
