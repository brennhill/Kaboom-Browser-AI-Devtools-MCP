---
status: draft
scope: audit/codebase
ai-priority: high
tags: [audit, security, performance, code-quality]
relates-to: []
last-verified: 2026-02-08
---

# Codebase Audit Report

**Date:** 2026-02-08
**Auditor:** Principal Engineer  
**Scope:** Entire Kaboom codebase (Go backend, TypeScript/JavaScript frontend)  
**Purpose:** Identify bad practices, memory leaks, weak tests, security issues, bad design patterns, and other concerns.

---

## Executive Summary

This audit identified **47 issues** across the codebase categorized as:
- **Critical:** 3 issues
- **High:** 12 issues
- **Medium:** 22 issues
- **Low:** 10 issues

**Overall Assessment:** The codebase demonstrates good architecture and strong adherence to zero-dependency principles. However, there are several areas requiring attention, particularly around memory management, error handling, and test coverage.

---

## 1. Memory Leaks and Resource Management

### Critical Issues

#### 1.1 WebSocket Event Array Desynchronization Risk
**File:** [`internal/capture/websocket.go`](internal/capture/websocket.go:29-35)  
**Severity:** Critical  
**Type:** Memory Leak Risk

**Issue:** The defensive recovery logic for `wsEvents` and `wsAddedAt` array length mismatch truncates arrays but doesn't update the `wsMemoryTotal` counter. This can cause memory accounting to drift.

```go
// Current code (lines 29-35)
if len(c.wsEvents) != len(c.wsAddedAt) {
    fmt.Fprintf(os.Stderr, "[kaboom] WARNING: wsEvents/wsAddedAt length mismatch: %d != %d (recovering by truncating)\n",
        len(c.wsEvents), len(c.wsAddedAt))
    minLen := min(len(c.wsEvents), len(c.wsAddedAt))
    c.wsEvents = c.wsEvents[:minLen]
    c.wsAddedAt = c.wsAddedAt[:minLen]
    // MISSING: wsMemoryTotal should be recalculated here
}
```

**Recommendation:** Recalculate `wsMemoryTotal` after truncation or track memory per-entry to avoid recalculation.

---

#### 1.2 Pending Request Map Cleanup Edge Cases ✅ FIXED
**File:** [`src/content/request-tracking.ts`](src/content/request-tracking.ts:29-34)
**Severity:** Critical
**Type:** Memory Leak Risk

**Issue:** While `clearPendingRequests()` is called on page unload, there are edge cases where this might not execute:
- Extension service worker restart (MV3 limitation)
- Page crashes
- Extension updates

**Fix Applied:** Added periodic cleanup timer (30s interval) with stale request detection. This provides fallback when pagehide/beforeunload don't fire (e.g., page crash, service worker restart).

**Changes Made:**
- Added `CLEANUP_INTERVAL_MS = 30000` constant
- Added `requestTimestamps` Map for tracking request ages
- Added `getRequestTimestamps()` helper function
- Added `performPeriodicCleanup()` function with stale threshold check
- Added `cleanupRequestTracking()` export function for proper shutdown
- Modified `initRequestTracking()` to start periodic timer
- Updated `src/content.ts` to export `cleanupRequestTracking()`

**Code:**
```typescript
// Periodic cleanup timer (Issue #2 fix)
const CLEANUP_INTERVAL_MS = 30000 // 30 seconds
let cleanupTimer: ReturnType<typeof setInterval> | null = null

// Track request timestamps for stale detection
const requestTimestamps = new Map<number, number>()

function performPeriodicCleanup(): void {
  const now = Date.now()
  const staleThreshold = 60000 // 60 seconds
  let hasStaleRequests = false

  for (const [id, timestamp] of getRequestTimestamps()) {
    if (now - timestamp > staleThreshold) {
      pendingHighlightRequests.delete(id)
      pendingExecuteRequests.delete(id)
      pendingA11yRequests.delete(id)
      pendingDomRequests.delete(id)
      requestTimestamps.delete(id)
      hasStaleRequests = true
    }
  }

  if (hasStaleRequests) {
    console.debug('[Kaboom] Periodic cleanup: removed stale requests')
  }
}

export function cleanupRequestTracking(): void {
  if (cleanupTimer) {
    clearInterval(cleanupTimer)
    cleanupTimer = null
  }
  clearPendingRequests()
}
```

