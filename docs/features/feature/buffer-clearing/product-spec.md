---
status: proposed
scope: feature/buffer-clearing
ai-priority: medium
tags: [feature]
relates-to: [tech-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: product-spec
feature_id: feature-buffer-clearing
last_reviewed: 2026-07-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Buffer-Specific Clearing - Product Spec

**Feature:** Buffer-Specific Clearing
**Priority:** ⭐⭐⭐ HIGH
**Version:** v5.3
**Status:** Approved for Implementation
**Created:** 2026-01-30

---

## Problem Statement

### The Issue

`configure({action: "clear"})` only clears console logs. All other buffers accumulate indefinitely, causing:
- Memory bloat (WebSocket events, network bodies, actions pile up)
- Stale data confusion (old requests mixed with new)
- Token limit issues (buffers grow too large to return)

#### Current behavior:
```javascript
configure({action: "clear"})
```
✅ Clears: console logs
❌ Does NOT clear: network_waterfall, network_bodies, websocket_events, actions, extension_logs

### Who It Affects

- **AI agents** needing fresh context after task completion
- **Developers** debugging new scenarios on same page
- **Long-running sessions** where buffers fill up over hours

### Current Workarounds (All Bad)

1. ❌ **Reload extension** - Loses all state, breaks workflows
2. ❌ **Refresh page** - Not always possible (loses app state)
3. ❌ **Live with stale data** - Confuses AI, wastes tokens
4. ❌ **Restart Kaboom server** - Nuclear option, loses everything

---

## Solution

Add **`buffer` parameter** to `configure({action: "clear"})` for granular clearing.

### API Design

```javascript
// Clear specific buffer
configure({action: "clear", buffer: "network"})
configure({action: "clear", buffer: "websocket"})
configure({action: "clear", buffer: "actions"})
configure({action: "clear", buffer: "logs"})

// Clear all buffers
configure({action: "clear", buffer: "all"})

// Backward compatible: clear logs only (current behavior)
configure({action: "clear"}) // Same as buffer: "logs"
```

### Buffer Categories

| Buffer Value | Clears | Use Case |
|--------------|--------|----------|
| `"network"` | network_waterfall + network_bodies | Fresh network debugging session |
| `"websocket"` | websocket_events + websocket_status | WebSocket reconnect testing |
| `"actions"` | enhancedActions | New user flow recording |
| `"logs"` | console logs + extension_logs | Start fresh debugging |
| `"all"` | Everything | Complete reset without reload |

#### Do NOT clear:
- Page state (DOM, vitals) - Read-only snapshots
- Tracking status - Managed by extension
- Server config - Persistent settings

---

## User Experience

### Before (v5.2)

```javascript
// Test scenario 1: login flow
observe({what: "network_waterfall"})
// → 50 requests (good)

// Now test scenario 2: logout flow
configure({action: "clear"}) // Only clears logs, not network
observe({what: "network_waterfall"})
// → 100 requests (50 from login + 50 from logout) ❌ Confusing!
```

### After (v5.3)

```javascript
// Test scenario 1: login flow
observe({what: "network_waterfall"})
// → 50 requests (good)

// Clear network buffer before new test
configure({action: "clear", buffer: "network"})

// Now test scenario 2: logout flow
observe({what: "network_waterfall"})
// → 50 requests (only logout) ✅ Clear context!
```

---

## Success Criteria

### Must Have

- ✅ `buffer: "network"` clears network buffers
- ✅ `buffer: "websocket"` clears WebSocket buffers
- ✅ `buffer: "actions"` clears action replay buffer
- ✅ `buffer: "logs"` clears console + extension logs
- ✅ `buffer: "all"` clears all buffers
- ✅ No `buffer` parameter = backward compatible (clears logs only)
- ✅ Returns confirmation with count of cleared items

### Nice to Have

- 📋 Clear by time range (last N minutes) - Defer to v6.0
- 📋 Selective clearing by URL filter - Defer to v6.0

---

## Edge Cases

### Invalid Buffer Name

```javascript
configure({action: "clear", buffer: "invalid"})
```
Returns: Error with valid buffer names

### Empty Buffers

```javascript
configure({action: "clear", buffer: "network"})
```
Returns: `{cleared: "network", count: 0}` (not an error)

### Clearing While Capture Active

```javascript
// Page is still loading, requests coming in
configure({action: "clear", buffer: "network"})
```
**Behavior:** Clears current buffer. New requests continue to arrive.
**Note:** This is expected. Clearing is instantaneous, not a pause.

---

## Response Format

```json
{
  "cleared": "network",
  "counts": {
    "network_waterfall": 150,
    "network_bodies": 45
  },
  "total_cleared": 195,
  "timestamp": "2026-01-30T10:30:00.000Z"
}
```

### For `buffer: "all"`:
```json
{
  "cleared": "all",
  "counts": {
    "network_waterfall": 150,
    "network_bodies": 45,
    "websocket_events": 230,
    "websocket_status": 3,
    "actions": 12,
    "logs": 500,
    "extension_logs": 25
  },
  "total_cleared": 965,
  "timestamp": "2026-01-30T10:30:00.000Z"
}
```

---

## Implementation Complexity

**Effort:** 1-2 hours

### Easy Parts (95% of work)

- Add `buffer` parameter to configure tool schema
- Parse buffer parameter
- Call existing clear methods
- Return counts

### Straightforward Parts (5% of work)

- Add `ClearNetworkBuffers()` method to Capture
- Add `ClearWebSocketBuffers()` method to Capture
- Add `ClearActionBuffer()` method to Capture
- Update MCP tool description

### Not Required

- ❌ No new storage structures
- ❌ No complex state management
- ❌ No backward compatibility issues

---

## Testing Strategy

### Unit Tests

```go
func TestClearNetworkBuffers(t *testing.T) {
	capture := setupTestCapture(t)

	// Add network data
	capture.AddNetworkWaterfall([]NetworkWaterfallEntry{{URL: "https://example.com"}})
	capture.AddNetworkBodies([]NetworkBody{{URL: "https://example.com"}})

	// Clear
	counts := capture.ClearNetworkBuffers()

	assert.Equal(t, 1, counts.NetworkWaterfall)
	assert.Equal(t, 1, counts.NetworkBodies)
	assert.Equal(t, 0, len(capture.networkWaterfall))
	assert.Equal(t, 0, len(capture.networkBodies))
}

func TestClearWebSocketBuffers(t *testing.T) {
	// Similar test for WebSocket buffers
}

func TestClearAllBuffers(t *testing.T) {
	capture := setupTestCapture(t)

	// Add data to all buffers
	// ...

	// Clear all
	counts := capture.ClearAllBuffers()

	// Verify all buffers are empty
	assert.Equal(t, 0, len(capture.networkWaterfall))
	assert.Equal(t, 0, len(capture.wsEvents))
	assert.Equal(t, 0, len(capture.enhancedActions))
	// ...
}
```

### Integration Tests

```go
func TestConfigureClearBuffer(t *testing.T) {
	server := setupTestServer(t)
	capture := setupTestCapture(t)
	handler := NewToolHandler(server, capture, nil)

	// Add data
	capture.AddNetworkWaterfall([]NetworkWaterfallEntry{{URL: "test"}})

	// Clear via MCP tool
	args := json.RawMessage(`{"action": "clear", "buffer": "network"}`)
	resp := handler.toolConfigure(JSONRPCRequest{ID: json.RawMessage(`1`)}, args)

	// Verify response
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	assert.Contains(t, result.Content[0].Text, `"cleared": "network"`)
	assert.Contains(t, result.Content[0].Text, `"network_waterfall": 1`)

	// Verify buffer is empty
	assert.Equal(t, 0, len(capture.networkWaterfall))
}
```

### Manual UAT

```bash
# 1. Generate data
# Load page with network activity, WebSocket connections, user interactions

# 2. Verify data exists
observe({what: "network_waterfall"})
# → Returns entries

observe({what: "websocket_events"})
# → Returns events

# 3. Clear network buffer
configure({action: "clear", buffer: "network"})
# → Returns count of cleared items

# 4. Verify network buffer is empty
observe({what: "network_waterfall"})
# → Returns empty array

# 5. Verify other buffers unaffected
observe({what: "websocket_events"})
# → Still returns events

# 6. Clear all buffers
configure({action: "clear", buffer: "all"})

# 7. Verify all buffers empty
observe({what: "websocket_events"})
# → Returns empty array
```

---

## Migration Path

### Backward Compatibility

✅ **100% backward compatible**

Old code continues to work:
```javascript
configure({action: "clear"}) // Clears logs only (current behavior)
```

New code gets granular clearing:
```javascript
configure({action: "clear", buffer: "network"}) // New feature
```

### Documentation Updates

- Update `configure` tool description
- Add buffer clearing examples to docs
- Update UAT test plan
- Add to CHANGELOG.md

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Accidentally clearing needed data | Medium | Medium | Clear confirmation messages, undo not planned (buffers refill quickly) |
| Confusion about buffer names | Low | Low | Clear tool description, error messages list valid names |
| Race condition (clear while data arriving) | Low | Low | Document as expected behavior |

---

## Dependencies

### Before This Feature

- ✅ v5.2.5 released
- ✅ All UAT issues fixed

### After This Feature

- 📋 Pagination (can implement in parallel)
- 📋 tabId in responses (in progress)

### Not Blocked By

- ❌ Time-range clearing (separate feature)
- ❌ Selective clearing by filter (separate feature)

---

## Metrics for Success

### Primary:
- Buffer clearing used in >25% of debugging sessions
- Reduced token usage from stale data

### Secondary:
- Fewer "refresh page" workarounds
- Clearer AI debugging sessions (less context pollution)

---

## Open Questions

1. **Q:** Should clearing return undo capability?
   **A:** No - buffers refill quickly, undo adds complexity

2. **Q:** Should we support clearing by time range (last N minutes)?
   **A:** Not in v5.3 - defer to v6.0 if needed

3. **Q:** What if buffer is being filled while clearing?
   **A:** Clear is instantaneous, new data continues arriving

---

## Related Features

- **Pagination** - Complementary feature for viewing large buffers
- **tabId in responses** - Helps AI understand data source
- **Time-range filtering** - Alternative to clearing (deferred)

---

## Approval

**Product:** ✅ Approved
**Engineering:** ✅ Approved
**Effort:** 1-2 hours
**Target:** v5.3

### Next Steps:
1. Create tech-spec.md with implementation details
2. Write tests first (TDD)
3. Implement buffer clearing methods
4. Update configure tool handler
5. Manual UAT testing
6. Documentation updates
