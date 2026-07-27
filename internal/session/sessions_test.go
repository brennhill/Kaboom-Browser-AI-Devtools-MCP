// Purpose: Tests for session lifecycle management.
// Docs: docs/features/feature/request-session-correlation/index.md

package session

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/snapdiff"
)

// ============================================
// Mock CaptureStateReader
// ============================================

type mockCaptureState struct {
	consoleErrors   []SnapshotError
	consoleWarnings []SnapshotError
	networkRequests []SnapshotNetworkRequest
	wsConnections   []SnapshotWSConnection
	performance     *performance.PerformanceSnapshot
	pageURL         string
}

func (m *mockCaptureState) GetConsoleErrors() []SnapshotError {
	if m.consoleErrors == nil {
		return []SnapshotError{}
	}
	return m.consoleErrors
}

func (m *mockCaptureState) GetConsoleWarnings() []SnapshotError {
	if m.consoleWarnings == nil {
		return []SnapshotError{}
	}
	return m.consoleWarnings
}

func (m *mockCaptureState) GetNetworkRequests() []SnapshotNetworkRequest {
	if m.networkRequests == nil {
		return []SnapshotNetworkRequest{}
	}
	return m.networkRequests
}

func (m *mockCaptureState) GetWSConnections() []SnapshotWSConnection {
	if m.wsConnections == nil {
		return []SnapshotWSConnection{}
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
		consoleErrors: []SnapshotError{
			{Type: "console", Message: "TypeError: cannot read null", Count: 1},
		},
		consoleWarnings: []SnapshotError{
			{Type: "console", Message: "Deprecation warning: componentWillMount", Count: 2},
		},
		networkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200, Duration: 150},
			{Method: "POST", URL: "/api/login", Status: 401, Duration: 50},
		},
		wsConnections: []SnapshotWSConnection{
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
		networkRequests: []SnapshotNetworkRequest{
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
		consoleErrors: []SnapshotError{
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
	mock.consoleErrors = []SnapshotError{
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
// Test: Compare Snapshots
// ============================================

func TestSessionManager_CompareDetectsNewErrors(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Snapshot A: no errors
	mock.consoleErrors = []SnapshotError{}
	sm.Capture("before", "")

	// Snapshot B: has errors
	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "React hydration mismatch", Count: 3},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(diff.Errors.New) != 1 {
		t.Fatalf("Expected 1 new error, got %d", len(diff.Errors.New))
	}
	if diff.Errors.New[0].Message != "React hydration mismatch" {
		t.Errorf("Expected 'React hydration mismatch', got %q", diff.Errors.New[0].Message)
	}
	if diff.Summary.NewErrors != 1 {
		t.Errorf("Expected summary.new_errors=1, got %d", diff.Summary.NewErrors)
	}
}

func TestSessionManager_CompareDetectsResolvedErrors(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Snapshot A: has errors
	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "TypeError: x is null", Count: 1},
		{Type: "console", Message: "ReferenceError: y is not defined", Count: 1},
	}
	sm.Capture("before", "")

	// Snapshot B: one error resolved
	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "TypeError: x is null", Count: 1},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(diff.Errors.Resolved) != 1 {
		t.Fatalf("Expected 1 resolved error, got %d", len(diff.Errors.Resolved))
	}
	if diff.Errors.Resolved[0].Message != "ReferenceError: y is not defined" {
		t.Errorf("Wrong resolved error: %q", diff.Errors.Resolved[0].Message)
	}
	if len(diff.Errors.Unchanged) != 1 {
		t.Errorf("Expected 1 unchanged error, got %d", len(diff.Errors.Unchanged))
	}
	if diff.Summary.ResolvedErrors != 1 {
		t.Errorf("Expected summary.resolved_errors=1, got %d", diff.Summary.ResolvedErrors)
	}
}

