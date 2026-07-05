---
feature: a11y-tree-snapshots
status: proposed
doc_type: tech-spec
feature_id: feature-a11y-tree-snapshots
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: A11y Tree Snapshots

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

A11y Tree Snapshots adds `observe({what: "a11y_tree"})` -- a new observation mode that traverses the page's accessibility tree and returns a compact, indented text representation with stable UIDs for interactive elements. This is implemented as a new query type dispatched through the existing pending-queries infrastructure.

The feature requires no `chrome.automation` API (which requires special permissions). Instead, it uses manual DOM traversal with `TreeWalker` and ARIA attribute extraction, running in inject.js alongside existing DOM query code. This is the same approach used by Playwright's accessibility tree implementation.

## Key Components

**Tree Traversal Engine**: Uses `document.createTreeWalker()` with `NodeFilter.SHOW_ELEMENT` to walk DOM tree. For each element: computes ARIA role (explicit `role` attribute or implicit from tag), computes accessible name (aria-label > aria-labelledby > alt > title > text content), assigns UID to interactive elements, builds minimal CSS selector for UID'd elements, formats output as indented text.

**UID Assignment**: Interactive elements get auto-incrementing integer UIDs (1, 2, 3, ...). Interactive roles include: link, button, textbox, checkbox, radio, combobox, listbox, menuitem, tab, switch, slider, spinbutton, searchbox. Also includes elements with `tabindex >= 0`.

UIDs are deterministic within a snapshot: same element structure produces same UIDs. Achieved by assigning UIDs in tree traversal order (depth-first, left-to-right).

**CSS Selector Generation**: For each UID'd element, computes minimal CSS selector with preference order: `#id` (if unique) > `[name="..."]` (if unique) > `[data-testid="..."]` (if present) > positional selector using `tag:nth-of-type(n)` up to 3 ancestors.

Selector stored in `uidMap` so AI can use it with `interact({action: "execute_js"})` or `interact({action: "highlight"})`.

**Text Formatting**: Output is indented text tree (not JSON) for token efficiency. Format:
```
role "name" [uid=N] attributes
```

Indentation = 2 spaces per depth level. Tables and lists summarized (`table "Name" (N rows)`, `list "Name" (N items)`) with first few items shown. Text content > 100 chars truncated with ellipsis.

**Query Dispatch**: Uses existing pending-queries polling. Server creates `PendingQuery{Type: "a11y_tree"}` -> Extension polls /pending-queries -> background.js dispatches to content.js -> content.js forwards to inject.js -> inject.js traverses DOM and builds tree -> result posted to /a11y-result -> Server returns to AI.

## Data Flows

```
AI calls observe({what: "a11y_tree", scope: "#main", interactive_only: false, max_depth: 8})
  |
  v
Server creates PendingQuery{Type: "a11y_tree", Params: {scope, interactive_only, max_depth}}
  |
  v
Extension polls /pending-queries, receives query
  |
  v
background.js dispatches to content.js: {type: "A11Y_TREE_QUERY", params: {...}}
  |
  v
content.js forwards to inject.js via postMessage
  |
  v
inject.js: buildA11yTree() function executes
  -> TreeWalker traverses DOM from scope root
  -> For each element: compute role, name, check if interactive
  -> Assign UID to interactive elements
  -> Build CSS selector for UID'd elements
  -> Format as indented text
  -> Enforce max_depth and node count limits
  |
  v
inject.js posts result to content.js
  |
  v
content.js forwards to background.js
  |
  v
background.js POSTs to /a11y-result
  |
  v
Server returns response to AI with tree text, uidMap, metadata
```

## Implementation Strategy

**New extension files**:
- `extension/lib/a11y-tree.js` (~200 lines): Tree traversal, UID assignment, text formatting
- Constants added to `extension/lib/constants.js`: `MAX_A11Y_TREE_NODES` (5000), `MAX_A11Y_TREE_DEPTH` (15), `MAX_TEXT_LENGTH` (100)

**Modified extension files**:
- `extension/background.js`: Add handler for `A11Y_TREE_QUERY` message type, dispatch to content script
- `extension/content.js`: Add postMessage listener for `KABOOM_A11Y_TREE_QUERY`, forward to inject.js
- `extension/inject.js`: Import a11y-tree.js module, handle tree query message

**Server files**:
- `cmd/browser-agent/queries.go`: Add `toolObserveA11yTree()` handler, creates pending query, waits for result
- `cmd/browser-agent/types.go`: Add `A11yTreeResult` struct
- `cmd/browser-agent/tools_core.go`: Wire up `observe({what: "a11y_tree"})` to `toolObserveA11yTree()`

**Trade-offs**:
- Text format vs JSON: Text is 3-5x more token-efficient but harder to parse programmatically. Chosen because token efficiency is core value proposition.
- Deterministic UIDs vs UUIDs: Integer UIDs (1, 2, 3) are token-efficient but only unique within a snapshot. UUIDs would be globally unique but cost more tokens. Chosen integers since UIDs only meaningful within snapshot.
- No caching: Unlike a11y audit (which caches axe-core results), tree snapshots are not cached. Tree traversal is fast (~100ms) and page state changes frequently, so fresh snapshot preferred.

## Edge Cases & Assumptions

### Edge Cases

- **Scope element does not exist**: Return `{error: "scope_not_found", message: "No element matches selector '#nonexistent'"}`.