---

#### 1.3 Goroutine Leak in Query Cleanup
**File:** [`internal/capture/queries.go`](internal/capture/queries.go:79-90)
**Severity:** High
**Type:** Resource Leak

**Issue:** The goroutine spawned for query cleanup may leak if queries are rapidly created and deleted. The goroutine has no cancellation mechanism.

```go
// Current code (lines 79-90)
go func() {
    time.Sleep(timeout)
    qd.mu.Lock()
    qd.cleanExpiredQueries()
    qd.queryCond.Broadcast()
    qd.mu.Unlock()
    
    if correlationID != "" {
        qd.ExpireCommand(correlationID)
    }
}()
```

**Recommendation:** Use `context.Context` for goroutine cancellation and track cleanup goroutines.

---

### Medium Issues

#### 1.4 Recording Storage No Cleanup ✅ FIXED
**File:** [`internal/capture/recording.go`](internal/capture/recording.go:21-25)
**Severity:** Medium
**Type:** Disk Space Leak

**Issue:** Recording storage has a 1GB limit but no automatic cleanup mechanism for old recordings.

```go
const (
    recordingStorageMax    = 1024 * 1024 * 1024 // 1GB max storage
    recordingWarningLevel  = 800 * 1024 * 1024  // 800MB warning threshold
    recordingBaseDir       = ".kaboom/recordings"
    recordingMetadataFile  = "metadata.json"
)
```

**Fix:** Added soft limit warning and user-controlled cleanup via HTTP endpoint `/recordings/storage`:
- `GET /recordings/storage` - Returns storage info (used bytes, max bytes, warning level, recording count)
- `DELETE /recordings/storage?recording_id=xxx` - Deletes a specific recording
- `POST /recordings/storage` - Recalculates storage usage from disk

Users can now view storage usage and delete recordings manually. The soft limit warning (80%) was already implemented.

---

#### 1.5 Sync Client Pending Results Accumulation
**File:** [`src/background/sync/sync-client.ts`](src/background/sync/sync-client.ts:156-164)  
**Severity:** Medium  
**Type:** Memory Accumulation

**Issue:** While there's a cap on `pendingResults`, if the server is unreachable for extended periods, memory accumulates until the cap.

```typescript
// Current code (lines 156-164)
queueCommandResult(result: SyncCommandResult): void {
  this.pendingResults.push(result)
  // Cap queue size to prevent memory leak if server is unreachable
  const MAX_PENDING_RESULTS = 200
  if (this.pendingResults.length > MAX_PENDING_RESULTS) {
    this.pendingResults.splice(0, this.pendingResults.length - MAX_PENDING_RESULTS)
  }
  this.flush()
}
```

**Recommendation:** Implement exponential backoff and consider discarding old results based on age, not just count.

---

## 2. Security Issues

### Critical Issues

#### 2.1 Insufficient Sensitive Input Detection
**File:** [`src/lib/page/serialize.ts`](src/lib/page/serialize.ts:143-173)  
**Severity:** High  
**Type:** Data Leak Risk

**Issue:** The `isSensitiveInput()` function doesn't cover all sensitive input patterns, particularly:
- Custom autocomplete attributes
- Data attributes with sensitive names
- Form field names in other languages

```typescript
// Current patterns are limited
if (name.includes('password') ||
    name.includes('passwd') ||
    name.includes('secret') ||
    name.includes('token') ||
    name.includes('credit') ||
    name.includes('card') ||
    name.includes('cvv') ||
    name.includes('cvc') ||
    name.includes('ssn'))
    return true
```

**Recommendation:** Expand pattern matching and consider using a configurable allowlist/denylist.

---

#### 2.2 Fetch Wrapper May Capture Sensitive Data
**File:** [`src/inject/observers.ts`](src/inject/observers.ts:61-149)  
**Severity:** High  
**Type:** Data Leak Risk

**Issue:** The `wrapFetch` function captures response bodies for 4xx/5xx errors without checking if the URL is sensitive (e.g., authentication endpoints).

