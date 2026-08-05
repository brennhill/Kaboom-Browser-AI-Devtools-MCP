// Purpose: Coverage-expansion tests for HAR export edge cases and branch paths.
// Docs: docs/features/feature/har-export/index.md

// export_coverage_test.go — Targeted coverage tests for uncovered export paths.
// Covers: ExportHARMergedToFile, httpStatusText branches, computeWaterfallTimings edge cases,
// matchesWaterfallFilter and matchesHARFilter.
package har

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// ============================================
// httpStatusText — Cover all status code branches
// ============================================

func TestHttpStatusText_AllCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code int
		want string
	}{
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{301, "Moved Permanently"},
		{302, "Found"},
		{304, "Not Modified"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
		{0, ""},
		{418, ""},
		{999, ""},
	}

	for _, tc := range cases {
		got := httpStatusText(tc.code)
		if got != tc.want {
			t.Errorf("httpStatusText(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

// ============================================
// computeWaterfallTimings — Edge cases
// ============================================

func TestComputeWaterfallTimings_ZeroFetchStart(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{
		Duration:    100.0,
		StartTime:   0,
		FetchStart:  0, // zero => early return
		ResponseEnd: 100.0,
	}
	send, wait, receive := computeWaterfallTimings(wf)
	if send != -1 {
		t.Errorf("send = %d, want -1", send)
	}
	if wait != 100 {
		t.Errorf("wait = %d, want 100", wait)
	}
	if receive != -1 {
		t.Errorf("receive = %d, want -1", receive)
	}
}

func TestComputeWaterfallTimings_ZeroResponseEnd(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{
		Duration:    50.0,
		StartTime:   10.0,
		FetchStart:  15.0,
		ResponseEnd: 0, // zero => early return
	}
	send, wait, receive := computeWaterfallTimings(wf)
	if send != -1 {
		t.Errorf("send = %d, want -1", send)
	}
	if wait != 50 {
		t.Errorf("wait = %d, want 50", wait)
	}
	if receive != -1 {
		t.Errorf("receive = %d, want -1", receive)
	}
}

func TestComputeWaterfallTimings_NegativeSendClampsToZero(t *testing.T) {
	t.Parallel()

	// FetchStart < StartTime => sendF would be negative, clamped to 0
	wf := types.NetworkWaterfallEntry{
		Duration:    100.0,
		StartTime:   20.0,
		FetchStart:  10.0, // less than StartTime
		ResponseEnd: 120.0,
	}
	send, wait, receive := computeWaterfallTimings(wf)
	if send != 0 {
		t.Errorf("send = %d, want 0 (clamped from negative)", send)
	}
	if wait < 0 {
		t.Errorf("wait = %d, want non-negative", wait)
	}
	if receive != 0 {
		t.Errorf("receive = %d, want 0", receive)
	}
}

func TestComputeWaterfallTimings_NegativeRemainClampsToZero(t *testing.T) {
	t.Parallel()

	// ResponseEnd < FetchStart => remainF would be negative, clamped to 0
	wf := types.NetworkWaterfallEntry{
		Duration:    100.0,
		StartTime:   10.0,
		FetchStart:  50.0,
		ResponseEnd: 30.0, // less than FetchStart
	}
	send, wait, receive := computeWaterfallTimings(wf)
	if send != 40 {
		t.Errorf("send = %d, want 40", send)
	}
	if wait != 0 {
		t.Errorf("wait = %d, want 0 (clamped from negative)", wait)
	}
	if receive != 0 {
		t.Errorf("receive = %d, want 0", receive)
	}
}

func TestComputeWaterfallTimings_ValidValues(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{
		Duration:    100.0,
		StartTime:   100.0,
		FetchStart:  110.0,
		ResponseEnd: 200.0,
	}
	send, wait, receive := computeWaterfallTimings(wf)
	if send != 10 {
		t.Errorf("send = %d, want 10", send)
	}
	if wait != 90 {
		t.Errorf("wait = %d, want 90", wait)
	}
	if receive != 0 {
		t.Errorf("receive = %d, want 0", receive)
	}
}

// ============================================
// matchesWaterfallFilter — Method filter
// ============================================

func TestMatchesWaterfallFilter_MethodFilter(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{URL: "https://example.com/api"}

	// Waterfall entries are implicitly GET
	if !matchesWaterfallFilter(wf, types.NetworkBodyFilter{Method: "GET"}) {
		t.Error("expected GET filter to match waterfall entry")
	}
	if !matchesWaterfallFilter(wf, types.NetworkBodyFilter{Method: "get"}) {
		t.Error("expected case-insensitive GET match")
	}
	if matchesWaterfallFilter(wf, types.NetworkBodyFilter{Method: "POST"}) {
		t.Error("expected POST filter to reject waterfall entry")
	}
	if matchesWaterfallFilter(wf, types.NetworkBodyFilter{Method: "PUT"}) {
		t.Error("expected PUT filter to reject waterfall entry")
	}
}

func TestMatchesWaterfallFilter_URLFilter(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{URL: "https://example.com/API/users"}

	if !matchesWaterfallFilter(wf, types.NetworkBodyFilter{URLFilter: "api"}) {
		t.Error("URL filter 'api' should match (case-insensitive)")
	}
	if matchesWaterfallFilter(wf, types.NetworkBodyFilter{URLFilter: "posts"}) {
		t.Error("URL filter 'posts' should not match")
	}
}

func TestMatchesWaterfallFilter_NoFilter(t *testing.T) {
	t.Parallel()

	wf := types.NetworkWaterfallEntry{URL: "https://example.com/anything"}
	if !matchesWaterfallFilter(wf, types.NetworkBodyFilter{}) {
		t.Error("empty filter should match everything")
	}
}

// ============================================
// matchesHARFilter — StatusMax branch
// ============================================

func TestMatchesHARFilter_StatusMax(t *testing.T) {
	t.Parallel()

	body := types.NetworkBody{Method: "GET", URL: "https://example.com", Status: 500}

	if matchesHARFilter(body, types.NetworkBodyFilter{StatusMax: 399}) {
		t.Error("status 500 should not pass StatusMax=399 filter")
	}
	if !matchesHARFilter(body, types.NetworkBodyFilter{StatusMax: 500}) {
		t.Error("status 500 should pass StatusMax=500 filter")
	}
	if !matchesHARFilter(body, types.NetworkBodyFilter{StatusMax: 599}) {
		t.Error("status 500 should pass StatusMax=599 filter")
	}
}

func TestMatchesHARFilter_StatusMinAndMax(t *testing.T) {
	t.Parallel()

	body := types.NetworkBody{Method: "GET", URL: "https://example.com", Status: 404}

	if !matchesHARFilter(body, types.NetworkBodyFilter{StatusMin: 400, StatusMax: 499}) {
		t.Error("status 404 should pass 400-499 range")
	}
	if matchesHARFilter(body, types.NetworkBodyFilter{StatusMin: 500, StatusMax: 599}) {
		t.Error("status 404 should not pass 500-599 range")
	}
}

// ============================================
// ExportHARMergedToFile — Full file export with merged data
// ============================================

func TestExportHARMergedToFile_Success(t *testing.T) {
	t.Parallel()

	bodies := []types.NetworkBody{
		{
			Timestamp:    "2026-01-23T10:30:00.000Z",
			Method:       "GET",
			URL:          "https://api.example.com/data",
			Status:       200,
			ResponseBody: `{"ok":true}`,
			ContentType:  "application/json",
			Duration:     142,
		},
	}
	waterfall := []types.NetworkWaterfallEntry{
		{
			URL:             "https://cdn.example.com/app.js",
			Duration:        80.0,
			StartTime:       100.0,
			FetchStart:      105.0,
			ResponseEnd:     180.0,
			DecodedBodySize: 8192,
			EncodedBodySize: 4096,
			Timestamp:       time.Now(),
		},
	}

	tmpFile := "/tmp/kaboom-test-har-merged-export.har"
	defer os.Remove(tmpFile)

	result, err := ExportHARMergedToFile(bodies, waterfall, types.NetworkBodyFilter{}, "6.0.3", tmpFile)
	if err != nil {
		t.Fatalf("ExportHARMergedToFile() error = %v", err)
	}

	if result.SavedTo != tmpFile {
		t.Errorf("saved_to = %q, want %q", result.SavedTo, tmpFile)
	}
	if result.EntriesCount != 2 {
		t.Errorf("entries_count = %d, want 2", result.EntriesCount)
	}
	if result.FileSizeBytes <= 0 {
		t.Errorf("file_size_bytes = %d, want positive", result.FileSizeBytes)
	}

	// Verify file content is valid HAR JSON
	data, err := os.ReadFile(tmpFile) // nosemgrep: go_filesystem_rule-fileread -- test reads output
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var harLog HARLog
	if err := json.Unmarshal(data, &harLog); err != nil {
		t.Fatalf("file is not valid HAR JSON: %v", err)
	}
	if harLog.Log.Version != "1.2" {
		t.Errorf("version = %q, want 1.2", harLog.Log.Version)
	}
	if len(harLog.Log.Entries) != 2 {
		t.Errorf("entries = %d, want 2", len(harLog.Log.Entries))
	}
}

func TestExportHARMergedToFile_PathTraversal(t *testing.T) {
	t.Parallel()

	_, err := ExportHARMergedToFile(nil, nil, types.NetworkBodyFilter{}, "test", "../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "unsafe path") {
		t.Errorf("expected 'unsafe path' error, got: %v", err)
	}
}

func TestExportHARMergedToFile_UnsafeAbsolutePath(t *testing.T) {
	t.Parallel()

	_, err := ExportHARMergedToFile(nil, nil, types.NetworkBodyFilter{}, "test", "/etc/hosts")
	if err == nil {
		t.Error("expected error for absolute path outside tmp")
	}
}

func TestExportHARMergedToFile_NonexistentDir(t *testing.T) {
	t.Parallel()

	_, err := ExportHARMergedToFile(
		[]types.NetworkBody{{Method: "GET", URL: "https://example.com", Status: 200}},
		nil, types.NetworkBodyFilter{}, "test",
		"/tmp/kaboom-nonexist-merged/deep/file.har",
	)
	if err == nil {
		t.Error("expected error for nonexistent parent dir")
	}
}

func TestExportHARMergedToFile_EmptyData(t *testing.T) {
	t.Parallel()

	tmpFile := "/tmp/kaboom-test-har-merged-empty.har"
	defer os.Remove(tmpFile)

	result, err := ExportHARMergedToFile(nil, nil, types.NetworkBodyFilter{}, "test", tmpFile)
	if err != nil {
		t.Fatalf("ExportHARMergedToFile() error = %v", err)
	}
	if result.EntriesCount != 0 {
		t.Errorf("entries_count = %d, want 0", result.EntriesCount)
	}
}

// ============================================
// isPathSafe — Additional branches
// ============================================

func TestIsPathSafe_PrivateTmpPrefix(t *testing.T) {
	t.Parallel()

	if !isPathSafe("/private/tmp/test.har") {
		t.Error("expected /private/tmp/ path to be safe")
	}
}

// ============================================
// HARExportResult JSON field names (snake_case)
// ============================================

func TestHARExportResult_JSONFields(t *testing.T) {
	t.Parallel()

	result := HARExportResult{
		SavedTo:       "/tmp/test.har",
		EntriesCount:  5,
		FileSizeBytes: 1234,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	s := string(data)

	// Our own result type must use snake_case
	if !strings.Contains(s, `"saved_to"`) {
		t.Error("expected snake_case field 'saved_to'")
	}
	if !strings.Contains(s, `"entries_count"`) {
		t.Error("expected snake_case field 'entries_count'")
	}
	if !strings.Contains(s, `"file_size_bytes"`) {
		t.Error("expected snake_case field 'file_size_bytes'")
	}
}
