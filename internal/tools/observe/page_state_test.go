// Purpose: Tests for the live-page observe modes (accessibility audit, screenshot data URLs).
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func completeNextPageStateQuery(t *testing.T, cap *capture.Capture, result json.RawMessage) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending := cap.Queries().GetPendingQueries()
		if len(pending) > 0 {
			cap.Queries().SetQueryResultWithClient(pending[0].ID, result, "")
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Error("page-state query was not enqueued")
}

func pageStateDeps(cap *capture.Capture) Deps {
	return Deps{
		Capture:              cap,
		DiagnosticHintString: func() string { return "doctor hint" },
	}
}

func TestGetStorageReportsCaptureFailuresAndQueuePressure(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}

	t.Run("extension error", func(t *testing.T) {
		cap := capture.NewCapture()
		defer cap.Close()
		cap.Extension().SetTrackingStatusForTest(1, "https://example.test")
		go completeNextPageStateQuery(t, cap, json.RawMessage(`{"error":"storage denied"}`))
		resp := GetStorage(pageStateDeps(cap), req, json.RawMessage(`{}`))
		if result := decodePageStateToolResult(t, resp); !result.IsError || !strings.Contains(result.Content[0].Text, "storage denied") {
			t.Fatalf("extension failure response = %+v", result)
		}
	})

	t.Run("invalid result", func(t *testing.T) {
		cap := capture.NewCapture()
		defer cap.Close()
		cap.Extension().SetTrackingStatusForTest(1, "https://example.test")
		go completeNextPageStateQuery(t, cap, json.RawMessage(`not-json`))
		resp := GetStorage(pageStateDeps(cap), req, json.RawMessage(`{}`))
		if result := decodePageStateToolResult(t, resp); !result.IsError || !strings.Contains(result.Content[0].Text, "invalid_json") {
			t.Fatalf("invalid-result response = %+v", result)
		}
	})

	t.Run("queue full", func(t *testing.T) {
		cap := capture.NewCapture()
		defer cap.Close()
		cap.Extension().SetTrackingStatusForTest(1, "https://example.test")
		for i := 0; i < queries.MaxPendingQueries; i++ {
			if _, err := cap.Queries().CreatePendingQuery(queries.PendingQuery{Type: "occupied"}); err != nil {
				t.Fatalf("fill queue: %v", err)
			}
		}
		resp := GetStorage(pageStateDeps(cap), req, json.RawMessage(`{}`))
		if result := decodePageStateToolResult(t, resp); !result.IsError || !strings.Contains(result.Content[0].Text, "queue_full") {
			t.Fatalf("queue-full response = %+v", result)
		}
	})
}

func TestGetStorageFiltersSuccessfulCapture(t *testing.T) {
	cap := capture.NewCapture()
	defer cap.Close()
	cap.Extension().SetTrackingStatusForTest(1, "https://example.test")
	result := json.RawMessage(`{
		"url":"https://example.test",
		"localStorage":{"token":"secret","theme":"dark"},
		"sessionStorage":{"draft":"saved"},
		"cookies":[{"name":"session","value":"abc"},{"name":"theme","value":"dark"}]
	}`)
	go completeNextPageStateQuery(t, cap, result)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 2}
	resp := GetStorage(pageStateDeps(cap), req, json.RawMessage(`{"storage_type":"local","key":"theme","summary":true}`))
	toolResult := decodePageStateToolResult(t, resp)
	if toolResult.IsError {
		t.Fatalf("storage capture failed: %s", toolResult.Content[0].Text)
	}
	if !strings.Contains(toolResult.Content[0].Text, "theme") || strings.Contains(toolResult.Content[0].Text, "secret") {
		t.Fatalf("filtered storage response = %s", toolResult.Content[0].Text)
	}
}

