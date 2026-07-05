---
feature: missing-tab-id
status: in-progress
tool: observe
mode: all
version: 0.7.12
doc_type: product-spec
feature_id: bug-missing-tab-id
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Missing Tab ID in MCP Responses (Bug Fix)

## Problem Statement

When users call `observe()` to retrieve browser telemetry, the response does not include a `tabId` field, making it impossible to filter or correlate data by browser tab. The extension's content script attaches `tabId` to captured events, but the server doesn't forward it in MCP responses.

### Current User Experience:
1. User has multiple tabs open with Kaboom extension
2. User calls `observe({what: "errors"})` to get console errors
3. Response contains errors from all tabs, but no `tabId` field
4. User cannot determine which errors came from which tab
5. User cannot filter telemetry by specific tab
6. Multi-tab workflows are not feasible

**Root Cause:** The extension's content.js correctly attaches `tabId` to telemetry events when posting to the server. The server receives `tabId` and stores it with the event. However, the MCP tool handlers (observe, generate) don't include `tabId` in their response payloads. The data is in the server's ring buffers, but not exposed via the API.

## Solution

Modify the server's MCP response handlers to include `tabId` in all telemetry entries. This applies to all observe modes (errors, logs, network_waterfall, network_bodies, websocket_events, etc.) and relevant generate outputs.

### Fixed User Experience:
1. User has multiple tabs open
2. User calls `observe({what: "errors"})`
3. Response includes `tabId` for each error entry
4. User can filter: `observe({what: "errors", tabId: 12345})`
5. User can correlate events across different data types by tabId
6. Multi-tab debugging workflows work correctly

## Requirements

1. **Include tabId in All Observe Responses:** Every entry in logs, errors, network_waterfall, network_bodies, websocket_events must include `tabId`
2. **Filter by tabId:** Add optional `tabId` parameter to observe tool to filter results by specific tab
3. **Backward Compatibility:** If `tabId` is missing from stored event (old data), handle gracefully (null or omit field)
4. **Tab Tracking Context:** Include currently tracked tabId in response metadata for context
5. **Document tabId Semantics:** Clarify that tabId is Chrome's internal tab identifier (integer), not stable across browser restarts
6. **Schema Compliance:** Add `tabId` field to existing schemas without breaking clients
7. **Performance:** Filtering by tabId should not add significant latency (< 10ms)

## Out of Scope

- Adding tabId to data types that don't have tab context (server health, configuration)
- Making tabId persistent across browser restarts (Chrome doesn't support this)
- Multi-tab correlation beyond simple tabId matching (e.g., session grouping)
- UI for tab management (extension is headless)
- Automatic tab switching based on queries (remains manual via interact tool)

## Success Criteria

1. All `observe()` responses include `tabId` for each entry
2. `observe({what: "errors", tabId: 12345})` returns only errors from tab 12345
3. `tabId` appears in logs, network requests, WebSocket events, enhanced actions
4. `tabId` value matches Chrome's internal tab ID (integer)
5. Currently tracked tabId available in response metadata
6. Backward compatibility: old entries without tabId don't cause errors
7. Documentation updated with tabId usage examples

## User Workflow

### Before Fix:
1. User opens 3 tabs: Tab A (id: 101), Tab B (id: 102), Tab C (id: 103)
2. Each tab generates errors
3. User calls `observe({what: "errors"})`
4. Receives errors from all tabs mixed together
5. Cannot identify which error is from which tab

### After Fix:
1. User opens 3 tabs: Tab A (id: 101), Tab B (id: 102), Tab C (id: 103)
2. Each tab generates errors
3. User calls `observe({what: "errors"})`
4. Receives errors with `tabId` field for each entry
5. User can see: error 1 is from tab 101, error 2 is from tab 102, etc.
6. User can filter: `observe({what: "errors", tabId: 102})` → only tab B errors

## Examples

### Example 1: Errors with tabId

#### Request:
```json
{
  "tool": "observe",
  "arguments": {
    "what": "errors"
  }
}
```

#### Before Fix Response:
```json
{
  "entries": [
    {
      "timestamp": "2026-01-28T10:00:00Z",
      "level": "error",
      "message": "TypeError: Cannot read property 'x' of null",
      "url": "https://example.com"
    },
    {
      "timestamp": "2026-01-28T10:00:05Z",
      "level": "error",
      "message": "ReferenceError: foo is not defined",
      "url": "https://other.com"
    }
  ]
}
```

#### After Fix Response:
```json
{
  "entries": [
    {
      "timestamp": "2026-01-28T10:00:00Z",
      "level": "error",
      "message": "TypeError: Cannot read property 'x' of null",
      "url": "https://example.com",
      "tabId": 101
    },
    {
      "timestamp": "2026-01-28T10:00:05Z",
      "level": "error",
      "message": "ReferenceError: foo is not defined",
      "url": "https://other.com",
      "tabId": 102
    }
  ],
  "metadata": {
    "currently_tracked_tab": 101
  }
}
```

### Example 2: Filter by tabId

#### Request:
```json
{
  "tool": "observe",
  "arguments": {
    "what": "network_waterfall",
    "tabId": 101
  }
}
```

#### Response:
```json
{
  "entries": [
    {
      "url": "https://example.com/api/data",
      "method": "GET",
      "status": 200,
      "timestamp": "2026-01-28T10:00:00Z",
      "tabId": 101
    }
  ],
  "metadata": {
    "filtered_by_tab": 101,
    "total_entries_all_tabs": 45,
    "returned_entries": 12
  }
}
```

### Example 3: Currently Tracked Tab Metadata

#### Request:
```json
{
  "tool": "observe",
  "arguments": {
    "what": "page"
  }
}
```

#### Response:
```json
{
  "url": "https://example.com",
  "title": "Example Domain",
  "tabId": 101,
  "tracking_active": true
}
```

---

## Notes

- `tabId` is Chrome's internal identifier (integer, e.g., 101, 102, 103)
- tabId is NOT stable across browser restarts (tab 101 today may be tab 205 tomorrow)
- tabId is NOT available for data captured before the fix (legacy entries won't have it)
- The currently tracked tab is stored separately and should be included in metadata
- Filtering by tabId is a server-side filter (not extension-side), applied when building MCP response
- Extension already attaches tabId via `chrome.tabs.getCurrent()` or message sender metadata
- Related specs: Tab tracking UX feature for managing which tab is tracked
