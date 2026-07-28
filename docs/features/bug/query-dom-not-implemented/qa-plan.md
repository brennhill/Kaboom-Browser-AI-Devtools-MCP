---
feature: query-dom-not-implemented
doc_type: qa-plan
feature_id: bug-query-dom-not-implemented
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Query DOM Not Implemented (Bug Fix)

> How to test the query_dom bug fix. Includes code-level testing and human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Message forwarding logic in background.js
- [ ] Background.js receives DOM query and forwards to content script on tracked tab
- [ ] Background.js rejects DOM query when no tab is tracked (returns clear error)
- [ ] Background.js handles content script timeout (5s) and posts error result
- [ ] Background.js posts successful query results to server
- [ ] Content script receives DOM_QUERY message and forwards to inject.js
- [ ] Inject.js receives DOM_QUERY and calls executeDOMQuery()
- [ ] Query results flow back through content → background → server

**Integration tests:** End-to-end query execution
- [ ] MCP tool analyze({what: "dom", selector: "h1"}) returns real h1 elements
- [ ] Universal selector "*" returns capped results (50 elements max), not empty
- [ ] Complex selector "button[data-testid], a.nav-link" returns matching elements
- [ ] Invalid selector returns error with meaningful message
- [ ] Empty results only when selector genuinely matches nothing
- [ ] url and pageTitle fields populated with actual page data
- [ ] Query on page with 100+ matching elements returns max 50 with correct totalMatchCount

**Edge case tests:** Error scenarios
- [ ] Query when no tab tracked returns error: "No tab is currently tracked"
- [ ] Query on tab without content script returns error: "Content script not loaded"
- [ ] Query timeout after 5 seconds returns timeout error
- [ ] Query on page with shadow DOM returns results from shadow roots
- [ ] Query on page with iframes scopes to main frame (existing behavior)
- [ ] Concurrent queries queue correctly and return individual results

### Security/Compliance Testing

**Data leak tests:** Verify no sensitive data exposed
- [ ] Query results do not include event listeners or executable code
- [ ] Text content truncation works (max 500 chars per element)
- [ ] Style attributes sanitized (no javascript: URLs)

**Permission tests:** Verify only authorized access
- [ ] Queries only execute on tracked tab, not arbitrary tabs
- [ ] Cross-origin iframe content not accessible (existing behavior maintained)

---

## Human UAT Walkthrough

### Scenario 1: Basic Query Works
1. Setup:
   - Start Kaboom server: `./dist/kaboom`
   - Load Chrome with extension
   - Navigate to <https://example.com>
   - Start tracking the tab via interact tool
2. Steps:
   - [ ] Call MCP tool: `analyze({what: "dom", selector: "h1"})`
   - [ ] Observe response
3. Expected Result: Response contains:
   ```json
   {
     "totalMatchCount": 1,
     "returnedMatchCount": 1,
     "url": "https://example.com/",
     "pageTitle": "Example Domain",
     "matches": [
       {
         "tag": "h1",
         "text": "Example Domain",
         "attributes": {},
         "bboxPixels": {"x": 347, "y": 151, "width": 265, "height": 37}
       }
     ]
   }
   ```
4. Verification:
   - [ ] totalMatchCount is 1, not 0
   - [ ] matches array has real element data
   - [ ] url and pageTitle are populated

### Scenario 2: Universal Selector Returns Results
1. Setup: Same as Scenario 1
2. Steps:
   - [ ] Call MCP tool: `analyze({what: "dom", selector: "*"})`
   - [ ] Observe response
3. Expected Result:
   - [ ] totalMatchCount > 0 (page has elements)
   - [ ] returnedMatchCount = 50 (capped at max)
   - [ ] matches array has 50 elements
   - [ ] url and pageTitle populated
4. Verification: This proves the feature is actually working, not returning fake empty results

### Scenario 3: No Tracked Tab Error
1. Setup:
   - Start Kaboom server
   - DO NOT track any tab
2. Steps:
   - [ ] Call MCP tool: `analyze({what: "dom", selector: "button"})`
   - [ ] Observe response
3. Expected Result: Error response with clear message
4. Verification: Error says "No tab is currently tracked" (not "not_implemented")

### Scenario 4: Invalid Selector Handling
1. Setup: Same as Scenario 1 (tracked tab)
2. Steps:
   - [ ] Call MCP tool: `analyze({what: "dom", selector: "[invalid::syntax"})`
   - [ ] Observe response
3. Expected Result: Error response with CSS selector syntax error
4. Verification: Error is about selector syntax, not implementation

### Scenario 5: Complex Selector Works
1. Setup: Navigate to a page with multiple element types (e.g., GitHub.com)
2. Steps:
   - [ ] Call MCP tool: `analyze({what: "dom", selector: "button, a[href]"})`
   - [ ] Observe response
3. Expected Result:
   - [ ] totalMatchCount > 0
   - [ ] matches array contains both button and anchor elements
   - [ ] Each element has correct tag name

### Scenario 6: Empty Results Are Genuine
1. Setup: Navigate to https://example.com
2. Steps:
   - [ ] Call MCP tool: `analyze({what: "dom", selector: ".nonexistent-class"})`
   - [ ] Observe response
3. Expected Result:
   ```json
   {
     "totalMatchCount": 0,
     "returnedMatchCount": 0,
     "url": "https://example.com/",
     "pageTitle": "Example Domain",
     "matches": []
   }
   ```
4. Verification: Empty results are OK when selector genuinely matches nothing, BUT url/pageTitle must still be populated

---

## Regression Testing

### Must Not Break

- [ ] Accessibility queries still work (`generate({what: "query_accessibility"})`)
- [ ] Page info queries still work (`observe({what: "page"})`)
- [ ] Network queries still work (`observe({what: "network_waterfall"})`)
- [ ] Other extension message types still handled correctly
- [ ] Tab tracking still works
- [ ] Content script injection on new tabs still works

### Regression Test Steps

1. Run existing extension test suite: `node --test tests/extension/*.test.js`
2. Verify all 20+ DOM query tests pass
3. Verify accessibility query tests pass
4. Test DOM query, then accessibility query in sequence (no interference)
5. Test with multiple page navigations (content script re-injection)

---

## Performance/Load Testing

### Query execution time:
- [ ] Simple selector (single element): < 50ms
- [ ] Complex selector (50 elements): < 200ms
- [ ] Universal selector (capped at 50): < 500ms

### Message passing overhead:
- [ ] Background → Content → Inject: < 10ms total

### No memory leaks:
- [ ] Run 100 consecutive queries
- [ ] Check extension memory usage (should not grow unbounded)
- [ ] Verify no message listeners accumulate

### Timeout handling:
- [ ] Freeze page (via DevTools), send query
- [ ] Verify timeout error returned after 5 seconds
- [ ] Verify subsequent queries still work (no permanent hang)
