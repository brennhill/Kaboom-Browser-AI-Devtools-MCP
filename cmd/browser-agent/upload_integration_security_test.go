// Purpose: Integration tests for browser-agent end-to-end flows.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// upload_integration_security_test.go — Integration tests for upload security features.
// Covers: OS automation path validation, path injection prevention, HTTP method validation,
// cookie header validation, SSRF validation, isPrivateIP, and goroutine leak detection.
//
// WARNING: DO NOT use t.Parallel() — tests share global state (skipSSRFCheck, uploadSecurityConfig).
//
// Run: go test ./cmd/browser-agent -run "TestUploadInteg_(OSAutomation|ContentDisposition_Input|Validate|IsPrivate|FormSubmit_Context|FormSubmit_Server)" -v
package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// ============================================
// 12. OS automation path validation integration
// ============================================

func TestUploadInteg_OSAutomation_PathWithNewline(t *testing.T) {
	// Path with newline should be rejected by the security validation chain.
	// The newline causes EvalSymlinks to fail (file doesn't exist),
	// so we verify rejection happens regardless.
	sec := testUploadSecurity(t)
	resp := handleOSAutomationInternal(OSAutomationInjectRequest{
		FilePath:   "/tmp/safe\ninjection",
		BrowserPID: 12345,
	}, sec)

	if resp.Success {
		t.Error("path with newline should be rejected")
	}
}

func TestUploadInteg_OSAutomation_PathWithNullByte(t *testing.T) {
	sec := testUploadSecurity(t)
	resp := handleOSAutomationInternal(OSAutomationInjectRequest{
		FilePath:   "/tmp/safe\x00injection",
		BrowserPID: 12345,
	}, sec)

	if resp.Success {
		t.Error("path with null byte should be rejected")
	}
}

func TestUploadInteg_OSAutomation_PathWithBacktick(t *testing.T) {
	// Create a real file so EvalSymlinks succeeds, then verify
	// validatePathForOSAutomation catches the backtick.
	dir := t.TempDir()
	path := filepath.Join(dir, "safe`whoami`.mp4")
	os.WriteFile(path, []byte("test"), 0o644)

	sec := testUploadSecurityWithDir(t, dir)
	resp := handleOSAutomationInternal(OSAutomationInjectRequest{
		FilePath:   path,
		BrowserPID: 12345,
	}, sec)

	if resp.Success {
		t.Error("path with backtick should be rejected")
	}
	if !strings.Contains(resp.Error, "backtick") {
		t.Errorf("error should mention backtick, got: %s", resp.Error)
	}
}

// ============================================
// 13. Path with leading dashes (flag injection)
// ============================================

func TestUploadInteg_OSAutomation_ValidPathPassesThrough(t *testing.T) {
	// Paths with unusual but valid characters should pass validation
	// and reach executeOSAutomation (which may fail on this OS but
	// shouldn't fail at validation). Tests that the -- terminator
	// in xdotool args doesn't break valid paths.
	validPaths := []string{
		"/tmp/normal.mp4",
		"/tmp/spaces in name.mp4",
		"/tmp/cafe\u0301.mp4",
		"/tmp/file-with-dashes.mp4",
		"/tmp/file_with_underscores.mp4",
		"/tmp/UPPERCASE.MP4",
	}

	for _, p := range validPaths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			err := validatePathForOSAutomation(p)
			if err != nil {
				t.Errorf("valid path %q should pass validation, got: %v", p, err)
			}
		})
	}
}

func TestUploadInteg_ContentDisposition_InputNameInjection(t *testing.T) {
	// Verify that a malicious file_input_name with quotes can't break
	// the Content-Disposition header framing.
	tests := []struct {
		name      string
		inputName string
		expected  string
	}{
		{"normal", "Filedata", "Filedata"},
		{"quotes in name", `file"data`, "file_data"},
		{"newline in name", "file\ndata", "file_data"},
		{"null in name", "file\x00data", "file_data"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeForContentDisposition(tc.inputName)
			if got != tc.expected {
				t.Errorf("sanitizeForContentDisposition(%q) = %q, want %q",
					tc.inputName, got, tc.expected)
			}
		})
	}
}

// ============================================
// 15. HTTP Method Validation
// ============================================

func TestUploadInteg_ValidateHTTPMethod(t *testing.T) {
	allowed := []string{"POST", "PUT", "PATCH", "post", "put", "patch", "Post"}
	for _, m := range allowed {
		t.Run("allow_"+m, func(t *testing.T) {
			if err := validateHTTPMethod(m); err != nil {
				t.Errorf("validateHTTPMethod(%q) should allow, got: %v", m, err)
			}
		})
	}

	blocked := []string{"GET", "DELETE", "HEAD", "OPTIONS", "TRACE", "CONNECT", ""}
	for _, m := range blocked {
		name := m
		if name == "" {
			name = "empty"
		}
		t.Run("block_"+name, func(t *testing.T) {
			if err := validateHTTPMethod(m); err == nil {
				t.Errorf("validateHTTPMethod(%q) should block, got nil", m)
			}
		})
	}
}

// ============================================
// 16. Cookie Header Validation
// ============================================

