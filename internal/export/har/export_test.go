// Purpose: Tests for HAR export formatting and spec compliance.
// Docs: docs/features/feature/har-export/index.md

// export_har_test.go — HAR export unit tests.
// Tests conversion logic, query string parsing, path safety, and filtering.
package har

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func TestPackageFileBoundary(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read HAR package: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
			count++
		}
	}
	if count > 10 {
		t.Fatalf("HAR package has %d Go files; maximum is 10", count)
	}
}

// ============================================
// TestNetworkBodyToHAREntry - Conversion tests
// ============================================

func TestNetworkBodyToHAREntry(t *testing.T) {
	t.Parallel()
	t.Run("basic GET", func(t *testing.T) {
		body := types.NetworkBody{
			Timestamp: "2026-01-23T10:30:00.000Z",
			Method:    "GET",
			URL:       "https://example.com/api/users",
			Status:    200,
			Duration:  142,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Request.Method != "GET" {
			t.Errorf("expected method GET, got %s", entry.Request.Method)
		}
		if entry.Request.URL != "https://example.com/api/users" {
			t.Errorf("expected URL https://example.com/api/users, got %s", entry.Request.URL)
		}
		if entry.Response.Status != 200 {
			t.Errorf("expected status 200, got %d", entry.Response.Status)
		}
		if entry.Response.StatusText != "OK" {
			t.Errorf("expected statusText OK, got %s", entry.Response.StatusText)
		}
		if entry.Time != 142 {
			t.Errorf("expected time 142, got %d", entry.Time)
		}
		if entry.StartedDateTime != "2026-01-23T10:30:00.000Z" {
			t.Errorf("expected startedDateTime 2026-01-23T10:30:00.000Z, got %s", entry.StartedDateTime)
		}
		if entry.Request.HTTPVersion != "HTTP/1.1" {
			t.Errorf("expected httpVersion HTTP/1.1, got %s", entry.Request.HTTPVersion)
		}
		if entry.Response.HTTPVersion != "HTTP/1.1" {
			t.Errorf("expected response httpVersion HTTP/1.1, got %s", entry.Response.HTTPVersion)
		}
		if entry.Request.HeadersSize != -1 {
			t.Errorf("expected headersSize -1, got %d", entry.Request.HeadersSize)
		}
		if entry.Response.HeadersSize != -1 {
			t.Errorf("expected response headersSize -1, got %d", entry.Response.HeadersSize)
		}
	})

	t.Run("POST with body", func(t *testing.T) {
		body := types.NetworkBody{
			Timestamp:   "2026-01-23T10:30:00.000Z",
			Method:      "POST",
			URL:         "https://example.com/api/users",
			Status:      201,
			Duration:    100,
			RequestBody: `{"name": "Alice"}`,
			ContentType: "application/json",
		}

		entry := networkBodyToHAREntry(body)

		if entry.Request.PostData == nil {
			t.Fatal("expected postData to be present")
		}
		if entry.Request.PostData.MimeType != "application/json" {
			t.Errorf("expected mimeType application/json, got %s", entry.Request.PostData.MimeType)
		}
		if entry.Request.PostData.Text != `{"name": "Alice"}` {
			t.Errorf("expected text to match request body, got %s", entry.Request.PostData.Text)
		}
		if entry.Request.BodySize != len(`{"name": "Alice"}`) {
			t.Errorf("expected bodySize %d, got %d", len(`{"name": "Alice"}`), entry.Request.BodySize)
		}
	})

	t.Run("no request body", func(t *testing.T) {
		body := types.NetworkBody{
			Method: "GET",
			URL:    "https://example.com/api/users",
			Status: 200,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Request.PostData != nil {
			t.Error("expected postData to be nil for GET with no body")
		}
	})

	t.Run("truncated request", func(t *testing.T) {
		body := types.NetworkBody{
			Method:           "POST",
			URL:              "https://example.com/api",
			Status:           200,
			RequestBody:      "truncated...",
			ContentType:      "text/plain",
			RequestTruncated: true,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Request.Comment != "Body truncated at 8KB by Kaboom Agentic Browser" {
			t.Errorf("expected truncation comment, got %q", entry.Request.Comment)
		}
	})

	t.Run("truncated response", func(t *testing.T) {
		body := types.NetworkBody{
			Method:            "GET",
			URL:               "https://example.com/api",
			Status:            200,
			ResponseBody:      "truncated...",
			ResponseTruncated: true,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Response.Comment != "Body truncated at 16KB by Kaboom Agentic Browser" {
			t.Errorf("expected truncation comment, got %q", entry.Response.Comment)
		}
	})

	t.Run("query string parsing", func(t *testing.T) {
		body := types.NetworkBody{
			Method: "GET",
			URL:    "https://x.com/api?foo=bar&baz=1",
			Status: 200,
		}

		entry := networkBodyToHAREntry(body)

		if len(entry.Request.QueryString) != 2 {
			t.Fatalf("expected 2 queryString entries, got %d", len(entry.Request.QueryString))
		}
		found := false
		for _, q := range entry.Request.QueryString {
			if q.Name == "foo" && q.Value == "bar" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected queryString entry foo=bar")
		}
	})

	t.Run("unknown status code", func(t *testing.T) {
		body := types.NetworkBody{
			Method: "GET",
			URL:    "https://example.com/api",
			Status: 999,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Response.StatusText != "" {
			t.Errorf("expected empty statusText for unknown code, got %q", entry.Response.StatusText)
		}
	})

	t.Run("duration maps to timings.wait", func(t *testing.T) {
		body := types.NetworkBody{
			Method:   "GET",
			URL:      "https://example.com/api",
			Status:   200,
			Duration: 250,
		}

		entry := networkBodyToHAREntry(body)

		if entry.Timings.Wait != 250 {
			t.Errorf("expected timings.wait 250, got %d", entry.Timings.Wait)
		}
		if entry.Timings.Send != -1 {
			t.Errorf("expected timings.send -1, got %d", entry.Timings.Send)
		}
		if entry.Timings.Receive != -1 {
			t.Errorf("expected timings.receive -1, got %d", entry.Timings.Receive)
		}
	})

	t.Run("headers arrays are empty not nil", func(t *testing.T) {
		body := types.NetworkBody{
			Method: "GET",
			URL:    "https://example.com/api",
			Status: 200,
		}

		entry := networkBodyToHAREntry(body)

		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		if strings.Contains(s, `"headers":null`) {
			t.Error("headers should be empty array, not null")
		}
		if strings.Contains(s, `"queryString":null`) {
			t.Error("queryString should be empty array, not null")
		}
	})

	t.Run("response body in content", func(t *testing.T) {
		body := types.NetworkBody{
			Method:       "GET",
			URL:          "https://example.com/api",
			Status:       200,
			ResponseBody: `{"id": 42}`,
			ContentType:  "application/json",
		}

		entry := networkBodyToHAREntry(body)

		if entry.Response.Content.Text != `{"id": 42}` {
			t.Errorf("expected response content text, got %s", entry.Response.Content.Text)
		}
		if entry.Response.Content.MimeType != "application/json" {
			t.Errorf("expected mimeType application/json, got %s", entry.Response.Content.MimeType)
		}
		if entry.Response.Content.Size != len(`{"id": 42}`) {
			t.Errorf("expected content size %d, got %d", len(`{"id": 42}`), entry.Response.Content.Size)
		}
		if entry.Response.BodySize != len(`{"id": 42}`) {
			t.Errorf("expected bodySize %d, got %d", len(`{"id": 42}`), entry.Response.BodySize)
		}
	})
}

// ============================================
// TestIsPathSafe - Path validation tests
// ============================================

func TestIsPathSafe(t *testing.T) {
	t.Parallel()
	// Use runtime temp dir to avoid sandbox TMPDIR mismatch
	runtimeTmpDir := os.TempDir()

	tests := []struct {
		name string
		path string
		safe bool
	}{
		{"tmp absolute path", "/tmp/test.har", true},
		{"os tempdir", runtimeTmpDir + "/test.har", true},
		{"relative simple", "output.har", true},
		{"relative subdir", "reports/output.har", true},
		{"traversal", "../../etc/passwd", false},
		{"absolute outside tmp", "/etc/hosts", false},
		{"absolute user dir", "/home/user/file.har", false},
		{"dot-dot in middle", "foo/../../../etc/passwd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathSafe(tt.path)
			if got != tt.safe {
				t.Errorf("isPathSafe(%q) = %v, want %v", tt.path, got, tt.safe)
			}
		})
	}
}

// ============================================
// TestParseQueryString - Query string parsing
// ============================================

func TestParseQueryString(t *testing.T) {
	t.Parallel()
	t.Run("basic params", func(t *testing.T) {
		result := parseQueryString("https://example.com/api?foo=bar&baz=1")
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result))
		}
	})

	t.Run("empty query", func(t *testing.T) {
		result := parseQueryString("https://example.com/api")
		if len(result) != 0 {
			t.Errorf("expected 0 entries, got %d", len(result))
		}
	})

	t.Run("encoded values", func(t *testing.T) {
		result := parseQueryString("https://example.com/api?key=hello+world&val=a%26b")
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result))
		}
		found := false
		for _, q := range result {
			if q.Name == "key" && q.Value == "hello world" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected decoded queryString entry key=hello world")
		}
	})

	t.Run("invalid URL returns empty", func(t *testing.T) {
		result := parseQueryString("://not-a-url")
		if len(result) != 0 {
			t.Errorf("expected 0 entries for invalid URL, got %d", len(result))
		}
	})
}

