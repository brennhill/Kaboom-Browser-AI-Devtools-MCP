// Purpose: Unit tests for browser-agent server routes logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func decodeJSONMap(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v body=%q", err, string(body))
	}
	return out
}

func localRequest(method, path string, body io.Reader) *http.Request {
	return httptest.NewRequest(method, "http://localhost"+path, body)
}

func TestSetupHTTPRoutesBasicEndpoints(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	mux, _ := setupHTTPRoutes(srv, cap)

	// JSON clients get the discovery response via content negotiation
	rootReq := localRequest(http.MethodGet, "/", nil)
	rootReq.Header.Set("Accept", "application/json")
	rootRR := httptest.NewRecorder()
	mux.ServeHTTP(rootRR, rootReq)
	if rootRR.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rootRR.Code, http.StatusOK)
	}
	rootBody := decodeJSONMap(t, rootRR.Body.Bytes())
	if rootBody["name"] != "kaboom-browser-devtools" {
		t.Fatalf("root name = %v, want kaboom-browser-devtools", rootBody["name"])
	}

	// Browsers (no Accept: application/json) get the HTML dashboard
	htmlReq := localRequest(http.MethodGet, "/", nil)
	htmlRR := httptest.NewRecorder()
	mux.ServeHTTP(htmlRR, htmlReq)
	if htmlRR.Code != http.StatusOK {
		t.Fatalf("GET / (html) status = %d, want %d", htmlRR.Code, http.StatusOK)
	}
	if ct := htmlRR.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("GET / (html) content-type = %q, want text/html; charset=utf-8", ct)
	}

	notFoundReq := localRequest(http.MethodGet, "/missing", nil)
	notFoundRR := httptest.NewRecorder()
	mux.ServeHTTP(notFoundRR, notFoundReq)
	if notFoundRR.Code != http.StatusNotFound {
		t.Fatalf("GET /missing status = %d, want %d", notFoundRR.Code, http.StatusNotFound)
	}

	healthReq := localRequest(http.MethodGet, "/health", nil)
	healthRR := httptest.NewRecorder()
	mux.ServeHTTP(healthRR, healthReq)
	if healthRR.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", healthRR.Code, http.StatusOK)
	}
	healthBody := decodeJSONMap(t, healthRR.Body.Bytes())
	if healthBody["status"] != "ok" {
		t.Fatalf("health status = %v, want ok", healthBody["status"])
	}
	if healthBody["name"] != "kaboom-browser-devtools" {
		t.Fatalf("health name = %v, want kaboom-browser-devtools", healthBody["name"])
	}
	if _, exists := healthBody["service-name"]; exists {
		t.Fatalf("health retains noncanonical service-name field: %v", healthBody)
	}

	healthBadReq := localRequest(http.MethodPost, "/health", nil)
	healthBadRR := httptest.NewRecorder()
	mux.ServeHTTP(healthBadRR, healthBadReq)
	if healthBadRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health status = %d, want %d", healthBadRR.Code, http.StatusMethodNotAllowed)
	}

	const rawSecret = "Bearer tokenValue1234567890abcdef"
	cap.DiagnosticLogs().AddHTTP(types.HTTPDebugEntry{
		Timestamp:    time.Now(),
		Endpoint:     "/mcp",
		Method:       http.MethodPost,
		RequestBody:  `{"auth":"` + rawSecret + `"}`,
		ResponseBody: `{"ok":true}`,
		DurationMs:   5,
	})

	diagReq := localRequest(http.MethodGet, "/diagnostics", nil)
	diagRR := httptest.NewRecorder()
	mux.ServeHTTP(diagRR, diagReq)
	if diagRR.Code != http.StatusOK {
		t.Fatalf("GET /diagnostics status = %d, want %d", diagRR.Code, http.StatusOK)
	}
	diagBody := decodeJSONMap(t, diagRR.Body.Bytes())
	if _, ok := diagBody["generated_at"]; !ok {
		t.Fatalf("diagnostics missing generated_at: %v", diagBody)
	}
	launchMode, ok := diagBody["launch_mode"].(map[string]any)
	if !ok {
		t.Fatalf("diagnostics missing launch_mode payload: %v", diagBody)
	}
	if launchMode["mode"] == "" {
		t.Fatalf("diagnostics launch_mode.mode missing: %v", launchMode)
	}
	httpDebug, ok := diagBody["http_debug_log"].(map[string]any)
	if !ok {
		t.Fatalf("diagnostics missing http_debug_log payload: %v", diagBody)
	}
	entries, ok := httpDebug["entries"].([]any)
	if !ok {
		t.Fatalf("diagnostics http_debug_log.entries missing: %v", httpDebug)
	}
	redactedFound := false
	for _, entryAny := range entries {
		entry, ok := entryAny.(map[string]any)
		if !ok {
			continue
		}
		if entry["endpoint"] != "/mcp" || entry["method"] != http.MethodPost {
			continue
		}
		bodyText, _ := entry["request_body"].(string)
		if strings.Contains(bodyText, rawSecret) {
			t.Fatalf("diagnostics leaked secret in request_body: %q", bodyText)
		}
		if strings.Contains(bodyText, "[REDACTED:bearer-token]") {
			redactedFound = true
		}
	}
	if !redactedFound {
		t.Fatal("diagnostics did not include redacted http debug request body")
	}

	traceQueryID, _ := cap.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "browser_action",
		CorrelationID: "diag-trace-corr",
	}, 30*time.Second, "test-client")
	_ = cap.Queries().GetPendingQueries()
	cap.Queries().AcknowledgePendingQuery(traceQueryID)
	cap.Queries().ApplyCommandResult("diag-trace-corr", "complete", json.RawMessage(`{"ok":true}`), "")

	diagWithTraceRR := httptest.NewRecorder()
	mux.ServeHTTP(diagWithTraceRR, localRequest(http.MethodGet, "/diagnostics", nil))
	if diagWithTraceRR.Code != http.StatusOK {
		t.Fatalf("GET /diagnostics (trace) status = %d, want %d", diagWithTraceRR.Code, http.StatusOK)
	}
	diagWithTrace := decodeJSONMap(t, diagWithTraceRR.Body.Bytes())
	tracePayload, ok := diagWithTrace["command_traces"].(map[string]any)
	if !ok {
		t.Fatalf("diagnostics missing command_traces payload: %v", diagWithTrace)
	}
	traceCount, _ := tracePayload["count"].(float64)
	if traceCount < 1 {
		t.Fatalf("diagnostics command_traces.count = %v, want >=1", traceCount)
	}
	traceEntries, ok := tracePayload["entries"].([]any)
	if !ok || len(traceEntries) == 0 {
		t.Fatalf("diagnostics command_traces.entries missing: %v", tracePayload["entries"])
	}
	firstTrace, ok := traceEntries[0].(map[string]any)
	if !ok {
		t.Fatalf("first trace entry is not object: %T", traceEntries[0])
	}
	if firstTrace["trace_id"] == "" {
		t.Fatalf("trace_id missing in diagnostics trace entry: %v", firstTrace)
	}
	if firstTrace["timeline"] == "" {
		t.Fatalf("timeline missing in diagnostics trace entry: %v", firstTrace)
	}

	diagBadReq := localRequest(http.MethodPost, "/diagnostics", nil)
	diagBadRR := httptest.NewRecorder()
	mux.ServeHTTP(diagBadRR, diagBadReq)
	if diagBadRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /diagnostics status = %d, want %d", diagBadRR.Code, http.StatusMethodNotAllowed)
	}

	diagAliasRR := httptest.NewRecorder()
	mux.ServeHTTP(diagAliasRR, localRequest(http.MethodGet, "/diagnostics.json", nil))
	if diagAliasRR.Code != http.StatusNotFound {
		t.Fatalf("GET /diagnostics.json = %d, want 404 after alias removal", diagAliasRR.Code)
	}

	shutdownBadReq := localRequest(http.MethodGet, "/shutdown", nil)
	shutdownBadReq.Header.Set("X-Kaboom-Client", "kaboom-extension")
	shutdownBadRR := httptest.NewRecorder()
	mux.ServeHTTP(shutdownBadRR, shutdownBadReq)
	if shutdownBadRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /shutdown status = %d, want %d", shutdownBadRR.Code, http.StatusMethodNotAllowed)
	}
}

