---
feature: extension-timeout
doc_type: qa-plan
feature_id: bug-extension-timeout
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan: Extension Timeout After 5-6 Operations (Bug Fix)

> How to test the extension timeout bug fix. Includes code-level testing, profiling, and human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

**Unit tests:** Message handling and cleanup
- [ ] Message queue processes messages in order
- [ ] Queue size never exceeds 10 pending operations
- [ ] Old operations dropped when queue full
- [ ] Event listeners registered once at startup (not per operation)
- [ ] Promises timeout after 10 seconds if no response
- [ ] Pending operations map cleared after completion
- [ ] No unhandled promise rejections

**Integration tests:** Multi-operation sequences
- [ ] 10 consecutive DOM queries complete successfully
- [ ] 20 consecutive accessibility audits complete successfully
- [ ] 30 mixed operations (DOM, A11y, network) complete successfully
- [ ] 50 consecutive observe() calls complete successfully
- [ ] Memory usage after 50 operations < 100MB
- [ ] No performance degradation (operation 50 same latency as operation 1)

**Edge case tests:** Error scenarios and recovery
- [ ] Operation timeout after 10 seconds (simulate frozen content script)
- [ ] Service worker restart mid-operation (operation fails gracefully)
- [ ] Content script disconnected (operation fails with clear error)
- [ ] Server unreachable during polling (operations queue but eventually timeout)
- [ ] Very large query result (50KB+) handled without crash
- [ ] Concurrent operations (5 sent simultaneously) all complete

### Performance Profiling Tests

#### Memory profiling:
- [ ] Baseline memory: < 20MB before any operations
- [ ] After 10 operations: < 40MB (growth rate acceptable)
- [ ] After 50 operations: < 100MB (bounded growth)
- [ ] No memory leaks detected via Chrome DevTools heap snapshot
- [ ] Garbage collection reclaims memory after operations

#### Message queue profiling:
- [ ] Queue depth = 0 when idle
- [ ] Queue depth max = 1 during normal operations (sequential)
- [ ] Queue depth max = 10 when operations are concurrent (enforced limit)
- [ ] Messages processed within 5ms each
- [ ] No message queue backup after 50 operations

#### Event listener profiling:
- [ ] chrome.runtime.onMessage listeners: 1 (not accumulating)
- [ ] window.addEventListener listeners: stable count (not growing)
- [ ] chrome.tabs.onUpdated listeners: stable count

---

## Human UAT Walkthrough

### Scenario 1: 50 Consecutive DOM Queries
1. Setup:
   - Start Kaboom server: `./dist/kaboom`
   - Load Chrome with extension, open DevTools
   - Navigate to complex page (e.g., GitHub.com)
   - Track the tab
2. Steps:
   - [ ] Run script that calls `analyze({what: "dom", selector: "div"})` 50 times in sequence
   - [ ] Monitor Chrome Task Manager for memory usage
   - [ ] Monitor DevTools Console for errors
   - [ ] Record operation latency for operations 1, 10, 25, 50
3. Expected Result:
   - [ ] All 50 operations complete successfully
   - [ ] No timeout errors
   - [ ] Memory stays below 100MB
   - [ ] Operation 50 latency similar to operation 1 (no degradation)
4. Verification: Extension remains responsive after 50 operations

### Scenario 2: Mixed Operations (30 Total)
1. Setup: Same as Scenario 1
2. Steps:
   - [ ] Operation 1-5: DOM queries
   - [ ] Operation 6-10: Accessibility audits
   - [ ] Operation 11-15: Network body requests
   - [ ] Operation 16-20: Page observations
   - [ ] Operation 21-25: Interact actions (highlight)
   - [ ] Operation 26-30: Mixed repeats
   - [ ] Check health after all operations: `configure({action: "health"})`
3. Expected Result:
   - [ ] All 30 operations complete
   - [ ] Health check shows: queue_depth = 0, pending_operations = 0, memory < 80MB
4. Verification: No operation type causes accumulation or hang

### Scenario 3: Service Worker Restart During Operations
1. Setup:
   - Start operations sequence
   - Navigate to chrome://extensions
   - Locate Kaboom extension, click "background page" to open service worker DevTools
2. Steps:
   - [ ] Start 20 DOM query sequence
   - [ ] After operation 10, manually restart service worker (close and reopen background page)
   - [ ] Continue remaining 10 operations
3. Expected Result:
   - [ ] Operation 10 may fail (service worker restarted)
   - [ ] Operations 11-20 succeed (state restored or cleanly restarted)
   - [ ] No indefinite hangs
4. Verification: Service worker restart doesn't cause permanent failure

