// Purpose: Provides test fixture constructors shared by capture package tests.
// Why: Reduces duplicated test bootstrapping code and keeps test setup behavior consistent.
// Docs: docs/features/feature/self-testing/index.md

package capture

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/httpingest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/resetter"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/telemetrystore"
	recordingmodel "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/server"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func newSyncHandlerForTest(capture *Capture) *syncruntime.Handler {
	return syncruntime.NewHandler(syncruntime.Dependencies{
		Runtime: capture.Extension(), Queries: capture.Queries(), Lifecycle: capture.Lifecycle(),
		FeatureUsage: capture.FeatureUsage(), ExtensionLogs: capture.ExtensionLogs(), DiagnosticLogs: capture.DiagnosticLogs(),
	})
}

func runSyncRequest(t *testing.T, capture *Capture, payload syncruntime.SyncRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal sync request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/sync", bytes.NewReader(body))
	response := httptest.NewRecorder()
	newSyncHandlerForTest(capture).HandleSync(response, request)
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

func runQueryResultRequest(t *testing.T, capture *Capture, payload string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/query-result", strings.NewReader(payload))
	w := httptest.NewRecorder()
	httpIngestForTest(capture).HandleQueryResult(w, req)
	return w
}

func assertCommandResult(t *testing.T, capture *Capture, corrID, wantStatus, wantError string) {
	t.Helper()
	command, found := capture.Queries().GetCommandResult(corrID)
	if !found {
		t.Fatal("expected command result")
	}
	if command.Status != wantStatus || command.Error != wantError {
		t.Fatalf("command result = (%q, %q), want (%q, %q)", command.Status, command.Error, wantStatus, wantError)
	}
}

const maxWSEvents = 500
const wsBufferMemoryLimit = 4 * 1024 * 1024

func TestMain(m *testing.M) {
	stateRoot, err := os.MkdirTemp("", "kaboom-capture-tests-*")
	if err != nil {
		panic("create capture test state root: " + err.Error())
	}
	if err := os.Setenv(state.StateDirEnv, stateRoot); err != nil {
		panic("set capture test state root: " + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(stateRoot)
	os.Exit(code)
}

// setupTestCapture creates a new Capture instance for testing.
func setupTestCapture(t *testing.T) *Capture {
	t.Helper()
	return NewCapture()
}

func replaceTelemetryForTest(capture *Capture, deps telemetrystore.Dependencies) {
	deps.ActiveTestIDs = capture.extension.GetActiveTestIDs
	capture.telemetry = telemetrystore.New(deps)
}

func httpIngestForTest(capture *Capture) *httpingest.Handlers {
	return httpingest.New(httpingest.Dependencies{Telemetry: capture.Telemetry(), Queries: capture.Queries(), Recordings: capture.Recordings(), Performance: capture.Performance(), Circuit: capture.Circuit()})
}

func resetterForTest(capture *Capture) *resetter.Resetter {
	return resetter.New(resetter.Dependencies{Extension: capture.Extension(), Telemetry: capture.Telemetry(), Performance: capture.Performance(), ExtensionLogs: capture.ExtensionLogs()})
}

func mustStartRecording(t *testing.T, capture *Capture, name, pageURL string, sensitive bool) string {
	t.Helper()
	recordingID, err := capture.Recordings().StartRecording(name, pageURL, sensitive)
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}
	return recordingID
}

func mustGetRecording(t *testing.T, capture *Capture, recordingID string) *recordingmodel.Recording {
	t.Helper()
	recording, err := capture.Recordings().GetRecording(recordingID)
	if err != nil {
		t.Fatalf("GetRecording(%q) error = %v", recordingID, err)
	}
	return recording
}

// setupTestServer creates a test instance of Server with a temporary log file.
func setupTestServer(t *testing.T) (*server.Server, string) {
	t.Helper()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")

	srv, err := server.NewServer(logFile, 10)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return srv, logFile
}

// setupToolHandler creates a placeholder tool handler for integration tests.
// Returns nil — integration tests that need real handler behavior should
// construct their own via the cmd layer.
func setupToolHandler(t *testing.T, server *server.Server, capture *Capture) any {
	t.Helper()
	return nil
}
