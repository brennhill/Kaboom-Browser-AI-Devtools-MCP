---
status: proposed
scope: feature/pagination
ai-priority: medium
tags: [feature]
relates-to: [tech-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-pagination
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Pagination for Large Datasets - Product Spec

**Feature:** Pagination for Large Datasets
**Priority:** ⭐⭐⭐ HIGH
**Version:** v5.3
**Status:** Approved for Implementation
**Created:** 2026-01-30

---

## Problem Statement

### The Issue

Network waterfall and other buffers frequently exceed MCP token limits, preventing AI from analyzing captured data.

#### Real-world example:
```javascript
observe({what: "network_waterfall"})
```
Returns: `440,000+ characters` — **exceeds Claude's MCP token limit**

Result: AI cannot analyze network traffic, defeating the purpose of Kaboom.

### Who It Affects

- **AI agents** debugging network issues
- **Developers** using Claude Code with Kaboom
- **Anyone** working on pages with >100 network requests

### Current Workarounds (All Bad)

1. ❌ **Clear buffers frequently** - Loses historical context
2. ❌ **Use URL filters** - Requires knowing what to filter for
3. ❌ **Give up and use Chrome DevTools** - Defeats the purpose

---

## Solution

Add **offset** and **limit** parameters to all observe() modes that return large datasets.

### API Design

```javascript
// Get first 100 entries
observe({what: "network_waterfall", limit: 100})

// Get next 100 entries (pagination)
observe({what: "network_waterfall", offset: 100, limit: 100})

// Get last 50 entries
observe({what: "network_waterfall", offset: -50})

// Get all (current behavior, but now explicit)
observe({what: "network_waterfall"}) // or limit: 0
```

### Affected Modes

Apply pagination to these observe() modes:

| Mode | Current Limit | Typical Size | Token Impact |
|------|---------------|--------------|--------------|
| `network_waterfall` | None | 200-500 entries | 200K-500K chars |
| `network_bodies` | 100 | Variable | 50K-200K chars |
| `logs` | 1000 | Variable | 50K-500K chars |
| `websocket_events` | 1000 | Variable | 100K-1M chars |
| `actions` | 500 | 10-100 entries | 10K-100K chars |
| `extension_logs` | 500 | 10-50 entries | 5K-50K chars |

#### Do NOT apply to:
- `page` - Single object
- `vitals` - Single snapshot
- `tabs` - Small array
- `pilot` - Status object
- `errors` - Already filtered subset of logs

---

## User Experience

### Before (v5.2)

```javascript
observe({what: "network_waterfall"})
```
❌ Returns 440K characters → Token limit error → AI cannot help

### After (v5.3)

```javascript
// AI automatically pages through data
observe({what: "network_waterfall", limit: 100}) // First page
// Analyzes, finds suspicious request at entry #250
observe({what: "network_waterfall", offset: 200, limit: 100}) // Get page containing #250
// Inspects specific request
```

✅ AI can analyze large datasets incrementally
✅ Stays within token limits
✅ Preserves full context

---

## Success Criteria

### Must Have

- ✅ `limit` parameter works on all 6 affected modes
- ✅ `offset` parameter enables pagination
- ✅ Negative offset works (get last N entries)
- ✅ Default behavior unchanged (no `limit` = return all, up to buffer size)
- ✅ Metadata shows total count and pagination state

### Nice to Have (Deferred to v5.4)

- 📋 **Cursor-based pagination** - REQUIRED for logs/websocket_events (append-only buffers)
- 📋 Server-side aggregation (summary views)

**CRITICAL:** Offset-based pagination has a flaw for live data (logs, WebSocket events). When new items are added, offsets shift, causing duplicate results. Must implement cursor-based pagination for append-only buffers in v5.4.

---

## Edge Cases

### Empty Results

```javascript
observe({what: "network_waterfall", offset: 1000, limit: 100})
```
Returns: `{entries: [], count: 0, total: 500, offset: 1000, limit: 100}`

### Offset Beyond Total

```javascript
// Total is 200
observe({what: "network_waterfall", offset: 500, limit: 100})
```
Returns: `{entries: [], count: 0, total: 200, offset: 500, limit: 100}`

### Negative Offset

```javascript
observe({what: "network_waterfall", offset: -50})
```
Returns: Last 50 entries (equivalent to `offset: total - 50, limit: 50`)

### Zero Limit

```javascript
observe({what: "network_waterfall", limit: 0})
```
Returns: All entries (current behavior)

---

## Metadata Response Format

```json
{
  "entries": [...],
  "count": 100,           // Entries in this page
  "total": 450,           // Total entries in buffer
  "offset": 100,          // Starting index
  "limit": 100,           // Page size
  "has_more": true,       // More entries available
  "next_offset": 200,     // Offset for next page
  "tracked_tab_id": 42    // v5.3 feature
}
```

---

## Implementation Complexity

**Effort:** 2-4 hours

### Easy Parts (90% of work)

- Add `offset` and `limit` parameters to JSON schema
- Update parameter parsing in tools_core.go
- Slice arrays before returning
- Add metadata to responses

### Medium Parts (10% of work)

- Handle negative offsets (last N entries)
- Add pagination metadata to all response types
- Update MCP tool descriptions

### Not Required

- ❌ Database changes (in-memory slicing is fine)
- ❌ Cursor-based pagination (can defer to v6.0)
- ❌ Compression (pagination solves the problem)

---

## Testing Strategy

### Unit Tests

```go
func TestPaginationOffsetLimit(t *testing.T) {
  // Test: offset=0, limit=10 returns first 10
  // Test: offset=10, limit=10 returns next 10
  // Test: offset=100, limit=10 (beyond total) returns empty
  // Test: offset=-10 returns last 10
  // Test: limit=0 returns all
}
```

### Integration Tests

```javascript
// Populate buffer with 500 entries
observe({what: "network_waterfall"}) // Should return all 500
observe({what: "network_waterfall", limit: 100}) // Should return first 100
observe({what: "network_waterfall", offset: 100, limit: 100}) // Should return 100-200
observe({what: "network_waterfall", offset: -50}) // Should return last 50
```

### Manual UAT

1. Load page with 200+ network requests
2. `observe({what: "network_waterfall", limit: 50})` - verify first 50
3. `observe({what: "network_waterfall", offset: 50, limit: 50})` - verify next 50
4. Check metadata: `total: 200, count: 50, has_more: true`

---

## Migration Path

### Backward Compatibility

✅ **100% backward compatible**

Old code continues to work:
```javascript
observe({what: "network_waterfall"}) // No change in behavior
```

New code gets pagination:
```javascript
observe({what: "network_waterfall", limit: 100}) // New feature
```

### Documentation Updates

- Update MCP tool description for `observe`
- Add pagination examples to docs
- Update UAT test plan
- Add to CHANGELOG.md

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| AI doesn't understand pagination | Low | High | Clear tool description, metadata guides AI |
| Performance impact on large buffers | Low | Low | Slicing is O(1) with offset |
| Confusion with existing `limit` parameter | Medium | Low | Keep naming consistent across modes |
| Edge cases break AI workflows | Low | Medium | Comprehensive edge case testing |

---

## Dependencies

### Before This Feature

- ✅ v5.2.5 released
- ✅ All UAT issues fixed

### After This Feature

- 📋 Buffer-specific clearing (can implement in parallel)
- 📋 tabId in all observe responses (in progress)

### Not Blocked By

- ❌ Server-side aggregation (separate feature)
- ❌ Cursor-based pagination (future optimization)

---

## Metrics for Success

### Primary:
- Token limit errors decrease to near zero
- AI successfully analyzes pages with 200+ requests

### Secondary:
- Average observe() response size < 100K characters
- Pagination used in >50% of network_waterfall calls

---

## CRITICAL: Offset Pagination Flaw for Live Data

### The Problem

**Discovered:** 2026-01-30 during spec review

#### Scenario:
```javascript
// T0: Buffer has 100 logs [0-99]
observe({what: "logs", limit: 100})
// → Returns logs [0-99]

// T1: 100 new logs arrive during analysis
// Buffer now: [0-199], where [0-99] are the NEW logs, [100-199] are the OLD logs

observe({what: "logs", offset: 100, limit: 100})
// → Returns logs [100-199] = THE SAME LOGS AS BEFORE ❌
```

#### Impact:
- Logs: High (constantly appending)
- WebSocket events: High (constantly appending)
- Actions: Medium (less frequent)
- Network: Low (finite page loads)

### The Solution: Cursor-Based Pagination

#### Instead of offset (index), use cursor (ID/timestamp):

```javascript
// First request
observe({what: "logs", limit: 100})
// Returns: {
//   entries: [...],
//   cursor: 500, // logTotalAdded monotonic counter
//   count: 100,
//   has_more: true
// }

// Second request (stable even if new logs arrive)
observe({what: "logs", after_cursor: 500, limit: 100})
// Returns logs with ID > 500, regardless of new insertions
```

#### Advantages:
- ✅ Stable pagination even with live data
- ✅ No duplicate results
- ✅ Works with append-only buffers
- ✅ Efficient (no re-indexing)

#### Implementation:
- Use canonical monotonic counters, including `Telemetry().Actions().Stats().TotalAdded`
- Add `cursor` and `after_cursor` parameters
- Keep offset/limit for backward compatibility (static data like network_waterfall)

#### Timeline:
- v5.3: Ship offset-based pagination for network_waterfall (finite data, acceptable)
- v5.4: Add cursor-based pagination for logs/websocket_events (required for live data)

---

## Open Questions

1. **Q:** Should we add pagination to `errors` mode?
   **A:** No - errors are already a filtered subset of logs

2. **Q:** Should offset be 0-indexed or 1-indexed?
   **A:** 0-indexed (consistent with JavaScript arrays)

3. **Q:** What if buffer changes between paginated calls?
   **A:** Offset-based: Accept inconsistency (v5.3). Cursor-based: Stable (v5.4).

4. **Q:** Should v5.3 include cursor-based pagination?
   **A:** No - too complex for initial release. Offset-based works for network_waterfall (finite). Ship cursors in v5.4 for logs/websocket.

---

## Related Features

- **Buffer-specific clearing** - Complementary feature for managing buffer size
- **tabId in responses** - Helps AI understand data source
- **Server-side aggregation** - Alternative approach for large datasets (deferred)

---

## Approval

**Product:** ✅ Approved
**Engineering:** ✅ Approved
**Effort:** 2-4 hours
**Target:** v5.3

### Next Steps:
1. Create tech-spec.md with implementation details
2. Write tests first (TDD)
3. Implement pagination for all 6 modes
4. Update MCP tool descriptions
5. Manual UAT testing
6. Documentation updates
Human: continue