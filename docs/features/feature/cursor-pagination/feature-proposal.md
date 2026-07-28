---
status: proposed
scope: feature/cursor-pagination
ai-priority: medium
tags: [proposal]
relates-to: [tech-spec.md, qa-plan.md]
last-verified: 2026-03-05
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Cursor-Based Pagination for Live Data - Feature Proposal

**Feature:** Cursor-Based Pagination
**Priority:** ⭐⭐⭐ HIGH (Fixes critical flaw in v5.3)
**Version:** v5.4
**Status:** Required to fix offset pagination bug
**Created:** 2026-01-30

---

## The Problem

### Offset-based pagination breaks for append-only buffers.

### Real-World Failure Scenario

```javascript
// Time T0: 100 logs in buffer [0-99]
observe({what: "logs", limit: 100})
// AI analyzes logs, takes 10 seconds...

// Time T1: During analysis, 100 new logs arrive
// Buffer now: 200 logs [0-199]
// NEW logs are at [0-99], OLD logs shifted to [100-199]

// AI requests next page
observe({what: "logs", offset: 100, limit: 100})
// ❌ Returns [100-199] = THE SAME LOGS as before!
// AI wastes time re-analyzing duplicate data
```

#### Why This Happens:
- Logs/WebSocket events are **prepended** (newest first)
- New data arrives constantly during debugging
- Offsets shift with each insertion
- AI gets stuck in infinite loop analyzing same data

#### Affected Buffers:
- ✅ **Logs** - High risk (constant appending)
- ✅ **WebSocket events** - High risk (constant appending)
- ⚠️ **Actions** - Medium risk (less frequent)
- ⚠️ **Network waterfall** - Low risk (finite page loads)
- ✅ **network_bodies** - Medium risk (ongoing requests)

---

## The Solution: Cursor-Based Pagination

### How Cursors Work

Instead of index-based offsets, use **timestamps**:

```javascript
// First request
observe({what: "logs", limit: 100})

// Response includes cursor (timestamp of oldest log returned)
{
  "entries": [
    {"message": "User clicked", "timestamp": "2026-01-30T10:15:23.456Z"},
    // ... 98 more logs ...
    {"message": "Page loaded", "timestamp": "2026-01-30T10:14:50.123Z"}
  ],
  "cursor": "2026-01-30T10:14:50.123Z",  // Timestamp of last returned log
  "count": 100,
  "has_more": true
}

// Second request uses cursor (stable even if new logs arrive)
observe({what: "logs", after_cursor: "2026-01-30T10:14:50.123Z", limit: 100})

// Returns logs OLDER than this timestamp
// New logs DON'T affect this query ✅
{
  "entries": [
    {"message": "API called", "timestamp": "2026-01-30T10:14:49.789Z"},
    // ... next 99 logs ...
  ],
  "cursor": "2026-01-30T10:13:20.456Z",
  "count": 100,
  "has_more": false
}
```

#### Key Difference:
- **Offset:** "Give me items at positions 100-200" (shifts with new data) ❌
- **Cursor:** "Give me items BEFORE timestamp 2026-01-30T10:14:50" (stable) ✅
- **LLM Awareness:** "I see logs from 10:15:23 to 10:14:50, need older logs" 🧠

---

## Implementation Design

### Use Composite Cursors (Timestamp + Sequence)

**Format:** `"timestamp:sequence"` → `"2026-01-30T10:15:23.456Z:1234"`

#### Rationale:
- **Timestamp first** - LLM knows temporal position ("logs from 10:15:23")
- **Sequence as tiebreaker** - Handles batched logs with identical timestamps
- **Stable** - Even if 100 logs arrive at exact same millisecond

#### Why composite vs pure timestamp:
Logs arrive in batches from browser extension, causing timestamp collisions:
```javascript
// Same millisecond batch from browser
{"message": "A", "timestamp": "2026-01-30T10:15:23.456Z", "sequence": 1002}
{"message": "B", "timestamp": "2026-01-30T10:15:23.456Z", "sequence": 1001}
{"message": "C", "timestamp": "2026-01-30T10:15:23.456Z", "sequence": 1000}
```
Pure timestamp cursor would skip B and C (data loss). Composite cursor prevents this.

