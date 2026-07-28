// Purpose: Tests for long-poll synchronization of captured data.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestHandleSync_LongPolling(t *testing.T) {
	cap := NewCapture()

	timeout := syncLongPollTimeout()
	queueDelay := timeout / 2
	if queueDelay < 20*time.Millisecond {
		queueDelay = 20 * time.Millisecond
	}

	// Start a goroutine that will queue a command halfway through the poll window.
	go func() {
		time.Sleep(queueDelay)
		cap.Queries().CreatePendingQuery(queries.PendingQuery{
			Type:   "test_cmd",
			Params: json.RawMessage(`{"foo":"bar"}`),
		})
	}()

	reqBody, _ := json.Marshal(SyncRequest{ExtSessionID: "test"})
	req := httptest.NewRequest("POST", "/sync", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	start := time.Now()
	cap.HandleSync(w, req)
	duration := time.Since(start)

	minExpected := queueDelay - 10*time.Millisecond
	if minExpected < 1*time.Millisecond {
		minExpected = 1 * time.Millisecond
	}
	if duration < minExpected {
		t.Errorf("Sync returned too fast (%v), long-polling should have waited for command (min %v)", duration, minExpected)
	}

	var resp SyncResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(resp.Commands))
	}
}

func TestHandleSync_TimeoutIfNoCommand(t *testing.T) {
	cap := NewCapture()

	timeout := syncLongPollTimeout()

	reqBody, _ := json.Marshal(SyncRequest{ExtSessionID: "test"})
	req := httptest.NewRequest("POST", "/sync", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	start := time.Now()
	cap.HandleSync(w, req) // Should wait roughly syncLongPollTimeout().
	duration := time.Since(start)

	minExpected := timeout - 20*time.Millisecond
	if minExpected < 1*time.Millisecond {
		minExpected = 1 * time.Millisecond
	}
	if duration < minExpected {
		t.Errorf("Sync timeout too short (%v), expected around %v", duration, timeout)
	}

	var resp SyncResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Commands) != 0 {
		t.Errorf("Expected 0 commands, got %d", len(resp.Commands))
	}
}