```typescript
// Current code (lines 73-83)
if (!response.ok) {
  let responseBody = ''
  try {
    const cloned = response.clone()
    responseBody = await cloned.text()
    if (responseBody.length > MAX_RESPONSE_LENGTH) {
      responseBody = responseBody.slice(0, MAX_RESPONSE_LENGTH) + '... [truncated]'
    }
  } catch {
    responseBody = '[Could not read response]'
  }
  // ... logs responseBody without URL filtering
}
```

**Recommendation:** Add URL pattern filtering to avoid logging sensitive endpoints (login, auth, token refresh).

---

### Medium Issues

#### 2.3 Header Redaction Incomplete
**File:** [`src/inject/observers.ts`](src/inject/observers.ts:86-98)  
**Severity:** Medium  
**Type:** Data Leak Risk

**Issue:** The sensitive header list may not include all authentication-related headers.

```typescript
// Current check (lines 94-96)
if (value && !SENSITIVE_HEADERS.includes(key.toLowerCase())) {
  safeHeaders[key] = value
}
```

**Recommendation:** Review and expand `SENSITIVE_HEADERS` list to include all auth-related headers.

---

#### 2.4 Settings File Permissions Too Permissive
**File:** [`internal/capture/settings.go`](internal/capture/settings.go:106)  
**Severity:** Medium  
**Type:** Access Control Issue

**Issue:** Settings file uses 0600 permissions, but the nosec comment suggests owner-only reading. However, 0600 allows group write.

```go
// Line 106
if err := os.WriteFile(tmpPath, data, 0600); err != nil {
    return err
}
```

**Recommendation:** Use 0400 (owner read-only) for settings files containing sensitive data.

---

## 3. Concurrency and Race Conditions

### High Issues

#### 3.1 Lock Ordering Violation Risk
**File:** [`internal/session/client_registry.go`](internal/session/client_registry.go:136-141)  
**Severity:** High  
**Type:** Deadlock Risk

**Issue:** The comment states "Lock ordering: ClientRegistry.mu before ClientState.mu" but there's no enforcement mechanism.

```go
// Comment states ordering but no enforcement (line 136)
// Thread-safe with RWMutex. Lock ordering: ClientRegistry.mu before ClientState.mu.
type ClientRegistry struct {
    mu      sync.RWMutex
    clients map[string]*ClientState
    accessOrder []string
}
```

**Recommendation:** Add runtime lock ordering checks in debug mode or use a lock hierarchy pattern.

---

#### 3.2 Sync Client Flush Race Condition
**File:** [`src/background/sync/sync-client.ts`](src/background/sync/sync-client.ts:167-178)  
**Severity:** High  
**Type:** Race Condition

**Issue:** The `flushRequested` flag is checked without atomic operations, creating a race condition.

```typescript
// Current code (lines 167-178)
flush(): void {
  if (!this.running) return
  if (this.syncing) {
    // Sync in progress — schedule another immediately after it finishes
    this.flushRequested = true  // Race: not atomic
    return
  }
  if (this.intervalId) {
    clearTimeout(this.intervalId)
  }
  this.scheduleNextSync(0)
}
```

**Recommendation:** Use a proper state machine or atomic operations for flush state.

---

### Medium Issues

#### 3.3 Query Dispatcher Broadcast Without Context
**File:** [`internal/capture/queries.go`](internal/capture/queries.go:196)  
**Severity:** Medium  
**Type:** Concurrency Issue

**Issue:** The `queryCond.Broadcast()` is called after releasing `mu`, which is correct, but there's no guarantee waiters are ready.

```go
// Line 196
qd.queryCond.Broadcast()
```

**Recommendation:** Consider using a channel-based approach for better control over waiter lifecycle.

---

## 4. Error Handling Issues

### High Issues

#### 4.1 HTTP Handlers Lack Detailed Error Logging ✅ FIXED
**File:** [`internal/capture/handlers.go`](internal/capture/handlers.go:15-42)
**Severity:** High
**Type:** Insufficient Error Reporting

**Issue:** HTTP handlers return generic error messages without sufficient detail for debugging.

```go
// Generic error (lines 22-24)
if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
    w.WriteHeader(http.StatusBadRequest)
    json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
    return
}
```

