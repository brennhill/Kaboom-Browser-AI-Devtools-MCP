// Purpose: Tests for the console-stream observe modes' summary builders and text truncation.
// Docs: docs/features/feature/observe/index.md

package logs

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
)

func TestBuildErrorsSummary_CountsBySource(t *testing.T) {
	t.Parallel()
	errors := []map[string]any{
		{"message": "err1", "source": "console", "timestamp": "2024-01-01T00:00:00Z"},
		{"message": "err2", "source": "console", "timestamp": "2024-01-01T00:00:01Z"},
		{"message": "err3", "source": "network", "timestamp": "2024-01-01T00:00:02Z"},
	}
	meta := core.ResponseMetadata{RetrievedAt: "2024-01-01T00:00:03Z", DataAge: "1.0s"}
	result := buildErrorsSummary(errors, 0, meta)

	total, _ := result["total"].(int)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}

	bySource, ok := result["by_source"].(map[string]int)
	if !ok {
		t.Fatal("by_source not a map[string]int")
	}
	if bySource["console"] != 2 {
		t.Errorf("console count = %d, want 2", bySource["console"])
	}
	if bySource["network"] != 1 {
		t.Errorf("network count = %d, want 1", bySource["network"])
	}
}

func TestBuildErrorsSummary_TopMessages(t *testing.T) {
	t.Parallel()
	errors := make([]map[string]any, 0)
	for i := 0; i < 10; i++ {
		errors = append(errors, map[string]any{"message": "repeated error", "source": "js"})
	}
	for i := 0; i < 3; i++ {
		errors = append(errors, map[string]any{"message": "less common", "source": "js"})
	}
	errors = append(errors, map[string]any{"message": "rare error", "source": "js"})

	result := buildErrorsSummary(errors, 0, core.ResponseMetadata{})
	topMessages, ok := result["top_messages"].([]map[string]any)
	if !ok {
		t.Fatal("top_messages not a []map[string]any")
	}
	if len(topMessages) == 0 {
		t.Fatal("top_messages is empty")
	}
	// First should be the most frequent
	if topMessages[0]["message"] != "repeated error" {
		t.Errorf("first top message = %v, want 'repeated error'", topMessages[0]["message"])
	}
	if topMessages[0]["count"] != 10 {
		t.Errorf("first count = %v, want 10", topMessages[0]["count"])
	}
	// Should be capped at 5
	if len(topMessages) > 5 {
		t.Errorf("top_messages len = %d, want <= 5", len(topMessages))
	}
}

func TestBuildErrorsSummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildErrorsSummary(nil, 0, core.ResponseMetadata{})
	total, _ := result["total"].(int)
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}

func TestBuildLogsSummary_CountsByLevel(t *testing.T) {
	t.Parallel()
	logs := []map[string]any{
		{"level": "info", "message": "a"},
		{"level": "info", "message": "b"},
		{"level": "warn", "message": "c"},
		{"level": "error", "message": "d"},
	}
	result := buildLogsSummary(logs, map[string]any{})

	total, _ := result["total"].(int)
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}

	byLevel, ok := result["by_level"].(map[string]int)
	if !ok {
		t.Fatal("by_level not a map[string]int")
	}
	if byLevel["info"] != 2 {
		t.Errorf("info count = %d, want 2", byLevel["info"])
	}
	if byLevel["warn"] != 1 {
		t.Errorf("warn count = %d, want 1", byLevel["warn"])
	}
	if byLevel["error"] != 1 {
		t.Errorf("error count = %d, want 1", byLevel["error"])
	}
}

func TestBuildLogsSummary_CountsBySource(t *testing.T) {
	t.Parallel()
	logs := []map[string]any{
		{"level": "info", "source": "console"},
		{"level": "info", "source": "console"},
		{"level": "warn", "source": "network"},
	}
	result := buildLogsSummary(logs, map[string]any{})

	bySource, ok := result["by_source"].(map[string]int)
	if !ok {
		t.Fatal("by_source not a map[string]int")
	}
	if bySource["console"] != 2 {
		t.Errorf("console count = %d, want 2", bySource["console"])
	}
}

func TestQuickLogsSummary_ByLevel(t *testing.T) {
	t.Parallel()
	logs := []map[string]any{
		{"level": "info"},
		{"level": "info"},
		{"level": "error"},
	}
	result := quickLogsSummary(logs)

	total, _ := result["total"].(int)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}

	byLevel, ok := result["by_level"].(map[string]int)
	if !ok {
		t.Fatal("by_level not a map[string]int")
	}
	if byLevel["info"] != 2 {
		t.Errorf("info = %d, want 2", byLevel["info"])
	}
	if byLevel["error"] != 1 {
		t.Errorf("error = %d, want 1", byLevel["error"])
	}
}

func TestBuildLogsSummary_EmptySourceBucketed(t *testing.T) {
	t.Parallel()
	logs := []map[string]any{
		{"level": "info", "source": ""},
		{"level": "info", "source": "console"},
	}
	result := buildLogsSummary(logs, map[string]any{})

	bySource, ok := result["by_source"].(map[string]int)
	if !ok {
		t.Fatal("by_source not a map[string]int")
	}
	if bySource["unknown"] != 1 {
		t.Errorf("unknown count = %d, want 1", bySource["unknown"])
	}
	if bySource["console"] != 1 {
		t.Errorf("console count = %d, want 1", bySource["console"])
	}
}

func TestTruncateRunes_UTF8Safe(t *testing.T) {
	t.Parallel()
	// 4-byte emoji chars: each is 1 rune but 4 bytes
	input := "Hello \U0001F600\U0001F600\U0001F600 world"
	result := truncateRunes(input, 9)
	runes := []rune(result)
	if len(runes) != 9 {
		t.Errorf("rune len = %d, want 9", len(runes))
	}
	// Should not have corrupted byte sequences
	for i, r := range runes {
		if r == '\uFFFD' {
			t.Errorf("replacement char at rune %d — truncation corrupted UTF-8", i)
		}
	}
}