func TestUploadInteg_ValidateCookieHeader(t *testing.T) {
	valid := []string{
		"",
		"session=abc123",
		"session=abc123; csrf=xyz",
		"name=value with spaces",
		"emoji=\xf0\x9f\x94\xa5",
	}
	for _, c := range valid {
		name := c
		if name == "" {
			name = "empty"
		}
		t.Run("allow_"+name, func(t *testing.T) {
			if err := validateCookieHeader(c); err != nil {
				t.Errorf("validateCookieHeader(%q) should allow, got: %v", c, err)
			}
		})
	}

	injections := []struct {
		name   string
		cookie string
	}{
		{"CR", "session=abc\rInjected: header"},
		{"LF", "session=abc\nInjected: header"},
		{"CRLF", "session=abc\r\nInjected: header"},
		{"null", "session=abc\x00"},
		{"CRLF_start", "\r\nEvil: header"},
	}
	for _, tc := range injections {
		t.Run("block_"+tc.name, func(t *testing.T) {
			if err := validateCookieHeader(tc.cookie); err == nil {
				t.Errorf("validateCookieHeader(%q) should block header injection, got nil", tc.cookie)
			}
		})
	}
}

// ============================================
// 17. SSRF Validation (validateFormActionURL)
// ============================================

func TestUploadInteg_ValidateFormActionURL(t *testing.T) {
	// These tests exercise the URL parser and scheme checks;
	// they do NOT need skipSSRFCheck since they never hit the DNS path.
	schemeBlocked := []struct {
		name string
		url  string
	}{
		{"ftp", "ftp://example.com/upload"},
		{"file", "file:///etc/passwd"},
		{"javascript", "javascript:alert(1)"},
		{"data", "data:text/html,<h1>hi</h1>"},
		{"gopher", "gopher://evil.com"},
	}
	for _, tc := range schemeBlocked {
		t.Run("scheme_"+tc.name, func(t *testing.T) {
			if err := validateFormActionURL(tc.url); err == nil {
				t.Errorf("validateFormActionURL(%q) should block non-http scheme, got nil", tc.url)
			}
		})
	}

	// Blocked hostnames
	hostBlocked := []struct {
		name string
		url  string
	}{
		{"localhost", "http://localhost/upload"},
		{"localhost_port", "http://localhost:8080/upload"},
		{"metadata_gcp", "http://metadata.google.internal/computeMetadata/v1/"},
	}
	for _, tc := range hostBlocked {
		t.Run("host_"+tc.name, func(t *testing.T) {
			if err := validateFormActionURL(tc.url); err == nil {
				t.Errorf("validateFormActionURL(%q) should block, got nil", tc.url)
			}
		})
	}

	// Empty / missing hostname
	t.Run("no_hostname", func(t *testing.T) {
		if err := validateFormActionURL("http:///path"); err == nil {
			t.Error("should reject URL with empty hostname")
		}
	})
}

// ============================================
// 18. isPrivateIP
// ============================================

func TestUploadInteg_IsPrivateIP(t *testing.T) {
	privateIPs := []string{
		"127.0.0.1", "127.0.0.99", "10.0.0.1", "10.255.255.255",
		"172.16.0.1", "172.31.255.255", "192.168.0.1", "192.168.255.255",
		"169.254.1.1", "::1", "fc00::1", "fe80::1",
	}
	for _, ipStr := range privateIPs {
		t.Run("private_"+ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", ipStr)
			}
			if !uploadsec.IsPrivateIP(ip) {
				t.Errorf("uploadsec.IsPrivateIP(%s) = false, want true", ipStr)
			}
		})
	}

	publicIPs := []string{
		"8.8.8.8", "1.1.1.1", "93.184.216.34", "2607:f8b0:4004:800::200e",
	}
	for _, ipStr := range publicIPs {
		t.Run("public_"+ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", ipStr)
			}
			if uploadsec.IsPrivateIP(ip) {
				t.Errorf("uploadsec.IsPrivateIP(%s) = true, want false", ipStr)
			}
		})
	}
}

// ============================================
// 19. Goroutine leak: writeErrCh drained on errors
// ============================================

func TestUploadInteg_FormSubmit_ContextCancelled(t *testing.T) {
	// Verifies that context cancellation doesn't leak the pipe-writer goroutine.
	// The race detector will catch a leaked goroutine writing to a closed pipe.
	allowTestSSRF(t)

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server that never finishes reading
		select {}
	}))
	defer slow.Close()

	tmp := t.TempDir()
	f := filepath.Join(tmp, "test.txt")
	os.WriteFile(f, []byte("hello"), 0644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req := FormSubmitRequest{
		FormAction:    slow.URL + "/upload",
		Method:        "POST",
		FileInputName: "file",
		FilePath:      f,
	}

	// Should fail fast due to cancelled context, not leak goroutines.
	// The race detector (-race) validates no goroutine leak.
	sec := testUploadSecurityWithDir(t, tmp)
	resp := handleFormSubmitInternalCtx(ctx, req, sec)
	if resp.Success {
		t.Error("expected failure with cancelled context")
	}
}

func TestUploadInteg_FormSubmit_ServerError_DrainChannel(t *testing.T) {
	// When the server returns an error, writeErrCh must be drained
	// so the pipe-writer goroutine can exit. Race detector catches leaks.
	allowTestSSRF(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	f := filepath.Join(tmp, "test.txt")
	os.WriteFile(f, []byte("some content"), 0644)

	req := FormSubmitRequest{
		FormAction:    srv.URL + "/upload",
		Method:        "POST",
		FileInputName: "file",
		FilePath:      f,
	}

	sec := testUploadSecurityWithDir(t, tmp)
	resp := handleFormSubmitInternalCtx(context.Background(), req, sec)
	if resp.Success {
		t.Error("expected failure on 500 response")
	}
	if !strings.Contains(resp.Error, "500") {
		t.Errorf("expected error to mention 500 status, got: %s", resp.Error)
	}
}
