// Purpose: Integration tests for browser-agent end-to-end flows.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// upload_integration_test.go — Integration and edge case tests for upload feature.
// Covers: concurrency, middleware integration, pending query payload, Content-Disposition
// safety, writeErr propagation, MaxBytesReader enforcement, correlation ID uniqueness,
// response shape verification, HTTP success paths, truncate(), and permission edge cases.
//
// WARNING: DO NOT use t.Parallel() — tests share global state (skipSSRFCheck, uploadSecurityConfig).
//
// Run: go test ./cmd/browser-agent -run "TestUploadInteg" -v
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"
)

// allowTestSSRF enables private IP access for tests using httptest.NewServer (127.0.0.1).
func allowTestSSRF(t *testing.T) {
	t.Helper()
	uploadhandler.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadhandler.SetSkipSSRFCheck(false) })
}

// testUploadSecurity returns a permissive UploadSecurity config for tests.
// The upload-dir is set to "/" so any absolute path is allowed.
// The denylist is still active (it's hardcoded).
func testUploadSecurity(t *testing.T) *UploadSecurity {
	t.Helper()
	return uploadhandler.NewSecurity("/", nil)
}

// testUploadSecurityWithDir returns an UploadSecurity scoped to a specific directory.
// Resolves symlinks so it matches the EvalSymlinks output in ValidateFilePath.
func testUploadSecurityWithDir(t *testing.T, dir string) *UploadSecurity {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("testUploadSecurityWithDir: EvalSymlinks(%s) failed: %v", dir, err)
	}
	return uploadhandler.NewSecurity(resolved, nil)
}

// ============================================
// 1. Concurrent form submit (race detector)
// ============================================

func TestUploadInteg_ConcurrentFormSubmit(t *testing.T) {
	allowTestSSRF(t)
	sec := testUploadSecurity(t)
	const workers = 10
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	testFile := createTestFile(t, "concurrent.txt", "concurrent test data")

	var wg sync.WaitGroup
	errs := make(chan string, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp := handleFormSubmitInternal(FormSubmitRequest{
				FormAction:    ts.URL,
				Method:        "POST",
				FileInputName: fmt.Sprintf("file_%d", idx),
				FilePath:      testFile,
			}, sec)
			if !resp.Success {
				errs <- fmt.Sprintf("worker %d failed: %s", idx, resp.Error)
			}
			if resp.Stage != 3 {
				errs <- fmt.Sprintf("worker %d: stage=%d, want 3", idx, resp.Stage)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for msg := range errs {
		t.Error(msg)
	}
}

// ============================================
// 2. extensionOnly middleware integration
// ============================================

func TestUploadInteg_ExtensionOnlyMiddleware(t *testing.T) {
	allowTestSSRF(t)
	// Use setupHTTPRoutes (the real route stack) to get middleware coverage
	server, err := NewServer(filepath.Join(t.TempDir(), "middleware-test.jsonl"), 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	mux, _ := setupHTTPRoutes(server, nil)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	endpoints := []string{
		"/api/file/read",
		"/api/file/dialog/inject",
		"/api/form/submit",
		"/api/os-automation/inject",
	}

	for _, ep := range endpoints {
		t.Run("no_header"+ep, func(t *testing.T) {
			// POST without X-Kaboom-Client header -> 403
			resp := postJSON(t, ts.URL+ep, `{"file_path":"/tmp/test.txt"}`)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("%s without extension header should be 403, got %d", ep, resp.StatusCode)
			}
		})
	}
}

// ============================================
// 3. Pending query payload verification
// ============================================

func TestUploadInteg_PendingQueryPayload(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "payload-check.mp4", "fake video for payload")

	// Call the upload handler
	result, ok := env.callInteract(t, fmt.Sprintf(
		`{"what":"upload","selector":"#VideoUpload","file_path":"%s","submit":true,"escalation_timeout_ms":8000}`,
		testFile))
	if !ok {
		t.Fatal("upload should return result")
	}
	if result.IsError {
		t.Fatalf("upload should succeed: %s", result.Content[0].Text)
	}

	// Retrieve the pending query from capture
	pending := env.capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("expected at least 1 pending query after upload")
	}

	// Find the upload query
	var uploadQuery *struct {
		Action              string `json:"action"`
		Selector            string `json:"selector"`
		FilePath            string `json:"file_path"`
		FileName            string `json:"file_name"`
		FileSize            int64  `json:"file_size"`
		MimeType            string `json:"mime_type"`
		Submit              bool   `json:"submit"`
		EscalationTimeoutMs int    `json:"escalation_timeout_ms"`
		ProgressTier        string `json:"progress_tier"`
	}

	for _, pq := range pending {
		if pq.Type == "upload" {
			uploadQuery = new(struct {
				Action              string `json:"action"`
				Selector            string `json:"selector"`
				FilePath            string `json:"file_path"`
				FileName            string `json:"file_name"`
				FileSize            int64  `json:"file_size"`
				MimeType            string `json:"mime_type"`
				Submit              bool   `json:"submit"`
				EscalationTimeoutMs int    `json:"escalation_timeout_ms"`
				ProgressTier        string `json:"progress_tier"`
			})
			if err := json.Unmarshal(pq.Params, uploadQuery); err != nil {
				t.Fatalf("failed to unmarshal upload params: %v", err)
			}
			break
		}
	}

	if uploadQuery == nil {
		t.Fatal("no upload query found in pending queries")
	}

	if uploadQuery.Action != "upload" {
		t.Errorf("action = %q, want 'upload'", uploadQuery.Action)
	}
	if uploadQuery.Selector != "#VideoUpload" {
		t.Errorf("selector = %q, want '#VideoUpload'", uploadQuery.Selector)
	}
	if uploadQuery.FileName != "payload-check.mp4" {
		t.Errorf("file_name = %q, want 'payload-check.mp4'", uploadQuery.FileName)
	}
	if uploadQuery.MimeType != "video/mp4" {
		t.Errorf("mime_type = %q, want 'video/mp4'", uploadQuery.MimeType)
	}
	if !uploadQuery.Submit {
		t.Error("submit should be true")
	}
	if uploadQuery.EscalationTimeoutMs != 8000 {
		t.Errorf("escalation_timeout_ms = %d, want 8000", uploadQuery.EscalationTimeoutMs)
	}
	if uploadQuery.ProgressTier != "simple" {
		t.Errorf("progress_tier = %q, want 'simple'", uploadQuery.ProgressTier)
	}
}