func TestGetStorageReturnsAllRawStorageFamilies(t *testing.T) {
	cap := capture.NewCapture()
	defer cap.Close()
	cap.Extension().SetTrackingStatusForTest(1, "https://example.test")
	go completeNextPageStateQuery(t, cap, json.RawMessage(`{
		"url":"https://example.test",
		"localStorage":{"theme":"dark"},
		"sessionStorage":{"draft":"saved"},
		"cookies":[{"name":"session","value":"abc"}]
	}`))
	resp := GetStorage(pageStateDeps(cap), mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 3}, json.RawMessage(`{}`))
	result := decodePageStateToolResult(t, resp)
	if result.IsError || !strings.Contains(result.Content[0].Text, "local_storage") ||
		!strings.Contains(result.Content[0].Text, "session_storage") || !strings.Contains(result.Content[0].Text, "cookies") {
		t.Fatalf("raw storage response = %+v", result)
	}
}

func TestToIntAcceptsWireAndServerNumericKinds(t *testing.T) {
	t.Parallel()
	for _, value := range []any{int(4), int32(5), int64(6), float64(7)} {
		if got, ok := toInt(value); !ok || got < 4 || got > 7 {
			t.Fatalf("toInt(%T(%v)) = %d, %t", value, value, got, ok)
		}
	}
	if _, ok := toInt("8"); ok {
		t.Fatal("toInt accepted a string")
	}
}

func TestJSONPathFilteringCoversValidAndRejectedShapes(t *testing.T) {
	t.Parallel()
	body := types.NetworkBody{ResponseBody: `{"data":{"items":[{"id":7}],"spaced key":"ok"}}`}
	for _, path := range []string{"$.data.items[0].id", `$["data"]["spaced key"]`, "$"} {
		filtered, include, err := ApplyNetworkBodyFilter(body, path)
		if err != nil || !include || filtered.ResponseBody == "" {
			t.Fatalf("ApplyNetworkBodyFilter(%q) = %#v, %t, %v", path, filtered, include, err)
		}
	}
	for _, path := range []string{" ", "data.", "data[", "data[]", `data[""]`, "data[abc]", "data[-1]", "data..id"} {
		if _, _, err := ApplyNetworkBodyFilter(body, path); err == nil {
			t.Fatalf("ApplyNetworkBodyFilter(%q) accepted malformed path", path)
		}
	}
	for _, testCase := range []struct {
		body types.NetworkBody
		path string
	}{
		{body: types.NetworkBody{}, path: "data"},
		{body: types.NetworkBody{ResponseBody: "not-json"}, path: "data"},
		{body: body, path: "data.missing"},
		{body: body, path: "data.items[9]"},
		{body: body, path: "data.items.id"},
	} {
		if _, include, err := ApplyNetworkBodyFilter(testCase.body, testCase.path); err != nil || include {
			t.Fatalf("ApplyNetworkBodyFilter(%q) = include %t, err %v", testCase.path, include, err)
		}
	}
}

func TestGetScreenshotValidatesAndPersistsSuccessfulCapture(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 4}

	t.Run("validation", func(t *testing.T) {
		cap := capture.NewCapture()
		defer cap.Close()
		cap.Extension().SetTrackingStatusForTest(1, "https://example.test")
		for _, args := range []json.RawMessage{json.RawMessage(`{"format":"gif"}`), json.RawMessage(`{"quality":101}`)} {
			if result := decodePageStateToolResult(t, GetScreenshot(pageStateDeps(cap), req, args)); !result.IsError {
				t.Fatalf("GetScreenshot(%s) accepted invalid options", args)
			}
		}
	})

	t.Run("save image", func(t *testing.T) {
		cap := capture.NewCapture()
		defer cap.Close()
		cap.Extension().SetTrackingStatusForTest(1, "https://example.test")
		go completeNextPageStateQuery(t, cap, json.RawMessage(`{"data_url":"data:image/png;base64,aGVsbG8=","width":10}`))
		path := filepath.Join(t.TempDir(), "nested", "shot.png")
		args, _ := json.Marshal(map[string]any{
			"format": "png", "quality": 90, "full_page": true,
			"selector": "main", "wait_for_stable": true, "save_to": path,
		})
		result := decodePageStateToolResult(t, GetScreenshot(pageStateDeps(cap), req, args))
		if result.IsError {
			t.Fatalf("GetScreenshot() = %+v", result)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "hello" {
			t.Fatalf("saved screenshot = %q, %v", data, err)
		}
		if len(result.Content) < 2 || result.Content[1].Type != "image" {
			t.Fatalf("inline image content = %+v", result.Content)
		}
	})
}