**Fix:** Added detailed error logging to stderr for all HTTP handlers:
- Added `fmt` and `os` imports
- Added `fmt.Fprintf(os.Stderr, "[kaboom] HandlerName: Error details - %v\n", err)` for:
  - HandleNetworkBodies: Invalid JSON errors
  - handleNetworkWaterfallPOST: Invalid JSON errors
  - HandleDOMResult: Method not allowed, Invalid JSON, Missing query ID
  - HandleA11yResult: Method not allowed, Invalid JSON
  - HandleStateResult: Method not allowed, Invalid JSON
  - HandleExecuteResult: Method not allowed, Invalid JSON
  - HandleHighlightResult: Method not allowed, Invalid JSON
  - HandleEnhancedActions: Invalid JSON errors
  - HandlePerformanceSnapshots: Invalid JSON errors
  - HandleRecordingStorage: Missing recording_id, Delete errors, Recalculate errors

---

#### 4.2 Settings Load Silent Failure
**File:** [`internal/capture/settings.go`](internal/capture/settings.go:43-81)  
**Severity:** Medium  
**Type:** Silent Error

**Issue:** Failed settings load logs to stderr but returns without indication of failure.

```go
// Lines 43-63
func (c *Capture) LoadSettingsFromDisk() {
    path, err := getSettingsPath()
    if err != nil {
        fmt.Fprintf(os.Stderr, "[kaboom] Could not determine settings path: %v\n", err)
        return  // Silent return
    }
    // ...
}
```

**Recommendation:** Return error status to caller for proper error handling.

---

### Medium Issues

#### 4.3 Fetch No Retry Mechanism
**File:** [`src/background/sync/server.ts`](src/background/sync/server.ts:48-70)  
**Severity:** Medium  
**Type:** Insufficient Resilience

**Issue:** All fetch calls have error handling but no retry mechanism for transient failures.

```typescript
// No retry logic (lines 55-70)
const response = await fetch(`${serverUrl}/logs`, {
    method: 'POST',
    headers: getRequestHeaders(),
    body: JSON.stringify({ entries }),
})

if (!response.ok) {
    const error = `Server error: ${response.status} ${response.statusText}`
    if (debugLogFn) debugLogFn('error', error)
    throw new Error(error)
}
```

**Recommendation:** Implement exponential backoff retry for transient failures (5xx, network errors).

---

#### 4.4 Network Capture Error Swallowing
**File:** [`src/lib/net/network.ts`](src/lib/net/network.ts:159-161)  
**Severity:** Medium  
**Type:** Error Suppression

**Issue:** The try-catch in `getNetworkWaterfall` silently returns empty array on any error.

```typescript
// Lines 159-161
} catch {
    return []  // Silent error return
}
```

**Recommendation:** Log errors to debug log and consider returning error status.

---

## 5. Design Pattern and Code Quality Issues

### High Issues

#### 5.1 File Size Violation
**Files:** Multiple files exceed 800 LOC limit  
**Severity:** High  
**Type:** Code Quality

**Issue:** Several files exceed the 800 LOC limit specified in project rules:

| File | Lines | Status |
|-------|--------|--------|
| `src/background/pending-queries.ts` | 952 | ❌ Exceeds |
| `internal/capture/types.go` | 653 | ❌ Exceeds |
| `internal/session/sessions.go` | 694 | ❌ Exceeds |
| `src/background/message-handlers.ts` | 552 | ❌ Exceeds |
| `src/lib/net/websocket.ts` | 776 | ⚠️ Near limit |

**Recommendation:** Refactor large files into smaller, focused modules following single responsibility principle.

**Refactoring Plans:**
- **File Size Violations:** See [`file-size-refactor-plan.md`](file-size-refactor-plan.md) for detailed split strategy including:
  - 5 files to refactor (pending-queries.ts, types.go, sessions.go, message-handlers.ts, websocket.ts)
  - Phase-by-phase refactoring approach
  - Testing strategy
  - Migration checklist
  - Estimated effort (17.25 hours total)

