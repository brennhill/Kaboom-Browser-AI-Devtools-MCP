// Purpose: Tests for HTTP server request routing and response formatting.
// Docs: docs/features/feature/backend-log-streaming/index.md

package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

func newTestServer(t *testing.T, maxEntries int) (*Server, string) {
	t.Helper()

	logFile := filepath.Join(t.TempDir(), "server-log.jsonl")
	s, err := NewServer(logFile, maxEntries)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return s, logFile
}

func TestServerAddEntries_RotationAndAccounting(t *testing.T) {
	t.Parallel()

	s, logFile := newTestServer(t, 2)
	callbackCalls := 0
	s.SetOnEntries(func(entries []types.LogEntry) {
		callbackCalls++
		if len(entries) != 3 {
			t.Errorf("callback len(entries) = %d, want 3", len(entries))
		}
	})

	added := s.addEntries([]types.LogEntry{
		{"seq": 1, "message": "first"},
		{"seq": 2, "message": "second"},
		{"seq": 3, "message": "third"},
	})
	if added != 3 {
		t.Fatalf("addEntries() = %d, want 3", added)
	}
	if callbackCalls != 1 {
		t.Fatalf("callbackCalls = %d, want 1", callbackCalls)
	}

	if got := s.GetLogCount(); got != 2 {
		t.Fatalf("GetLogCount() = %d, want 2 after rotation", got)
	}
	if got := s.GetLogTotalAdded(); got != 3 {
		t.Fatalf("GetLogTotalAdded() = %d, want 3", got)
	}

	entries := s.getEntries()
	if entries[0]["seq"] != 2 || entries[1]["seq"] != 3 {
		t.Fatalf("rotated entries = %#v, want seq 2 then 3", entries)
	}

	data, err := os.ReadFile(logFile) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("log file line count = %d, want 2", len(lines))
	}
}

func TestGetLogEntries_ReturnsMapCopies(t *testing.T) {
	t.Parallel()

	s, _ := newTestServer(t, 10)
	s.addEntries([]types.LogEntry{{"level": "info"}})

	first := s.GetLogEntries()
	if len(first) != 1 {
		t.Fatalf("expected one entry, got %d", len(first))
	}
	first[0]["level"] = "mutated"

	second := s.GetLogEntries()
	if second[0]["level"] != "info" {
		t.Fatalf("mutation leaked into server state: got %v", second[0]["level"])
	}
}

func TestGetLogSnapshot_AndTimestampCopies(t *testing.T) {
	t.Parallel()

	s, _ := newTestServer(t, 10)
	s.addEntries([]types.LogEntry{{"id": "a"}})
	time.Sleep(2 * time.Millisecond)
	s.addEntries([]types.LogEntry{{"id": "b"}})

	snapshot := s.GetLogSnapshot()
	if snapshot.EntryCount != 2 {
		t.Fatalf("snapshot.EntryCount = %d, want 2", snapshot.EntryCount)
	}
	if snapshot.TotalAdded != 2 {
		t.Fatalf("snapshot.TotalAdded = %d, want 2", snapshot.TotalAdded)
	}
	if snapshot.OldestAddedAt.IsZero() || snapshot.LastAddedAt.IsZero() {
		t.Fatalf("expected non-zero timestamp bounds, got oldest=%v newest=%v", snapshot.OldestAddedAt, snapshot.LastAddedAt)
	}
	if snapshot.LastAddedAt.Before(snapshot.OldestAddedAt) {
		t.Fatalf("newest timestamp before oldest: oldest=%v newest=%v", snapshot.OldestAddedAt, snapshot.LastAddedAt)
	}

	times := s.GetLogTimestamps()
	if len(times) != 2 {
		t.Fatalf("GetLogTimestamps() len = %d, want 2", len(times))
	}
	times[0] = time.Time{}
	fresh := s.GetLogTimestamps()
	if fresh[0].IsZero() {
		t.Fatal("timestamp copy mutation leaked into server state")
	}
}

func TestSanitizeForFilename(t *testing.T) {
	t.Parallel()

	input := "unsafe /:*?<>| name " + strings.Repeat("x", 80)
	got := util.SanitizeForFilename(input)
	if len(got) > 50 {
		t.Fatalf("sanitized filename length = %d, want <= 50", len(got))
	}
	// Verify no unsafe characters remain (only a-zA-Z0-9._- are allowed).
	for _, c := range got {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-') {
			t.Fatalf("sanitized filename still has unsafe char %q in: %q", string(c), got)
		}
	}
}

func TestHandleScreenshot_PostAndValidation(t *testing.T) {
	// Not parallel: t.Setenv modifies process environment.
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	s, _ := newTestServer(t, 10)

	jpegBytes := []byte{0xff, 0xd8, 0xff, 0xd9}
	dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpegBytes)
	body := map[string]string{
		"data_url":       dataURL,
		"url":            "https://example.com/page",
		"correlation_id": "corr/1:2",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/screenshot", bytes.NewReader(bodyJSON))
	rr := httptest.NewRecorder()
	s.handleScreenshot(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /screenshot status = %d, want 200, body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["correlation_id"] != "corr/1:2" {
		t.Fatalf("correlation_id = %q, want corr/1:2", resp["correlation_id"])
	}
	if !strings.Contains(resp["filename"], "example.com") {
		t.Fatalf("expected hostname in filename, got %q", resp["filename"])
	}
	expectedDir := filepath.Join(stateRoot, "screenshots")
	if !strings.HasPrefix(resp["path"], expectedDir) {
		t.Fatalf("screenshot path %q should be in screenshots dir %q", resp["path"], expectedDir)
	}
	saved, err := os.ReadFile(resp["path"])
	if err != nil {
		t.Fatalf("read saved screenshot: %v", err)
	}
	if !bytes.Equal(saved, jpegBytes) {
		t.Fatalf("saved screenshot bytes mismatch: got %v want %v", saved, jpegBytes)
	}

	// Method guard
	getReq := httptest.NewRequest(http.MethodGet, "/screenshot", nil)
	getRR := httptest.NewRecorder()
	s.handleScreenshot(getRR, getReq)
	if getRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /screenshot status = %d, want 405", getRR.Code)
	}

	// Invalid base64 payload
	invalidReq := httptest.NewRequest(http.MethodPost, "/screenshot", strings.NewReader(`{"data_url":"data:image/jpeg;base64,!!!"}`))
	invalidRR := httptest.NewRecorder()
	s.handleScreenshot(invalidRR, invalidReq)
	if invalidRR.Code != http.StatusBadRequest {
		t.Fatalf("invalid base64 status = %d, want 400", invalidRR.Code)
	}
}

func TestClearEntries_ClearsMemoryAndFile(t *testing.T) {
	t.Parallel()

	s, logFile := newTestServer(t, 10)
	s.addEntries([]types.LogEntry{{"msg": "a"}, {"msg": "b"}})
	if s.getEntryCount() != 2 {
		t.Fatalf("precondition failed: expected 2 entries")
	}

	s.clearEntries()
	if s.getEntryCount() != 0 {
		t.Fatalf("expected 0 entries after clear, got %d", s.getEntryCount())
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected log file to be empty after clear, got %q", string(data))
	}
}
