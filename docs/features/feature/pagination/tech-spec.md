---
status: proposed
scope: feature/pagination/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-pagination
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Pagination for Large Datasets - Technical Spec

**Feature:** Pagination for Large Datasets
**Version:** v5.3
**Created:** 2026-01-30
**Effort:** 2-4 hours

---

## Architecture Overview

Add `offset` and `limit` parameters to existing observe() modes that return array data. Implement slicing at the Go layer before JSON serialization.

**Key Principle:** Pagination is **read-time filtering**, not storage changes. Buffers remain unchanged.

---

## Implementation Plan

### Phase 1: Schema Updates (30 min)

**File:** `cmd/browser-agent/tools_core.go`

Add offset/limit parameters to observe tool schema:

```go
// Line ~760: Update observe tool inputSchema
"offset": map[string]interface{}{
  "type":        "number",
  "description": "Starting index for pagination (0-indexed). Negative values count from end (e.g., -50 = last 50 entries). Applies to: network_waterfall, network_bodies, logs, websocket_events, actions, extension_logs.",
},
"limit": map[string]interface{}{
  "type":        "number",
  "description": "Maximum entries to return. 0 or omitted = return all. Applies to: network_waterfall, network_bodies, logs, websocket_events, actions, extension_logs.",
},
```

Update tool description to mention pagination:
```go
Description: "Read current browser state with pagination support. Use offset and limit to page through large datasets...",
```

### Phase 2: Helper Function (30 min)

**File:** `cmd/browser-agent/pagination.go` (NEW)

```go
package main

// PaginationParams holds offset and limit for slicing arrays
type PaginationParams struct {
	Offset int
	Limit  int
}

// PaginationResult holds the paginated slice and metadata
type PaginationResult struct {
	Data       interface{} // The paginated slice
	Count      int         // Items in this page
	Total      int         // Total items in buffer
	Offset     int         // Actual offset used
	Limit      int         // Actual limit used
	HasMore    bool        // More items available
	NextOffset int         // Offset for next page (0 if no more)
}

// ApplyPagination slices a generic array with offset/limit support
// Handles:
// - Negative offsets (count from end)
// - Offset beyond total (returns empty)
// - Limit=0 (returns all)
// - Metadata for pagination state
func ApplyPagination(data interface{}, params PaginationParams) PaginationResult {
	// Use reflection to get slice length
	// Handle negative offset: offset = total + offset
	// Clamp offset to [0, total]
	// Apply limit
	// Calculate metadata
	// Return result with metadata
}
```

#### Implementation:

```go
func ApplyPagination(data interface{}, params PaginationParams) PaginationResult {
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Slice {
		panic("ApplyPagination requires a slice")
	}

	total := v.Len()
	offset := params.Offset
	limit := params.Limit

	// Handle negative offset (last N entries)
	if offset < 0 {
		offset = total + offset
		if offset < 0 {
			offset = 0
		}
		// When using negative offset, limit is implied if not set
		if limit == 0 {
			limit = total - offset
		}
	}

	// Clamp offset
	if offset > total {
		offset = total
	}

	// Calculate end index
	end := total
	if limit > 0 {
		end = offset + limit
		if end > total {
			end = total
		}
	}

	// Slice the data
	sliced := v.Slice(offset, end).Interface()
	count := end - offset

	// Calculate metadata
	hasMore := end < total
	nextOffset := 0
	if hasMore {
		nextOffset = end
	}

	return PaginationResult{
		Data:       sliced,
		Count:      count,
		Total:      total,
		Offset:     offset,
		Limit:      limit,
		HasMore:    hasMore,
		NextOffset: nextOffset,
	}
}
```

### Phase 3: Update observe() Handlers (60-90 min)

Apply pagination to 6 modes: `network_waterfall`, `network_bodies`, `logs`, `websocket_events`, `actions`, `extension_logs`.

#### Example: network_waterfall

