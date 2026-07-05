---
feature: missing-tab-id
status: in-progress
doc_type: tech-spec
feature_id: bug-missing-tab-id
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Missing Tab ID in MCP Responses (Bug Fix)

> Plain language only. No code. Describes HOW to add tabId to MCP responses.

## Architecture Overview

Kaboom's data flow with tabs:
1. Content script runs in each tab, captures telemetry
2. Content script attaches `tabId` when posting events to server
3. Server stores events in ring buffers (logs, network, WebSocket, etc.)
4. MCP tools query ring buffers and return data
5. **Current gap:** tabId is stored but not included in MCP response payloads

**The Fix:** Modify MCP response builders to include the stored `tabId` field in every entry.

## Key Components

- **Ring buffer structures:** In-memory storage for logs, network, WebSocket, etc.
- **POST endpoint handlers:** Receive events from extension, store with tabId
- **MCP observe handlers:** Build responses from ring buffers
- **MCP generate handlers:** Build artifacts (reproductions, tests) from telemetry
- **Tab tracking state:** Currently tracked tabId stored separately in server memory
- **Filter logic:** Apply tabId filter when building responses (optional)

## Data Flows

### Current (Missing tabId) Flow
```
Content script → POST /logs with {message: "...", tabId: 101}
→ Server stores: {message: "...", tabId: 101} in logs buffer
→ MCP observe({what: "logs"}) → Server queries buffer → builds response
→ Response: [{message: "..."}] (tabId dropped)
```

### Fixed Flow
```
Content script → POST /logs with {message: "...", tabId: 101}
→ Server stores: {message: "...", tabId: 101} in logs buffer
→ MCP observe({what: "logs"}) → Server queries buffer → builds response
→ Response: [{message: "...", tabId: 101}] (tabId included)
```

### Filter by tabId Flow
```
MCP observe({what: "logs", tabId: 101})
→ Server queries buffer → filters entries where tabId == 101
→ Response: [{message: "from tab 101", tabId: 101}]
```

## Implementation Strategy

