// sync_command_lifecycle_test.go — Tests sync polling and command-result lifecycle.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// Adaptive Polling Interval Tests
// ============================================

func TestHandleSync_AdaptivePoll_FastWhenPendingCommands(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	// Create a pending query so there are commands waiting
	cap.Queries().CreatePendingQuery(queries.PendingQuery{
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

	cap.Queries().CreatePendingQuery(queries.PendingQuery{
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
	queryID, _ := cap.Queries().CreatePendingQuery(queries.PendingQuery{
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
	cap.Queries().RegisterCommand(corrID, "q-sync-error-001", queries.AsyncCommandTimeout)

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
	queryID, _ := cap.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
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
				queryID, _ = cap.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
					Type:          "dom_action",
					Params:        json.RawMessage(`{"action":"click","selector":"#publish"}`),
					CorrelationID: corrID,
				}, queries.AsyncCommandTimeout, "")
				if queryID == "" {
					t.Fatal("expected queryID to be created")
				}
			} else if tc.hasCorrelation {
				cap.Queries().RegisterCommand(corrID, "q-"+corrID, queries.AsyncCommandTimeout)
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
				cmd, found := cap.Queries().GetCommandResult(corrID)
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
				if _, found := cap.Queries().TakeQueryResult(queryID); !found {
					t.Fatal("expected query result to be stored for id-only command result")
				}
			}
		})
	}
}

func TestHandleSync_LastCommandAckPreventsRedelivery(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	queryID, _ := cap.Queries().CreatePendingQuery(queries.PendingQuery{
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