func TestGetIndexedDBValidatesTrackingAndReturnsRows(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 5}
	cap := capture.NewCapture()
	defer cap.Close()
	for _, args := range []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`{"database":"app"}`)} {
		if result := decodePageStateToolResult(t, GetIndexedDB(pageStateDeps(cap), req, args)); !result.IsError {
			t.Fatalf("GetIndexedDB(%s) accepted missing parameters", args)
		}
	}
	if result := decodePageStateToolResult(t, GetIndexedDB(pageStateDeps(cap), req, json.RawMessage(`{"database":"app","store":"users"}`))); !result.IsError {
		t.Fatal("GetIndexedDB accepted an untracked tab")
	}
	cap.Extension().SetTrackingStatusForTest(1, "https://example.test")
	go completeNextPageStateQuery(t, cap, json.RawMessage(`{"success":true,"result":{"entries":[{"id":1}],"count":9,"object_stores":["users"]}}`))
	result := decodePageStateToolResult(t, GetIndexedDB(pageStateDeps(cap), req, json.RawMessage(`{"database":"app","store":"users","limit":3}`)))
	if result.IsError || !strings.Contains(result.Content[0].Text, `"count":9`) || !strings.Contains(result.Content[0].Text, "object_stores") {
		t.Fatalf("GetIndexedDB result = %+v", result)
	}
}

func TestGetScreenshotReportsExtensionAndPersistenceErrors(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 6}
	for name, payload := range map[string]json.RawMessage{
		"invalid json":    json.RawMessage(`not-json`),
		"extension error": json.RawMessage(`{"error":"capture denied"}`),
	} {
		t.Run(name, func(t *testing.T) {
			cap := capture.NewCapture()
			defer cap.Close()
			cap.Extension().SetTrackingStatusForTest(1, "https://example.test")
			go completeNextPageStateQuery(t, cap, payload)
			if result := decodePageStateToolResult(t, GetScreenshot(pageStateDeps(cap), req, json.RawMessage(`{}`))); !result.IsError {
				t.Fatalf("GetScreenshot accepted %s", name)
			}
		})
	}
	cap := capture.NewCapture()
	defer cap.Close()
	cap.Extension().SetTrackingStatusForTest(1, "https://example.test")
	go completeNextPageStateQuery(t, cap, json.RawMessage(`{"data_url":"data:image/png;base64,aGVsbG8="}`))
	result := decodePageStateToolResult(t, GetScreenshot(pageStateDeps(cap), req, json.RawMessage(`{"save_to":"bad.txt"}`)))
	if result.IsError || !strings.Contains(result.Content[0].Text, "save_to_error") {
		t.Fatalf("save failure response = %+v", result)
	}
}

func decodePageStateToolResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode tool result: %v", err)
	}
	return result
}

func TestPageStateUsesCanonicalMCPResponses(t *testing.T) {
	t.Parallel()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "page_state.go"))
	if err != nil {
		t.Fatalf("read page_state.go: %v", err)
	}
	for _, forbidden := range []string{
		"mcp.StructuredErrorResponse(",
		`mcp.JSONRPCResponse{JSONRPC: "2.0"`,
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("page_state.go retains parallel MCP response construction %q", forbidden)
		}
	}
}

// ============================================
// Mock Deps for RunA11yAudit tests
// ============================================

type mockA11yDeps struct {
	cap           *capture.Capture
	a11yResult    json.RawMessage
	a11yErr       error
	diagnosticStr string
}

