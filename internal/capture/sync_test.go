// Purpose: Tests for capture synchronization protocol and data delivery.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestHandleSync_BasicRequest(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a sync request
	req := SyncRequest{
		ExtSessionID: "test_session_123",
		Settings: &SyncSettings{
			PilotEnabled:    true,
			TrackingEnabled: false,
			TrackedTabID:    0,
			TrackedTabURL:   "",
		},
	}

	w := runSyncRequest(t, cap, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	resp := decodeSyncResponse(t, w)

	// Verify response
	if !resp.Ack {
		t.Error("Expected Ack to be true")
	}
	if resp.NextPollMs != 1000 {
		t.Errorf("Expected NextPollMs to be 1000, got %d", resp.NextPollMs)
	}
	if resp.ServerTime == "" {
		t.Error("Expected ServerTime to be set")
	}

	// Verify state was updated
	cap.mu.RLock()
	if cap.extensionState.extSessionID != "test_session_123" {
		t.Errorf("Expected session to be 'test_session_123', got '%s'", cap.extensionState.extSessionID)
	}
	if !cap.extensionState.pilotEnabled {
		t.Error("Expected pilotEnabled to be true")
	}
	cap.mu.RUnlock()
}

func TestHandleSync_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Try GET instead of POST
	w := runSyncRawRequest(t, cap, "GET", nil)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleSync_InvalidJSON(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Send invalid JSON
	w := runSyncRawRequest(t, cap, "POST", []byte("not json"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleSync_WithExtensionLogs(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create request with extension logs
	req := SyncRequest{
		ExtSessionID: "test_session",
		ExtensionLogs: []types.ExtensionLog{
			{
				Level:    "info",
				Message:  "Test log message",
				Source:   "background",
				Category: "test",
			},
		},
	}

	w := runSyncRequest(t, cap, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify logs were stored
	logs := cap.ExtensionLogs().Entries()
	if len(logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(logs))
	}
	if logs[0].Message != "Test log message" {
		t.Errorf("Expected log message 'Test log message', got '%s'", logs[0].Message)
	}
}

func TestHandleSync_WithExtensionLogs_RedactsSensitiveData(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	const (
		bearer = "Bearer tokenValue1234567890abcdef"
		awsKey = "AKIA1234567890ABCDEF"
	)

	req := SyncRequest{
		ExtSessionID: "test_session",
		ExtensionLogs: []types.ExtensionLog{
			{
				Level:    "debug",
				Message:  "sync saw " + bearer,
				Source:   "background",
				Category: "AUTH",
				Data:     json.RawMessage(`{"aws":"` + awsKey + `","header":"` + bearer + `"}`),
			},
		},
	}

	w := runSyncRequest(t, cap, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	logs := cap.ExtensionLogs().Entries()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 log, got %d", len(logs))
	}

	entry := logs[0]
	if strings.Contains(entry.Message, bearer) {
		t.Fatalf("Message should be redacted, got %q", entry.Message)
	}
	if !strings.Contains(entry.Message, "[REDACTED:bearer-token]") {
		t.Fatalf("Expected bearer token marker in message, got %q", entry.Message)
	}

	dataText := string(entry.Data)
	if strings.Contains(dataText, bearer) || strings.Contains(dataText, awsKey) {
		t.Fatalf("Expected redacted data, got %s", dataText)
	}
}

func TestHandleSync_UpdatesLastPollAt(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Initially lastPollAt should be zero
	cap.mu.RLock()
	initialPollAt := cap.extensionState.lastPollAt
	cap.mu.RUnlock()

	if !initialPollAt.IsZero() {
		t.Error("Expected initial lastPollAt to be zero")
	}

	// Send sync request
	req := SyncRequest{ExtSessionID: "test"}
	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Verify lastPollAt was updated
	cap.mu.RLock()
	newPollAt := cap.extensionState.lastPollAt
	cap.mu.RUnlock()

	if newPollAt.IsZero() {
		t.Error("Expected lastPollAt to be set after sync")
	}
}

func TestHandleSync_StoresInProgressHeartbeat(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	progress := 42.5
	req := SyncRequest{
		ExtSessionID: "test-session",
		InProgress: []SyncInProgress{
			{
				ID:            "q-123",
				CorrelationID: "corr-123",
				Type:          "browser_action",
				Status:        "running",
				ProgressPct:   &progress,
				StartedAt:     time.Now().Add(-2 * time.Second).UTC().Format(time.RFC3339),
				UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	w := runSyncRequest(t, cap, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	pilot, ok := cap.GetPilotStatus().(map[string]any)
	if !ok {
		t.Fatal("expected pilot status to be a map")
	}
	if pilot["in_progress_count"] != 1 {
		t.Fatalf("in_progress_count = %v, want 1", pilot["in_progress_count"])
	}

	inProgress, ok := pilot["in_progress"].([]SyncInProgress)
	if !ok {
		t.Fatalf("in_progress type = %T, want []SyncInProgress", pilot["in_progress"])
	}
	if len(inProgress) != 1 {
		t.Fatalf("len(in_progress) = %d, want 1", len(inProgress))
	}
	if inProgress[0].CorrelationID != "corr-123" {
		t.Fatalf("in_progress[0].correlation_id = %q, want corr-123", inProgress[0].CorrelationID)
	}
}

func TestHandleSync_MissingInProgressHeartbeatFailsStartedCommand(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	corrID := "corr-missing-heartbeat"
	queryID, _ := cap.CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"navigate","url":"https://example.com"}`),
		CorrelationID: corrID,
	}, queries.AsyncCommandTimeout, "")
	if queryID == "" {
		t.Fatal("expected queryID")
	}

	// First sync dispatches the command to extension.
	firstReqBody := []byte(`{"ext_session_id":"session-1","in_progress":[]}`)
	firstResp := runSyncRawRequest(t, cap, "POST", firstReqBody)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("first sync status = %d, want 200", firstResp.Code)
	}

	// Second sync ACKs receipt but still reports no in_progress entries.
	secondReqBody := mustMarshalJSON(t, map[string]any{
		"ext_session_id":   "session-1",
		"last_command_ack": queryID,
		"in_progress":      []any{},
	})
	secondResp := runSyncRawRequest(t, cap, "POST", secondReqBody)
	if secondResp.Code != http.StatusOK {
		t.Fatalf("second sync status = %d, want 200", secondResp.Code)
	}

	cmd, found := cap.GetCommandResult(corrID)
	if !found {
		t.Fatal("expected command result after second sync")
	}
	if cmd.Status != "pending" {
		t.Fatalf("command status after first miss = %q, want pending", cmd.Status)
	}

	// Third sync still has no in_progress entry -> command should fail fast.
	thirdReqBody := mustMarshalJSON(t, map[string]any{
		"ext_session_id": "session-1",
		"in_progress":    []any{},
	})
	thirdResp := runSyncRawRequest(t, cap, "POST", thirdReqBody)
	if thirdResp.Code != http.StatusOK {
		t.Fatalf("third sync status = %d, want 200", thirdResp.Code)
	}

	cmd, found = cap.GetCommandResult(corrID)
	if !found {
		t.Fatal("expected command result after desync reconciliation")
	}
	if cmd.Status != "error" {
		t.Fatalf("command status after second miss = %q, want error", cmd.Status)
	}
	if !strings.Contains(cmd.Error, "extension_lost_command") {
		t.Fatalf("command error = %q, want extension_lost_command", cmd.Error)
	}
}

func TestUpdateSyncConnectionState_NoReconnectForShortPollGap(t *testing.T) {
	t.Parallel()
	cap := NewCapture()
	defer cap.Close()

	now := time.Now()
	cap.mu.Lock()
	cap.extensionState.lastPollAt = now.Add(-6 * time.Second)
	cap.extensionState.lastSyncSeen = now.Add(-6 * time.Second)
	cap.extensionState.lastExtensionConnected = true
	cap.mu.Unlock()

	state := cap.updateSyncConnectionState(
		SyncRequest{ExtSessionID: "session-short-gap"},
		"client-short-gap",
		now,
	)

	if state.isReconnect {
		t.Fatal("expected isReconnect=false for 6s gap (< disconnect threshold)")
	}
	if state.wasDisconnected {
		t.Fatal("expected wasDisconnected=false for 6s gap (< disconnect threshold)")
	}
}

func TestUpdateSyncConnectionState_ReconnectAfterDisconnectThreshold(t *testing.T) {
	t.Parallel()
	cap := NewCapture()
	defer cap.Close()

	now := time.Now()
	cap.mu.Lock()
	cap.extensionState.lastPollAt = now.Add(-12 * time.Second)
	cap.extensionState.lastSyncSeen = now.Add(-12 * time.Second)
	cap.extensionState.lastExtensionConnected = true
	cap.mu.Unlock()

	state := cap.updateSyncConnectionState(
		SyncRequest{ExtSessionID: "session-long-gap"},
		"client-long-gap",
		now,
	)

	if !state.wasDisconnected {
		t.Fatal("expected wasDisconnected=true after 12s gap")
	}
	if !state.isReconnect {
		t.Fatal("expected isReconnect=true after disconnect threshold is crossed")
	}
}

// ============================================
// Adaptive Polling Interval Tests
// ============================================

func TestHandleSync_AdaptivePoll_FastWhenPendingCommands(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a pending query so there are commands waiting
	cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})

	// Sync should return fast poll interval (200ms) since commands are pending
	req := SyncRequest{ExtSessionID: "test_session"}
	w := runSyncRequest(t, cap, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	resp := decodeSyncResponse(t, w)

	if len(resp.Commands) == 0 {
		t.Fatal("Expected at least one command in response")
	}
	if resp.NextPollMs != 200 {
		t.Errorf("Expected NextPollMs to be 200 when commands pending, got %d", resp.NextPollMs)
	}
}

func TestHandleSync_CommandsIncludeTabID(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	cap.CreatePendingQuery(queries.PendingQuery{
		Type:          "dom_action",
		Params:        json.RawMessage(`{"action":"click","selector":"#submit"}`),
		TabID:         42,
		CorrelationID: "corr-tab-42",
	})

	req := SyncRequest{ExtSessionID: "test_session"}
	w := runSyncRequest(t, cap, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	resp := decodeSyncResponse(t, w)

	if len(resp.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(resp.Commands))
	}
	if resp.Commands[0].TabID != 42 {
		t.Fatalf("Expected command tab_id 42, got %d", resp.Commands[0].TabID)
	}
	if resp.Commands[0].CorrelationID != "corr-tab-42" {
		t.Fatalf("Expected correlation_id corr-tab-42, got %q", resp.Commands[0].CorrelationID)
	}
	if resp.Commands[0].TraceID != "corr-tab-42" {
		t.Fatalf("Expected trace_id corr-tab-42, got %q", resp.Commands[0].TraceID)
	}
}

func TestHandleSync_AdaptivePoll_SlowWhenNoCommands(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// No pending queries — should get default 1000ms interval
	req := SyncRequest{ExtSessionID: "test_session"}
	w := runSyncRequest(t, cap, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	resp := decodeSyncResponse(t, w)

	if len(resp.Commands) != 0 {
		t.Errorf("Expected no commands, got %d", len(resp.Commands))
	}
	if resp.NextPollMs != 1000 {
		t.Errorf("Expected NextPollMs to be 1000 when idle, got %d", resp.NextPollMs)
	}
}

func TestHandleSync_AdaptivePoll_RevertsAfterResultDelivered(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a pending query
	queryID, _ := cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})

	// First sync: should be fast (200ms) — commands pending
	req1 := SyncRequest{ExtSessionID: "test_session"}
	w1 := runSyncRequest(t, cap, req1)
	resp1 := decodeSyncResponse(t, w1)
	if resp1.NextPollMs != 200 {
		t.Errorf("First sync: expected NextPollMs 200, got %d", resp1.NextPollMs)
	}

	// Extension delivers result via second sync
	resultBytes, _ := json.Marshal(map[string]string{"html": "<body>test</body>"})
	req2 := SyncRequest{
		ExtSessionID: "test_session",
		CommandResults: []SyncCommandResult{
			{ID: queryID, Status: "complete", Result: resultBytes},
		},
	}
	w2 := runSyncRequest(t, cap, req2)
	resp2 := decodeSyncResponse(t, w2)

	// After result delivered, no more pending commands — should revert to 1000ms
	if resp2.NextPollMs != 1000 {
		t.Errorf("Second sync (after result): expected NextPollMs 1000, got %d", resp2.NextPollMs)
	}
}

func TestHandleSync_CommandResultPropagatesErrorStatus(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	corrID := "sync-corr-error-001"
	cap.RegisterCommand(corrID, "q-sync-error-001", queries.AsyncCommandTimeout)

	req := SyncRequest{
		ExtSessionID: "test_session",
		CommandResults: []SyncCommandResult{
			{
				ID:            "q-sync-error-001",
				CorrelationID: corrID,
				Status:        "error",
				Error:         "sync path failure",
			},
		},
	}
	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	assertCommandResult(t, cap, corrID, "error", "sync path failure")
}

func TestHandleSync_CommandResultWithIDAndCorrelationPreservesErrorStatus(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	corrID := "sync-corr-with-id-error-001"
	queryID, _ := cap.CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "dom_action",
		Params:        json.RawMessage(`{"action":"click","selector":"#publish"}`),
		CorrelationID: corrID,
	}, queries.AsyncCommandTimeout, "")
	if queryID == "" {
		t.Fatal("expected queryID to be created")
	}

	req := SyncRequest{
		ExtSessionID: "test_session",
		CommandResults: []SyncCommandResult{
			{
				ID:            queryID,
				CorrelationID: corrID,
				Status:        "error",
				Result:        json.RawMessage(`{"success":false}`),
				Error:         "dom_action_failed",
			},
		},
	}
	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	assertCommandResult(t, cap, corrID, "error", "dom_action_failed")
}

func TestHandleSync_CommandResultLifecycleMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		hasID          bool
		hasCorrelation bool
		status         string
		err            string
		expectStatus   string
		expectError    string
	}{
		{
			name:           "id+correlation explicit error",
			hasID:          true,
			hasCorrelation: true,
			status:         "error",
			err:            "hard failure",
			expectStatus:   "error",
			expectError:    "hard failure",
		},
		{
			name:           "id+correlation complete with error coerces to error",
			hasID:          true,
			hasCorrelation: true,
			status:         "complete",
			err:            "masked failure",
			expectStatus:   "error",
			expectError:    "masked failure",
		},
		{
			name:           "id+correlation timeout remains timeout",
			hasID:          true,
			hasCorrelation: true,
			status:         "timeout",
			err:            "timed out",
			expectStatus:   "timeout",
			expectError:    "timed out",
		},
		{
			name:           "correlation only error",
			hasID:          false,
			hasCorrelation: true,
			status:         "error",
			err:            "corr-only failure",
			expectStatus:   "error",
			expectError:    "corr-only failure",
		},
		{
			name:           "id only stores query result",
			hasID:          true,
			hasCorrelation: false,
			status:         "complete",
			err:            "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cap := NewCapture()

			corrID := ""
			if tc.hasCorrelation {
				corrID = "sync-matrix-" + strings.ReplaceAll(tc.name, " ", "-")
			}

			queryID := ""
			if tc.hasID {
				queryID, _ = cap.CreatePendingQueryWithTimeout(queries.PendingQuery{
					Type:          "dom_action",
					Params:        json.RawMessage(`{"action":"click","selector":"#publish"}`),
					CorrelationID: corrID,
				}, queries.AsyncCommandTimeout, "")
				if queryID == "" {
					t.Fatal("expected queryID to be created")
				}
			} else if tc.hasCorrelation {
				cap.RegisterCommand(corrID, "q-"+corrID, queries.AsyncCommandTimeout)
			}

			result := SyncCommandResult{
				Status: tc.status,
				Result: json.RawMessage(`{"ok":false}`),
				Error:  tc.err,
			}
			if tc.hasID {
				result.ID = queryID
			}
			if tc.hasCorrelation {
				result.CorrelationID = corrID
			}

			req := SyncRequest{
				ExtSessionID:   "test_session",
				CommandResults: []SyncCommandResult{result},
			}
			w := runSyncRequest(t, cap, req)
			if w.Code != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", w.Code)
			}

			if tc.hasCorrelation {
				cmd, found := cap.GetCommandResult(corrID)
				if !found {
					t.Fatal("expected command result to be present for correlation_id")
				}
				if cmd.Status != tc.expectStatus {
					t.Errorf("command status = %q, want %q", cmd.Status, tc.expectStatus)
				}
				if cmd.Error != tc.expectError {
					t.Errorf("command error = %q, want %q", cmd.Error, tc.expectError)
				}
				return
			}

			if tc.hasID {
				if _, found := cap.TakeQueryResult(queryID); !found {
					t.Fatal("expected query result to be stored for id-only command result")
				}
			}
		})
	}
}

// ============================================
// Waterfall On-Demand Tests via Sync
// ============================================

// TestHandleSync_WaterfallQueryDelivery verifies that waterfall queries
// are delivered to extension via sync response commands.
func TestHandleSync_WaterfallQueryDelivery(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a waterfall query (simulating MCP requesting fresh data)
	queryID, _ := cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "waterfall",
		Params: json.RawMessage(`{}`),
	})
	if queryID == "" {
		t.Fatal("Failed to create waterfall query")
	}

	// Extension polls /sync and receives the command
	req := SyncRequest{ExtSessionID: "test_session"}
	w := runSyncRequest(t, cap, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Parse response and verify waterfall command is present
	resp := decodeSyncResponse(t, w)

	if len(resp.Commands) == 0 {
		t.Fatal("Expected at least one command in sync response")
	}

	found := false
	for _, cmd := range resp.Commands {
		if cmd.Type == "waterfall" && cmd.ID == queryID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Waterfall command not found in sync response. Commands: %v", resp.Commands)
	}
}

// TestHandleSync_WaterfallResultDelivery verifies that waterfall results
// are stored correctly when extension posts them via sync.
func TestHandleSync_WaterfallResultDelivery(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a waterfall query
	queryID, _ := cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "waterfall",
		Params: json.RawMessage(`{}`),
	})

	// Simulate extension returning waterfall data via sync
	waterfallResult := map[string]any{
		"entries": []map[string]any{
			{"url": "https://api.example.com/users", "duration": 150.5},
		},
		"page_url": "https://example.com",
	}
	resultBytes, _ := json.Marshal(waterfallResult)

	req := SyncRequest{
		ExtSessionID: "test_session",
		CommandResults: []SyncCommandResult{
			{
				ID:     queryID,
				Status: "complete",
				Result: resultBytes,
			},
		},
	}

	w := runSyncRequest(t, cap, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Verify result was stored
	result, found := cap.TakeQueryResult(queryID)
	if !found {
		t.Fatal("Expected query result to be stored")
	}

	// Verify result content
	var storedResult map[string]any
	if err := json.Unmarshal(result, &storedResult); err != nil {
		t.Fatalf("Failed to unmarshal stored result: %v", err)
	}

	entries, ok := storedResult["entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Errorf("Expected 1 entry in result, got: %v", storedResult)
	}
}

func TestHandleSync_LastCommandAckPreventsRedelivery(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	queryID, _ := cap.CreatePendingQuery(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})
	if queryID == "" {
		t.Fatal("expected query ID")
	}

	firstReqBody := mustMarshalJSON(t, SyncRequest{ExtSessionID: "ack-session"})
	firstResp := runSyncRawRequest(t, cap, "POST", firstReqBody)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("first sync status = %d, want 200", firstResp.Code)
	}

	first := decodeSyncResponse(t, firstResp)
	if len(first.Commands) == 0 || first.Commands[0].ID != queryID {
		t.Fatalf("first sync should return query %q, got %+v", queryID, first.Commands)
	}

	ackReqBody := mustMarshalJSON(t, SyncRequest{
		ExtSessionID:   "ack-session",
		LastCommandAck: queryID,
	})
	ackResp := runSyncRawRequest(t, cap, "POST", ackReqBody)
	if ackResp.Code != http.StatusOK {
		t.Fatalf("ack sync status = %d, want 200", ackResp.Code)
	}

	second := decodeSyncResponse(t, ackResp)
	if len(second.Commands) != 0 {
		t.Fatalf("acknowledged command %q should not be redelivered, got %+v", queryID, second.Commands)
	}
}
