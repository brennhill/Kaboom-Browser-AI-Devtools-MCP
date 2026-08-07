// server_routes_unit_test.go — Tests composition-root HTTP route registration.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func decodeJSONMap(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode JSON: %v; body=%q", err, body)
	}
	return decoded
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

func TestSetupHTTPRoutesRegistersCoreEndpoints(t *testing.T) {
	t.Parallel()
	server := newTestServerForHandlers(t)
	mux, _ := setupHTTPRoutes(server, capture.NewCapture())

	jsonRoot := localRequest(http.MethodGet, "/", nil)
	jsonRoot.Header.Set("Accept", "application/json")
	jsonRecorder := httptest.NewRecorder()
	mux.ServeHTTP(jsonRecorder, jsonRoot)
	if jsonRecorder.Code != http.StatusOK || decodeJSONMap(t, jsonRecorder.Body.Bytes())["name"] != "kaboom-browser-devtools" {
		t.Fatalf("JSON root = %d %s", jsonRecorder.Code, jsonRecorder.Body.String())
	}

	htmlRecorder := httptest.NewRecorder()
	mux.ServeHTTP(htmlRecorder, localRequest(http.MethodGet, "/", nil))
	if htmlRecorder.Code != http.StatusOK || htmlRecorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("HTML root = %d %q", htmlRecorder.Code, htmlRecorder.Header().Get("Content-Type"))
	}

	for _, test := range []struct {
		path   string
		status int
	}{
		{"/health", http.StatusOK},
		{"/diagnostics", http.StatusOK},
		{"/diagnostics.json", http.StatusNotFound},
		{"/missing", http.StatusNotFound},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, localRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.status {
			t.Errorf("GET %s = %d, want %d", test.path, recorder.Code, test.status)
		}
	}
}

func TestSetupHTTPRoutesNilCaptureDoesNotPanic(t *testing.T) {
	server := newTestServerForHandlers(t)
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("setupHTTPRoutes panicked with nil capture: %v\n%s", recovered, debug.Stack())
		}
	}()
	mux, handler := setupHTTPRoutes(server, nil)
	if mux == nil || handler == nil {
		t.Fatal("setupHTTPRoutes returned a nil route dependency")
	}
}
