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

	if len(c.GetNetworkTimestamps()) != 0 || len(c.GetWebSocketTimestamps()) != 0 || len(c.GetActionTimestamps()) != 0 {
		t.Fatal("new capture should return empty timestamp slices")
	}

	c.AddNetworkBodies([]types.NetworkBody{
		{URL: "https://example.test/a", Status: 200, Duration: 80},
		{URL: "https://example.test/b", Status: 503, Duration: 120},
	})
	c.AddWebSocketEvents([]types.WebSocketEvent{
		{Event: "open", URL: "wss://example.test/ws", ID: "ws-1"},
	})
	c.AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", URL: "https://example.test", Timestamp: 123},
	})

	snap := c.GetSnapshot()
	if snap.NetworkTotalAdded != 2 || snap.WebSocketTotalAdded != 1 || snap.ActionTotalAdded != 1 {
		t.Fatalf("snapshot totals = %+v, want 2/1/1", snap)
	}
	if snap.NetworkCount != 2 || snap.WebSocketCount != 1 || snap.ActionCount != 1 {
		t.Fatalf("snapshot counts = %+v, want 2/1/1", snap)
	}
	if got := c.GetNetworkErrorTotalAdded(); got != 1 {
		t.Fatalf("GetNetworkErrorTotalAdded() = %d, want 1", got)
	}

	nb := c.GetNetworkBodies()
	nb[0].URL = "https://mutated.test"
	if fresh := c.GetNetworkBodies()[0].URL; fresh == "https://mutated.test" {
		t.Fatal("GetNetworkBodies should return a copied slice")
	}

	ws := c.GetAllWebSocketEvents()
	ws[0].URL = "wss://mutated.test"
	if fresh := c.GetAllWebSocketEvents()[0].URL; fresh == "wss://mutated.test" {
		t.Fatal("GetAllWebSocketEvents should return a copied slice")
	}

	actions := c.GetAllEnhancedActions()
	actions[0].Type = "mutated"
	if fresh := c.GetAllEnhancedActions()[0].Type; fresh == "mutated" {
		t.Fatal("GetAllEnhancedActions should return a copied slice")
	}

	c.SetTestBoundaryStart("health-test")
	health := c.GetHealthSnapshot()
	if health.NetworkBodyCount != 2 || health.WebSocketCount != 1 || health.ActionCount != 1 {
		t.Fatalf("health counts = %+v, want 2/1/1", health)
	}
	if health.ActiveTestIDCount != 1 {
		t.Fatalf("health ActiveTestIDCount = %d, want 1", health.ActiveTestIDCount)
	}
}

func TestCapturePerformanceSnapshotAccessors(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	for i := 0; i < 105; i++ {
		c.AddPerformanceSnapshots([]performance.PerformanceSnapshot{
			{
				URL: fmt.Sprintf("https://example.test/%d", i),
			},
		})
	}

	all := c.GetPerformanceSnapshots()
	if len(all) != 100 {
		t.Fatalf("GetPerformanceSnapshots len = %d, want 100 (LRU cap)", len(all))
	}

	if _, ok := c.GetPerformanceSnapshotByURL("https://example.test/0"); ok {
		t.Fatal("expected oldest snapshot to be evicted")
	}
	if latest, ok := c.GetPerformanceSnapshotByURL("https://example.test/104"); !ok || latest.URL == "" {
		t.Fatalf("latest snapshot lookup = (%+v,%v), want found", latest, ok)
	}
	if _, ok := c.GetPerformanceSnapshotByURL("https://example.test/missing"); ok {
		t.Fatal("missing snapshot lookup should return ok=false")
	}
}

func TestCaptureBeforeSnapshotStoreAndConsume(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	c.StoreBeforeSnapshot("corr-1", performance.PerformanceSnapshot{URL: "https://example.test/before"})
	if snap, ok := c.GetAndDeleteBeforeSnapshot("corr-1"); !ok || snap.URL != "https://example.test/before" {
		t.Fatalf("GetAndDeleteBeforeSnapshot(corr-1) = (%+v,%v), want found snapshot", snap, ok)
	}
	if _, ok := c.GetAndDeleteBeforeSnapshot("corr-1"); ok {
		t.Fatal("before snapshot should be consume-on-read")
	}

	for i := 0; i < 60; i++ {
		c.StoreBeforeSnapshot(fmt.Sprintf("corr-%d", i), performance.PerformanceSnapshot{URL: fmt.Sprintf("u-%d", i)})
	}

	c.mu.RLock()
	beforeCount := len(c.perf.beforeSnapshots)
	c.mu.RUnlock()
	if beforeCount > 50 {
		t.Fatalf("beforeSnapshots size = %d, want <= 50", beforeCount)
	}
}

func TestCaptureClientRegistryAccessor(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	reg := c.GetClientRegistry()
	if reg != nil {
		t.Fatalf("GetClientRegistry() = %#v, want nil before registry is injected", reg)
	}
}

func TestCaptureSnapshotTimestampsAreCopied(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	c.AddNetworkBodies([]types.NetworkBody{{URL: "https://example.test", Status: 200}})
	c.AddWebSocketEvents([]types.WebSocketEvent{{Event: "open", ID: "1", URL: "wss://example.test"}})
	c.AddEnhancedActions([]types.EnhancedAction{{Type: "click", Timestamp: time.Now().UnixMilli()}})

	netTS := c.GetNetworkTimestamps()
	wsTS := c.GetWebSocketTimestamps()
	actTS := c.GetActionTimestamps()
	if len(netTS) != 1 || len(wsTS) != 1 || len(actTS) != 1 {
		t.Fatalf("timestamp lengths = %d/%d/%d, want 1/1/1", len(netTS), len(wsTS), len(actTS))
	}

	// Mutate local slices and verify capture state is unaffected.
	netTS[0] = time.Time{}
	wsTS[0] = time.Time{}
	actTS[0] = time.Time{}
	if c.GetNetworkTimestamps()[0].IsZero() || c.GetWebSocketTimestamps()[0].IsZero() || c.GetActionTimestamps()[0].IsZero() {
		t.Fatal("timestamp accessors should return copies")
	}
}
