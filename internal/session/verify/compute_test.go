// Purpose: Tests for session verification metric computation.
// Docs: docs/features/feature/pagination/index.md

// verify_compute_test.go — Tests for verify_compute.go.
// Covers: sumErrorCounts, buildIssueSummary, buildVerifyErrorMap, diffConsoleErrors,
// formatNetworkEntry, buildNetworkKeyMap, classifyNetworkErrorResolution,
// diffNetworkErrors, computeLoadTimeDiff, computeVerification, determineVerdict,
// requireVerifSessionID, HandleTool dispatch.
package verify

import (
	"encoding/json"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
)

// ============================================
// sumErrorCounts
// ============================================

func TestSumErrorCounts_Empty(t *testing.T) {
	t.Parallel()
	if total := sumErrorCounts(nil); total != 0 {
		t.Errorf("Expected 0 for nil, got %d", total)
	}
	if total := sumErrorCounts([]VerifyError{}); total != 0 {
		t.Errorf("Expected 0 for empty, got %d", total)
	}
}

func TestSumErrorCounts_Single(t *testing.T) {
	t.Parallel()
	errors := []VerifyError{{Count: 5}}
	if total := sumErrorCounts(errors); total != 5 {
		t.Errorf("Expected 5, got %d", total)
	}
}

func TestSumErrorCounts_Multiple(t *testing.T) {
	t.Parallel()
	errors := []VerifyError{
		{Count: 3},
		{Count: 7},
		{Count: 1},
	}
	if total := sumErrorCounts(errors); total != 11 {
		t.Errorf("Expected 11, got %d", total)
	}
}

// ============================================
// buildIssueSummary
// ============================================

func TestBuildIssueSummary_NoIssues(t *testing.T) {
	t.Parallel()
	s := buildIssueSummary(nil, nil)
	if s.ConsoleErrors != 0 {
		t.Errorf("Expected console_errors=0, got %d", s.ConsoleErrors)
	}
	if s.NetworkErrors != 0 {
		t.Errorf("Expected network_errors=0, got %d", s.NetworkErrors)
	}
	if s.TotalIssues != 0 {
		t.Errorf("Expected total_issues=0, got %d", s.TotalIssues)
	}
}

func TestBuildIssueSummary_WithIssues(t *testing.T) {
	t.Parallel()
	consoleErrors := []VerifyError{
		{Count: 2},
		{Count: 3},
	}
	networkErrors := []VerifyNetworkEntry{
		{Status: 500},
		{Status: 404},
		{Status: 502},
	}
	s := buildIssueSummary(consoleErrors, networkErrors)

	if s.ConsoleErrors != 5 {
		t.Errorf("Expected console_errors=5, got %d", s.ConsoleErrors)
	}
	if s.NetworkErrors != 3 {
		t.Errorf("Expected network_errors=3, got %d", s.NetworkErrors)
	}
	if s.TotalIssues != 8 {
		t.Errorf("Expected total_issues=8, got %d", s.TotalIssues)
	}
}

// ============================================
// buildVerifyErrorMap
// ============================================

func TestBuildVerifyErrorMap_Empty(t *testing.T) {
	t.Parallel()
	m := buildVerifyErrorMap(nil)
	if len(m) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(m))
	}
}

func TestBuildVerifyErrorMap_KeyedByNormalized(t *testing.T) {
	t.Parallel()
	errors := []VerifyError{
		{Message: "Error at app.js:42", Normalized: "Error at [file]", Count: 1},
		{Message: "Error at app.js:99", Normalized: "Error at [file]", Count: 1},
	}
	m := buildVerifyErrorMap(errors)

	// Both map to same normalized key, second wins
	if len(m) != 1 {
		t.Errorf("Expected 1 entry (same normalized), got %d", len(m))
	}
	entry, exists := m["Error at [file]"]
	if !exists {
		t.Fatal("Expected entry for 'Error at [file]'")
	}
	// Last write wins
	if entry.Message != "Error at app.js:99" {
		t.Errorf("Expected last-wins message 'Error at app.js:99', got %q", entry.Message)
	}
}

// ============================================
// diffConsoleErrors
// ============================================

