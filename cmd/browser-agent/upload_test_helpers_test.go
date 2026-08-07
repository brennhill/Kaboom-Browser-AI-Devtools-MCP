// upload_test_helpers_test.go — Shared test helpers for upload tests.
// Why: Provides the upload environment and HTTP fixture used by upload integration tests.

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agenthttp "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
	uploadapi "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/osauto"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

type uploadTestEnv struct {
	*toolTestEnv
}

// newUploadTestEnv creates a test environment with upload automation enabled.
func newUploadTestEnv(t *testing.T) *uploadTestEnv {
	t.Helper()
	uploadsec.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadsec.SetSkipSSRFCheck(false) })

	server, err := NewServer(filepath.Join(t.TempDir(), "test-upload.jsonl"), 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	cap := capture.NewCapture()
	mockConnectedTrackedTab(t, cap)
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.tools.Executor.(*ToolHandler)

	handler.uploadSecurity = uploadsec.NewSecurity("/", nil)

	return &uploadTestEnv{
		toolTestEnv: &toolTestEnv{handler: handler, server: server, capture: cap},
	}
}

// createTestFile creates a temp file with given content and returns its path.
func createTestFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return path
}

// handleFileRead directly calls the file read handler for unit testing.
func (e *uploadTestEnv) handleFileRead(t *testing.T, req upload.FileReadRequest) upload.FileReadResponse {
	t.Helper()
	return upload.HandleFileRead(req, e.handler.uploadSecurity, false)
}

// handleDialogInject directly calls the dialog inject handler for unit testing.
func (e *uploadTestEnv) handleDialogInject(t *testing.T, req upload.FileDialogInjectRequest) upload.StageResponse {
	t.Helper()
	return upload.HandleDialogInject(req, e.handler.uploadSecurity)
}

// handleFormSubmit directly calls the form submit handler for unit testing.
func (e *uploadTestEnv) handleFormSubmit(t *testing.T, req upload.FormSubmitRequest) upload.StageResponse {
	t.Helper()
	return upload.HandleFormSubmit(req, e.handler.uploadSecurity)
}

// handleOSAutomation directly calls the OS automation handler for unit testing.
func (e *uploadTestEnv) handleOSAutomation(t *testing.T, req upload.OSAutomationInjectRequest) upload.StageResponse {
	t.Helper()
	return osauto.HandleOSAutomation(req, e.handler.uploadSecurity)
}

func newUploadHTTPServer(t *testing.T, osAutomationEnabled bool) *httptest.Server {
	t.Helper()
	uploadsec.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadsec.SetSkipSSRFCheck(false) })
	handlers := uploadapi.NewHandlers(uploadsec.NewSecurity("/", nil), osAutomationEnabled, agenthttp.JSON)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/file/read", handlers.HandleFileRead)
	mux.HandleFunc("/api/file/dialog/inject", handlers.HandleFileDialogInject)
	mux.HandleFunc("/api/form/submit", handlers.HandleFormSubmit)
	mux.HandleFunc("/api/os-automation/inject", handlers.HandleOSAutomation)
	return httptest.NewServer(mux)
}

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	response, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	return response
}
