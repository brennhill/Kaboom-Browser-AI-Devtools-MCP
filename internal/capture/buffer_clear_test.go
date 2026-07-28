// Purpose: Tests for capture buffer clearing and reset behavior.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

// TestClearNetworkBuffers verifies clearing network_waterfall and network_bodies buffers.
func TestClearNetworkBuffers(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add network data directly to buffers
	capture.mu.Lock()
	capture.networkWaterfall.entries = []types.NetworkWaterfallEntry{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
	}
	capture.mu.Unlock()

	capture.AddNetworkBodies([]types.NetworkBody{
		{URL: "https://example.com/1"},
	})

	// Verify data exists
	capture.mu.RLock()
	initialWaterfall := len(capture.networkWaterfall.entries)
	initialBodies := len(capture.buffers.networkBodies)
	capture.mu.RUnlock()

	if initialWaterfall != 2 {
		t.Fatalf("Expected 2 waterfall entries, got %d", initialWaterfall)
	}
	if initialBodies != 1 {
		t.Fatalf("Expected 1 body entry, got %d", initialBodies)
	}

	// Clear
	counts := capture.ClearNetworkBuffers()

	// Verify counts
	if counts.NetworkWaterfall != 2 {
		t.Errorf("Expected NetworkWaterfall count = 2, got %d", counts.NetworkWaterfall)
	}
	if counts.NetworkBodies != 1 {
		t.Errorf("Expected NetworkBodies count = 1, got %d", counts.NetworkBodies)
	}
	if counts.Total() != 3 {
		t.Errorf("Expected total = 3, got %d", counts.Total())
	}

	// Verify buffers empty
	capture.mu.RLock()
	if len(capture.networkWaterfall.entries) != 0 {
		t.Errorf("Expected networkWaterfall to be empty, got %d entries", len(capture.networkWaterfall.entries))
	}
	if len(capture.buffers.networkBodies) != 0 {
		t.Errorf("Expected networkBodies to be empty, got %d entries", len(capture.buffers.networkBodies))
	}
	if capture.buffers.networkTotalAdded != 0 {
		t.Errorf("Expected networkTotalAdded = 0, got %d", capture.buffers.networkTotalAdded)
	}
	capture.mu.RUnlock()
}

// TestClearWebSocketBuffers verifies clearing websocket_events and websocket_status buffers.
func TestClearWebSocketBuffers(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add WS events
	capture.AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "conn1", Direction: "outgoing", Data: "test"},
		{ID: "conn1", Direction: "incoming", Data: "response"},
	})

	// Add WS connections (open event only — it does not enter the event buffer).
	capture.mu.Lock()
	capture.wsConnections.TrackEvent(types.WebSocketEvent{Event: "open", ID: "conn1", URL: "ws://localhost"})
	capture.mu.Unlock()

	// Clear
	counts := capture.ClearWebSocketBuffers()

	// Verify counts
	if counts.WebSocketEvents != 2 {
		t.Errorf("Expected WebSocketEvents count = 2, got %d", counts.WebSocketEvents)
	}
	if counts.WebSocketStatus != 1 {
		t.Errorf("Expected WebSocketStatus count = 1, got %d", counts.WebSocketStatus)
	}

	// Verify buffers empty
	capture.mu.RLock()
	if len(capture.buffers.wsEvents) != 0 {
		t.Errorf("Expected wsEvents to be empty, got %d entries", len(capture.buffers.wsEvents))
	}
	if capture.wsConnections.Count() != 0 {
		t.Errorf("Expected connections to be empty, got %d entries", capture.wsConnections.Count())
	}
	capture.mu.RUnlock()
}

// TestClearActionBuffer verifies clearing enhancedActions buffer.
func TestClearActionBuffer(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add actions
	capture.AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Timestamp: 1738238000000},
		{Type: "input", Timestamp: 1738238001000},
	})

	// Clear
	counts := capture.ClearActionBuffer()

	// Verify counts
	if counts.Actions != 2 {
		t.Errorf("Expected Actions count = 2, got %d", counts.Actions)
	}

	// Verify buffer empty
	capture.mu.RLock()
	if len(capture.buffers.enhancedActions) != 0 {
		t.Errorf("Expected enhancedActions to be empty, got %d entries", len(capture.buffers.enhancedActions))
	}
	capture.mu.RUnlock()
}

// TestClearAllCapture verifies clearing all capture buffers at once.
func TestClearAllCapture(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add data to all capture buffers
	capture.mu.Lock()
	capture.networkWaterfall.entries = []types.NetworkWaterfallEntry{{URL: "test"}}
	capture.mu.Unlock()

	capture.AddWebSocketEvents([]types.WebSocketEvent{{ID: "conn1", Data: "test"}})
	capture.AddEnhancedActions([]types.EnhancedAction{{Type: "click", Timestamp: 1738238000000}})

	// Regression: extension logs must be cleared by ClearAll too. They used to be
	// left behind ("All" was a lie), so any caller that forgot the separate
	// ClearExtensionLogs() leaked stale logs. ClearAll now clears them and returns
	// the count.
	capture.mu.Lock()
	capture.extensionLogs.logs = append(capture.extensionLogs.logs, types.ExtensionLog{Level: "debug", Message: "ext log", Timestamp: time.Now()})
	capture.mu.Unlock()

	// Clear all
	extensionLogsCleared := capture.ClearAll()
	if extensionLogsCleared != 1 {
		t.Errorf("Expected ClearAll to clear and report 1 extension log, got %d", extensionLogsCleared)
	}

	// Verify all buffers empty
	capture.mu.RLock()
	defer capture.mu.RUnlock()

	if len(capture.networkWaterfall.entries) != 0 {
		t.Error("Expected networkWaterfall to be empty")
	}
	if len(capture.buffers.wsEvents) != 0 {
		t.Error("Expected wsEvents to be empty")
	}
	if len(capture.buffers.enhancedActions) != 0 {
		t.Error("Expected enhancedActions to be empty")
	}
	if len(capture.extensionLogs.logs) != 0 {
		t.Errorf("Expected extensionLogs to be empty after ClearAll, got %d entries", len(capture.extensionLogs.logs))
	}
}

// TestClearEmptyBuffers verifies clearing empty buffers returns zero counts without error.
func TestClearEmptyBuffers(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Clear empty network buffers
	counts := capture.ClearNetworkBuffers()

	// Should return zero counts, not error
	if counts.NetworkWaterfall != 0 {
		t.Errorf("Expected NetworkWaterfall count = 0, got %d", counts.NetworkWaterfall)
	}
	if counts.NetworkBodies != 0 {
		t.Errorf("Expected NetworkBodies count = 0, got %d", counts.NetworkBodies)
	}
	if counts.Total() != 0 {
		t.Errorf("Expected total = 0, got %d", counts.Total())
	}
}