func TestDiffConsoleErrors_Resolved(t *testing.T) {
	t.Parallel()
	before := []VerifyError{
		{Message: "err-A", Normalized: "err-A", Count: 2},
		{Message: "err-B", Normalized: "err-B", Count: 1},
	}
	after := []VerifyError{
		{Message: "err-B", Normalized: "err-B", Count: 1},
	}

	changes, newIssues := diffConsoleErrors(before, after)

	if len(changes) != 1 {
		t.Fatalf("Expected 1 resolved change, got %d", len(changes))
	}
	if changes[0].Type != "resolved" {
		t.Errorf("Expected type 'resolved', got %q", changes[0].Type)
	}
	if changes[0].Category != "console" {
		t.Errorf("Expected category 'console', got %q", changes[0].Category)
	}
	if changes[0].Before != "err-A (x2)" {
		t.Errorf("Expected 'err-A (x2)', got %q", changes[0].Before)
	}
	if changes[0].After != "(not seen)" {
		t.Errorf("Expected '(not seen)', got %q", changes[0].After)
	}
	if len(newIssues) != 0 {
		t.Errorf("Expected 0 new issues, got %d", len(newIssues))
	}
}

func TestDiffConsoleErrors_NewIssues(t *testing.T) {
	t.Parallel()
	before := []VerifyError{}
	after := []VerifyError{
		{Message: "new error", Normalized: "new error", Count: 1},
	}

	changes, newIssues := diffConsoleErrors(before, after)

	if len(changes) != 0 {
		t.Errorf("Expected 0 changes, got %d", len(changes))
	}
	if len(newIssues) != 1 {
		t.Fatalf("Expected 1 new issue, got %d", len(newIssues))
	}
	if newIssues[0].Type != "new" {
		t.Errorf("Expected type 'new', got %q", newIssues[0].Type)
	}
	if newIssues[0].Before != "(not seen)" {
		t.Errorf("Expected '(not seen)', got %q", newIssues[0].Before)
	}
	if newIssues[0].After != "new error" {
		t.Errorf("Expected 'new error', got %q", newIssues[0].After)
	}
}

func TestDiffConsoleErrors_CountOneSuffix(t *testing.T) {
	t.Parallel()
	// Count=1 should NOT have "(x1)" suffix
	before := []VerifyError{
		{Message: "single-err", Normalized: "single-err", Count: 1},
	}
	after := []VerifyError{}

	changes, _ := diffConsoleErrors(before, after)

	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(changes))
	}
	if changes[0].Before != "single-err" {
		t.Errorf("Expected 'single-err' without count suffix, got %q", changes[0].Before)
	}
}

func TestDiffConsoleErrors_BothEmpty(t *testing.T) {
	t.Parallel()
	changes, newIssues := diffConsoleErrors(nil, nil)
	if len(changes) != 0 {
		t.Errorf("Expected 0 changes, got %d", len(changes))
	}
	if len(newIssues) != 0 {
		t.Errorf("Expected 0 new issues, got %d", len(newIssues))
	}
}

// ============================================
// formatNetworkEntry
// ============================================

func TestFormatNetworkEntry(t *testing.T) {
	t.Parallel()
	entry := VerifyNetworkEntry{Method: "POST", URL: "/api/login", Status: 500}
	result := formatNetworkEntry(entry)
	if result != "POST /api/login -> 500" {
		t.Errorf("Expected 'POST /api/login -> 500', got %q", result)
	}
}

func TestFormatNetworkEntry_ZeroStatus(t *testing.T) {
	t.Parallel()
	entry := VerifyNetworkEntry{Method: "GET", URL: "/timeout", Status: 0}
	result := formatNetworkEntry(entry)
	if result != "GET /timeout -> 0" {
		t.Errorf("Expected 'GET /timeout -> 0', got %q", result)
	}
}

// ============================================
// buildNetworkKeyMap
// ============================================

func TestBuildNetworkKeyMap_Empty(t *testing.T) {
	t.Parallel()
	m := buildNetworkKeyMap(nil)
	if len(m) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(m))
	}
}