func TestSessionManager_CompareDetectsNewNetworkCalls(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
		{Method: "GET", URL: "/api/feature-flags", Status: 200},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(diff.Network.NewEndpoints) != 1 {
		t.Fatalf("Expected 1 new endpoint, got %d", len(diff.Network.NewEndpoints))
	}
	if diff.Network.NewEndpoints[0].URL != "/api/feature-flags" {
		t.Errorf("Expected '/api/feature-flags', got %q", diff.Network.NewEndpoints[0].URL)
	}
}

func TestSessionManager_CompareDetectsRemovedNetworkCalls(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
		{Method: "GET", URL: "/api/legacy", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(diff.Network.MissingEndpoints) != 1 {
		t.Fatalf("Expected 1 missing endpoint, got %d", len(diff.Network.MissingEndpoints))
	}
	if diff.Network.MissingEndpoints[0].URL != "/api/legacy" {
		t.Errorf("Expected '/api/legacy', got %q", diff.Network.MissingEndpoints[0].URL)
	}
}

func TestSessionManager_CompareDetectsStatusCodeChanges(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/dashboard", Status: 200, Duration: 100},
	}
	sm.Capture("before", "")

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/dashboard", Status: 502, Duration: 440},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(diff.Network.StatusChanges) != 1 {
		t.Fatalf("Expected 1 status change, got %d", len(diff.Network.StatusChanges))
	}
	sc := diff.Network.StatusChanges[0]
	if sc.BeforeStatus != 200 || sc.AfterStatus != 502 {
		t.Errorf("Expected 200->502, got %d->%d", sc.BeforeStatus, sc.AfterStatus)
	}
	if sc.Method != "GET" || sc.URL != "/api/dashboard" {
		t.Errorf("Wrong endpoint: %s %s", sc.Method, sc.URL)
	}
}

func TestSessionManager_CompareDetectsNewNetworkErrors(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
		{Method: "GET", URL: "/api/notifications", Status: 502},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(diff.Network.NewErrors) != 1 {
		t.Fatalf("Expected 1 new network error, got %d", len(diff.Network.NewErrors))
	}
	if diff.Network.NewErrors[0].Status != 502 {
		t.Errorf("Expected status 502, got %d", diff.Network.NewErrors[0].Status)
	}
	if diff.Summary.NewNetworkErrors != 1 {
		t.Errorf("Expected summary.new_network_errors=1, got %d", diff.Summary.NewNetworkErrors)
	}
}

func TestSessionManager_ComparePerformanceRegression(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.performance = &performance.PerformanceSnapshot{
		Timing:  performance.PerformanceTiming{Load: 1100, TimeToFirstByte: 200},
		Network: performance.NetworkSummary{RequestCount: 12, TransferSize: 340000},
	}
	sm.Capture("before", "")

	mock.performance = &performance.PerformanceSnapshot{
		Timing:  performance.PerformanceTiming{Load: 3200, TimeToFirstByte: 800},
		Network: performance.NetworkSummary{RequestCount: 47, TransferSize: 2400000},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Performance.LoadTime == nil {
		t.Fatal("Expected load_time comparison")
	}
	if diff.Performance.LoadTime.Before != 1100 || diff.Performance.LoadTime.After != 3200 {
		t.Errorf("Expected load 1100->3200, got %v->%v", diff.Performance.LoadTime.Before, diff.Performance.LoadTime.After)
	}
	if !diff.Performance.LoadTime.Regression {
		t.Error("Expected load time regression=true")
	}
	if diff.Summary.PerformanceRegressions < 1 {
		t.Errorf("Expected at least 1 performance regression, got %d", diff.Summary.PerformanceRegressions)
	}
}

func TestSessionManager_CompareVsCurrent(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Save snapshot
	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "Error A", Count: 1},
	}
	sm.Capture("saved", "")

	// Change mock state to simulate "current"
	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "Error A", Count: 1},
		{Type: "console", Message: "Error B", Count: 1},
	}

	diff, err := sm.Compare("saved", "current")
	if err != nil {
		t.Fatalf("Compare vs current failed: %v", err)
	}

	if diff.B != "current" {
		t.Errorf("Expected b='current', got %q", diff.B)
	}
	if len(diff.Errors.New) != 1 {
		t.Errorf("Expected 1 new error vs current, got %d", len(diff.Errors.New))
	}
}

