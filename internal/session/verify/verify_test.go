// Purpose: Tests for session state verification and integrity checks.
// Docs: docs/features/feature/pagination/index.md

// verify_test.go — Tests for the verify_fix MCP tool.
// Tests the verification loop: start (baseline) -> watch -> compare workflow.
// Covers session lifecycle, verdict determination, error normalization, and limits.
package verify

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
)

// ============================================
// Mock VerifyStateReader
// ============================================

// mockVerifyState implements session.CaptureStateReader for verification testing
type mockVerifyState struct {
	consoleErrors   []session.SnapshotError
	consoleWarnings []session.SnapshotError
	networkRequests []session.SnapshotNetworkRequest
	wsConnections   []session.SnapshotWSConnection
	performance     *performance.Snapshot
	pageURL         string
}

func (m *mockVerifyState) GetConsoleErrors() []session.SnapshotError {
	if m.consoleErrors == nil {
		return []session.SnapshotError{}
	}
	return m.consoleErrors
}

func (m *mockVerifyState) GetConsoleWarnings() []session.SnapshotError {
	if m.consoleWarnings == nil {
		return []session.SnapshotError{}
	}
	return m.consoleWarnings
}

func (m *mockVerifyState) GetNetworkRequests() []session.SnapshotNetworkRequest {
	if m.networkRequests == nil {
		return []session.SnapshotNetworkRequest{}
	}
	return m.networkRequests
}

func (m *mockVerifyState) GetWSConnections() []session.SnapshotWSConnection {
	if m.wsConnections == nil {
		return []session.SnapshotWSConnection{}
	}
	return m.wsConnections
}

func (m *mockVerifyState) GetPerformance() *performance.Snapshot {
	return m.performance
}

func (m *mockVerifyState) GetCurrentPageURL() string {
	return m.pageURL
}

// ============================================
// Test: Session Lifecycle - Start
// ============================================

func TestVerificationManager_Start(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Cannot read property 'user' of undefined", Count: 2},
			{Type: "console", Message: "Failed to load resource", Count: 1},
		},
		networkRequests: []session.SnapshotNetworkRequest{
			{Method: "POST", URL: "/api/login", Status: 500, Duration: 150},
			{Method: "GET", URL: "/api/users", Status: 200, Duration: 50},
		},
		performance: &performance.Snapshot{
			URL: "http://localhost:3000/login",
			Timing: performance.Timing{
				Load: 3200,
			},
		},
		pageURL: "http://localhost:3000/login",
	}

	vm := NewVerificationManager(mock)

	result, err := vm.Start("fix-login-error", "")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if result.VerifSessionID == "" {
		t.Error("Expected non-empty session ID")
	}
	if !strings.HasPrefix(result.VerifSessionID, "verify-") {
		t.Errorf("Session ID should have 'verify-' prefix, got %q", result.VerifSessionID)
	}
	if result.Status != "baseline_captured" {
		t.Errorf("Expected status 'baseline_captured', got %q", result.Status)
	}
	if result.Baseline.ConsoleErrors != 3 {
		t.Errorf("Expected 3 console errors in baseline, got %d", result.Baseline.ConsoleErrors)
	}
	if result.Baseline.NetworkErrors != 1 {
		t.Errorf("Expected 1 network error in baseline, got %d", result.Baseline.NetworkErrors)
	}
	if len(result.Baseline.ErrorDetails) != 3 {
		t.Errorf("Expected 3 error details, got %d", len(result.Baseline.ErrorDetails))
	}
}

func TestVerificationManager_StartWithLabel(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	result, err := vm.Start("fix-login-error", "")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if result.Label != "fix-login-error" {
		t.Errorf("Expected label 'fix-login-error', got %q", result.Label)
	}
}

func TestVerificationManager_StartWithURLFilter(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		networkRequests: []session.SnapshotNetworkRequest{
			{Method: "POST", URL: "/api/login", Status: 500},
			{Method: "GET", URL: "/api/users", Status: 200},
			{Method: "GET", URL: "/static/main.js", Status: 200},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)

	result, err := vm.Start("api-test", "/api/")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// URL filter should only count API requests
	if result.Baseline.NetworkErrors != 1 {
		t.Errorf("Expected 1 network error (filtered to /api/), got %d", result.Baseline.NetworkErrors)
	}
}

// ============================================
// Test: Session Lifecycle - Watch
// ============================================