func TestHealthEndpointExposesDroppedCount(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	mux, _ := setupHTTPRoutes(srv, cap)

	// Create a server with a channel of size 1 and NO async worker,
	// so the channel stays full when we manually fill it.
	tinyLogSrv := newTestServerForHandlers(t)
	previousLogs := tinyLogSrv.logs
	t.Cleanup(func() { previousLogs.Shutdown(2 * time.Second) })
	tinyLogSrv.logs = logstore.New(logstore.Config{
		LogFile:    filepath.Join(t.TempDir(), "drop.jsonl"),
		MaxEntries: 100,
		ChanSize:   1,
		AddWarning: func(string) {},
	})

	tinyMux, _ := setupHTTPRoutes(tinyLogSrv, cap)

	// Fill queue (no worker draining it), then trigger a drop
	_ = tinyLogSrv.logs.AppendToFile([]types.LogEntry{{"level": "info", "message": "fill"}})
	_ = tinyLogSrv.logs.AppendToFile([]types.LogEntry{{"level": "info", "message": "drop"}})

	healthReq := localRequest(http.MethodGet, "/health", nil)
	healthRR := httptest.NewRecorder()
	tinyMux.ServeHTTP(healthRR, healthReq)
	if healthRR.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", healthRR.Code, http.StatusOK)
	}

	healthBody := decodeJSONMap(t, healthRR.Body.Bytes())
	logs, ok := healthBody["logs"].(map[string]any)
	if !ok {
		t.Fatalf("health response missing logs object: %v", healthBody)
	}

	droppedCount, ok := logs["dropped_count"]
	if !ok {
		t.Fatal("health logs missing dropped_count field")
	}
	if droppedCount.(float64) != 1 {
		t.Fatalf("dropped_count = %v, want 1", droppedCount)
	}

	// Shut down cleanly (no worker was started, so Shutdown times out fast)
	tinyLogSrv.logs.Shutdown(10 * time.Millisecond)

	// Verify zero-state too: fresh server should have 0 dropped_count
	freshReq := localRequest(http.MethodGet, "/health", nil)
	freshRR := httptest.NewRecorder()
	mux.ServeHTTP(freshRR, freshReq)
	freshBody := decodeJSONMap(t, freshRR.Body.Bytes())
	freshLogs := freshBody["logs"].(map[string]any)
	if freshLogs["dropped_count"].(float64) != 0 {
		t.Fatalf("fresh server dropped_count = %v, want 0", freshLogs["dropped_count"])
	}
}

