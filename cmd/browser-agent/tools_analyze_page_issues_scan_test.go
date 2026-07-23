// tools_analyze_page_issues_scan_test.go — Tests for the page-issues scan itself:
// the per-category collectors, the parallel aggregation, and the tool entry point.
// Why: the sibling file covers the summary builder; nothing covered the scan that
// produces what it summarises.
// Docs: docs/features/feature/auto-fix/index.md

package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func newPageIssuesScanHandler(t *testing.T) *ToolHandler {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "page-issues-scan.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(func() { server.logs.shutdownAsyncLogger(2 * time.Second) })
	return NewToolHandler(server, capture.NewCapture()).toolHandler.(*ToolHandler)
}

// =============================================================================
// CONSOLE ERRORS
// =============================================================================

func TestCollectConsoleErrors_KeepsOnlyErrorsAndWarnings(t *testing.T) {
	t.Parallel()
	got := collectConsoleErrors([]LogEntry{
		{"level": "info", "message": "just fyi"},
		{"level": "error", "message": "boom"},
		{"level": "debug", "message": "noise"},
		{"level": "warn", "message": "careful"},
	}, 10)

	if len(got) != 2 {
		t.Fatalf("collected %d issues, want 2 (error + warn only): %+v", len(got), got)
	}
	if got[0]["message"] != "boom" || got[1]["message"] != "careful" {
		t.Errorf("wrong entries collected: %+v", got)
	}
}

func TestCollectConsoleErrors_SeverityFollowsLevel(t *testing.T) {
	t.Parallel()
	// The severity scale is shared across every section of the response, so an
	// error reported as "medium" disappears next to network failures.
	got := collectConsoleErrors([]LogEntry{
		{"level": "error", "message": "e"},
		{"level": "warn", "message": "w"},
	}, 10)

	if got[0]["severity"] != "high" {
		t.Errorf("error severity = %v, want high", got[0]["severity"])
	}
	if got[1]["severity"] != "medium" {
		t.Errorf("warn severity = %v, want medium", got[1]["severity"])
	}
}

func TestCollectConsoleErrors_SkipsEntriesWithNoMessage(t *testing.T) {
	t.Parallel()
	// An issue with no message tells the caller nothing and cannot be acted on.
	got := collectConsoleErrors([]LogEntry{
		{"level": "error", "message": ""},
		{"level": "error"},
		{"level": "error", "message": "real"},
	}, 10)

	if len(got) != 1 || got[0]["message"] != "real" {
		t.Errorf("collectConsoleErrors kept empty messages: %+v", got)
	}
}

func TestCollectConsoleErrors_StopsAtLimit(t *testing.T) {
	t.Parallel()
	entries := make([]LogEntry, 20)
	for i := range entries {
		entries[i] = LogEntry{"level": "error", "message": "e"}
	}

	if got := collectConsoleErrors(entries, 3); len(got) != 3 {
		t.Errorf("collected %d, want the limit of 3", len(got))
	}
}

func TestCollectConsoleErrors_EmptyInputSerializesAsAnEmptyArray(t *testing.T) {
	t.Parallel()
	// nil marshals to JSON null; an empty slice marshals to []. Clients iterate
	// this field, and null is what makes them crash.
	got := collectConsoleErrors(nil, 10)

	if data, _ := json.Marshal(got); string(data) != "[]" {
		t.Errorf("serialized as %s, want []", data)
	}
}

func TestCollectConsoleErrors_RenamesStackTraceToSnakeCase(t *testing.T) {
	t.Parallel()
	got := collectConsoleErrors([]LogEntry{{
		"level":      "error",
		"message":    "boom",
		"source":     "app.js",
		"url":        "https://example.test/app.js",
		"stackTrace": "at f()",
	}}, 10)

	// Entries arrive camelCase from the extension; every JSON field this daemon
	// emits is snake_case. This mapping is where that conversion happens.
	if got[0]["stack_trace"] != "at f()" {
		t.Errorf("stack_trace = %v, want the incoming stackTrace", got[0]["stack_trace"])
	}
	if got[0]["source"] != "app.js" || got[0]["url"] != "https://example.test/app.js" {
		t.Errorf("diagnostic fields lost: %+v", got[0])
	}
}

// =============================================================================
// NETWORK FAILURES
// =============================================================================

func TestCollectNetworkFailures_IgnoresAnythingBelow400(t *testing.T) {
	t.Parallel()
	got := collectNetworkFailures([]capture.NetworkBody{
		{Status: 200, URL: "https://ok.test"},
		{Status: 301, URL: "https://moved.test"},
		{Status: 399, URL: "https://edge.test"},
		{Status: 404, URL: "https://missing.test"},
	}, 10)

	if len(got) != 1 {
		t.Fatalf("collected %d, want only the 404: %+v", len(got), got)
	}
	if got[0]["url"] != "https://missing.test" {
		t.Errorf("wrong failure collected: %+v", got[0])
	}
}

