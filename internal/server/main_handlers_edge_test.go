// Purpose: Edge-case tests for HTTP server handlers boundary conditions.
// Docs: docs/features/feature/backend-log-streaming/index.md

package server

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestNewServerLoadsEntriesAndBoundsFromDisk(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "logs.jsonl")
	content := strings.Join([]string{
		"",
		`{"seq":1,"msg":"one"}`,
		`{`,
		`{"seq":2,"msg":"two"}`,
		`{"seq":3,"msg":"three"}`,
		"",
	}, "\n")
	if err := os.WriteFile(logFile, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	s, err := NewServer(logFile, 2)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	entries := s.getEntries()
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0]["seq"] != float64(2) || entries[1]["seq"] != float64(3) {
		t.Fatalf("bounded entries unexpected: %#v", entries)
	}
}

func TestNewServerPathErrors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// MkdirAll should fail when a path component is an existing file.
	blocker := filepath.Join(root, "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocker) error = %v", err)
	}
	if _, err := NewServer(filepath.Join(blocker, "logs.jsonl"), 5); err == nil {
		t.Fatal("NewServer() expected mkdir error, got nil")
	}

	// loadEntries should fail when the log file path is a directory.
	logPathAsDir := filepath.Join(root, "log-dir")
	if err := os.MkdirAll(logPathAsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(logPathAsDir) error = %v", err)
	}
	if _, err := NewServer(logPathAsDir, 5); err == nil {
		t.Fatal("NewServer() expected loadEntries error for directory log path, got nil")
	}
}

func TestLoadEntriesScannerTokenTooLong(t *testing.T) {
	t.Parallel()

	logFile := filepath.Join(t.TempDir(), "too-long.jsonl")
	if err := os.WriteFile(logFile, []byte(strings.Repeat("a", 10*1024*1024+1)), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	s := &Server{
		logFile:    logFile,
		maxEntries: 10,
		entries:    make([]types.LogEntry, 0),
	}
	if err := s.loadEntries(); err == nil {
		t.Fatal("loadEntries() expected scanner error for oversized token, got nil")
	}
}

func TestSaveEntriesAndCopyErrorPath(t *testing.T) {
	t.Parallel()

	s, logFile := newTestServer(t, 10)
	s.entries = []types.LogEntry{
		{"ok": "first"},
		{"bad": func() {}},
		{"ok": "second"},
	}

	s.mu.Lock()
	err := s.saveEntries()
	s.mu.Unlock()
	if err != nil {
		t.Fatalf("saveEntries() error = %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("saved lines = %d, want 2 marshalable entries", len(lines))
	}

	s.logFile = filepath.Join(t.TempDir(), "missing", "dir", "logs.jsonl")
	if err := s.saveEntriesCopy([]types.LogEntry{{"x": "y"}}); err == nil {
		t.Fatal("saveEntriesCopy() expected create error, got nil")
	}
}

func TestAppendToFileSkipsUnmarshalableAndOpenErrors(t *testing.T) {
	t.Parallel()

	s, logFile := newTestServer(t, 10)
	err := s.appendToFile([]types.LogEntry{
		{"ok": 1},
		{"bad": func() {}},
		{"ok": 2},
	})
	if err != nil {
		t.Fatalf("appendToFile() error = %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("appended lines = %d, want 2 marshalable entries", len(lines))
	}

	s.logFile = t.TempDir() // directory path causes OpenFile to fail
	if err := s.appendToFile([]types.LogEntry{{"msg": "x"}}); err == nil {
		t.Fatal("appendToFile() expected OpenFile error for directory path, got nil")
	}
}

// TestSaveEntriesCopyAtomicWrite is the regression test for the truncate-write
// save path: os.Create truncated the live log before writing, so a failed save
// destroyed the previous contents. Saves now go through a same-directory temp
// file plus os.Rename, leaving the existing log untouched on failure.
func TestSaveEntriesCopyAtomicWrite(t *testing.T) {
	t.Parallel()

	s, logFile := newTestServer(t, 10)
	if err := s.saveEntriesCopy([]types.LogEntry{{"msg": "original"}}); err != nil {
		t.Fatalf("saveEntriesCopy() error = %v", err)
	}

	// A successful save must not leave the temp file behind.
	if _, err := os.Stat(logFile + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind after save: stat err = %v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(logFile)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("log file mode = %o, want 0600 (owner-only, matches live store)", got)
		}
	}

	// Block the temp path with a directory: the save must fail without
	// touching the existing log.
	if err := os.MkdirAll(logFile+".tmp", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := s.saveEntriesCopy([]types.LogEntry{{"msg": "replacement"}}); err == nil {
		t.Fatal("saveEntriesCopy() expected error when temp path is blocked, got nil")
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "original") {
		t.Fatalf("existing log was modified by a failed save: %s", data)
	}
	if strings.Contains(string(data), "replacement") {
		t.Fatalf("failed save must not be applied: %s", data)
	}
}

// TestAppendToFileOwnerOnlyPermissions verifies new log files are created 0600
// (owner-only), matching the live store's privacy choice for captured telemetry.
func TestAppendToFileOwnerOnlyPermissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not meaningful on windows")
	}

	s, logFile := newTestServer(t, 10)
	if err := s.appendToFile([]types.LogEntry{{"msg": "hello"}}); err != nil {
		t.Fatalf("appendToFile() error = %v", err)
	}

	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("appended log file mode = %o, want 0600", got)
	}
}