// ============================================
// TestExportHARMerged - Merged waterfall tests
// ============================================

func TestExportHARMerged_WaterfallOnly(t *testing.T) {
	t.Parallel()
	now := time.Now()
	waterfall := []types.NetworkWaterfallEntry{
		{
			URL:             "https://example.com/style.css",
			InitiatorType:   "link",
			Duration:        50.5,
			StartTime:       100.0,
			FetchStart:      105.0,
			ResponseEnd:     150.5,
			TransferSize:    1024,
			DecodedBodySize: 2048,
			EncodedBodySize: 1024,
			Timestamp:       now,
		},
		{
			URL:             "https://example.com/app.js",
			InitiatorType:   "script",
			Duration:        80.0,
			StartTime:       200.0,
			FetchStart:      205.0,
			ResponseEnd:     280.0,
			TransferSize:    4096,
			DecodedBodySize: 8192,
			EncodedBodySize: 4096,
			Timestamp:       now.Add(time.Second),
		},
	}

	harLog := ExportHARMerged(nil, waterfall, types.NetworkBodyFilter{}, "test")

	if len(harLog.Log.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(harLog.Log.Entries))
	}

	entry := harLog.Log.Entries[0]
	if entry.Request.Method != "GET" {
		t.Errorf("expected method GET, got %s", entry.Request.Method)
	}
	if entry.Request.URL != "https://example.com/style.css" {
		t.Errorf("expected URL style.css, got %s", entry.Request.URL)
	}
	if entry.Response.Status != 0 {
		t.Errorf("expected status 0, got %d", entry.Response.Status)
	}
	if entry.Response.Content.Size != 2048 {
		t.Errorf("expected content size 2048, got %d", entry.Response.Content.Size)
	}
	if entry.Response.BodySize != 1024 {
		t.Errorf("expected bodySize 1024, got %d", entry.Response.BodySize)
	}
	if entry.Time != 50 {
		t.Errorf("expected time 50, got %d", entry.Time)
	}
	if entry.Comment != "From resource timing (no body captured)" {
		t.Errorf("expected resource timing comment, got %q", entry.Comment)
	}
}

