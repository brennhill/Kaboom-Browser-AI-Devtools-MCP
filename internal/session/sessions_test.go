// sessions_test.go — Tests snapshot capture, lifecycle, concurrency, and limits.
// Docs: docs/features/feature/historical-snapshots/index.md

package session

import (
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestRuntimeStateReaderProjectsCanonicalTelemetry(t *testing.T) {
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	capturefixture.Track(cap, 4, "https://tracked.example.test")
	cap.Telemetry().AddNetworkBodies([]types.NetworkBody{{
		Method:       "GET",
		URL:          "https://api.example.test/data",
		Status:       503,
		Duration:     42,
		ResponseBody: "down",
		ContentType:  "application/json",
	}})
	cap.Telemetry().AddWebSocketEvents([]types.WebSocketEvent{
		{Event: "open", ID: "socket-1", URL: "wss://example.test/live"},
		{Event: "message", ID: "socket-1", Direction: "incoming", Data: "hello"},
	})
	entries := []types.LogEntry{
		{"level": "error", "message": " broken "},
		{"level": "error", "message": "broken"},
		{"level": "warn", "message": "careful"},
		{"level": "info", "message": "ignored"},
	}
	perf := []performance.PerformanceSnapshot{
		{URL: "https://old.example.test", Timestamp: "2026-01-01T00:00:00Z"},
		{URL: "https://new.example.test", Timestamp: "2026-01-01T00:00:01.123Z"},
	}
	reader := NewRuntimeStateReader(
		func() []types.LogEntry { return entries },
		func() []performance.PerformanceSnapshot { return perf },
		cap,
	)

	errors := reader.GetConsoleErrors()
	if len(errors) != 1 || errors[0].Message != "broken" || errors[0].Count != 2 {
		t.Fatalf("console errors = %#v", errors)
	}
	warnings := reader.GetConsoleWarnings()
	if len(warnings) != 1 || warnings[0].Message != "careful" {
		t.Fatalf("console warnings = %#v", warnings)
	}
	requests := reader.GetNetworkRequests()
	if len(requests) != 1 || requests[0].Status != 503 || requests[0].ResponseSize != 4 {
		t.Fatalf("network requests = %#v", requests)
	}
	connections := reader.GetWSConnections()
	if len(connections) != 1 || connections[0].URL != "wss://example.test/live" {
		t.Fatalf("websocket connections = %#v", connections)
	}
	if snapshot := reader.GetPerformance(); snapshot == nil || snapshot.URL != "https://new.example.test" {
		t.Fatalf("performance snapshot = %#v", snapshot)
	}
	if got := reader.GetCurrentPageURL(); got != "https://tracked.example.test" {
		t.Fatalf("page URL = %q", got)
	}
}

func TestRuntimeStateReaderHandlesMissingSources(t *testing.T) {
	reader := NewRuntimeStateReader(nil, nil, nil)
	if len(reader.GetConsoleErrors()) != 0 ||
		len(reader.GetNetworkRequests()) != 0 ||
		len(reader.GetWSConnections()) != 0 ||
		reader.GetPerformance() != nil ||
		reader.GetCurrentPageURL() != "" {
		t.Fatal("missing runtime sources should project empty state")
	}
}

func TestRuntimeStateReaderAggregatesConsoleErrors(t *testing.T) {
	t.Parallel()
	reader := NewRuntimeStateReader(func() []types.LogEntry {
		return []types.LogEntry{
			{"level": "error", "message": " broken "},
			{"level": "error", "message": "broken"},
			{"level": "warn", "message": "careful"},
		}
	}, nil, nil)
	errors := reader.GetConsoleErrors()
	if len(errors) != 1 || errors[0].Message != "broken" || errors[0].Count != 2 {
		t.Fatalf("errors = %#v", errors)
	}
}

// ============================================
// Mock CaptureStateReader
// ============================================

type mockCaptureState struct {
	consoleErrors   []types.SnapshotError
	consoleWarnings []types.SnapshotError
	networkRequests []types.SnapshotNetworkRequest
	wsConnections   []types.SnapshotWSConnection
	performance     *performance.PerformanceSnapshot
	pageURL         string
}

func (m *mockCaptureState) GetConsoleErrors() []types.SnapshotError {
	if m.consoleErrors == nil {
		return []types.SnapshotError{}
	}
	return m.consoleErrors
}

func (m *mockCaptureState) GetConsoleWarnings() []types.SnapshotError {
	if m.consoleWarnings == nil {
		return []types.SnapshotError{}
	}
	return m.consoleWarnings
}

func (m *mockCaptureState) GetNetworkRequests() []types.SnapshotNetworkRequest {
	if m.networkRequests == nil {
		return []types.SnapshotNetworkRequest{}
	}
	return m.networkRequests
}

func (m *mockCaptureState) GetWSConnections() []types.SnapshotWSConnection {
	if m.wsConnections == nil {
		return []types.SnapshotWSConnection{}
	}
	return m.wsConnections
}

func (m *mockCaptureState) GetPerformance() *performance.PerformanceSnapshot {
	return m.performance
}

func (m *mockCaptureState) GetCurrentPageURL() string {
	return m.pageURL
}

// ============================================
// Test: Capture (Save) Snapshot
// ============================================

func TestSessionManager_CaptureSnapshot(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		consoleErrors: []types.SnapshotError{
			{Type: "console", Message: "TypeError: cannot read null", Count: 1},
		},
		consoleWarnings: []types.SnapshotError{
			{Type: "console", Message: "Deprecation warning: componentWillMount", Count: 2},
		},
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200, Duration: 150},
			{Method: "POST", URL: "/api/login", Status: 401, Duration: 50},
		},
		wsConnections: []types.SnapshotWSConnection{
			{URL: "ws://localhost:8080/ws", State: "open"},
		},
		performance: &performance.PerformanceSnapshot{
			URL: "http://localhost:3000/dashboard",
			Timing: performance.PerformanceTiming{
				Load:             1100,
				TimeToFirstByte:  200,
				DomContentLoaded: 800,
				DomInteractive:   750,
			},
			Network: performance.NetworkSummary{
				RequestCount: 12,
				TransferSize: 340000,
			},
		},
		pageURL: "http://localhost:3000/dashboard",
	}

	sm := NewSessionManager(10, mock)

	result, err := sm.Capture("before-deploy", "")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if result.Name != "before-deploy" {
		t.Errorf("Expected name 'before-deploy', got %q", result.Name)
	}
	if result.CapturedAt.IsZero() {
		t.Error("CapturedAt should not be zero")
	}
	if result.PageURL != "http://localhost:3000/dashboard" {
		t.Errorf("Expected page URL 'http://localhost:3000/dashboard', got %q", result.PageURL)
	}
	if len(result.ConsoleErrors) != 1 {
		t.Errorf("Expected 1 console error, got %d", len(result.ConsoleErrors))
	}
	if len(result.ConsoleWarnings) != 1 {
		t.Errorf("Expected 1 console warning, got %d", len(result.ConsoleWarnings))
	}
	if len(result.NetworkRequests) != 2 {
		t.Errorf("Expected 2 network requests, got %d", len(result.NetworkRequests))
	}
	if len(result.WebSocketConnections) != 1 {
		t.Errorf("Expected 1 WS connection, got %d", len(result.WebSocketConnections))
	}
	if result.Performance == nil {
		t.Fatal("Performance should not be nil")
	}
	if result.Performance.Timing.Load != 1100 {
		t.Errorf("Expected load time 1100, got %v", result.Performance.Timing.Load)
	}
}