func TestHandleScreenshotValidationAndWriteFailure(t *testing.T) {
	// Not parallel: t.Setenv modifies process environment.
	s, _ := newTestServer(t, 10)

	missingDataReq := httptest.NewRequest(http.MethodPost, "/screenshot", strings.NewReader(`{"url":"https://example.com"}`))
	missingDataRR := httptest.NewRecorder()
	s.handleScreenshot(missingDataRR, missingDataReq)
	if missingDataRR.Code != http.StatusBadRequest {
		t.Fatalf("missing data_url status = %d, want %d", missingDataRR.Code, http.StatusBadRequest)
	}

	badFormatReq := httptest.NewRequest(http.MethodPost, "/screenshot", strings.NewReader(`{"data_url":"not-a-data-url"}`))
	badFormatRR := httptest.NewRecorder()
	s.handleScreenshot(badFormatRR, badFormatReq)
	if badFormatRR.Code != http.StatusBadRequest {
		t.Fatalf("invalid data_url format status = %d, want %d", badFormatRR.Code, http.StatusBadRequest)
	}

	image := []byte{0xff, 0xd8, 0xff, 0xd9}
	dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(image)

	// Create a file where a directory is expected so MkdirAll fails.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte{}, 0o644); err != nil {
		t.Fatalf("create blocker file: %v", err)
	}
	t.Setenv(state.StateDirEnv, blocker)

	writeFailReq := httptest.NewRequest(http.MethodPost, "/screenshot", strings.NewReader(`{"data_url":"`+dataURL+`"}`))
	writeFailRR := httptest.NewRecorder()
	s.handleScreenshot(writeFailRR, writeFailReq)
	if writeFailRR.Code != http.StatusInternalServerError {
		t.Fatalf("write failure status = %d, want %d", writeFailRR.Code, http.StatusInternalServerError)
	}
}

func TestHandleScreenshotBodyTooLarge(t *testing.T) {
	t.Parallel()

	s, _ := newTestServer(t, 10)
	tooLargeBody := bytes.NewBufferString(`{"data_url":"` + strings.Repeat("a", maxPostBodySize+1) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/screenshot", tooLargeBody)
	rr := httptest.NewRecorder()
	s.handleScreenshot(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("body too large status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestJSONResponseEncodeErrorPath(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	jsonResponse(rr, http.StatusTeapot, map[string]any{
		"bad": func() {},
	})
	if rr.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusTeapot)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

func TestOldestAndNewestLogTime(t *testing.T) {
	t.Parallel()

	s, _ := newTestServer(t, 10)
	if got := s.GetOldestLogTime(); !got.IsZero() {
		t.Fatalf("GetOldestLogTime() on empty server = %v, want zero", got)
	}
	if got := s.GetNewestLogTime(); !got.IsZero() {
		t.Fatalf("GetNewestLogTime() on empty server = %v, want zero", got)
	}

	s.addEntries([]types.LogEntry{{"id": "a"}})
	time.Sleep(2 * time.Millisecond)
	s.addEntries([]types.LogEntry{{"id": "b"}})

	oldest := s.GetOldestLogTime()
	newest := s.GetNewestLogTime()
	if oldest.IsZero() || newest.IsZero() {
		t.Fatalf("expected non-zero timestamps, got oldest=%v newest=%v", oldest, newest)
	}
	if newest.Before(oldest) {
		t.Fatalf("newest before oldest: oldest=%v newest=%v", oldest, newest)
	}
}
