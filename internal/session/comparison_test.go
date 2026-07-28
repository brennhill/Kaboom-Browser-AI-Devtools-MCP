// Purpose: Tests for session state comparison and diff output.
// Docs: docs/features/feature/request-session-correlation/index.md

// comparison_test.go — Tests for comparison.go Compare function.
// Covers: snapshot lookup errors, "current" reserved name, full diff result fields.
package session

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

// ============================================
// Compare: Snapshot Not Found
// ============================================

func TestCompare_SnapshotANotFound(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	_, err := sm.Compare("nonexistent-a", "nonexistent-b")
	if err == nil {
		t.Fatal("Expected error when snapshot A not found")
	}
	if !strings.Contains(err.Error(), "nonexistent-a") {
		t.Errorf("Error should mention snapshot name, got: %v", err)
	}
}

func TestCompare_SnapshotBNotFound(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("exists", "")
	_, err := sm.Compare("exists", "nonexistent-b")
	if err == nil {
		t.Fatal("Expected error when snapshot B not found")
	}
	if !strings.Contains(err.Error(), "nonexistent-b") {
		t.Errorf("Error should mention snapshot name, got: %v", err)
	}
}

// ============================================
// Compare: Result Fields
// ============================================

func TestCompare_ResultFieldsAB(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("snap-a", "")
	sm.Capture("snap-b", "")

	result, err := sm.Compare("snap-a", "snap-b")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.A != "snap-a" {
		t.Errorf("Expected A='snap-a', got %q", result.A)
	}
	if result.B != "snap-b" {
		t.Errorf("Expected B='snap-b', got %q", result.B)
	}
}

func TestCompare_AgainstCurrentBuildsLiveSnapshot(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		consoleErrors: []types.SnapshotError{
			{Type: "console", Message: "base error", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}
	sm := NewSessionManager(10, mock)
	sm.Capture("baseline", "")

	// Mutate mock to represent "current" state
	mock.consoleErrors = []types.SnapshotError{
		{Type: "console", Message: "base error", Count: 1},
		{Type: "console", Message: "new live error", Count: 1},
	}

	result, err := sm.Compare("baseline", "current")
	if err != nil {
		t.Fatalf("Compare vs current failed: %v", err)
	}

	if result.B != "current" {
		t.Errorf("Expected B='current', got %q", result.B)
	}
	if len(result.Errors.New) != 1 {
		t.Fatalf("Expected 1 new error, got %d", len(result.Errors.New))
	}
	if result.Errors.New[0].Message != "new live error" {
		t.Errorf("Expected 'new live error', got %q", result.Errors.New[0].Message)
	}
}

func TestCompare_CurrentUsesURLFilterFromSnapshotA(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/data", Status: 200},
			{Method: "GET", URL: "/static/app.js", Status: 200},
		},
		pageURL: "http://localhost:3000",
	}
	sm := NewSessionManager(10, mock)

	// Capture with URL filter
	sm.Capture("filtered-snap", "/api/")

	// Add a new /api/ endpoint to the live state
	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/data", Status: 200},
		{Method: "GET", URL: "/api/new-endpoint", Status: 200},
		{Method: "GET", URL: "/static/app.js", Status: 200},
		{Method: "GET", URL: "/static/extra.css", Status: 200},
	}

	result, err := sm.Compare("filtered-snap", "current")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	// The /static/ endpoints should not appear in new endpoints
	// since the URL filter from snap A should be applied to "current"
	for _, ep := range result.Network.NewEndpoints {
		if strings.HasPrefix(ep.URL, "/static/") {
			t.Errorf("Static endpoint should be filtered out, got %q", ep.URL)
		}
	}
}

// ============================================
// Compare: Full Diff Structure
// ============================================