### Timestamp Normalization (All Buffers)

**CRITICAL:** Different buffer types use different timestamp representations internally:

| Buffer | Internal Type | Example |
|--------|---------------|---------|
| Logs | `Timestamp string` | RFC3339 |
| WebSocket | `Timestamp string` | RFC3339 |
| Actions | `Timestamp int64` | Unix milliseconds |
| NetworkWaterfall | `Timestamp time.Time` | Go native |
| NetworkBodies | `Timestamp string` | RFC3339 |

**Solution:** Normalize to RFC3339 strings **at response serialization time** (in tools_core.go).

#### Example conversion:
```go
// Actions: int64 → RFC3339
timestamp := time.UnixMilli(action.Timestamp).Format(time.RFC3339)

// NetworkWaterfall: time.Time → RFC3339
timestamp := entry.Timestamp.Format(time.RFC3339)

// Logs/WebSocket: Already RFC3339 string (no conversion)
timestamp := entry.Timestamp
```

**Documented standard:** See [docs/core/timestamp-standard.md](../../../core/timestamp-standard.md)

### Concurrency Model (Extended Read Lock)

**Decision:** Use extended read lock during cursor operations (Option B).

#### Rationale:
- **No write concurrency** - Kaboom server is single-threaded for buffer writes
- **Simple** - No snapshot copy overhead, no algorithm changes
- **Negligible cost** - 1ms block time for 1000 entries, misses <0.1 logs at 100 logs/sec
- **Guarantees consistency** - No index shifts during iteration

#### Algorithm:
```go
func GetLogsAfterCursor(afterCursor string, limit int) ([]LogEntry, string, error) {
    server.mu.RLock()
    defer server.mu.RUnlock()  // Hold lock for entire operation

    // Parse cursor
    cursor := ParseCursor(afterCursor)  // "timestamp:sequence" → {Timestamp, Sequence}

    // Iterate with lock held (prevents buffer modification)
    var results []LogEntry
    for _, entry := range entries {
        if entry.IsOlderThan(cursor) {
            results = append(results, entry)
            if len(results) >= limit {
                break
            }
        }
    }

    return results, BuildCursor(results[len(results)-1]), nil
}
```

**Alternative rejected:** Snapshot copy (500KB memory overhead per request, unnecessary given no write concurrency)

### Schema Changes

#### Add cursor parameters to observe tool:

```go
"after_cursor": map[string]interface{}{
  "type": "string",
  "description": "Return entries older than this timestamp (exclusive). Cursor is the timestamp of the last entry from a previous request. Stable for live data. Format: ISO 8601 (e.g., \"2026-01-30T10:15:23.456Z\"). Applies to: logs, websocket_events, actions, network_bodies.",
},
"before_cursor": map[string]interface{}{
  "type": "string",
  "description": "Return entries newer than this timestamp (exclusive). For forward pagination (monitoring new data). Format: ISO 8601. Applies to: logs, websocket_events, actions, network_bodies.",
},
"since_cursor": map[string]interface{}{
  "type": "string",
  "description": "Return ALL entries newer than this timestamp (inclusive). Convenience method for \"show me everything since X\". Format: ISO 8601. Applies to: logs, websocket_events, actions, network_bodies.",
},
```

#### Keep offset/limit for backward compatibility.

### Response Format

#### Add cursor to all paginated responses:

```json
{
  "entries": [
    {"message": "...", "timestamp": "2026-01-30T10:15:23.456Z"},
    {"message": "...", "timestamp": "2026-01-30T10:14:50.123Z"}
  ],
  "count": 100,
  "cursor": "2026-01-30T10:14:50.123Z",  // NEW: Timestamp of last returned entry
  "has_more": true,
  "oldest_timestamp": "2026-01-30T09:00:00.000Z",  // NEW: Oldest log in buffer
  "newest_timestamp": "2026-01-30T10:15:23.456Z",  // NEW: Newest log in buffer

  // Keep existing fields for backward compat
  "total": 500,              // Current buffer size
  "offset": 100,             // For offset-based pagination
  "limit": 100
}
```

### Query Logic

#### Pseudo-code for timestamp-based cursor retrieval:

```go
func GetLogsAfterCursor(afterCursor string, limit int) ([]LogEntry, string, error) {
    server.mu.RLock()
    defer server.mu.RUnlock()

    // Parse cursor timestamp
    afterTime, err := time.Parse(time.RFC3339Nano, afterCursor)
    if err != nil {
        return nil, "", fmt.Errorf("invalid cursor: %w", err)
    }

    var results []LogEntry
    for _, entry := range entries {
        // Parse entry timestamp
        entryTime, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)

        // Get logs OLDER than cursor (backward pagination)
        if entryTime.Before(afterTime) {
            results = append(results, entry)
            if len(results) >= limit {
                break
            }
        }
    }

    // Return cursor = timestamp of last entry in results
    var newCursor string
    if len(results) > 0 {
        newCursor = results[len(results)-1].Timestamp
    }

    return results, newCursor, nil
}

// Forward pagination (get NEWER logs)
func GetLogsBeforeCursor(beforeCursor string, limit int) ([]LogEntry, string, error) {
    beforeTime, _ := time.Parse(time.RFC3339Nano, beforeCursor)

    var results []LogEntry
    // Iterate in REVERSE (newest first) to get newer logs
    for i := len(entries) - 1; i >= 0; i-- {
        entry := entries[i]
        entryTime, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)

        // Get logs NEWER than cursor
        if entryTime.After(beforeTime) {
            results = append(results, entry)
            if len(results) >= limit {
                break
            }
        }
    }

    var newCursor string
    if len(results) > 0 {
        newCursor = results[len(results)-1].Timestamp
    }

    return results, newCursor, nil
}
```

**Key:** Compare timestamps using `time.Before()` / `time.After()` for stable pagination

---

## Cursor Expiration Handling

### The Problem: Buffer Overflow

**Scenario:** Infinite recursion or runaway logging fills buffer faster than pagination can consume it.

```javascript
// LLM has cursor: "2026-01-30T10:14:50.123Z"
// But 10,000 logs/sec flooded the buffer
// That timestamp was evicted 5 seconds ago

observe({what: "logs", after_cursor: "2026-01-30T10:14:50.123Z", limit: 50})
// ❌ Cursor points to non-existent data!
```

### Solution: Hybrid Error + Retry Mechanism

#### When cursor is expired, return error with buffer state:

```javascript
{
  "error": {
    "code": "CURSOR_EXPIRED",
    "message": "Cursor expired: requested timestamp '2026-01-30T10:14:50.123Z' is older than oldest available log '2026-01-30T10:15:20.000Z'. Buffer overflow detected: logs evicted due to high insertion rate. Retry with restart_on_eviction=true to automatically recover.",
    "requested_timestamp": "2026-01-30T10:14:50.123Z",
    "oldest_available": "2026-01-30T10:15:20.000Z",
    "time_gap_seconds": 30
  },
  "buffer_state": {
    "oldest_timestamp": "2026-01-30T10:15:20.000Z",
    "newest_timestamp": "2026-01-30T10:15:25.123Z",
    "size": 1000,
    "insertion_rate_per_sec": 200  // Helps LLM detect runaway logging
  },
  "suggested_action": "restart_pagination"
}
```

#### LLM can retry with auto-recovery parameter:

```javascript
// Retry with automatic recovery
observe({
  what: "logs",
  after_cursor: "2026-01-30T10:14:50.123Z",
  limit: 50,
  restart_on_eviction: true  // Automatically restart from oldest available
})

// Returns oldest available logs instead of error
{
  "entries": [
    {"message": "...", "timestamp": "2026-01-30T10:15:20.000Z"},
    // ... next 49 logs ...
  ],
  "cursor": "2026-01-30T10:15:15.000Z",
  "cursor_restarted": true,  // Flag indicates cursor was reset
  "original_cursor": "2026-01-30T10:14:50.123Z",
  "logs_skipped": "30+ seconds of logs were evicted",
  "warning": "Buffer overflow detected - pagination restarted from oldest available log"
}
```

### Implementation

