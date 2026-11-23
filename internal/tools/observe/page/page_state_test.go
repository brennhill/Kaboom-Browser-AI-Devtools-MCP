// Purpose: Tests for the live-page observe modes (accessibility audit, screenshot data URLs).
// Docs: docs/features/feature/observe/index.md

package page

import (
	"encoding/json"
	"errors"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/analysis/app/a11yaudit"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/commandtest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/testsupport"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/browsercommand/query"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/composition/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/testsupport/fixture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/util"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/observe/core"
)

func completeNextPageStateQuery(t *testing.T, cap *capture.Capture, result json.RawMessage) {
	t.Helper()
	cap.Commands().AwaitPending(time.Second)
	pending := cap.Commands().Pending()
	if len(pending) == 0 {
		t.Error("page-state query was not enqueued")
		return
	}
	commandtest.DeliverForClient(cap.Commands(), pending[0].ID, result, "")
}

func TestGetStorageReportsCaptureFailuresAndQueuePressure(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}

	t.Run("extension error", func(t *testing.T) {
		cap := capture.NewCapture()
		defer cap.Close()
		capturefixture.Track(cap, 1, "https://example.test")
		go completeNextPageStateQuery(t, cap, json.RawMessage(`{"error":"storage denied"}`))
		resp := GetStorage(testsupport.Deps(cap), req, json.RawMessage(`{}`))
		if result := testsupport.DecodeToolResult(t, resp); !result.IsError || !strings.Contains(result.Content[0].Text, "storage denied") {
			t.Fatalf("extension failure response = %+v", result)
		}
	})

	t.Run("invalid result", func(t *testing.T) {
		cap := capture.NewCapture()
		defer cap.Close()
		capturefixture.Track(cap, 1, "https://example.test")
		go completeNextPageStateQuery(t, cap, json.RawMessage(`not-json`))
		resp := GetStorage(testsupport.Deps(cap), req, json.RawMessage(`{}`))
		if result := testsupport.DecodeToolResult(t, resp); !result.IsError || !strings.Contains(result.Content[0].Text, "invalid_json") {
			t.Fatalf("invalid-result response = %+v", result)
		}
	})

	t.Run("queue full", func(t *testing.T) {
		cap := capture.NewCapture()
		defer cap.Close()
		capturefixture.Track(cap, 1, "https://example.test")
		for i := 0; i < queries.MaxPendingQueries; i++ {
			if _, err := commandtest.Enqueue(cap.Commands(), queries.PendingQuery{Type: "occupied"}); err != nil {
				t.Fatalf("fill queue: %v", err)
			}
		}
		resp := GetStorage(testsupport.Deps(cap), req, json.RawMessage(`{}`))
		if result := testsupport.DecodeToolResult(t, resp); !result.IsError || !strings.Contains(result.Content[0].Text, "queue_full") {
			t.Fatalf("queue-full response = %+v", result)
		}
	})
}

func TestGetPageInfoReportsCSPRestrictionsOnlyWhenActive(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		restricted bool
		level      string
		wantCount  int
	}{
		{name: "clear", level: "none"},
		{name: "script execution", restricted: true, level: "script_exec", wantCount: 1},
		{name: "page blocked", restricted: true, level: "page_blocked", wantCount: 16},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			captured := capture.NewCapture()
			defer captured.Close()
			capturefixture.SetCSP(captured, testCase.restricted, testCase.level)
			data := testsupport.ExtractMCPJSON(t, GetPageInfo(testsupport.Deps(captured), mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 1}, nil))
			actions, present := data["blocked_actions"].([]any)
			if testCase.wantCount == 0 {
				if present || data["blocked_reason"] != nil {
					t.Fatalf("clear page info includes CSP guidance: %#v", data)
				}
				return
			}
			if !present || len(actions) != testCase.wantCount || data["blocked_reason"] == "" || data["csp_level"] != testCase.level {
				t.Fatalf("CSP page info = %#v", data)
			}
		})
	}
}