func TestLogsEndpointValidationAndMethods(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	mux, _ := setupHTTPRoutes(srv, cap)

	// GET /logs returns 405 (reads go through /telemetry?type=logs)
	getReq := localRequest(http.MethodGet, "/logs", nil)
	getReq.Header.Set("X-Kaboom-Client", "kaboom-extension")
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /logs status = %d, want %d", getRR.Code, http.StatusMethodNotAllowed)
	}

	badJSONReq := localRequest(http.MethodPost, "/logs", bytes.NewBufferString("{"))
	badJSONReq.Header.Set("X-Kaboom-Client", "kaboom-extension")
	badJSONRR := httptest.NewRecorder()
	mux.ServeHTTP(badJSONRR, badJSONReq)
	if badJSONRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /logs invalid json status = %d, want %d", badJSONRR.Code, http.StatusBadRequest)
	}

	missingEntriesReq := localRequest(http.MethodPost, "/logs", bytes.NewBufferString(`{"foo":"bar"}`))
	missingEntriesReq.Header.Set("X-Kaboom-Client", "kaboom-extension")
	missingEntriesRR := httptest.NewRecorder()
	mux.ServeHTTP(missingEntriesRR, missingEntriesReq)
	if missingEntriesRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /logs missing entries status = %d, want %d", missingEntriesRR.Code, http.StatusBadRequest)
	}

	validReq := localRequest(http.MethodPost, "/logs", bytes.NewBufferString(`{"entries":[{"level":"error","message":"boom"},{"level":"invalid","message":"skip"}]}`))
	validReq.Header.Set("X-Kaboom-Client", "kaboom-extension")
	validRR := httptest.NewRecorder()
	mux.ServeHTTP(validRR, validReq)
	if validRR.Code != http.StatusOK {
		t.Fatalf("POST /logs valid payload status = %d, want %d", validRR.Code, http.StatusOK)
	}
	validBody := decodeJSONMap(t, validRR.Body.Bytes())
	if validBody["received"].(float64) != 1 || validBody["rejected"].(float64) != 1 {
		t.Fatalf("POST /logs counts unexpected: %v", validBody)
	}

	deleteReq := localRequest(http.MethodDelete, "/logs", nil)
	deleteReq.Header.Set("X-Kaboom-Client", "kaboom-extension")
	deleteRR := httptest.NewRecorder()
	mux.ServeHTTP(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusOK {
		t.Fatalf("DELETE /logs status = %d, want %d", deleteRR.Code, http.StatusOK)
	}

	putReq := localRequest(http.MethodPut, "/logs", nil)
	putReq.Header.Set("X-Kaboom-Client", "kaboom-extension")
	putRR := httptest.NewRecorder()
	mux.ServeHTTP(putRR, putReq)
	if putRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT /logs status = %d, want %d", putRR.Code, http.StatusMethodNotAllowed)
	}
}

