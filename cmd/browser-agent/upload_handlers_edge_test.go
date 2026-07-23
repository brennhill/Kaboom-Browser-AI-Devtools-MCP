// Purpose: Edge-case tests for browser-agent boundary conditions.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// upload_handlers_edge_test.go — Security, sanitization, MIME, and edge-case tests for file upload.
//
// WARNING: DO NOT use t.Parallel() — tests share global state (skipSSRFCheck, uploadSecurityConfig).
//
// Run: go test ./cmd/browser-agent -run "TestUploadHandler" -v
package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"
)

// ============================================
// Directory path rejection
// ============================================

func TestUploadHandler_FileRead_DirectoryRejected(t *testing.T) {
	env := newUploadTestEnv(t)
	dir := t.TempDir()

	resp := env.handleFileRead(t, FileReadRequest{FilePath: dir})

	if resp.Success {
		t.Error("reading a directory should fail")
	}
	if !strings.Contains(strings.ToLower(resp.Error), "directory") {
		t.Errorf("error should mention directory, got: %s", resp.Error)
	}
}

// ============================================
// Script injection prevention
// ============================================

func TestUploadHandler_AppleScriptSanitization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{"normal path", "/Users/test/file.mp4", "/Users/test/file.mp4", ""},
		{"double quotes", `/tmp/file"name.mp4`, `\"`, ""},
		{"backslash", `/tmp/file\name.mp4`, `\\`, ""},
		// Injection: quotes are escaped so AppleScript sees literal text, not a command break.
		// The unescaped input would break out of the keystroke string in AppleScript.
		// After sanitization, all " become \" so the string stays intact.
		{"injection attempt", `/tmp/"; do shell script "rm -rf /"`, `\"`, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeForAppleScript(tc.input)
			if tc.contains != "" && !strings.Contains(result, tc.contains) {
				t.Errorf("expected result to contain %q, got %q", tc.contains, result)
			}
			// The injection payload should be escaped, not executable
			if tc.excludes != "" && strings.Contains(result, tc.excludes) {
				t.Errorf("expected result NOT to contain %q, got %q", tc.excludes, result)
			}
		})
	}
}

func TestUploadHandler_SendKeysSanitization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal path", `C:\Users\test\file.mp4`, `C:\Users\test\file.mp4`},
		{"plus sign", `C:\test+file.mp4`, `C:\test{+}file.mp4`},
		{"percent sign", `C:\test%file.mp4`, `C:\test{%}file.mp4`},
		{"caret", `C:\test^file.mp4`, `C:\test{^}file.mp4`},
		{"tilde", `C:\test~file.mp4`, `C:\test{~}file.mp4`},
		{"parens", `C:\test(1).mp4`, `C:\test{(}1{)}.mp4`},
		{"braces", `C:\test{1}.mp4`, `C:\test{{}1{}}.mp4`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeForSendKeys(tc.input)
			if result != tc.expected {
				t.Errorf("sanitizeForSendKeys(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestUploadHandler_PathValidation_RejectsMetachars(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"normal path", "/Users/test/file.mp4", false},
		{"null byte", "/tmp/file\x00.mp4", true},
		{"newline", "/tmp/file\n.mp4", true},
		{"carriage return", "/tmp/file\r.mp4", true},
		{"backtick", "/tmp/file`.mp4", true},
		{"spaces ok", "/tmp/my file.mp4", false},
		{"unicode ok", "/tmp/café.mp4", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePathForOSAutomation(tc.path)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for path %q, got nil", tc.path)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for path %q: %v", tc.path, err)
			}
		})
	}
}

// ============================================
// Windows double-escape: SendKeys + PowerShell
// ============================================