func TestGetStorageFiltersSuccessfulCapture(t *testing.T) {
	cap := capture.NewCapture()
	defer cap.Close()
	capturefixture.Track(cap, 1, "https://example.test")
	result := json.RawMessage(`{
		"url":"https://example.test",
		"localStorage":{"token":"secret","theme":"dark"},
		"sessionStorage":{"draft":"saved"},
		"cookies":[{"name":"session","value":"abc"},{"name":"theme","value":"dark"}]
	}`)
	go completeNextPageStateQuery(t, cap, result)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 2}
	resp := GetStorage(testsupport.Deps(cap), req, json.RawMessage(`{"storage_type":"local","key":"theme","summary":true}`))
	toolResult := testsupport.DecodeToolResult(t, resp)
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
	capturefixture.Track(cap, 1, "https://example.test")
	go completeNextPageStateQuery(t, cap, json.RawMessage(`{
		"url":"https://example.test",
		"localStorage":{"theme":"dark"},
		"sessionStorage":{"draft":"saved"},
		"cookies":[{"name":"session","value":"abc"}]
	}`))
	resp := GetStorage(testsupport.Deps(cap), mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 3}, json.RawMessage(`{}`))
	result := testsupport.DecodeToolResult(t, resp)
	if result.IsError || !strings.Contains(result.Content[0].Text, "local_storage") ||
		!strings.Contains(result.Content[0].Text, "session_storage") || !strings.Contains(result.Content[0].Text, "cookies") {
		t.Fatalf("raw storage response = %+v", result)
	}
}

