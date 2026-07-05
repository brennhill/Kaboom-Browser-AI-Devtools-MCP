---
feature: missing-tab-id
doc_type: qa-plan
feature_id: bug-missing-tab-id
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Missing Tab ID in MCP Responses (Bug Fix)

> How to test the tabId inclusion bug fix. Includes code-level testing and human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** tabId in responses
- [ ] Log entry includes tabId when posted with tabId
- [ ] Network entry includes tabId when posted with tabId
- [ ] WebSocket event includes tabId when posted with tabId
- [ ] Enhanced action includes tabId when posted with tabId
- [ ] Entry without tabId (legacy) handled gracefully (null or omitted)
- [ ] observe() response serialization includes tabId field

**Integration tests:** End-to-end with multiple tabs
- [ ] Open 2 tabs, post events from each, observe() returns entries with correct tabIds
- [ ] Filter by tabId: observe({what: "logs", tabId: 101}) returns only tab 101 entries
- [ ] Filter with no matches returns empty array with metadata
- [ ] Metadata includes currently tracked tab
- [ ] tabId appears in all observe modes (errors, logs, network, WebSocket, actions)

**Edge case tests:** Error scenarios
- [ ] Entry with missing tabId doesn't crash response builder
- [ ] Filter by non-existent tabId returns empty array (not error)
- [ ] Filter with invalid tabId type (string instead of int) handled gracefully
- [ ] Multiple tabs posting simultaneously: all tabIds correct (no race conditions)
- [ ] Tab closed but data in buffer: tabId still returned correctly

### Security/Compliance Testing

**Data leak tests:** Verify no new security issues
- [ ] tabId is just an integer, no sensitive data leaked
- [ ] tabId doesn't expose cross-origin data (already captured per design)

---

## Human UAT Walkthrough

### Scenario 1: Basic tabId Inclusion
1. Setup:
   - Start Kaboom server: `./dist/kaboom`
   - Load Chrome with extension
   - Open Tab A, navigate to <https://example.com>
   - Note Tab A's ID (visible in chrome://extensions developer mode console)
2. Steps:
   - [ ] Trigger a console error in Tab A
   - [ ] Call MCP tool: `observe({what: "errors"})`
   - [ ] Observe response
3. Expected Result:
   ```json
   {
     "entries": [
       {
         "message": "TypeError: ...",
         "tabId": 101,
         ...
       }
     ]
   }
   ```
4. Verification:
   - [ ] tabId field is present
   - [ ] tabId value matches Tab A's ID

### Scenario 2: Multiple Tabs with Different tabIds
1. Setup:
   - Open Tab A (id: 101) with <https://example.com>
   - Open Tab B (id: 102) with <https://other.com>
   - Each tab triggers different console errors
2. Steps:
   - [ ] Call MCP tool: `observe({what: "errors"})`
   - [ ] Observe response
3. Expected Result:
   ```json
   {
     "entries": [
       {
         "message": "Error from Tab A",
         "url": "https://example.com",
         "tabId": 101
       },
       {
         "message": "Error from Tab B",
         "url": "https://other.com",
         "tabId": 102
       }
     ],
     "metadata": {
       "currently_tracked_tab": 101
     }
   }
   ```
4. Verification:
   - [ ] Both errors have different tabIds
   - [ ] tabIds match actual Chrome tab IDs
   - [ ] Metadata shows which tab is tracked

### Scenario 3: Filter by Specific tabId
1. Setup: Same as Scenario 2 (2 tabs with errors)
2. Steps:
   - [ ] Call MCP tool: `observe({what: "errors", tabId: 102})`
   - [ ] Observe response
3. Expected Result:
   ```json
   {
     "entries": [
       {
         "message": "Error from Tab B",
         "url": "https://other.com",
         "tabId": 102
       }
     ],
     "metadata": {
       "filtered_by_tab": 102,
       "total_entries_all_tabs": 2,
       "returned_entries": 1
     }
   }
   ```
4. Verification:
   - [ ] Only Tab B's error returned
   - [ ] Metadata shows filter was applied

### Scenario 4: Network Requests with tabId
1. Setup: Open 2 tabs, each makes API requests
2. Steps:
   - [ ] Tab A: fetch('<https://api.example.com/data>')
   - [ ] Tab B: fetch('<https://api.other.com/users>')
   - [ ] Call MCP tool: `observe({what: "network_waterfall"})`
   - [ ] Observe response