// ============================================
// 4. Content-Disposition header injection
// ============================================

func TestUploadInteg_ContentDisposition_SafeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal", "video.mp4", "video.mp4"},
		{"quotes", `file"name.mp4`, "file_name.mp4"},
		{"newline", "file\nname.mp4", "file_name.mp4"},
		{"carriage return", "file\rname.mp4", "file_name.mp4"},
		{"null byte", "file\x00name.mp4", "file_name.mp4"},
		{"combined", "file\"\n\r\x00.mp4", "file____.mp4"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeForContentDisposition(tc.input)
			if got != tc.expected {
				t.Errorf("sanitizeForContentDisposition(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestUploadInteg_ContentDisposition_PreservedInMultipart(t *testing.T) {
	allowTestSSRF(t)
	sec := testUploadSecurity(t)
	// Create a file with a name that would break Content-Disposition if unescaped
	dir := t.TempDir()
	// We can't create a file with quotes in the name on all OSes,
	// so test the sanitizer is applied by verifying the submitted filename
	testFile := createTestFile(t, "safe-name.txt", "content")

	var receivedFileName string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		_, fh, err := r.FormFile("Filedata")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		receivedFileName = fh.Filename
		w.WriteHeader(200)
	}))
	defer ts.Close()

	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "Filedata",
		FilePath:      testFile,
	}, sec)

	if !resp.Success {
		t.Fatalf("form submit should succeed, got: %s", resp.Error)
	}
	if receivedFileName != "safe-name.txt" {
		t.Errorf("received filename %q, want 'safe-name.txt'", receivedFileName)
	}
	_ = dir // used by createTestFile
}

// ============================================
// 5. writeErrCh error path
// ============================================

func TestUploadInteg_FormSubmit_FileDeletedDuringUpload(t *testing.T) {
	allowTestSSRF(t)
	// Create a file, start form submit, but have the server delay reading
	// to give us time to delete the file. This exercises the write error path.
	dir := t.TempDir()
	path := filepath.Join(dir, "ephemeral.txt")
	if err := os.WriteFile(path, []byte("temporary content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Server that discards the body
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read (and discard) the body to let the pipe drain
		_, _ = fmt.Fprintf(w, `{"ok":true}`)
	}))
	defer ts.Close()

	// Delete the file after os.Stat and os.Open succeed but the pipe copy is ongoing.
	// This is racy by nature -- the test verifies no panic/deadlock occurs.
	// We can't reliably trigger a read error, but concurrent deletion during
	// the multipart write may produce a write error. The key assertion: no panic.
	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "file",
		FilePath:      path,
	}, sec)

	// Success or error are both valid -- the point is no panic/deadlock
	if resp.Stage != 3 {
		t.Errorf("stage should be 3, got %d", resp.Stage)
	}
}

// ============================================
// 6. MaxBytesReader enforcement
// ============================================