func TestBuildNetworkKeyMap_KeyedByMethodPath(t *testing.T) {
	t.Parallel()
	entries := []VerifyNetworkEntry{
		{Method: "GET", Path: "/api/users", URL: "/api/users", Status: 200},
		{Method: "POST", Path: "/api/users", URL: "/api/users", Status: 201},
	}
	m := buildNetworkKeyMap(entries)

	if len(m) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(m))
	}
	if _, ok := m["GET /api/users"]; !ok {
		t.Error("Expected 'GET /api/users' key")
	}
	if _, ok := m["POST /api/users"]; !ok {
		t.Error("Expected 'POST /api/users' key")
	}
}

// ============================================
// classifyNetworkErrorResolution
// ============================================

func TestClassifyNetworkErrorResolution_Resolved(t *testing.T) {
	t.Parallel()
	result := classifyNetworkErrorResolution(VerifyNetworkEntry{Status: 200})
	if result != "resolved" {
		t.Errorf("Expected 'resolved' for status 200, got %q", result)
	}

	result = classifyNetworkErrorResolution(VerifyNetworkEntry{Status: 301})
	if result != "resolved" {
		t.Errorf("Expected 'resolved' for status 301, got %q", result)
	}

	result = classifyNetworkErrorResolution(VerifyNetworkEntry{Status: 399})
	if result != "resolved" {
		t.Errorf("Expected 'resolved' for status 399, got %q", result)
	}
}

func TestClassifyNetworkErrorResolution_Changed(t *testing.T) {
	t.Parallel()
	result := classifyNetworkErrorResolution(VerifyNetworkEntry{Status: 400})
	if result != "changed" {
		t.Errorf("Expected 'changed' for status 400, got %q", result)
	}

	result = classifyNetworkErrorResolution(VerifyNetworkEntry{Status: 500})
	if result != "changed" {
		t.Errorf("Expected 'changed' for status 500, got %q", result)
	}

	result = classifyNetworkErrorResolution(VerifyNetworkEntry{Status: 0})
	if result != "changed" {
		t.Errorf("Expected 'changed' for status 0, got %q", result)
	}
}

// ============================================
// diffNetworkErrors
// ============================================

func TestDiffNetworkErrors_ResolvedEndpoint(t *testing.T) {
	t.Parallel()
	before := &VerifSnapshot{
		NetworkErrors: []VerifyNetworkEntry{
			{Method: "POST", URL: "/api/login", Path: "/api/login", Status: 500},
		},
		AllNetworkRequests: []VerifyNetworkEntry{},
	}
	after := &VerifSnapshot{
		NetworkErrors: []VerifyNetworkEntry{},
		AllNetworkRequests: []VerifyNetworkEntry{
			{Method: "POST", URL: "/api/login", Path: "/api/login", Status: 200},
		},
	}

	changes, newIssues := diffNetworkErrors(before, after)

	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "resolved" {
		t.Errorf("Expected type 'resolved', got %q", changes[0].Type)
	}
	if len(newIssues) != 0 {
		t.Errorf("Expected 0 new issues, got %d", len(newIssues))
	}
}

func TestDiffNetworkErrors_ChangedStatus(t *testing.T) {
	t.Parallel()
	before := &VerifSnapshot{
		NetworkErrors: []VerifyNetworkEntry{
			{Method: "GET", URL: "/api/data", Path: "/api/data", Status: 500},
		},
		AllNetworkRequests: []VerifyNetworkEntry{},
	}
	after := &VerifSnapshot{
		NetworkErrors: []VerifyNetworkEntry{
			{Method: "GET", URL: "/api/data", Path: "/api/data", Status: 502},
		},
		AllNetworkRequests: []VerifyNetworkEntry{},
	}

	changes, newIssues := diffNetworkErrors(before, after)

	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "changed" {
		t.Errorf("Expected type 'changed', got %q", changes[0].Type)
	}
	if len(newIssues) != 0 {
		t.Errorf("Expected 0 new issues, got %d", len(newIssues))
	}
}