func TestExportHARMerged_BodyAndWaterfall(t *testing.T) {
	t.Parallel()
	bodies := []types.NetworkBody{
		{
			Timestamp:    "2026-01-23T10:30:00.000Z",
			Method:       "GET",
			URL:          "https://api.example.com/data",
			Status:       200,
			Duration:     142,
			ResponseBody: `{"ok":true}`,
			ContentType:  "application/json",
		},
	}
	waterfall := []types.NetworkWaterfallEntry{
		{
			URL:             "https://api.example.com/data",
			Duration:        142.0,
			StartTime:       100.0,
			FetchStart:      105.0,
			ResponseEnd:     242.0,
			DecodedBodySize: 11,
			EncodedBodySize: 11,
			Timestamp:       time.Now(),
		},
	}

	harLog := ExportHARMerged(bodies, waterfall, types.NetworkBodyFilter{}, "test")

	if len(harLog.Log.Entries) != 1 {
		t.Fatalf("expected 1 entry (deduped), got %d", len(harLog.Log.Entries))
	}

	entry := harLog.Log.Entries[0]
	// Should retain body data from NetworkBody
	if entry.Response.Content.Text != `{"ok":true}` {
		t.Errorf("expected response body preserved, got %q", entry.Response.Content.Text)
	}
	if entry.Response.Status != 200 {
		t.Errorf("expected status 200, got %d", entry.Response.Status)
	}
	// Timings should be enriched from waterfall
	if entry.Timings.Send == -1 && entry.Timings.Receive == -1 {
		t.Error("expected timings to be enriched from waterfall, but send and receive are still -1")
	}
}

