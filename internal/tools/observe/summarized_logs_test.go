// Purpose: Tests for log summarization and grouping.
// Docs: docs/features/feature/observe/index.md

// summarized_logs_test.go — Tests for log aggregation fingerprinting and grouping.
package observe

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestGetSummarizedLogsFiltersAndBuildsWireResponse(t *testing.T) {
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	cap.Extension().SetTrackingStatusForTest(7, "https://app.example.test")
	entries := []types.LogEntry{
		{"type": "lifecycle", "message": "ignored"},
		{"level": "warn", "message": "noisy", "tabId": float64(7), "url": "https://app.example.test", "ts": "2026-01-01T00:00:00Z"},
		{"level": "info", "message": "wrong level", "tabId": float64(7), "url": "https://app.example.test", "ts": "2026-01-01T00:00:01Z"},
		{"level": "error", "message": "wrong tab", "tabId": float64(9), "url": "https://app.example.test", "ts": "2026-01-01T00:00:02Z"},
		{"level": "error", "message": "Request 1001 failed", "source": "console", "tabId": float64(7), "url": "https://app.example.test/a", "ts": "2026-01-01T00:00:03Z"},
		{"level": "error", "message": "Request 1002 failed", "source": "console", "tabId": float64(7), "url": "https://app.example.test/b", "ts": "2026-01-01T00:00:04Z"},
	}
	deps := Deps{
		Capture: cap,
		LogEntries: func() ([]types.LogEntry, []time.Time) {
			return entries, nil
		},
		IsConsoleNoise: func(entry types.LogEntry) bool {
			return entry["message"] == "noisy"
		},
	}
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`7`)}

	resp := GetSummarizedLogs(deps, req, json.RawMessage(`{
		"scope":"current_page",
		"min_level":"warn",
		"source":"console",
		"min_group_size":2
	}`))
	id, ok := resp.ID.(json.RawMessage)
	if !ok || string(id) != "7" {
		t.Fatalf("response id = %v, want raw 7", resp.ID)
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	text := result.Content[0].Text
	for _, want := range []string{
		`"total_entries":2`,
		`"noise_suppressed":1`,
		`"groups":1`,
		`"start":"2026-01-01T00:00:03Z"`,
		`"end":"2026-01-01T00:00:04Z"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("response missing %s: %s", want, text)
		}
	}
}

func TestGetSummarizedLogsRejectsUnknownScope(t *testing.T) {
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	resp := GetSummarizedLogs(
		Deps{Capture: cap},
		mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`8`)},
		json.RawMessage(`{"scope":"workspace"}`),
	)
	if !strings.Contains(string(resp.Result), "Invalid scope") {
		t.Fatalf("unexpected result: %s", resp.Result)
	}
}

// ============================================
// Fingerprinting Tests
// ============================================

func TestFingerprint_UUID(t *testing.T) {
	t.Parallel()
	msg := "Connection abc12345-def6-7890-abcd-ef1234567890 established"
	fp := fingerprintMessage(msg)
	if strings.Contains(fp, "abc12345") {
		t.Errorf("fingerprint should not contain UUID: %s", fp)
	}
}

func TestFingerprint_Numbers(t *testing.T) {
	t.Parallel()
	fp1 := fingerprintMessage("Request took 1234ms")
	fp2 := fingerprintMessage("Request took 5678ms")
	if fp1 != fp2 {
		t.Errorf("fingerprints should match for different numbers: %q vs %q", fp1, fp2)
	}
}

func TestFingerprint_HexHash(t *testing.T) {
	t.Parallel()
	fp1 := fingerprintMessage("Module loaded: abcdef1234567890")
	fp2 := fingerprintMessage("Module loaded: 1234567890abcdef")
	if fp1 != fp2 {
		t.Errorf("fingerprints should match for different hex hashes: %q vs %q", fp1, fp2)
	}
}

func TestFingerprint_Timestamps(t *testing.T) {
	t.Parallel()
	fp1 := fingerprintMessage("Event at 2026-02-20T10:00:01Z")
	fp2 := fingerprintMessage("Event at 2026-02-20T14:30:00Z")
	if fp1 != fp2 {
		t.Errorf("fingerprints should match for different timestamps: %q vs %q", fp1, fp2)
	}
}

func TestFingerprint_URLs(t *testing.T) {
	t.Parallel()
	fp1 := fingerprintMessage("Fetched https://api.example.com/users/123")
	fp2 := fingerprintMessage("Fetched https://api.example.com/items/456")
	if fp1 != fp2 {
		t.Errorf("fingerprints should match for different URLs: %q vs %q", fp1, fp2)
	}
}

func TestFingerprint_QuotedStrings(t *testing.T) {
	t.Parallel()
	fp1 := fingerprintMessage(`Error: "this is a very long error message that should be replaced"`)
	fp2 := fingerprintMessage(`Error: "completely different long string that should also match here"`)
	if fp1 != fp2 {
		t.Errorf("fingerprints should match for different long quoted strings: %q vs %q", fp1, fp2)
	}
}

func TestFingerprint_FilePaths(t *testing.T) {
	t.Parallel()
	fp1 := fingerprintMessage("Loading /Users/bob/src/app/main.js")
	fp2 := fingerprintMessage("Loading /Users/alice/lib/util/helper.js")
	if fp1 != fp2 {
		t.Errorf("fingerprints should match for different paths: %q vs %q", fp1, fp2)
	}
}

func TestFingerprint_Slugification(t *testing.T) {
	t.Parallel()
	fp := fingerprintMessage("WebSocket heartbeat acknowledged")
	if len(fp) > 64 {
		t.Errorf("fingerprint should be truncated to 64 chars, got %d", len(fp))
	}
	// Should be lowercase with underscores
	for _, ch := range fp {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_') {
			t.Errorf("fingerprint contains invalid char %c: %s", ch, fp)
			break
		}
	}
}

func TestFingerprint_EmptyMessage(t *testing.T) {
	t.Parallel()
	fp := fingerprintMessage("")
	if fp != "" {
		t.Errorf("empty message should produce empty fingerprint, got %q", fp)
	}
}

// ============================================
// Grouping Tests
// ============================================

func TestGroupLogs_IdenticalMessages(t *testing.T) {
	t.Parallel()
	entries := makeLogEntries(
		logEntry("log", "heartbeat ack", "ws.js"),
		logEntry("log", "heartbeat ack", "ws.js"),
		logEntry("log", "heartbeat ack", "ws.js"),
	)
	groups, anomalies := groupLogs(entries, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Count != 3 {
		t.Errorf("expected count=3, got %d", groups[0].Count)
	}
	if len(anomalies) != 0 {
		t.Errorf("expected 0 anomalies, got %d", len(anomalies))
	}
}

func TestGroupLogs_VariableContent(t *testing.T) {
	t.Parallel()
	entries := makeLogEntries(
		logEntry("log", "User 123 logged in", "auth.js"),
		logEntry("log", "User 456 logged in", "auth.js"),
		logEntry("log", "User 789 logged in", "auth.js"),
	)
	groups, anomalies := groupLogs(entries, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group (variable content collapsed), got %d", len(groups))
	}
	if groups[0].Count != 3 {
		t.Errorf("expected count=3, got %d", groups[0].Count)
	}
	if len(anomalies) != 0 {
		t.Errorf("expected 0 anomalies, got %d", len(anomalies))
	}
}

func TestGroupLogs_Anomalies(t *testing.T) {
	t.Parallel()
	entries := makeLogEntries(
		logEntry("log", "heartbeat", "ws.js"),
		logEntry("log", "heartbeat", "ws.js"),
		logEntry("warn", "Something unusual happened", "app.js"),
	)
	groups, anomalies := groupLogs(entries, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(anomalies) != 1 {
		t.Fatalf("expected 1 anomaly, got %d", len(anomalies))
	}
}

func TestGroupLogs_AllUnique(t *testing.T) {
	t.Parallel()
	entries := makeLogEntries(
		logEntry("log", "first message", "a.js"),
		logEntry("warn", "second message", "b.js"),
		logEntry("error", "third message", "c.js"),
	)
	groups, anomalies := groupLogs(entries, 2)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
	if len(anomalies) != 3 {
		t.Errorf("expected 3 anomalies, got %d", len(anomalies))
	}
}

func TestGroupLogs_AllIdentical(t *testing.T) {
	t.Parallel()
	entries := makeLogEntries(
		logEntry("log", "same", "x.js"),
		logEntry("log", "same", "x.js"),
		logEntry("log", "same", "x.js"),
	)
	groups, anomalies := groupLogs(entries, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(anomalies) != 0 {
		t.Errorf("expected 0 anomalies, got %d", len(anomalies))
	}
}

func TestGroupLogs_MinGroupSize1(t *testing.T) {
	t.Parallel()
	entries := makeLogEntries(
		logEntry("log", "unique", "x.js"),
	)
	groups, anomalies := groupLogs(entries, 1)
	if len(groups) != 1 {
		t.Errorf("expected 1 group with min_group_size=1, got %d", len(groups))
	}
	if len(anomalies) != 0 {
		t.Errorf("expected 0 anomalies with min_group_size=1, got %d", len(anomalies))
	}
}

func TestGroupLogs_LevelBreakdown(t *testing.T) {
	t.Parallel()
	entries := makeLogEntries(
		logEntry("log", "same message", "x.js"),
		logEntry("warn", "same message", "x.js"),
		logEntry("log", "same message", "x.js"),
	)
	groups, _ := groupLogs(entries, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	breakdown := groups[0].LevelBreakdown
	if breakdown["log"] != 2 {
		t.Errorf("expected log=2, got %d", breakdown["log"])
	}
	if breakdown["warn"] != 1 {
		t.Errorf("expected warn=1, got %d", breakdown["warn"])
	}
}

func TestGroupLogs_MultipleSources(t *testing.T) {
	t.Parallel()
	entries := makeLogEntries(
		logEntry("log", "same message", "a.js"),
		logEntry("log", "same message", "b.js"),
		logEntry("log", "same message", "a.js"),
	)
	groups, _ := groupLogs(entries, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Sources) != 2 {
		t.Errorf("expected 2 sources, got %d: %v", len(groups[0].Sources), groups[0].Sources)
	}
}

func TestGroupLogs_Empty(t *testing.T) {
	t.Parallel()
	groups, anomalies := groupLogs(nil, 2)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
	if len(anomalies) != 0 {
		t.Errorf("expected 0 anomalies, got %d", len(anomalies))
	}
}

// ============================================
// Periodicity Tests
// ============================================

func TestDetectPeriodicity_Regular(t *testing.T) {
	t.Parallel()
	entries := makeLogEntriesWithTimestamps(
		logEntryTS("log", "heartbeat", "ws.js", "2026-02-20T10:00:01Z"),
		logEntryTS("log", "heartbeat", "ws.js", "2026-02-20T10:00:04Z"),
		logEntryTS("log", "heartbeat", "ws.js", "2026-02-20T10:00:07Z"),
		logEntryTS("log", "heartbeat", "ws.js", "2026-02-20T10:00:10Z"),
	)
	groups, _ := groupLogs(entries, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	detectPeriodicity(groups)
	if !groups[0].IsPeriodic {
		t.Error("expected group to be periodic")
	}
	// ~3 seconds between entries
	if groups[0].PeriodSeconds < 2.5 || groups[0].PeriodSeconds > 3.5 {
		t.Errorf("expected period ~3s, got %f", groups[0].PeriodSeconds)
	}
}

func TestDetectPeriodicity_Irregular(t *testing.T) {
	t.Parallel()
	entries := makeLogEntriesWithTimestamps(
		logEntryTS("log", "event", "x.js", "2026-02-20T10:00:01Z"),
		logEntryTS("log", "event", "x.js", "2026-02-20T10:00:02Z"),
		logEntryTS("log", "event", "x.js", "2026-02-20T10:00:20Z"),
		logEntryTS("log", "event", "x.js", "2026-02-20T10:00:21Z"),
	)
	groups, _ := groupLogs(entries, 2)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	detectPeriodicity(groups)
	if groups[0].IsPeriodic {
		t.Error("expected group to NOT be periodic (high jitter)")
	}
}

// ============================================
// Test Helpers
// ============================================

type testEntry struct {
	level   string
	message string
	source  string
	ts      string
}

func logEntry(level, message, source string) testEntry {
	return testEntry{level: level, message: message, source: source}
}

func logEntryTS(level, message, source, ts string) testEntry {
	return testEntry{level: level, message: message, source: source, ts: ts}
}

func makeLogEntries(entries ...testEntry) []logEntryView {
	result := make([]logEntryView, len(entries))
	for i, e := range entries {
		result[i] = logEntryView{
			Level:   e.level,
			Message: e.message,
			Source:  e.source,
			TS:      e.ts,
		}
	}
	return result
}

func makeLogEntriesWithTimestamps(entries ...testEntry) []logEntryView {
	return makeLogEntries(entries...)
}
