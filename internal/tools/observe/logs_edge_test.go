// logs_edge_test.go — Tests log normalization and extension filtering edge contracts.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
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

func TestGetBrowserLogsAppliesCurrentPageFiltersAndIncludesExtensionLogs(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.Extension().SetTrackingStatusForTest(7, "https://app.example.test")
	cap.ExtensionLogs().Add([]types.ExtensionLog{
		{Level: "info", Message: "extension info", Timestamp: time.Now()},
		{Level: "error", Message: "extension error", Timestamp: time.Now()},
	})
	entries := []types.LogEntry{
		{"level": "error", "source": "console", "url": "https://app.example.test/page", "tabId": float64(7), "message": "kept", "ts": "2026-07-29T10:00:00Z"},
		{"level": "warn", "source": "console", "url": "https://app.example.test/page", "tabId": float64(7), "message": "below threshold"},
		{"level": "error", "source": "network", "url": "https://app.example.test/page", "tabId": float64(7), "message": "wrong source"},
		{"level": "error", "source": "console", "url": "https://other.example.test", "tabId": float64(8), "message": "wrong tab"},
		{"type": "lifecycle", "event": "startup", "timestamp": "2026-07-29T09:00:00Z"},
		{"level": "error", "source": "console", "url": "https://app.example.test/page", "tabId": float64(7), "message": "noise"},
	}
	deps := Deps{
		Capture:       cap,
		LogEntries:    func() ([]types.LogEntry, []time.Time) { return entries, nil },
		LogTotalAdded: func() int64 { return int64(len(entries)) },
		IsConsoleNoise: func(entry types.LogEntry) bool {
			return entry["message"] == "noise"
		},
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	resp := GetBrowserLogs(deps, req, json.RawMessage(`{
		"min_level":"error",
		"source":"console",
		"include_extension_logs":true,
		"extension_limit":1
	}`))
	data := extractMCPJSON(t, resp)
	if data["count"] != float64(1) {
		t.Fatalf("count = %v, want 1; response=%#v", data["count"], data)
	}
	logs := data["logs"].([]any)
	if got := logs[0].(map[string]any)["message"]; got != "kept" {
		t.Fatalf("message = %v, want kept", got)
	}
	metadata := data["metadata"].(map[string]any)
	if metadata["noise_suppressed"] != float64(1) || metadata["scope"] != "current_page" {
		t.Fatalf("metadata = %#v", metadata)
	}
	extensionLogs := data["extension_logs"].([]any)
	if len(extensionLogs) != 1 || extensionLogs[0].(map[string]any)["message"] != "extension error" {
		t.Fatalf("extension_logs = %#v", extensionLogs)
	}
}

func TestGetBrowserLogsSummarizesInternalEntriesAndReportsInvalidParameters(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	entries := []types.LogEntry{
		{"type": "lifecycle", "event": "startup", "timestamp": "2026-07-29T09:00:00Z"},
		{"level": "warn", "source": "console", "message": "warning", "ts": "2026-07-29T10:00:00Z"},
	}
	deps := Deps{
		Capture:        cap,
		LogEntries:     func() ([]types.LogEntry, []time.Time) { return entries, nil },
		LogTotalAdded:  func() int64 { return int64(len(entries)) },
		IsConsoleNoise: func(types.LogEntry) bool { return false },
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`2`)}

	resp := GetBrowserLogs(deps, req, json.RawMessage(`{
		"min_level":"critical",
		"scope":"somewhere",
		"include_internal":true,
		"summary":true
	}`))
	data := extractMCPJSON(t, resp)
	if data["total"] != float64(2) {
		t.Fatalf("total = %v, want 2; response=%#v", data["total"], data)
	}
	if hint, _ := data["param_hint"].(string); hint == "" {
		t.Fatalf("missing combined param_hint: %#v", data)
	}
	bySource := data["by_source"].(map[string]any)
	if bySource["daemon"] != float64(1) || bySource["console"] != float64(1) {
		t.Fatalf("by_source = %#v", bySource)
	}
}

func TestGetBrowserLogsRejectsMalformedCursor(t *testing.T) {
	t.Parallel()
	deps := (&mockTransientDeps{cap: capture.NewCapture()}).deps()
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`3`)}

	resp := GetBrowserLogs(deps, req, json.RawMessage(`{"after_cursor":"not-a-cursor"}`))
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected structured cursor error, got %#v", result)
	}
}

func TestGetBrowserErrorsScopesTrackedPageAndSummarizesNoise(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.Extension().SetTrackingStatusForTest(7, "https://app.example.test")
	entries := []types.LogEntry{
		{"level": "error", "source": "console", "url": "https://app.example.test/page", "tabId": float64(7), "message": "kept", "ts": "2026-07-29T10:00:00Z"},
		{"level": "error", "source": "console", "url": "https://app.example.test/page", "tabId": float64(7), "message": "noise"},
		{"level": "error", "source": "console", "url": "https://other.example.test", "tabId": float64(8), "message": "wrong tab"},
		{"level": "warn", "source": "console", "url": "https://app.example.test/page", "tabId": float64(7), "message": "not an error"},
	}
	deps := Deps{
		Capture:    cap,
		LogEntries: func() ([]types.LogEntry, []time.Time) { return entries, nil },
		IsConsoleNoise: func(entry types.LogEntry) bool {
			return entry["message"] == "noise"
		},
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`4`)}

	resp := GetBrowserErrors(deps, req, json.RawMessage(`{"scope":"invalid","summary":true}`))
	data := extractMCPJSON(t, resp)
	if data["total"] != float64(1) || data["noise_suppressed"] != float64(1) {
		t.Fatalf("summary = %#v", data)
	}
	if hint, _ := data["param_hint"].(string); hint == "" {
		t.Fatalf("missing invalid-scope hint: %#v", data)
	}
	top := data["top_messages"].([]any)
	if len(top) != 1 || top[0].(map[string]any)["message"] != "kept" {
		t.Fatalf("top_messages = %#v", top)
	}
}
