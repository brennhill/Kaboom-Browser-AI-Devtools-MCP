// Purpose: Tests for checkpoint metric computation and aggregation.

// ai_checkpoint_compute_test.go — Tests for checkpoint diff computation, severity, and summaries.
package checkpoint

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// recentSlice edge cases
// ============================================

func TestRecentSlice_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		available int
		newCount  int
		want      int
	}{
		{"zero_new", 10, 0, -1},
		{"negative_new", 10, -5, -1},
		{"new_exceeds_available", 5, 10, 5},
		{"new_equals_available", 10, 10, 10},
		{"new_less_than_available", 10, 3, 3},
		{"both_zero", 0, 0, -1},
		{"available_zero_positive_new", 0, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recentSlice(tt.available, tt.newCount)
			if got != tt.want {
				t.Errorf("recentSlice(%d, %d) = %d, want %d", tt.available, tt.newCount, got, tt.want)
			}
		})
	}
}

// ============================================
// FingerprintMessage: various dynamic content
// ============================================

func TestFingerprintMessage_Various(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		contains []string
		absent   []string
	}{
		{
			name:     "uuid_replacement",
			input:    "Error for id 550e8400-e29b-41d4-a716-446655440000",
			contains: []string{"{uuid}"},
			absent:   []string{"550e8400"},
		},
		{
			name:     "large_number_replacement",
			input:    "User 123456 failed at step 9999",
			contains: []string{"{n}"},
			absent:   []string{"123456", "9999"},
		},
		{
			name:     "iso_timestamp_replacement",
			input:    "Event at 2024-01-15T10:30:45.123Z",
			contains: []string{"{ts}"},
			absent:   []string{"2024-01-15"},
		},
		{
			name:     "multiple_replacements",
			input:    "Order 12345 at 2024-01-15T10:30:45Z for user 550e8400-e29b-41d4-a716-446655440000",
			contains: []string{"{n}", "{ts}", "{uuid}"},
			absent:   []string{"12345", "2024-01-15", "550e8400"},
		},
		{
			name:     "no_dynamic_content",
			input:    "Static error message",
			contains: []string{"Static error message"},
			absent:   []string{"{n}", "{uuid}", "{ts}"},
		},
		{
			name:     "small_numbers_preserved",
			input:    "Error code 42 on line 7",
			contains: []string{"42", "7"},
			absent:   []string{"{n}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FingerprintMessage(tt.input)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("FingerprintMessage(%q) = %q, want to contain %q", tt.input, result, want)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(result, absent) {
					t.Errorf("FingerprintMessage(%q) = %q, should not contain %q", tt.input, result, absent)
				}
			}
		})
	}
}

// ============================================
// truncateMessage: edge cases
// ============================================

func TestTruncateMessage_EdgeCases(t *testing.T) {
	t.Parallel()

	// Short message (no truncation needed)
	short := "short"
	if got := truncateMessage(short); got != short {
		t.Errorf("truncateMessage(%q) = %q, want %q", short, got, short)
	}

	// Exactly maxMessageLen
	exact := strings.Repeat("a", maxMessageLen)
	if got := truncateMessage(exact); got != exact {
		t.Errorf("truncateMessage at exact limit should not truncate, got len=%d", len(got))
	}

	// One over maxMessageLen
	over := strings.Repeat("b", maxMessageLen+1)
	if got := truncateMessage(over); len(got) > maxMessageLen {
		t.Errorf("truncateMessage over limit should truncate, got len=%d", len(got))
	}

	// Empty string
	if got := truncateMessage(""); got != "" {
		t.Errorf("truncateMessage empty should return empty, got %q", got)
	}
}

// ============================================
// determineSeverity: all paths
// ============================================