func TestCompare_FullDiffStructure(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Snapshot A: errors, network, performance
	mock.consoleErrors = []types.SnapshotError{
		{Type: "console", Message: "old error", Count: 1},
	}
	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 200, Duration: 100},
		{Method: "POST", URL: "/api/data", Status: 200, Duration: 50},
	}
	mock.performance = &performance.PerformanceSnapshot{
		Timing:  performance.PerformanceTiming{Load: 1000},
		Network: performance.NetworkSummary{RequestCount: 10, TransferSize: 50000},
	}
	sm.Capture("a", "")

	// Snapshot B: different errors, different network, worse performance
	mock.consoleErrors = []types.SnapshotError{
		{Type: "console", Message: "new error", Count: 2},
	}
	mock.networkRequests = []types.SnapshotNetworkRequest{
		{Method: "GET", URL: "/api/users", Status: 500, Duration: 300},
		{Method: "GET", URL: "/api/new-thing", Status: 404},
	}
	mock.performance = &performance.PerformanceSnapshot{
		Timing:  performance.PerformanceTiming{Load: 5000},
		Network: performance.NetworkSummary{RequestCount: 30, TransferSize: 200000},
	}
	sm.Capture("b", "")

	result, err := sm.Compare("a", "b")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	// Errors: old resolved, new appeared
	if len(result.Errors.New) != 1 {
		t.Errorf("Expected 1 new error, got %d", len(result.Errors.New))
	}
	if len(result.Errors.Resolved) != 1 {
		t.Errorf("Expected 1 resolved error, got %d", len(result.Errors.Resolved))
	}

	// Network: /api/data missing, /api/new-thing new, /api/users status changed
	if len(result.Network.StatusChanges) != 1 {
		t.Errorf("Expected 1 status change, got %d", len(result.Network.StatusChanges))
	}
	if len(result.Network.MissingEndpoints) != 1 {
		t.Errorf("Expected 1 missing endpoint, got %d", len(result.Network.MissingEndpoints))
	}

	// Performance: load time regressed (1000 -> 5000 = 5x, > 1.5x threshold)
	if result.Performance.LoadTime == nil {
		t.Fatal("Expected load time metric")
	}
	if result.Performance.LoadTime.Before != 1000 {
		t.Errorf("Expected load time before=1000, got %v", result.Performance.LoadTime.Before)
	}
	if result.Performance.LoadTime.After != 5000 {
		t.Errorf("Expected load time after=5000, got %v", result.Performance.LoadTime.After)
	}
	if !result.Performance.LoadTime.Regression {
		t.Error("Expected load time regression=true")
	}

	// Summary: mixed (resolved errors + new errors + network regression + perf regression)
	if result.Summary.Verdict != "mixed" {
		t.Errorf("Expected verdict 'mixed', got %q", result.Summary.Verdict)
	}
	if result.Summary.NewErrors != 1 {
		t.Errorf("Expected new_errors=1, got %d", result.Summary.NewErrors)
	}
	if result.Summary.ResolvedErrors != 1 {
		t.Errorf("Expected resolved_errors=1, got %d", result.Summary.ResolvedErrors)
	}
	if result.Summary.PerformanceRegressions < 1 {
		t.Errorf("Expected at least 1 performance regression, got %d", result.Summary.PerformanceRegressions)
	}
}

// ============================================
// Compare: Empty Snapshots
// ============================================

func TestCompare_BothSnapshotsEmpty(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("empty-a", "")
	sm.Capture("empty-b", "")

	result, err := sm.Compare("empty-a", "empty-b")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(result.Errors.New) != 0 {
		t.Errorf("Expected 0 new errors, got %d", len(result.Errors.New))
	}
	if len(result.Errors.Resolved) != 0 {
		t.Errorf("Expected 0 resolved errors, got %d", len(result.Errors.Resolved))
	}
	if len(result.Errors.Unchanged) != 0 {
		t.Errorf("Expected 0 unchanged errors, got %d", len(result.Errors.Unchanged))
	}
	if len(result.Network.NewEndpoints) != 0 {
		t.Errorf("Expected 0 new endpoints, got %d", len(result.Network.NewEndpoints))
	}
	if len(result.Network.MissingEndpoints) != 0 {
		t.Errorf("Expected 0 missing endpoints, got %d", len(result.Network.MissingEndpoints))
	}
	if len(result.Network.StatusChanges) != 0 {
		t.Errorf("Expected 0 status changes, got %d", len(result.Network.StatusChanges))
	}
	if result.Performance.LoadTime != nil {
		t.Error("Expected nil load time for no performance data")
	}
	if result.Summary.Verdict != "unchanged" {
		t.Errorf("Expected verdict 'unchanged', got %q", result.Summary.Verdict)
	}
}

// ============================================
// Compare: Identical Snapshots
// ============================================

func TestCompare_IdenticalSnapshots(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		consoleErrors: []types.SnapshotError{
			{Type: "console", Message: "persists", Count: 5},
		},
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/health", Status: 200, Duration: 10},
		},
		performance: &performance.PerformanceSnapshot{
			Timing:  performance.PerformanceTiming{Load: 800},
			Network: performance.NetworkSummary{RequestCount: 5, TransferSize: 10000},
		},
		pageURL: "http://localhost:3000",
	}
	sm := NewSessionManager(10, mock)

	sm.Capture("snap-1", "")
	sm.Capture("snap-2", "")

	result, err := sm.Compare("snap-1", "snap-2")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(result.Errors.New) != 0 {
		t.Errorf("Expected 0 new errors for identical snapshots, got %d", len(result.Errors.New))
	}
	if len(result.Errors.Resolved) != 0 {
		t.Errorf("Expected 0 resolved errors, got %d", len(result.Errors.Resolved))
	}
	if len(result.Errors.Unchanged) != 1 {
		t.Errorf("Expected 1 unchanged error, got %d", len(result.Errors.Unchanged))
	}
	if result.Summary.Verdict != "unchanged" {
		t.Errorf("Expected verdict 'unchanged', got %q", result.Summary.Verdict)
	}
}