func TestUploadHandler_WindowsDoubleEscape(t *testing.T) {
	// Verifies the two-layer escaping applied in executeWindowsAutomation:
	// Layer 1: sanitizeForSendKeys escapes SendKeys metacharacters (+^%~(){})
	// Layer 2: strings.ReplaceAll escapes " → `" for PowerShell string literals
	tests := []struct {
		name     string
		input    string
		expected string // after both layers
	}{
		{
			"normal path",
			`C:\Users\test\file.mp4`,
			`C:\Users\test\file.mp4`,
		},
		{
			"path with plus and quotes",
			`C:\test+dir\file"name.mp4`,
			// + → {+} (SendKeys), " → `" (PowerShell)
			"C:\\test{+}dir\\file`\"name.mp4",
		},
		{
			"all SendKeys specials no quotes",
			`C:\a+b^c%d~e(f)g{h}i.mp4`,
			`C:\a{+}b{^}c{%}d{~}e{(}f{)}g{{}h{}}i.mp4`,
		},
		{
			"quotes only",
			`C:\Users\"quoted".mp4`,
			// " → `" (PowerShell)
			"C:\\Users\\`\"quoted`\".mp4",
		},
		{
			"SendKeys specials and quotes combined",
			`C:\test+dir\"file(1)".mp4`,
			// + → {+}, " → `", ( → {(}, ) → {)}
			"C:\\test{+}dir\\`\"file{(}1{)}`\".mp4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Layer 1: SendKeys escaping
			sendKeysEscaped := sanitizeForSendKeys(tc.input)
			// Layer 2: PowerShell quote escaping
			psEscaped := strings.ReplaceAll(sendKeysEscaped, `"`, "`\"")

			if psEscaped != tc.expected {
				t.Errorf("double escape(%q):\n  got:  %q\n  want: %q", tc.input, psEscaped, tc.expected)
			}

			// Invariant: no unescaped " should remain (all " become `")
			if strings.Contains(psEscaped, `"`) && !strings.Contains(psEscaped, "`\"") {
				t.Errorf("found unescaped quote in result: %q", psEscaped)
			}
		})
	}
}

// ============================================
// maxBase64FileSize threshold
// ============================================

func TestUploadHandler_FileRead_LargeFileSkipsBase64(t *testing.T) {
	// Create a sparse file that appears large but doesn't use disk space.
	// We only need the file to be > maxBase64FileSize for the stat check.
	dir := t.TempDir()
	path := filepath.Join(dir, "large.bin")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Seek to just past the threshold and write 1 byte to make the file "large"
	if _, err := f.Seek(maxBase64FileSize+1, 0); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{0x42}); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	resp := handleFileReadInternal(FileReadRequest{FilePath: path}, testUploadSecurity(t), false)
	if !resp.Success {
		t.Fatalf("file read should succeed for large file, got error: %s", resp.Error)
	}
	if resp.DataBase64 != "" {
		t.Error("file > maxBase64FileSize should NOT include base64 data")
	}
	if resp.FileSize <= maxBase64FileSize {
		t.Errorf("file size should be > %d, got %d", maxBase64FileSize, resp.FileSize)
	}
	if resp.FileName != "large.bin" {
		t.Errorf("file_name should be 'large.bin', got %q", resp.FileName)
	}
}

func TestUploadHandler_FileRead_ExactThresholdIncludesBase64(t *testing.T) {
	// File exactly at the threshold (<=) should include base64
	dir := t.TempDir()
	path := filepath.Join(dir, "exact.bin")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Seek to threshold and write 1 byte (total size = maxBase64FileSize)
	if _, err := f.Seek(maxBase64FileSize-1, 0); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{0x42}); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	resp := handleFileReadInternal(FileReadRequest{FilePath: path}, testUploadSecurity(t), false)
	if !resp.Success {
		t.Fatalf("file read should succeed, got error: %s", resp.Error)
	}
	// File exactly at 100MB should include base64 (<=)
	if resp.DataBase64 == "" {
		t.Error("file at exactly maxBase64FileSize should include base64 data")
	}
}

// ============================================
// MIME type edge cases
// ============================================

func TestUploadHandler_MimeType_CaseInsensitive(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"FILE.MP4", "video/mp4"},
		{"Image.JPG", "image/jpeg"},
		{"doc.PDF", "application/pdf"},
		{"DATA.JSON", "application/json"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := detectMimeType(tc.filename)
			if got != tc.expected {
				t.Errorf("detectMimeType(%q) = %q, want %q", tc.filename, got, tc.expected)
			}
		})
	}
}

func TestUploadHandler_MimeType_NoExtension(t *testing.T) {
	got := detectMimeType("Makefile")
	if got != "application/octet-stream" {
		t.Errorf("file without extension should be octet-stream, got %q", got)
	}
}

func TestUploadHandler_MimeType_DotFile(t *testing.T) {
	got := detectMimeType(".gitignore")
	if got != "application/octet-stream" {
		t.Errorf(".gitignore should be octet-stream, got %q", got)
	}
}