- **Blank page (about:blank)**: Return minimal tree with just `document "(empty)"` and zero interactive elements.

- **Page mid-load**: Return whatever currently in DOM. Snapshot reflects instant of traversal. AI can re-request after load completes.

- **Extremely deep DOM (depth > 15)**: Traversal stops at `max_depth`. `maxDepthReached: true`. Hint suggests narrowing scope.

- **10,000+ DOM nodes**: Node processing caps at 5,000 nodes. Remaining summarized as `...(N more nodes truncated)`. Total output capped at 50KB.

- **Extension disconnected**: Standard timeout error (consistent with existing query timeout behavior).

- **ARIA `role="presentation"` or `role="none"`**: Elements skipped (explicitly marked as non-semantic).

- **`aria-hidden="true"` elements**: Skipped along with all descendants (intentionally hidden from assistive technology).

- **Concurrent a11y_tree requests**: Each creates independent traversal. No caching, so each request gets fresh snapshot.

### Assumptions

- A1: Extension connected and tracking tab (standard precondition for all on-demand queries).
- A2: Page has meaningful DOM loaded (not browser internal page like `chrome://extensions`).
- A3: ARIA roles and labels reasonably well-authored. Poorly-labeled pages produce less useful tree but feature still functions (falls back to tag-based implicit roles).
- A4: `TreeWalker` API available in all Chromium-based browsers Kaboom targets (Chrome 90+). Well-supported, stable API.
- A5: Integer UIDs sufficient. Don't need UUIDs or globally unique identifiers since UIDs only meaningful within single snapshot, regenerated on each call.

## Risks & Mitigations

### Risk 1: Large pages overwhelm output
- **Description**: Complex pages with thousands of DOM nodes produce multi-MB output exceeding context window.
- **Mitigation**: Hard cap at 5,000 nodes processed. Total output capped at 50KB. Tables/lists summarized (first 3 rows, first 5 items). Text truncated at 100 chars. These limits tested on complex pages (e.g., Gmail, Google Sheets) and provide sufficient signal while staying within limits.

### Risk 2: UIDs not stable across snapshots
- **Description**: Same page visited twice produces different UIDs for same elements.
- **Mitigation**: UIDs assigned in deterministic tree traversal order (depth-first, left-to-right). Same DOM structure produces same UIDs. However, if page mutates between snapshots (dynamic content), UIDs may shift. This is acceptable -- UIDs are snapshot-scoped, not session-scoped.

### Risk 3: Generated CSS selectors don't match
- **Description**: Selector built for UID'd element doesn't actually select that element (due to dynamic attributes or selector generation bug).
- **Mitigation**: After generating selector, validate it by querying DOM (`document.querySelector(selector)`) and checking it returns expected element. If validation fails, fall back to positional selector (guaranteed to work but fragile).

### Risk 4: Main thread blocking on large traversals
- **Description**: Traversing 5,000 nodes takes > 50ms continuous main thread time, degrading browsing.
- **Mitigation**: Tree traversal yields control every 500 nodes via `setTimeout(0)` to allow browser to handle pending events. This keeps continuous blocking < 50ms while allowing full traversal to complete.

### Risk 5: Sensitive text exposed in accessible names
- **Description**: Accessible names may contain user email addresses, phone numbers, or PII displayed on page.
- **Mitigation**: Data stays on localhost (never leaves machine). Equivalent exposure to existing `analyze({what: "dom"})`. For pages with sensitive data, AI can use `scope` parameter to limit tree to non-sensitive regions. No additional redaction beyond existing privacy layer.

## Dependencies

### Depends on:
- Existing query dispatch infrastructure (`pending-queries` polling, content.js message bridge, inject.js execution context)
- `dom-queries.js` module for shared constants and utilities (element visibility check, text truncation limits)

### Depended on by:
- AI agents needing to identify interactive elements before issuing `interact` commands
- The `uidMap` output designed to bridge `observe` and `interact` tools

## Performance Considerations

| Metric | Target | Implementation notes |
|--------|--------|---------------------|
| Traversal time (typical page, <500 nodes) | < 100ms | Uses native TreeWalker, minimal computation per node |
| Traversal time (complex page, 2000+ nodes) | < 500ms | Yields control every 500 nodes to prevent blocking |
| Response size (typical page) | < 10KB | Text format, truncation, summarization |
| Response size (complex page) | < 50KB | Hard cap prevents context overflow |
| Memory impact during traversal | < 2MB | Transient allocation, GC'd after response |
| Main thread blocking | < 50ms continuous | Yields via setTimeout(0) every 500 nodes |

## Security Considerations

**Data captured**: Accessible names (text content, labels), ARIA attributes, element roles, CSS selectors. No raw HTML, no attribute values beyond ARIA-related ones, no computed styles.

**Redaction**: Input values for password fields (`type="password"`) and fields with `autocomplete` hints containing "password", "cc-", or "secret" replaced with `[REDACTED]`. Follows existing `SENSITIVE_INPUT_TYPES` constant pattern.

**Privacy implications**: Accessible name may contain user-visible text (email addresses displayed on page). Equivalent to `analyze({what: "dom"})` exposure. Data stays on localhost, acceptable.

**Attack surface**: No change. Feature reads DOM (read-only). Does not execute arbitrary code, inject scripts, or modify page state. Traversal runs in inject.js in same security context as existing DOM queries.

**UID stability**: UIDs derived from DOM structure, not from sensitive content. Attacker cannot learn private data from observing UIDs.
