// token_tracker_test.go — Tests for token savings tracker.

package tracking

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRecord_SingleCategory(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()

	tr.Record("test_output", 2000, 50)

	stats := tr.GetSessionStats()
	if stats.TotalTokensSaved != 1950 {
		t.Errorf("TotalTokensSaved = %d, want 1950", stats.TotalTokensSaved)
	}
	if stats.TotalCompressions != 1 {
		t.Errorf("TotalCompressions = %d, want 1", stats.TotalCompressions)
	}
	cat, ok := stats.ByCategory["test_output"]
	if !ok {
		t.Fatal("expected test_output category in stats")
	}
	if cat.TokensBefore != 2000 {
		t.Errorf("TokensBefore = %d, want 2000", cat.TokensBefore)
	}
	if cat.TokensAfter != 50 {
		t.Errorf("TokensAfter = %d, want 50", cat.TokensAfter)
	}
	if cat.TokensSaved != 1950 {
		t.Errorf("TokensSaved = %d, want 1950", cat.TokensSaved)
	}
	if cat.Count != 1 {
		t.Errorf("Count = %d, want 1", cat.Count)
	}
}

func TestRecord_MultipleCategories(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()

	tr.Record("test_output", 2000, 50)
	tr.Record("build_output", 1000, 100)
	tr.Record("test_output", 3000, 200)

	stats := tr.GetSessionStats()
	if stats.TotalTokensSaved != (1950 + 900 + 2800) {
		t.Errorf("TotalTokensSaved = %d, want %d", stats.TotalTokensSaved, 1950+900+2800)
	}
	if stats.TotalCompressions != 3 {
		t.Errorf("TotalCompressions = %d, want 3", stats.TotalCompressions)
	}
	if len(stats.ByCategory) != 2 {
		t.Errorf("ByCategory has %d entries, want 2", len(stats.ByCategory))
	}

	testCat := stats.ByCategory["test_output"]
	if testCat.TokensBefore != 5000 {
		t.Errorf("test_output TokensBefore = %d, want 5000", testCat.TokensBefore)
	}
	if testCat.TokensAfter != 250 {
		t.Errorf("test_output TokensAfter = %d, want 250", testCat.TokensAfter)
	}
	if testCat.Count != 2 {
		t.Errorf("test_output Count = %d, want 2", testCat.Count)
	}

	buildCat := stats.ByCategory["build_output"]
	if buildCat.TokensBefore != 1000 {
		t.Errorf("build_output TokensBefore = %d, want 1000", buildCat.TokensBefore)
	}
	if buildCat.Count != 1 {
		t.Errorf("build_output Count = %d, want 1", buildCat.Count)
	}
}

func TestRecord_Concurrent(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()

	const goroutines = 100
	const recordsPerGoroutine = 50
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				tr.Record("generic", 100, 10)
			}
		}()
	}
	wg.Wait()

	stats := tr.GetSessionStats()
	expectedCompressions := goroutines * recordsPerGoroutine
	if stats.TotalCompressions != expectedCompressions {
		t.Errorf("TotalCompressions = %d, want %d", stats.TotalCompressions, expectedCompressions)
	}
	expectedSaved := expectedCompressions * 90
	if stats.TotalTokensSaved != expectedSaved {
		t.Errorf("TotalTokensSaved = %d, want %d", stats.TotalTokensSaved, expectedSaved)
	}
}

func TestGetSessionSummary_Empty(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()

	summary := tr.GetSessionSummary()
	if summary != "" {
		t.Errorf("expected empty summary for no records, got %q", summary)
	}
}

func TestGetSessionSummary_WithData(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()

	tr.Record("test_output", 40000, 400)
	tr.Record("build_output", 6000, 200)
	tr.Record("search", 3200, 600)

	summary := tr.GetSessionSummary()
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}

	// Must contain the header line with total tokens saved and compression percentage.
	if !strings.Contains(summary, "Kaboom saved") {
		t.Errorf("summary missing 'Kaboom saved' header: %s", summary)
	}
	// Must contain category breakdowns.
	if !strings.Contains(summary, "Test output") {
		t.Errorf("summary missing 'Test output' line: %s", summary)
	}
	if !strings.Contains(summary, "Build output") {
		t.Errorf("summary missing 'Build output' line: %s", summary)
	}
	if !strings.Contains(summary, "Search") {
		t.Errorf("summary missing 'Search' line: %s", summary)
	}
	// Must contain percentage indicators.
	if !strings.Contains(summary, "%") {
		t.Errorf("summary missing percentage: %s", summary)
	}
}

func TestGetSessionStats_CompressionPct(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()

	tr.Record("generic", 1000, 100)

	stats := tr.GetSessionStats()
	// 900 saved out of 1000 = 90%
	if stats.CompressionPct < 89.9 || stats.CompressionPct > 90.1 {
		t.Errorf("CompressionPct = %f, want ~90.0", stats.CompressionPct)
	}
}