func TestSessionManager_CaptureWithURLFilter(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200},
			{Method: "GET", URL: "/api/dashboard", Status: 200},
			{Method: "GET", URL: "/static/main.js", Status: 200},
		},
		pageURL: "http://localhost:3000",
	}

	sm := NewSessionManager(10, mock)
	result, err := sm.Capture("api-only", "/api/")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if len(result.NetworkRequests) != 2 {
		t.Errorf("Expected 2 filtered network requests, got %d", len(result.NetworkRequests))
	}
	if result.URLFilter != "/api/" {
		t.Errorf("Expected URLFilter '/api/', got %q", result.URLFilter)
	}
}

func TestSessionManager_CaptureOverwritesDuplicate(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		consoleErrors: []types.SnapshotError{
			{Type: "console", Message: "Error one", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}

	sm := NewSessionManager(10, mock)

	_, err := sm.Capture("snapshot-a", "")
	if err != nil {
		t.Fatalf("First capture failed: %v", err)
	}

	// Update mock state
	mock.consoleErrors = []types.SnapshotError{
		{Type: "console", Message: "Error two", Count: 1},
	}

	result, err := sm.Capture("snapshot-a", "")
	if err != nil {
		t.Fatalf("Second capture failed: %v", err)
	}

	// Should have the updated state
	if len(result.ConsoleErrors) != 1 || result.ConsoleErrors[0].Message != "Error two" {
		t.Errorf("Expected overwritten snapshot with 'Error two', got %v", result.ConsoleErrors)
	}

	// Should still only have one snapshot
	list := sm.List()
	if len(list) != 1 {
		t.Errorf("Expected 1 snapshot after overwrite, got %d", len(list))
	}
}

func TestSessionManager_CaptureNameValidation(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Empty name
	_, err := sm.Capture("", "")
	if err == nil {
		t.Error("Expected error for empty name")
	}

	// Reserved name "current"
	_, err = sm.Capture("current", "")
	if err == nil {
		t.Error("Expected error for reserved name 'current'")
	}

	// Name too long (>50 chars)
	longName := "this-is-a-very-long-snapshot-name-that-exceeds-fifty-characters-limit"
	_, err = sm.Capture(longName, "")
	if err == nil {
		t.Error("Expected error for name exceeding 50 characters")
	}
}

func TestSessionManager_CaptureEmptyState(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	result, err := sm.Capture("empty-state", "")
	if err != nil {
		t.Fatalf("Capture of empty state failed: %v", err)
	}

	if len(result.ConsoleErrors) != 0 {
		t.Errorf("Expected 0 console errors, got %d", len(result.ConsoleErrors))
	}
	if len(result.NetworkRequests) != 0 {
		t.Errorf("Expected 0 network requests, got %d", len(result.NetworkRequests))
	}
	if result.Performance != nil {
		t.Error("Expected nil performance for empty state")
	}
}

// ============================================
// Test: List Snapshots
// ============================================

func TestSessionManager_List(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []types.SnapshotError{
		{Type: "console", Message: "Err", Count: 1},
	}
	sm.Capture("snapshot-1", "")

	mock.consoleErrors = []types.SnapshotError{}
	sm.Capture("snapshot-2", "")

	list := sm.List()
	if len(list) != 2 {
		t.Fatalf("Expected 2 snapshots, got %d", len(list))
	}

	// Verify ordering (insertion order)
	if list[0].Name != "snapshot-1" {
		t.Errorf("Expected first snapshot 'snapshot-1', got %q", list[0].Name)
	}
	if list[1].Name != "snapshot-2" {
		t.Errorf("Expected second snapshot 'snapshot-2', got %q", list[1].Name)
	}

	// Verify metadata
	if list[0].ErrorCount != 1 {
		t.Errorf("Expected error_count=1 for snapshot-1, got %d", list[0].ErrorCount)
	}
}

func TestSessionManager_ListEmpty(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	list := sm.List()
	if len(list) != 0 {
		t.Errorf("Expected empty list, got %d", len(list))
	}
}

// ============================================
// Test: Delete Snapshot
// ============================================

func TestSessionManager_Delete(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("to-delete", "")
	sm.Capture("to-keep", "")

	err := sm.Delete("to-delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	list := sm.List()
	if len(list) != 1 {
		t.Fatalf("Expected 1 snapshot after delete, got %d", len(list))
	}
	if list[0].Name != "to-keep" {
		t.Errorf("Expected 'to-keep' to remain, got %q", list[0].Name)
	}
}

func TestSessionManager_DeleteNonExistent(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	err := sm.Delete("nonexistent")
	if err == nil {
		t.Error("Expected error when deleting non-existent snapshot")
	}
}

// ============================================
// Test: Max Snapshots Eviction
// ============================================

func TestSessionManager_MaxSnapshotsEviction(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(3, mock)

	sm.Capture("first", "")
	sm.Capture("second", "")
	sm.Capture("third", "")

	// Adding a fourth should evict "first"
	sm.Capture("fourth", "")

	list := sm.List()
	if len(list) != 3 {
		t.Fatalf("Expected 3 snapshots (max), got %d", len(list))
	}

	// "first" should be gone
	for _, snap := range list {
		if snap.Name == "first" {
			t.Error("Expected 'first' to be evicted")
		}
	}

	// "second", "third", "fourth" should exist
	names := make(map[string]bool)
	for _, snap := range list {
		names[snap.Name] = true
	}
	if !names["second"] || !names["third"] || !names["fourth"] {
		t.Errorf("Expected second, third, fourth to exist, got %v", names)
	}
}

// ============================================
// Test: Snapshot Name Case Sensitivity
// ============================================

func TestSessionManager_CaseSensitiveNames(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []types.SnapshotError{{Type: "console", Message: "E1", Count: 1}}
	sm.Capture("Snapshot", "")

	mock.consoleErrors = []types.SnapshotError{{Type: "console", Message: "E2", Count: 1}}
	sm.Capture("snapshot", "")

	list := sm.List()
	if len(list) != 2 {
		t.Fatalf("Expected 2 distinct snapshots (case-sensitive), got %d", len(list))
	}
}

// ============================================
// Test: Concurrent Access
// ============================================

func TestSessionManager_ConcurrentSafety(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		pageURL: "http://localhost:3000",
		consoleErrors: []types.SnapshotError{
			{Type: "console", Message: "concurrent error", Count: 1},
		},
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/test", Status: 200},
		},
	}
	sm := NewSessionManager(10, mock)

	var wg sync.WaitGroup
	// Concurrent saves
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "snap-" + time.Now().Format("150405.000000000") + "-" + string(rune('a'+idx))
			sm.Capture(name, "")
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.List()
		}()
	}

	wg.Wait()

	// Should not panic and list should have up to 10 items
	list := sm.List()
	if len(list) > 10 {
		t.Errorf("Expected at most 10 snapshots, got %d", len(list))
	}
}