### Scenario 4: Memory Profiling with Heap Snapshot
1. Setup: Chrome DevTools → Memory tab
2. Steps:
   - [ ] Take heap snapshot (baseline)
   - [ ] Run 10 DOM queries
   - [ ] Take heap snapshot (after 10)
   - [ ] Run 40 more DOM queries (50 total)
   - [ ] Take heap snapshot (after 50)
   - [ ] Compare snapshots for detached DOM nodes, accumulating objects
3. Expected Result:
   - [ ] No significant increase in detached DOM nodes
   - [ ] No accumulating arrays or objects (listeners, promises)
   - [ ] Memory growth is linear and bounded (not exponential)
4. Verification: No memory leaks detected

### Scenario 5: Rapid Concurrent Operations
1. Setup: Same as Scenario 1
2. Steps:
   - [ ] Send 10 operations simultaneously (via Promise.all or parallel script)
   - [ ] Monitor queue depth via health check
   - [ ] Wait for all to complete
3. Expected Result:
   - [ ] All 10 operations complete (may take longer due to queuing)
   - [ ] Queue depth max = 10 (enforced limit)
   - [ ] No operations dropped or lost
   - [ ] No crashes or errors
4. Verification: Concurrent operations handled gracefully

### Scenario 6: Operation Timeout on Frozen Page
1. Setup: Navigate to page, track tab
2. Steps:
   - [ ] Freeze page via DevTools (Rendering → Rendering paused)
   - [ ] Send DOM query: `analyze({what: "dom"})`
   - [ ] Wait up to 15 seconds
3. Expected Result:
   - [ ] Operation times out after ~10 seconds
   - [ ] Error message: "Operation timed out waiting for content script response"
   - [ ] Extension remains responsive (not permanently hung)
4. Verification: Timeout mechanism prevents indefinite hangs

### Scenario 7: Health Check After Stress Test
1. Setup: Complete 50 operations as in Scenario 1
2. Steps:
   - [ ] Call `configure({action: "health"})`
   - [ ] Observe response
3. Expected Result:
   ```json
   {
     "status": "healthy",
     "operations_completed": 50,
     "memory_mb": 45,
     "message_queue_depth": 0,
     "pending_operations": 0,
     "uptime_seconds": 600
   }
   ```
4. Verification: All metrics within healthy ranges

---

## Regression Testing

### Must Not Break

- [ ] Single operation still works (operation 1)
- [ ] DOM queries return correct results
- [ ] Accessibility audits return correct results
- [ ] Network captures work correctly
- [ ] Tab tracking works
- [ ] Extension startup time not significantly increased
- [ ] Operation latency not significantly increased (< 10% slower)

### Regression Test Steps

1. Run existing extension test suite: `node --test tests/extension/*.test.js`
2. Verify all tests pass (no new failures)
3. Manually test each MCP tool (observe, generate, configure, interact)
4. Verify telemetry capture still works (logs, network, WebSocket)
5. Test on multiple pages (simple, complex, SPA)

---

## Performance/Load Testing

### Latency benchmarks:
- [ ] Operation 1: baseline latency (e.g., 2s for DOM query)
- [ ] Operation 10: latency within 10% of baseline
- [ ] Operation 25: latency within 10% of baseline
- [ ] Operation 50: latency within 10% of baseline

### Memory benchmarks:
- [ ] Baseline: < 20MB
- [ ] After 10 ops: < 40MB
- [ ] After 50 ops: < 100MB
- [ ] After 100 ops: < 150MB (if testing further)

### Queue depth benchmarks:
- [ ] Sequential operations: queue depth max = 1
- [ ] Concurrent operations (10): queue depth max = 10
- [ ] After all operations complete: queue depth = 0

### Garbage collection:
- [ ] Force GC via DevTools after 50 operations
- [ ] Memory drops to near baseline (some growth acceptable)
- [ ] No large objects retained indefinitely

---

## Diagnostic Logging

### Required logs for debugging:
- [ ] "Message received: [type]" when message arrives
- [ ] "Forwarding query [id] to tab [tabId]"
- [ ] "Query result posted to server [id]"
- [ ] "Queue depth: [N]" after each operation
- [ ] "Memory usage: [MB]" every 10 operations
- [ ] "Operation timed out [id]" when timeout occurs
- [ ] "Event listener count: [N]" at startup and periodically

### Console warnings to add:
- [ ] Warning when queue depth > 5: "Message queue backing up"
- [ ] Warning when memory > 80MB: "Extension memory high"
- [ ] Warning when operation takes > 10s: "Operation slow"
- [ ] Warning when unhandled rejection: "Promise rejected without handler"