func TestSaveLoadLifetime(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "lifetime.json")

	tr := NewTokenTracker()
	tr.Record("test_output", 2000, 50)
	tr.Record("build_output", 1000, 100)

	if err := tr.SaveLifetime(path); err != nil {
		t.Fatalf("SaveLifetime: %v", err)
	}

	loaded, err := LoadLifetime(path)
	if err != nil {
		t.Fatalf("LoadLifetime: %v", err)
	}

	if loaded.TotalTokensSaved != 2850 {
		t.Errorf("TotalTokensSaved = %d, want 2850", loaded.TotalTokensSaved)
	}
	if loaded.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d, want 1", loaded.TotalSessions)
	}
	if loaded.TotalCompressions != 2 {
		t.Errorf("TotalCompressions = %d, want 2", loaded.TotalCompressions)
	}
	if loaded.FirstSession == "" {
		t.Error("FirstSession should not be empty")
	}
	if loaded.LastSession == "" {
		t.Error("LastSession should not be empty")
	}

	// Verify the file is valid JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
}

func TestSaveLifetime_MergesWithExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "lifetime.json")

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)

	// First session with controlled clock.
	tr1 := NewTokenTracker()
	tr1.nowFunc = func() time.Time { return t1 }
	tr1.Record("test_output", 2000, 50)
	if err := tr1.SaveLifetime(path); err != nil {
		t.Fatalf("SaveLifetime (1st): %v", err)
	}

	loaded1, _ := LoadLifetime(path)
	firstSession := loaded1.FirstSession

	// Second session with a later clock.
	tr2 := NewTokenTracker()
	tr2.nowFunc = func() time.Time { return t2 }
	tr2.Record("test_output", 3000, 100)
	tr2.Record("search", 500, 100)
	if err := tr2.SaveLifetime(path); err != nil {
		t.Fatalf("SaveLifetime (2nd): %v", err)
	}

	loaded2, err := LoadLifetime(path)
	if err != nil {
		t.Fatalf("LoadLifetime: %v", err)
	}

	// Accumulated: 1950 + 2900 + 400 = 5250
	if loaded2.TotalTokensSaved != 5250 {
		t.Errorf("TotalTokensSaved = %d, want 5250", loaded2.TotalTokensSaved)
	}
	if loaded2.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d, want 2", loaded2.TotalSessions)
	}
	if loaded2.TotalCompressions != 3 {
		t.Errorf("TotalCompressions = %d, want 3", loaded2.TotalCompressions)
	}
	if loaded2.FirstSession != firstSession {
		t.Errorf("FirstSession changed: was %q, now %q", firstSession, loaded2.FirstSession)
	}
	if loaded2.LastSession == firstSession {
		t.Error("LastSession should differ from FirstSession after second save")
	}
	if loaded2.LastSession != t2.UTC().Format(time.RFC3339) {
		t.Errorf("LastSession = %q, want %q", loaded2.LastSession, t2.UTC().Format(time.RFC3339))
	}

	// Category accumulation check.
	testCat := loaded2.ByCategory["test_output"]
	if testCat.TokensBefore != 5000 {
		t.Errorf("test_output TokensBefore = %d, want 5000", testCat.TokensBefore)
	}
	if testCat.Count != 2 {
		t.Errorf("test_output Count = %d, want 2", testCat.Count)
	}
	searchCat := loaded2.ByCategory["search"]
	if searchCat.Count != 1 {
		t.Errorf("search Count = %d, want 1", searchCat.Count)
	}
}

func TestSaveLifetime_CreatesDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "lifetime.json")

	tr := NewTokenTracker()
	tr.Record("generic", 1000, 100)

	if err := tr.SaveLifetime(path); err != nil {
		t.Fatalf("SaveLifetime with nested dirs: %v", err)
	}

	// Verify file exists and is readable.
	loaded, err := LoadLifetime(path)
	if err != nil {
		t.Fatalf("LoadLifetime: %v", err)
	}
	if loaded.TotalTokensSaved != 900 {
		t.Errorf("TotalTokensSaved = %d, want 900", loaded.TotalTokensSaved)
	}
}

func TestReset(t *testing.T) {
	t.Parallel()
	tr := NewTokenTracker()

	tr.Record("test_output", 2000, 50)
	tr.Record("build_output", 1000, 100)

	tr.Reset()

	stats := tr.GetSessionStats()
	if stats.TotalTokensSaved != 0 {
		t.Errorf("after reset, TotalTokensSaved = %d, want 0", stats.TotalTokensSaved)
	}
	if stats.TotalCompressions != 0 {
		t.Errorf("after reset, TotalCompressions = %d, want 0", stats.TotalCompressions)
	}
	if len(stats.ByCategory) != 0 {
		t.Errorf("after reset, ByCategory has %d entries, want 0", len(stats.ByCategory))
	}

	summary := tr.GetSessionSummary()
	if summary != "" {
		t.Errorf("after reset, expected empty summary, got %q", summary)
	}
}

func TestLoadLifetime_FileNotFound(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	loaded, err := LoadLifetime(path)
	if err != nil {
		t.Fatalf("LoadLifetime should not error on missing file: %v", err)
	}
	if loaded.TotalTokensSaved != 0 {
		t.Errorf("TotalTokensSaved = %d, want 0 for missing file", loaded.TotalTokensSaved)
	}
	if loaded.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d, want 0 for missing file", loaded.TotalSessions)
	}
}
