// coverage_contract_test.go — Behavioral coverage for operational HTTP routes.

package operationalapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/health"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestOperationalDiagnosticsRedactsHTTPAndIncludesCommandTrace(t *testing.T) {
	handler := newOperationalTestHandler(t)
	const rawSecret = "Bearer tokenValue1234567890abcdef"
	handler.options.Capture.DiagnosticLogs().AddHTTP(types.HTTPDebugEntry{
		Timestamp: time.Now(), Endpoint: "/mcp", Method: http.MethodPost,
		RequestBody: `{"auth":"` + rawSecret + `"}`, ResponseBody: `{"ok":true}`, DurationMs: 5,
	})
	queryID, err := handler.options.Capture.Queries().CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type: "browser_action", CorrelationID: "diag-trace-corr",
	}, 30*time.Second, "test-client")
	if err != nil {
		t.Fatal(err)
	}
	_ = handler.options.Capture.Queries().GetPendingQueries()
	handler.options.Capture.Queries().AcknowledgePendingQuery(queryID)
	handler.options.Capture.Queries().ApplyCommandResult("diag-trace-corr", "complete", json.RawMessage(`{"ok":true}`), "")

	recorder := httptest.NewRecorder()
	handler.ServeDiagnostics(recorder, httptest.NewRequest(http.MethodGet, "/diagnostics", nil))
	body := decodeOperationalResponse(t, recorder)
	httpDebug := body["http_debug_log"].(map[string]any)
	encodedDebug, err := json.Marshal(httpDebug)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encodedDebug), rawSecret) || !strings.Contains(string(encodedDebug), "[REDACTED:bearer-token]") {
		t.Fatalf("HTTP diagnostics were not redacted: %s", encodedDebug)
	}
	traces := body["command_traces"].(map[string]any)
	entries := traces["entries"].([]any)
	if traces["count"].(float64) < 1 || len(entries) == 0 {
		t.Fatalf("command traces = %#v", traces)
	}
	first := entries[0].(map[string]any)
	if first["trace_id"] == "" || first["timeline"] == "" {
		t.Fatalf("command trace missing identity or timeline: %#v", first)
	}
}

func newOperationalTestHandler(t *testing.T) *Handler {
	t.Helper()
	logs := logstore.New(logstore.Config{
		MaxEntries: 8,
		LogFile:    filepath.Join(t.TempDir(), "kaboom.jsonl"),
		AddWarning: func(string) {},
	})
	return New(Options{
		Logs:            logs,
		Capture:         capture.NewCapture(),
		Version:         "0.9.0-test",
		StartedAt:       time.Now().Add(-2 * time.Second),
		MaxPostBodySize: 1024,
		TerminalStatus: func() TerminalStatus {
			return TerminalStatus{Port: 7891, Error: "occupied", BlockedByPID: 42, BlockedCommand: "other"}
		},
		AvailableVersion: func() string { return "0.9.1" },
		UpgradeInfo: func() *health.UpgradeInfo {
			return &health.UpgradeInfo{Pending: true, NewVersion: "0.9.1"}
		},
	})
}

func decodeOperationalResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
	}
	return body
}

func TestOperationalHealthReportsOptionalRuntimeState(t *testing.T) {
	handler := newOperationalTestHandler(t)
	handler.options.Logs = logstore.New(logstore.Config{
		MaxEntries: 8, ChanSize: 1, LogFile: filepath.Join(t.TempDir(), "bounded.jsonl"), AddWarning: func(string) {},
	})
	_ = handler.options.Logs.AppendToFile([]types.LogEntry{{"level": "info", "message": "fill"}})
	_ = handler.options.Logs.AppendToFile([]types.LogEntry{{"level": "info", "message": "drop"}})
	t.Cleanup(func() { handler.options.Logs.Shutdown(time.Millisecond) })
	recorder := httptest.NewRecorder()
	handler.ServeHealth(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := decodeOperationalResponse(t, recorder)
	if body["available_version"] != "0.9.1" || body["upgrade_pending"] == nil {
		t.Fatalf("missing upgrade state: %#v", body)
	}
	if body["terminal_port"] != float64(7891) || body["terminal_error"] != "occupied" {
		t.Fatalf("missing terminal diagnostics: %#v", body)
	}
	blocked := body["terminal_blocked_by"].(map[string]any)
	if blocked["pid"] != float64(42) || blocked["command"] != "other" {
		t.Fatalf("terminal blocker = %#v", blocked)
	}
	logs := body["logs"].(map[string]any)
	if logs["dropped_count"] != float64(1) {
		t.Fatalf("health log pressure = %#v, want one dropped batch", logs)
	}

	methodRecorder := httptest.NewRecorder()
	handler.ServeHealth(methodRecorder, httptest.NewRequest(http.MethodPost, "/health", nil))
	if methodRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", methodRecorder.Code)
	}
}

