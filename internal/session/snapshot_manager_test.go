// Purpose: Tests for session snapshot capture and restoration.
// Docs: docs/features/feature/request-session-correlation/index.md

// snapshot_manager_test.go — Tests for snapshot-manager.go.
// Covers: NewSessionManager defaults, Capture, captureCurrentState,
// List ordering, Delete, eviction, concurrent operations.
package session

import (
	"fmt"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

type performanceSampleCapture struct {
	*mockCaptureState
	samples []performance.PerformanceSnapshot
}

func (capture *performanceSampleCapture) GetPerformanceSamples() []performance.PerformanceSnapshot {
	return capture.samples
}

func TestSessionManagerCapturesRepeatedPerformanceSamplesForCurrentPage(t *testing.T) {
	t.Parallel()
	latest := &performance.PerformanceSnapshot{URL: "/app", Timing: performance.PerformanceTiming{Load: 300}}
	reader := &performanceSampleCapture{
		mockCaptureState: &mockCaptureState{performance: latest, pageURL: "/app"},
		samples: []performance.PerformanceSnapshot{
			{URL: "/other", Timing: performance.PerformanceTiming{Load: 50}},
			{URL: "/app", Timing: performance.PerformanceTiming{Load: 100}},
			{URL: "/app", Timing: performance.PerformanceTiming{Load: 300}},
		},
	}
	snapshot, err := NewSessionManager(10, reader).Capture("profile", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.PerformanceSamples) != 2 || snapshot.PerformanceSamples[0].Timing.Load != 100 {
		t.Fatalf("page-scoped performance samples = %+v", snapshot.PerformanceSamples)
	}
}

// ============================================
// NewSessionManager
// ============================================

func TestNewSessionManager_DefaultMaxSnapshots(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}

	// maxSnapshots <= 0 should default to 10
	sm := NewSessionManager(0, mock)
	if sm.maxSize != 10 {
		t.Errorf("Expected default maxSize=10, got %d", sm.maxSize)
	}

	sm2 := NewSessionManager(-5, mock)
	if sm2.maxSize != 10 {
		t.Errorf("Expected default maxSize=10 for negative input, got %d", sm2.maxSize)
	}
}

func TestNewSessionManager_CustomMaxSnapshots(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}

	sm := NewSessionManager(5, mock)
	if sm.maxSize != 5 {
		t.Errorf("Expected maxSize=5, got %d", sm.maxSize)
	}
}

func TestNewSessionManager_InitialState(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	if sm.snaps == nil {
		t.Fatal("snaps map should be initialized")
	}
	if sm.order == nil {
		t.Fatal("order slice should be initialized")
	}
	if len(sm.snaps) != 0 {
		t.Errorf("Expected empty snaps map, got %d entries", len(sm.snaps))
	}
	if len(sm.order) != 0 {
		t.Errorf("Expected empty order slice, got %d entries", len(sm.order))
	}
}

// ============================================
// Capture: Snapshot Fields
// ============================================