### Step 1: Verify tabId Is Stored
Check the ring buffer storage:
- Confirm that POST endpoints (/logs, /network-bodies, /websocket-events) store tabId
- Verify the tabId field is present in stored data structures
- Check if tabId is an integer (Chrome's tab ID) or string
- Verify tabId is set for all event types (logs, network, WebSocket, actions)

### Step 2: Identify MCP Response Builders
Locate the code that builds MCP responses:
- Find the observe tool handlers for each "what" mode (errors, logs, network_waterfall, etc.)
- Identify where entries are serialized from ring buffer to JSON
- Check if response builders iterate over ring buffer entries and construct output
- Verify the structure of output entries (which fields are included)

### Step 3: Add tabId to Response Entries
Modify response builders:
- For each entry in the response, include the stored tabId field
- Handle missing tabId gracefully (old entries before fix may not have it)
- If tabId is null/missing, either omit the field or set to null (document choice)
- Ensure tabId is included for all observe modes (errors, logs, network_waterfall, network_bodies, websocket_events, enhanced_actions, etc.)

### Step 4: Add tabId Filter Parameter
Add filtering capability:
- Update observe tool schema to accept optional `tabId` parameter (integer)
- In response builders, if `tabId` parameter provided, filter entries where entry.tabId == param.tabId
- If no entries match, return empty array with metadata explaining filter
- Ensure filter doesn't break when entries lack tabId (skip those entries)

### Step 5: Add Tracked Tab Metadata
Include currently tracked tab in metadata:
- Server maintains "currently tracked tab" state (set via interact tool)
- Include this in response metadata: `{currently_tracked_tab: 101}`
- Helps AI understand which tab is active for queries/actions
- Metadata should be in all observe responses (optional for generate)

### Step 6: Update Tool Schemas
Document the new field:
- Add tabId to observe tool response schema documentation
- Add tabId filter parameter to observe tool input schema
- Mark tabId as optional (nullable) in responses (backward compatibility)
- Add examples showing tabId usage to MCP tool descriptions

### Step 7: Test Backward Compatibility
Ensure old data doesn't break:
- If ring buffer contains entries without tabId (from before fix), handle gracefully
- Don't crash or error when tabId is missing
- Either omit tabId field from response or set to null (consistent choice)
- Document that tabId is only available for telemetry captured after fix

## Edge Cases & Assumptions

### Edge Case 1: tabId Missing from Stored Entry
**Handling:** When building response, check if entry.tabId exists. If not, omit field or set to null. Document that tabId is only available for recent telemetry.

### Edge Case 2: Filter by tabId with No Matches
**Handling:** Return empty array with metadata: `{filtered_by_tab: 101, total_entries_all_tabs: 45, returned_entries: 0}`. Makes clear that filter worked but no data matched.

### Edge Case 3: Multiple Tabs Tracked Simultaneously
**Handling:** Current design tracks only one tab at a time. tabId filter allows querying any tab's data (not just tracked tab). Metadata shows which single tab is currently tracked.

### Edge Case 4: tabId for Server-Generated Events
**Handling:** Some events may originate from server-side logic (e.g., query results). These don't have a tab context. Either set tabId to null or omit it.

### Edge Case 5: Tab Closed, Data Still in Buffer
**Handling:** tabId may reference a closed tab. This is fine; tabId is just an identifier. Don't validate if tab still exists (expensive and unnecessary).

### Assumption 1: Extension Always Sends tabId
We assume content scripts always include tabId when posting events. Verify this in extension code; if not, fix extension-side first.

### Assumption 2: tabId Is an Integer
We assume Chrome's tab IDs are integers. Server should store as integer, not string, for efficient filtering.

### Assumption 3: tabId Is Unique Per Browser Instance
Within a single browser session, tab IDs are unique. Across restarts, IDs may repeat. This is acceptable; tabId is session-scoped.

## Risks & Mitigations

### Risk 1: Breaking Existing Clients
**Mitigation:** Adding tabId to responses is backward-compatible (clients can ignore unknown fields). Ensure field is consistently named "tabId" (camelCase or snake_case per Kaboom convention).

### Risk 2: Filter Performance Degradation
**Mitigation:** Filtering by tabId is O(N) scan of ring buffer. For typical buffer sizes (1000 entries), this is < 1ms. Acceptable overhead. Don't add complex indexing (premature optimization).

### Risk 3: Confusion About tabId Stability
**Mitigation:** Document clearly that tabId is Chrome's internal ID, not stable across restarts. Users should not persist tabId in external systems.

### Risk 4: Missing tabId in Extension Posts
**Mitigation:** If extension doesn't send tabId, server can't include it. Verify extension-side first. If missing, fix extension to send tabId with every event.

## Dependencies

- **Existing:** Extension content scripts post events with tabId (verify this)
- **Existing:** Server stores tabId in ring buffers (verify this)
- **Existing:** Tab tracking state in server (currently tracked tab)
- **New:** Include tabId in MCP response serialization
- **New:** Add tabId filter parameter to observe tool
- **New:** Add tracked tab metadata to responses

## Performance Considerations

- tabId inclusion in response: no overhead (field already in memory)
- tabId filtering: O(N) scan of ring buffer, < 1ms for typical buffers
- No impact on capture performance (tabId already sent by extension)
- Response size increase: ~10 bytes per entry (`"tabId": 101,`)
- Filter performance scales linearly with buffer size (acceptable)

## Security Considerations

- **tabId is not sensitive:** Chrome's tab ID is not secret. Safe to expose via MCP.
- **No cross-tab data leak:** tabId filter allows querying any tab, but Kaboom already captures all tabs (no new exposure).
- **No privilege escalation:** Knowing a tabId doesn't grant access to the tab itself (that's Chrome's job).
- **Localhost-only:** As always, MCP is localhost-only. tabId never leaves the machine.

## Test Plan Reference

See qa-plan.md for detailed testing strategy. Key test scenarios:
1. observe() responses include tabId for all entries
2. Filter by tabId returns only entries from that tab
3. Backward compatibility: old entries without tabId don't break responses
4. Metadata includes currently tracked tab
5. tabId appears in logs, network, WebSocket, actions
6. Filter with no matches returns empty array with clear metadata
7. Regression: all observe modes still work correctly