func TestVerificationManager_Watch(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Error in baseline", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)

	// Start session
	startResult, err := vm.Start("test-watch", "")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Watch
	watchResult, err := vm.Watch(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	if watchResult.VerifSessionID != startResult.VerifSessionID {
		t.Errorf("Session IDs should match: %q != %q", watchResult.VerifSessionID, startResult.VerifSessionID)
	}
	if watchResult.Status != "watching" {
		t.Errorf("Expected status 'watching', got %q", watchResult.Status)
	}
	if watchResult.Message == "" {
		t.Error("Watch result should have a message")
	}
}

func TestVerificationManager_WatchWithoutStart(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.Watch("non-existent-session")
	if err == nil {
		t.Error("Expected error when watching non-existent session")
	}
}

func TestVerificationManager_WatchAlreadyWatching(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	startResult, _ := vm.Start("test", "")
	_, err := vm.Watch(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("First watch failed: %v", err)
	}

	// Calling watch again should still work (idempotent)
	_, err = vm.Watch(startResult.VerifSessionID)
	if err != nil {
		t.Errorf("Second watch should be idempotent, got: %v", err)
	}
}

// ============================================
// Test: Session Lifecycle - Compare
// ============================================

func TestVerificationManager_Compare_Fixed(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Cannot read property 'user' of undefined", Count: 2},
		},
		networkRequests: []session.SnapshotNetworkRequest{
			{Method: "POST", URL: "/api/login", Status: 500},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)

	// Start (capture broken baseline)
	startResult, _ := vm.Start("fix-test", "")

	// Simulate fix: clear all errors
	mock.consoleErrors = nil
	mock.networkRequests = []session.SnapshotNetworkRequest{
		{Method: "POST", URL: "/api/login", Status: 200},
	}

	// Watch
	vm.Watch(startResult.VerifSessionID)

	// Compare
	result, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.Status != "compared" {
		t.Errorf("Expected status 'compared', got %q", result.Status)
	}
	if result.Result.Verdict != "fixed" {
		t.Errorf("Expected verdict 'fixed', got %q", result.Result.Verdict)
	}
	if result.Result.Before.ConsoleErrors != 2 {
		t.Errorf("Expected 2 console errors before, got %d", result.Result.Before.ConsoleErrors)
	}
	if result.Result.After.ConsoleErrors != 0 {
		t.Errorf("Expected 0 console errors after, got %d", result.Result.After.ConsoleErrors)
	}

	// Check changes list
	hasResolved := false
	for _, change := range result.Result.Changes {
		if change.Type == "resolved" {
			hasResolved = true
			break
		}
	}
	if !hasResolved {
		t.Error("Expected at least one resolved change")
	}
}

func TestVerificationManager_Compare_Improved(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Error A", Count: 1},
			{Type: "console", Message: "Error B", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)
	startResult, _ := vm.Start("improve-test", "")

	// Simulate partial fix: only one error remains
	mock.consoleErrors = []session.SnapshotError{
		{Type: "console", Message: "Error B", Count: 1},
	}

	vm.Watch(startResult.VerifSessionID)
	result, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.Result.Verdict != "improved" {
		t.Errorf("Expected verdict 'improved', got %q", result.Result.Verdict)
	}
}

func TestVerificationManager_Compare_Unchanged(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Persistent error", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)
	startResult, _ := vm.Start("unchanged-test", "")

	// Same errors persist
	vm.Watch(startResult.VerifSessionID)
	result, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.Result.Verdict != "unchanged" {
		t.Errorf("Expected verdict 'unchanged', got %q", result.Result.Verdict)
	}
}

func TestVerificationManager_Compare_DifferentIssue(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Original error", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)
	startResult, _ := vm.Start("different-test", "")

	// Original fixed but new error appeared
	mock.consoleErrors = []session.SnapshotError{
		{Type: "console", Message: "New different error", Count: 1},
	}

	vm.Watch(startResult.VerifSessionID)
	result, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.Result.Verdict != "different_issue" {
		t.Errorf("Expected verdict 'different_issue', got %q", result.Result.Verdict)
	}
	if len(result.Result.NewIssues) != 1 {
		t.Errorf("Expected 1 new issue, got %d", len(result.Result.NewIssues))
	}
}

func TestVerificationManager_Compare_Regressed(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Error A", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)
	startResult, _ := vm.Start("regressed-test", "")

	// More errors than before
	mock.consoleErrors = []session.SnapshotError{
		{Type: "console", Message: "Error A", Count: 1},
		{Type: "console", Message: "Error B", Count: 1},
		{Type: "console", Message: "Error C", Count: 1},
	}

	vm.Watch(startResult.VerifSessionID)
	result, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.Result.Verdict != "regressed" {
		t.Errorf("Expected verdict 'regressed', got %q", result.Result.Verdict)
	}
}