func TestSessionManager_ConcurrentSaveAndCompare(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		pageURL:       "http://localhost:3000",
		consoleErrors: []types.SnapshotError{{Type: "console", Message: "err", Count: 1}},
	}
	sm := NewSessionManager(10, mock)

	sm.Capture("base", "")

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			sm.Compare("base", "current")
		}()
		go func(idx int) {
			defer wg.Done()
			name := "concurrent-" + string(rune('a'+idx))
			sm.Capture(name, "")
		}(i)
	}
	wg.Wait()
	// No panics = success
}

// ============================================
// Test: Snapshot Limits
// ============================================

func TestSessionManager_ConsoleEntriesLimit(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Create more than 50 errors
	errors := make([]types.SnapshotError, 60)
	for i := range errors {
		errors[i] = types.SnapshotError{Type: "console", Message: "Error " + string(rune('A'+i%26)), Count: 1}
	}
	mock.consoleErrors = errors

	result, err := sm.Capture("limited", "")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if len(result.ConsoleErrors) > 50 {
		t.Errorf("Expected at most 50 console errors, got %d", len(result.ConsoleErrors))
	}
}

func TestSessionManager_NetworkRequestsLimit(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Create more than 100 requests
	requests := make([]types.SnapshotNetworkRequest, 120)
	for i := range requests {
		requests[i] = types.SnapshotNetworkRequest{Method: "GET", URL: "/api/" + string(rune('a'+i%26)), Status: 200}
	}
	mock.networkRequests = requests

	result, err := sm.Capture("limited", "")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if len(result.NetworkRequests) > 100 {
		t.Errorf("Expected at most 100 network requests, got %d", len(result.NetworkRequests))
	}
}
