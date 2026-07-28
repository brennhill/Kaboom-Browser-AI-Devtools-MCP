// Purpose: Tests for capture disconnect detection and reconnection.
// Docs: docs/features/feature/backend-log-streaming/index.md

// disconnect_detection_test.go — Tests for extension disconnect detection.
// Covers: IsExtensionConnected, GetExtensionStatus, auto-expiry of pending
// queries when extension disconnects, and pilot status enrichment.
package capture

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// IsExtensionConnected
// ============================================

func TestIsExtensionConnected_FalseWhenNeverSynced(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	if c.Extension().IsExtensionConnected() {
		t.Fatal("IsExtensionConnected() = true, want false when no sync has occurred")
	}
}

func TestIsExtensionConnected_TrueAfterRecentSync(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.lastSyncSeen = time.Now()
	})

	if !c.Extension().IsExtensionConnected() {
		t.Fatal("IsExtensionConnected() = false, want true after recent sync")
	}
}

func TestIsExtensionConnected_FalseAfterTimeout(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.lastSyncSeen = time.Now().Add(-15 * time.Second)
	})

	if c.Extension().IsExtensionConnected() {
		t.Fatal("IsExtensionConnected() = true, want false after 15s without sync")
	}
}

func TestIsExtensionConnected_TrueAtBoundary(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	// Just under the threshold
	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.lastSyncSeen = time.Now().Add(-9 * time.Second)
	})

	if !c.Extension().IsExtensionConnected() {
		t.Fatal("IsExtensionConnected() = false, want true at 9s (under 10s threshold)")
	}
}

// ============================================
// GetExtensionStatus
// ============================================

func TestGetExtensionStatus_ReturnsConnectionInfo(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	now := time.Now()
	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.lastSyncSeen = now
		state.lastSyncClientID = "client-abc"
	})

	status := c.Extension().GetExtensionStatus()

	connected, ok := status["connected"].(bool)
	if !ok || !connected {
		t.Fatalf("expected connected=true, got %v", status["connected"])
	}

	clientID, ok := status["client_id"].(string)
	if !ok || clientID != "client-abc" {
		t.Fatalf("expected client_id='client-abc', got %v", status["client_id"])
	}

	lastSeen, ok := status["last_seen"].(string)
	if !ok || lastSeen == "" {
		t.Fatalf("expected non-empty last_seen, got %v", status["last_seen"])
	}
}

func TestGetExtensionStatus_DisconnectedWhenStale(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.lastSyncSeen = time.Now().Add(-20 * time.Second)
		state.lastSyncClientID = "client-old"
	})

	status := c.Extension().GetExtensionStatus()

	connected, ok := status["connected"].(bool)
	if !ok || connected {
		t.Fatalf("expected connected=false, got %v", status["connected"])
	}
}

func TestGetExtensionStatus_NeverConnected(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	status := c.Extension().GetExtensionStatus()

	connected := status["connected"].(bool)
	if connected {
		t.Fatal("expected connected=false when never synced")
	}

	lastSeen := status["last_seen"].(string)
	if lastSeen != "" {
		t.Fatalf("expected empty last_seen when never synced, got %q", lastSeen)
	}
}

// ============================================
// HandleSync updates lastSyncSeen
// ============================================

func TestHandleSync_UpdatesLastSyncSeen(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	// Before sync: not connected
	if c.Extension().IsExtensionConnected() {
		t.Fatal("expected not connected before first sync")
	}

	// Send sync request
	req := SyncRequest{ExtSessionID: "test-session"}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	httpReq.Header.Set("X-Kaboom-Client", "client-123")
	w := httptest.NewRecorder()

	NewSyncHandler(c).HandleSync(w, httpReq)

	// After sync: connected
	if !c.Extension().IsExtensionConnected() {
		t.Fatal("expected connected after sync")
	}

	// Verify client ID was tracked
	status := c.Extension().GetExtensionStatus()
	if status["client_id"] != "client-123" {
		t.Fatalf("expected client_id='client-123', got %v", status["client_id"])
	}
}

