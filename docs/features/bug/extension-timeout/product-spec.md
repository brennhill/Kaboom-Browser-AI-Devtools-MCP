---
feature: extension-timeout
status: in-progress
tool: all
mode: all
version: 0.7.12
doc_type: product-spec
feature_id: bug-extension-timeout
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec: Extension Timeout After 5-6 Operations (Bug Fix)

## Problem Statement

After performing 5-6 operations (DOM queries, accessibility audits, network captures, etc.), the Kaboom extension becomes unresponsive. Subsequent MCP tool calls hang, timeout, or return no data. The extension appears frozen and requires a browser restart to recover.

### Current User Experience:
1. User starts session, performs 3-4 operations successfully
2. After 5th or 6th operation, next operation hangs indefinitely
3. MCP tools timeout waiting for extension response
4. User must reload extension or restart browser
5. Workflow interrupted, telemetry lost

**Root Cause:** Possible causes:
- **Message queue backup:** Background service worker message queue fills up, stops processing
- **Memory leak:** Extension memory grows unbounded, triggers browser throttling
- **Event listener accumulation:** Multiple listeners registered on each operation, causing performance degradation
- **Ring buffer overflow:** Internal buffers in extension overflow and block new operations
- **Unhandled promise rejection:** Async operations fail but don't reject properly, leaving operations pending

## Solution

Identify and fix the resource leak or queue bottleneck that causes the extension to become unresponsive after several operations. Ensure the extension can handle unlimited consecutive operations without degradation.

### Fixed User Experience:
1. User performs 5-6 operations successfully
2. Continues to perform 10, 20, 50+ operations
3. All operations complete normally
4. No timeouts, no hangs, no need to restart
5. Consistent performance throughout session

## Requirements

1. **Investigate Message Queue:** Trace message handling in background.js to identify backup points
2. **Profile Memory Usage:** Measure extension memory after 1, 5, 10, 20 operations to identify leaks
3. **Audit Event Listeners:** Ensure listeners are registered once (not on every operation)
4. **Fix Resource Leaks:** Close connections, clear buffers, remove listeners when no longer needed
5. **Add Health Monitoring:** Log queue depth, memory usage, pending operations for debugging
6. **Timeout Enforcement:** Ensure operations timeout gracefully after 5-10 seconds (don't hang indefinitely)
7. **Backward Compatibility:** Fix must not break existing functionality or change API

## Out of Scope

- Adding new extension capabilities
- Changing the MCP tool schemas
- Performance optimizations beyond fixing the timeout bug
- Supporting multiple simultaneous operations (operations already queued)
- Persistent state recovery (after browser crash)

## Success Criteria

1. Extension handles 50+ consecutive operations without hanging
2. Memory usage remains bounded (< 100MB after 50 operations)
3. Operation latency stays consistent (no degradation over time)
4. Message queue never backs up indefinitely
5. No unhandled promise rejections in console
6. Extension health check passes after 50 operations
7. Browser DevTools show no memory leaks or accumulating listeners

## User Workflow

### Before Fix:
1. User performs 5 operations successfully
2. 6th operation hangs
3. User waits, sees timeout error
4. Reloads extension or browser

### After Fix:
1. User performs 5, 10, 20, 50 operations
2. All operations complete normally
3. Consistent performance throughout
4. No intervention needed

## Examples

### Example 1: Consecutive DOM Queries (Before Fix)

#### Operations:
1. `analyze({what: "dom", selector: "h1"})` → Success (2s)
2. `analyze({what: "dom", selector: "button"})` → Success (2s)
3. `analyze({what: "dom", selector: "a"})` → Success (2s)
4. `analyze({what: "dom", selector: "img"})` → Success (2s)
5. `analyze({what: "dom", selector: "div"})` → Success (3s, slower)
6. `analyze({what: "dom", selector: "span"})` → Timeout (30s, no response)

### Example 2: Consecutive DOM Queries (After Fix)

#### Operations:
1. `analyze({what: "dom", selector: "h1"})` → Success (2s)
2. `analyze({what: "dom", selector: "button"})` → Success (2s)
...
10. `analyze({what: "dom", selector: "section"})` → Success (2s)
...
50. `analyze({what: "dom", selector: "footer"})` → Success (2s)

All operations complete with consistent latency.

### Example 3: Mixed Operations (After Fix)

#### Operations:
1. `observe({what: "errors"})` → Success
2. `analyze({what: "dom"})` → Success
3. `observe({what: "network_bodies"})` → Success
4. `generate({action: "query_accessibility"})` → Success
5. `interact({action: "highlight"})` → Success
...
20. Mixed operations continue → All succeed

### Example 4: Health Check After 50 Operations

#### Request:
```json
{
  "tool": "configure",
  "arguments": {
    "action": "health"
  }
}
```

#### Response:
```json
{
  "status": "healthy",
  "operations_completed": 50,
  "memory_mb": 45,
  "message_queue_depth": 0,
  "pending_operations": 0,
  "uptime_seconds": 1200
}
```

---

## Notes

- Chrome Manifest V3 service workers have a 5-minute inactivity timeout (not the issue here, but important context)
- Service workers are terminated and restarted automatically by Chrome, which should clear state
- If the issue is service worker-specific, the fix may involve proper state cleanup on worker restart
- Message queue backup is the most likely culprit based on symptom pattern (works initially, then fails)
- Memory profiling tools: Chrome Task Manager, DevTools Memory panel, chrome://extensions with Developer Mode
- Related issues: Check if polling for pending queries accumulates handlers or connections
