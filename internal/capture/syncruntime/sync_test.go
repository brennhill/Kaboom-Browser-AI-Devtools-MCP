// sync_test.go — Tests sync request ingestion and connection state.
// Docs: docs/features/feature/backend-log-streaming/index.md

package syncruntime

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestExtractBrowserNameUsesPrivacySafeFamilies(t *testing.T) {
	t.Parallel()
	for userAgent, want := range map[string]string{
		"Mozilla Chrome Brave": "brave", "Mozilla Chrome Edg/120": "edge",
		"Mozilla Chrome/120": "chrome", "Mozilla Firefox/120": "firefox",
		"Mozilla Safari/17": "safari", "custom client": "unknown",
	} {
		if got := extractBrowserName(userAgent); got != want {
			t.Fatalf("extractBrowserName(%q) = %q, want %q", userAgent, got, want)
		}
	}
}

func TestMajorMinor(t *testing.T) {
	t.Parallel()
	for input, want := range map[string]string{"6.0.3": "6.0", "1.2.3": "1.2", "10.20.30": "10.20", "6.0": "6.0", "6": "", "": ""} {
		if got := majorMinor(input); got != want {
			t.Errorf("majorMinor(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHandleSync_BasicRequest(t *testing.T) {
	t.Parallel()
	cap := newTestState()

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
	state := extensionStateSnapshotForTest(cap.Extension())
	if state.extSessionID != "test_session_123" {
		t.Errorf("Expected session to be 'test_session_123', got '%s'", state.extSessionID)
	}
	if !state.pilotEnabled {
		t.Error("Expected pilotEnabled to be true")
	}
}

func TestHandleSync_StoresAndClearsCommandContractIdentity(t *testing.T) {
	t.Parallel()
	cap := newTestState()
	defer cap.Close()

	runSyncRequest(t, cap, SyncRequest{
		ExtSessionID:      "contract-session",
		CommandContractID: "sha256:loaded-contract",
	})
	if got := cap.Extension().CommandContractID(); got != "sha256:loaded-contract" {
		t.Fatalf("CommandContractID() = %q, want loaded contract", got)
	}

	// Missing identity means the currently loaded extension predates the
	// contract handshake; never retain a matching value from an older sync.
	runSyncRequest(t, cap, SyncRequest{ExtSessionID: "contract-session"})
	if got := cap.Extension().CommandContractID(); got != "" {
		t.Fatalf("CommandContractID() = %q after absent identity, want empty", got)
	}
}

func TestHandleSync_RejectsSupersededConnectionGeneration(t *testing.T) {
	t.Parallel()
	cap := newTestState()

	first := runSyncRequest(t, cap, SyncRequest{
		ExtSessionID: "session-old",
		Settings:     &SyncSettings{PilotEnabled: false},
	})
	if first.Code != http.StatusOK {
		t.Fatalf("first sync status = %d, want 200", first.Code)
	}
	firstGeneration := decodeSyncResponse(t, first).ConnectionGeneration
	if firstGeneration == 0 {
		t.Fatal("first sync did not assign a connection generation")
	}

	current := runSyncRequest(t, cap, SyncRequest{
		ExtSessionID: "session-current",
		Settings:     &SyncSettings{PilotEnabled: true},
	})
	if current.Code != http.StatusOK {
		t.Fatalf("current sync status = %d, want 200", current.Code)
	}
	currentGeneration := decodeSyncResponse(t, current).ConnectionGeneration
	if currentGeneration <= firstGeneration {
		t.Fatalf("current generation = %d, want > %d", currentGeneration, firstGeneration)
	}

	stale := runSyncRequest(t, cap, SyncRequest{
		ExtSessionID:         "session-old",
		ConnectionGeneration: firstGeneration,
		Settings:             &SyncSettings{PilotEnabled: false},
	})
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale sync status = %d, want 409", stale.Code)
	}

	state := extensionStateSnapshotForTest(cap.Extension())
	if state.extSessionID != "session-current" {
		t.Fatalf("extension session = %q, want current session", state.extSessionID)
	}
	if !state.pilotEnabled {
		t.Fatal("stale sync mutated current pilot state")
	}
}

func TestHandleSync_RejectsStaleCommandResultWithoutCompletingCurrentWork(t *testing.T) {
	t.Parallel()
	cap := newTestState()

	first := runSyncRequest(t, cap, SyncRequest{ExtSessionID: "session-old"})
	firstGeneration := decodeSyncResponse(t, first).ConnectionGeneration
	runSyncRequest(t, cap, SyncRequest{ExtSessionID: "session-current"})

	correlationID := "corr-current-generation"
	queryID, _ := cap.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"highlight"}`),
		CorrelationID: correlationID,
	}, queries.AsyncCommandTimeout, "")

	stale := runSyncRequest(t, cap, SyncRequest{
		ExtSessionID:         "session-old",
		ConnectionGeneration: firstGeneration,
		CommandResults: []SyncCommandResult{{
			ID:            queryID,
			CorrelationID: correlationID,
			Status:        "complete",
			Result:        json.RawMessage(`{"success":true}`),
		}},
	})
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale result status = %d, want 409", stale.Code)
	}
	result, found := cap.Queries().GetCommandResult(correlationID)
	if !found || result.Status != "pending" {
		t.Fatalf("current command result = %#v, found=%v; want pending", result, found)
	}
}

func TestHandleSync_RejectsStaleResultInsideCurrentHeartbeat(t *testing.T) {
	t.Parallel()
	cap := newTestState()

	first := runSyncRequest(t, cap, SyncRequest{ExtSessionID: "session-old"})
	firstGeneration := decodeSyncResponse(t, first).ConnectionGeneration
	current := runSyncRequest(t, cap, SyncRequest{ExtSessionID: "session-current"})
	currentGeneration := decodeSyncResponse(t, current).ConnectionGeneration

	correlationID := "corr-stale-result-current-heartbeat"
	queryID, _ := cap.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"highlight"}`),
		CorrelationID: correlationID,
	}, queries.AsyncCommandTimeout, "")

	response := runSyncRequest(t, cap, SyncRequest{
		ExtSessionID:         "session-current",
		ConnectionGeneration: currentGeneration,
		CommandResults: []SyncCommandResult{{
			ID:                   queryID,
			CorrelationID:        correlationID,
			ConnectionGeneration: firstGeneration,
			Status:               "complete",
			Result:               json.RawMessage(`{"success":true}`),
		}},
	})
	if response.Code != http.StatusOK {
		t.Fatalf("current heartbeat status = %d, want 200", response.Code)
	}
	result, found := cap.Queries().GetCommandResult(correlationID)
	if !found || result.Status != "pending" {
		t.Fatalf("current command result = %#v, found=%v; want pending", result, found)
	}
}