// ============================================
// Pilot status includes extension_last_seen
// ============================================

func TestGetPilotStatus_IncludesExtensionLastSeen(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	now := time.Now()
	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.pilotEnabled = true
		state.lastPollAt = now
		state.lastSyncSeen = now
		state.lastSyncClientID = "test-client"
	})

	status, ok := c.Extension().GetPilotStatus().(map[string]any)
	if !ok {
		t.Fatalf("GetPilotStatus returned unexpected type: %T", c.Extension().GetPilotStatus())
	}

	if status["extension_connected"] != true {
		t.Fatalf("expected extension_connected=true, got %v", status["extension_connected"])
	}

	lastSeen, ok := status["extension_last_seen"].(string)
	if !ok || lastSeen == "" {
		t.Fatalf("expected non-empty extension_last_seen, got %v", status["extension_last_seen"])
	}
}

func TestGetPilotStatus_EmptyLastSeenWhenNeverSynced(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	status, ok := c.Extension().GetPilotStatus().(map[string]any)
	if !ok {
		t.Fatalf("GetPilotStatus returned unexpected type: %T", c.Extension().GetPilotStatus())
	}

	lastSeen, ok := status["extension_last_seen"].(string)
	if !ok || lastSeen != "" {
		t.Fatalf("expected empty extension_last_seen, got %v", status["extension_last_seen"])
	}
}

func TestGetPilotStatus_DefaultsToAssumedEnabledUntilAuthoritative(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	status, ok := c.Extension().GetPilotStatus().(map[string]any)
	if !ok {
		t.Fatalf("GetPilotStatus returned unexpected type: %T", c.Extension().GetPilotStatus())
	}

	if status["enabled"] != true {
		t.Fatalf("enabled = %v, want true while startup state is uncertain", status["enabled"])
	}
	if status["state"] != "assumed_enabled" {
		t.Fatalf("state = %v, want assumed_enabled", status["state"])
	}
	if status["authoritative"] != false {
		t.Fatalf("authoritative = %v, want false before first explicit setting", status["authoritative"])
	}
}

func TestGetPilotStatus_ExplicitDisableFromSyncIsAuthoritative(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	reqBody := `{
		"ext_session_id":"pilot-explicit-disable",
		"settings":{
			"pilot_enabled":false,
			"tracking_enabled":false,
			"tracked_tab_id":0,
			"tracked_tab_url":"",
			"tracked_tab_title":"",
			"capture_logs":true,
			"capture_network":true,
			"capture_websocket":true,
			"capture_actions":true
		}
	}`
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewBufferString(reqBody))
	w := httptest.NewRecorder()
	NewSyncHandler(c).HandleSync(w, httpReq)

	status, ok := c.Extension().GetPilotStatus().(map[string]any)
	if !ok {
		t.Fatalf("GetPilotStatus returned unexpected type: %T", c.Extension().GetPilotStatus())
	}

	if status["enabled"] != false {
		t.Fatalf("enabled = %v, want false for explicit pilot disable", status["enabled"])
	}
	if status["state"] != "explicitly_disabled" {
		t.Fatalf("state = %v, want explicitly_disabled", status["state"])
	}
	if status["authoritative"] != true {
		t.Fatalf("authoritative = %v, want true after sync settings", status["authoritative"])
	}
}

// ============================================
// Auto-expire pending queries on disconnect
// ============================================

func TestGetPendingQueries_ExpiresOnDisconnect(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	// Simulate extension was connected, then disconnected
	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.lastSyncSeen = time.Now().Add(-15 * time.Second)
	})

	// Create a pending query with a correlation ID
	c.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "query_dom",
		Params:        json.RawMessage(`{"selector":".test"}`),
		CorrelationID: "corr-disconnect-1",
	}, 30*time.Second, "")

	// Verify query was created
	pending := c.Queries().GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending query, got %d", len(pending))
	}

	// Now call the disconnect-aware method
	result := NewSyncHandler(c).GetPendingQueriesDisconnectAware()
	if len(result) != 0 {
		t.Fatalf("expected 0 queries after disconnect expiry, got %d", len(result))
	}

	// Verify the command was marked as expired with disconnect reason
	cmd, found := c.Queries().GetCommandResult("corr-disconnect-1")
	if !found {
		t.Fatal("expected command result to exist after disconnect expiry")
	}
	if cmd.Status != "expired" {
		t.Fatalf("expected status='expired', got %q", cmd.Status)
	}
	if cmd.Error != "extension_disconnected" {
		t.Fatalf("expected error='extension_disconnected', got %q", cmd.Error)
	}
}