func TestSetupHTTPRoutes_NilCaptureDoesNotPanic(t *testing.T) {
	srv := newTestServerForHandlers(t)

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("setupHTTPRoutes panicked with nil capture: %v\n%s", recovered, debug.Stack())
		}
	}()

	mux, handler := setupHTTPRoutes(srv, nil)
	if mux == nil || handler == nil {
		t.Fatal("setupHTTPRoutes returned a nil route dependency")
	}
}

func TestTelemetryEndpointReadContract(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	srv.logs.AddEntries([]types.LogEntry{{"level": "error", "message": "boom"}})
	mux, _ := setupHTTPRoutes(srv, capture.NewCapture())

	missingRR := httptest.NewRecorder()
	mux.ServeHTTP(missingRR, localRequest(http.MethodGet, "/telemetry", nil))
	if missingRR.Code != http.StatusBadRequest {
		t.Fatalf("GET /telemetry without type status = %d, want %d", missingRR.Code, http.StatusBadRequest)
	}

	logsRR := httptest.NewRecorder()
	mux.ServeHTTP(logsRR, localRequest(http.MethodGet, "/telemetry?type=logs&limit=1", nil))
	if logsRR.Code != http.StatusOK {
		t.Fatalf("GET /telemetry?type=logs status = %d, want %d", logsRR.Code, http.StatusOK)
	}
	body := decodeJSONMap(t, logsRR.Body.Bytes())
	if body["type"] != "logs" || body["count"] != float64(1) {
		t.Fatalf("telemetry response = %v, want type=logs count=1", body)
	}
}

