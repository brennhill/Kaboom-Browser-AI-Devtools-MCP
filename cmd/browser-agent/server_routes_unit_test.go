// Purpose: Unit tests for browser-agent server routes logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"bytes"
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"testing"
	"time"

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

func extensionRouteRequest(method, path string, body io.Reader) *http.Request {
	request := localRequest(method, path, body)
	request.Header.Set("X-Kaboom-Client", "kaboom-extension/test")
	return request
}

func TestSetupHTTPRoutesRejectsWrongMethodsForPostOnlyEndpoints(t *testing.T) {
	t.Parallel()

	server := newTestServerForHandlers(t)
	mux, _ := setupHTTPRoutes(server, capture.NewCapture())
	paths := []string{
		"/websocket-events", "/network-bodies", "/network-waterfall", "/query-result",
		"/enhanced-actions", "/performance-snapshots", "/sync", "/logs", "/screenshots",
		"/draw-mode/complete", "/shutdown", "/clear", "/test-boundary",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, extensionRouteRequest(http.MethodGet, path, nil))
			if recorder.Code != http.StatusMethodNotAllowed {
				t.Errorf("GET %s status = %d, want 405", path, recorder.Code)
			}
		})
	}
}

func TestSetupHTTPRoutesReturnsJSONForMalformedIngestPayloads(t *testing.T) {
	t.Parallel()

	server := newTestServerForHandlers(t)
	mux, _ := setupHTTPRoutes(server, capture.NewCapture())
	paths := []string{
		"/network-bodies", "/network-waterfall", "/query-result", "/enhanced-actions",
		"/performance-snapshots", "/logs", "/draw-mode/complete",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := extensionRouteRequest(http.MethodPost, path, bytes.NewBufferString("{invalid"))
			mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("POST %s status = %d, want 400", path, recorder.Code)
			}
			if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
				t.Errorf("POST %s content type = %q, want application/json", path, contentType)
			}
			if body := decodeJSONMap(t, recorder.Body.Bytes()); body["error"] == nil {
				t.Errorf("POST %s response missing error: %#v", path, body)
			}
		})
	}
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