func TestSessionManager_CompareNonExistentSnapshot(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	_, err := sm.Compare("nonexistent", "also-nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent snapshot")
	}

	sm.Capture("exists", "")
	_, err = sm.Compare("exists", "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent compare_b")
	}
}

// ============================================
// Test: Verdict Logic
// ============================================

func TestSessionManager_VerdictImproved(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "Error one", Count: 1},
		{Type: "console", Message: "Error two", Count: 1},
	}
	sm.Capture("before", "")

	mock.consoleErrors = []SnapshotError{}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Summary.Verdict != "improved" {
		t.Errorf("Expected verdict 'improved', got %q", diff.Summary.Verdict)
	}
}

func TestSessionManager_VerdictRegressed(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []SnapshotError{}
	sm.Capture("before", "")

	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "New error", Count: 1},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Summary.Verdict != "regressed" {
		t.Errorf("Expected verdict 'regressed', got %q", diff.Summary.Verdict)
	}
}

func TestSessionManager_VerdictRegressedByNetworkError(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/data", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/data", Status: 500},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Summary.Verdict != "regressed" {
		t.Errorf("Expected verdict 'regressed', got %q", diff.Summary.Verdict)
	}
}

func TestSessionManager_VerdictRegressedByPerformance(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.performance = &performance.PerformanceSnapshot{
		Timing:  performance.PerformanceTiming{Load: 1000},
		Network: performance.NetworkSummary{RequestCount: 10, TransferSize: 100000},
	}
	sm.Capture("before", "")

	mock.performance = &performance.PerformanceSnapshot{
		Timing:  performance.PerformanceTiming{Load: 3000},
		Network: performance.NetworkSummary{RequestCount: 10, TransferSize: 100000},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Summary.Verdict != "regressed" {
		t.Errorf("Expected verdict 'regressed', got %q", diff.Summary.Verdict)
	}
}

func TestSessionManager_VerdictUnchanged(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "Error A", Count: 1},
	}
	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
	}
	sm.Capture("before", "")
	// Same state — capture again
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Summary.Verdict != "unchanged" {
		t.Errorf("Expected verdict 'unchanged', got %q", diff.Summary.Verdict)
	}
}

func TestSessionManager_VerdictMixed(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "Old error", Count: 1},
	}
	sm.Capture("before", "")

	// Old error resolved, but new one appeared
	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "New error", Count: 1},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Summary.Verdict != "mixed" {
		t.Errorf("Expected verdict 'mixed', got %q", diff.Summary.Verdict)
	}
}

// ============================================
// Test: List Snapshots
// ============================================

