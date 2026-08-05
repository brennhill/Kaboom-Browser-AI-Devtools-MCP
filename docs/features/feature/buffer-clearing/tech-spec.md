---
status: proposed
scope: feature/buffer-clearing/implementation
ai-priority: high
tags: [implementation, architecture]
relates-to: [product-spec.md, qa-plan.md]
last-verified: 2026-03-05
doc_type: tech-spec
feature_id: feature-buffer-clearing
last_reviewed: 2026-08-05
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Buffer-Specific Clearing - Technical Spec

**Feature:** Buffer-Specific Clearing
**Version:** v5.3
**Created:** 2026-01-30
**Effort:** 1-2 hours

---

## Architecture Overview

Add `buffer` parameter to `configure({what: "clear"})` tool. Implement buffer-specific clear methods in Capture struct. Return counts of cleared items.

**Key Principle:** Clearing is **destructive and immediate**. No undo, no transactions.

---

## Implementation Plan

### Phase 1: Schema Updates (15 min)

**File:** `cmd/browser-agent/tools_core.go`

Add `buffer` parameter to configure tool schema:

```go
// Line ~770: Update configure tool inputSchema
"buffer": map[string]interface{}{
  "type":        "string",
  "description": "Which buffer to clear (applies to action: \"clear\"). Valid values: \"network\" (network_waterfall + network_bodies), \"websocket\" (websocket_events + websocket_status), \"actions\" (user interactions), \"logs\" (console + extension logs), \"all\" (everything). Default: \"logs\" (backward compatible).",
  "enum":        []string{"network", "websocket", "actions", "logs", "inbox", "all"},
},
```

Update tool description:
```go
Description: "...\n\nBuffer clearing: configure({what: \"clear\", buffer: \"network\"}) clears network buffers. buffer: \"all\" clears everything. Omit buffer parameter to clear logs only (backward compatible).",
```

### Phase 2: Add Clear Methods to Capture (30 min)

**File:** `cmd/browser-agent/types.go` or new `cmd/browser-agent/buffer_clear.go`

#### BufferClearCounts Struct

```go
// BufferClearCounts holds counts of cleared items from each buffer
type BufferClearCounts struct {
	NetworkWaterfall int `json:"network_waterfall,omitempty"`
	NetworkBodies    int `json:"network_bodies,omitempty"`
	WebSocketEvents  int `json:"websocket_events,omitempty"`
	WebSocketStatus  int `json:"websocket_status,omitempty"`
	Actions          int `json:"actions,omitempty"`
	Logs             int `json:"logs,omitempty"`
	ExtensionLogs    int `json:"extension_logs,omitempty"`
}

// Total returns sum of all cleared items
func (c *BufferClearCounts) Total() int {
	return c.NetworkWaterfall + c.NetworkBodies + c.WebSocketEvents +
		c.WebSocketStatus + c.Actions + c.Logs + c.ExtensionLogs
}
```

#### Clear Methods

```go
// ClearNetworkBuffers coordinates the two independent network owners.
func (s *telemetrystore.Store) ClearNetworkBuffers() types.BufferClearCounts {
	return types.BufferClearCounts{
		NetworkWaterfall: s.NetworkWaterfall().Clear(),
		NetworkBodies:    s.NetworkBodies().Clear(),
	}
}

// The canonical owner clears retained events and derived connections together.
webSocketCounts := targets.Capture.Telemetry().WebSockets().Clear()

// The configure owner clears actions through the canonical action store.
actionsCleared := targets.Capture.Telemetry().Actions().Clear()

// ClearLogBuffers clears console logs and extension logs (Server + Capture)
func (s *Server, c *Capture) ClearLogBuffers() BufferClearCounts {
	s.mu.Lock()
	logCount := len(s.entries)
	s.entries = make([]LogEntry, 0)
	s.logAddedAt = make([]time.Time, 0)
	s.logTotalAdded = 0
	s.mu.Unlock()

	c.mu.Lock()
	extLogCount := len(c.extensionLogs)
	c.extensionLogs = make([]ExtensionLogEntry, 0)
	c.mu.Unlock()

	return BufferClearCounts{
		Logs:          logCount,
		ExtensionLogs: extLogCount,
	}
}

// ClearAllBuffers clears all buffers (network, websocket, actions, logs)
func (s *Server, c *Capture) ClearAllBuffers() BufferClearCounts {
	networkCounts := c.ClearNetworkBuffers()
	wsCounts := c.Telemetry().WebSockets().Clear()
	actionsCleared := c.Telemetry().Actions().Clear()
	logCounts := ClearLogBuffers(s, c)

	return BufferClearCounts{
		NetworkWaterfall: networkCounts.NetworkWaterfall,
		NetworkBodies:    networkCounts.NetworkBodies,
		WebSocketEvents:  wsCounts.Events,
		WebSocketStatus:  wsCounts.Connections,
		Actions:          actionsCleared,
		Logs:             logCounts.Logs,
		ExtensionLogs:    logCounts.ExtensionLogs,
	}
}
```