func TestExportHARMerged_Dedup(t *testing.T) {
	t.Parallel()
	bodies := []types.NetworkBody{
		{
			Timestamp: "2026-01-23T10:30:00.000Z",
			Method:    "GET",
			URL:       "https://example.com/api",
			Status:    200,
			Duration:  100,
		},
	}
	waterfall := []types.NetworkWaterfallEntry{
		{
			URL:       "https://example.com/api",
			Duration:  100.0,
			Timestamp: time.Now(),
		},
	}

	harLog := ExportHARMerged(bodies, waterfall, types.NetworkBodyFilter{}, "test")

	if len(harLog.Log.Entries) != 1 {
		t.Fatalf("expected 1 entry (same URL deduped), got %d", len(harLog.Log.Entries))
	}
}

func TestExportHARMerged_Sorted(t *testing.T) {
	t.Parallel()
	earlyTime, _ := time.Parse(time.RFC3339, "2026-01-23T10:30:00.000Z")
	lateTime, _ := time.Parse(time.RFC3339, "2026-01-23T10:30:05.000Z")
	bodies := []types.NetworkBody{
		{
			Timestamp: "2026-01-23T10:30:05.000Z",
			Method:    "GET",
			URL:       "https://example.com/late",
			Status:    200,
		},
	}
	waterfall := []types.NetworkWaterfallEntry{
		{
			URL:       "https://example.com/early",
			Duration:  50.0,
			Timestamp: earlyTime,
		},
		{
			URL:       "https://example.com/middle",
			Duration:  30.0,
			Timestamp: lateTime.Add(-2 * time.Second),
		},
	}

	harLog := ExportHARMerged(bodies, waterfall, types.NetworkBodyFilter{}, "test")

	if len(harLog.Log.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(harLog.Log.Entries))
	}

	// Should be sorted chronologically: early, middle, late
	if harLog.Log.Entries[0].Request.URL != "https://example.com/early" {
		t.Errorf("expected first entry to be /early, got %s", harLog.Log.Entries[0].Request.URL)
	}
	if harLog.Log.Entries[1].Request.URL != "https://example.com/middle" {
		t.Errorf("expected second entry to be /middle, got %s", harLog.Log.Entries[1].Request.URL)
	}
	if harLog.Log.Entries[2].Request.URL != "https://example.com/late" {
		t.Errorf("expected third entry to be /late, got %s", harLog.Log.Entries[2].Request.URL)
	}
}

func TestExportHARMerged_FilterAppliesToBoth(t *testing.T) {
	t.Parallel()
	bodies := []types.NetworkBody{
		{Method: "GET", URL: "https://example.com/api/users", Status: 200},
	}
	waterfall := []types.NetworkWaterfallEntry{
		{URL: "https://example.com/api/posts", Duration: 50, Timestamp: time.Now()},
		{URL: "https://cdn.example.com/logo.png", Duration: 30, Timestamp: time.Now()},
	}

	harLog := ExportHARMerged(bodies, waterfall, types.NetworkBodyFilter{URLFilter: "api"}, "test")

	if len(harLog.Log.Entries) != 2 {
		t.Fatalf("expected 2 entries (filtered to 'api'), got %d", len(harLog.Log.Entries))
	}
	for _, entry := range harLog.Log.Entries {
		if !strings.Contains(entry.Request.URL, "api") {
			t.Errorf("entry %s should not pass 'api' URL filter", entry.Request.URL)
		}
	}
}

func TestBuildHARResponse_WithHeaders(t *testing.T) {
	t.Parallel()
	body := types.NetworkBody{
		Method:      "GET",
		URL:         "https://example.com/api",
		Status:      200,
		ContentType: "application/json",
		ResponseHeaders: map[string]string{
			"Content-Type":  "application/json",
			"Cache-Control": "max-age=3600",
			"X-Request-Id":  "abc-123",
		},
	}

	resp := buildHARResponse(body)

	if len(resp.Headers) != 3 {
		t.Fatalf("expected 3 response headers, got %d", len(resp.Headers))
	}

	headerMap := make(map[string]string)
	for _, h := range resp.Headers {
		headerMap[h.Name] = h.Value
	}
	if headerMap["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type header, got %v", headerMap)
	}
	if headerMap["Cache-Control"] != "max-age=3600" {
		t.Errorf("expected Cache-Control header, got %v", headerMap)
	}
}

func TestExportHARMerged_EmptyBoth(t *testing.T) {
	t.Parallel()
	harLog := ExportHARMerged(nil, nil, types.NetworkBodyFilter{}, "test")

	if len(harLog.Log.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(harLog.Log.Entries))
	}
	if harLog.Log.Version != "1.2" {
		t.Errorf("expected version 1.2, got %s", harLog.Log.Version)
	}
}