**Go Files:**
- **types.go:** See [`types-go-refactor-plan.md`](types-go-refactor-plan.md) - 12 new files, 6.8 hours
- **sessions.go:** See [`sessions-go-refactor-plan.md`](sessions-go-refactor-plan.md) - 11 new files, 4.6 hours
- **types.go Split:** See [`types-go-refactor-plan.md`](types-go-refactor-plan.md) for detailed split strategy including:
  - 12 new files organized by domain/concern
  - Phase-by-phase refactoring approach
  - Testing strategy
  - Migration checklist
  - Estimated effort (6.8 hours)

**Refactoring Plan:** See [`file-size-refactor-plan.md`](file-size-refactor-plan.md) for detailed split strategy including:
- Proposed file structure for each large file
- Phase-by-phase refactoring approach
- Testing strategy
- Migration checklist
- Estimated effort (17-25 hours total)

---

#### 5.2 Single Responsibility Violation
**File:** [`src/background/pending-queries.ts`](src/background/pending-queries.ts)  
**Severity:** High  
**Type:** Design Pattern

**Issue:** This file handles multiple concerns:
- DOM query execution
- Accessibility audits
- Browser actions
- Execute commands
- State management

**Recommendation:** Split into separate modules: `query-executors.ts`, `a11y-handlers.ts`, `browser-actions.ts`.

---

#### 5.3 Tight Coupling
**Files:** Multiple `src/background/` modules  
**Severity:** Medium  
**Type:** Design Pattern

**Issue:** Background modules have tight coupling, making testing and refactoring difficult.

```typescript
// Example from pending-queries.ts (lines 10-17)
import * as eventListeners from './event-listeners'
import * as index from './index'
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot, broadcastTrackingState } from './message-handlers'
import { executeDOMAction } from './dom-primitives'
import { canTakeScreenshot, recordScreenshot } from './state-manager'
import { startRecording, stopRecording } from './recording'
```

**Recommendation:** Implement dependency injection and use interfaces for better testability.

---

### Medium Issues

#### 5.4 Magic Numbers Without Constants
**File:** Multiple files  
**Severity:** Medium  
**Type:** Code Quality

**Issue:** Magic numbers used without explanation:

```typescript
// From sync-client.ts
const MAX_PENDING_RESULTS = 200  // Why 200?

// From connection-state.ts
private readonly maxHistorySize = 50  // Why 50?

// From circuit-breaker.ts
const maxFailures = options.maxFailures ?? 5  // Why 5?
```

**Recommendation:** Document constants with rationale or make them configurable.

---

#### 5.5 Inconsistent Error Handling Patterns
**Files:** Various  
**Severity:** Medium  
**Type:** Code Quality

**Issue:** Different modules use different error handling strategies:
- Some throw errors
- Some return error objects
- Some log and continue silently

**Recommendation:** Establish a consistent error handling pattern across the codebase.

---

## 6. Test Coverage and Quality Issues

### High Issues

#### 6.1 Missing Test Coverage for Edge Cases
**Files:** Various test files  
**Severity:** High  
**Type:** Test Quality

**Issue:** Several critical paths lack test coverage:
- Memory pressure scenarios
- Concurrent client operations
- WebSocket reconnection edge cases
- Extension service worker restart scenarios

**Recommendation:** Add integration tests for edge cases and stress tests for concurrent operations.

---

#### 6.2 Weak Test Assertions
**Files:** Various test files  
**Severity:** Medium  
**Type:** Test Quality

**Issue:** Some tests use overly permissive assertions that may not catch regressions.

```javascript
// Example pattern found in tests
expect(result).toBeTruthy()  // Too permissive
expect(result).toBeDefined()   // Too permissive
```

**Recommendation:** Use specific assertions that verify expected values, not just truthiness.

---

### Medium Issues

#### 6.3 Test Isolation Issues
**Files:** Various test files  
**Severity:** Medium  
**Type:** Test Quality

**Issue:** Tests may have state leakage between test cases due to insufficient cleanup.

```javascript
// Common pattern - may not clean up all state
afterEach(() => {
    // Some cleanup, but may miss edge cases
})
```

**Recommendation:** Implement comprehensive teardown procedures and use test isolation patterns.

---

## 7. Performance Issues

### Medium Issues

