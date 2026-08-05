// Purpose: Unit tests for capture pipeline extension state logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"context"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// WaitForExtensionConnected tests (issue #302)
// ============================================

func TestWaitForExtensionConnected_AlreadyConnected(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Simulate extension already connected.
	connectForTest(c)

	if !c.Extension().WaitForExtensionConnected(context.Background(), 5*time.Second) {
		t.Fatal("WaitForExtensionConnected returned false when extension already connected")
	}
}

func TestWaitForExtensionConnected_Timeout(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	// Extension never connects.

	if c.Extension().WaitForExtensionConnected(context.Background(), 100*time.Millisecond) {
		t.Fatal("WaitForExtensionConnected returned true; expected false after timeout")
	}
}

func TestMarkDisconnectedPreservesAuthoritativeSettings(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	c.extension.updateSyncConnectionState(SyncRequest{Settings: &SyncSettings{
		PilotEnabled:    true,
		TrackingEnabled: true,
		TrackedTabID:    42,
		TrackedTabURL:   "https://example.test",
	}}, "client", time.Now())
	lastSeen := c.Extension().GetExtensionStatus()["last_seen"]

	c.Extension().MarkDisconnected()

	if c.Extension().IsExtensionConnected() {
		t.Fatal("extension remained connected")
	}
	status := c.Extension().GetExtensionStatus()
	if status["last_seen"] != lastSeen || status["last_seen"] == "" {
		t.Fatalf("disconnect discarded last-seen evidence: before=%v after=%v", lastSeen, status["last_seen"])
	}
	if !c.Extension().IsPilotEnabled() {
		t.Fatal("disconnect discarded authoritative pilot setting")
	}
	tracked, tabID, _ := c.Extension().GetTrackingStatus()
	if !tracked || tabID != 42 {
		t.Fatalf("disconnect discarded tracking state: (%v, %d)", tracked, tabID)
	}
}

// TestWaitForExtensionConnected_ContextCancelled lives in readiness_gate_test.go
// with full timing bounds checks (P1-1).

func TestCaptureTestHelpersAndTTL(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	c.Telemetry().AddNetworkBodies([]types.NetworkBody{
		{URL: "https://example.test/a", Status: 200},
		{URL: "https://example.test/b", Status: 500},
	})
	c.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Event: "open", URL: "wss://example.test"},
	})
	c.Telemetry().AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", URL: "https://example.test", Timestamp: 123},
	})
	if got := c.Telemetry().NetworkBodies().Stats().TotalAdded; got != 2 {
		t.Fatalf("GetNetworkTotalAdded() = %d, want 2", got)
	}
	if got := c.Telemetry().WebSockets().Stats().TotalAdded; got != 1 {
		t.Fatalf("GetWebSocketTotalAdded() = %d, want 1", got)
	}
	if got := c.Telemetry().Actions().Stats().TotalAdded; got != 1 {
		t.Fatalf("GetActionTotalAdded() = %d, want 1", got)
	}

	setPilotForTest(c, true)
	if !c.Extension().IsPilotEnabled() {
		t.Fatal("SetPilotEnabled(true) did not update state")
	}
	trackForTest(c, 77, "https://tracked.test")
	enabled, tabID, tabURL := c.Extension().GetTrackingStatus()
	if !enabled || tabID != 77 || tabURL != "https://tracked.test" {
		t.Fatalf("tracking state = (%v,%d,%q), want (true,77,https://tracked.test)", enabled, tabID, tabURL)
	}

	if pending := c.Queries().GetPendingQueries(); len(pending) != 0 {
		t.Fatalf("GetPendingQueries() = %+v, want none before adding pending query", pending)
	}
	c.Queries().CreatePendingQuery(queries.PendingQuery{Type: "query_dom", Params: []byte(`{"selector":".x"}`)})
	c.Queries().CreatePendingQuery(queries.PendingQuery{Type: "accessibility", Params: []byte(`{"scope":"page"}`)})
	pending := c.Queries().GetPendingQueries()
	if len(pending) != 2 || pending[len(pending)-1].Type != "accessibility" {
		t.Fatalf("pending queries = %+v, want accessibility query last", pending)
	}
}
