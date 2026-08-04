// response_proc_test.go — Tests response, process, async, text, time, media, and URL helpers.
package util

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync"
	"testing"
	"time"
)

func TestJSONResponse(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	JSONResponse(rr, http.StatusCreated, map[string]any{"ok": true, "count": 2})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["ok"] != true {
		t.Fatalf("body ok = %v, want true", body["ok"])
	}
}

func TestJSONResponseEncodeErrorDoesNotPanic(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	JSONResponse(rr, http.StatusOK, map[string]any{
		"bad": make(chan int), // unsupported by encoding/json
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestSetDetachedProcess(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("echo", "hi")
	SetDetachedProcess(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatalf("SysProcAttr = %+v, expected Setsid=true", cmd.SysProcAttr)
	}
}

func TestSafeGoNormalExecution(t *testing.T) {
	var done sync.WaitGroup
	done.Add(1)
	executed := false

	SafeGo(func() {
		executed = true
		done.Done()
	})

	done.Wait()
	if !executed {
		t.Error("SafeGo did not execute the function")
	}
}

func TestSafeGoPanicRecovery(t *testing.T) {
	recovered := make(chan bool, 1)

	SafeGo(func() {
		defer func() { recovered <- true }()
		panic("test panic")
	})

	select {
	case <-recovered:
		// Goroutine survived the panic — success
	case <-time.After(2 * time.Second):
		t.Fatal("SafeGo goroutine did not recover from panic within timeout")
	}
}

func TestSafeGoNilPanicRecovery(t *testing.T) {
	recovered := make(chan bool, 1)

	SafeGo(func() {
		defer func() { recovered <- true }()
		panic(nil)
	})

	select {
	case <-recovered:
		// Goroutine survived nil panic — success
	case <-time.After(2 * time.Second):
		t.Fatal("SafeGo goroutine did not recover from nil panic within timeout")
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"ab", 3, "ab"},
		{"abcd", 3, "..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 4, "abcd"},
		{"abcde", 4, "a..."},
		{"abcdef", 0, ""},
		{"abcdef", 1, "."},
		{"abcdef", 2, ".."},
	}
	for _, tt := range tests {
		got := Truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestParseTimestamp_RFC3339(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("2024-01-15T10:30:00Z")
	want := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ParseTimestamp(RFC3339) = %v, want %v", got, want)
	}
}

func TestParseTimestamp_RFC3339Nano(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("2024-01-15T10:30:00.123456789Z")
	want := time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC)
	if !got.Equal(want) {
		t.Errorf("ParseTimestamp(RFC3339Nano) = %v, want %v", got, want)
	}
}

func TestParseTimestamp_RFC3339WithOffset(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("2024-01-15T10:30:00+05:00")
	if got.IsZero() {
		t.Error("ParseTimestamp(RFC3339 with offset) returned zero time")
	}
}

func TestParseTimestamp_EmptyString(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("")
	if !got.IsZero() {
		t.Errorf("ParseTimestamp(empty) = %v, want zero time", got)
	}
}

func TestParseTimestamp_InvalidString(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("not-a-timestamp")
	if !got.IsZero() {
		t.Errorf("ParseTimestamp(invalid) = %v, want zero time", got)
	}
}

func TestParseTimestamp_RFC3339NanoMilliseconds(t *testing.T) {
	t.Parallel()
	got := ParseTimestamp("2024-06-15T08:00:00.500Z")
	if got.IsZero() {
		t.Error("ParseTimestamp(RFC3339Nano millis) returned zero time")
	}
	if got.Nanosecond() != 500000000 {
		t.Errorf("ParseTimestamp nanosecond = %d, want 500000000", got.Nanosecond())
	}
}

// ============================================
// ExtractURLPath Tests
// ============================================

func TestExtractURLPath_FullURL(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("https://example.com/api/v1/users?page=2&limit=10")
	if got != "/api/v1/users" {
		t.Errorf("ExtractURLPath(full URL with query) = %q, want /api/v1/users", got)
	}
}

func TestExtractURLPath_URLWithFragment(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("https://example.com/docs#section-3")
	if got != "/docs" {
		t.Errorf("ExtractURLPath(URL with fragment) = %q, want /docs", got)
	}
}

func TestExtractURLPath_RootPath(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("https://example.com/")
	if got != "/" {
		t.Errorf("ExtractURLPath(root path) = %q, want /", got)
	}
}

func TestExtractURLPath_NoPath(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("https://example.com")
	if got != "/" {
		t.Errorf("ExtractURLPath(no path) = %q, want /", got)
	}
}

func TestExtractURLPath_EmptyString(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("")
	if got != "/" {
		t.Errorf("ExtractURLPath(empty) = %q, want /", got)
	}
}

func TestExtractURLPath_JustPath(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("/api/health")
	if got != "/api/health" {
		t.Errorf("ExtractURLPath(just path) = %q, want /api/health", got)
	}
}

func TestExtractURLPath_UnparseableURL(t *testing.T) {
	t.Parallel()
	input := string([]byte{0x7f})
	got := ExtractURLPath(input)
	if got != input {
		t.Errorf("ExtractURLPath(unparseable) = %q, want original input %q", got, input)
	}
}

// ============================================
// ExtractOrigin Tests
// ============================================

func TestExtractOrigin_StandardHTTPS(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("https://example.com/path?query=1")
	if got != "https://example.com" {
		t.Errorf("ExtractOrigin(standard HTTPS) = %q, want https://example.com", got)
	}
}

func TestExtractOrigin_WithPort(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("http://localhost:8080/api")
	if got != "http://localhost:8080" {
		t.Errorf("ExtractOrigin(with port) = %q, want http://localhost:8080", got)
	}
}

func TestExtractOrigin_DataURL(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("data:text/html,<h1>Hello</h1>")
	if got != "" {
		t.Errorf("ExtractOrigin(data:) = %q, want empty", got)
	}
}

func TestExtractOrigin_BlobURL(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("blob:https://example.com/uuid-here")
	if got != "https://example.com" {
		t.Errorf("ExtractOrigin(blob:) = %q, want https://example.com", got)
	}
}

func TestExtractOrigin_NoScheme(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("example.com/path")
	if got != "" {
		t.Errorf("ExtractOrigin(no scheme) = %q, want empty", got)
	}
}

func TestExtractOrigin_NoHost(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("file:///path/to/file")
	if got != "" {
		t.Errorf("ExtractOrigin(file:///) = %q, want empty", got)
	}
}

func TestExtractOrigin_EmptyString(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("")
	if got != "" {
		t.Errorf("ExtractOrigin(empty) = %q, want empty", got)
	}
}

func TestExtractOrigin_MalformedURL(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("://invalid")
	if got != "" {
		t.Errorf("ExtractOrigin(malformed) = %q, want empty", got)
	}
}