3. Expected Result:
   ```json
   {
     "entries": [
       {
         "url": "https://api.example.com/data",
         "method": "GET",
         "status": 200,
         "tabId": 101
       },
       {
         "url": "https://api.other.com/users",
         "method": "GET",
         "status": 200,
         "tabId": 102
       }
     ]
   }
   ```
4. Verification: Network requests have correct tabIds

### Scenario 5: Filter by tabId with No Matches
1. Setup: Only Tab A (id: 101) has errors
2. Steps:
   - [ ] Call MCP tool: `observe({what: "errors", tabId: 999})`
   - [ ] Observe response
3. Expected Result:
   ```json
   {
     "entries": [],
     "metadata": {
       "filtered_by_tab": 999,
       "total_entries_all_tabs": 5,
       "returned_entries": 0
     }
   }
   ```
4. Verification: Empty result with clear explanation

### Scenario 6: WebSocket Events with tabId
1. Setup: Open tab with WebSocket connection
2. Steps:
   - [ ] Page opens WebSocket: `new WebSocket("ws://localhost:3000")`
   - [ ] Call MCP tool: `observe({what: "websocket_events"})`
   - [ ] Observe response
3. Expected Result:
   ```json
   {
     "entries": [
       {
         "type": "open",
         "url": "ws://localhost:3000",
         "timestamp": "...",
         "tabId": 101
       }
     ]
   }
   ```
4. Verification: WebSocket events include tabId

### Scenario 7: Currently Tracked Tab in Metadata
1. Setup: Track Tab A (id: 101)
2. Steps:
   - [ ] Call MCP tool: `observe({what: "page"})`
   - [ ] Observe response
3. Expected Result:
   ```json
   {
     "url": "https://example.com",
     "title": "Example Domain",
     "tabId": 101,
     "tracking_active": true
   }
   ```
4. Verification: Currently tracked tabId is clear

---

## Regression Testing

### Must Not Break

- [ ] observe() without tabId filter still works (returns all tabs)
- [ ] All observe modes work correctly (errors, logs, network, WebSocket)
- [ ] Response structure unchanged (tabId added, other fields same)
- [ ] Performance not degraded (< 10ms overhead for filter)
- [ ] Backward compatibility: old clients ignore tabId field (unknown field)

### Regression Test Steps

1. Run existing server test suite: `make test`
2. Verify all observe modes return data correctly
3. Test observe() on single tab (no filter) → all data returned
4. Test observe() with filter on non-existent tab → empty array
5. Verify response times unchanged (< 200ms for typical observe())

---

## Performance/Load Testing

### tabId inclusion overhead:
- [ ] observe() response time with tabId: < 5ms overhead vs without
- [ ] Response size increase: ~10 bytes per entry (acceptable)

### Filtering overhead:
- [ ] Filter by tabId on 1000 entries: < 10ms
- [ ] Filter by tabId on 100 entries: < 2ms
- [ ] No indexing needed (linear scan acceptable for ring buffer size)

### Multi-tab scenario:
- [ ] 5 tabs, each with 100 log entries
- [ ] observe({what: "logs"}) returns 500 entries with tabIds: < 300ms
- [ ] observe({what: "logs", tabId: 103}) returns 100 entries: < 150ms

---

## Schema Validation

### Response schema:
- [ ] tabId field is integer type (not string)
- [ ] tabId field is optional/nullable (backward compatibility)
- [ ] tabId appears in all observe modes:
  - [ ] errors
  - [ ] logs
  - [ ] network_waterfall
  - [ ] network_bodies
  - [ ] websocket_events
  - [ ] enhanced_actions
- [ ] Metadata includes currently_tracked_tab (integer)
- [ ] Filter metadata includes: filtered_by_tab, total_entries_all_tabs, returned_entries

### Input schema:
- [ ] observe tool accepts optional tabId parameter (integer)
- [ ] Invalid tabId type handled gracefully (error message)
- [ ] Missing tabId (no filter) works correctly

---

## Documentation Verification

### User documentation:
- [ ] tabId field documented in observe response schema
- [ ] tabId filter parameter documented in observe input schema
- [ ] Examples showing tabId usage added to tool descriptions
- [ ] Caveat documented: tabId is not stable across browser restarts
- [ ] Caveat documented: tabId only available for telemetry captured after fix

### Developer documentation:
- [ ] Extension must send tabId with all events (documented)
- [ ] Server stores tabId in all ring buffers (documented)
- [ ] Filter logic documented (server-side, linear scan)