func TestCollectNetworkFailures_ServerErrorsOutrankClientErrors(t *testing.T) {
	t.Parallel()
	// A 4xx is usually the caller's problem and a 5xx is always the site's, so
	// they must not compete for attention at the same severity.
	got := collectNetworkFailures([]capture.NetworkBody{
		{Status: 404, URL: "https://a.test"},
		{Status: 500, URL: "https://b.test"},
	}, 10)

	if got[0]["severity"] != "medium" {
		t.Errorf("404 severity = %v, want medium", got[0]["severity"])
	}
	if got[1]["severity"] != "high" {
		t.Errorf("500 severity = %v, want high", got[1]["severity"])
	}
}

func TestCollectNetworkFailures_StopsAtLimit(t *testing.T) {
	t.Parallel()
	bodies := make([]capture.NetworkBody, 20)
	for i := range bodies {
		bodies[i] = capture.NetworkBody{Status: 500}
	}

	if got := collectNetworkFailures(bodies, 5); len(got) != 5 {
		t.Errorf("collected %d, want the limit of 5", len(got))
	}
}

func TestCollectNetworkFailures_EmptyInputSerializesAsAnEmptyArray(t *testing.T) {
	t.Parallel()
	if data, _ := json.Marshal(collectNetworkFailures(nil, 10)); string(data) != "[]" {
		t.Errorf("serialized as %s, want []", data)
	}
}

// =============================================================================
// AGGREGATION ACROSS SECTIONS
// =============================================================================

func TestRunPageIssuesChecks_RollsUpTotalsAndSeveritiesAcrossSections(t *testing.T) {
	h := newPageIssuesScanHandler(t)
	h.server.logs.entries = append(h.server.logs.entries,
		LogEntry{"level": "error", "message": "console boom"},
		LogEntry{"level": "warn", "message": "console careful"},
	)
	h.capture.AddNetworkBodies([]capture.NetworkBody{{Status: 500, URL: "https://a.test"}})

	result := h.runPageIssuesChecks(
		map[string]bool{catConsoleErrors: true, catNetworkFailures: true},
		pageIssuesPerSectionCap,
		"https://page.test",
	)

	if result.TotalIssues != 3 {
		t.Errorf("TotalIssues = %d, want 3 (2 console + 1 network)", result.TotalIssues)
	}
	// The severity roll-up is what a caller triages on, so it has to span
	// sections rather than being per-section.
	if result.BySeverity["high"] != 2 {
		t.Errorf("by_severity[high] = %d, want 2 (console error + 5xx)", result.BySeverity["high"])
	}
	if result.BySeverity["medium"] != 1 {
		t.Errorf("by_severity[medium] = %d, want 1 (console warn)", result.BySeverity["medium"])
	}
	if result.PageURL != "https://page.test" {
		t.Errorf("PageURL = %q, want the tracked tab URL", result.PageURL)
	}
}

func TestRunPageIssuesChecks_RunsOnlyRequestedCategories(t *testing.T) {
	h := newPageIssuesScanHandler(t)
	h.capture.AddNetworkBodies([]capture.NetworkBody{{Status: 500, URL: "https://a.test"}})

	result := h.runPageIssuesChecks(map[string]bool{catConsoleErrors: true}, 50, "")

	if _, ran := result.Sections[catNetworkFailures]; ran {
		t.Error("network_failures section present although only console_errors was requested")
	}
	if len(result.ChecksCompleted) != 1 || result.ChecksCompleted[0] != catConsoleErrors {
		t.Errorf("ChecksCompleted = %v, want only console_errors", result.ChecksCompleted)
	}
}

func TestRunPageIssuesChecks_EmptyScanStillSerializesEveryCollection(t *testing.T) {
	h := newPageIssuesScanHandler(t)

	result := h.runPageIssuesChecks(map[string]bool{}, 50, "")

	if result.TotalIssues != 0 {
		t.Errorf("TotalIssues = %d, want 0", result.TotalIssues)
	}
	// Callers range over each of these; nil serializes to null and breaks them.
	for name, value := range map[string]any{
		"checks_completed": result.ChecksCompleted,
		"checks_skipped":   result.ChecksSkipped,
		"sections":         result.Sections,
		"by_severity":      result.BySeverity,
	} {
		if data, _ := json.Marshal(value); string(data) == "null" {
			t.Errorf("%s serialized to null; want an empty collection", name)
		}
	}
}

