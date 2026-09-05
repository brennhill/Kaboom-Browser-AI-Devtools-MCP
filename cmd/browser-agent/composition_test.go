// composition_test.go — Tests browser-agent composition-root wiring and contracts.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/binarywatch"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/runtimeflags"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func newTestServerForHandlers(t *testing.T) *Server {
	t.Helper()
	server, err := NewServer(filepath.Join(t.TempDir(), "logs.jsonl"), 1000)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(server.Close)
	return server
}

func TestEarlyDoctorActivatesExplicitStateDirectory(t *testing.T) {
	t.Setenv(state.StateDirEnv, "")
	stateRoot := t.TempDir()
	flags := runtimeflags.Values{DoctorMode: true, StateDir: stateRoot}
	if err := activateEarlyModeStateDir(flags); err != nil {
		t.Fatalf("activate early Doctor state: %v", err)
	}
	resolved, err := state.RootDir()
	if err != nil {
		t.Fatalf("resolve state root: %v", err)
	}
	if resolved != stateRoot {
		t.Fatalf("state root = %q, want %q", resolved, stateRoot)
	}
}

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

// --- Daemon startup wiring helpers (main_connection_mcp.go) ---

func TestReportUpgradeMarkerAnnouncesAndClearsMarker(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	server := newTestServerForHandlers(t)

	markerPath, err := state.UpgradeMarkerFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := binarywatch.WriteMarker("1.2.3", "1.3.0", markerPath); err != nil {
		t.Fatal(err)
	}

	reportUpgradeMarker(server, 7890)

	warnings := server.warnings.Drain()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "Upgraded from v1.2.3 to v1.3.0") {
		t.Fatalf("warnings = %#v", warnings)
	}
	if _, statErr := os.Stat(markerPath); !os.IsNotExist(statErr) {
		t.Fatalf("marker not cleared: %v", statErr)
	}
}

func TestReportUpgradeMarkerSilentWithoutMarker(t *testing.T) {
	t.Setenv(state.StateDirEnv, t.TempDir())
	server := newTestServerForHandlers(t)

	reportUpgradeMarker(server, 7890)

	if warnings := server.warnings.Drain(); len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
}

func TestEnsurePortAvailableSucceedsOnFreePort(t *testing.T) {
	server := newTestServerForHandlers(t)
	port := freeLoopbackPort(t)

	if err := ensurePortAvailable(server, daemonHTTPDeps(server), port, "test"); err != nil {
		t.Fatalf("free port rejected: %v", err)
	}
}

func TestEnsurePortAvailableErrorsWhenPortHeldBySelf(t *testing.T) {
	server := newTestServerForHandlers(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	heldPort := listener.Addr().(*net.TCPAddr).Port

	// The holder is this test process, which ReclaimPort must not kill; the
	// retry preflight still fails, so the error is returned.
	err = ensurePortAvailable(server, daemonHTTPDeps(server), heldPort, "test")
	if err == nil {
		t.Fatal("self-held port must surface an error, not start a rival server")
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("error = %v", err)
	}
}

func TestRecordTerminalUnavailableReportsBindFailure(t *testing.T) {
	server := newTestServerForHandlers(t)

	recordTerminalUnavailable(server, 17891, errors.New("bind: address already in use"))

	snapshot := server.terminalStatus.Snapshot()
	if snapshot.Available {
		t.Fatal("terminal must be marked unavailable")
	}
	if snapshot.Port != 17891 {
		t.Fatalf("port = %d, want the wanted port 17891", snapshot.Port)
	}
	if !strings.Contains(snapshot.Error, "address already in use") {
		t.Fatalf("error = %q", snapshot.Error)
	}
}

func TestStartBinaryUpgradeWatcherRespectsDisableEnv(t *testing.T) {
	t.Setenv("KABOOM_NO_AUTO_UPGRADE", "1")
	server := newTestServerForHandlers(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startBinaryUpgradeWatcher(server, 7890, ctx)

	// binarywatch.Start returns a nil *State when disabled, which reaches the
	// runtime interface as a typed nil — assert on the concrete value.
	if watcher, _ := server.runtime.Upgrade().(*binarywatch.State); watcher != nil {
		t.Fatal("disabled auto-upgrade must not register a watcher")
	}
}

func TestStartBinaryUpgradeWatcherRegistersWatcher(t *testing.T) {
	t.Setenv("KABOOM_NO_AUTO_UPGRADE", "")
	server := newTestServerForHandlers(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startBinaryUpgradeWatcher(server, 7890, ctx)

	watcher, ok := server.runtime.Upgrade().(*binarywatch.State)
	if !ok || watcher == nil {
		t.Fatal("watcher was not registered on the runtime")
	}
	if pending, newVersion, _ := watcher.UpgradeInfo(); pending || newVersion != "" {
		t.Fatalf("fresh watcher reports upgrade = (%v, %q)", pending, newVersion)
	}
	cancel()
}

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