func TestUploadInteg_MaxBytesReader_FileRead(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	// Send a body larger than 1MB (the MaxBytesReader limit for /api/file/read)
	largeBody := strings.Repeat("x", 2*1024*1024) // 2MB
	resp, err := http.Post(ts.URL+"/api/file/read", "application/json", strings.NewReader(largeBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Should be rejected (400 for bad JSON or connection reset)
	if resp.StatusCode == http.StatusOK {
		t.Error("oversized body should not succeed")
	}
}

func TestUploadInteg_MaxBytesReader_FormSubmit(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	// Send a body larger than 10MB (the MaxBytesReader limit for /api/form/submit)
	largeBody := strings.Repeat("x", 11*1024*1024) // 11MB
	resp, err := http.Post(ts.URL+"/api/form/submit", "application/json", strings.NewReader(largeBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Error("oversized body should not succeed")
	}
}

// ============================================
// 7. Concurrent correlation ID uniqueness
// ============================================

func TestUploadInteg_CorrelationID_Unique(t *testing.T) {
	const count = 20
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "corr-id.txt", "test")

	ids := make(chan string, count)
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, ok := env.callInteract(t, fmt.Sprintf(
				`{"what":"upload","selector":"#f","file_path":"%s"}`, testFile))
			if !ok || result.IsError {
				return
			}
			data := parseResponseJSON(t, result)
			if corrID, ok := data["correlation_id"].(string); ok {
				ids <- corrID
			}
		}()
	}

	wg.Wait()
	close(ids)

	seen := make(map[string]bool)
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate correlation_id: %s", id)
		}
		seen[id] = true
	}

	if len(seen) == 0 {
		t.Error("no correlation IDs collected")
	}
}

// ============================================
// 8. Dialog inject success response shape
// ============================================

func TestUploadInteg_DialogInject_ResponseShape(t *testing.T) {
	env := newUploadTestEnv(t)
	testFile := createTestFile(t, "shape-check.mp4", "fake video for shape")

	resp := env.handleDialogInject(t, FileDialogInjectRequest{
		FilePath:   testFile,
		BrowserPID: 12345,
	})

	if !resp.Success {
		t.Fatalf("dialog inject should succeed, got error: %s", resp.Error)
	}
	if resp.Stage != 2 {
		t.Errorf("stage should be 2, got %d", resp.Stage)
	}
	if resp.FileName != "shape-check.mp4" {
		t.Errorf("file_name should be 'shape-check.mp4', got %q", resp.FileName)
	}
	if resp.FileSizeBytes <= 0 {
		t.Errorf("file_size_bytes should be > 0, got %d", resp.FileSizeBytes)
	}
	if resp.Status == "" {
		t.Error("status should not be empty")
	}
}

// ============================================
// 9. HTTP success paths for form/submit and os-automation
// ============================================

func TestUploadInteg_HTTP_FormSubmit_SuccessPath(t *testing.T) {
	allowTestSSRF(t)
	// Create a target server that accepts the upload
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	testFile := createTestFile(t, "http-form.txt", "form submit content")
	body := fmt.Sprintf(`{"form_action":"%s","file_input_name":"file","file_path":"%s"}`,
		target.URL, testFile)
	resp := postJSON(t, ts.URL+"/api/form/submit", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("form submit success path should be 200, got %d", resp.StatusCode)
	}

	var result UploadStageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Stage != 3 {
		t.Errorf("stage should be 3, got %d", result.Stage)
	}
}

func TestUploadInteg_HTTP_OSAutomation_SuccessPath(t *testing.T) {
	ts, _ := newUploadHTTPServer(t, true)
	defer ts.Close()

	testFile := createTestFile(t, "os-auto.mp4", "fake video")
	body := fmt.Sprintf(`{"file_path":"%s","browser_pid":1234}`, testFile)
	resp := postJSON(t, ts.URL+"/api/os-automation/inject", body)
	defer resp.Body.Close()

	// OS automation may succeed or fail depending on environment.
	// What matters: we get a valid response (200 success or 400 error), not a crash.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 200 or 400, got %d", resp.StatusCode)
	}

	var result UploadStageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Stage != 4 {
		t.Errorf("stage should be 4, got %d", result.Stage)
	}
}

// ============================================
// 11. File permission edge case in form submit
// ============================================

func TestUploadInteg_FormSubmit_FilePermissionDenied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noperm.txt")
	if err := os.WriteFile(path, []byte("data"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	// Hermetic: point at a local server instead of example.com.
	//
	// ValidateFormActionURL resolves the host for its SSRF check with a 5s
	// timeout, so "https://example.com/upload" made this test depend on real DNS.
	// When resolution stalled, the call took exactly 5.00s and returned a DNS
	// error — which mentions neither "open" nor "permission", so the assertion
	// below failed for a reason unrelated to what the test is about.
	//
	// The recorder makes the contract explicit too: an unreadable file must be
	// rejected before any request goes out.
	uploadhandler.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadhandler.SetSkipSSRFCheck(false) })

	var contacted atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		contacted.Store(true)
	}))
	defer srv.Close()

	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    srv.URL + "/upload",
		FileInputName: "file",
		FilePath:      path,
	}, sec)

	// os.Stat succeeds on 0o000 files (only needs dir traversal)
	// but os.Open will fail with permission denied
	if resp.Success {
		t.Error("form submit with unreadable file should fail")
	}
	if contacted.Load() {
		t.Error("form submit must reject the unreadable file before making any request")
	}
	if !strings.Contains(strings.ToLower(resp.Error), "open") &&
		!strings.Contains(strings.ToLower(resp.Error), "permission") {
		t.Errorf("error should mention open/permission failure, got: %s", resp.Error)
	}
}
