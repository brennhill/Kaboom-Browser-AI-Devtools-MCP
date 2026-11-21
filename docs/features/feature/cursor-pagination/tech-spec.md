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
last_reviewed: 2026-11-21
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Technical Specification: Cursor-Based Pagination

**Status:** SHIPPED in v5.3
**Canonical Reference:** Codebase (`internal/pagination/pagination.go`, `internal/tools/observe/logs/logs.go`)
**Last Verified:** 2026-01-30

---

## Overview

Cursor-based pagination was implemented in v5.3 to solve token limit problems. This specification documents the shipped implementation.

### Core Implementation Details

#### Where it lives:
- Pagination engine: `internal/pagination/pagination.go` (`ApplyCursorPagination`, `ParseCursor`, `BuildCursor`, eviction handling) and `internal/pagination/entries.go` (per-stream sequence enrichment and wrappers)
- Observe handler wired to it: `GetBrowserLogs` in `internal/tools/observe/logs/logs.go`
- Response metadata (`cursor`, `cursor_restarted`, `original_cursor`): `internal/tools/observe/core/metadata.go`
- Tests: `internal/pagination/pagination_test.go`, `internal/pagination/cursor_test.go`, `internal/tools/observe/logs/logs_edge_test.go`

#### Supported data streams:
- `logs` — fully wired: cursor params are parsed and applied at runtime.
- `network_waterfall`, `network_bodies`, `websocket_events`, `actions` — the mode specs (`internal/tools/configure/capabilities/modespecs_observe.go`) advertise `after_cursor` / `before_cursor` / `since_cursor` / `restart_on_eviction`, and `internal/pagination/entries.go` has WebSocket and action helpers (`ApplyWebSocketCursorPagination`, `ApplyActionCursorPagination`), but the handlers in `internal/tools/observe/network/network.go` and `internal/tools/observe/session/session.go` do not currently apply them; these modes honor `limit` only.
- `errors` — not paginated (limit only).

### Key Parameters

MCP arguments (snake_case): `after_cursor`, `before_cursor`, `since_cursor`, `limit`, `restart_on_eviction`. Internally they map to:

```go
type CursorParams struct {
  AfterCursor       string // entries older than cursor
  BeforeCursor      string // entries newer than cursor
  SinceCursor       string // all entries at or after cursor (no limit)
  Limit             int
  RestartOnEviction bool
}

// Cursor format: "timestamp:sequence" (or ":sequence" when no timestamp)
// Example: "2026-01-30T10:15:23.456Z:1234"
```

If more than one cursor is supplied, precedence is `after` > `before` > `since`.

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

Each data stream has its own bounded ring buffer (the log buffer defaults to `MaxLogEntries` = 1000 in `internal/serverdefaults/defaults.go`). Entries carry a monotonically increasing sequence derived from the buffer's total-added counter. When the buffer is full, the oldest entries are evicted; a cursor whose sequence is older than the oldest retained entry is expired and returns an error unless `restart_on_eviction=true`, in which case pagination restarts from the oldest entry and the metadata carries a warning with the lost sequence range.

### Performance Characteristics

- Pagination: O(n) over the filtered buffer — `filterByCursor` is a linear scan comparing each entry's timestamp/sequence to the cursor, then the limit is applied
- Cursors are stateless strings; the server stores no per-cursor state
- Benchmarks: `internal/pagination/pagination_bench_test.go`

---

## Known Limitations

1. **Cursor expiration** — Very old cursors may not be findable if buffer evicted entries
2. **Single-direction iteration** — Can't iterate both forward and backward from same cursor
3. **Offset + Limit not combined** — Use cursor-based pagination for large datasets

---

## Testing

All pagination scenarios are covered in:
- **Core tests:** `internal/pagination/pagination_test.go`, `internal/pagination/cursor_test.go`, `internal/pagination/pagination_bench_test.go`
- **Handler tests:** `internal/tools/observe/logs/logs_edge_test.go`
- **UAT scenarios:** the v5.3 UAT checklist was retired; see `docs/core/quality/comprehensive-uat-plan.md`

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
- **../../../core/quality/comprehensive-uat-plan.md** — Comprehensive UAT plan

---

## For Implementers

When building on cursor pagination:
- Always use cursor format `timestamp:sequence` for consistency
- Set `restart_on_eviction=true` for long-running agents
- Test with buffers near capacity (stress test with 5000+ entries)
- Account for cursor eviction in error handling