func TestSessionManager_List(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []SnapshotError{
		{Type: "console", Message: "Err", Count: 1},
	}
	sm.Capture("snapshot-1", "")

	mock.consoleErrors = []SnapshotError{}
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

	mock.consoleErrors = []SnapshotError{{Type: "console", Message: "E1", Count: 1}}
	sm.Capture("Snapshot", "")

	mock.consoleErrors = []SnapshotError{{Type: "console", Message: "E2", Count: 1}}
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
		consoleErrors: []SnapshotError{
			{Type: "console", Message: "concurrent error", Count: 1},
		},
		networkRequests: []SnapshotNetworkRequest{
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
		consoleErrors: []SnapshotError{{Type: "console", Message: "err", Count: 1}},
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
// Test: Performance Diff Details
// ============================================

func TestSessionManager_ComparePerformanceNoRegression(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.performance = &performance.PerformanceSnapshot{
		Timing:  performance.PerformanceTiming{Load: 1000},
		Network: performance.NetworkSummary{RequestCount: 10, TransferSize: 100000},
	}
	sm.Capture("before", "")

	// Slightly faster — no regression
	mock.performance = &performance.PerformanceSnapshot{
		Timing:  performance.PerformanceTiming{Load: 900},
		Network: performance.NetworkSummary{RequestCount: 10, TransferSize: 100000},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Performance.LoadTime == nil {
		t.Fatal("Expected load time diff")
	}
	if diff.Performance.LoadTime.Regression {
		t.Error("Expected no regression for faster load")
	}
	if diff.Summary.PerformanceRegressions != 0 {
		t.Errorf("Expected 0 performance regressions, got %d", diff.Summary.PerformanceRegressions)
	}
}

func TestSessionManager_CompareNoPerformanceData(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.performance = nil
	sm.Capture("before", "")
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Performance.LoadTime != nil {
		t.Error("Expected nil load time when no performance data")
	}
}

// ============================================
// Test: JSON Serialization
// ============================================

func TestSessionDiff_JSONSerialization(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []SnapshotError{}
	sm.Capture("a", "")
	mock.consoleErrors = []SnapshotError{{Type: "console", Message: "err", Count: 1}}
	sm.Capture("b", "")

	diff, err := sm.Compare("a", "b")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	data, err := json.Marshal(diff)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed snapdiff.Result
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if parsed.Summary.Verdict != "regressed" {
		t.Errorf("Expected verdict 'regressed' after round-trip, got %q", parsed.Summary.Verdict)
	}
}

// ============================================
// Test: Tool Handler Integration
// ============================================

func TestHandleDiffSessions_Capture(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		pageURL: "http://localhost:3000/test",
		consoleErrors: []SnapshotError{
			{Type: "console", Message: "test error", Count: 1},
		},
		networkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/health", Status: 200},
		},
	}
	sm := NewSessionManager(10, mock)

	params := map[string]any{
		"action": "capture",
		"name":   "test-snap",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err != nil {
		t.Fatalf("HandleTool capture failed: %v", err)
	}

	resultJSON, _ := json.Marshal(result)
	var response map[string]any
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if response["action"] != "captured" {
		t.Errorf("Expected action 'captured', got %v", response["action"])
	}
	if response["name"] != "test-snap" {
		t.Errorf("Expected name 'test-snap', got %v", response["name"])
	}
}

func TestHandleDiffSessions_List(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("snap-1", "")
	sm.Capture("snap-2", "")

	params := map[string]any{"action": "list"}
	paramsJSON, _ := json.Marshal(params)

	result, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err != nil {
		t.Fatalf("HandleTool list failed: %v", err)
	}

	resultJSON, _ := json.Marshal(result)
	var response map[string]any
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if response["action"] != "listed" {
		t.Errorf("Expected action 'listed', got %v", response["action"])
	}
	snapshots, ok := response["snapshots"].([]any)
	if !ok {
		t.Fatal("Expected snapshots array in response")
	}
	if len(snapshots) != 2 {
		t.Errorf("Expected 2 snapshots, got %d", len(snapshots))
	}
}

func TestHandleDiffSessions_Compare(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.consoleErrors = []SnapshotError{}
	sm.Capture("before", "")
	mock.consoleErrors = []SnapshotError{{Type: "console", Message: "err", Count: 1}}
	sm.Capture("after", "")

	params := map[string]any{
		"action":    "compare",
		"compare_a": "before",
		"compare_b": "after",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err != nil {
		t.Fatalf("HandleTool compare failed: %v", err)
	}

	resultJSON, _ := json.Marshal(result)
	var response map[string]any
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if response["action"] != "compared" {
		t.Errorf("Expected action 'compared', got %v", response["action"])
	}
}

func TestHandleDiffSessions_Delete(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("to-delete", "")

	params := map[string]any{
		"action": "delete",
		"name":   "to-delete",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err != nil {
		t.Fatalf("HandleTool delete failed: %v", err)
	}

	resultJSON, _ := json.Marshal(result)
	var response map[string]any
	if err := json.Unmarshal(resultJSON, &response); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if response["action"] != "deleted" {
		t.Errorf("Expected action 'deleted', got %v", response["action"])
	}

	list := sm.List()
	if len(list) != 0 {
		t.Errorf("Expected 0 snapshots after delete, got %d", len(list))
	}
}

func TestHandleDiffSessions_InvalidAction(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	params := map[string]any{"action": "invalid"}
	paramsJSON, _ := json.Marshal(params)

	_, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err == nil {
		t.Error("Expected error for invalid action")
	}
}

func TestHandleDiffSessions_MissingAction(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	params := map[string]any{}
	paramsJSON, _ := json.Marshal(params)

	_, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err == nil {
		t.Error("Expected error for missing action")
	}
}

func TestHandleDiffSessions_CaptureRequiresName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	params := map[string]any{"action": "capture"}
	paramsJSON, _ := json.Marshal(params)

	_, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err == nil {
		t.Error("Expected error when capture action has no name")
	}
}

func TestHandleDiffSessions_CompareRequiresParams(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	params := map[string]any{"action": "compare"}
	paramsJSON, _ := json.Marshal(params)

	_, err := sm.HandleTool(json.RawMessage(paramsJSON))
	if err == nil {
		t.Error("Expected error when compare has no compare_a/compare_b")
	}
}

func TestHandleDiffSessions_URLFilter(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		pageURL: "http://localhost:3000",
		networkRequests: []SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200},
			{Method: "GET", URL: "/static/app.js", Status: 200},
		},
	}
	sm := NewSessionManager(10, mock)

	params := map[string]any{
		"action": "capture",
		"name":   "filtered",
		"url":    "/api/",
	}
	paramsJSON, _ := json.Marshal(params)

	sm.HandleTool(json.RawMessage(paramsJSON))

	list := sm.List()
	if len(list) != 1 {
		t.Fatalf("Expected 1 snapshot, got %d", len(list))
	}
}

