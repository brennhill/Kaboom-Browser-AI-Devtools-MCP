// sync_test_helpers_test.go — Shared helpers for /sync request tests.

package syncruntime

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/featureusage"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/httpingest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/perfstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/circuit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/redaction"
)

type testState struct {
	runtime        *Runtime
	queries        *queries.QueryDispatcher
	lifecycle      *lifecycle.Observer
	featureUsage   *featureusage.Observer
	extensionLogs  *logstore.Extension
	diagnosticLogs *logstore.Diagnostic
	telemetry      *telemetrystore.Store
	performance    *perfstore.Store
	recordings     *recording.RecordingManager
	circuit        *circuit.CircuitBreaker
}

func newTestState() *testState {
	runtime := New()
	lifecycleObserver := lifecycle.NewObserver()
	redactor := redaction.NewRedactionEngine("")
	return &testState{
		runtime: runtime, queries: queries.NewQueryDispatcher(), lifecycle: lifecycleObserver,
		featureUsage: featureusage.New(), extensionLogs: logstore.NewExtension(redactor.Redact),
		diagnosticLogs: logstore.NewDiagnostic(redactor.Redact),
		telemetry:      telemetrystore.New(telemetrystore.Dependencies{ActiveTestIDs: runtime.GetActiveTestIDs}),
		performance:    perfstore.New(), recordings: recording.NewRecordingManager(),
		circuit: circuit.NewCircuitBreaker(lifecycleObserver.Emit),
	}
}

func (s *testState) Close()                             { s.queries.Close() }
func (s *testState) Extension() *Runtime                { return s.runtime }
func (s *testState) Queries() *queries.QueryDispatcher  { return s.queries }
func (s *testState) ExtensionLogs() *logstore.Extension { return s.extensionLogs }
func (s *testState) Telemetry() *telemetrystore.Store   { return s.telemetry }

func newTestHandler(s *testState) *Handler {
	return NewHandler(Dependencies{Runtime: s.runtime, Queries: s.queries, Lifecycle: s.lifecycle,
		FeatureUsage: s.featureUsage, ExtensionLogs: s.extensionLogs, DiagnosticLogs: s.diagnosticLogs})
}

func httpIngestForTest(s *testState) *httpingest.Handlers {
	return httpingest.New(httpingest.Dependencies{Telemetry: s.telemetry, Queries: s.queries,
		Recordings: s.recordings, Performance: s.performance, Circuit: s.circuit})
}

func mustMarshalJSON(t *testing.T, payload any) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal JSON payload: %v", err)
	}
	return data
}

func runSyncRawRequest(t *testing.T, cap *testState, method string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/sync", bytes.NewReader(body))
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	newTestHandler(cap).HandleSync(w, req)
	return w
}

func runSyncRequest(t *testing.T, cap *testState, payload SyncRequest) *httptest.ResponseRecorder {
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

func assertCommandResult(t *testing.T, cap *testState, corrID, wantStatus, wantError string) {
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

func runQueryResultRequest(t *testing.T, cap *testState, payload string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader(payload))
	w := httptest.NewRecorder()
	httpIngestForTest(cap).HandleQueryResult(w, req)
	return w
}

func TestSyncWireSharedFixtureRoundTrips(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../scripts/contracts/testdata/sync-roundtrip.json")
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