func TestJSONPathFilteringCoversValidAndRejectedShapes(t *testing.T) {
	t.Parallel()
	body := types.NetworkBody{ResponseBody: `{"data":{"items":[{"id":7}],"spaced key":"ok"}}`}
	for _, path := range []string{"$.data.items[0].id", `$["data"]["spaced key"]`, "$"} {
		filtered, include, err := core.ApplyNetworkBodyFilter(body, path)
		if err != nil || !include || filtered.ResponseBody == "" {
			t.Fatalf("core.ApplyNetworkBodyFilter(%q) = %#v, %t, %v", path, filtered, include, err)
		}
	}
	for _, path := range []string{" ", "data.", "data[", "data[]", `data[""]`, "data[abc]", "data[-1]", "data..id"} {
		if _, _, err := core.ApplyNetworkBodyFilter(body, path); err == nil {
			t.Fatalf("core.ApplyNetworkBodyFilter(%q) accepted malformed path", path)
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
		if _, include, err := core.ApplyNetworkBodyFilter(testCase.body, testCase.path); err != nil || include {
			t.Fatalf("core.ApplyNetworkBodyFilter(%q) = include %t, err %v", testCase.path, include, err)
		}
	}
}

func TestGetScreenshotValidatesAndPersistsSuccessfulCapture(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 4}

	t.Run("validation", func(t *testing.T) {
		cap := capture.NewCapture()
		defer cap.Close()
		capturefixture.Track(cap, 1, "https://example.test")
		for _, args := range []json.RawMessage{json.RawMessage(`{"format":"gif"}`), json.RawMessage(`{"quality":101}`)} {
			if result := testsupport.DecodeToolResult(t, GetScreenshot(testsupport.Deps(cap), req, args)); !result.IsError {
				t.Fatalf("GetScreenshot(%s) accepted invalid options", args)
			}
		}
	})

	t.Run("save image", func(t *testing.T) {
		cap := capture.NewCapture()
		defer cap.Close()
		capturefixture.Track(cap, 1, "https://example.test")
		go completeNextPageStateQuery(t, cap, json.RawMessage(`{"data_url":"data:image/png;base64,aGVsbG8=","width":10}`))
		path := filepath.Join(t.TempDir(), "nested", "shot.png")
		args, _ := json.Marshal(map[string]any{
			"format": "png", "quality": 90, "full_page": true,
			"selector": "main", "wait_for_stable": true, "save_to": path,
		})
		result := testsupport.DecodeToolResult(t, GetScreenshot(testsupport.Deps(cap), req, args))
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

func TestLivePageAnalysisRejectsUntrackedCapture(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	t.Cleanup(cap.Close)
	deps := testsupport.Deps(cap)
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 40}
	for name, response := range map[string]mcp.JSONRPCResponse{
		"screenshot":    GetScreenshot(deps, req, nil),
		"accessibility": RunA11yAudit(deps, req, nil),
	} {
		result := testsupport.DecodeToolResult(t, response)
		if !result.IsError || !strings.Contains(strings.ToLower(result.Content[0].Text), "tab") {
			t.Fatalf("%s untracked result = %+v", name, result)
		}
	}
}

func TestGetScreenshotReturnsCanonicalJPEGAndTextOnlyShapes(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 41}
	for name, payload := range map[string]json.RawMessage{
		"jpeg image": json.RawMessage(`{"filename":"shot.jpg","data_url":"data:image/jpeg;base64,aGVsbG8="}`),
		"text only":  json.RawMessage(`{"filename":"shot.jpg","path":"/tmp/shot.jpg"}`),
	} {
		t.Run(name, func(t *testing.T) {
			cap := capture.NewCapture()
			defer cap.Close()
			capturefixture.Track(cap, 1, "https://example.test")
			go completeNextPageStateQuery(t, cap, payload)
			result := testsupport.DecodeToolResult(t, GetScreenshot(testsupport.Deps(cap), req, json.RawMessage(`{}`)))
			if result.IsError || len(result.Content) == 0 || result.Content[0].Type != "text" {
				t.Fatalf("screenshot result = %+v", result)
			}
			if name == "text only" && len(result.Content) != 1 {
				t.Fatalf("text-only content = %+v", result.Content)
			}
			if name == "jpeg image" && (len(result.Content) != 2 || result.Content[1].Type != "image" || result.Content[1].MimeType != "image/jpeg" || result.Content[1].Data != "aGVsbG8=") {
				t.Fatalf("JPEG content = %+v", result.Content)
			}
		})
	}
}

func TestGetIndexedDBValidatesTrackingAndReturnsRows(t *testing.T) {
	req := mcp.JSONRPCRequest{JSONRPC: mcp.JSONRPCVersion, ID: 5}
	cap := capture.NewCapture()
	defer cap.Close()
	for _, args := range []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`{"database":"app"}`)} {
		if result := testsupport.DecodeToolResult(t, GetIndexedDB(testsupport.Deps(cap), req, args)); !result.IsError {
			t.Fatalf("GetIndexedDB(%s) accepted missing parameters", args)
		}
	}
	if result := testsupport.DecodeToolResult(t, GetIndexedDB(testsupport.Deps(cap), req, json.RawMessage(`{"database":"app","store":"users"}`))); !result.IsError {
		t.Fatal("GetIndexedDB accepted an untracked tab")
	}
	capturefixture.Track(cap, 1, "https://example.test")
	go completeNextPageStateQuery(t, cap, json.RawMessage(`{"success":true,"result":{"entries":[{"id":1}],"count":9,"object_stores":["users"]}}`))
	result := testsupport.DecodeToolResult(t, GetIndexedDB(testsupport.Deps(cap), req, json.RawMessage(`{"database":"app","store":"users","limit":3}`)))
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
			capturefixture.Track(cap, 1, "https://example.test")
			go completeNextPageStateQuery(t, cap, payload)
			if result := testsupport.DecodeToolResult(t, GetScreenshot(testsupport.Deps(cap), req, json.RawMessage(`{}`))); !result.IsError {
				t.Fatalf("GetScreenshot accepted %s", name)
			}
		})
	}
	cap := capture.NewCapture()
	defer cap.Close()
	capturefixture.Track(cap, 1, "https://example.test")
	go completeNextPageStateQuery(t, cap, json.RawMessage(`{"data_url":"data:image/png;base64,aGVsbG8="}`))
	result := testsupport.DecodeToolResult(t, GetScreenshot(testsupport.Deps(cap), req, json.RawMessage(`{"save_to":"bad.txt"}`)))
	if result.IsError || !strings.Contains(result.Content[0].Text, "save_to_error") {
		t.Fatalf("save failure response = %+v", result)
	}
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
// Mock core.Deps for RunA11yAudit tests
// ============================================

