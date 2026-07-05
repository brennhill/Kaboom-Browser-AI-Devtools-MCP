---
feature: query-dom-not-implemented
status: in-progress
tool: generate
mode: dom
version: 0.7.12
doc_type: product-spec
feature_id: bug-query-dom-not-implemented
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Query DOM Not Implemented (Bug Fix)

## Problem Statement

The MCP schema advertises a `analyze({what: "dom"})` action under the `generate` tool, but when users attempt to call it, they receive misleading empty results instead of actual DOM query data. Even universal selectors like `"*"` return 0 matches on pages that clearly contain DOM elements.

### Current User Experience:
1. User calls `analyze({what: "dom", selector: "h1"})`
2. Receives schema-compliant response: `{totalMatchCount: 0, matches: null, url: "", pageTitle: ""}`
3. User assumes their selector is wrong or the page has no matching elements
4. User wastes time debugging selector syntax when the real issue is the feature isn't implemented

**Root Cause:** The backend query executor is built, the extension DOM query executor is built (`executeDOMQuery` in `dom-queries.js`), but the message forwarding chain between background.js → content.js → inject.js is incomplete. Background.js has a TODO comment and returns `{success: false, error: 'not_implemented'}` instead of forwarding the query to the content script.

## Solution

Complete the message forwarding chain so that DOM queries flow from the MCP tool through the server, to background.js, to content.js, to inject.js, where `executeDOMQuery()` executes and returns real results.

### Fixed User Experience:
1. User calls `analyze({what: "dom", selector: "h1"})`
2. Query is forwarded to the content script on the tracked tab
3. `executeDOMQuery()` runs and returns actual DOM elements
4. User receives real data: `{totalMatchCount: 1, returnedMatchCount: 1, matches: [{tag: "h1", text: "Example Domain", ...}]}`

## Requirements

1. **Complete Message Forwarding:** Wire background.js to forward `type: "dom"` queries to content.js using `chrome.tabs.sendMessage`
2. **Content Script Handler:** Add message listener in content.js for `DOM_QUERY` messages
3. **Inject Script Executor:** Ensure inject.js can access and call `executeDOMQuery()` from dom-queries.js
4. **Result Posting:** Post query results back to the server via `postQueryResult()`
5. **Error Handling:** If content script fails or times out, return structured error (not fake empty results)
6. **Tab Validation:** Only execute queries on the tracked tab; return error if no tab is tracked
7. **Schema Compliance:** Results must match the existing query_dom response schema (url, pageTitle, totalMatchCount, returnedMatchCount, matches array)

## Out of Scope

- Adding new query capabilities beyond what `executeDOMQuery()` already supports
- Changing the DOM query schema or parameters
- Performance optimizations (already bounded by `DOM_QUERY_MAX_ELEMENTS`)
- Multi-tab query support (single-tab tracking is by design)

## Success Criteria

1. `analyze({what: "dom", selector: "h1"})` returns actual h1 elements from the page
2. Universal selector `"*"` returns real DOM elements (up to max limit)
3. Empty results only occur when selector genuinely matches nothing (not due to implementation gap)
4. `url` and `pageTitle` fields are populated with actual page data
5. Error responses clearly indicate when queries can't execute (e.g., no tracked tab)
6. All 20+ existing extension tests for DOM queries pass

## User Workflow

### Before Fix:
1. User calls `analyze({what: "dom", selector: "button"})`
2. Receives 0 matches with empty url/pageTitle
3. User debugs selector, wastes time

### After Fix:
1. User calls `analyze({what: "dom", selector: "button"})`
2. Query executes in tracked tab's DOM
3. User receives real button elements with their properties
4. User can immediately analyze the DOM structure

## Examples

### Example 1: Query for Headings
```json
{
  "tool": "generate",
  "arguments": {
    "what": "dom",
    "selector": "h1, h2, h3"
  }
}
```

#### Before Fix Response:
```json
{
  "totalMatchCount": 0,
  "returnedMatchCount": 0,
  "matches": null,
  "url": "",
  "pageTitle": ""
}
```

#### After Fix Response:
```json
{
  "totalMatchCount": 12,
  "returnedMatchCount": 12,
  "url": "https://example.com/docs",
  "pageTitle": "Documentation - Example",
  "matches": [
    {
      "tag": "h1",
      "text": "Getting Started",
      "attributes": {"class": "doc-title"},
      "bboxPixels": {"x": 20, "y": 100, "width": 800, "height": 48}
    },
    // ... more matches
  ]
}
```

### Example 2: No Tracked Tab Error
```json
{
  "tool": "generate",
  "arguments": {
    "what": "dom",
    "selector": "button"
  }
}
```

#### Response (No Tab Tracked):
```json
{
  "error": "No tab is currently tracked. Use interact({action: 'track_tab'}) first."
}
```

---

## Notes

- Existing implementation of `executeDOMQuery()` in `extension/lib/dom-queries.js` is complete and tested
- Backend server query executor in `cmd/browser-agent/queries.go` already handles DOM query creation and result posting
- Only missing piece is the message forwarding in `extension/background.js` lines 2634-2640 (currently returns 'not_implemented')
- Fix should follow the same pattern as accessibility queries (which work correctly)
- Related specs: See `docs/features/feature/query-dom/product-spec.md` for the original feature specification