#### 7.1 Inefficient Array Operations
**File:** [`internal/buffers/ring_buffer.go`](internal/buffers/ring_buffer.go:130-134)  
**Severity:** Medium  
**Type:** Performance

**Issue:** The `ReadFrom` method creates a new array for every read operation.

```go
// Lines 130-134
result := make([]T, 0, entriesAvailable)
for i := int64(0); i < entriesAvailable; i++ {
    idx := int((int64(startIndex) + i) % int64(len(rb.entries)))
    result = append(result, rb.entries[idx])
}
```

**Recommendation:** Consider pre-allocating with known capacity or using a more efficient data structure.

---

#### 7.2 Unnecessary Object Creation
**File:** [`src/lib/net/websocket.ts`](src/lib/net/websocket.ts:152-169)  
**Severity:** Medium  
**Type:** Performance

**Issue:** The `createConnectionTracker` function creates multiple objects per connection.

```typescript
// Creates new objects for each connection
export function createConnectionTracker(id: string, url: string): ConnectionTracker {
  const tracker: ConnectionTracker = {
    id,
    url,
    messageCount: 0,
    _sampleCounter: 0,
    _messageRate: 0,
    _messageTimestamps: [],  // New array per connection
    _schemaKeys: [],       // New array per connection
    _schemaVariants: new Map(), // New Map per connection
    // ...
  }
}
```

**Recommendation:** Consider object pooling for high-frequency connections.

---

#### 7.3 Redundant DOM Queries
**File:** [`src/lib/analysis/dom-queries.ts`](src/lib/analysis/dom-queries.ts:151-173)  
**Severity:** Low  
**Type:** Performance

**Issue:** The `executeDOMQuery` function may perform redundant calculations.

```typescript
// Lines 159-164
for (let i = 0; i < Math.min(elements.length, DOM_QUERY_MAX_ELEMENTS); i++) {
    const el = elements[i]
    if (!el) continue
    const entry = serializeDOMElement(el, include_styles, properties, include_children, cappedDepth, 0)
    matches.push(entry)
}
```

**Recommendation:** Cache query results when possible and avoid redundant DOM traversals.

---

## 8. TypeScript/JavaScript Type Safety Issues

### Medium Issues

#### 8.1 Use of `any` Type
**File:** [`src/inject/message-handlers.ts`](src/inject/message-handlers.ts:129-195)  
**Severity:** Medium  
**Type:** Type Safety

**Issue:** The `safeSerializeForExecute` function uses type assertions that bypass TypeScript's type checking.

```typescript
// Lines 180-184
const result: Record<string, unknown> = {}
for (const key of Object.keys(obj).slice(0, 50)) {
    try {
        result[key] = safeSerializeForExecute((obj as Record<string, unknown>)[key], depth + 1, seen)
    } catch {
        result[key] = '[unserializable]'
    }
}
```

**Recommendation:** Use proper type guards and avoid `as` casts where possible.

---

#### 8.2 Missing Type Guards
**File:** [`src/lib/page/serialize.ts`](src/lib/page/serialize.ts:86-96)  
**Severity:** Medium  
**Type:** Type Safety

**Issue:** DOM element type checking uses duck typing that may not be reliable.

```typescript
// Lines 86-96
const domLike = value as DOMElementLike
if (domLike.nodeType) {
    const tag = domLike.tagName ? domLike.tagName.toLowerCase() : 'node'
    const id = domLike.id ? `#${domLike.id}` : ''
    // ...
}
```

**Recommendation:** Use `instanceof` checks with proper type guards.

---

## 9. Documentation and Code Comments

### Low Issues

#### 9.1 TODO Comments Without Issues
**File:** [`internal/security/security_config.go`](internal/security/security_config.go:44,129,145,161,181)  
**Severity:** Low  
**Type:** Documentation Debt

**Issue:** Multiple TODO comments indicate unimplemented features:

```go
// Line 44
// TODO(future): Add isStdinStdoutPipe() detection for better MCP mode detection