### Phase 3: Update configure Handler (15-30 min)

**File:** `cmd/browser-agent/tools_core.go`

Update `toolConfigureDismiss` → create new `toolConfigureClear`:

```go
func (h *ToolHandler) toolConfigureClear(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Buffer string `json:"buffer"` // "network", "websocket", "actions", "logs", "all"
	}
	_ = json.Unmarshal(args, &params)

	// Default to "logs" for backward compatibility
	if params.Buffer == "" {
		params.Buffer = "logs"
	}

	var counts BufferClearCounts
	var bufferName string

	switch params.Buffer {
	case "network":
		counts = h.capture.ClearNetworkBuffers()
		bufferName = "network"

	case "websocket":
		wsCounts := h.capture.Telemetry().WebSockets().Clear()
		counts.WebSocketEvents = wsCounts.Events
		counts.WebSocketStatus = wsCounts.Connections
		bufferName = "websocket"

	case "actions":
		counts.Actions = h.capture.Telemetry().Actions().Clear()
		bufferName = "actions"

	case "logs":
		counts = ClearLogBuffers(h.server, h.capture)
		bufferName = "logs"

	case "all":
		counts = ClearAllBuffers(h.server, h.capture)
		bufferName = "all"

	default:
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidParam,
				fmt.Sprintf("Invalid buffer: %s", params.Buffer),
				"Use one of: network, websocket, actions, logs, all",
				withParam("buffer"),
			),
		}
	}

	data := map[string]interface{}{
		"cleared":       bufferName,
		"counts":        counts,
		"total_cleared": counts.Total(),
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	summary := fmt.Sprintf("Cleared %s buffer(s): %d total items", bufferName, counts.Total())
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, data)}
}
```

Register `clear` with the canonical `internal/toolconfigure.Dispatcher`.
Composition supplies a handler that calls `toolconfigure.HandleClear` with
explicit capture, log, inbox, and annotation owners; no `ToolHandler`
forwarding method or central switch is retained.

---

## Testing Plan

### Unit Tests

**File:** `cmd/browser-agent/buffer_clear_test.go` (NEW)