func TestDiffNetworkErrors_NewNetworkError(t *testing.T) {
	t.Parallel()
	before := &VerifSnapshot{
		NetworkErrors:      []VerifyNetworkEntry{},
		AllNetworkRequests: []VerifyNetworkEntry{},
	}
	after := &VerifSnapshot{
		NetworkErrors: []VerifyNetworkEntry{
			{Method: "GET", URL: "/api/broken", Path: "/api/broken", Status: 404},
		},
		AllNetworkRequests: []VerifyNetworkEntry{
			{Method: "GET", URL: "/api/broken", Path: "/api/broken", Status: 404},
		},
	}

	changes, newIssues := diffNetworkErrors(before, after)

	if len(changes) != 0 {
		t.Errorf("Expected 0 changes, got %d", len(changes))
	}
	if len(newIssues) != 1 {
		t.Fatalf("Expected 1 new issue, got %d", len(newIssues))
	}
	if newIssues[0].Type != "new" {
		t.Errorf("Expected type 'new', got %q", newIssues[0].Type)
	}
	if newIssues[0].Category != "network" {
		t.Errorf("Expected category 'network', got %q", newIssues[0].Category)
	}
}

func TestDiffNetworkErrors_EndpointDisappeared(t *testing.T) {
	t.Parallel()
	before := &VerifSnapshot{
		NetworkErrors: []VerifyNetworkEntry{
			{Method: "GET", URL: "/api/temp", Path: "/api/temp", Status: 500},
		},
		AllNetworkRequests: []VerifyNetworkEntry{},
	}
	after := &VerifSnapshot{
		NetworkErrors:      []VerifyNetworkEntry{},
		AllNetworkRequests: []VerifyNetworkEntry{},
	}

	changes, newIssues := diffNetworkErrors(before, after)

	// Endpoint gone from both errors and all network => "resolved" with "(not seen)"
	if len(changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "resolved" {
		t.Errorf("Expected type 'resolved', got %q", changes[0].Type)
	}
	if changes[0].After != "(not seen)" {
		t.Errorf("Expected '(not seen)', got %q", changes[0].After)
	}
	if len(newIssues) != 0 {
		t.Errorf("Expected 0 new issues, got %d", len(newIssues))
	}
}

func TestDiffNetworkErrors_SameStatus_NoChange(t *testing.T) {
	t.Parallel()
	before := &VerifSnapshot{
		NetworkErrors: []VerifyNetworkEntry{
			{Method: "GET", URL: "/api/err", Path: "/api/err", Status: 500},
		},
		AllNetworkRequests: []VerifyNetworkEntry{},
	}
	after := &VerifSnapshot{
		NetworkErrors: []VerifyNetworkEntry{
			{Method: "GET", URL: "/api/err", Path: "/api/err", Status: 500},
		},
		AllNetworkRequests: []VerifyNetworkEntry{},
	}

	changes, newIssues := diffNetworkErrors(before, after)

	// Same status, should produce no changes
	if len(changes) != 0 {
		t.Errorf("Expected 0 changes for same status, got %d", len(changes))
	}
	if len(newIssues) != 0 {
		t.Errorf("Expected 0 new issues, got %d", len(newIssues))
	}
}

// ============================================
// computeLoadTimeDiff
// ============================================

func TestComputeLoadTimeDiff_BothNil(t *testing.T) {
	t.Parallel()
	result := computeLoadTimeDiff(
		&VerifSnapshot{Performance: nil},
		&VerifSnapshot{Performance: nil},
	)
	if result != nil {
		t.Error("Expected nil when both performances are nil")
	}
}

func TestComputeLoadTimeDiff_BeforeNil(t *testing.T) {
	t.Parallel()
	result := computeLoadTimeDiff(
		&VerifSnapshot{Performance: nil},
		&VerifSnapshot{Performance: &performance.Snapshot{Timing: performance.Timing{Load: 1000}}},
	)
	if result != nil {
		t.Error("Expected nil when before performance is nil")
	}
}

func TestComputeLoadTimeDiff_ValidData(t *testing.T) {
	t.Parallel()
	result := computeLoadTimeDiff(
		&VerifSnapshot{Performance: &performance.Snapshot{Timing: performance.Timing{Load: 1000}}},
		&VerifSnapshot{Performance: &performance.Snapshot{Timing: performance.Timing{Load: 1500}}},
	)
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.LoadTimeBefore != "1000ms" {
		t.Errorf("Expected '1000ms', got %q", result.LoadTimeBefore)
	}
	if result.LoadTimeAfter != "1500ms" {
		t.Errorf("Expected '1500ms', got %q", result.LoadTimeAfter)
	}
	if result.Change != "+50%" {
		t.Errorf("Expected '+50%%', got %q", result.Change)
	}
}

func TestComputeLoadTimeDiff_ZeroBefore(t *testing.T) {
	t.Parallel()
	result := computeLoadTimeDiff(
		&VerifSnapshot{Performance: &performance.Snapshot{Timing: performance.Timing{Load: 0}}},
		&VerifSnapshot{Performance: &performance.Snapshot{Timing: performance.Timing{Load: 500}}},
	)
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	// When before.Load == 0, no percent change is computed
	if result.Change != "" {
		t.Errorf("Expected empty change when before=0, got %q", result.Change)
	}
}

func TestComputeLoadTimeDiff_Decrease(t *testing.T) {
	t.Parallel()
	result := computeLoadTimeDiff(
		&VerifSnapshot{Performance: &performance.Snapshot{Timing: performance.Timing{Load: 2000}}},
		&VerifSnapshot{Performance: &performance.Snapshot{Timing: performance.Timing{Load: 1000}}},
	)
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Change != "-50%" {
		t.Errorf("Expected '-50%%', got %q", result.Change)
	}
}

// ============================================
// determineVerdict
// ============================================

func TestDetermineVerdict_NoIssuesDetected(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	result := VerificationResult{
		Before:    IssueSummary{TotalIssues: 0},
		After:     IssueSummary{TotalIssues: 0},
		Changes:   []VerifyChange{},
		NewIssues: []VerifyChange{},
	}
	verdict := vm.determineVerdict(result)
	if verdict != "no_issues_detected" {
		t.Errorf("Expected 'no_issues_detected', got %q", verdict)
	}
}

func TestDetermineVerdict_Fixed(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	result := VerificationResult{
		Before:    IssueSummary{TotalIssues: 2},
		After:     IssueSummary{TotalIssues: 0},
		Changes:   []VerifyChange{{Type: "resolved"}},
		NewIssues: []VerifyChange{},
	}
	verdict := vm.determineVerdict(result)
	if verdict != "fixed" {
		t.Errorf("Expected 'fixed', got %q", verdict)
	}
}

func TestDetermineVerdict_Improved(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	result := VerificationResult{
		Before:    IssueSummary{TotalIssues: 3},
		After:     IssueSummary{TotalIssues: 1},
		Changes:   []VerifyChange{{Type: "resolved"}},
		NewIssues: []VerifyChange{},
	}
	verdict := vm.determineVerdict(result)
	if verdict != "improved" {
		t.Errorf("Expected 'improved', got %q", verdict)
	}
}

func TestDetermineVerdict_DifferentIssue(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	result := VerificationResult{
		Before:    IssueSummary{TotalIssues: 1},
		After:     IssueSummary{TotalIssues: 1},
		Changes:   []VerifyChange{{Type: "resolved"}},
		NewIssues: []VerifyChange{{Type: "new"}},
	}
	verdict := vm.determineVerdict(result)
	if verdict != "different_issue" {
		t.Errorf("Expected 'different_issue', got %q", verdict)
	}
}

func TestDetermineVerdict_Regressed(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	result := VerificationResult{
		Before:    IssueSummary{TotalIssues: 1},
		After:     IssueSummary{TotalIssues: 3},
		Changes:   []VerifyChange{},
		NewIssues: []VerifyChange{{Type: "new"}, {Type: "new"}},
	}
	verdict := vm.determineVerdict(result)
	if verdict != "regressed" {
		t.Errorf("Expected 'regressed', got %q", verdict)
	}
}

func TestDetermineVerdict_Unchanged(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	result := VerificationResult{
		Before:    IssueSummary{TotalIssues: 2},
		After:     IssueSummary{TotalIssues: 2},
		Changes:   []VerifyChange{},
		NewIssues: []VerifyChange{},
	}
	verdict := vm.determineVerdict(result)
	if verdict != "unchanged" {
		t.Errorf("Expected 'unchanged', got %q", verdict)
	}
}

// ============================================
// countResolvedChanges
// ============================================

func TestCountResolvedChanges_None(t *testing.T) {
	t.Parallel()
	if n := countResolvedChanges(nil); n != 0 {
		t.Errorf("Expected 0, got %d", n)
	}
	if n := countResolvedChanges([]VerifyChange{{Type: "new"}, {Type: "changed"}}); n != 0 {
		t.Errorf("Expected 0, got %d", n)
	}
}

func TestCountResolvedChanges_Multiple(t *testing.T) {
	t.Parallel()
	changes := []VerifyChange{
		{Type: "resolved"},
		{Type: "changed"},
		{Type: "resolved"},
		{Type: "new"},
	}
	if n := countResolvedChanges(changes); n != 2 {
		t.Errorf("Expected 2, got %d", n)
	}
}

// ============================================
// requireVerifSessionID
// ============================================

func TestRequireSessionID_Empty(t *testing.T) {
	t.Parallel()
	err := requireVerifSessionID("", "watch")
	if err == nil {
		t.Fatal("Expected error for empty verif_session_id")
	}
}

func TestRequireSessionID_NonEmpty(t *testing.T) {
	t.Parallel()
	err := requireVerifSessionID("verify-123", "watch")
	if err != nil {
		t.Errorf("Expected no error for non-empty verif_session_id, got: %v", err)
	}
}

// ============================================
// HandleTool (VerificationManager) — Error Paths
// ============================================

func TestVerifyHandleTool_InvalidJSON(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.HandleTool(json.RawMessage(`{invalid json`))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestVerifyHandleTool_EmptyAction(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.HandleTool(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("Expected error for empty action")
	}
}

func TestVerifyHandleTool_UnknownAction(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.HandleTool(json.RawMessage(`{"action":"destroy"}`))
	if err == nil {
		t.Fatal("Expected error for unknown action")
	}
}

func TestVerifyHandleTool_WatchMissingSessionID(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.HandleTool(json.RawMessage(`{"action":"watch"}`))
	if err == nil {
		t.Fatal("Expected error for watch without verif_session_id")
	}
}

func TestVerifyHandleTool_CompareMissingSessionID(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.HandleTool(json.RawMessage(`{"action":"compare"}`))
	if err == nil {
		t.Fatal("Expected error for compare without verif_session_id")
	}
}

func TestVerifyHandleTool_StatusMissingSessionID(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.HandleTool(json.RawMessage(`{"action":"status"}`))
	if err == nil {
		t.Fatal("Expected error for status without verif_session_id")
	}
}

func TestVerifyHandleTool_CancelMissingSessionID(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{pageURL: "http://localhost:3000"}
	vm := NewVerificationManager(mock)

	_, err := vm.HandleTool(json.RawMessage(`{"action":"cancel"}`))
	if err == nil {
		t.Fatal("Expected error for cancel without verif_session_id")
	}
}

func TestVerifyHandleTool_StartReturnsFields(t *testing.T) {
	t.Parallel()
	mock := &mockVerifyState{
		consoleErrors: []session.SnapshotError{{Type: "error", Message: "e", Count: 1}},
		pageURL:       "http://localhost:3000",
	}
	vm := NewVerificationManager(mock)

	result, err := vm.HandleTool(json.RawMessage(`{"action":"start","label":"my-label"}`))
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	m := result.(map[string]any)
	if m["status"] != "baseline_captured" {
		t.Errorf("Expected 'baseline_captured', got %v", m["status"])
	}
	if m["label"] != "my-label" {
		t.Errorf("Expected label 'my-label', got %v", m["label"])
	}
	if m["verif_session_id"] == nil || m["verif_session_id"] == "" {
		t.Error("Expected non-empty verif_session_id")
	}
	if m["baseline"] == nil {
		t.Error("Expected baseline in result")
	}
}
