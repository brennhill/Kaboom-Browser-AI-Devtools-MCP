// metadata_test.go — Tests for observe response metadata including data_age_ms.
package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capturefixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pagination"
)

func TestCorePackageFileBoundary(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read observe package: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
			count++
		}
	}
	if count > 10 {
		t.Fatalf("observe core package has %d Go files; maximum is 10", count)
	}
}

func TestBuildResponseMetadata_DataAgeMs_FreshData(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	capturefixture.Connect(cap)

	newestEntry := time.Now().Add(-500 * time.Millisecond)
	meta := BuildResponseMetadata(cap, newestEntry)

	// data_age_ms should be approximately 500 (within 100ms tolerance)
	if meta.DataAgeMs < 400 || meta.DataAgeMs > 700 {
		t.Errorf("DataAgeMs = %d, want ~500 for 500ms-old data", meta.DataAgeMs)
	}
}

func TestBuildResponseMetadata_DataAgeMs_ZeroForVeryFresh(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	capturefixture.Connect(cap)

	newestEntry := time.Now()
	meta := BuildResponseMetadata(cap, newestEntry)

	// data_age_ms should be 0 or very small for just-now data
	if meta.DataAgeMs > 100 {
		t.Errorf("DataAgeMs = %d, want <=100 for fresh data", meta.DataAgeMs)
	}
}

func TestBuildResponseMetadata_DataAgeMs_StaleData(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	capturefixture.Connect(cap)

	newestEntry := time.Now().Add(-30 * time.Second)
	meta := BuildResponseMetadata(cap, newestEntry)

	// data_age_ms should be approximately 30000
	if meta.DataAgeMs < 29000 || meta.DataAgeMs > 31000 {
		t.Errorf("DataAgeMs = %d, want ~30000 for 30s-old data", meta.DataAgeMs)
	}
}

func TestBuildResponseMetadata_DataAgeMs_NoData(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	capturefixture.Connect(cap)

	meta := BuildResponseMetadata(cap, time.Time{})

	// When no data, data_age_ms should be -1 (sentinel for "no data")
	if meta.DataAgeMs != -1 {
		t.Errorf("DataAgeMs = %d, want -1 for no data", meta.DataAgeMs)
	}
	if meta.DataAge != "no_data" {
		t.Errorf("DataAge = %q, want 'no_data'", meta.DataAge)
	}
}

func TestBuildResponseMetadata_DataAgeMs_FutureTimestamp(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	capturefixture.Connect(cap)

	// Simulate NTP clock adjustment: newestEntry is in the future relative to now.
	newestEntry := time.Now().Add(5 * time.Second)
	meta := BuildResponseMetadata(cap, newestEntry)

	// data_age_ms must be clamped to 0, not negative.
	if meta.DataAgeMs < 0 {
		t.Errorf("DataAgeMs = %d, want >= 0 (negative age should be clamped)", meta.DataAgeMs)
	}
	if meta.DataAgeMs != 0 {
		t.Errorf("DataAgeMs = %d, want 0 for future timestamp", meta.DataAgeMs)
	}
	if meta.DataAge != "0.0s" {
		t.Errorf("DataAge = %q, want '0.0s' for clamped age", meta.DataAge)
	}
}

func TestBuildResponseMetadata_DataAgeMs_InPaginatedMetadata(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	capturefixture.Connect(cap)

	newestEntry := time.Now().Add(-2 * time.Second)
	pMeta := &pagination.CursorPaginationMetadata{
		Total:   10,
		HasMore: false,
	}
	meta := BuildPaginatedResponseMetadata(cap, newestEntry, pMeta)

	dataAgeMs, ok := meta["data_age_ms"]
	if !ok {
		t.Fatal("paginated metadata missing 'data_age_ms' field")
	}
	ageMs, ok := dataAgeMs.(int64)
	if !ok {
		t.Fatalf("data_age_ms type = %T, want int64", dataAgeMs)
	}
	if ageMs < 1500 || ageMs > 3000 {
		t.Errorf("data_age_ms = %d, want ~2000 for 2s-old data", ageMs)
	}
}

func TestBuildPaginatedMetadataWithSummary_FirstPage(t *testing.T) {
	t.Parallel()
	pMeta := &pagination.CursorPaginationMetadata{
		Total:   50,
		HasMore: true,
		Cursor:  "cursor123",
	}
	summaryFn := func() map[string]any {
		return map[string]any{"total": 50, "by_level": map[string]int{"info": 30, "error": 20}}
	}

	// isFirstPage=true
	result := BuildPaginatedMetadataWithSummary(nil, time.Time{}, pMeta, true, summaryFn)

	summary, ok := result["summary"]
	if !ok {
		t.Fatal("expected summary key on first page")
	}
	summaryMap, ok := summary.(map[string]any)
	if !ok {
		t.Fatal("summary not a map[string]any")
	}
	if summaryMap["total"] != 50 {
		t.Errorf("summary total = %v, want 50", summaryMap["total"])
	}
}

func TestBuildPaginatedMetadataWithSummary_SubsequentPage(t *testing.T) {
	t.Parallel()
	pMeta := &pagination.CursorPaginationMetadata{
		Total:   50,
		HasMore: true,
		Cursor:  "cursor456",
	}
	called := false
	summaryFn := func() map[string]any {
		called = true
		return map[string]any{"total": 50}
	}

	// isFirstPage=false
	result := BuildPaginatedMetadataWithSummary(nil, time.Time{}, pMeta, false, summaryFn)

	if _, ok := result["summary"]; ok {
		t.Error("expected no summary key on subsequent page")
	}
	if called {
		t.Error("summaryFn should not be called for subsequent pages")
	}
}