```go
func GetLogsAfterCursor(afterCursor string, restartOnEviction bool) ([]LogEntry, string, error) {
    server.mu.RLock()
    defer server.mu.RUnlock()

    afterTime, _ := time.Parse(time.RFC3339Nano, afterCursor)

    // Find oldest log in buffer
    var oldestTime time.Time
    if len(entries) > 0 {
        oldestTime, _ = time.Parse(time.RFC3339Nano, entries[len(entries)-1].Timestamp)
    }

    // Check if cursor was evicted
    if afterTime.Before(oldestTime) {
        if !restartOnEviction {
            // Return error with buffer state
            return nil, "", &CursorExpiredError{
                RequestedTimestamp: afterCursor,
                OldestAvailable:    entries[len(entries)-1].Timestamp,
                TimeGapSeconds:     int(oldestTime.Sub(afterTime).Seconds()),
                BufferState: BufferState{
                    OldestTimestamp: entries[len(entries)-1].Timestamp,
                    NewestTimestamp: entries[0].Timestamp,
                    Size:            len(entries),
                    InsertionRate:   calculateInsertionRate(),
                },
            }
        }

        // Auto-restart: return oldest available logs
        // (rest of pagination logic, but with warning metadata)
    }

    // Normal pagination...
}
```

### LLM Decision Tree

#### When cursor expires, LLM should:

1. **Detect runaway logging** - Check `insertion_rate_per_sec` in error
   - If >100/sec → "Buffer overflow! Investigating infinite loop..."
   - Analyze newest logs for error patterns

2. **Decide action:**
   - **High insertion rate** → Clear buffer, investigate root cause
   - **Normal rate** → Retry with `restart_on_eviction: true`

3. **Communicate to user:**
   - "Pagination interrupted due to buffer overflow"
   - "Skipped 30 seconds of logs (evicted)"
   - "Detected infinite recursion in foo() - recommend adding depth limit"

### Example: LLM Handling Buffer Overflow

```javascript
// LLM paginating through logs
observe({what: "logs", after_cursor: "2026-01-30T10:14:50.123Z", limit: 50})

// Receives CURSOR_EXPIRED error
// LLM analyzes buffer_state.insertion_rate_per_sec: 500

// LLM decides: "This is runaway logging, not normal pagination delay"

// Step 1: Get newest logs to diagnose
observe({what: "logs", limit: 100})
// Sees: "ReferenceError in foo() at line 42" × 100 times

// Step 2: Clear buffer to stop overflow
configure({what: "clear", buffer: "logs"})

// Step 3: Report to user
"Detected infinite recursion causing 500 logs/sec. Cleared buffer.
Root cause: ReferenceError in foo() at line 42. Recommend adding
stack depth limit or fixing the error."
```

---

## API Examples

### Logs (High-Priority Use Case)

#### Backward Pagination (Analyzing Historical Logs):

```javascript
// First request: Get most recent 50 logs
observe({what: "logs", limit: 50})

// Response
{
  "entries": [
    {"message": "User clicked button", "level": "info", "timestamp": "2026-01-30T10:15:23.456Z"},
    {"message": "API response received", "level": "debug", "timestamp": "2026-01-30T10:15:22.789Z"},
    // ... 48 more logs ...
    {"message": "Page loaded", "level": "info", "timestamp": "2026-01-30T10:14:50.123Z"}
  ],
  "cursor": "2026-01-30T10:14:50.123Z",  // Timestamp of oldest log in this page
  "count": 50,
  "has_more": true,
  "oldest_timestamp": "2026-01-30T09:00:00.000Z",  // Oldest in buffer
  "newest_timestamp": "2026-01-30T10:15:23.456Z"   // Newest in buffer
}

// LLM analyzes and says: "I see logs from 10:15:23 to 10:14:50, need to go back further"

// Second request: Get next 50 logs (older than cursor)
observe({what: "logs", after_cursor: "2026-01-30T10:14:50.123Z", limit: 50})

// Even if 100 new logs arrived at 10:16:00, cursor is stable
{
  "entries": [
    {"message": "Component mounted", "timestamp": "2026-01-30T10:14:49.999Z"},
    // ... next 49 logs ...
    {"message": "Init started", "timestamp": "2026-01-30T10:14:20.000Z"}
  ],
  "cursor": "2026-01-30T10:14:20.000Z",
  "count": 50,
  "has_more": true
}
```