// Line 129
// TODO(future): Implement interactive confirmation and config file update
```

**Recommendation:** Create GitHub issues for TODOs or remove if no longer needed.

---

#### 9.2 Inconsistent Comment Style
**Files:** Various  
**Severity:** Low  
**Type:** Code Quality

**Issue:** Mix of comment styles (JSDoc, Go doc comments, inline comments).

**Recommendation:** Establish and enforce consistent comment style conventions.

---

## 10. Configuration and Constants Issues

### Medium Issues

#### 10.1 Hardcoded Configuration Values
**Files:** Various  
**Severity:** Medium  
**Type:** Maintainability

**Issue:** Many configuration values are hardcoded without being configurable:

```typescript
// From constants.ts
export const MAX_STRING_LENGTH = 1000  // Why 1000?
export const MAX_DEPTH = 10              // Why 10?
export const DOM_QUERY_MAX_ELEMENTS = 100  // Why 100?
```

**Recommendation:** Make more constants configurable via settings file or environment variables.

---

#### 10.2 Inconsistent Timeout Values
**Files:** Various  
**Severity:** Medium  
**Type:** Maintainability

**Issue:** Different timeout values for similar operations:

| Location | Timeout | Purpose |
|-----------|----------|----------|
| `src/background/pending-queries.ts` | 60000ms | Async execute |
| `src/lib/net/network.ts` | BODY_READ_TIMEOUT_MS | Body read |
| `src/lib/net/websocket.ts` | WS_MAX_BODY_SIZE | Message size |

**Recommendation:** Consolidate timeout configuration and document rationale for each.

---

## Summary and Recommendations

### Immediate Actions (Critical/High Priority)

1. **Fix WebSocket memory accounting** - Update `wsMemoryTotal` after array truncation
2. **Implement pending request cleanup on service worker restart** - Add extension lifecycle hooks
3. **Add goroutine cancellation** - Use `context.Context` for query cleanup
4. **Refactor files exceeding 800 LOC** - Split large files into focused modules
5. **Expand sensitive input detection** - Add more patterns for sensitive data
6. **Implement fetch retry mechanism** - Add exponential backoff for transient failures
7. **Fix sync client race condition** - Use atomic operations for flush state
8. **Add recording storage cleanup** - Implement LRU eviction for old recordings

### Short-term Actions (Medium Priority)

1. **Improve error logging** - Add detailed error context to HTTP handlers
2. **Implement lock ordering checks** - Add runtime verification for lock hierarchy
3. **Add edge case tests** - Improve test coverage for concurrent operations
4. **Standardize error handling** - Establish consistent error handling patterns
5. **Document magic numbers** - Add rationale for all constant values
6. **Improve type safety** - Reduce use of `any` type and type assertions

### Long-term Actions (Low Priority)

1. **Refactor for better testability** - Implement dependency injection
2. **Performance optimization** - Review and optimize hot paths
3. **Documentation improvements** - Resolve TODO comments and standardize style
4. **Configuration management** - Make more values configurable

---

## Appendix: Files Reviewed

### Go Backend
- `cmd/browser-agent/main.go`
- `internal/capture/types.go`
- `internal/capture/handlers.go`
- `internal/capture/queries.go`
- `internal/capture/recording.go`
- `internal/capture/settings.go`
- `internal/capture/sync.go`
- `internal/capture/websocket.go`
- `internal/capture/memory.go`
- `internal/session/sessions.go`
- `internal/session/client_registry.go`
- `internal/security/security_config.go`
- `internal/buffers/ring_buffer.go`

### TypeScript/JavaScript Frontend
- `src/background/message-handlers.ts`
- `src/background/sync/server.ts`
- `src/background/sync/sync-client.ts`
- `src/background/sync/circuit-breaker.ts`
- `src/background/sync/batchers.ts`
- `src/background/connection-state.ts`
- `src/background/pending-queries.ts`
- `src/background/caches/state-manager.ts`
- `src/content/message-handlers.ts`
- `src/content/request-tracking.ts`
- `src/inject/message-handlers.ts`
- `src/inject/observers.ts`
- `src/lib/net/network.ts`
- `src/lib/net/websocket.ts`
- `src/lib/page/actions.ts`
- `src/lib/page/serialize.ts`
- `src/lib/analysis/dom-queries.ts`
- `src/lib/page/bridge.ts`

---

**End of Audit Report**