**File:** `cmd/browser-agent/network.go`

##### Before:
```go
func (h *ToolHandler) toolGetNetworkWaterfall(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL   string `json:"url"`
		Limit int    `json:"limit"` // REMOVE THIS - replaced by pagination
	}
	_ = json.Unmarshal(args, &params)

	// ... filtering logic ...

	// Apply old limit
	if params.Limit > 0 && len(entries) > params.Limit {
		entries = entries[len(entries)-params.Limit:]
	}

	return JSONRPCResponse{...}
}
```

##### After:
```go
func (h *ToolHandler) toolGetNetworkWaterfall(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL    string `json:"url"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	_ = json.Unmarshal(args, &params)

	// ... existing filtering logic (URL, etc.) ...

	// Apply pagination
	paginated := ApplyPagination(entries, PaginationParams{
		Offset: params.Offset,
		Limit:  params.Limit,
	})

	data := map[string]interface{}{
		"entries":     paginated.Data,
		"count":       paginated.Count,
		"total":       paginated.Total,
		"offset":      paginated.Offset,
		"limit":       paginated.Limit,
		"has_more":    paginated.HasMore,
		"next_offset": paginated.NextOffset,
		// ... existing metadata ...
	}

	summary := fmt.Sprintf("%d of %d network waterfall entries", paginated.Count, paginated.Total)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, data)}
}
```

#### Apply Same Pattern To:

1. **network_bodies** (`cmd/browser-agent/network.go:185`)
   - Parse offset/limit from args
   - Apply pagination after filtering
   - Add metadata to response

2. **logs** (`cmd/browser-agent/tools_core.go` - toolGetLogs)
   - Parse offset/limit from args
   - Apply pagination after filtering
   - Add metadata to response

3. **websocket_events** (`cmd/browser-agent/websocket.go` - toolGetWSEvents)
   - Parse offset/limit from args
   - Apply pagination after filtering
   - Add metadata to response

4. **actions** (`cmd/browser-agent/actions.go` - toolGetActions)
   - Parse offset/limit from args
   - Apply pagination after filtering
   - Add metadata to response

5. **extension_logs** (`cmd/browser-agent/tools_core.go` - toolGetExtensionLogs)
   - Parse offset/limit from args
   - Apply pagination after filtering
   - Add metadata to response

---

## Testing Plan

### Unit Tests

**File:** `cmd/browser-agent/pagination_test.go` (NEW)

```go
func TestApplyPagination_Basic(t *testing.T) {
	data := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	// First page
	result := ApplyPagination(data, PaginationParams{Offset: 0, Limit: 5})
	assert.Equal(t, []int{0, 1, 2, 3, 4}, result.Data)
	assert.Equal(t, 5, result.Count)
	assert.Equal(t, 10, result.Total)
	assert.True(t, result.HasMore)
	assert.Equal(t, 5, result.NextOffset)

	// Second page
	result = ApplyPagination(data, PaginationParams{Offset: 5, Limit: 5})
	assert.Equal(t, []int{5, 6, 7, 8, 9}, result.Data)
	assert.Equal(t, 5, result.Count)
	assert.False(t, result.HasMore)
	assert.Equal(t, 0, result.NextOffset)
}

func TestApplyPagination_NegativeOffset(t *testing.T) {
	data := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	// Last 3 entries
	result := ApplyPagination(data, PaginationParams{Offset: -3, Limit: 0})
	assert.Equal(t, []int{7, 8, 9}, result.Data)
	assert.Equal(t, 3, result.Count)
	assert.Equal(t, 10, result.Total)
	assert.False(t, result.HasMore)
}

func TestApplyPagination_OffsetBeyondTotal(t *testing.T) {
	data := []int{0, 1, 2, 3, 4}

	result := ApplyPagination(data, PaginationParams{Offset: 100, Limit: 10})
	assert.Equal(t, []int{}, result.Data)
	assert.Equal(t, 0, result.Count)
	assert.Equal(t, 5, result.Total)
	assert.False(t, result.HasMore)
}

