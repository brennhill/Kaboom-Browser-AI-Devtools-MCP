// tools_session_audit_reader_test.go — Tests for the adapter that turns live
// handler state into a session snapshot.
// Why: this is what an audit trail records. A wrong or empty answer here is not
// visible at the time — it is only discovered later, when the record is needed.
// Docs: docs/features/feature/enterprise-audit/index.md

package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

func newAuditReaderHandler(t *testing.T) *ToolHandler {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "audit-reader.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	t.Cleanup(func() { server.logs.shutdownAsyncLogger(2 * time.Second) })
	return NewToolHandler(server, capture.NewCapture()).toolHandler.(*ToolHandler)
}

// =============================================================================
// CONSOLE
// =============================================================================

func TestAuditReader_ConsoleErrorsAndWarningsDoNotBleedIntoEachOther(t *testing.T) {
	h := newAuditReaderHandler(t)
	h.server.logs.entries = append(h.server.logs.entries,
		LogEntry{"level": "error", "message": "boom"},
		LogEntry{"level": "warn", "message": "careful"},
		LogEntry{"level": "info", "message": "fyi"},
	)
	r := newToolCaptureStateReader(h)

	errors := r.GetConsoleErrors()
	warnings := r.GetConsoleWarnings()

	if len(errors) != 1 || errors[0].Message != "boom" {
		t.Errorf("GetConsoleErrors = %+v, want only the error", errors)
	}
	if len(warnings) != 1 || warnings[0].Message != "careful" {
		t.Errorf("GetConsoleWarnings = %+v, want only the warning", warnings)
	}
}

func TestAuditReader_WarningsCountBothSpellingsOfTheLevel(t *testing.T) {
	// Browsers and the extension disagree on "warn" vs "warning"; an audit that
	// recognised only one would under-report on whichever page used the other.
	h := newAuditReaderHandler(t)
	h.server.logs.entries = append(h.server.logs.entries,
		LogEntry{"level": "warn", "message": "a"},
		LogEntry{"level": "warning", "message": "b"},
	)

	warnings := newToolCaptureStateReader(h).GetConsoleWarnings()

	if len(warnings) != 2 {
		t.Errorf("collected %d warnings, want both spellings: %+v", len(warnings), warnings)
	}
}

func TestAuditReader_IdenticalMessagesAreCollapsedWithACount(t *testing.T) {
	// A page that logs the same error 500 times must not produce 500 records; the
	// count is the signal, and the repetition is noise.
	h := newAuditReaderHandler(t)
	for i := 0; i < 5; i++ {
		h.server.logs.entries = append(h.server.logs.entries, LogEntry{"level": "error", "message": "same"})
	}

	errors := newToolCaptureStateReader(h).GetConsoleErrors()

	if len(errors) != 1 {
		t.Fatalf("collected %d records, want 1 collapsed record: %+v", len(errors), errors)
	}
	if errors[0].Count != 5 {
		t.Errorf("Count = %d, want 5", errors[0].Count)
	}
}

func TestAuditReader_MessagesAreTrimmedBeforeBeingCompared(t *testing.T) {
	// Whitespace differences would otherwise split one recurring error into
	// several records, each with a misleadingly low count.
	h := newAuditReaderHandler(t)
	h.server.logs.entries = append(h.server.logs.entries,
		LogEntry{"level": "error", "message": "  spaced  "},
		LogEntry{"level": "error", "message": "spaced"},
	)

	errors := newToolCaptureStateReader(h).GetConsoleErrors()

	if len(errors) != 1 {
		t.Fatalf("collected %d records, want 1: %+v", len(errors), errors)
	}
	if errors[0].Message != "spaced" {
		t.Errorf("Message = %q, want the trimmed form", errors[0].Message)
	}
	if errors[0].Count != 2 {
		t.Errorf("Count = %d, want 2", errors[0].Count)
	}
}

func TestAuditReader_BlankMessagesAreDropped(t *testing.T) {
	h := newAuditReaderHandler(t)
	h.server.logs.entries = append(h.server.logs.entries,
		LogEntry{"level": "error", "message": "   "},
		LogEntry{"level": "error"},
		LogEntry{"level": "error", "message": "real"},
	)

	errors := newToolCaptureStateReader(h).GetConsoleErrors()

	if len(errors) != 1 || errors[0].Message != "real" {
		t.Errorf("GetConsoleErrors = %+v, want only the real message", errors)
	}
}

func TestAuditReader_ConsoleOrderIsStableAcrossCalls(t *testing.T) {
	// The records are built from a map. Without the explicit sort the order would
	// vary per call, and two snapshots of identical state would not compare equal.
	h := newAuditReaderHandler(t)
	for _, msg := range []string{"zebra", "alpha", "middle"} {
		h.server.logs.entries = append(h.server.logs.entries, LogEntry{"level": "error", "message": msg})
	}
	r := newToolCaptureStateReader(h)

	first := r.GetConsoleErrors()
	for i := 0; i < 20; i++ {
		again := r.GetConsoleErrors()
		for j := range first {
			if first[j].Message != again[j].Message {
				t.Fatalf("order changed between calls: %v then %v", first, again)
			}
		}
	}
	if first[0].Message != "alpha" || first[2].Message != "zebra" {
		t.Errorf("records are not sorted by message: %+v", first)
	}
}