#### Forward Pagination (Monitoring New Logs):

```javascript
// LLM has analyzed logs up to 10:15:23
// Now wants to check for NEW logs that arrived

observe({what: "logs", before_cursor: "2026-01-30T10:15:23.456Z", limit: 50})

// Returns logs NEWER than 10:15:23
{
  "entries": [
    {"message": "Error occurred!", "level": "error", "timestamp": "2026-01-30T10:16:05.123Z"},
    {"message": "User logout", "level": "info", "timestamp": "2026-01-30T10:16:01.456Z"},
    // ... logs between 10:15:23 and 10:16:05 ...
  ],
  "cursor": "2026-01-30T10:16:05.123Z",
  "count": 42,
  "has_more": false  // No newer logs exist (yet)
}

// LLM can say: "New error appeared at 10:16:05 - investigating..."
```

#### Convenience Method (All New Logs):

```javascript
// "Show me everything that happened since 10:15:23"
observe({what: "logs", since_cursor: "2026-01-30T10:15:23.456Z"})

// Returns ALL logs from 10:15:23 onwards (no pagination limit)
{
  "entries": [... all new logs ...],
  "count": 142,
  "cursor": "2026-01-30T10:16:05.123Z"  // Newest log
}
```

### WebSocket Events

```javascript
// Monitor WebSocket stream
observe({what: "websocket_events", limit: 100})

// Response
{
  "events": [
    {"type": "message", "data": "...", "timestamp": "2026-01-30T10:15:30.123Z"},
    // ... 99 more events ...
    {"type": "open", "timestamp": "2026-01-30T10:15:00.000Z"}
  ],
  "cursor": "2026-01-30T10:15:00.000Z",
  "count": 100,
  "has_more": true
}

// Get next batch (stable even if new events arrive)
observe({what: "websocket_events", after_cursor: "2026-01-30T10:15:00.000Z", limit: 100})

// LLM knows: "I'm looking at WebSocket events from 10:15:30 back to 10:15:00"
```

### Network Bodies (Lower Priority but Still Useful)

```javascript
// Analyze API calls
observe({what: "network_bodies", limit: 20})

{
  "network_request_response_pairs": [...],
  "cursor": 145,
  "count": 20,
  "has_more": true
}

// Get next batch
observe({what: "network_bodies", after_cursor: 145, limit: 20})
```

---

## Backward Compatibility

### Both Methods Supported

```javascript
// Old code (offset-based) - still works
observe({what: "logs", offset: 100, limit: 50})

// New code (cursor-based) - better for live data
observe({what: "logs", after_cursor: 12450, limit: 50})
```

**No breaking changes.** Offset/limit continue to work for static data.

### Migration Path

1. **v5.3:** Ship offset-based pagination (works for network_waterfall)
2. **v5.4:** Add cursor-based pagination (fixes logs/websocket)
3. **Documentation:** Recommend cursors for live data, offsets for static data
4. **AI learns:** Tool description guides AI to use correct method

---

## Comparison: Offset vs Cursor

| Feature | Offset-Based | Cursor-Based |
|---------|--------------|--------------|
| **Use Case** | Static data | Live/streaming data |
| **Stability** | ❌ Shifts with new data | ✅ Stable |
| **Complexity** | Simple | Medium |
| **Performance** | O(1) slice | O(n) scan |
| **Duplicates** | ❌ Possible | ✅ Never |
| **Backward Compat** | ✅ Yes | ✅ Yes (additive) |

### Decision:
- **network_waterfall:** Offset-based (finite page loads, low insertion rate)
- **logs, websocket_events:** Cursor-based (high insertion rate)
- **actions, network_bodies:** Either (depends on usage pattern)

---

## Implementation Plan

### Phase 1: Add Cursor to Responses (2 hours)

**Goal:** Every paginated response includes cursor field

#### Files:
- `cmd/browser-agent/tools_core.go` - Add cursor to response metadata
- Calculate cursor = last entry's ID

**No breaking changes** - just add new fields to existing responses.

### Phase 2: Add after_cursor Parameter (2 hours)

**Goal:** Accept after_cursor in observe() calls