```go
func TestClearNetworkBuffers(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add data
	capture.AddNetworkWaterfall([]NetworkWaterfallEntry{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
	})
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://example.com/1"},
	})

	// Clear
	counts := capture.Telemetry().ClearNetworkBuffers()

	// Verify counts
	assert.Equal(t, 2, counts.NetworkWaterfall)
	assert.Equal(t, 1, counts.NetworkBodies)
	assert.Equal(t, 3, counts.Total())

	// Verify buffers empty
	assert.Empty(t, capture.Telemetry().NetworkWaterfall().Entries())
	assert.Empty(t, capture.Telemetry().NetworkBodies().Snapshot().Bodies)
	assert.Equal(t, int64(0), capture.Telemetry().NetworkBodies().Stats().TotalAdded)
}

func TestWebSocketStoreClear(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add WS events
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Event: "open", ID: "conn1", URL: "ws://localhost"},
		{Event: "message", ID: "conn1", Direction: "outgoing", Data: "test"},
		{Event: "message", ID: "conn1", Direction: "incoming", Data: "response"},
	})

	// Clear
	counts := capture.Telemetry().WebSockets().Clear()

	// Verify counts
	assert.Equal(t, 3, counts.Events)
	assert.Equal(t, 1, counts.Connections)

	// Verify buffers empty
	assert.Equal(t, 0, capture.Telemetry().WebSockets().Stats().Count)
	assert.Equal(t, 0, capture.Telemetry().WebSockets().Stats().ConnectionCount)
}

func TestActionStoreClear(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add actions
	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: "2026-01-30T10:00:00Z"},
		{Type: "input", Timestamp: "2026-01-30T10:00:01Z"},
	})

	// Clear
	actionsCleared := capture.Telemetry().Actions().Clear()

	// Verify counts
	assert.Equal(t, 2, actionsCleared)

	// Verify buffer empty
	assert.Equal(t, 0, capture.Telemetry().Actions().Stats().Count)
}

func TestClearLogBuffers(t *testing.T) {
	t.Parallel()
	server := setupTestServer(t)
	capture := setupTestCapture(t)

	// Add logs
	server.AddLog(LogEntry{Level: "info", Message: "test"})
	server.AddLog(LogEntry{Level: "error", Message: "error"})

	// Add extension logs
	capture.mu.Lock()
	capture.extensionLogs = append(capture.extensionLogs, ExtensionLogEntry{Level: "debug", Message: "ext log"})
	capture.mu.Unlock()

	// Clear
	counts := ClearLogBuffers(server, capture)

	// Verify counts
	assert.Equal(t, 2, counts.Logs)
	assert.Equal(t, 1, counts.ExtensionLogs)

	// Verify buffers empty
	server.mu.RLock()
	assert.Equal(t, 0, len(server.entries))
	server.mu.RUnlock()

	capture.mu.RLock()
	assert.Equal(t, 0, len(capture.extensionLogs))
	capture.mu.RUnlock()
}

func TestClearAllBuffers(t *testing.T) {
	t.Parallel()
	server := setupTestServer(t)
	capture := setupTestCapture(t)

	// Add data to all buffers
	capture.AddNetworkWaterfall([]NetworkWaterfallEntry{{URL: "test"}})
	capture.AddWSEvents([]WebSocketEvent{{Data: "test"}})
	capture.AddEnhancedActions([]EnhancedAction{{Type: "click"}})
	server.AddLog(LogEntry{Message: "test"})

	// Clear all
	counts := ClearAllBuffers(server, capture)

	// Verify all counts
	assert.Equal(t, 1, counts.NetworkWaterfall)
	assert.Equal(t, 1, counts.WebSocketEvents)
	assert.Equal(t, 1, counts.Actions)
	assert.Equal(t, 1, counts.Logs)
	assert.Equal(t, 4, counts.Total())

	// Verify all buffers empty
	assert.Equal(t, 0, len(capture.networkWaterfall))
	assert.Equal(t, 0, capture.Telemetry().WebSockets().Stats().Count)
	assert.Equal(t, 0, capture.Telemetry().Actions().Stats().Count)
	assert.Equal(t, 0, len(server.entries))
}
```

### Integration Tests

**File:** `cmd/browser-agent/tools_test.go`

```go
func TestToolConfigureClearNetwork(t *testing.T) {
	t.Parallel()
	server := setupTestServer(t)
	capture := setupTestCapture(t)
	handler := NewToolHandler(server, capture, nil)

	// Add network data
	capture.AddNetworkWaterfall([]NetworkWaterfallEntry{{URL: "https://example.com"}})

	// Clear via MCP tool
	args := json.RawMessage(`{"action": "clear", "buffer": "network"}`)
	resp := handler.configureDispatcher.Handle(JSONRPCRequest{ID: json.RawMessage(`1`)}, args)

	// Verify response
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	assert.Contains(t, result.Content[0].Text, `"cleared": "network"`)
	assert.Contains(t, result.Content[0].Text, `"network_waterfall": 1`)

	// Verify buffer empty
	assert.Equal(t, 0, len(capture.networkWaterfall))
}

func TestToolConfigureClearBackwardCompatible(t *testing.T) {
	t.Parallel()
	server := setupTestServer(t)
	capture := setupTestCapture(t)
	handler := NewToolHandler(server, capture, nil)

	// Add logs
	server.AddLog(LogEntry{Message: "test"})

	// Clear without buffer parameter (backward compatible)
	args := json.RawMessage(`{"action": "clear"}`)
	resp := handler.configureDispatcher.Handle(JSONRPCRequest{ID: json.RawMessage(`1`)}, args)

	// Verify logs cleared
	assert.Equal(t, 0, len(server.entries))

	// Verify response
	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	assert.Contains(t, result.Content[0].Text, `"cleared": "logs"`)
}

func TestToolConfigureClearInvalidBuffer(t *testing.T) {
	t.Parallel()
	server := setupTestServer(t)
	capture := setupTestCapture(t)
	handler := NewToolHandler(server, capture, nil)

	// Try invalid buffer
	args := json.RawMessage(`{"action": "clear", "buffer": "invalid"}`)
	resp := handler.configureDispatcher.Handle(JSONRPCRequest{ID: json.RawMessage(`1`)}, args)

	// Verify error
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "Invalid buffer")
}
```