func TestApplyPagination_ZeroLimit(t *testing.T) {
	data := []int{0, 1, 2, 3, 4}

	result := ApplyPagination(data, PaginationParams{Offset: 0, Limit: 0})
	assert.Equal(t, data, result.Data)
	assert.Equal(t, 5, result.Count)
	assert.False(t, result.HasMore)
}
```

### Integration Tests

**File:** `cmd/browser-agent/network_test.go`

```go
func TestNetworkWaterfallPagination(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)
	server := setupTestServer(t)
	handler := NewToolHandler(server, capture, nil)

	// Add 200 network waterfall entries
	entries := make([]NetworkWaterfallEntry, 200)
	for i := range entries {
		entries[i] = NetworkWaterfallEntry{
			URL:      fmt.Sprintf("https://example.com/api/%d", i),
			Method:   "GET",
			Duration: 100,
		}
	}
	capture.AddNetworkWaterfall(entries)

	// Test first page
	args := json.RawMessage(`{"limit": 50}`)
	resp := handler.toolGetNetworkWaterfall(JSONRPCRequest{ID: json.RawMessage(`1`)}, args)
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	data := result.Content[0].Text // JSON data

	assert.Contains(t, data, `"count": 50`)
	assert.Contains(t, data, `"total": 200`)
	assert.Contains(t, data, `"has_more": true`)
	assert.Contains(t, data, `"next_offset": 50`)

	// Test second page
	args = json.RawMessage(`{"offset": 50, "limit": 50}`)
	resp = handler.toolGetNetworkWaterfall(JSONRPCRequest{ID: json.RawMessage(`2`)}, args)
	// ... verify second page ...

	// Test last entries
	args = json.RawMessage(`{"offset": -10}`)
	resp = handler.toolGetNetworkWaterfall(JSONRPCRequest{ID: json.RawMessage(`3`)}, args)
	// ... verify last 10 entries ...
}
```

### Manual UAT

```bash
# 1. Start Kaboom
./kaboom --port 7890

# 2. In Claude Code, test pagination
observe({what: "network_waterfall", limit: 10})
# Verify: Returns 10 entries, metadata shows total and has_more

observe({what: "network_waterfall", offset: 10, limit: 10})
# Verify: Returns next 10 entries

observe({what: "network_waterfall", offset: -5})
# Verify: Returns last 5 entries

observe({what: "logs", limit: 20})
# Verify: Returns 20 log entries with pagination metadata
```

---

## Edge Cases & Error Handling

### Invalid Parameters

```go
// Negative limit
observe({what: "network_waterfall", limit: -10})
→ Treat as limit: 0 (return all)

// Float offset/limit
observe({what: "network_waterfall", offset: 10.5, limit: 5.5})
→ JSON unmarshal truncates to int (offset: 10, limit: 5)

// Offset > total
observe({what: "network_waterfall", offset: 1000, limit: 10})
→ Return empty array with metadata showing total
```

### Empty Buffers

```go
// No data captured yet
observe({what: "network_waterfall", offset: 0, limit: 100})
→ Return: {entries: [], count: 0, total: 0, has_more: false}
```

### Concurrent Modifications

**Problem:** Buffer changes between paginated calls

**Solution:** Accept inconsistency. Document that pagination is a snapshot, not transactional.

**Future:** Cursor-based pagination (v6.0+) solves this with stable cursors.

---

## Performance Considerations

### Memory

- **Before:** Return entire buffer (200-500 entries × ~2KB = 400KB-1MB)
- **After:** Return paginated slice (50 entries × ~2KB = 100KB)
- **Savings:** 75-90% reduction in response size

### CPU

- **Slicing:** O(1) with offset
- **Reflection:** Negligible overhead (<1μs per call)
- **Total:** <1ms additional latency

### Network

- Smaller responses = faster MCP roundtrips
- Enables AI to analyze data within token limits

---

## Backward Compatibility

### Existing Code

```javascript
// Old code (no offset/limit)
observe({what: "network_waterfall"})
```
**Behavior:** Returns all entries (up to buffer size)
**Unchanged:** ✅

### New Code

```javascript
// New code (with pagination)
observe({what: "network_waterfall", limit: 100})
```
**Behavior:** Returns first 100 entries with metadata
**New feature:** ✅

### Mixed Usage

```javascript
// First call: no pagination
observe({what: "network_waterfall"})