func TestUploadHandler_MimeType_DoubleExtension(t *testing.T) {
	got := detectMimeType("archive.tar.gz")
	if got != "application/gzip" {
		t.Errorf("archive.tar.gz should detect .gz, got %q", got)
	}
}

// ============================================
// Progress tier boundary at 0
// ============================================

func TestUploadHandler_ProgressTier_ZeroBytes(t *testing.T) {
	tier := getProgressTier(0)
	if tier != ProgressTierSimple {
		t.Errorf("0 bytes should use simple tier, got %s", tier)
	}
}

// ============================================
// Relative path rejection in form submit & dialog
// ============================================

func TestUploadHandler_FormSubmit_RelativePathRejected(t *testing.T) {
	// Local server, not example.com: ValidateFormActionURL resolves the host for
	// its SSRF check with a 5s timeout, so an external URL made this test depend
	// on real DNS. The recorder proves the relative path is rejected before any
	// request rather than relying on the test never reaching the network.
	uploadhandler.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadhandler.SetSkipSSRFCheck(false) })

	var contacted atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		contacted.Store(true)
	}))
	defer srv.Close()

	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    srv.URL + "/upload",
		FileInputName: "file",
		FilePath:      "../../../etc/passwd",
	}, testUploadSecurity(t))

	if resp.Success {
		t.Error("form submit with relative path should fail")
	}
	if contacted.Load() {
		t.Error("form submit must reject the relative path before making any request")
	}
	if !strings.Contains(resp.Error, "absolute path") {
		t.Errorf("error should mention absolute path, got: %s", resp.Error)
	}
}

func TestUploadHandler_DialogInject_RelativePathRejected(t *testing.T) {
	resp := handleDialogInjectInternal(FileDialogInjectRequest{
		FilePath:   "../../../etc/passwd",
		BrowserPID: 1234,
	}, testUploadSecurity(t))

	if resp.Success {
		t.Error("dialog inject with relative path should fail")
	}
	if !strings.Contains(resp.Error, "absolute path") {
		t.Errorf("error should mention absolute path, got: %s", resp.Error)
	}
}

func TestUploadHandler_FileRead_RelativePathRejected(t *testing.T) {
	resp := handleFileReadInternal(FileReadRequest{
		FilePath: "../../../etc/passwd",
	}, testUploadSecurity(t), false)

	if resp.Success {
		t.Error("file read with relative path should fail")
	}
	if !strings.Contains(resp.Error, "absolute path") {
		t.Errorf("error should mention absolute path, got: %s", resp.Error)
	}
}

// ============================================
// Form submit HTTP error status codes
// ============================================

func TestUploadHandler_FormSubmit_HTTP401Response(t *testing.T) {
	uploadhandler.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadhandler.SetSkipSSRFCheck(false) })
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer ts.Close()

	testFile := createTestFile(t, "auth.txt", "test content")
	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "file",
		FilePath:      testFile,
	}, sec)

	if resp.Success {
		t.Error("401 response should not be success")
	}
	if !strings.Contains(resp.Error, "401") {
		t.Errorf("error should mention 401, got: %s", resp.Error)
	}
}

func TestUploadHandler_FormSubmit_HTTP403Response(t *testing.T) {
	uploadhandler.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadhandler.SetSkipSSRFCheck(false) })
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer ts.Close()

	testFile := createTestFile(t, "csrf.txt", "test content")
	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "file",
		FilePath:      testFile,
	}, sec)

	if resp.Success {
		t.Error("403 response should not be success")
	}
	if !strings.Contains(resp.Error, "403") {
		t.Errorf("error should mention 403, got: %s", resp.Error)
	}
}

func TestUploadHandler_FormSubmit_HTTP422Response(t *testing.T) {
	uploadhandler.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadhandler.SetSkipSSRFCheck(false) })
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
	}))
	defer ts.Close()

	testFile := createTestFile(t, "validation.txt", "test content")
	sec := testUploadSecurity(t)
	resp := handleFormSubmitInternal(FormSubmitRequest{
		FormAction:    ts.URL,
		Method:        "POST",
		FileInputName: "file",
		FilePath:      testFile,
	}, sec)

	if resp.Success {
		t.Error("422 response should not be success")
	}
	if !strings.Contains(resp.Error, "422") {
		t.Errorf("error should mention 422, got: %s", resp.Error)
	}
}