func TestVerificationManager_Compare_NoIssuesDetected(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: nil, // No errors
		networkRequests: []session.SnapshotNetworkRequest{
			{Method: "GET", URL: "/api/users", Status: 200},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)
	startResult, _ := vm.Start("no-issues-test", "")

	vm.Watch(startResult.VerifSessionID)
	result, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.Result.Verdict != "no_issues_detected" {
		t.Errorf("Expected verdict 'no_issues_detected', got %q", result.Result.Verdict)
	}
}

func TestVerificationManager_CompareWithoutWatch(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	startResult, _ := vm.Start("no-watch-test", "")

	// Compare without watching should fail
	_, err := vm.Compare(startResult.VerifSessionID)
	if err == nil {
		t.Error("Expected error when comparing without watch")
	}
	if !strings.Contains(err.Error(), "watch") {
		t.Errorf("Error should mention watch requirement: %v", err)
	}
}

func TestVerificationManager_CompareNonExistent(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.Compare("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}
}

// ============================================
// Test: Session Lifecycle - Cancel
// ============================================

func TestVerificationManager_Cancel(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	startResult, _ := vm.Start("cancel-test", "")

	result, err := vm.Cancel(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	if result.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got %q", result.Status)
	}

	// Trying to watch should now fail
	_, err = vm.Watch(startResult.VerifSessionID)
	if err == nil {
		t.Error("Expected error watching cancelled session")
	}
}

func TestVerificationManager_CancelNonExistent(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.Cancel("non-existent")
	if err == nil {
		t.Error("Expected error cancelling non-existent session")
	}
}

// ============================================
// Test: Session Lifecycle - Status
// ============================================

func TestVerificationManager_Status(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	startResult, _ := vm.Start("status-test", "")

	status, err := vm.Status(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.VerifSessionID != startResult.VerifSessionID {
		t.Errorf("Session ID mismatch")
	}
	if status.Status != "baseline_captured" {
		t.Errorf("Expected 'baseline_captured', got %q", status.Status)
	}
}

func TestVerificationManager_StatusAfterWatch(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	startResult, _ := vm.Start("status-watch-test", "")
	vm.Watch(startResult.VerifSessionID)

	status, err := vm.Status(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if status.Status != "watching" {
		t.Errorf("Expected 'watching', got %q", status.Status)
	}
}

func TestVerificationManager_StatusNonExistent(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.Status("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}
}

// ============================================
// Test: Error Normalization
// ============================================

func TestNormalizeVerifyErrorMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		// UUIDs should be normalized
		{
			input:    "Failed to load user 550e8400-e29b-41d4-a716-446655440000",
			expected: "Failed to load user [uuid]",
		},
		// Numeric IDs should be normalized
		{
			input:    "Cannot find order with id 12345",
			expected: "Cannot find order with id [id]",
		},
		// Line numbers should be normalized
		{
			input:    "Error at app.js:42",
			expected: "Error at [file]",
		},
		// Timestamps should be normalized
		{
			input:    "Request failed at 2025-01-20T14:30:00Z",
			expected: "Request failed at [timestamp]",
		},
		// Multiple normalizations
		{
			input:    "User 12345 not found at 2025-01-20T14:30:00Z",
			expected: "User [id] not found at [timestamp]",
		},
		// No changes needed
		{
			input:    "Cannot read property 'user' of undefined",
			expected: "Cannot read property 'user' of undefined",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeVerifyErrorMessage(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeErrorMessage(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// ============================================
// Test: Error Matching in Comparison
// ============================================

func TestVerificationManager_ErrorMatchingNormalized(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Failed to load user 550e8400-e29b-41d4-a716-446655440000", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)
	startResult, _ := vm.Start("normalization-test", "")

	// Same error but different UUID
	mock.consoleErrors = []session.SnapshotError{
		{Type: "console", Message: "Failed to load user a1b2c3d4-e5f6-7890-abcd-ef1234567890", Count: 1},
	}

	vm.Watch(startResult.VerifSessionID)
	result, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	// Should be unchanged because normalized messages match
	if result.Result.Verdict != "unchanged" {
		t.Errorf("Expected verdict 'unchanged' (normalized match), got %q", result.Result.Verdict)
	}
}

// ============================================
// Test: Network Status Changes
// ============================================

func TestVerificationManager_NetworkStatusChange(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		networkRequests: []session.SnapshotNetworkRequest{
			{Method: "POST", URL: "/api/login", Status: 500},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)
	startResult, _ := vm.Start("network-test", "")

	// Fix: endpoint now returns 200
	mock.networkRequests = []session.SnapshotNetworkRequest{
		{Method: "POST", URL: "/api/login", Status: 200},
	}

	vm.Watch(startResult.VerifSessionID)
	result, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	// Check that network change was detected
	foundChange := false
	for _, change := range result.Result.Changes {
		if change.Category == "network" && change.Type == "resolved" {
			foundChange = true
			if !strings.Contains(change.Before, "500") {
				t.Errorf("Before should mention 500 status: %s", change.Before)
			}
			if !strings.Contains(change.After, "200") {
				t.Errorf("After should mention 200 status: %s", change.After)
			}
		}
	}
	if !foundChange {
		t.Error("Expected to find network change from 500 to 200")
	}
}

// ============================================
// Test: Performance Diff
// ============================================

func TestVerificationManager_PerformanceDiff(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		performance: &performance.Snapshot{
			URL: "http://localhost:3000",
			Timing: performance.Timing{
				Load: 3200,
			},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)
	startResult, _ := vm.Start("perf-test", "")

	// Performance improved
	mock.performance = &performance.Snapshot{
		URL: "http://localhost:3000",
		Timing: performance.Timing{
			Load: 1100,
		},
	}

	vm.Watch(startResult.VerifSessionID)
	result, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if result.Result.PerformanceDiff == nil {
		t.Fatal("Expected performance diff")
	}
	if result.Result.PerformanceDiff.LoadTimeBefore != "3200ms" {
		t.Errorf("Expected load time before '3200ms', got %q", result.Result.PerformanceDiff.LoadTimeBefore)
	}
	if result.Result.PerformanceDiff.LoadTimeAfter != "1100ms" {
		t.Errorf("Expected load time after '1100ms', got %q", result.Result.PerformanceDiff.LoadTimeAfter)
	}
}

// ============================================
// Test: Session Limits
// ============================================

func TestVerificationManager_MaxConcurrentSessions(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	// Start 3 sessions (the max)
	for i := 0; i < 3; i++ {
		_, err := vm.Start("session", "")
		if err != nil {
			t.Fatalf("Start %d failed: %v", i+1, err)
		}
	}

	// 4th should fail
	_, err := vm.Start("session", "")
	if err == nil {
		t.Error("Expected error when exceeding max concurrent sessions")
	}
	if !strings.Contains(err.Error(), "maximum") {
		t.Errorf("Error should mention maximum sessions: %v", err)
	}
}

func TestVerificationManager_SessionTTL(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManagerWithTTL(mock, 100*time.Millisecond)

	startResult, _ := vm.Start("ttl-test", "")

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Session should be expired
	_, err := vm.Watch(startResult.VerifSessionID)
	if err == nil {
		t.Error("Expected error for expired session")
	}
}

func TestVerificationManager_SessionAutoCleanup(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManagerWithTTL(mock, 100*time.Millisecond)

	// Start 3 sessions
	vm.Start("session1", "")
	vm.Start("session2", "")
	vm.Start("session3", "")

	// Wait for TTL
	time.Sleep(150 * time.Millisecond)

	// After cleanup (triggered by new start), should be able to start new session
	_, err := vm.Start("session4", "")
	if err != nil {
		t.Errorf("Should be able to start after cleanup: %v", err)
	}
}

// ============================================
// Test: MCP Tool Handler
// ============================================

func TestVerificationManager_HandleTool_Start(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Test error", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}
	vm := NewVerificationManager(mock)

	params := json.RawMessage(`{"action":"start","label":"test-label"}`)
	result, err := vm.HandleTool(params)
	if err != nil {
		t.Fatalf("HandleTool failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	if resultMap["status"] != "baseline_captured" {
		t.Errorf("Expected status 'baseline_captured', got %v", resultMap["status"])
	}
	if resultMap["verif_session_id"] == "" {
		t.Error("Expected verif_session_id in result")
	}
}

func TestVerificationManager_HandleTool_FullWorkflow(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Original error", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}
	vm := NewVerificationManager(mock)

	// Start
	params := json.RawMessage(`{"action":"start"}`)
	startResult, err := vm.HandleTool(params)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	startMap := startResult.(map[string]any)
	sessionID := startMap["verif_session_id"].(string)

	// Simulate fix
	mock.consoleErrors = nil

	// Watch
	watchParams, _ := json.Marshal(map[string]string{"action": "watch", "verif_session_id": sessionID})
	_, err = vm.HandleTool(watchParams)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	// Compare
	compareParams, _ := json.Marshal(map[string]string{"action": "compare", "verif_session_id": sessionID})
	compareResult, err := vm.HandleTool(compareParams)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	compareMap := compareResult.(map[string]any)
	result := compareMap["result"].(map[string]any)
	if result["verdict"] != "fixed" {
		t.Errorf("Expected verdict 'fixed', got %v", result["verdict"])
	}
}

func TestVerificationManager_HandleTool_Status(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	// Start
	params := json.RawMessage(`{"action":"start"}`)
	startResult, _ := vm.HandleTool(params)
	startMap := startResult.(map[string]any)
	sessionID := startMap["verif_session_id"].(string)

	// Status
	statusParams, _ := json.Marshal(map[string]string{"action": "status", "verif_session_id": sessionID})
	statusResult, err := vm.HandleTool(statusParams)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	statusMap := statusResult.(map[string]any)
	if statusMap["status"] != "baseline_captured" {
		t.Errorf("Expected status 'baseline_captured', got %v", statusMap["status"])
	}
}

func TestVerificationManager_HandleTool_Cancel(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	// Start
	params := json.RawMessage(`{"action":"start"}`)
	startResult, _ := vm.HandleTool(params)
	startMap := startResult.(map[string]any)
	sessionID := startMap["verif_session_id"].(string)

	// Cancel
	cancelParams, _ := json.Marshal(map[string]string{"action": "cancel", "verif_session_id": sessionID})
	cancelResult, err := vm.HandleTool(cancelParams)
	if err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}

	cancelMap := cancelResult.(map[string]any)
	if cancelMap["status"] != "cancelled" {
		t.Errorf("Expected status 'cancelled', got %v", cancelMap["status"])
	}
}

func TestVerificationManager_HandleTool_InvalidAction(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	params := json.RawMessage(`{"action":"invalid"}`)
	_, err := vm.HandleTool(params)
	if err == nil {
		t.Error("Expected error for invalid action")
	}
}

func TestVerificationManager_HandleTool_MissingAction(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	params := json.RawMessage(`{}`)
	_, err := vm.HandleTool(params)
	if err == nil {
		t.Error("Expected error for missing action")
	}
}

func TestVerificationManager_HandleTool_MissingSessionID(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	// Watch without verif_session_id
	params := json.RawMessage(`{"action":"watch"}`)
	_, err := vm.HandleTool(params)
	if err == nil {
		t.Error("Expected error for missing verif_session_id")
	}
}

// ============================================
// Test: Edge Cases
// ============================================

func TestVerificationManager_EmptyBaseline(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors:   nil,
		networkRequests: nil,
		pageURL:         "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)
	startResult, err := vm.Start("empty-test", "")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if startResult.Baseline.ConsoleErrors != 0 {
		t.Errorf("Expected 0 console errors, got %d", startResult.Baseline.ConsoleErrors)
	}
	if startResult.Baseline.NetworkErrors != 0 {
		t.Errorf("Expected 0 network errors, got %d", startResult.Baseline.NetworkErrors)
	}
}

func TestVerificationManager_CompareMultipleTimes(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{
			{Type: "console", Message: "Error", Count: 1},
		},
		pageURL: "http://localhost:3000",
	}

	vm := NewVerificationManager(mock)
	startResult, _ := vm.Start("multi-compare", "")
	mock.consoleErrors = nil
	vm.Watch(startResult.VerifSessionID)

	// First compare
	result1, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("First compare failed: %v", err)
	}
	if result1.Result.Verdict != "fixed" {
		t.Errorf("First compare: expected 'fixed', got %q", result1.Result.Verdict)
	}

	// Add new error
	mock.consoleErrors = []session.SnapshotError{
		{Type: "console", Message: "New error", Count: 1},
	}

	// Second compare should work (re-run scenario)
	result2, err := vm.Compare(startResult.VerifSessionID)
	if err != nil {
		t.Fatalf("Second compare failed: %v", err)
	}
	// Now there's a new error compared to baseline (which had 1 error)
	if result2.Result.Verdict != "different_issue" {
		t.Errorf("Second compare: expected 'different_issue', got %q", result2.Result.Verdict)
	}
}

// ============================================
// Test: Concurrent Access
// ============================================

func TestVerificationManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	done := make(chan bool)

	// Start multiple goroutines
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			_, _ = vm.Start("concurrent", "")
		}()
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have max 3 sessions
	// (others should error due to limit)
}
