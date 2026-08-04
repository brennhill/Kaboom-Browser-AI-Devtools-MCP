// sync_test_helpers_test.go — Shared helpers for /sync request tests.

package capture

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
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
	NewSyncHandler(cap).HandleSync(w, req)
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

func TestSyncWireSharedFixtureRoundTrips(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../scripts/contracts/testdata/sync-roundtrip.json")
	if err != nil {
		t.Fatalf("read shared sync fixture: %v", err)
	}
	var fixture struct {
		Request  json.RawMessage `json:"request"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode shared sync fixture: %v", err)
	}

	assertJSONRoundTrip[SyncRequest](t, fixture.Request)
	assertJSONRoundTrip[SyncResponse](t, fixture.Response)
}

func assertJSONRoundTrip[T any](t *testing.T, raw json.RawMessage) {
	t.Helper()
	var decoded T
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode %T: %v", decoded, err)
	}
	roundTrip, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("encode %T: %v", decoded, err)
	}
	var want, got any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("normalize source fixture: %v", err)
	}
	if err := json.Unmarshal(roundTrip, &got); err != nil {
		t.Fatalf("normalize round trip: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wire round trip drift:\n got: %s\nwant: %s", roundTrip, raw)
	}
}