func TestOperationalLogsValidateIngestClearAndMethods(t *testing.T) {
	handler := newOperationalTestHandler(t)
	cases := []struct {
		name   string
		method string
		body   string
		status int
	}{
		{"invalid_json", http.MethodPost, `{`, http.StatusBadRequest},
		{"missing_entries", http.MethodPost, `{}`, http.StatusBadRequest},
		{"valid_entries", http.MethodPost, `{"entries":[{"type":"console","level":"error","args":["boom"],"ts":"2026-07-29T00:00:00Z"}]}`, http.StatusOK},
		{"unsupported_method", http.MethodGet, ``, http.StatusMethodNotAllowed},
		{"clear", http.MethodDelete, ``, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, "/logs", strings.NewReader(tc.body))
			handler.ServeLogs(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.status, recorder.Body.String())
			}
		})
	}
	mixedRecorder := httptest.NewRecorder()
	mixedRequest := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(
		`{"entries":[{"level":"error","message":"boom"},{"level":"invalid","message":"skip"}]}`,
	))
	handler.ServeLogs(mixedRecorder, mixedRequest)
	mixed := decodeOperationalResponse(t, mixedRecorder)
	if mixed["received"] != float64(1) || mixed["rejected"] != float64(1) {
		t.Fatalf("mixed ingest counts = %#v", mixed)
	}
	clearRecorder := httptest.NewRecorder()
	handler.ServeLogs(clearRecorder, httptest.NewRequest(http.MethodDelete, "/logs", nil))
	if handler.options.Logs.EntryCount() != 0 {
		t.Fatalf("entries after clear = %d, want 0", handler.options.Logs.EntryCount())
	}
}

func TestOperationalDiagnosticsSummarizesLatestConsoleEvent(t *testing.T) {
	handler := newOperationalTestHandler(t)
	handler.options.Capture.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{{
		Type: "ws_connect", ID: "conn-1", URL: "wss://example.test",
	}})
	handler.options.Capture.Telemetry().AddNetworkBodies([]types.NetworkBody{{
		URL: "https://example.test/api", Method: http.MethodGet, Status: http.StatusOK,
	}})
	handler.options.Capture.Telemetry().AddEnhancedActions([]types.EnhancedAction{{
		Type: "click", Timestamp: 1000,
	}})
	longMessage := strings.Repeat("x", 120)
	handler.options.Logs.AddEntries([]types.LogEntry{{
		"type": "console", "level": "warn", "args": []any{longMessage}, "ts": "2026-07-29T00:00:00Z",
	}})

	recorder := httptest.NewRecorder()
	handler.ServeDiagnostics(recorder, httptest.NewRequest(http.MethodGet, "/diagnostics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := decodeOperationalResponse(t, recorder)
	console := body["last_events"].(map[string]any)["console"].(map[string]any)
	message := console["message"].(string)
	if len(message) != 103 || !strings.HasSuffix(message, "...") {
		t.Fatalf("console message = %q, want 100 chars plus ellipsis", message)
	}
	if body["launch_mode"] == nil || body["system"] == nil {
		t.Fatalf("diagnostics missing runtime sections: %#v", body)
	}
	buffers := body["buffers"].(map[string]any)
	for _, name := range []string{"websocket_events", "network_bodies", "actions"} {
		if count, ok := buffers[name].(float64); !ok || count < 1 {
			t.Errorf("buffers[%q] = %v, want a real captured count", name, buffers[name])
		}
	}
	circuit := body["circuit"].(map[string]any)
	if _, ok := circuit["open"]; !ok {
		t.Errorf("circuit state missing open field: %#v", circuit)
	}

	methodRecorder := httptest.NewRecorder()
	handler.ServeDiagnostics(methodRecorder, httptest.NewRequest(http.MethodPost, "/diagnostics", nil))
	if methodRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", methodRecorder.Code)
	}
}
