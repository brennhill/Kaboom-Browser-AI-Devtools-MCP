// upload_test_helpers_test.go — Shared test helpers for upload tests.
// Why: Provides the upload environment and HTTP fixture used by upload integration tests.

package uploadintegration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	uploadapi "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

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

func newUploadHTTPServer(t *testing.T, osAutomationEnabled bool) *httptest.Server {
	t.Helper()
	uploadsec.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadsec.SetSkipSSRFCheck(false) })
	handlers := uploadapi.NewHandlers(uploadsec.NewSecurity("/", nil), osAutomationEnabled, writeTestJSON)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/file/read", handlers.HandleFileRead)
	mux.HandleFunc("/api/file/dialog/inject", handlers.HandleFileDialogInject)
	mux.HandleFunc("/api/form/submit", handlers.HandleFormSubmit)
	mux.HandleFunc("/api/os-automation/inject", handlers.HandleOSAutomation)
	return httptest.NewServer(mux)
}

func writeTestJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		panic("test JSON response encode failed: " + err.Error())
	}
}

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	response, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	return response
}