func (m *mockA11yDeps) deps() Deps {
	return Deps{
		Capture:              m.cap,
		LogEntries:           func() ([]types.LogEntry, []time.Time) { return nil, nil },
		LogTotalAdded:        func() int64 { return 0 },
		IsConsoleNoise:       func(types.LogEntry) bool { return false },
		DiagnosticHintString: func() string { return m.diagnosticStr },
		ExecuteA11yQuery: func(string, []string, any, bool) (json.RawMessage, error) {
			return m.a11yResult, m.a11yErr
		},
	}
}

// ============================================
// Waterfall Summary Tests
// ============================================

func TestBuildA11ySummary_Compact(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"passes": []any{
			map[string]any{"id": "rule1"},
			map[string]any{"id": "rule2"},
		},
		"violations": []any{
			map[string]any{
				"id":     "color-contrast",
				"impact": "serious",
				"nodes":  []any{map[string]any{}, map[string]any{}, map[string]any{}},
			},
			map[string]any{
				"id":     "image-alt",
				"impact": "critical",
				"nodes":  []any{map[string]any{}},
			},
		},
		"incomplete": []any{
			map[string]any{"id": "aria-label"},
		},
	}

	result := buildA11ySummary(auditResult)

	if result["pass"] != 2 {
		t.Errorf("pass = %v, want 2", result["pass"])
	}
	if result["violations"] != 2 {
		t.Errorf("violations = %v, want 2", result["violations"])
	}
	if result["incomplete"] != 1 {
		t.Errorf("incomplete = %v, want 1", result["incomplete"])
	}

	topIssues, ok := result["top_issues"].([]map[string]any)
	if !ok {
		t.Fatalf("top_issues wrong type: %T", result["top_issues"])
	}
	if len(topIssues) != 2 {
		t.Fatalf("expected 2 top issues, got %d", len(topIssues))
	}
	// Should be sorted by node count descending
	if topIssues[0]["rule"] != "color-contrast" {
		t.Errorf("first issue = %v, want color-contrast", topIssues[0]["rule"])
	}
	if topIssues[0]["count"] != 3 {
		t.Errorf("first issue count = %v, want 3", topIssues[0]["count"])
	}
	if topIssues[0]["severity"] != "serious" {
		t.Errorf("first issue severity = %v, want serious", topIssues[0]["severity"])
	}
}

func TestBuildA11ySummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildA11ySummary(map[string]any{})
	if result["pass"] != 0 {
		t.Errorf("pass = %v, want 0", result["pass"])
	}
	if result["violations"] != 0 {
		t.Errorf("violations = %v, want 0", result["violations"])
	}
}

func TestBuildA11ySummary_TopIssuesLimitedTo5(t *testing.T) {
	t.Parallel()
	violations := make([]any, 7)
	for i := range violations {
		violations[i] = map[string]any{
			"id":     "rule-" + string(rune('a'+i)),
			"impact": "minor",
			"nodes":  []any{map[string]any{}},
		}
	}
	auditResult := map[string]any{"violations": violations}

	result := buildA11ySummary(auditResult)
	topIssues := result["top_issues"].([]map[string]any)
	if len(topIssues) != 5 {
		t.Errorf("expected 5 top issues (capped), got %d", len(topIssues))
	}
}

// ============================================
// Issue #276: A11y Audit Partial Results Tests
// ============================================

func TestRunA11yAudit_TimeoutReturnsPartialResults(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.Extension().SetTrackingStatusForTest(1, "https://example.com")

	deps := &mockA11yDeps{
		cap:     cap,
		a11yErr: errors.New("context deadline exceeded"),
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := RunA11yAudit(deps.deps(), req, json.RawMessage(`{}`))

	// Should NOT be an error response — should return partial results gracefully
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result.IsError {
		t.Fatal("timeout should return partial results, not isError:true")
	}

	// Should contain an error field in the data
	text := result.Content[0].Text
	if !strings.Contains(text, "error") {
		t.Errorf("partial result should contain 'error' field, got: %s", text)
	}
	if !strings.Contains(text, "timeout") && !strings.Contains(text, "deadline") {
		t.Errorf("partial result should mention timeout or deadline, got: %s", text)
	}

	// Should have empty violations/passes arrays and partial flag (partial result structure)
	var data map[string]any
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("partial result text should contain JSON object")
	}
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("failed to parse partial result JSON: %v", err)
	}
	if _, ok := data["violations"]; !ok {
		t.Error("partial result should include 'violations' field")
	}
	if _, ok := data["summary"]; !ok {
		t.Error("partial result should include 'summary' field")
	}
	summary, ok := data["summary"].(map[string]any)
	if !ok {
		t.Fatalf("partial result summary should be object, got %T", data["summary"])
	}
	if summary["violations"] != float64(0) {
		t.Errorf("expected zero violations summary, got %+v", summary)
	}
	if summary["passes"] != float64(0) {
		t.Errorf("expected zero passes summary, got %+v", summary)
	}
	if data["partial"] != true {
		t.Errorf("partial result should have partial=true, got: %v", data["partial"])
	}
}