func TestHandleScreenshotRoutes(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	mux, _ := setupHTTPRoutes(srv, cap)

	methodReq := localRequest(http.MethodGet, "/screenshots", nil)
	methodReq.Header.Set("X-Kaboom-Client", "kaboom-extension")
	methodRR := httptest.NewRecorder()
	mux.ServeHTTP(methodRR, methodReq)
	if methodRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /screenshots status = %d, want %d", methodRR.Code, http.StatusMethodNotAllowed)
	}

	// Each POST uses a unique versioned client ID to avoid rate limiting (1 screenshot/sec/client).
	invalidJSONReq := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString("{"))
	invalidJSONReq.Header.Set("X-Kaboom-Client", "kaboom-extension/test-1")
	invalidJSONRR := httptest.NewRecorder()
	mux.ServeHTTP(invalidJSONRR, invalidJSONReq)
	if invalidJSONRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /screenshots invalid json status = %d, want %d", invalidJSONRR.Code, http.StatusBadRequest)
	}

	missingDataReq := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(`{"url":"https://example.test"}`))
	missingDataReq.Header.Set("X-Kaboom-Client", "kaboom-extension/test-2")
	missingDataRR := httptest.NewRecorder()
	mux.ServeHTTP(missingDataRR, missingDataReq)
	if missingDataRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /screenshots missing data_url status = %d, want %d", missingDataRR.Code, http.StatusBadRequest)
	}

	badFormatReq := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(`{"data_url":"not-a-data-url"}`))
	badFormatReq.Header.Set("X-Kaboom-Client", "kaboom-extension/test-3")
	badFormatRR := httptest.NewRecorder()
	mux.ServeHTTP(badFormatRR, badFormatReq)
	if badFormatRR.Code != http.StatusBadRequest {
		t.Fatalf("POST /screenshots bad data_url format status = %d, want %d", badFormatRR.Code, http.StatusBadRequest)
	}

	badBase64Req := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(`{"data_url":"data:image/jpeg;base64,%%%INVALID%%%"}`))
	badBase64Req.Header.Set("X-Kaboom-Client", "kaboom-extension/test-4")
	badBase64RR := httptest.NewRecorder()
	mux.ServeHTTP(badBase64RR, badBase64Req)
	if badBase64RR.Code != http.StatusBadRequest {
		t.Fatalf("POST /screenshots invalid base64 status = %d, want %d", badBase64RR.Code, http.StatusBadRequest)
	}

	rawImage := []byte("abc123")
	dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(rawImage)
	validBody := `{"data_url":"` + dataURL + `","url":"https://example.test/page","correlation_id":"corr-1","query_id":"query-1"}`
	validReq := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(validBody))
	validReq.Header.Set("X-Kaboom-Client", "kaboom-extension/test-5")
	validRR := httptest.NewRecorder()
	mux.ServeHTTP(validRR, validReq)
	if validRR.Code != http.StatusOK {
		t.Fatalf("POST /screenshots valid status = %d, want %d body=%q", validRR.Code, http.StatusOK, validRR.Body.String())
	}
	resp := decodeJSONMap(t, validRR.Body.Bytes())
	savePath := resp["path"].(string)
	if _, err := os.Stat(savePath); err != nil {
		t.Fatalf("saved screenshot path %q stat error = %v", savePath, err)
	}
	if !strings.Contains(resp["filename"].(string), "example.test") {
		t.Fatalf("filename = %q, expected sanitized hostname", resp["filename"])
	}

	if result, ok := cap.Queries().TakeQueryResult("query-1"); !ok || len(result) == 0 {
		t.Fatalf("expected query result for query-1 to be set, got ok=%v result=%q", ok, string(result))
	}

	unsolicitedBody := `{"data_url":"` + dataURL + `","url":"https://example.test/page"}`
	rlReq1 := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(unsolicitedBody))
	rlReq1.Header.Set("X-Kaboom-Client", "kaboom-extension/rl-client")
	rlRR1 := httptest.NewRecorder()
	mux.ServeHTTP(rlRR1, rlReq1)
	if rlRR1.Code != http.StatusOK {
		t.Fatalf("rate-limit first request status = %d, want %d", rlRR1.Code, http.StatusOK)
	}

	rlReq2 := localRequest(http.MethodPost, "/screenshots", bytes.NewBufferString(unsolicitedBody))
	rlReq2.Header.Set("X-Kaboom-Client", "kaboom-extension/rl-client")
	rlRR2 := httptest.NewRecorder()
	mux.ServeHTTP(rlRR2, rlReq2)
	if rlRR2.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limit second request status = %d, want %d", rlRR2.Code, http.StatusTooManyRequests)
	}

}
