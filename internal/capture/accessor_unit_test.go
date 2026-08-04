// Purpose: Unit tests for capture pipeline accessor logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

func TestCaptureAccessorSnapshotsAndCopies(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	if len(c.telemetry.buffers.networkTimestamps()) != 0 ||
		len(c.telemetry.buffers.webSocketTimestamps()) != 0 ||
		len(c.telemetry.buffers.actionTimestamps()) != 0 {
		t.Fatal("new capture should return empty timestamp slices")
	}

	c.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{URL: "https://example.test/a", Status: 200, Duration: 80},
		{URL: "https://example.test/b", Status: 503, Duration: 120},
	})
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Event: "open", URL: "wss://example.test/ws", ID: "ws-1"},
	})
	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", URL: "https://example.test", Timestamp: 123},
	})

	snap := c.Telemetry().GetSnapshot()
	if snap.NetworkTotalAdded != 2 || snap.WebSocketTotalAdded != 1 || snap.ActionTotalAdded != 1 {
		t.Fatalf("snapshot totals = %+v, want 2/1/1", snap)
	}
	if snap.NetworkCount != 2 || snap.WebSocketCount != 1 || snap.ActionCount != 1 {
		t.Fatalf("snapshot counts = %+v, want 2/1/1", snap)
	}
	if got := c.Telemetry().GetNetworkErrorTotalAdded(); got != 1 {
		t.Fatalf("GetNetworkErrorTotalAdded() = %d, want 1", got)
	}

	nb := c.Telemetry().GetNetworkBodies()
	nb[0].URL = "https://mutated.test"
	if fresh := c.Telemetry().GetNetworkBodies()[0].URL; fresh == "https://mutated.test" {
		t.Fatal("GetNetworkBodies should return a copied slice")
	}

	ws := c.Telemetry().GetAllWebSocketEvents()
	ws[0].URL = "wss://mutated.test"
	if fresh := c.Telemetry().GetAllWebSocketEvents()[0].URL; fresh == "wss://mutated.test" {
		t.Fatal("GetAllWebSocketEvents should return a copied slice")
	}

	actions := c.Telemetry().GetAllEnhancedActions()
	actions[0].Type = "mutated"
	if fresh := c.Telemetry().GetAllEnhancedActions()[0].Type; fresh == "mutated" {
		t.Fatal("GetAllEnhancedActions should return a copied slice")
	}

	c.Extension().SetTestBoundaryStart("health-test")
	health := NewHealthReader(c).Snapshot()
	if health.NetworkBodyCount != 2 || health.WebSocketCount != 1 || health.ActionCount != 1 {
		t.Fatalf("health counts = %+v, want 2/1/1", health)
	}
	if health.ActiveTestIDCount != 1 {
		t.Fatalf("health ActiveTestIDCount = %d, want 1", health.ActiveTestIDCount)
	}
}