func TestAuditReader_NilHandlerStateYieldsEmptySlicesNotNil(t *testing.T) {
	// These feed straight into a JSON snapshot; nil becomes null and breaks
	// anything that ranges over the audit record.
	r := newToolCaptureStateReader(nil)

	if got := r.GetConsoleErrors(); got == nil || len(got) != 0 {
		t.Errorf("GetConsoleErrors = %v, want an empty slice", got)
	}
	if got := r.GetNetworkRequests(); got == nil || len(got) != 0 {
		t.Errorf("GetNetworkRequests = %v, want an empty slice", got)
	}
	if got := r.GetWSConnections(); got == nil || len(got) != 0 {
		t.Errorf("GetWSConnections = %v, want an empty slice", got)
	}
	if got := r.GetPerformance(); got != nil {
		t.Errorf("GetPerformance = %v, want nil", got)
	}
	if got := r.GetCurrentPageURL(); got != "" {
		t.Errorf("GetCurrentPageURL = %q, want empty", got)
	}
}

// =============================================================================
// NETWORK
// =============================================================================

func TestAuditReader_NetworkRequestsCarryTheFieldsAnAuditNeeds(t *testing.T) {
	h := newAuditReaderHandler(t)
	h.capture.AddNetworkBodies([]capture.NetworkBody{{
		Method:       "POST",
		URL:          "https://api.test/v1/thing",
		Status:       503,
		Duration:     1234,
		ContentType:  "application/json",
		ResponseBody: "0123456789",
	}})

	got := newToolCaptureStateReader(h).GetNetworkRequests()

	if len(got) != 1 {
		t.Fatalf("got %d requests, want 1", len(got))
	}
	req := got[0]
	if req.Method != "POST" || req.URL != "https://api.test/v1/thing" || req.Status != 503 {
		t.Errorf("request identity lost: %+v", req)
	}
	// The body itself is deliberately not recorded — only its size — so an audit
	// trail cannot become a copy of every response the browser ever saw.
	if req.ResponseSize != 10 {
		t.Errorf("ResponseSize = %d, want the body length 10", req.ResponseSize)
	}
	if req.Duration != 1234 || req.ContentType != "application/json" {
		t.Errorf("request metadata lost: %+v", req)
	}
}

// =============================================================================
// PERFORMANCE
// =============================================================================

func TestAuditReader_PerformancePicksTheNewestSnapshot(t *testing.T) {
	// Snapshots accumulate; an audit that recorded the first one would describe
	// the page as it was on load, not as it was when the audit ran.
	h := newAuditReaderHandler(t)
	h.capture.AddPerformanceSnapshots([]performance.Snapshot{
		{Timestamp: "2026-01-01T10:00:00Z", URL: "https://old.test"},
		{Timestamp: "2026-01-01T12:00:00Z", URL: "https://newest.test"},
		{Timestamp: "2026-01-01T11:00:00Z", URL: "https://middle.test"},
	})

	got := newToolCaptureStateReader(h).GetPerformance()

	if got == nil {
		t.Fatal("GetPerformance returned nil with snapshots present")
	}
	if got.URL != "https://newest.test" {
		t.Errorf("GetPerformance chose %q, want the newest snapshot", got.URL)
	}
}

func TestAuditReader_PerformanceAcceptsBothTimestampPrecisions(t *testing.T) {
	// Timestamps arrive as RFC3339 or RFC3339Nano depending on the source. An
	// unparsed timestamp sorts as the zero time and loses to everything.
	h := newAuditReaderHandler(t)
	h.capture.AddPerformanceSnapshots([]performance.Snapshot{
		{Timestamp: "2026-01-01T10:00:00.123456789Z", URL: "https://nano.test"},
		{Timestamp: "2026-01-01T09:00:00Z", URL: "https://second.test"},
	})

	got := newToolCaptureStateReader(h).GetPerformance()

	if got == nil || got.URL != "https://nano.test" {
		t.Errorf("GetPerformance = %+v, want the nanosecond-precision snapshot", got)
	}
}

func TestAuditReader_PerformanceIsNilWithNoSnapshots(t *testing.T) {
	h := newAuditReaderHandler(t)

	if got := newToolCaptureStateReader(h).GetPerformance(); got != nil {
		t.Errorf("GetPerformance = %+v, want nil so the snapshot omits the section", got)
	}
}

// =============================================================================
// PAGE URL
// =============================================================================

func TestAuditReader_PageURLPrefersTheTrackedTab(t *testing.T) {
	h := newAuditReaderHandler(t)
	h.capture.SetTrackingStatusForTest(1, "https://tracked.test/page")
	h.capture.AddNetworkBodies([]capture.NetworkBody{{URL: "https://api.test/xhr"}})

	if got := newToolCaptureStateReader(h).GetCurrentPageURL(); got != "https://tracked.test/page" {
		t.Errorf("GetCurrentPageURL = %q, want the tracked tab URL", got)
	}
}

func TestAuditReader_PageURLFallsBackToPerformanceThenNetwork(t *testing.T) {
	// The fallback order matters: a performance snapshot's URL is the page, while
	// a network body's URL is usually an XHR endpoint, which is a worse answer.
	h := newAuditReaderHandler(t)
	h.capture.AddPerformanceSnapshots([]performance.Snapshot{
		{Timestamp: "2026-01-01T10:00:00Z", URL: "https://perf.test/page"},
	})
	h.capture.AddNetworkBodies([]capture.NetworkBody{{URL: "https://api.test/xhr"}})

	if got := newToolCaptureStateReader(h).GetCurrentPageURL(); got != "https://perf.test/page" {
		t.Errorf("GetCurrentPageURL = %q, want the performance snapshot URL", got)
	}
}

func TestAuditReader_PageURLUsesTheMostRecentRequestAsALastResort(t *testing.T) {
	h := newAuditReaderHandler(t)
	h.capture.AddNetworkBodies([]capture.NetworkBody{
		{URL: "https://first.test"},
		{URL: "https://latest.test"},
	})

	if got := newToolCaptureStateReader(h).GetCurrentPageURL(); got != "https://latest.test" {
		t.Errorf("GetCurrentPageURL = %q, want the most recent request", got)
	}
}
