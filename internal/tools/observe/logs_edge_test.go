// logs_edge_test.go — Tests log normalization and extension filtering edge contracts.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestNormalizeBrowserLogEntryPreservesLifecycleContext(t *testing.T) {
	entry := map[string]any{
		"type": "lifecycle", "event": "startup", "timestamp": "2026-01-02T03:04:05Z",
		"pid": 42, "port": 7890, "custom": "value",
	}
	got := normalizeBrowserLogEntry(entry)
	if got["level"] != "info" || got["message"] != "startup" || got["source"] != "daemon" {
		t.Fatalf("normalized = %#v", got)
	}
	if got["type"] != "lifecycle" || got["event"] != "startup" || got["pid"] != 42 || got["port"] != 7890 {
		t.Fatalf("lifecycle fields = %#v", got)
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["custom"] != "value" {
		t.Fatalf("extras = %#v", got["data"])
	}
	if ts := logEntryTimestamp(map[string]any{"ts": "first", "timestamp": "second"}); ts != "first" {
		t.Fatalf("timestamp precedence = %q", ts)
	}
	if ts := logEntryTimestamp(map[string]any{}); ts != "" {
		t.Fatalf("empty timestamp = %q", ts)
	}
}

func TestBuildExtensionLogEntriesFiltersLevelAndLimit(t *testing.T) {
	logs := []types.ExtensionLog{
		{Level: "debug", Message: "debug"},
		{Level: "warn", Message: "warn"},
		{Level: "error", Message: "error"},
	}
	if got := buildExtensionLogEntries(logs, 10, "warn", ""); len(got) != 1 || got[0]["message"] != "warn" {
		t.Fatalf("level filter = %#v", got)
	}
	if got := buildExtensionLogEntries(logs, 1, "", "warn"); len(got) != 1 || got[0]["message"] != "error" {
		t.Fatalf("min level/limit = %#v", got)
	}
}