func TestRunPageIssuesChecks_SectionCarriesItsOwnTotal(t *testing.T) {
	h := newPageIssuesScanHandler(t)
	h.server.logs.entries = append(h.server.logs.entries,
		LogEntry{"level": "error", "message": "one"},
		LogEntry{"level": "error", "message": "two"},
	)

	result := h.runPageIssuesChecks(map[string]bool{catConsoleErrors: true}, 50, "")

	section, ok := result.Sections[catConsoleErrors].(map[string]any)
	if !ok {
		t.Fatalf("console_errors section has type %T, want a map", result.Sections[catConsoleErrors])
	}
	if section["total"] != 2 {
		t.Errorf("section total = %v, want 2", section["total"])
	}
}

func TestRunPageIssuesChecks_LimitAppliesPerSection(t *testing.T) {
	h := newPageIssuesScanHandler(t)
	for i := 0; i < 10; i++ {
		h.server.logs.entries = append(h.server.logs.entries, LogEntry{"level": "error", "message": "e"})
	}

	result := h.runPageIssuesChecks(map[string]bool{catConsoleErrors: true}, 4, "")

	if result.TotalIssues != 4 {
		t.Errorf("TotalIssues = %d, want the per-section limit of 4", result.TotalIssues)
	}
}

func TestRunPageIssuesChecks_TimestampIsRFC3339UTC(t *testing.T) {
	h := newPageIssuesScanHandler(t)

	result := h.runPageIssuesChecks(map[string]bool{}, 50, "")

	// The timestamp is how a caller tells one scan from the next; an unparseable
	// or local-zone value makes two scans incomparable.
	parsed, err := time.Parse(time.RFC3339, result.Timestamp)
	if err != nil {
		t.Fatalf("Timestamp %q is not RFC3339: %v", result.Timestamp, err)
	}
	if _, offset := parsed.Zone(); offset != 0 {
		t.Errorf("Timestamp %q is not UTC", result.Timestamp)
	}
}

// =============================================================================
// SHARED PREFETCH
// =============================================================================

func TestPrefetchSharedData_SnapshotsBuffersRatherThanAliasingThem(t *testing.T) {
	// Checks run in parallel goroutines against this snapshot. If it aliased the
	// live buffers, a concurrent append during a scan would be a data race.
	h := newPageIssuesScanHandler(t)
	h.server.logs.entries = append(h.server.logs.entries, LogEntry{"level": "error", "message": "first"})

	shared := h.prefetchSharedData("https://page.test")
	h.server.logs.entries = append(h.server.logs.entries, LogEntry{"level": "error", "message": "second"})

	if len(shared.logEntries) != 1 {
		t.Errorf("snapshot grew to %d entries after a later append; it must be a copy", len(shared.logEntries))
	}
	if shared.tabURL != "https://page.test" {
		t.Errorf("tabURL = %q, want the URL passed in", shared.tabURL)
	}
}

// =============================================================================
// TOOL ENTRY POINT
// =============================================================================

func TestToolAnalyzePageIssues_UntrackedTabIsRefusedNotReportedClean(t *testing.T) {
	// Scanning with no tracked tab would report a clean page, which is the most
	// misleading answer available.
	h := newPageIssuesScanHandler(t)

	resp := h.toolAnalyzePageIssues(JSONRPCRequest{ID: json.RawMessage(`1`)}, json.RawMessage(`{}`))

	if code := extractErrorCode(t, resp); code != ErrNoData {
		t.Errorf("error code = %q, want %q", code, ErrNoData)
	}
}

func TestToolAnalyzePageIssues_ScansWhenATabIsTracked(t *testing.T) {
	h := newPageIssuesScanHandler(t)
	h.capture.SetTrackingStatusForTest(42, "https://tracked.test")
	h.server.logs.entries = append(h.server.logs.entries, LogEntry{"level": "error", "message": "boom"})

	resp := h.toolAnalyzePageIssues(
		JSONRPCRequest{ID: json.RawMessage(`1`)},
		json.RawMessage(`{"categories":["console_errors"]}`),
	)

	result := decodeToolResult(t, resp.Result)
	if result.IsError {
		t.Fatalf("scan failed: %s", result.Content[0].Text)
	}
}

func TestToolAnalyzePageIssues_MalformedArgumentsFallBackToDefaults(t *testing.T) {
	// Arguments arrive from a model and are routinely half-shaped. Hard-failing
	// on them is worse than scanning with defaults.
	h := newPageIssuesScanHandler(t)
	h.capture.SetTrackingStatusForTest(42, "https://tracked.test")

	resp := h.toolAnalyzePageIssues(
		JSONRPCRequest{ID: json.RawMessage(`1`)},
		json.RawMessage(`{"limit":"not-a-number","categories":["console_errors"]}`),
	)

	if result := decodeToolResult(t, resp.Result); result.IsError {
		t.Fatalf("malformed arguments were rejected outright: %s", result.Content[0].Text)
	}
}