// ============================================
// Test: Network Matching by Method+URLPath
// ============================================

func TestSessionManager_CompareMatchesByMethodAndURLPath(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Same URL path, different query params — should match
	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users?page=1", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users?page=2", Status: 200},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	// Should NOT show as new/missing since path matches
	if len(diff.Network.NewEndpoints) != 0 {
		t.Errorf("Expected 0 new endpoints (same path), got %d", len(diff.Network.NewEndpoints))
	}
	if len(diff.Network.MissingEndpoints) != 0 {
		t.Errorf("Expected 0 missing endpoints (same path), got %d", len(diff.Network.MissingEndpoints))
	}
}

func TestSessionManager_CompareDifferentMethodsSameURL(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []SnapshotNetworkRequest{
		{Method: "POST", URL: "/api/users", Status: 201},
	}
	sm.Capture("after", "")

	diff, err := sm.Compare("before", "after")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	// Different methods = different endpoints
	if len(diff.Network.NewEndpoints) != 1 {
		t.Errorf("Expected 1 new endpoint (POST), got %d", len(diff.Network.NewEndpoints))
	}
	if len(diff.Network.MissingEndpoints) != 1 {
		t.Errorf("Expected 1 missing endpoint (GET), got %d", len(diff.Network.MissingEndpoints))
	}
}

// ============================================
// Test: Snapshot Limits
// ============================================

func TestSessionManager_ConsoleEntriesLimit(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Create more than 50 errors
	errors := make([]SnapshotError, 60)
	for i := range errors {
		errors[i] = SnapshotError{Type: "console", Message: "Error " + string(rune('A'+i%26)), Count: 1}
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
	requests := make([]SnapshotNetworkRequest, 120)
	for i := range requests {
		requests[i] = SnapshotNetworkRequest{Method: "GET", URL: "/api/" + string(rune('a'+i%26)), Status: 200}
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