func TestHandleSync_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	cap := newTestState()

	// Try GET instead of POST
	w := runSyncRawRequest(t, cap, "GET", nil)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleSync_InvalidJSON(t *testing.T) {
	t.Parallel()
	cap := newTestState()

	// Send invalid JSON
	w := runSyncRawRequest(t, cap, "POST", []byte("not json"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleSync_WithExtensionLogs(t *testing.T) {
	t.Parallel()
	cap := newTestState()

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
	cap := newTestState()

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
	cap := newTestState()

	// Initially lastPollAt should be zero
	initialPollAt := extensionStateSnapshotForTest(cap.Extension()).lastPollAt

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
	newPollAt := extensionStateSnapshotForTest(cap.Extension()).lastPollAt

	if newPollAt.IsZero() {
		t.Error("Expected lastPollAt to be set after sync")
	}
}

func TestHandleSync_StoresInProgressHeartbeat(t *testing.T) {
	t.Parallel()
	cap := newTestState()

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

	pilot, ok := cap.Extension().GetPilotStatus().(map[string]any)
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
	cap := newTestState()

	corrID := "corr-missing-heartbeat"
	queryID, _ := cap.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
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

	cmd, found := cap.Queries().GetCommandResult(corrID)
	if !found {
		t.Fatal("expected command result after second sync")
	}
	if cmd.Status != "pending" {
		t.Fatalf("command status after first miss = %q, want pending", cmd.Status)
	}

	// Move the deterministic reconciliation clock beyond the bounded grace.
	mutateExtensionStateForTest(cap.Extension(), func(state *runtimeState) {
		state.missingInProgressSince[corrID] = time.Now().Add(-missingInProgressGrace)
	})

	// Third sync still has no in_progress entry after the grace -> fail fast.
	thirdReqBody := mustMarshalJSON(t, map[string]any{
		"ext_session_id": "session-1",
		"in_progress":    []any{},
	})
	thirdResp := runSyncRawRequest(t, cap, "POST", thirdReqBody)
	if thirdResp.Code != http.StatusOK {
		t.Fatalf("third sync status = %d, want 200", thirdResp.Code)
	}

	cmd, found = cap.Queries().GetCommandResult(corrID)
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

func TestHandleSync_ImmediateMissingHeartbeatsDoNotFailResultInFlight(t *testing.T) {
	t.Parallel()
	cap := newTestState()

	corrID := "corr-result-in-flight"
	queryID, _ := cap.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"highlight","selector":"#target"}`),
		CorrelationID: corrID,
	}, queries.AsyncCommandTimeout, "")
	if queryID == "" {
		t.Fatal("expected queryID")
	}

	runSyncRawRequest(t, cap, "POST", []byte(`{"ext_session_id":"session-1","in_progress":[]}`))
	for i := 0; i < 3; i++ {
		body := mustMarshalJSON(t, map[string]any{
			"ext_session_id":   "session-1",
			"last_command_ack": queryID,
			"in_progress":      []any{},
		})
		response := runSyncRawRequest(t, cap, "POST", body)
		if response.Code != http.StatusOK {
			t.Fatalf("heartbeat %d status = %d, want 200", i+1, response.Code)
		}
	}

	cmd, found := cap.Queries().GetCommandResult(corrID)
	if !found {
		t.Fatal("expected command result after immediate partial heartbeats")
	}
	if cmd.Status != "pending" {
		t.Fatalf("command status = %q, want pending while terminal result is in flight", cmd.Status)
	}

	terminalBody := mustMarshalJSON(t, map[string]any{
		"ext_session_id": "session-1",
		"command_results": []map[string]any{{
			"id":             queryID,
			"correlation_id": corrID,
			"status":         "complete",
			"result":         map[string]any{"success": true},
		}},
		"in_progress": []any{},
	})
	terminalResponse := runSyncRawRequest(t, cap, "POST", terminalBody)
	if terminalResponse.Code != http.StatusOK {
		t.Fatalf("terminal result status = %d, want 200", terminalResponse.Code)
	}

	cmd, found = cap.Queries().GetCommandResult(corrID)
	if !found || cmd.Status != "complete" {
		t.Fatalf("terminal command = %#v, found=%v; want complete", cmd, found)
	}
}

func TestUpdateSyncConnectionState_NoReconnectForShortPollGap(t *testing.T) {
	t.Parallel()
	cap := newTestState()
	defer cap.Close()

	now := time.Now()
	mutateExtensionStateForTest(cap.Extension(), func(state *runtimeState) {
		state.lastPollAt = now.Add(-6 * time.Second)
		state.lastSyncSeen = now.Add(-6 * time.Second)
		state.lastExtensionConnected = true
		state.lastExtensionConnected = true
	})

	state := cap.runtime.updateSyncConnectionState(
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
	cap := newTestState()
	defer cap.Close()

	now := time.Now()
	mutateExtensionStateForTest(cap.Extension(), func(state *runtimeState) {
		state.lastPollAt = now.Add(-12 * time.Second)
		state.lastSyncSeen = now.Add(-12 * time.Second)
		state.lastExtensionConnected = true
		state.lastExtensionConnected = true
	})

	state := cap.runtime.updateSyncConnectionState(
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

// A harness that validates the /sync contract is not a second browser extension.
// Adopting it as the authoritative session bumps the connection generation, which
// supersedes the real extension's in-flight poll, and its partial settings erase the
// tracked tab the real extension reported — the connected UAT's intermittent
// "no tracked browser tab" failures.
func TestHandleSync_ProbeClientNeverBecomesTheAuthoritativeSession(t *testing.T) {
	t.Parallel()
	cap := newTestState()
	defer cap.Close()

	runSyncRequestAsClient(t, cap, "kaboom-extension/0.9.0", SyncRequest{
		ExtSessionID: "real-extension",
		Settings: &SyncSettings{
			PilotEnabled: true, TrackingEnabled: true, TrackedTabID: 41,
			TrackedTabURL: "https://app.example.com/fixture",
		},
	})
	before := extensionStateSnapshotForTest(cap.Extension())
	if !before.trackingEnabled || before.trackedTabID != 41 {
		t.Fatalf("expected the real extension to own tracking, got %#v", before)
	}

	cap.Queries().CreatePendingQuery(queries.PendingQuery{Type: "query_dom", Params: []byte(`{"selector":".x"}`)})

	w := runSyncRequestAsClient(t, cap, "kaboom-probe/0.9.0", SyncRequest{
		ExtSessionID: "pilot-success-js",
		Settings:     &SyncSettings{PilotEnabled: true},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("probe sync status = %d, want 200 so contract tests can validate the endpoint", w.Code)
	}
	resp := decodeSyncResponse(t, w)
	if !resp.Ack {
		t.Error("probe sync must still acknowledge a well-formed payload")
	}
	if len(resp.Commands) != 0 {
		t.Fatalf("probe received %d commands; only the extension may drain the queue", len(resp.Commands))
	}

	after := extensionStateSnapshotForTest(cap.Extension())
	if after.extSessionID != "real-extension" {
		t.Errorf("extSessionID = %q, want the real extension to keep the session", after.extSessionID)
	}
	if after.connectionGeneration != before.connectionGeneration {
		t.Errorf("connectionGeneration = %d, want %d unchanged", after.connectionGeneration, before.connectionGeneration)
	}
	if !after.trackingEnabled || after.trackedTabID != 41 {
		t.Errorf("probe erased tracking: trackingEnabled=%v trackedTabID=%d", after.trackingEnabled, after.trackedTabID)
	}
	if pending := cap.Queries().GetPendingQueries(); len(pending) != 1 {
		t.Errorf("pending queries = %d, want the command still queued for the extension", len(pending))
	}
}

func TestHandleSync_ProbeClientDoesNotMakeTheExtensionLookConnected(t *testing.T) {
	t.Parallel()
	cap := newTestState()
	defer cap.Close()

	runSyncRequestAsClient(t, cap, "kaboom-probe/0.9.0", SyncRequest{ExtSessionID: "probe-only"})

	if cap.Extension().IsExtensionConnected() {
		t.Fatal("a probe must never present itself as a connected extension")
	}
}