func TestGetPendingQueries_DoesNotExpireWhenConnected(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	// Simulate extension recently connected
	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.lastSyncSeen = time.Now()
	})

	// Create pending query
	c.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "query_dom",
		Params:        json.RawMessage(`{"selector":".test"}`),
		CorrelationID: "corr-connected-1",
	}, 30*time.Second, "")

	// Should return the query normally
	result := NewSyncHandler(c).GetPendingQueriesDisconnectAware()
	if len(result) != 1 {
		t.Fatalf("expected 1 pending query when connected, got %d", len(result))
	}
}

func TestGetPendingQueries_DoesNotExpireWhenNeverSynced(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	// lastSyncSeen is zero (never synced) — don't expire, extension might
	// still connect for the first time
	c.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:   "query_dom",
		Params: json.RawMessage(`{"selector":".test"}`),
	}, 30*time.Second, "")

	result := NewSyncHandler(c).GetPendingQueriesDisconnectAware()
	if len(result) != 1 {
		t.Fatalf("expected 1 pending query when never synced, got %d", len(result))
	}
}

func TestHandleSync_ExpiresPendingOnDisconnect(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	defer c.Close()

	// Simulate a past sync (extension was connected, now stale)
	mutateExtensionStateForTest(c.Extension(), func(state *ExtensionState) {
		state.lastSyncSeen = time.Now().Add(-15 * time.Second)
		state.lastPollAt = time.Now().Add(-15 * time.Second)
	})

	// Create pending queries with correlation IDs
	c.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "execute_js",
		Params:        json.RawMessage(`{"script":"alert(1)"}`),
		CorrelationID: "corr-sync-expire-1",
	}, 30*time.Second, "")

	c.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "navigate",
		Params:        json.RawMessage(`{"url":"https://example.com"}`),
		CorrelationID: "corr-sync-expire-2",
	}, 30*time.Second, "")

	// The sync handler calls GetPendingQueries internally via GetPendingQueriesDisconnectAware.
	// After reconnection (new sync), queries should have been expired before this sync processes.
	req := SyncRequest{ExtSessionID: "reconnect-session"}
	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/sync", bytes.NewReader(body))
	w := httptest.NewRecorder()

	NewSyncHandler(c).HandleSync(w, httpReq)

	// After the sync, the pending queries should have been expired
	// and the new sync should return no commands (they were expired before delivery)
	var resp SyncResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Commands list should be empty since they were expired due to disconnect
	if len(resp.Commands) != 0 {
		t.Fatalf("expected 0 commands after disconnect expiry, got %d", len(resp.Commands))
	}

	// Verify both commands were expired
	cmd1, found := c.Queries().GetCommandResult("corr-sync-expire-1")
	if !found {
		t.Fatal("expected corr-sync-expire-1 to exist")
	}
	if cmd1.Status != "expired" || cmd1.Error != "extension_disconnected" {
		t.Fatalf("corr-sync-expire-1: status=%q error=%q, want expired/extension_disconnected", cmd1.Status, cmd1.Error)
	}

	cmd2, found := c.Queries().GetCommandResult("corr-sync-expire-2")
	if !found {
		t.Fatal("expected corr-sync-expire-2 to exist")
	}
	if cmd2.Status != "expired" || cmd2.Error != "extension_disconnected" {
		t.Fatalf("corr-sync-expire-2: status=%q error=%q, want expired/extension_disconnected", cmd2.Status, cmd2.Error)
	}
}