// Second call: with pagination
observe({what: "network_waterfall", limit: 50})
```
**Behavior:** Both work independently
**No conflict:** ✅

---

## Migration Checklist

### Code Changes

- [ ] Create `pagination.go` with `ApplyPagination` helper
- [ ] Add `offset` and `limit` to observe tool schema
- [ ] Update `network_waterfall` handler
- [ ] Update `network_bodies` handler
- [ ] Update `logs` handler
- [ ] Update `websocket_events` handler
- [ ] Update `actions` handler
- [ ] Update `extension_logs` handler

### Testing

- [ ] Write unit tests for `ApplyPagination`
- [ ] Write integration tests for each mode
- [ ] Manual UAT with real browser data
- [ ] Test edge cases (negative offset, beyond total, etc.)

### Documentation

- [ ] Update MCP tool description for observe
- [ ] Add pagination examples to docs
- [ ] Update CHANGELOG.md
- [ ] Update UAT test plan

### Quality Gates

- [ ] `make compile-ts` passes
- [ ] `go vet ./cmd/browser-agent/` passes
- [ ] `make test` passes (all unit tests)
- [ ] Manual UAT checklist completed
- [ ] No performance regression

---

## Files Modified

| File | Lines Changed | Description |
|------|---------------|-------------|
| `cmd/browser-agent/pagination.go` | +150 | NEW: Pagination helper |
| `cmd/browser-agent/pagination_test.go` | +200 | NEW: Unit tests |
| `cmd/browser-agent/tools_core.go` | +10 | Add offset/limit to schema |
| `cmd/browser-agent/network.go` | +30 | Apply pagination to network_waterfall, network_bodies |
| `cmd/browser-agent/websocket.go` | +20 | Apply pagination to websocket_events |
| `cmd/browser-agent/actions.go` | +20 | Apply pagination to actions |
| **Total** | **~430 lines** | **2-4 hours** |

---

## Deployment Plan

1. **Merge to `UNSTABLE` branch**
2. **Run full test suite**
3. **Manual UAT testing**
4. **Update documentation**
5. **Merge to `stable` for v5.3 release**

---

## Success Metrics

### Pre-Release:
- All unit tests pass
- Integration tests pass
- Manual UAT completes successfully

### Post-Release:
- Token limit errors decrease to <5% of previous rate
- AI successfully analyzes pages with 200+ requests
- No bug reports related to pagination

---

## Open Questions

**Q:** Should we add pagination to `errors` mode?
**A:** No - errors are already filtered subset of logs

**Q:** Should we use 0-indexed or 1-indexed offsets?
**A:** 0-indexed (consistent with JavaScript arrays)

**Q:** What about cursor-based pagination?
**A:** Defer to v6.0+ (current solution is sufficient for v5.3)

---

## Related Specs

- [product-spec.md](./product-spec.md) - Product requirements
- [Buffer Clearing product-spec.md](../buffer-clearing/product-spec.md) - Complementary feature
- [roadmap.md](../../../roadmap.md) - v5.3 planning

---

## Approval

**Engineering:** ✅ Approved
**Effort Estimate:** 2-4 hours
**Target:** v5.3

### Next Steps:
1. Implement `pagination.go` helper
2. Write unit tests (TDD)
3. Update all 6 observe modes
4. Integration testing
5. Manual UAT
6. Documentation updates