func TestCapture_AllFieldsPopulated(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		consoleErrors: []types.SnapshotError{
			{Type: "error", Message: "Uncaught TypeError", Count: 3},
		},
		consoleWarnings: []types.SnapshotError{
			{Type: "warning", Message: "Deprecation warning", Count: 1},
		},
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/data", Status: 200, Duration: 45, ResponseSize: 1024, ContentType: "application/json"},
		},
		wsConnections: []types.SnapshotWSConnection{
			{URL: "ws://localhost:8080/ws", State: "open", MessageRate: 5.2},
		},
		performance: &performance.PerformanceSnapshot{
			URL:    "http://localhost:3000",
			Timing: performance.PerformanceTiming{Load: 1500, TimeToFirstByte: 100},
			Network: performance.NetworkSummary{
				RequestCount: 8,
				TransferSize: 120000,
			},
		},
		pageURL: "http://localhost:3000/dashboard",
	}
	sm := NewSessionManager(10, mock)

	snap, err := sm.Capture("full-snapshot", "")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	// Name
	if snap.Name != "full-snapshot" {
		t.Errorf("Expected name 'full-snapshot', got %q", snap.Name)
	}

	// CapturedAt
	if snap.CapturedAt.IsZero() {
		t.Error("CapturedAt should not be zero")
	}

	// PageURL
	if snap.PageURL != "http://localhost:3000/dashboard" {
		t.Errorf("Expected pageURL 'http://localhost:3000/dashboard', got %q", snap.PageURL)
	}

	// Console errors
	if len(snap.ConsoleErrors) != 1 {
		t.Fatalf("Expected 1 console error, got %d", len(snap.ConsoleErrors))
	}
	if snap.ConsoleErrors[0].Type != "error" {
		t.Errorf("Expected error type 'error', got %q", snap.ConsoleErrors[0].Type)
	}
	if snap.ConsoleErrors[0].Message != "Uncaught TypeError" {
		t.Errorf("Expected message 'Uncaught TypeError', got %q", snap.ConsoleErrors[0].Message)
	}
	if snap.ConsoleErrors[0].Count != 3 {
		t.Errorf("Expected count=3, got %d", snap.ConsoleErrors[0].Count)
	}

	// Console warnings
	if len(snap.ConsoleWarnings) != 1 {
		t.Fatalf("Expected 1 console warning, got %d", len(snap.ConsoleWarnings))
	}
	if snap.ConsoleWarnings[0].Count != 1 {
		t.Errorf("Expected warning count=1, got %d", snap.ConsoleWarnings[0].Count)
	}

	// Network requests
	if len(snap.NetworkRequests) != 1 {
		t.Fatalf("Expected 1 network request, got %d", len(snap.NetworkRequests))
	}
	nr := snap.NetworkRequests[0]
	if nr.Method != "GET" || nr.URL != "/api/data" || nr.Status != 200 || nr.Duration != 45 {
		t.Errorf("Network request fields wrong: %+v", nr)
	}
	if nr.ResponseSize != 1024 {
		t.Errorf("Expected response_size=1024, got %d", nr.ResponseSize)
	}
	if nr.ContentType != "application/json" {
		t.Errorf("Expected content_type='application/json', got %q", nr.ContentType)
	}

	// WebSocket connections
	if len(snap.WebSocketConnections) != 1 {
		t.Fatalf("Expected 1 WS connection, got %d", len(snap.WebSocketConnections))
	}
	ws := snap.WebSocketConnections[0]
	if ws.URL != "ws://localhost:8080/ws" || ws.State != "open" || ws.MessageRate != 5.2 {
		t.Errorf("WS connection fields wrong: %+v", ws)
	}

	// Performance (deep copy check)
	if snap.Performance == nil {
		t.Fatal("Performance should not be nil")
	}
	if snap.Performance.Timing.Load != 1500 {
		t.Errorf("Expected load=1500, got %v", snap.Performance.Timing.Load)
	}
	if snap.Performance.Timing.TimeToFirstByte != 100 {
		t.Errorf("Expected TTFB=100, got %v", snap.Performance.Timing.TimeToFirstByte)
	}

	// Verify deep copy - mutate original should not affect snapshot
	mock.performance.Timing.Load = 9999
	if snap.Performance.Timing.Load == 9999 {
		t.Error("Performance should be deep copied, not a reference")
	}
}

func TestCapture_NilPerformance(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		pageURL:     "http://localhost:3000",
		performance: nil,
	}
	sm := NewSessionManager(10, mock)

	snap, err := sm.Capture("no-perf", "")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if snap.Performance != nil {
		t.Error("Expected nil performance when reader returns nil")
	}
}

// ============================================
// Capture: URL Filter
// ============================================

func TestCapture_URLFilterMatchesSubstring(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "https://example.com/api/v1/users", Status: 200},
			{Method: "GET", URL: "https://example.com/api/v2/users", Status: 200},
			{Method: "GET", URL: "https://cdn.example.com/style.css", Status: 200},
			{Method: "GET", URL: "https://example.com/images/logo.png", Status: 200},
		},
		pageURL: "http://localhost:3000",
	}
	sm := NewSessionManager(10, mock)

	snap, err := sm.Capture("api-filter", "/api/")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if len(snap.NetworkRequests) != 2 {
		t.Errorf("Expected 2 filtered requests, got %d", len(snap.NetworkRequests))
	}
	if snap.URLFilter != "/api/" {
		t.Errorf("Expected URLFilter='/api/', got %q", snap.URLFilter)
	}
}

