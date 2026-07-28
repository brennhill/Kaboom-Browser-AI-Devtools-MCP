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
	capture.Telemetry().NetworkWaterfall().Add([]types.NetworkWaterfallEntry{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
	}, "")

	capture.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{URL: "https://example.com/1"},
	})

	// Verify data exists
	initialWaterfall := len(capture.Telemetry().NetworkWaterfall().Entries())
	capture.telemetry.mu.RLock()
	initialBodies := len(capture.telemetry.buffers.networkBodies)
	capture.telemetry.mu.RUnlock()

	if initialWaterfall != 2 {
		t.Fatalf("Expected 2 waterfall entries, got %d", initialWaterfall)
	}
	if initialBodies != 1 {
		t.Fatalf("Expected 1 body entry, got %d", initialBodies)
	}

	// Clear
	counts := capture.Telemetry().ClearNetworkBuffers()

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
	if entries := capture.Telemetry().NetworkWaterfall().Entries(); len(entries) != 0 {
		t.Errorf("Expected networkWaterfall to be empty, got %d entries", len(entries))
	}
	capture.telemetry.mu.RLock()
	if len(capture.telemetry.buffers.networkBodies) != 0 {
		t.Errorf("Expected networkBodies to be empty, got %d entries", len(capture.telemetry.buffers.networkBodies))
	}
	if capture.telemetry.buffers.networkTotalAdded != 0 {
		t.Errorf("Expected networkTotalAdded = 0, got %d", capture.telemetry.buffers.networkTotalAdded)
	}
	capture.telemetry.mu.RUnlock()
}

// TestClearWebSocketBuffers verifies clearing websocket_events and websocket_status buffers.
func TestClearWebSocketBuffers(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add WS events
	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{ID: "conn1", Direction: "outgoing", Data: "test"},
		{ID: "conn1", Direction: "incoming", Data: "response"},
	})

	// Add WS connections (open event only — it does not enter the event buffer).
	capture.telemetry.mu.Lock()
	capture.telemetry.wsConnections.TrackEvent(types.WebSocketEvent{Event: "open", ID: "conn1", URL: "ws://localhost"})
	capture.telemetry.mu.Unlock()

	// Clear
	counts := capture.Telemetry().ClearWebSocketBuffers()

	// Verify counts
	if counts.WebSocketEvents != 2 {
		t.Errorf("Expected WebSocketEvents count = 2, got %d", counts.WebSocketEvents)
	}
	if counts.WebSocketStatus != 1 {
		t.Errorf("Expected WebSocketStatus count = 1, got %d", counts.WebSocketStatus)
	}

	// Verify buffers empty
	capture.telemetry.mu.RLock()
	if len(capture.telemetry.buffers.wsEvents) != 0 {
		t.Errorf("Expected wsEvents to be empty, got %d entries", len(capture.telemetry.buffers.wsEvents))
	}
	if capture.telemetry.wsConnections.Count() != 0 {
		t.Errorf("Expected connections to be empty, got %d entries", capture.telemetry.wsConnections.Count())
	}
	capture.telemetry.mu.RUnlock()
}

// TestClearActionBuffer verifies clearing enhancedActions buffer.
func TestClearActionBuffer(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add actions
	capture.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Timestamp: 1738238000000},
		{Type: "input", Timestamp: 1738238001000},
	})

	// Clear
	counts := capture.Telemetry().ClearActionBuffer()

	// Verify counts
	if counts.Actions != 2 {
		t.Errorf("Expected Actions count = 2, got %d", counts.Actions)
	}

	// Verify buffer empty
	capture.telemetry.mu.RLock()
	if len(capture.telemetry.buffers.enhancedActions) != 0 {
		t.Errorf("Expected enhancedActions to be empty, got %d entries", len(capture.telemetry.buffers.enhancedActions))
	}
	capture.telemetry.mu.RUnlock()
}

// TestClearAllCapture verifies clearing all capture buffers at once.
func TestClearAllCapture(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add data to all capture buffers
	capture.Telemetry().NetworkWaterfall().Add([]types.NetworkWaterfallEntry{{URL: "test"}}, "")

	capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{ID: "conn1", Data: "test"}})
	capture.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "click", Timestamp: 1738238000000}})

	// Regression: extension logs must be cleared by ClearAll too. They used to be
	// left behind ("All" was a lie), so any caller that forgot the separate
	// ClearExtensionLogs() leaked stale logs. ClearAll now clears them and returns
	// the count.
	capture.ExtensionLogs().Add([]types.ExtensionLog{{Level: "debug", Message: "ext log", Timestamp: time.Now()}})

	// Clear all
	extensionLogsCleared := capture.ClearAll()
	if extensionLogsCleared != 1 {
		t.Errorf("Expected ClearAll to clear and report 1 extension log, got %d", extensionLogsCleared)
	}

	// Verify all buffers empty
	if len(capture.Telemetry().NetworkWaterfall().Entries()) != 0 {
		t.Error("Expected networkWaterfall to be empty")
	}
	capture.telemetry.mu.RLock()
	defer capture.telemetry.mu.RUnlock()
	if len(capture.telemetry.buffers.wsEvents) != 0 {
		t.Error("Expected wsEvents to be empty")
	}
	if len(capture.telemetry.buffers.enhancedActions) != 0 {
		t.Error("Expected enhancedActions to be empty")
	}
	if logs := capture.ExtensionLogs().Entries(); len(logs) != 0 {
		t.Errorf("Expected extensionLogs to be empty after ClearAll, got %d entries", len(logs))
	}
}

// TestClearEmptyBuffers verifies clearing empty buffers returns zero counts without error.
func TestClearEmptyBuffers(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Clear empty network buffers
	counts := capture.Telemetry().ClearNetworkBuffers()

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
