// Purpose: Tests for the cross-stream observe modes (error bundles, timeline) and their summaries.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"testing"
	"time"
)

func TestBuildTimelineSummary_CountsByType(t *testing.T) {
	t.Parallel()
	entries := []timelineEntry{
		{Timestamp: "2024-01-01T00:00:01Z", Type: "action", Summary: "click"},
		{Timestamp: "2024-01-01T00:00:02Z", Type: "action", Summary: "type"},
		{Timestamp: "2024-01-01T00:00:03Z", Type: "error", Summary: "ReferenceError"},
		{Timestamp: "2024-01-01T00:00:04Z", Type: "network", Summary: "GET /api"},
		{Timestamp: "2024-01-01T00:00:05Z", Type: "network", Summary: "POST /api"},
		{Timestamp: "2024-01-01T00:00:06Z", Type: "network", Summary: "GET /img"},
		{Timestamp: "2024-01-01T00:00:07Z", Type: "websocket", Summary: "message"},
	}

	result := buildTimelineSummary(entries)

	counts, ok := result["counts_by_type"].(map[string]int)
	if !ok {
		t.Fatalf("counts_by_type wrong type: %T", result["counts_by_type"])
	}
	if counts["action"] != 2 {
		t.Errorf("action count = %d, want 2", counts["action"])
	}
	if counts["error"] != 1 {
		t.Errorf("error count = %d, want 1", counts["error"])
	}
	if counts["network"] != 3 {
		t.Errorf("network count = %d, want 3", counts["network"])
	}
	if counts["websocket"] != 1 {
		t.Errorf("websocket count = %d, want 1", counts["websocket"])
	}

	if result["total"] != 7 {
		t.Errorf("total = %v, want 7", result["total"])
	}

	timeRange, ok := result["time_range"].(map[string]string)
	if !ok {
		t.Fatalf("time_range wrong type: %T", result["time_range"])
	}
	if timeRange["first"] != "2024-01-01T00:00:01Z" {
		t.Errorf("first = %v, want 2024-01-01T00:00:01Z", timeRange["first"])
	}
	if timeRange["last"] != "2024-01-01T00:00:07Z" {
		t.Errorf("last = %v, want 2024-01-01T00:00:07Z", timeRange["last"])
	}
}

func TestBuildTimelineSummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildTimelineSummary(nil)
	if result["total"] != 0 {
		t.Errorf("total = %v, want 0", result["total"])
	}
}

// ============================================
// History Limit Tests
// ============================================

func TestBuildErrorBundlesSummary_Counts(t *testing.T) {
	t.Parallel()
	bundles := []map[string]any{
		{"error": map[string]any{"message": "err1"}},
		{"error": map[string]any{"message": "err2"}},
		{"error": map[string]any{"message": "err1"}},
	}
	meta := ResponseMetadata{RetrievedAt: "2024-01-01T00:00:00Z"}
	result := buildErrorBundlesSummary(bundles, time.Now(), meta)

	total, _ := result["total_bundles"].(int)
	if total != 3 {
		t.Errorf("total_bundles = %d, want 3", total)
	}

	messages, ok := result["unique_error_messages"].([]string)
	if !ok {
		t.Fatal("unique_error_messages not a []string")
	}
	if len(messages) != 2 {
		t.Errorf("unique_error_messages len = %d, want 2", len(messages))
	}

	// Verify metadata is included
	if _, ok := result["metadata"]; !ok {
		t.Error("expected metadata key in error bundles summary")
	}
}