func TestCapture_EmptyURLFilterKeepsAll(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		networkRequests: []types.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200},
			{Method: "GET", URL: "/static/app.js", Status: 200},
		},
		pageURL: "http://localhost:3000",
	}
	sm := NewSessionManager(10, mock)

	snap, err := sm.Capture("no-filter", "")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if len(snap.NetworkRequests) != 2 {
		t.Errorf("Expected 2 requests with empty filter, got %d", len(snap.NetworkRequests))
	}
}

// ============================================
// Capture: Eviction
// ============================================

func TestCapture_EvictsOldestWhenAtCapacity(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(2, mock)

	sm.Capture("first", "")
	sm.Capture("second", "")
	sm.Capture("third", "")

	list := sm.List()
	if len(list) != 2 {
		t.Fatalf("Expected 2 snapshots at max capacity, got %d", len(list))
	}

	// "first" should be evicted, "second" and "third" remain
	names := make(map[string]bool)
	for _, entry := range list {
		names[entry.Name] = true
	}
	if names["first"] {
		t.Error("Expected 'first' to be evicted")
	}
	if !names["second"] {
		t.Error("Expected 'second' to remain")
	}
	if !names["third"] {
		t.Error("Expected 'third' to remain")
	}
}

func TestCapture_OverwriteDoesNotEvict(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(2, mock)

	sm.Capture("snap-a", "")
	sm.Capture("snap-b", "")

	// Overwriting snap-a should not trigger eviction
	mock.consoleErrors = []types.SnapshotError{{Type: "error", Message: "updated", Count: 1}}
	snap, err := sm.Capture("snap-a", "")
	if err != nil {
		t.Fatalf("Overwrite capture failed: %v", err)
	}

	list := sm.List()
	if len(list) != 2 {
		t.Fatalf("Expected 2 snapshots, got %d", len(list))
	}

	// snap-a should now be at the end of order (re-inserted)
	if list[0].Name != "snap-b" {
		t.Errorf("Expected snap-b first in order, got %q", list[0].Name)
	}
	if list[1].Name != "snap-a" {
		t.Errorf("Expected snap-a second in order, got %q", list[1].Name)
	}
	if snap.ConsoleErrors[0].Message != "updated" {
		t.Errorf("Expected overwritten message 'updated', got %q", snap.ConsoleErrors[0].Message)
	}
}

func TestCapture_EvictionChainMultiple(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(2, mock)

	// Fill capacity
	sm.Capture("a", "")
	sm.Capture("b", "")

	// Add 3 more, should evict a, then b, then c
	sm.Capture("c", "")
	sm.Capture("d", "")
	sm.Capture("e", "")

	list := sm.List()
	if len(list) != 2 {
		t.Fatalf("Expected 2 snapshots, got %d", len(list))
	}

	names := make(map[string]bool)
	for _, entry := range list {
		names[entry.Name] = true
	}
	if !names["d"] || !names["e"] {
		t.Errorf("Expected d and e to remain, got %v", names)
	}
}

// ============================================
// Capture: Console/Network Limits
// ============================================

func TestCapture_ConsoleErrorsLimit(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	errors := make([]types.SnapshotError, 60)
	for i := range errors {
		errors[i] = types.SnapshotError{Type: "console", Message: fmt.Sprintf("err-%d", i), Count: 1}
	}
	mock.consoleErrors = errors

	snap, err := sm.Capture("limited", "")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if len(snap.ConsoleErrors) != maxConsolePerSnapshot {
		t.Errorf("Expected %d console errors (max), got %d", maxConsolePerSnapshot, len(snap.ConsoleErrors))
	}
}

func TestCapture_ConsoleWarningsLimit(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	warnings := make([]types.SnapshotError, 60)
	for i := range warnings {
		warnings[i] = types.SnapshotError{Type: "warning", Message: fmt.Sprintf("warn-%d", i), Count: 1}
	}
	mock.consoleWarnings = warnings

	snap, err := sm.Capture("limited-warn", "")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if len(snap.ConsoleWarnings) != maxConsolePerSnapshot {
		t.Errorf("Expected %d console warnings (max), got %d", maxConsolePerSnapshot, len(snap.ConsoleWarnings))
	}
}