### Manual UAT

See [product-spec.md](./product-spec.md#manual-uat)

---

## Edge Cases & Error Handling

### Empty Buffers

```go
// Clear empty buffer
configure({what: "clear", buffer: "network"})
→ Returns: {cleared: "network", counts: {}, total_cleared: 0}
```
**Not an error** - Clearing empty buffer is valid operation.

### Concurrent Modifications

**Problem:** Data arriving while clearing

**Solution:** Mutex locks ensure atomicity. Clear is instantaneous. New data after clear will start fresh buffer.

```go
c.mu.Lock()
defer c.mu.Unlock()
// Clear operations are atomic
```

### Invalid Buffer Name

```go
configure({what: "clear", buffer: "xyz"})
→ Error: "Invalid buffer: xyz. Use one of: network, websocket, actions, logs, all"
```

---

## Performance Considerations

### Memory

- **Before clear:** Buffers consume 10-100MB depending on activity
- **After clear:** Buffers consume ~1KB (empty slices)
- **Savings:** Immediate memory reclamation by Go GC

### CPU

- **Clear operation:** O(1) - just replace slices
- **Total latency:** <1ms per buffer

### Network

- No network impact (server-side operation)

---

## Backward Compatibility

### Existing Code

```javascript
// Old code (no buffer parameter)
configure({what: "clear"})
```
**Behavior:** Clears logs only (current behavior)
**Unchanged:** ✅

### New Code

```javascript
// New code (with buffer parameter)
configure({what: "clear", buffer: "network"})
```
**Behavior:** Clears network buffers
**New feature:** ✅

---

## Migration Checklist

### Code Changes

- [ ] Add `BufferClearCounts` struct to types.go
- [ ] Add `ClearNetworkBuffers()` to Capture
- [ ] Clear WebSocket state directly through `Telemetry().WebSockets().Clear()`
- [ ] Clear actions directly through `Telemetry().Actions().Clear()`
- [ ] Add `ClearLogBuffers()` helper
- [ ] Add `ClearAllBuffers()` helper
- [ ] Add `buffer` parameter to configure tool schema
- [ ] Add `toolConfigureClear()` handler
- [ ] Update `internal/toolconfigure.Dispatcher`

### Testing

- [ ] Write unit tests for each clear method
- [ ] Write integration tests for MCP tool
- [ ] Manual UAT with real browser data
- [ ] Test edge cases (empty buffers, invalid names)

### Documentation

- [ ] Update configure tool description
- [ ] Add buffer clearing examples to docs
- [ ] Update CHANGELOG.md

### Quality Gates

- [ ] `go vet ./cmd/browser-agent/` passes
- [ ] `make test` passes (all unit tests)
- [ ] Manual UAT checklist completed
- [ ] Backward compatibility verified

---

## Files Modified

| File | Lines Changed | Description |
|------|---------------|-------------|
| `cmd/browser-agent/buffer_clear.go` | +200 | NEW: Clear methods and helpers |
| `cmd/browser-agent/buffer_clear_test.go` | +300 | NEW: Unit tests |
| `cmd/browser-agent/tools_core.go` | +80 | Add buffer param, toolConfigureClear |
| `cmd/browser-agent/tools_test.go` | +100 | Integration tests |
| **Total** | **~680 lines** | **1-2 hours** |

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
- Buffer clearing used in >25% of sessions
- Reduced "reload extension" workarounds

---

## Related Specs

- [product-spec.md](./product-spec.md) - Product requirements
- [Pagination product-spec.md](../pagination/product-spec.md) - Complementary feature
- [roadmap.md](../../../roadmap.md) - v5.3 planning

---

## Approval

**Engineering:** ✅ Approved
**Effort Estimate:** 1-2 hours
**Target:** v5.3

### Next Steps:
1. Implement clear methods in Capture
2. Write unit tests (TDD)
3. Update configure tool handler
4. Integration testing
5. Manual UAT
6. Documentation updates