type mockA11yDeps struct {
	cap           *capture.Capture
	a11yResult    json.RawMessage
	a11yErr       error
	diagnosticStr string
}

func (m *mockA11yDeps) deps() core.Deps {
	return core.Deps{
		Extension: m.cap.Extension(), Telemetry: m.cap.Telemetry(), Performance: m.cap.Performance(), ExtensionLogs: m.cap.ExtensionLogs(),
		LogEntries:           func() ([]types.LogEntry, []time.Time) { return nil, nil },
		LogTotalAdded:        func() int64 { return 0 },
		IsConsoleNoise:       func(types.LogEntry) bool { return false },
		DiagnosticHintString: func() string { return m.diagnosticStr },
		A11y: a11yaudit.New(a11yaudit.Deps{
			TrackingEnabled: func() bool {
				enabled, _, _ := m.cap.Extension().GetTrackingStatus()
				return enabled
			},
			Execute: func(string, []string, any, bool) (json.RawMessage, error) {
				return m.a11yResult, m.a11yErr
			},
		}),
	}
}

// ============================================
// Waterfall Summary Tests
// ============================================

func TestRunA11yAudit_TimeoutReturnsPartialResults(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	capturefixture.Track(cap, 1, "https://example.com")

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
	capturefixture.Track(cap, 1, "https://example.com")

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
// util.SplitDataURL Tests
// ============================================

func TestParseDataURL_ValidJPEG(t *testing.T) {
	t.Parallel()
	data, mime := util.SplitDataURL("data:image/jpeg;base64,/9j/4AAQSkZJRg==")
	if data != "/9j/4AAQSkZJRg==" {
		t.Errorf("base64Data = %q, want %q", data, "/9j/4AAQSkZJRg==")
	}
	if mime != "image/jpeg" {
		t.Errorf("mimeType = %q, want %q", mime, "image/jpeg")
	}
}

func TestParseDataURL_ValidPNG(t *testing.T) {
	t.Parallel()
	data, mime := util.SplitDataURL("data:image/png;base64,iVBORw0KGgo=")
	if data != "iVBORw0KGgo=" {
		t.Errorf("base64Data = %q, want %q", data, "iVBORw0KGgo=")
	}
	if mime != "image/png" {
		t.Errorf("mimeType = %q, want %q", mime, "image/png")
	}
}

func TestParseDataURL_MalformedNoDataPrefix(t *testing.T) {
	t.Parallel()
	data, mime := util.SplitDataURL("image/jpeg;base64,/9j/4AAQ")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for missing data: prefix, got data=%q mime=%q", data, mime)
	}
}

func TestParseDataURL_MalformedNoBase64Marker(t *testing.T) {
	t.Parallel()
	data, mime := util.SplitDataURL("data:image/jpeg;charset=utf-8,sometext")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for missing base64 marker, got data=%q mime=%q", data, mime)
	}
}

func TestParseDataURL_EmptyString(t *testing.T) {
	t.Parallel()
	data, mime := util.SplitDataURL("")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for empty input, got data=%q mime=%q", data, mime)
	}
}

func TestRunA11yAudit_ResultWithErrorFieldReturnsGracefully(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	capturefixture.Track(cap, 1, "https://example.com")

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
