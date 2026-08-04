// Purpose: Tests for upload HTTP handler request processing.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// upload_handlers_test.go — HTTP endpoint and happy-path tests for file upload.
// Tests the HTTP layer (status codes, content types, disabled/enabled gating)
// and verifies MCP handler response contracts that the unit tests miss.
//
// WARNING: DO NOT use t.Parallel() — tests share global state (skipSSRFCheck, uploadSecurityConfig).
//
// Run: go test ./cmd/browser-agent -run "TestUploadHandler" -v
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agenthttp "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
	uploadapi "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/httpapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// ============================================
// MCP handler happy-path response contract
// ============================================

func TestUploadHandler_MCPHappyPath_ResponseContract(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "contract-test.txt", "contract test content")

	result, ok := env.callInteract(t, fmt.Sprintf(
		`{"what":"upload","selector":"#Filedata","file_path":"%s"}`, testFile))
	if !ok {
		t.Fatal("upload should return result")
	}

	if result.IsError {
		t.Fatalf("upload happy path should not be an error: %s", result.Content[0].Text)
	}

	data := parseResponseJSON(t, result)

	assertObjectShape(t, "upload happy path", data, []fieldSpec{
		required("status", "string"),
		required("correlation_id", "string"),
		required("file_name", "string"),
		required("file_size", "number"),
		required("mime_type", "string"),
		required("progress_tier", "string"),
		required("message", "string"),
	})

	// Verify correlation_id prefix
	corrID, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corrID, "upload_") {
		t.Errorf("correlation_id should start with 'upload_', got %q", corrID)
	}

	// Verify file metadata matches actual file
	if fn, _ := data["file_name"].(string); fn != "contract-test.txt" {
		t.Errorf("file_name should be 'contract-test.txt', got %q", fn)
	}
	if fs, _ := data["file_size"].(float64); int64(fs) != 21 {
		t.Errorf("file_size should be 21, got %v", data["file_size"])
	}
	if mt, _ := data["mime_type"].(string); mt != "text/plain" {
		t.Errorf("mime_type should be 'text/plain', got %q", mt)
	}
}

// ============================================
// Base64 content decode verification
// ============================================

func TestUploadHandler_FileRead_Base64Roundtrip(t *testing.T) {
	env := newUploadTestEnv(t)
	// Use bytes that aren't valid UTF-8 to ensure binary fidelity
	content := []byte{0x00, 0x01, 0xFF, 0xFE, 0x89, 0x50, 0x4E, 0x47}
	dir := t.TempDir()
	path := filepath.Join(dir, "binary.bin")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	resp := env.handleFileRead(t, upload.FileReadRequest{FilePath: path})
	if !resp.Success {
		t.Fatalf("file read failed: %s", resp.Error)
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.DataBase64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if len(decoded) != len(content) {
		t.Fatalf("decoded length %d != original %d", len(decoded), len(content))
	}
	for i := range content {
		if decoded[i] != content[i] {
			t.Errorf("byte %d: decoded 0x%02x != original 0x%02x", i, decoded[i], content[i])
		}
	}
}

// ============================================
// Stage 3: Form submit with httptest server
// ============================================

func TestUploadHandler_FormSubmit_WithTestServer(t *testing.T) {
	uploadsec.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadsec.SetSkipSSRFCheck(false) })
	testFile := createTestFile(t, "upload.txt", "file content for form submit")

	var (
		receivedFileName  string
		receivedFileData  string
		receivedCSRF      string
		receivedCustom    string
		receivedCookie    string
		receivedInputName string
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookie = r.Header.Get("Cookie")

		ct := r.Header.Get("Content-Type")
		_, params, err := mime.ParseMediaType(ct)
		if err != nil {
			http.Error(w, "bad content type", 400)
			return
		}

		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			data, _ := io.ReadAll(part)
			name := part.FormName()
			switch {
			case part.FileName() != "":
				receivedFileName = part.FileName()
				receivedFileData = string(data)
				receivedInputName = name
			case name == "csrf_token":
				receivedCSRF = string(data)
			default:
				receivedCustom = string(data)
			}
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	sec := testUploadSecurity(t)
	resp := upload.HandleFormSubmit(upload.FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "Filedata",
		FilePath:      testFile,
		CSRFToken:     "tok123",
		Fields:        map[string]string{"title": "My Upload"},
		Cookies:       "session=abc",
	}, sec)

	if !resp.Success {
		t.Fatalf("form submit should succeed, got error: %s", resp.Error)
	}
	if resp.Stage != 3 {
		t.Errorf("expected stage 3, got %d", resp.Stage)
	}
	if resp.DurationMs < 0 {
		t.Error("duration_ms should be >= 0")
	}

	// Verify server received correct data
	if receivedFileName != "upload.txt" {
		t.Errorf("server received filename %q, want 'upload.txt'", receivedFileName)
	}
	if receivedFileData != "file content for form submit" {
		t.Errorf("server received file data %q, want original content", receivedFileData)
	}
	if receivedInputName != "Filedata" {
		t.Errorf("server received input name %q, want 'Filedata'", receivedInputName)
	}
	if receivedCSRF != "tok123" {
		t.Errorf("server received CSRF %q, want 'tok123'", receivedCSRF)
	}
	if receivedCustom != "My Upload" {
		t.Errorf("server received custom field %q, want 'My Upload'", receivedCustom)
	}
	if receivedCookie != "session=abc" {
		t.Errorf("server received cookie %q, want 'session=abc'", receivedCookie)
	}
}