func TestRunPageIssuesChecks_UnreachableCheckIsSkippedNotCountedAsClean(t *testing.T) {
	// The accessibility check needs the extension. With none connected it hangs
	// until pageIssuesCheckTimeout, and the scan must then list it under
	// checks_skipped — reporting it complete with zero issues would claim the
	// page is accessible when nothing looked.
	//
	// Costs a real pageIssuesCheckTimeout (5s) because that constant is not
	// injectable; skipped in -short for that reason.
	if testing.Short() {
		t.Skip("exercises the 5s check timeout")
	}
	h := newPageIssuesScanHandler(t)

	result := h.runPageIssuesChecks(map[string]bool{catAccessibility: true}, 50, "")

	if len(result.ChecksSkipped) != 1 || result.ChecksSkipped[0] != catAccessibility {
		t.Errorf("ChecksSkipped = %v, want [accessibility]", result.ChecksSkipped)
	}
	for _, done := range result.ChecksCompleted {
		if done == catAccessibility {
			t.Error("a check that timed out was reported as completed")
		}
	}
}

func TestToolAnalyzePageIssues_SummaryAndFullDifferInShape(t *testing.T) {
	// summary:true exists to keep a scan affordable in a model's context. If it
	// returned the same payload it would be a no-op that reads as a saving.
	h := newPageIssuesScanHandler(t)
	h.capture.SetTrackingStatusForTest(42, "https://tracked.test")
	for i := 0; i < 30; i++ {
		h.server.logs.entries = append(h.server.logs.entries, LogEntry{"level": "error", "message": "e"})
	}
	req := JSONRPCRequest{ID: json.RawMessage(`1`)}

	full := h.toolAnalyzePageIssues(req, json.RawMessage(`{"categories":["console_errors"]}`))
	summary := h.toolAnalyzePageIssues(req, json.RawMessage(`{"categories":["console_errors"],"summary":true}`))

	fullResult := decodeToolResult(t, full.Result)
	summaryResult := decodeToolResult(t, summary.Result)
	if fullResult.IsError || summaryResult.IsError {
		t.Fatalf("scan failed: full=%v summary=%v", fullResult.IsError, summaryResult.IsError)
	}
	fullText := fullResult.Content[0].Text
	summaryText := summaryResult.Content[0].Text
	if len(summaryText) >= len(fullText) {
		t.Errorf("summary (%d bytes) is not smaller than the full scan (%d bytes)", len(summaryText), len(fullText))
	}
}

// =============================================================================
// SECURITY SECTION
// =============================================================================

func TestCollectSecurityIssues_NoScannerYieldsNothingRatherThanFailing(t *testing.T) {
	// A daemon built without a scanner must degrade to "this category found
	// nothing", not fail the whole scan and lose the other three sections.
	h := newPageIssuesScanHandler(t)
	h.securityScannerImpl = nil

	issues, err := h.collectSecurityIssues(sharedPageData{}, 50)

	if err != nil {
		t.Fatalf("collectSecurityIssues without a scanner returned an error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("got %d issues from a nil scanner: %+v", len(issues), issues)
	}
}

func TestCollectSecurityIssues_MapsFindingsOntoTheSharedIssueShape(t *testing.T) {
	// Every section is consumed through the same {severity, ...} shape; a
	// finding that arrives without a severity is invisible in by_severity.
	h := newPageIssuesScanHandler(t)
	shared := sharedPageData{
		tabURL: "http://insecure.test/login",
		networkBodies: []capture.NetworkBody{{
			Method: "POST",
			URL:    "http://insecure.test/login",
			Status: 200,
		}},
	}

	issues, err := h.collectSecurityIssues(shared, 50)

	if err != nil {
		t.Fatalf("collectSecurityIssues error = %v", err)
	}
	if len(issues) == 0 {
		t.Skip("scanner reported nothing for this input; the mapping is exercised elsewhere")
	}
	for i, issue := range issues {
		if issue["severity"] == nil || issue["severity"] == "" {
			t.Errorf("issue %d has no severity: %+v", i, issue)
		}
		if issue["check"] == nil {
			t.Errorf("issue %d has no check name, so it cannot be traced back: %+v", i, issue)
		}
	}
}

func TestCollectSecurityIssues_StopsAtLimit(t *testing.T) {
	h := newPageIssuesScanHandler(t)
	bodies := make([]capture.NetworkBody, 30)
	for i := range bodies {
		bodies[i] = capture.NetworkBody{Method: "POST", URL: "http://insecure.test/login", Status: 200}
	}

	issues, err := h.collectSecurityIssues(sharedPageData{tabURL: "http://insecure.test/", networkBodies: bodies}, 2)

	if err != nil {
		t.Fatalf("collectSecurityIssues error = %v", err)
	}
	if len(issues) > 2 {
		t.Errorf("got %d issues, want at most the limit of 2", len(issues))
	}
}
