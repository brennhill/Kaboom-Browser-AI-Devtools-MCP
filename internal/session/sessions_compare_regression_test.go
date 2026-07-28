// sessions_compare_regression_test.go — Tests snapshot comparison and verdict behavior.
// Docs: docs/features/feature/historical-snapshots/index.md

package session

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session/snapdiff"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// Test: Compare Snapshots
// ============================================

func TestSessionManager_CompareDetectsNewErrors(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Snapshot A: no errors
	mock.consoleErrors = []types.SnapshotError{}
	sm.Capture("before", "")

	// Snapshot B: has errors
	mock.consoleErrors = []types.SnapshotError{
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
	mock.consoleErrors = []types.SnapshotError{
		{Type: "console", Message: "TypeError: x is null", Count: 1},
		{Type: "console", Message: "ReferenceError: y is not defined", Count: 1},
	}
	sm.Capture("before", "")

	// Snapshot B: one error resolved
	mock.consoleErrors = []types.SnapshotError{
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

	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []types.SnapshotNetworkRequest{
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

	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
		{Method: "GET", URL: "/api/legacy", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []types.SnapshotNetworkRequest{
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

	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/dashboard", Status: 200, Duration: 100},
	}
	sm.Capture("before", "")

	mock.networkRequests = []types.SnapshotNetworkRequest{
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

	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []types.SnapshotNetworkRequest{
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
	mock.consoleErrors = []types.SnapshotError{
		{Type: "console", Message: "Error A", Count: 1},
	}
	sm.Capture("saved", "")

	// Change mock state to simulate "current"
	mock.consoleErrors = []types.SnapshotError{
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

	mock.consoleErrors = []types.SnapshotError{
		{Type: "console", Message: "Error one", Count: 1},
		{Type: "console", Message: "Error two", Count: 1},
	}
	sm.Capture("before", "")

	mock.consoleErrors = []types.SnapshotError{}
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

	mock.consoleErrors = []types.SnapshotError{}
	sm.Capture("before", "")

	mock.consoleErrors = []types.SnapshotError{
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

	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/data", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []types.SnapshotNetworkRequest{
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

	mock.consoleErrors = []types.SnapshotError{
		{Type: "console", Message: "Error A", Count: 1},
	}
	mock.networkRequests = []types.SnapshotNetworkRequest{
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

	mock.consoleErrors = []types.SnapshotError{
		{Type: "console", Message: "Old error", Count: 1},
	}
	sm.Capture("before", "")

	// Old error resolved, but new one appeared
	mock.consoleErrors = []types.SnapshotError{
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

	mock.consoleErrors = []types.SnapshotError{}
	sm.Capture("a", "")
	mock.consoleErrors = []types.SnapshotError{{Type: "console", Message: "err", Count: 1}}
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
// ============================================
// Test: Network Matching by Method+URLPath
// ============================================

func TestSessionManager_CompareMatchesByMethodAndURLPath(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Same URL path, different query params — should match
	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users?page=1", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []types.SnapshotNetworkRequest{
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

	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200},
	}
	sm.Capture("before", "")

	mock.networkRequests = []types.SnapshotNetworkRequest{
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