func TestDetermineSeverity_AllPaths(t *testing.T) {
	t.Parallel()

	cm := &CheckpointManager{}

	tests := []struct {
		name     string
		resp     DiffResponse
		expected string
	}{
		{
			name:     "clean_empty",
			resp:     DiffResponse{},
			expected: "clean",
		},
		{
			name: "error_from_console",
			resp: DiffResponse{
				Console: &ConsoleDiff{Errors: []ConsoleEntry{{Message: "err", Count: 1}}},
			},
			expected: "error",
		},
		{
			name: "error_from_network_failures",
			resp: DiffResponse{
				Network: &NetworkDiff{Failures: []NetworkFailure{{Path: "/api", Status: 500}}},
			},
			expected: "error",
		},
		{
			name: "warning_from_console",
			resp: DiffResponse{
				Console: &ConsoleDiff{Warnings: []ConsoleEntry{{Message: "warn", Count: 1}}},
			},
			expected: "warning",
		},
		{
			name: "warning_from_ws_disconnections",
			resp: DiffResponse{
				WebSocket: &WebSocketDiff{Disconnections: []WSDisconnection{{URL: "ws://example.com"}}},
			},
			expected: "warning",
		},
		{
			name: "error_takes_precedence_over_warning",
			resp: DiffResponse{
				Console:   &ConsoleDiff{Errors: []ConsoleEntry{{Message: "err", Count: 1}}, Warnings: []ConsoleEntry{{Message: "warn", Count: 1}}},
				WebSocket: &WebSocketDiff{Disconnections: []WSDisconnection{{URL: "ws://example.com"}}},
			},
			expected: "error",
		},
		{
			name: "nil_console_and_network",
			resp: DiffResponse{
				Console: nil,
				Network: nil,
			},
			expected: "clean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cm.determineSeverity(tt.resp)
			if got != tt.expected {
				t.Errorf("determineSeverity() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ============================================
// buildSummary: all paths
// ============================================

func TestBuildSummary_AllPaths(t *testing.T) {
	t.Parallel()

	cm := &CheckpointManager{}

	tests := []struct {
		name     string
		resp     DiffResponse
		contains []string
	}{
		{
			name:     "clean",
			resp:     DiffResponse{Severity: "clean"},
			contains: []string{"No significant changes."},
		},
		{
			name: "console_errors",
			resp: DiffResponse{
				Severity: "error",
				Console:  &ConsoleDiff{Errors: []ConsoleEntry{{Message: "err", Count: 3}}},
			},
			contains: []string{"3 new console error(s)"},
		},
		{
			name: "network_failures",
			resp: DiffResponse{
				Severity: "error",
				Network:  &NetworkDiff{Failures: []NetworkFailure{{Path: "/a"}, {Path: "/b"}}},
			},
			contains: []string{"2 network failure(s)"},
		},
		{
			name: "console_warnings",
			resp: DiffResponse{
				Severity: "warning",
				Console:  &ConsoleDiff{Warnings: []ConsoleEntry{{Message: "w1", Count: 2}, {Message: "w2", Count: 1}}},
			},
			contains: []string{"3 new console warning(s)"},
		},
		{
			name: "ws_disconnections",
			resp: DiffResponse{
				Severity:  "warning",
				WebSocket: &WebSocketDiff{Disconnections: []WSDisconnection{{URL: "ws://a"}, {URL: "ws://b"}, {URL: "ws://c"}}},
			},
			contains: []string{"3 websocket disconnection(s)"},
		},
		{
			name: "multiple_parts",
			resp: DiffResponse{
				Severity:  "error",
				Console:   &ConsoleDiff{Errors: []ConsoleEntry{{Count: 1}}, Warnings: []ConsoleEntry{{Count: 2}}},
				Network:   &NetworkDiff{Failures: []NetworkFailure{{Path: "/x"}}},
				WebSocket: &WebSocketDiff{Disconnections: []WSDisconnection{{URL: "ws://a"}}},
			},
			contains: []string{"console error", "network failure", "console warning", "websocket disconnection"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cm.buildSummary(tt.resp)
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("buildSummary() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

// ============================================
// sumConsoleCounts
// ============================================

func TestSumConsoleCounts(t *testing.T) {
	t.Parallel()

	if got := sumConsoleCounts(nil); got != 0 {
		t.Errorf("sumConsoleCounts(nil) = %d, want 0", got)
	}

	if got := sumConsoleCounts([]ConsoleEntry{}); got != 0 {
		t.Errorf("sumConsoleCounts(empty) = %d, want 0", got)
	}

	entries := []ConsoleEntry{
		{Message: "a", Count: 3},
		{Message: "b", Count: 7},
		{Message: "c", Count: 1},
	}
	if got := sumConsoleCounts(entries); got != 11 {
		t.Errorf("sumConsoleCounts = %d, want 11", got)
	}
}

// ============================================
// containsString
// ============================================

func TestContainsString_Exhaustive(t *testing.T) {
	t.Parallel()

	if containsString(nil, "a") {
		t.Error("containsString(nil, a) should be false")
	}
	if containsString([]string{}, "a") {
		t.Error("containsString(empty, a) should be false")
	}
	if !containsString([]string{"a", "b", "c"}, "a") {
		t.Error("containsString should find first element")
	}
	if !containsString([]string{"a", "b", "c"}, "c") {
		t.Error("containsString should find last element")
	}
	if containsString([]string{"a", "b", "c"}, "d") {
		t.Error("containsString should return false for missing element")
	}
}

// ============================================
// pruneEmptyDiffs: various scenarios
// ============================================

func TestPruneEmptyDiffs(t *testing.T) {
	t.Parallel()

	cm := &CheckpointManager{}

	// All nil diffs remain nil
	resp := &DiffResponse{}
	cm.pruneEmptyDiffs(resp)
	if resp.Console != nil || resp.Network != nil || resp.WebSocket != nil || resp.Actions != nil {
		t.Error("all-nil diffs should remain nil after pruning")
	}

	// Console with TotalNew=0 pruned
	resp = &DiffResponse{Console: &ConsoleDiff{TotalNew: 0}}
	cm.pruneEmptyDiffs(resp)
	if resp.Console != nil {
		t.Error("Console with TotalNew=0 should be pruned")
	}

	// Console with TotalNew>0 kept
	resp = &DiffResponse{Console: &ConsoleDiff{TotalNew: 1}}
	cm.pruneEmptyDiffs(resp)
	if resp.Console == nil {
		t.Error("Console with TotalNew=1 should be kept")
	}

	// Network with all empty fields pruned
	resp = &DiffResponse{Network: &NetworkDiff{TotalNew: 0}}
	cm.pruneEmptyDiffs(resp)
	if resp.Network != nil {
		t.Error("Network with all empty fields should be pruned")
	}

	// Network with failures kept even if TotalNew=0
	resp = &DiffResponse{Network: &NetworkDiff{TotalNew: 0, Failures: []NetworkFailure{{Path: "/a"}}}}
	cm.pruneEmptyDiffs(resp)
	if resp.Network == nil {
		t.Error("Network with failures should be kept even if TotalNew=0")
	}

	// Network with NewEndpoints kept
	resp = &DiffResponse{Network: &NetworkDiff{TotalNew: 0, NewEndpoints: []string{"/b"}}}
	cm.pruneEmptyDiffs(resp)
	if resp.Network == nil {
		t.Error("Network with NewEndpoints should be kept")
	}

	// Network with Degraded kept
	resp = &DiffResponse{Network: &NetworkDiff{TotalNew: 0, Degraded: []NetworkDegraded{{Path: "/c"}}}}
	cm.pruneEmptyDiffs(resp)
	if resp.Network == nil {
		t.Error("Network with Degraded should be kept")
	}

	// WebSocket with TotalNew=0 pruned
	resp = &DiffResponse{WebSocket: &WebSocketDiff{TotalNew: 0}}
	cm.pruneEmptyDiffs(resp)
	if resp.WebSocket != nil {
		t.Error("WebSocket with TotalNew=0 should be pruned")
	}

	// Actions with TotalNew=0 pruned
	resp = &DiffResponse{Actions: &ActionsDiff{TotalNew: 0}}
	cm.pruneEmptyDiffs(resp)
	if resp.Actions != nil {
		t.Error("Actions with TotalNew=0 should be pruned")
	}
}

// ============================================
// classifyNetworkBody: new failed endpoint (not in known)
// ============================================

func TestClassifyNetworkBody_NewFailedEndpoint(t *testing.T) {
	t.Parallel()

	diff := &NetworkDiff{}
	body := capture.NetworkBody{
		URL:    "http://localhost:3000/new-api",
		Status: 500,
		Method: "GET",
	}
	known := map[string]endpointState{}

	classifyNetworkBody(diff, body, known)

	// Unknown 500 should be a new endpoint, not a failure (no previous status to compare)
	if len(diff.NewEndpoints) != 1 || diff.NewEndpoints[0] != "/new-api" {
		t.Errorf("expected /new-api as new endpoint, got %v", diff.NewEndpoints)
	}
	if len(diff.Failures) != 0 {
		t.Errorf("expected 0 failures for unknown 500, got %v", diff.Failures)
	}
}

// ============================================
// classifyNetworkBody: known endpoint degrades then fails
// ============================================

func TestClassifyNetworkBody_KnownEndpointFailure(t *testing.T) {
	t.Parallel()

	diff := &NetworkDiff{}
	body := capture.NetworkBody{
		URL:    "http://localhost:3000/api/data",
		Status: 503,
		Method: "GET",
	}
	known := map[string]endpointState{
		"/api/data": {Status: 200, Duration: 100},
	}

	classifyNetworkBody(diff, body, known)

	if len(diff.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(diff.Failures))
	}
	if diff.Failures[0].Path != "/api/data" {
		t.Errorf("failure path = %q, want /api/data", diff.Failures[0].Path)
	}
	if diff.Failures[0].Status != 503 {
		t.Errorf("failure status = %d, want 503", diff.Failures[0].Status)
	}
	if diff.Failures[0].PreviousStatus != 200 {
		t.Errorf("failure previous_status = %d, want 200", diff.Failures[0].PreviousStatus)
	}
}

// ============================================
// classifyNetworkBody: successful request to known endpoint (no degrade)
// ============================================

func TestClassifyNetworkBody_SuccessfulNotDegraded(t *testing.T) {
	t.Parallel()

	diff := &NetworkDiff{}
	body := capture.NetworkBody{
		URL:      "http://localhost:3000/api/fast",
		Status:   200,
		Method:   "GET",
		Duration: 50,
	}
	known := map[string]endpointState{
		"/api/fast": {Status: 200, Duration: 100},
	}

	classifyNetworkBody(diff, body, known)

	if len(diff.Degraded) != 0 {
		t.Error("faster request should not be marked as degraded")
	}
}

// ============================================
// classifyNetworkBody: degraded latency detection
// ============================================

func TestClassifyNetworkBody_DegradedLatency(t *testing.T) {
	t.Parallel()

	diff := &NetworkDiff{}
	body := capture.NetworkBody{
		URL:      "http://localhost:3000/api/slow",
		Status:   200,
		Method:   "GET",
		Duration: 350, // > 100 * 3 (degradedLatencyFactor)
	}
	known := map[string]endpointState{
		"/api/slow": {Status: 200, Duration: 100},
	}

	classifyNetworkBody(diff, body, known)

	if len(diff.Degraded) != 1 {
		t.Fatalf("expected 1 degraded endpoint, got %d", len(diff.Degraded))
	}
	if diff.Degraded[0].Path != "/api/slow" {
		t.Errorf("degraded path = %q, want /api/slow", diff.Degraded[0].Path)
	}
	if diff.Degraded[0].Duration != 350 {
		t.Errorf("degraded duration = %d, want 350", diff.Degraded[0].Duration)
	}
	if diff.Degraded[0].Baseline != 100 {
		t.Errorf("degraded baseline = %d, want 100", diff.Degraded[0].Baseline)
	}
}

// ============================================
// classifyNetworkBody: zero duration not degraded
// ============================================

func TestClassifyNetworkBody_ZeroDurationNotDegraded(t *testing.T) {
	t.Parallel()

	diff := &NetworkDiff{}
	body := capture.NetworkBody{
		URL:      "http://localhost:3000/api/nodur",
		Status:   200,
		Method:   "GET",
		Duration: 0,
	}
	known := map[string]endpointState{
		"/api/nodur": {Status: 200, Duration: 100},
	}

	classifyNetworkBody(diff, body, known)

	if len(diff.Degraded) != 0 {
		t.Error("zero duration should not be marked as degraded")
	}
}

// ============================================
// appendUniqueEndpoint: deduplication
// ============================================

func TestAppendUniqueEndpoint_Deduplication(t *testing.T) {
	t.Parallel()

	diff := &NetworkDiff{}
	appendUniqueEndpoint(diff, "/api/test")
	appendUniqueEndpoint(diff, "/api/test")
	appendUniqueEndpoint(diff, "/api/other")

	if len(diff.NewEndpoints) != 2 {
		t.Errorf("expected 2 unique endpoints, got %d: %v", len(diff.NewEndpoints), diff.NewEndpoints)
	}
}

// ============================================
// capNetworkDiff: capping all fields
// ============================================

func TestCapNetworkDiff_CapsAllFields(t *testing.T) {
	t.Parallel()

	diff := &NetworkDiff{}
	for i := 0; i < maxDiffEntriesPerCat+10; i++ {
		diff.Failures = append(diff.Failures, NetworkFailure{Path: fmt.Sprintf("/f%d", i)})
		diff.NewEndpoints = append(diff.NewEndpoints, fmt.Sprintf("/e%d", i))
		diff.Degraded = append(diff.Degraded, NetworkDegraded{Path: fmt.Sprintf("/d%d", i)})
	}

	capNetworkDiff(diff)

	if len(diff.Failures) != maxDiffEntriesPerCat {
		t.Errorf("Failures len = %d, want %d", len(diff.Failures), maxDiffEntriesPerCat)
	}
	if len(diff.NewEndpoints) != maxDiffEntriesPerCat {
		t.Errorf("NewEndpoints len = %d, want %d", len(diff.NewEndpoints), maxDiffEntriesPerCat)
	}
	if len(diff.Degraded) != maxDiffEntriesPerCat {
		t.Errorf("Degraded len = %d, want %d", len(diff.Degraded), maxDiffEntriesPerCat)
	}
}