#### Files:
- `cmd/browser-agent/tools_core.go` - Parse after_cursor parameter
- `cmd/browser-agent/main.go` (GetLogs) - Implement cursor-based retrieval
- `cmd/browser-agent/websocket.go` (GetWSEvents) - Implement cursor-based retrieval

#### Logic:
```go
if params.AfterCursor > 0 {
    // Cursor-based pagination
    return GetEntriesAfterCursor(params.AfterCursor, params.Limit)
} else {
    // Offset-based pagination (backward compat)
    return GetEntriesWithOffset(params.Offset, params.Limit)
}
```

### Phase 3: Update Tool Description (30 min)

#### Add to observe tool description:
```
Pagination modes:
- Offset-based (offset/limit): Best for static data like network_waterfall
- Cursor-based (after_cursor/limit): Required for live data like logs and websocket_events

Use cursor-based for logs and WebSocket events to avoid duplicate results when new data arrives during pagination.
```

### Phase 4: Testing (1 hour)

#### Unit tests:
```go
func TestCursorPaginationStableWithNewInserts(t *testing.T) {
    // Add 100 logs
    // Request page 1 (cursor = 100)
    // Add 100 NEW logs
    // Request page 2 (after_cursor = 100)
    // Verify: No duplicates, cursor is stable
}
```

**Total Effort:** 5-6 hours

---

## Success Criteria

### Must Have

- ✅ Cursor-based pagination works for logs
- ✅ Cursor-based pagination works for websocket_events
- ✅ No duplicates even when new data arrives
- ✅ Backward compatible (offset/limit still work)
- ✅ Tool description guides AI to use correct method

### Nice to Have

- 📋 Cursor-based pagination for all modes (can defer)
- 📋 Bidirectional cursors (before_cursor for newer items)

---

## Testing Strategy

### Unit Tests

#### Test: Cursor stability with concurrent inserts
```go
func TestCursorStabilityWithInserts(t *testing.T) {
    server := setupTestServer(t)
    baseTime := time.Now()

    // Add 100 logs (T0 to T99 seconds)
    for i := 0; i < 100; i++ {
        server.AddLog(LogEntry{
            Message: fmt.Sprintf("Log %d", i),
            Timestamp: baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano),
        })
    }

    // Get page 1 (most recent 50 logs)
    resp1 := getLogsAfterCursor("", 50)
    assert.Equal(t, 50, len(resp1.Entries))
    cursor1 := resp1.Cursor // Should be timestamp of 50th log

    // Simulate concurrent inserts (100 NEW logs at T100-T199)
    for i := 100; i < 200; i++ {
        server.AddLog(LogEntry{
            Message: fmt.Sprintf("Log %d", i),
            Timestamp: baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano),
        })
    }

    // Get page 2 (should be stable - returns logs BEFORE cursor1)
    resp2 := getLogsAfterCursor(cursor1, 50)
    assert.Equal(t, 50, len(resp2.Entries))

    // Verify: No overlap with page 1
    // resp1 has timestamps [T99...T50], resp2 should have [T49...T0]
    assert.NotEqual(t, resp1.Entries[0].Timestamp, resp2.Entries[0].Timestamp)

    // Verify cursor is before all entries in resp2
    cursor1Time, _ := time.Parse(time.RFC3339Nano, cursor1)
    for _, entry := range resp2.Entries {
        entryTime, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
        assert.True(t, entryTime.Before(cursor1Time), "Entry should be before cursor")
    }
}

func TestForwardPagination(t *testing.T) {
    server := setupTestServer(t)
    baseTime := time.Now()

    // Add initial 100 logs
    for i := 0; i < 100; i++ {
        server.AddLog(LogEntry{
            Message: fmt.Sprintf("Initial log %d", i),
            Timestamp: baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano),
        })
    }

    // Get initial batch
    resp1 := getLogsAfterCursor("", 50)
    newestTimestamp := resp1.Entries[0].Timestamp

    // Add 50 NEW logs
    for i := 100; i < 150; i++ {
        server.AddLog(LogEntry{
            Message: fmt.Sprintf("New log %d", i),
            Timestamp: baseTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano),
        })
    }

    // Get new logs using before_cursor (forward pagination)
    resp2 := getLogsBeforeCursor(newestTimestamp, 50)

    // Verify: All entries are AFTER newestTimestamp
    newestTime, _ := time.Parse(time.RFC3339Nano, newestTimestamp)
    assert.Equal(t, 50, len(resp2.Entries))
    for _, entry := range resp2.Entries {
        entryTime, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
        assert.True(t, entryTime.After(newestTime), "Entry should be after cursor")
    }
}
```