// ============================================
// HTTP endpoint tests
// ============================================

// newUploadHTTPServer creates a test HTTP server with the 4 upload routes registered.
// osAutomationEnabled controls Stage 4 gating; Stages 1-3 are always available.
func newUploadHTTPServer(t *testing.T, osAutomationEnabled bool) *httptest.Server {
	t.Helper()
	// Allow private IPs in tests (httptest.NewServer uses 127.0.0.1)
	uploadsec.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadsec.SetSkipSSRFCheck(false) })
	// Set permissive upload security for HTTP handler tests.
	security := uploadsec.NewSecurity("/", nil)

	mux := http.NewServeMux()
	handlers := uploadapi.NewHandlers(security, osAutomationEnabled, agenthttp.JSON)
	mux.HandleFunc("/api/file/read", handlers.HandleFileRead)
	mux.HandleFunc("/api/file/dialog/inject", handlers.HandleFileDialogInject)
	mux.HandleFunc("/api/form/submit", handlers.HandleFormSubmit)
	mux.HandleFunc("/api/os-automation/inject", handlers.HandleOSAutomation)

	return httptest.NewServer(mux)
}

// --- /api/file/read ---

func TestUploadHandler_HTTP_FileRead_MethodNotAllowed(t *testing.T) {
	ts := newUploadHTTPServer(t, true)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/file/read")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/file/read should be 405, got %d", resp.StatusCode)
	}
}

func TestUploadHandler_HTTP_FileRead_Success(t *testing.T) {
	ts := newUploadHTTPServer(t, true)
	defer ts.Close()

	testFile := createTestFile(t, "http-read.txt", "http test content")
	resp := postJSON(t, ts.URL+"/api/file/read",
		fmt.Sprintf(`{"file_path":"%s"}`, testFile))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/file/read with valid file should be 200, got %d", resp.StatusCode)
	}

	var body upload.FileReadResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if !body.Success {
		t.Errorf("expected success, got error: %s", body.Error)
	}
	if body.FileName != "http-read.txt" {
		t.Errorf("file_name = %q, want 'http-read.txt'", body.FileName)
	}
	if body.FileSize != 17 {
		t.Errorf("file_size = %d, want 17", body.FileSize)
	}
	if body.DataBase64 == "" {
		t.Error("data_base64 should not be empty for small file")
	}
}

func TestUploadHandler_HTTP_FileRead_NotFound(t *testing.T) {
	ts := newUploadHTTPServer(t, true)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/file/read", `{"file_path":"/nonexistent/file.txt"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("file not found should be 404, got %d", resp.StatusCode)
	}
}

func TestUploadHandler_HTTP_FileRead_InvalidJSON(t *testing.T) {
	ts := newUploadHTTPServer(t, true)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/file/read", `{not valid json}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid JSON should be 400, got %d", resp.StatusCode)
	}
}

// --- /api/file/dialog/inject ---

func TestUploadHandler_HTTP_DialogInject_Success(t *testing.T) {
	ts := newUploadHTTPServer(t, true)
	defer ts.Close()

	testFile := createTestFile(t, "dialog.mp4", "fake video")
	resp := postJSON(t, ts.URL+"/api/file/dialog/inject",
		fmt.Sprintf(`{"file_path":"%s","browser_pid":1234}`, testFile))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("valid dialog inject should be 200, got %d", resp.StatusCode)
	}
}

// --- /api/form/submit ---

func TestUploadHandler_HTTP_FormSubmit_MissingRequired(t *testing.T) {
	ts := newUploadHTTPServer(t, true)
	defer ts.Close()

	// Missing form_action
	resp := postJSON(t, ts.URL+"/api/form/submit",
		`{"file_input_name":"f","file_path":"/tmp/x"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing form_action should be 400, got %d", resp.StatusCode)
	}
}

// --- /api/os-automation/inject ---

func TestUploadHandler_HTTP_OSAutomation_Disabled(t *testing.T) {
	ts := newUploadHTTPServer(t, false)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/os-automation/inject",
		`{"file_path":"/tmp/test.mp4","browser_pid":1234}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("OS automation when disabled should be 403, got %d", resp.StatusCode)
	}

	var body upload.StageResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if !strings.Contains(body.Error, "enable-os-upload-automation") {
		t.Errorf("error should mention --enable-os-upload-automation, got: %s", body.Error)
	}
}

// ============================================
// Helpers
// ============================================

// postJSON sends a POST request with JSON body to the given URL.
func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	return resp
}
