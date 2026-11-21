// sync_command_lifecycle_test.go — Tests sync polling, command-result lifecycle and old-poll-after-reconnect interleaving.
// Docs: docs/features/feature/backend-log-streaming/index.md

package syncruntime

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// ============================================
// Adaptive Polling Interval Tests
// ============================================

func TestHandleSync_AdaptivePoll_FastWhenPendingCommands(t *testing.T) {
	t.Parallel()
	cap := newTestState()

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
	cap := newTestState()

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
	cap := newTestState()

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
	cap := newTestState()

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
	cap := newTestState()

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
	cap := newTestState()

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

			cap := newTestState()

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
	cap := newTestState()

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

// ============================================
// Superseded poll interleaving (kaboom-jdm5.2)
// ============================================

// pausedPoll runs one HandleSync on a goroutine and parks it at a seam until
// released, giving the test full control over interleaving (no sleeps).
type pausedPoll struct {
	reached  chan struct{}
	release  chan struct{}
	done     chan struct{}
	recorder *httptest.ResponseRecorder
}

func startPausedPoll(t *testing.T, handler *Handler, payload SyncRequest, park func(func())) *pausedPoll {
	t.Helper()
	poll := &pausedPoll{
		reached: make(chan struct{}), release: make(chan struct{}), done: make(chan struct{}),
		recorder: httptest.NewRecorder(),
	}
	park(func() {
		close(poll.reached)
		<-poll.release
	})
	request := httptest.NewRequest(http.MethodPost, "/sync", bytes.NewReader(mustMarshalJSON(t, payload)))
	go func() {
		defer close(poll.done)
		handler.HandleSync(poll.recorder, request)
	}()
	<-poll.reached
	return poll
}

func (p *pausedPoll) resume() *httptest.ResponseRecorder {
	close(p.release)
	<-p.done
	return p.recorder
}

func TestHandleSync_OldPollResumingAfterReconnectCannotMutateCurrentCommands(t *testing.T) {
	t.Parallel()
	cap := newTestState()
	t.Cleanup(cap.Close)
	clock := newManualClock(time.Now())
	cap.clock = clock.Now

	first := runSyncRequest(t, cap, SyncRequest{ExtSessionID: "session-old"})
	oldGeneration := decodeSyncResponse(t, first).ConnectionGeneration

	// The old extension instance goes quiet past the disconnect threshold, then
	// its poll arrives (observing a disconnect gap) and is parked after
	// adopting connection state but before its command exchange.
	clock.Advance(extensionDisconnectThreshold + time.Second)
	oldHandler := newTestHandler(cap)
	oldPoll := startPausedPoll(t, oldHandler, SyncRequest{
		ExtSessionID:         "session-old",
		ConnectionGeneration: oldGeneration,
		InProgress:           []SyncInProgress{},
	}, func(hook func()) { oldHandler.beforeExchange = hook })

	// Reconnect: the new extension session supersedes the old generation and
	// the daemon queues current-generation work.
	current := runSyncRequest(t, cap, SyncRequest{ExtSessionID: "session-new"})
	currentGeneration := decodeSyncResponse(t, current).ConnectionGeneration
	if currentGeneration <= oldGeneration {
		t.Fatalf("current generation = %d, want > %d", currentGeneration, oldGeneration)
	}
	correlationID := "corr-current-after-reconnect"
	queryID, err := cap.commands.Enqueue(queries.PendingQuery{
		Type: "browser_action", Params: json.RawMessage(`{"action":"highlight"}`), CorrelationID: correlationID,
	}, queries.AsyncCommandTimeout, "")
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	response := oldPoll.resume()
	if response.Code != http.StatusConflict {
		t.Fatalf("resumed old poll status = %d, want 409", response.Code)
	}
	command, found := cap.commands.Command(correlationID)
	if !found || command.Status != "pending" {
		t.Fatalf("current command = %#v found=%v; superseded disconnect expiry must not touch it", command, found)
	}
	if position := cap.commands.QueuePosition(correlationID); position < 0 {
		t.Fatalf("current command %s left the delivery queue after superseded poll", queryID)
	}
}

func TestHandleSync_OldPollResumingFromLongPollAfterReconnectIsRejected(t *testing.T) {
	t.Parallel()
	cap := newTestState()
	t.Cleanup(cap.Close)

	first := runSyncRequest(t, cap, SyncRequest{ExtSessionID: "session-old"})
	oldGeneration := decodeSyncResponse(t, first).ConnectionGeneration

	oldHandler := newTestHandler(cap)
	oldPoll := startPausedPoll(t, oldHandler, SyncRequest{
		ExtSessionID: "session-old", ConnectionGeneration: oldGeneration,
	}, func(hook func()) {
		oldHandler.awaitPending = func(time.Duration) []queries.PendingQueryResponse {
			hook()
			return cap.commands.Pending()
		}
	})

	runSyncRequest(t, cap, SyncRequest{ExtSessionID: "session-new"})
	correlationID := "corr-current-long-poll"
	if _, err := cap.commands.Enqueue(queries.PendingQuery{
		Type: "browser_action", Params: json.RawMessage(`{"action":"highlight"}`), CorrelationID: correlationID,
	}, queries.AsyncCommandTimeout, ""); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	response := oldPoll.resume()
	if response.Code != http.StatusConflict {
		t.Fatalf("resumed long-poll status = %d, want 409 (commands must not be delivered to a superseded poll)", response.Code)
	}
	if cap.commands.QueueDepth() != 1 {
		t.Fatalf("queue depth = %d, want 1: current work stays queued for the current generation", cap.commands.QueueDepth())
	}
}
