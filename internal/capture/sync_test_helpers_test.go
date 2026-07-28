// sync_test_helpers_test.go — Shared helpers for /sync request tests.

package capture

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func mustMarshalJSON(t *testing.T, payload any) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal JSON payload: %v", err)
	}
	return data
}

func runSyncRawRequest(t *testing.T, cap *Capture, method string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/sync", bytes.NewReader(body))
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	cap.HandleSync(w, req)
	return w
}

func runSyncRequest(t *testing.T, cap *Capture, payload SyncRequest) *httptest.ResponseRecorder {
	t.Helper()
	return runSyncRawRequest(t, cap, "POST", mustMarshalJSON(t, payload))
}

func decodeSyncResponse(t *testing.T, w *httptest.ResponseRecorder) SyncResponse {
	t.Helper()
	var resp SyncResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode sync response: %v", err)
	}
	return resp
}

func assertCommandResult(t *testing.T, cap *Capture, corrID, wantStatus, wantError string) {
	t.Helper()
	cmd, found := cap.Queries().GetCommandResult(corrID)
	if !found {
		t.Fatal("expected command result to be present for correlation_id")
	}
	if cmd.Status != wantStatus {
		t.Errorf("command status = %q, want %q", cmd.Status, wantStatus)
	}
	if cmd.Error != wantError {
		t.Errorf("command error = %q, want %q", cmd.Error, wantError)
	}
}

func runQueryResultRequest(t *testing.T, cap *Capture, payload string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader(payload))
	w := httptest.NewRecorder()
	NewHTTPHandlers(cap).HandleQueryResult(w, req)
	return w
}