func TestCapture_NetworkRequestsLimit(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	requests := make([]types.SnapshotNetworkRequest, 120)
	for i := range requests {
		requests[i] = types.SnapshotNetworkRequest{Method: "GET", URL: fmt.Sprintf("/api/%d", i), Status: 200}
	}
	mock.networkRequests = requests

	snap, err := sm.Capture("limited-net", "")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if len(snap.NetworkRequests) != maxNetworkPerSnapshot {
		t.Errorf("Expected %d network requests (max), got %d", maxNetworkPerSnapshot, len(snap.NetworkRequests))
	}
}

// ============================================
// List: Ordering and Metadata
// ============================================

func TestList_InsertionOrder(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("alpha", "")
	sm.Capture("beta", "")
	sm.Capture("gamma", "")

	list := sm.List()
	if len(list) != 3 {
		t.Fatalf("Expected 3, got %d", len(list))
	}

	expected := []string{"alpha", "beta", "gamma"}
	for i, name := range expected {
		if list[i].Name != name {
			t.Errorf("Expected order[%d]=%q, got %q", i, name, list[i].Name)
		}
	}
}

func TestList_EntryMetadata(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		consoleErrors: []types.SnapshotError{
			{Type: "console", Message: "err-1", Count: 1},
			{Type: "console", Message: "err-2", Count: 1},
		},
		pageURL: "http://localhost:3000/app",
	}
	sm := NewSessionManager(10, mock)
	sm.Capture("meta-test", "")

	list := sm.List()
	if len(list) != 1 {
		t.Fatalf("Expected 1, got %d", len(list))
	}

	entry := list[0]
	if entry.Name != "meta-test" {
		t.Errorf("Expected name 'meta-test', got %q", entry.Name)
	}
	if entry.CapturedAt.IsZero() {
		t.Error("CapturedAt should not be zero")
	}
	if entry.PageURL != "http://localhost:3000/app" {
		t.Errorf("Expected page_url 'http://localhost:3000/app', got %q", entry.PageURL)
	}
	if entry.ErrorCount != 2 {
		t.Errorf("Expected error_count=2, got %d", entry.ErrorCount)
	}
}

// ============================================
// Delete
// ============================================

func TestDelete_MiddleElement(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("first", "")
	sm.Capture("middle", "")
	sm.Capture("last", "")

	err := sm.Delete("middle")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	list := sm.List()
	if len(list) != 2 {
		t.Fatalf("Expected 2 after delete, got %d", len(list))
	}

	// Order should be preserved
	if list[0].Name != "first" {
		t.Errorf("Expected first, got %q", list[0].Name)
	}
	if list[1].Name != "last" {
		t.Errorf("Expected last, got %q", list[1].Name)
	}
}

func TestDelete_AllSnapshots(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("one", "")
	sm.Capture("two", "")

	sm.Delete("one")
	sm.Delete("two")

	list := sm.List()
	if len(list) != 0 {
		t.Errorf("Expected empty list after deleting all, got %d", len(list))
	}
}

func TestDelete_NonExistent(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	err := sm.Delete("does-not-exist")
	if err == nil {
		t.Fatal("Expected error when deleting non-existent snapshot")
	}
}

// ============================================
// Concurrent Access
// ============================================

func TestSessionManager_ConcurrentCaptureDeleteList(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{
		pageURL:       "http://localhost:3000",
		consoleErrors: []types.SnapshotError{{Type: "error", Message: "err", Count: 1}},
	}
	sm := NewSessionManager(20, mock)

	var wg sync.WaitGroup

	// Concurrent captures
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := fmt.Sprintf("snap-%d", idx)
			sm.Capture(name, "")
		}(i)
	}

	// Concurrent lists
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.List()
		}()
	}

	// Concurrent deletes (some will fail, that's fine)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sm.Delete(fmt.Sprintf("snap-%d", idx))
		}(i)
	}

	wg.Wait()

	// No panics = success
	list := sm.List()
	if len(list) > 20 {
		t.Errorf("Expected at most 20 snapshots, got %d", len(list))
	}
}
