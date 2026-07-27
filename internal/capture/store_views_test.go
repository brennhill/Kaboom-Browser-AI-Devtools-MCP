package capture

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

func TestCaptureStoreViews_ExposeFocusedSnapshotContracts(t *testing.T) {
	c := NewCapture()

	c.AddNetworkBodiesForTest([]NetworkBody{{URL: "https://api.example.com/data", Method: "GET", Status: 200}})
	c.AddWebSocketEventsForTest([]WebSocketEvent{{Event: "message", ID: "ws-1", URL: "wss://example.com/ws", Data: "ok"}})
	c.AddEnhancedActionsForTest([]EnhancedAction{{Type: "click", URL: "https://example.com"}})
	c.AddNetworkWaterfallEntries([]NetworkWaterfallEntry{{URL: "https://cdn.example.com/app.js"}}, "https://example.com")
	c.AddExtensionLogs([]ExtensionLog{{Level: "info", Message: "extension ok"}})
	c.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{URL: "https://example.com"}})

	events := c.EventBuffers()
	if len(events.NetworkBodies()) != 1 {
		t.Fatalf("network bodies count = %d, want 1", len(events.NetworkBodies()))
	}
	if len(events.WebSocketEvents()) != 1 {
		t.Fatalf("websocket events count = %d, want 1", len(events.WebSocketEvents()))
	}
	if len(events.EnhancedActions()) != 1 {
		t.Fatalf("enhanced actions count = %d, want 1", len(events.EnhancedActions()))
	}

	waterfall := c.NetworkWaterfallStore()
	if waterfall.Count() != 1 {
		t.Fatalf("waterfall count = %d, want 1", waterfall.Count())
	}
	if len(waterfall.Entries()) != 1 {
		t.Fatalf("waterfall entries len = %d, want 1", len(waterfall.Entries()))
	}

	extLogs := c.ExtensionLogStore()
	if len(extLogs.Entries()) != 1 {
		t.Fatalf("extension logs count = %d, want 1", len(extLogs.Entries()))
	}

	perf := c.PerformanceSnapshotStore()
	if len(perf.Snapshots()) != 1 {
		t.Fatalf("performance snapshots count = %d, want 1", len(perf.Snapshots()))
	}
	if _, ok := perf.SnapshotByURL("https://example.com"); !ok {
		t.Fatal("expected snapshot by URL")
	}
}
