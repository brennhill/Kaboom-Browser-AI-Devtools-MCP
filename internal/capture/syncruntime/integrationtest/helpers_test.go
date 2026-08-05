// helpers_test.go — Public-boundary fixtures for extension sync integration tests.
package integrationtest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
)

func newSyncHandlerForTest(state *capture.Capture) *syncruntime.Handler {
	return syncruntime.NewHandler(syncruntime.Dependencies{
		Runtime: state.Extension(), Queries: state.Queries(), Lifecycle: state.Lifecycle(),
		FeatureUsage: state.FeatureUsage(), ExtensionLogs: state.ExtensionLogs(), DiagnosticLogs: state.DiagnosticLogs(),
	})
}

func runSyncRequest(t *testing.T, state *capture.Capture, payload syncruntime.SyncRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal sync request: %v", err)
	}
	response := httptest.NewRecorder()
	newSyncHandlerForTest(state).HandleSync(response, httptest.NewRequest(http.MethodPost, "/sync", bytes.NewReader(body)))
	return response
}

func decodeSyncResponse(t *testing.T, response *httptest.ResponseRecorder) syncruntime.SyncResponse {
	t.Helper()
	var payload syncruntime.SyncResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode sync response: %v", err)
	}
	return payload
}
