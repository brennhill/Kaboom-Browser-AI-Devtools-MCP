// Purpose: Tests for long-poll synchronization of captured data.
// Docs: docs/features/feature/backend-log-streaming/index.md

package syncruntime

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestHandleSync_LongPolling(t *testing.T) {
	cap := newTestState()
	t.Cleanup(cap.Close)
	waiting := make(chan struct{})
	releaseWait := make(chan struct{})
	handler := newTestHandler(cap)
	handler.waitForPendingQueries = func(time.Duration) {
		close(waiting)
		<-releaseWait
	}

	reqBody, _ := json.Marshal(SyncRequest{ExtSessionID: "test"})
	req := httptest.NewRequest("POST", "/sync", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.HandleSync(w, req)
	}()
	<-waiting
	if _, err := cap.Queries().CreatePendingQuery(queries.PendingQuery{
		Type:   "test_cmd",
		Params: json.RawMessage(`{"foo":"bar"}`),
	}); err != nil {
		t.Fatalf("CreatePendingQuery() error = %v", err)
	}
	close(releaseWait)
	<-done

	var resp SyncResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(resp.Commands))
	}
}

func TestHandleSync_TimeoutIfNoCommand(t *testing.T) {
	cap := newTestState()
	t.Cleanup(cap.Close)

	timeout := syncLongPollTimeout()
	var receivedTimeout time.Duration
	handler := newTestHandler(cap)
	handler.waitForPendingQueries = func(got time.Duration) { receivedTimeout = got }

	reqBody, _ := json.Marshal(SyncRequest{ExtSessionID: "test"})
	req := httptest.NewRequest("POST", "/sync", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleSync(w, req)
	if receivedTimeout != timeout {
		t.Fatalf("wait timeout = %v, want %v", receivedTimeout, timeout)
	}

	var resp SyncResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Commands) != 0 {
		t.Errorf("Expected 0 commands, got %d", len(resp.Commands))
	}
}