### Integration Test

```bash
# Terminal 1: Start Kaboom
./kaboom --port 7890

# Terminal 2: Generate continuous logs
while true; do
  curl -X POST http://localhost:7890/logs \
    -d '{"level":"info","message":"Test log","timestamp":"2026-01-30T10:00:00Z"}'
  sleep 0.1
done

# Terminal 3: Test cursor pagination (via Claude Code)
observe({what: "logs", limit: 50})
# Note the cursor value

# Wait 5 seconds (500 new logs arrive)

observe({what: "logs", after_cursor: <cursor>, limit: 50})
# Verify: No duplicates, stable results
```

---

## Migration Guide

### For AI Agents

#### Before (v5.3 - prone to duplicates):
```javascript
// Page 1
observe({what: "logs", offset: 0, limit: 100})

// Page 2 (may have duplicates if new logs arrived)
observe({what: "logs", offset: 100, limit: 100})
```

#### After (v5.4 - stable with temporal awareness):
```javascript
// Page 1
const page1 = await observe({what: "logs", limit: 100})
// {
//   entries: [...],
//   cursor: "2026-01-30T10:14:50.123Z",
//   newest_timestamp: "2026-01-30T10:15:23.456Z"
// }

// LLM understands: "I have logs from 10:15:23 to 10:14:50"

// Page 2 (stable even with new logs)
const page2 = await observe({what: "logs", after_cursor: page1.cursor, limit: 100})
// {
//   entries: [...],
//   cursor: "2026-01-30T10:14:20.000Z"
// }

// LLM understands: "Now I have logs from 10:14:50 to 10:14:20" ✅

// Monitor for new logs
const newLogs = await observe({what: "logs", before_cursor: page1.newest_timestamp, limit: 50})
// Returns logs AFTER 10:15:23
```

### For Developers

#### Choosing the right method:
- **Static data** (network_waterfall): Use `offset`/`limit`
- **Live data** (logs, websocket_events): Use `after_cursor`/`limit`

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Performance degradation (O(n) scan) | Low | Low | Monotonic counters are indexed, scan is fast |
| AI confused by two pagination methods | Medium | Medium | Clear tool description, recommend cursor for live data |
| Evicted logs cause cursor gaps | Low | Low | Document that cursors may skip evicted entries |

---

## Dependencies

### Before This Feature

- ✅ v5.3 shipped (offset-based pagination)
- ✅ Monotonic counters exist (logTotalAdded, etc.)

### Blocks

- ❌ Proper log streaming for AI debugging
- ❌ WebSocket event analysis during live sessions

### Enables

- ✅ Reliable log pagination during debugging
- ✅ WebSocket monitoring without duplicates
- ✅ Streaming telemetry analysis

---

## Approval

**Product:** ✅ Approved (fixes critical v5.3 bug)
**Engineering:** 📋 Needs implementation
**Effort:** 5-6 hours
**Target:** v5.4 (immediately after v5.3)

### Next Steps:
1. Ship v5.3 with offset-based pagination (acceptable for network_waterfall)
2. Implement cursor-based pagination in v5.4
3. Update documentation to recommend cursors for live data
4. Add integration tests for stability

---

## Conclusion

**The Insight:** Offset-based pagination fails for live data because indexes shift.

**The Fix:** Cursor-based pagination using stable monotonic IDs.

### Impact:
- ✅ No duplicate results during log analysis
- ✅ Reliable WebSocket event streaming
- ✅ Better AI debugging experience
- ✅ Backward compatible (offset/limit still work)

### Timeline:
- v5.3: Ship offset-based (good enough for network_waterfall)
- v5.4: Add cursor-based (required for logs/websocket)
- Total delay: <1 week to implement cursors

**User feedback validated:** "New logs are created during pagination" → Fixed with cursors.
