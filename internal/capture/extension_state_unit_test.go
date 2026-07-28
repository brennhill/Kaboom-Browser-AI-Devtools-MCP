// Purpose: Unit tests for capture pipeline extension state logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"context"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// WaitForExtensionConnected tests (issue #302)
// ============================================

func TestWaitForExtensionConnected_AlreadyConnected(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Simulate extension already connected.
	c.SimulateExtensionConnectForTest()

	if !c.Extension().WaitForExtensionConnected(context.Background(), 5*time.Second) {
		t.Fatal("WaitForExtensionConnected returned false when extension already connected")
	}
}

func TestWaitForExtensionConnected_ConnectsDuringWait(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Connect at 50ms — well before the first 200ms poll tick, giving a comfortable
	// 150ms margin before the tick catches the connection.
	go func() {
		time.Sleep(50 * time.Millisecond)
		c.SimulateExtensionConnectForTest()
	}()

	if !c.Extension().WaitForExtensionConnected(context.Background(), time.Second) {
		t.Fatal("WaitForExtensionConnected returned false; expected true after late connection")
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

// TestWaitForExtensionConnected_ContextCancelled lives in readiness_gate_test.go
// with full timing bounds checks (P1-1).

func TestCaptureTestHelpersAndTTL(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	c.AddNetworkBodiesForTest([]types.NetworkBody{
		{URL: "https://example.test/a", Status: 200},
		{URL: "https://example.test/b", Status: 500},
	})
	c.AddWebSocketEventsForTest([]types.WebSocketEvent{
		{Event: "open", URL: "wss://example.test"},
	})
	c.AddEnhancedActionsForTest([]types.EnhancedAction{
		{Type: "click", URL: "https://example.test", Timestamp: 123},
	})
	if got := c.GetNetworkTotalAdded(); got != 2 {
		t.Fatalf("GetNetworkTotalAdded() = %d, want 2", got)
	}
	if got := c.GetWebSocketTotalAdded(); got != 1 {
		t.Fatalf("GetWebSocketTotalAdded() = %d, want 1", got)
	}
	if got := c.GetActionTotalAdded(); got != 1 {
		t.Fatalf("GetActionTotalAdded() = %d, want 1", got)
	}

	c.SetPilotEnabled(true)
	if !c.Extension().IsPilotEnabled() {
		t.Fatal("SetPilotEnabled(true) did not update state")
	}
	c.SetTrackingStatusForTest(77, "https://tracked.test")
	enabled, tabID, tabURL := c.Extension().GetTrackingStatus()
	if !enabled || tabID != 77 || tabURL != "https://tracked.test" {
		t.Fatalf("tracking state = (%v,%d,%q), want (true,77,https://tracked.test)", enabled, tabID, tabURL)
	}

	if q := c.Queries().GetLastPendingQuery(); q != nil {
		t.Fatalf("GetLastPendingQuery() = %+v, want nil before adding pending query", q)
	}
	c.Queries().CreatePendingQuery(queries.PendingQuery{Type: "query_dom", Params: []byte(`{"selector":".x"}`)})
	c.Queries().CreatePendingQuery(queries.PendingQuery{Type: "accessibility", Params: []byte(`{"scope":"page"}`)})
	last := c.Queries().GetLastPendingQuery()
	if last == nil || last.Type != "accessibility" {
		t.Fatalf("last pending query = %+v, want accessibility query", last)
	}

	if isExpiredByTTL(time.Now(), 0) {
		t.Fatal("isExpiredByTTL should return false when ttl=0")
	}
	if !isExpiredByTTL(time.Now().Add(-2*time.Second), time.Second) {
		t.Fatal("isExpiredByTTL should return true for expired entry")
	}
	if isExpiredByTTL(time.Now().Add(-200*time.Millisecond), time.Second) {
		t.Fatal("isExpiredByTTL should return false for recent entry")
	}
}
