// Purpose: Tests for the session-activity observe modes (actions, history) and their summaries.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
	"time"
)

func TestBuildVitalsMapIncludesActionableAttribution(t *testing.T) {
	t.Parallel()
	snapshots := []performance.PerformanceSnapshot{{
		URL: "/app",
		VitalsAttribution: &performance.WireVitalsAttribution{
			LCP: &performance.WireLCPAttribution{
				Element:           performance.WireElementDescriptor{Tag: "img", ID: "hero"},
				AttributionStatus: "available",
			},
		},
	}}
	result := buildVitalsMap(snapshots)
	attribution, ok := result["attribution"].(*performance.WireVitalsAttribution)
	if !ok || attribution.LCP == nil || attribution.LCP.Element.ID != "hero" {
		t.Fatalf("actionable attribution missing: %#v", result)
	}
}

func TestBuildHistoryEntries_Limit(t *testing.T) {
	t.Parallel()
	now := time.Now().UnixMilli()
	actions := []types.EnhancedAction{
		{Type: "navigate", Timestamp: now - 3000, ToURL: "https://a.com"},
		{Type: "navigate", Timestamp: now - 2000, ToURL: "https://b.com"},
		{Type: "navigate", Timestamp: now - 1000, ToURL: "https://c.com"},
	}

	entries := buildHistoryEntries(actions)
	if len(entries) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(entries))
	}

	// Now test with limit
	limited := limitHistoryEntries(entries, 2)
	if len(limited) != 2 {
		t.Errorf("expected 2 entries with limit, got %d", len(limited))
	}
	// Should keep the most recent (last) entries
	if limited[0].ToURL != "https://b.com" {
		t.Errorf("first limited entry = %s, want https://b.com", limited[0].ToURL)
	}
	if limited[1].ToURL != "https://c.com" {
		t.Errorf("second limited entry = %s, want https://c.com", limited[1].ToURL)
	}
}

func TestLimitHistoryEntries_NoTruncation(t *testing.T) {
	t.Parallel()
	entries := []historyEntry{
		{ToURL: "https://a.com"},
		{ToURL: "https://b.com"},
	}
	limited := limitHistoryEntries(entries, 10)
	if len(limited) != 2 {
		t.Errorf("expected 2 entries (no truncation), got %d", len(limited))
	}
}

func TestLimitHistoryEntries_ZeroLimit(t *testing.T) {
	t.Parallel()
	entries := []historyEntry{
		{ToURL: "https://a.com"},
	}
	// Zero limit means no limit applied
	limited := limitHistoryEntries(entries, 0)
	if len(limited) != 1 {
		t.Errorf("expected 1 entry with zero limit, got %d", len(limited))
	}
}

// ============================================
// A11y Summary Tests
// ============================================

func TestBuildActionsSummary_ByType(t *testing.T) {
	t.Parallel()
	now := time.Now().UnixMilli()
	actions := []types.EnhancedAction{
		{Type: "click", Timestamp: now},
		{Type: "click", Timestamp: now + 1000},
		{Type: "type", Timestamp: now + 2000},
		{Type: "navigate", Timestamp: now + 3000},
	}
	result := buildActionsSummary(actions, ResponseMetadata{})

	total, _ := result["total"].(int)
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}

	byType, ok := result["by_type"].(map[string]int)
	if !ok {
		t.Fatal("by_type not a map[string]int")
	}
	if byType["click"] != 2 {
		t.Errorf("click = %d, want 2", byType["click"])
	}
	if byType["type"] != 1 {
		t.Errorf("type = %d, want 1", byType["type"])
	}
}

func TestBuildActionsSummary_TimeRange(t *testing.T) {
	t.Parallel()
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	t2 := time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC).UnixMilli()
	actions := []types.EnhancedAction{
		{Type: "click", Timestamp: t1},
		{Type: "click", Timestamp: t2},
	}
	result := buildActionsSummary(actions, ResponseMetadata{})

	timeRange, ok := result["time_range"].(map[string]string)
	if !ok {
		t.Fatal("time_range not a map[string]string")
	}
	if timeRange["first"] == "" || timeRange["last"] == "" {
		t.Error("expected first and last timestamps in time_range")
	}
}

func TestBuildActionsSummary_EpochTimestamp(t *testing.T) {
	t.Parallel()
	// Timestamp 0 = Unix epoch. Should still produce time_range.
	actions := []types.EnhancedAction{
		{Type: "click", Timestamp: 0},
		{Type: "click", Timestamp: 1000},
	}
	result := buildActionsSummary(actions, ResponseMetadata{})
	if _, ok := result["time_range"]; !ok {
		t.Error("expected time_range even with epoch timestamp 0")
	}
}

func TestBuildHistorySummary_Counts(t *testing.T) {
	t.Parallel()
	entries := []historyEntry{
		{Timestamp: "2024-01-01T10:00:00Z", ToURL: "http://a.com", Type: "navigate"},
		{Timestamp: "2024-01-01T10:01:00Z", ToURL: "http://b.com", Type: "navigate"},
		{Timestamp: "2024-01-01T10:02:00Z", ToURL: "http://b.com/page", Type: "page_visit"},
	}
	result := buildHistorySummary(entries, ResponseMetadata{RetrievedAt: "2024-01-01T10:03:00Z"})

	total, _ := result["total"].(int)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	byType, ok := result["by_type"].(map[string]int)
	if !ok {
		t.Fatal("by_type not a map[string]int")
	}
	if byType["navigate"] != 2 {
		t.Errorf("navigate = %d, want 2", byType["navigate"])
	}
	if byType["page_visit"] != 1 {
		t.Errorf("page_visit = %d, want 1", byType["page_visit"])
	}
	uniqueURLs, ok := result["unique_urls"].(int)
	if !ok {
		t.Fatal("unique_urls not an int")
	}
	if uniqueURLs != 3 {
		t.Errorf("unique_urls = %d, want 3", uniqueURLs)
	}
}

func TestBuildHistorySummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildHistorySummary(nil, ResponseMetadata{})
	total, _ := result["total"].(int)
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}
