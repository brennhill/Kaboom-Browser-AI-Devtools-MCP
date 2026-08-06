---
status: shipped
version-applies-to: v5.3+
scope: feature/cursor-pagination
ai-priority: high
tags: [pagination, cursor-based, shipped, core-feature]
relates-to: [feature-proposal.md, qa-plan.md, product-spec.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-cursor-pagination
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Technical Specification: Cursor-Based Pagination

**Status:** SHIPPED in v5.3
**Canonical Reference:** Codebase (cmd/browser-agent/tools_core.go)
**Last Verified:** 2026-01-30

---

## Overview

Cursor-based pagination was implemented in v5.3 to solve token limit problems. This specification documents the shipped implementation.

### Core Implementation Details

#### Where it lives:
- Go implementation: `cmd/browser-agent/tools_core.go` (paginate utilities and observe handlers)
- Test file: `cmd/browser-agent/composite_tools_test.go`

#### Supported data streams:
- `errors` (logs with pagination)
- `logs` (console logs with cursor support)
- `network_waterfall` (HTTP requests)
- `network_bodies` (request/response bodies)
- `websocket_events` (WebSocket messages)
- `actions` (user interactions)

### Key Parameters

```go
type ObserveRequest struct {
  What string
  // Pagination
  After_cursor string   // Return entries older than cursor
  Before_cursor string  // Return entries newer than cursor
  Since_cursor string   // Return all entries since cursor (no limit)

  // Filtering & limiting
  Limit int             // Max entries to return
  Offset int            // Skip N entries before applying limit
  Head_limit int        // Max entries (alternative to limit)
  // ... other params
}

// Cursor format: "timestamp:sequence"
// Example: "2026-01-30T10:15:23.456Z:1234"
```

### Cursor Format & Stability

- **Format:** `timestamp:sequence` (ISO 8601 timestamp + sequence number)
- **Stability:** Cursors are stable for LIVE data
- **Eviction:** Older cursors may expire if buffer overflows
- **Restart on eviction:** Use `restart_on_eviction=true` parameter to auto-recover

---

## Implementation Details

### Pagination Logic

#### Forward iteration (after_cursor):
- Return entries OLDER than cursor
- Used for "load more" / scroll down
- Cursor moves backward in time

#### Backward iteration (before_cursor):
- Return entries NEWER than cursor
- Used for "new updates" / scroll up
- Cursor moves forward in time

#### Snapshot reading (since_cursor):
- Return ALL entries since cursor (no limit)
- Single call to read all new data
- Convenience method for "show me everything since X"

### Buffer Management

Each data stream has its own circular buffer:
- **logs buffer:** ~500 entries
- **network_waterfall:** ~300 requests
- **websocket_events:** ~200 messages
- **actions:** ~1000 interactions

When buffer is full, oldest entries are evicted. Cursors for evicted entries become invalid.

### Performance Characteristics

- Pagination: O(n) where n = entries returned (not scanned)
- Cursor lookup: O(log n) with binary search optimization
- Memory overhead: ~200 bytes per cursor position

---

## Known Limitations

1. **Cursor expiration** — Very old cursors may not be findable if buffer evicted entries
2. **Single-direction iteration** — Can't iterate both forward and backward from same cursor
3. **Offset + Limit not combined** — Use cursor-based pagination for large datasets

---

## Testing

All pagination scenarios are covered in:
- **Core test:** `cmd/browser-agent/composite_tools_test.go`
- **UAT scenarios:** `docs/core/quality/uat-v5.3-checklist.md`

Tested:
- ✓ Basic pagination with after_cursor
- ✓ Reverse pagination with before_cursor
- ✓ Snapshot reading with since_cursor
- ✓ Cursor expiration handling
- ✓ Large buffer handling (5000+ entries)
- ✓ Performance regression prevention

---

## Related Documents

- **product-spec.md** — Feature requirements
- **feature-proposal.md** — Original proposal
- **../../../core/quality/uat-v5.3-checklist.md** — Comprehensive UAT tests
- **ADR-cursor-pagination.md** — Architecture decision record

---

## For Implementers

When building on cursor pagination:
- Always use cursor format `timestamp:sequence` for consistency
- Set `restart_on_eviction=true` for long-running agents
- Test with buffers near capacity (stress test with 5000+ entries)
- Account for cursor eviction in error handling
