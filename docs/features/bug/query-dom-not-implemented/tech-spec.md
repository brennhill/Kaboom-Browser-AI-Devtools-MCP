---
feature: query-dom-not-implemented
status: in-progress
doc_type: tech-spec
feature_id: bug-query-dom-not-implemented
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Query DOM Not Implemented (Bug Fix)

> Plain language only. No code. Describes HOW to fix the implementation gap.

## Architecture Overview

Kaboom uses a request/response pattern for on-demand queries:
1. MCP tool creates a pending query in the server
2. Extension polls for pending queries
3. Background service worker receives query via `/poll-for-queries`
4. Background worker forwards query to content script on tracked tab
5. Content script forwards to inject script which executes the query
6. Results flow back: inject → content → background → server
7. MCP tool retrieves completed query results

**The Bug:** Step 4 is incomplete. Background.js receives DOM queries but returns `{success: false, error: 'not_implemented'}` instead of forwarding them to the content script.

## Key Components

- **background.js (lines 2634-2640):** Currently has TODO comment and returns "not_implemented" error for DOM queries
- **content.js:** Needs a message listener for `DOM_QUERY` type messages (similar to existing A11Y_AUDIT handler)
- **inject.js:** Already has access to `executeDOMQuery()` via dom-queries.js import
- **dom-queries.js:** Contains complete `executeDOMQuery()` implementation (no changes needed)
- **queries.go (server):** Already handles DOM query creation and result retrieval (no changes needed)

## Data Flows

### Current (Broken) Flow
```
MCP Tool → Server creates pending query → Extension polls → background.js receives query
→ background.js returns 'not_implemented' → Server wraps as empty results → MCP Tool
```

### Fixed Flow
```
MCP Tool → Server creates pending query → Extension polls → background.js receives query
→ background.js sends chrome.tabs.sendMessage to content script
→ content.js forwards to inject.js
→ inject.js calls executeDOMQuery()
→ Results: inject.js → content.js → background.js → server → MCP Tool
```

## Implementation Strategy

### Step 1: Modify Background.js Query Handler
Replace the TODO block (lines 2634-2640) with proper message forwarding logic:
- Check that a tab is currently tracked (reject if not)
- Use `chrome.tabs.sendMessage()` to send query to content script
- Message structure: `{type: 'DOM_QUERY', params: query.params}`
- Wait for response from content script
- Post result to server using `postQueryResult(serverUrl, query.id, 'dom', result)`
- Handle timeout if content script doesn't respond within 5 seconds

### Step 2: Add Content Script Message Listener
In content.js, add a listener for `DOM_QUERY` messages:
- Receive message from background.js with query parameters
- Forward to inject.js by posting message to window
- Wait for response from inject.js
- Return result to background.js

### Step 3: Ensure Inject Script Can Execute
In inject.js, add listener for DOM_QUERY messages:
- Receive query parameters from content.js
- Call `executeDOMQuery(params)` (already implemented in dom-queries.js)
- Return results to content.js

### Step 4: Verify Result Format
Ensure returned data matches the schema expected by server:
- `url` - current page URL
- `pageTitle` - document title
- `totalMatchCount` - total elements matching selector
- `returnedMatchCount` - actual elements returned (capped by max limit)
- `matches` - array of serialized DOM elements

## Edge Cases & Assumptions

### Edge Case 1: No Tracked Tab
**Handling:** Background.js should check for tracked tab before sending message. If no tab is tracked, immediately post error result to server: `{success: false, error: 'No tab is currently tracked'}`

### Edge Case 2: Content Script Not Loaded
**Handling:** If `chrome.tabs.sendMessage()` fails because content script isn't injected, catch the error and post: `{success: false, error: 'Content script not loaded on tracked tab'}`

### Edge Case 3: Query Timeout
**Handling:** If content script doesn't respond within 5 seconds, post timeout error. This prevents queries from hanging indefinitely on frozen pages.

### Edge Case 4: Invalid Selector
**Handling:** Let `executeDOMQuery()` handle this. It will throw an error which should be caught and returned as `{success: false, error: errorMessage}`

### Assumption 1: Content Script Always Injected on Tracked Tab
We assume that if a tab is tracked, the content script is already injected. This is true because tracking is only enabled after content script loads.

### Assumption 2: Inject Script Has Access to dom-queries.js
We assume inject.js imports or includes dom-queries.js. Verify this import is present and executeDOMQuery is in scope.

### Assumption 3: Query Parameters Match executeDOMQuery Signature
Server-side query creation must pass the correct parameter structure: `{selector, include_styles, properties, include_children, max_depth}`

## Risks & Mitigations

### Risk 1: Message Passing Adds Latency
**Mitigation:** DOM queries are already async and best-effort. The message passing overhead (< 10ms) is negligible compared to query execution time. Document expected latency in tool description.

### Risk 2: Content Script Memory Leak
**Mitigation:** Ensure message listeners are registered once (not on every query). Use `chrome.runtime.onMessage.addListener` pattern correctly.

### Risk 3: Query Results Too Large
**Mitigation:** `executeDOMQuery()` already caps results at `DOM_QUERY_MAX_ELEMENTS` (50 elements) and truncates text at `DOM_QUERY_MAX_TEXT` (500 chars). No additional capping needed.

### Risk 4: Breaking Existing Accessibility Queries
**Mitigation:** Follow the exact same pattern as accessibility queries. Copy the message forwarding structure and adapt for DOM queries. Test that a11y queries still work after changes.

## Dependencies

- **Existing:** `executeDOMQuery()` in dom-queries.js (already implemented)
- **Existing:** Server-side query executor in queries.go (already implemented)
- **Existing:** Content script injection on tracked tabs (already working)
- **New:** Message listener in content.js for DOM_QUERY type
- **New:** Message forwarding in background.js query handler

## Performance Considerations

- Query execution time: 10-500ms depending on selector complexity and page size
- Message passing overhead: < 10ms total (background → content → inject → back)
- Result serialization: Already bounded by max elements/text limits
- No impact on main thread: queries run async and don't block browsing
- Query timeout: 5 seconds prevents indefinite hanging

## Security Considerations

- **DOM Access:** Inject script already has full DOM access (by design). No new permissions needed.
- **Query Scope:** Queries execute only on tracked tab, not all tabs (maintains single-tab isolation)
- **Selector Injection:** Selectors are passed as strings to `querySelectorAll()`. Malicious selectors can't execute code, only match elements.
- **Result Sanitization:** DOM elements are serialized to plain objects. No live DOM references leak to server.
- **Privacy:** Text content may contain PII. This is already a known constraint of DOM queries. Document in tool description.

## Test Plan Reference

See qa-plan.md for detailed testing strategy. Key test scenarios:
1. Query returns real DOM elements on working page
2. Universal selector returns capped results (not 0)
3. Invalid selector returns error (not fake empty results)
4. No tracked tab returns clear error
5. Content script not loaded returns error
6. Query timeout after 5 seconds
7. Regression: accessibility queries still work
8. Regression: page info queries still work