func TestCaptureNestedSnapshotsDetachAtIngestAndRead(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	c.Extension().SetTestBoundaryStart("test-original")
	sampled := &types.SamplingInfo{Rate: "1:1"}
	bodies := []types.NetworkBody{{URL: "https://example.test", ResponseHeaders: map[string]string{"x-test": "original"}, TestIDs: []string{"test-original"}}}
	events := []types.WebSocketEvent{{ID: "ws", Sampled: sampled, TestIDs: []string{"test-original"}}}
	actions := []types.EnhancedAction{{Type: "click", Selectors: map[string]any{"css": "#original"}, TestIDs: []string{"test-original"}}}

	c.Telemetry().AddNetworkBodies(bodies)
	c.Telemetry().AddWebSocketEvents(events)
	c.Telemetry().AddEnhancedActions(actions)
	bodies[0].ResponseHeaders["x-test"] = "input-mutated"
	bodies[0].TestIDs[0] = "input-mutated"
	events[0].Sampled.Rate = "input-mutated"
	events[0].TestIDs[0] = "input-mutated"
	actions[0].Selectors["css"] = "input-mutated"
	actions[0].TestIDs[0] = "input-mutated"

	storedBodies := c.Telemetry().GetNetworkBodies()
	storedEvents := c.Telemetry().GetAllWebSocketEvents()
	storedActions := c.Telemetry().GetAllEnhancedActions()
	if storedBodies[0].ResponseHeaders["x-test"] != "original" || storedBodies[0].TestIDs[0] != "test-original" {
		t.Fatalf("network body retained caller-owned nested state: %+v", storedBodies[0])
	}
	if storedEvents[0].Sampled.Rate != "1:1" || storedEvents[0].TestIDs[0] != "test-original" {
		t.Fatalf("WebSocket event retained caller-owned nested state: %+v", storedEvents[0])
	}
	if storedActions[0].Selectors["css"] != "#original" || storedActions[0].TestIDs[0] != "test-original" {
		t.Fatalf("enhanced action retained caller-owned nested state: %+v", storedActions[0])
	}

	storedBodies[0].ResponseHeaders["x-test"] = "output-mutated"
	storedEvents[0].Sampled.Rate = "output-mutated"
	storedActions[0].Selectors["css"] = "output-mutated"
	if c.Telemetry().GetNetworkBodies()[0].ResponseHeaders["x-test"] != "original" {
		t.Fatal("network body snapshot aliases retained headers")
	}
	if c.Telemetry().GetAllWebSocketEvents()[0].Sampled.Rate != "1:1" {
		t.Fatal("WebSocket snapshot aliases retained sampling metadata")
	}
	if c.Telemetry().GetAllEnhancedActions()[0].Selectors["css"] != "#original" {
		t.Fatal("enhanced action snapshot aliases retained selectors")
	}
}

func TestTelemetryPressureReportsSaturationAndRecovery(t *testing.T) {
	c := NewCapture()
	now := time.Now().Add(-2 * time.Second)
	c.telemetry.mu.Lock()
	c.telemetry.buffers.appendNetworkBodies(make([]types.NetworkBody, MaxNetworkBodies+3), now)
	c.telemetry.buffers.appendWebSocketEvents(make([]types.WebSocketEvent, MaxWSEvents+2), now, nil)
	c.telemetry.buffers.appendEnhancedActions(make([]types.EnhancedAction, MaxEnhancedActions+1), now)
	c.telemetry.mu.Unlock()

	pressure := c.Telemetry().Pressure()
	assertPressure := func(name string, got PressureStats, size int, dropped int64) {
		t.Helper()
		if got.Size != size || got.Capacity != size || got.Dropped != dropped || got.OldestAge < time.Second {
			t.Fatalf("%s pressure = %#v, want size/capacity=%d dropped=%d and positive age", name, got, size, dropped)
		}
	}
	assertPressure("network", pressure.Network, MaxNetworkBodies, 3)
	assertPressure("websocket", pressure.WebSocket, MaxWSEvents, 2)
	assertPressure("actions", pressure.Actions, MaxEnhancedActions, 1)
	c.Telemetry().NetworkWaterfall().addAt(make([]types.NetworkWaterfallEntry, DefaultNetworkWaterfallCapacity+4), "https://example.test", now)
	if got := c.Telemetry().Pressure().NetworkWaterfall; got.Size != DefaultNetworkWaterfallCapacity || got.Dropped != 4 {
		t.Fatalf("network waterfall pressure = %#v, want bounded with four drops", got)
	}

	c.Telemetry().ClearNetworkBuffers()
	c.Telemetry().ClearWebSocketBuffers()
	c.Telemetry().ClearActionBuffer()
	pressure = c.Telemetry().Pressure()
	if pressure.Network.Size != 0 || pressure.WebSocket.Size != 0 || pressure.Actions.Size != 0 {
		t.Fatalf("pressure did not recover after clear: %#v", pressure)
	}
	if pressure.Network.Dropped != 3 || pressure.WebSocket.Dropped != 2 || pressure.Actions.Dropped != 1 {
		t.Fatalf("clear erased cumulative drop evidence: %#v", pressure)
	}
}

