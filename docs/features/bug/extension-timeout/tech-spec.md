---
feature: extension-timeout
status: in-progress
doc_type: tech-spec
feature_id: bug-extension-timeout
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec: Extension Timeout After 5-6 Operations (Bug Fix)

> Plain language only. No code. Describes HOW to diagnose and fix the extension timeout issue.

## Architecture Overview

Kaboom extension architecture involves three components:
1. **Background service worker (background.js):** Message hub, polls server, forwards queries to content scripts
2. **Content script (content.js):** Bridge between page and background worker
3. **Inject script (inject.js):** Executes queries in page context, captures telemetry

Operations flow: MCP → Server → Background polls → Background forwards to Content → Content forwards to Inject → Results flow back.

**The Bug:** After 5-6 operations, this flow breaks down. Messages stop flowing, responses don't return, or operations hang indefinitely.

## Key Components

- **Message queue in background.js:** Handles incoming messages from content scripts and server polling
- **Polling loop:** Background worker polls `/pending-queries` every 2 seconds
- **Event listeners:** chrome.runtime.onMessage, chrome.tabs.sendMessage handlers
- **Pending operations map:** Tracks in-flight queries waiting for results
- **Content script message handlers:** Receives queries and posts results
- **Memory buffers:** Ring buffers for telemetry storage in background worker

## Data Flows

### Hypothesis 1: Message Queue Backup
```
Operation 1-4: Message arrives → queued → processed → response sent → queue cleared
Operation 5: Message arrives → queued → NOT processed (queue backed up) → no response
Operation 6+: Messages pile up in queue → extension unresponsive
```

### Hypothesis 2: Promise Rejection Not Handled
```
Operation 1-4: Query sent → content script responds → promise resolves
Operation 5: Query sent → content script errors → promise never rejects → operation hangs
Operation 6+: More promises hang → pending operations accumulate → queue blocks
```

### Hypothesis 3: Event Listener Accumulation
```
Operation 1: Add listener for query result
Operation 2: Add another listener (should reuse, but adds duplicate)
...
Operation 5: 5 listeners registered, all firing on messages → performance degrades
Operation 6: 6 listeners → chrome.runtime.onMessage overloaded → hangs
```

### Hypothesis 4: Memory Leak Triggers Throttling
```
Operation 1-4: Memory grows slowly (50MB → 70MB)
Operation 5: Memory exceeds threshold (80MB) → Chrome throttles service worker
Operation 6: Worker throttled → operations slow to a crawl → appear to hang
```

## Implementation Strategy

### Step 1: Reproduce the Issue
Set up a test environment:
- Start Kaboom server and extension
- Run a script that performs 10 consecutive DOM queries via MCP
- Monitor extension with Chrome DevTools (Console, Memory, Performance)
- Identify at which operation number the issue occurs
- Note symptoms: timeout, error message, memory spike, console warnings

### Step 2: Profile Memory Usage
Use Chrome Task Manager and DevTools:
- Measure extension memory before operations (baseline)
- Measure after 1, 5, 10, 20 operations
- Check for unbounded growth (indicates memory leak)
- Check for sudden spikes (indicates buffer overflow)
- Identify which objects accumulate (buffers, listeners, promises)

### Step 3: Audit Event Listeners
Check listener registration:
- Search for `chrome.runtime.onMessage.addListener` calls
- Verify listeners are added once at startup (not on every operation)
- Check if listeners are removed when no longer needed (removeListener)
- Verify no duplicate handlers registered
- Check window.addEventListener and postMessage listeners in content/inject scripts

### Step 4: Trace Message Flow
Add debug logging to background.js:
- Log when message received from content script
- Log when query forwarded to content script
- Log when result posted to server
- Log queue depth at each step
- Identify where messages get stuck (queue, forwarding, posting)