func TestRunA11yAudit_AlreadyRunningReturnsPartialResults(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.Extension().SetTrackingStatusForTest(1, "https://example.com")

	deps := &mockA11yDeps{
		cap:     cap,
		a11yErr: errors.New("Axe is already running"),
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := RunA11yAudit(deps.deps(), req, json.RawMessage(`{}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result.IsError {
		t.Fatal("already-running should return partial results, not isError:true")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "error") {
		t.Errorf("partial result should contain error field, got: %s", text)
	}
	if !strings.Contains(text, "already running") {
		t.Errorf("partial result should mention 'already running', got: %s", text)
	}

	// Verify partial flag is set
	var data map[string]any
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("partial result text should contain JSON object")
	}
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("failed to parse partial result JSON: %v", err)
	}
	if data["partial"] != true {
		t.Errorf("partial result should have partial=true, got: %v", data["partial"])
	}
}

// ============================================
// parseDataURL Tests
// ============================================

func TestParseDataURL_ValidJPEG(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("data:image/jpeg;base64,/9j/4AAQSkZJRg==")
	if data != "/9j/4AAQSkZJRg==" {
		t.Errorf("base64Data = %q, want %q", data, "/9j/4AAQSkZJRg==")
	}
	if mime != "image/jpeg" {
		t.Errorf("mimeType = %q, want %q", mime, "image/jpeg")
	}
}

func TestParseDataURL_ValidPNG(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("data:image/png;base64,iVBORw0KGgo=")
	if data != "iVBORw0KGgo=" {
		t.Errorf("base64Data = %q, want %q", data, "iVBORw0KGgo=")
	}
	if mime != "image/png" {
		t.Errorf("mimeType = %q, want %q", mime, "image/png")
	}
}

func TestParseDataURL_MalformedNoDataPrefix(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("image/jpeg;base64,/9j/4AAQ")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for missing data: prefix, got data=%q mime=%q", data, mime)
	}
}

func TestParseDataURL_MalformedNoBase64Marker(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("data:image/jpeg;charset=utf-8,sometext")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for missing base64 marker, got data=%q mime=%q", data, mime)
	}
}

func TestParseDataURL_EmptyString(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for empty input, got data=%q mime=%q", data, mime)
	}
}

func TestRunA11yAudit_ResultWithErrorFieldReturnsGracefully(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.Extension().SetTrackingStatusForTest(1, "https://example.com")

	// Simulate extension returning partial results with an error field
	partialResult := map[string]any{
		"violations":   []any{},
		"passes":       []any{},
		"incomplete":   []any{},
		"inapplicable": []any{},
		"summary": map[string]any{
			"violations":   0,
			"passes":       0,
			"incomplete":   0,
			"inapplicable": 0,
		},
		"error": "Accessibility audit timeout",
	}
	resultJSON, _ := json.Marshal(partialResult)

	deps := &mockA11yDeps{
		cap:        cap,
		a11yResult: resultJSON,
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := RunA11yAudit(deps.deps(), req, json.RawMessage(`{}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	// Should NOT be isError — partial results are still useful
	if result.IsError {
		t.Fatal("result with error field should not be isError:true")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "error") {
		t.Errorf("response should preserve error field, got: %s", text)
	}
}
