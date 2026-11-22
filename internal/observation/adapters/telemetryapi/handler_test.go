// handler_test.go — Tests local telemetry HTTP type, limit, and validation contracts.

package telemetryapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/logstate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
)

func TestHandlerSupportsEveryTelemetryTypeAndValidation(t *testing.T) {
	logs := logstore.New(logstore.Config{LogFile: filepath.Join(t.TempDir(), "telemetry.jsonl"), MaxEntries: 100})
	t.Cleanup(func() { logs.Shutdown(0) })
	logs.AddEntries([]types.LogEntry{{"level": "error", "message": "boom"}})
	captured := capture.NewCapture()
	handler := Handler(logs, Sources{Telemetry: captured.Telemetry(), Performance: captured.Performance(), ExtensionLogs: captured.ExtensionLogs()})

	for _, telemetryType := range []string{
		"logs", "network_waterfall", "network_bodies", "websocket_events",
		"actions", "performance_snapshots", "extension_logs", "websocket_status",
	} {
		recorder := httptest.NewRecorder()
		handler(recorder, httptest.NewRequest(http.MethodGet, "/telemetry?type="+telemetryType+"&limit=invalid", nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"`+telemetryType+`"`) {
			t.Fatalf("%s response = %d %s", telemetryType, recorder.Code, recorder.Body.String())
		}
	}
	limited := httptest.NewRecorder()
	handler(limited, httptest.NewRequest(http.MethodGet, "/telemetry?type=logs&limit=1", nil))
	if limited.Code != http.StatusOK || !strings.Contains(limited.Body.String(), `"count":1`) {
		t.Fatalf("bounded logs response = %d %s", limited.Code, limited.Body.String())
	}
	for _, test := range []struct {
		method string
		target string
		status int
	}{
		{http.MethodPost, "/telemetry?type=logs", http.StatusMethodNotAllowed},
		{http.MethodGet, "/telemetry", http.StatusBadRequest},
		{http.MethodGet, "/telemetry?type=unknown", http.StatusBadRequest},
	} {
		recorder := httptest.NewRecorder()
		handler(recorder, httptest.NewRequest(test.method, test.target, nil))
		if recorder.Code != test.status {
			t.Fatalf("%s %s = %d, want %d", test.method, test.target, recorder.Code, test.status)
		}
	}
}