func TestCapturePerformanceSnapshotAccessors(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	for i := 0; i < 105; i++ {
		c.Performance().Add([]performance.PerformanceSnapshot{
			{
				URL: fmt.Sprintf("https://example.test/%d", i),
			},
		})
	}

	all := c.Performance().Entries()
	if len(all) != 100 {
		t.Fatalf("GetPerformanceSnapshots len = %d, want 100 (LRU cap)", len(all))
	}

	if _, ok := c.Performance().ByURL("https://example.test/0"); ok {
		t.Fatal("expected oldest snapshot to be evicted")
	}
	if latest, ok := c.Performance().ByURL("https://example.test/104"); !ok || latest.URL == "" {
		t.Fatalf("latest snapshot lookup = (%+v,%v), want found", latest, ok)
	}
	if _, ok := c.Performance().ByURL("https://example.test/missing"); ok {
		t.Fatal("missing snapshot lookup should return ok=false")
	}
	pressure := c.Performance().Pressure()
	if pressure.Snapshots.Size != maxPerformanceSnapshots || pressure.Snapshots.Dropped != 5 {
		t.Fatalf("performance snapshot pressure = %#v, want size=%d dropped=5", pressure.Snapshots, maxPerformanceSnapshots)
	}
}

func TestCaptureBeforeSnapshotStoreAndConsume(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	c.Performance().StoreBefore("corr-1", performance.PerformanceSnapshot{URL: "https://example.test/before"})
	if snap, ok := c.Performance().TakeBefore("corr-1"); !ok || snap.URL != "https://example.test/before" {
		t.Fatalf("GetAndDeleteBeforeSnapshot(corr-1) = (%+v,%v), want found snapshot", snap, ok)
	}
	if _, ok := c.Performance().TakeBefore("corr-1"); ok {
		t.Fatal("before snapshot should be consume-on-read")
	}

	for i := 0; i < 60; i++ {
		c.Performance().StoreBefore(fmt.Sprintf("corr-%d", i), performance.PerformanceSnapshot{URL: fmt.Sprintf("u-%d", i)})
	}

	c.perf.mu.RLock()
	beforeCount := len(c.perf.beforeSnapshots)
	c.perf.mu.RUnlock()
	if beforeCount > 50 {
		t.Fatalf("beforeSnapshots size = %d, want <= 50", beforeCount)
	}
	if got := c.Performance().Pressure().BeforeSnapshots; got.Size != maxBeforeSnapshots || got.Dropped != 10 {
		t.Fatalf("before snapshot pressure = %#v, want size=%d dropped=10", got, maxBeforeSnapshots)
	}
}

func TestCaptureClientRegistryAccessor(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	reg := c.Clients().Registry()
	if reg != nil {
		t.Fatalf("Clients().Registry() = %#v, want nil before registry is injected", reg)
	}
}

func TestCaptureSnapshotTimestampsAreCopied(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	c.Telemetry().AddNetworkBodies([]types.NetworkBody{{URL: "https://example.test", Status: 200}})
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{Event: "open", ID: "1", URL: "wss://example.test"}})
	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{{Type: "click", Timestamp: time.Now().UnixMilli()}})

	netTS := c.telemetry.buffers.networkTimestamps()
	wsTS := c.telemetry.buffers.webSocketTimestamps()
	actTS := c.telemetry.buffers.actionTimestamps()
	if len(netTS) != 1 || len(wsTS) != 1 || len(actTS) != 1 {
		t.Fatalf("timestamp lengths = %d/%d/%d, want 1/1/1", len(netTS), len(wsTS), len(actTS))
	}

	// Mutate local slices and verify capture state is unaffected.
	netTS[0] = time.Time{}
	wsTS[0] = time.Time{}
	actTS[0] = time.Time{}
	if c.telemetry.buffers.networkTimestamps()[0].IsZero() ||
		c.telemetry.buffers.webSocketTimestamps()[0].IsZero() ||
		c.telemetry.buffers.actionTimestamps()[0].IsZero() {
		t.Fatal("timestamp accessors should return copies")
	}
}