### Step 5: Check Promise Handling
Audit async operations:
- Verify all promises have .catch() handlers (no unhandled rejections)
- Check if promises timeout correctly (don't wait indefinitely)
- Verify promise chains don't accumulate (clean up after resolve/reject)
- Check if pending operations map is cleared after completion

### Step 6: Investigate Polling Loop
Check the /pending-queries polling mechanism:
- Verify polling interval is consistent (2 seconds)
- Check if polling loop ever stops or hangs
- Verify polling doesn't accumulate (no overlapping polls)
- Check if fetch errors in polling loop are handled gracefully
- Ensure polling doesn't block message processing

### Step 7: Fix Identified Issue
Based on diagnosis, apply fix:
- **If message queue backup:** Add queue size limits, prioritize recent messages, drop old ones
- **If promise rejection:** Add timeout wrappers, ensure all promises reject after 10s
- **If listener accumulation:** Move listener registration to startup, use single handler
- **If memory leak:** Clear buffers after operations, limit buffer sizes, use WeakMap for temporary data
- **If polling overlap:** Ensure only one polling fetch active at a time, cancel previous if still pending

### Step 8: Add Health Monitoring
Implement diagnostic endpoints:
- Track operation count, memory usage, queue depth, pending operations
- Expose via `configure({action: "health"})`
- Log warnings when thresholds exceeded (queue > 10, memory > 80MB, pending > 5)
- Add automatic cleanup when limits exceeded

## Edge Cases & Assumptions

### Edge Case 1: Service Worker Restart Mid-Operation
**Handling:** Chrome may restart the service worker every 5 minutes or when idle. Ensure state is restored or operations fail gracefully (not hang). Use chrome.storage.session to persist critical state.

### Edge Case 2: Content Script Disconnected
**Handling:** If content script is unloaded (page navigated away), background worker should timeout pending operations to that tab after 5 seconds. Don't wait indefinitely.

### Edge Case 3: Server Unreachable
**Handling:** If polling fails because server is down, don't accumulate retry attempts indefinitely. Exponential backoff or circuit breaker pattern.

### Edge Case 4: Very Large Query Results
**Handling:** If a query result is massive (50KB+ JSON), posting to server may fail or take too long. Truncate results or stream in chunks.

### Edge Case 5: Concurrent Operations
**Handling:** If user sends 5 operations simultaneously (not queued), ensure message handlers don't collide. Use correlation IDs to match requests/responses.

### Assumption 1: Operations Are Sequential
We assume operations are sent one at a time, not concurrently. If concurrent, the queue may overflow differently.

### Assumption 2: Browser Has Sufficient Resources
We assume the browser has enough memory and CPU. On low-end systems, any extension may become unresponsive; not specific to Kaboom.

### Assumption 3: Issue Is Extension-Side
We assume the server is responding correctly and not causing backpressure. Verify server health separately.

## Risks & Mitigations

### Risk 1: Fix Introduces New Performance Regression
**Mitigation:** Benchmark operation latency before and after fix. Ensure median latency doesn't increase by more than 10%.

### Risk 2: Fix Breaks Existing Functionality
**Mitigation:** Run full extension test suite after fix. Test all MCP tools (observe, generate, configure, interact) to ensure no breakage.

### Risk 3: Issue Is Chrome Bug
**Mitigation:** If issue persists after all Kaboom fixes, file Chrome bug report. Document Chrome version where issue occurs. Test on different Chrome versions.

### Risk 4: Memory Leak Is in Third-Party Library
**Mitigation:** Profile which objects are leaking. If axe-core or another bundled library leaks, consider updating to newer version or replacing.

## Dependencies

- **Existing:** Background service worker message handling
- **Existing:** Content script message forwarding
- **Existing:** Polling loop for pending queries
- **New:** Health monitoring endpoint (configure action: "health")
- **New:** Memory profiling and diagnostic logging
- **New:** Timeout enforcement on pending operations

## Performance Considerations

- Message processing time: < 5ms per message (should not accumulate)
- Polling interval: 2 seconds (should not overlap or accumulate)
- Memory per operation: < 1MB (buffers cleared after operation)
- Queue size limit: 10 pending operations (drop oldest if exceeded)
- Operation timeout: 10 seconds (fail operation, don't hang)

## Security Considerations

- **Resource exhaustion attack:** If malicious page floods extension with messages, queue could overflow. Mitigation: rate limiting (max 10 operations per second from a single tab).
- **Memory exhaustion attack:** If malicious page sends huge query results, extension memory could balloon. Mitigation: result size limits (max 50KB per result, truncate larger).
- **No new attack surface:** This bug fix doesn't expose new functionality or data. Security posture unchanged.

## Test Plan Reference

See qa-plan.md for detailed testing strategy. Key test scenarios:
1. 50 consecutive DOM queries complete without timeout
2. Memory usage remains bounded (< 100MB after 50 ops)
3. No unhandled promise rejections in console
4. Health check shows queue depth = 0 after operations
5. Mixed operation types (DOM, accessibility, network) work reliably
6. Service worker restart mid-operation handled gracefully
7. Regression: all existing functionality still works
