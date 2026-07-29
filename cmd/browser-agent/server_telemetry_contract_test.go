// server_telemetry_contract_test.go — Covers the local telemetry HTTP contract.
// Docs: docs/features/feature/observe/index.md

package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func TestTelemetryHandlerAllTypesAndValidation(t *testing.T) {
	server, err := NewServer(filepath.Join(t.TempDir(), "telemetry.jsonl"), 100)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	handler := handleTelemetry(server, capture.NewCapture())

	for _, telemetryType := range []string{
		"logs", "network_waterfall", "network_bodies", "websocket_events",
		"actions", "performance_snapshots", "extension_logs", "websocket_status",
	} {
		t.Run(telemetryType, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler(recorder, httptest.NewRequest(http.MethodGet, "/telemetry?type="+telemetryType+"&limit=invalid", nil))
			if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"`+telemetryType+`"`) {
				t.Fatalf("response = %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}

	for _, tc := range []struct {
		method string
		target string
		status int
	}{
		{method: http.MethodPost, target: "/telemetry?type=logs", status: http.StatusMethodNotAllowed},
		{method: http.MethodGet, target: "/telemetry", status: http.StatusBadRequest},
		{method: http.MethodGet, target: "/telemetry?type=unknown", status: http.StatusBadRequest},
	} {
		recorder := httptest.NewRecorder()
		handler(recorder, httptest.NewRequest(tc.method, tc.target, nil))
		if recorder.Code != tc.status {
			t.Fatalf("%s %s = %d, want %d", tc.method, tc.target, recorder.Code, tc.status)
		}
	}
}
