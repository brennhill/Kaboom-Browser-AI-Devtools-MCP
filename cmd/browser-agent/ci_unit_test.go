// Purpose: Unit tests for browser-agent ci logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"bytes"
	"encoding/json"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/ciapi"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func newTestServerForHandlers(t *testing.T) *Server {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "logs.jsonl")
	s, err := NewServer(logPath, 1000)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(func() {
		s.logs.Shutdown(2 * time.Second)
	})
	return s
}

func TestHandleSnapshot_MethodAndSinceValidation(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	handler := ciapi.Snapshot(srv.logs, cap)

	notGetReq := httptest.NewRequest(http.MethodPost, "/snapshot", nil)
	notGetRR := httptest.NewRecorder()
	handler.ServeHTTP(notGetRR, notGetReq)
	if notGetRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", notGetRR.Code, http.StatusMethodNotAllowed)
	}

	badSinceReq := httptest.NewRequest(http.MethodGet, "/snapshot?since=not-a-time", nil)
	badSinceRR := httptest.NewRecorder()
	handler.ServeHTTP(badSinceRR, badSinceReq)
	if badSinceRR.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", badSinceRR.Code, http.StatusBadRequest)
	}
}

func TestHandleSnapshot_WithStatsAndActiveTestIDFallback(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()

	srv.logs.AddEntries([]types.LogEntry{
		{"level": "error", "message": "boom", "ts": time.Now().UTC().Format(time.RFC3339Nano)},
		{"level": "warn", "message": "warn", "ts": time.Now().UTC().Format(time.RFC3339Nano)},
		{"level": "info", "message": "info", "ts": time.Now().UTC().Format(time.RFC3339Nano)},
	})
	cap.AddWebSocketEvents([]types.WebSocketEvent{
		{Event: "open", URL: "wss://one"},
		{Event: "message", URL: "wss://one"},
		{Event: "open", URL: "wss://two"},
	})
	cap.AddNetworkBodies([]types.NetworkBody{
		{URL: "https://example.test/a", Status: 200},
		{URL: "https://example.test/b", Status: 502},
	})
	cap.AddEnhancedActions([]types.EnhancedAction{
		{Type: "click", Timestamp: 1, URL: "https://example.test"},
	})
	cap.SetTestBoundaryStart("test-123")

	handler := ciapi.Snapshot(srv.logs, cap)
	req := httptest.NewRequest(http.MethodGet, "/snapshot", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp ciapi.SnapshotResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if resp.TestID != "test-123" {
		t.Fatalf("TestID = %q, want test-123", resp.TestID)
	}
	if got := resp.Stats.TotalLogs; got != 3 {
		t.Fatalf("TotalLogs = %d, want 3", got)
	}
	if got := resp.Stats.ErrorCount; got != 1 {
		t.Fatalf("ErrorCount = %d, want 1", got)
	}
	if got := resp.Stats.WarningCount; got != 1 {
		t.Fatalf("WarningCount = %d, want 1", got)
	}
	if got := resp.Stats.NetworkFailures; got != 1 {
		t.Fatalf("NetworkFailures = %d, want 1", got)
	}
	if got := resp.Stats.WSConnections; got != 2 {
		t.Fatalf("WSConnections = %d, want 2 unique URLs", got)
	}
	if len(resp.EnhancedActions) != 1 {
		t.Fatalf("EnhancedActions len = %d, want 1", len(resp.EnhancedActions))
	}
}

func TestHandleSnapshot_SinceFilter(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()
	handler := ciapi.Snapshot(srv.logs, cap)

	oldTS := time.Now().UTC().Add(-10 * time.Second)
	cutoff := time.Now().UTC().Add(-5 * time.Second)
	newTS := time.Now().UTC().Add(-1 * time.Second)
	srv.logs.AddEntries([]types.LogEntry{
		{"level": "error", "message": "old", "ts": oldTS.Format(time.RFC3339Nano)},
		{"level": "error", "message": "new", "ts": newTS.Format(time.RFC3339Nano)},
	})

	req := httptest.NewRequest(http.MethodGet, "/snapshot?since="+cutoff.Format(time.RFC3339Nano), nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp ciapi.SnapshotResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Logs) != 1 {
		t.Fatalf("logs len = %d, want 1", len(resp.Logs))
	}
	if msg, _ := resp.Logs[0]["message"].(string); msg != "new" {
		t.Fatalf("kept message = %q, want new", msg)
	}
}

func TestHandleClearAndTestBoundaryHandlers(t *testing.T) {
	t.Parallel()

	srv := newTestServerForHandlers(t)
	cap := capture.NewCapture()

	srv.logs.AddEntries([]types.LogEntry{{"level": "error", "message": "x"}})
	cap.AddNetworkBodies([]types.NetworkBody{{URL: "https://example.test", Status: 200}})

	clearHandler := ciapi.Clear(srv.logs, cap)

	getReq := httptest.NewRequest(http.MethodGet, "/clear", nil)
	getRR := httptest.NewRecorder()
	clearHandler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /clear status = %d, want %d", getRR.Code, http.StatusMethodNotAllowed)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/clear", nil)
	postRR := httptest.NewRecorder()
	clearHandler.ServeHTTP(postRR, postReq)
	if postRR.Code != http.StatusOK {
		t.Fatalf("POST /clear status = %d, want %d", postRR.Code, http.StatusOK)
	}
	if srv.logs.EntryCount() != 0 {
		t.Fatalf("server entry count = %d, want 0 after clear", srv.logs.EntryCount())
	}
	if len(cap.GetNetworkBodies()) != 0 {
		t.Fatalf("network bodies len = %d, want 0 after clear", len(cap.GetNetworkBodies()))
	}

	testBoundary := ciapi.TestBoundary(cap)

	startReq := httptest.NewRequest(http.MethodPost, "/test-boundary", bytes.NewBufferString(`{"test_id":"checkout","action":"start"}`))
	startRR := httptest.NewRecorder()
	testBoundary.ServeHTTP(startRR, startReq)
	if startRR.Code != http.StatusOK {
		t.Fatalf("start boundary status = %d, want %d", startRR.Code, http.StatusOK)
	}
	found := false
	for _, id := range cap.GetActiveTestIDs() {
		if id == "checkout" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected checkout to be active after start")
	}

	endReq := httptest.NewRequest(http.MethodPost, "/test-boundary", bytes.NewBufferString(`{"test_id":"checkout","action":"end"}`))
	endRR := httptest.NewRecorder()
	testBoundary.ServeHTTP(endRR, endReq)
	if endRR.Code != http.StatusOK {
		t.Fatalf("end boundary status = %d, want %d", endRR.Code, http.StatusOK)
	}
	for _, id := range cap.GetActiveTestIDs() {
		if id == "checkout" {
			t.Fatal("expected checkout to be inactive after end")
		}
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/test-boundary", bytes.NewBufferString(`{"test_id":"x","action":"pause"}`))
	invalidRR := httptest.NewRecorder()
	testBoundary.ServeHTTP(invalidRR, invalidReq)
	if invalidRR.Code != http.StatusBadRequest {
		t.Fatalf("invalid action status = %d, want %d", invalidRR.Code, http.StatusBadRequest)
	}
}

func TestFilterLogsSinceAndComputeSnapshotStats(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	logs := []types.LogEntry{
		{"level": "error", "message": "bad-ts", "ts": "not-time"},
		{"level": "warn", "message": "old", "ts": now.Add(-5 * time.Second).Format(time.RFC3339Nano)},
		{"level": "error", "message": "new", "ts": now.Format(time.RFC3339Nano)},
	}
	filtered := ciapi.FilterLogsSince(logs, now.Add(-1*time.Second))
	if len(filtered) != 1 {
		t.Fatalf("filtered logs len = %d, want 1", len(filtered))
	}

	stats := ciapi.ComputeSnapshotStats(
		[]types.LogEntry{
			{"level": "error"},
			{"level": "warning"},
			{"level": "warn"},
		},
		[]types.WebSocketEvent{
			{URL: "wss://one"},
			{URL: "wss://one"},
			{URL: "wss://two"},
		},
		[]types.NetworkBody{
			{Status: 200},
			{Status: 404},
		},
	)
	if stats.ErrorCount != 1 || stats.WarningCount != 2 || stats.NetworkFailures != 1 || stats.WSConnections != 2 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}
