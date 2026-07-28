// Purpose: Tests for tool test helper utilities.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_test_helpers_test.go — Shared test helpers for all tool tests.
// Consolidates duplicated factories, parsers, JSON extractors, and assertion helpers.
package main

import (
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/logstore"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/testgenhandler"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolcatalog"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolgenerate"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

func toolSchemasForTest() []mcp.MCPTool { return schema.AllTools() }

// ============================================
// Factory + Test Environment
// ============================================

// makeToolHandler creates a ToolHandler with a temp-dir-backed Server and fresh Capture.
// Replaces: makeObserveToolHandler, makeAnalyzeToolHandler, makeGenerateToolHandler,
// makeConfigureToolHandler, makeInteractToolHandler (all identical).
func makeToolHandler(t *testing.T) (*ToolHandler, *Server, *capture.Capture) {
	t.Helper()
	server, err := NewServer(t.TempDir()+"/test.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	cap.Extension().SetPilotEnabled(false) // keep legacy test default: explicitly disabled unless test opts in
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.tools.Executor.(*ToolHandler)
	return handler, server, cap
}

// toolTestEnv bundles a ToolHandler, Server, and Capture for test convenience.
// Replaces: observeTestEnv, analyzeTestEnv, generateTestEnv, configureTestEnv,
// interactTestEnv, bundleTestEnv, videoTestEnv (all same 3 fields).
type toolTestEnv struct {
	handler *ToolHandler
	server  *Server
	capture *capture.Capture
}

// newToolTestEnv creates a toolTestEnv with t.TempDir() and t.Cleanup.
// Fixes: hardcoded /tmp/ paths and missing t.Cleanup in legacy variants.
func newToolTestEnv(t *testing.T) *toolTestEnv {
	t.Helper()
	logFile := t.TempDir() + "/test.jsonl"
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	cap.Extension().SetPilotEnabled(false) // keep legacy test default: explicitly disabled unless test opts in
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.tools.Executor.(*ToolHandler)
	return &toolTestEnv{handler: handler, server: server, capture: cap}
}

// mockConnectedTrackedTab simulates an extension sync and a tracked active tab.
// Use this for tests that exercise interact flows requiring extension + tab state.
func mockConnectedTrackedTab(t *testing.T, cap *capture.Capture) {
	t.Helper()
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	cap.HandleSync(httptest.NewRecorder(), httpReq)
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")
}

// ============================================
// JSON Extraction
// ============================================

// extractJSONFromText scans for the first '{' or '[' and returns everything from that point.
// Canonical version — replaces extractJSONFromMCPText, extractJSON, extractJSONFromStructuredError.
func extractJSONFromText(text string) string {
	for i, ch := range text {
		if ch == '{' || ch == '[' {
			return text[i:]
		}
	}
	return text
}

// ============================================
// Response Parsing
// ============================================

// parseToolResult unmarshals an MCPToolResult from a JSONRPCResponse.
func parseToolResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("parseToolResult: %v; raw=%s", err, string(resp.Result))
	}
	return result
}

// extractResultJSON extracts the JSON body from the first content block of an MCP result.
func extractResultJSON(t *testing.T, result mcp.MCPToolResult) map[string]any {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("extractResultJSON: no content blocks")
	}
	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatalf("extractResultJSON: no JSON object found in text: %s", text)
	}
	jsonPart := text[idx:]
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonPart), &data); err != nil {
		t.Fatalf("extractResultJSON: failed to parse JSON: %v\nraw: %s", err, jsonPart)
	}
	return data
}

// extractStructuredErrorJSON parses the JSON from an MCP error response.
func extractStructuredErrorJSON(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var result mcp.MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("extractStructuredErrorJSON: failed to parse MCPToolResult: %v", err)
	}
	if !result.IsError {
		t.Fatal("extractStructuredErrorJSON: expected isError: true")
	}
	if len(result.Content) == 0 {
		t.Fatal("extractStructuredErrorJSON: no content blocks")
	}
	text := result.Content[0].Text
	jsonPart := extractJSONFromText(text)
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonPart), &data); err != nil {
		t.Fatalf("extractStructuredErrorJSON: failed to parse JSON: %v\ntext: %s", err, text)
	}
	return data
}

// firstText extracts the first text block from a result, or "".
func firstText(result mcp.MCPToolResult) string {
	if len(result.Content) > 0 {
		return result.Content[0].Text
	}
	return ""
}

// ============================================
// Tool Call Wrappers
// ============================================

// callToolRaw dispatches through HandleToolCall (goes through validation/audit).
func callToolRaw(h *ToolHandler, name string, argsJSON string) mcp.JSONRPCResponse {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp, _ := h.HandleToolCall(req, name, json.RawMessage(argsJSON))
	return resp
}

// callObserveRaw invokes the canonical observe dispatcher and returns the raw JSONRPCResponse.
func callObserveRaw(h *ToolHandler, what string) mcp.JSONRPCResponse {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	args := json.RawMessage(`{"what":"` + what + `"}`)
	return h.observeDispatcher.Handle(req, args)
}

// callAnalyzeRaw invokes the canonical analyze dispatcher with async normalization.
func callAnalyzeRaw(h *ToolHandler, argsJSON string) mcp.JSONRPCResponse {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	return h.analyzeDispatcher.Handle(req, normalizeAnalyzeArgsForAsync(argsJSON))
}

// callConfigureRaw invokes the canonical configure dispatcher directly.
func callConfigureRaw(h *ToolHandler, argsJSON string) mcp.JSONRPCResponse {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	return h.configureDispatcher.Handle(req, json.RawMessage(argsJSON))
}

// callGenerateRaw invokes toolGenerate directly.
func callGenerateRaw(h *ToolHandler, argsJSON string) mcp.JSONRPCResponse {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	return h.generateDispatcher.Handle(req, json.RawMessage(argsJSON))
}

// callInteractRaw invokes toolInteract with async normalization.
func callInteractRaw(h *ToolHandler, argsJSON string) mcp.JSONRPCResponse {
	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	return h.toolInteract(req, normalizeInteractArgsForAsync(argsJSON))
}

// ============================================
// Async Normalization Helpers
// ============================================

// normalizeAnalyzeArgsForAsync adds sync=false for async-capable analyze operations
// (dom, page_summary, link_health) unless sync/wait/background is already specified.
func normalizeAnalyzeArgsForAsync(argsJSON string) json.RawMessage {
	raw := json.RawMessage(argsJSON)

	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return raw
	}

	what, _ := params["what"].(string)
	switch what {
	case "dom", "page_summary", "link_health", "computed_styles", "forms", "form_validation", "navigation", "page_structure":
	default:
		return raw
	}

	if _, hasSync := params["sync"]; hasSync {
		return raw
	}
	if _, hasWait := params["wait"]; hasWait {
		return raw
	}
	if _, hasBackground := params["background"]; hasBackground {
		return raw
	}

	params["sync"] = false
	if normalized, err := json.Marshal(params); err == nil {
		return json.RawMessage(normalized)
	}
	return raw
}

// normalizeObserveArgsForAsync adds sync=false for async-capable observe operations
// (page_inventory) unless sync/wait/background is already specified.
func normalizeObserveArgsForAsync(argsJSON string) json.RawMessage {
	raw := json.RawMessage(argsJSON)

	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return raw
	}

	what, _ := params["what"].(string)
	switch what {
	case "page_inventory":
	default:
		return raw
	}

	if _, hasSync := params["sync"]; hasSync {
		return raw
	}
	if _, hasWait := params["wait"]; hasWait {
		return raw
	}
	if _, hasBackground := params["background"]; hasBackground {
		return raw
	}

	params["sync"] = false
	if normalized, err := json.Marshal(params); err == nil {
		return json.RawMessage(normalized)
	}
	return raw
}

// normalizeInteractArgsForAsync adds background=true for interact calls with an action
// unless background/sync/wait is already specified.
func normalizeInteractArgsForAsync(argsJSON string) json.RawMessage {
	raw := json.RawMessage(argsJSON)

	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return raw
	}
	_, hasWhat := params["what"]
	_, hasAction := params["action"]
	if !hasWhat && !hasAction {
		return raw
	}
	if _, hasBackground := params["background"]; hasBackground {
		return raw
	}
	if _, hasSync := params["sync"]; hasSync {
		return raw
	}
	if _, hasWait := params["wait"]; hasWait {
		return raw
	}

	params["background"] = true
	if normalized, err := json.Marshal(params); err == nil {
		return json.RawMessage(normalized)
	}
	return raw
}

// ============================================
// Assertion Helpers
// ============================================

// snakeCasePattern matches valid snake_case keys: lowercase alpha start, then lowercase alphanum/underscore.
var snakeCasePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// specExceptions lists camelCase fields required by MCP or JSON-RPC specs.
var specExceptions = map[string]bool{
	"jsonrpc":           true, // JSON-RPC 2.0 spec
	"isError":           true, // SPEC:MCP
	"protocolVersion":   true, // SPEC:MCP
	"serverInfo":        true, // SPEC:MCP
	"mimeType":          true, // SPEC:MCP
	"inputSchema":       true, // SPEC:MCP
	"resourceTemplates": true, // SPEC:MCP
}

// assertSnakeCaseFields recursively checks that all JSON field names use snake_case,
// with exceptions for known MCP protocol fields.
func assertSnakeCaseFields(t *testing.T, jsonStr string) {
	t.Helper()

	var raw any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		t.Fatalf("assertSnakeCaseFields: invalid JSON: %v", err)
	}
	checkSnakeCaseRecursive(t, raw, "")
}

func checkSnakeCaseRecursive(t *testing.T, v any, path string) {
	t.Helper()
	switch val := v.(type) {
	case map[string]any:
		for key, child := range val {
			fullPath := path + "." + key
			if !specExceptions[key] && !snakeCasePattern.MatchString(key) {
				t.Errorf("JSON field %q is NOT snake_case (path: %s)", key, fullPath)
			}
			checkSnakeCaseRecursive(t, child, fullPath)
		}
	case []any:
		for i, child := range val {
			checkSnakeCaseRecursive(t, child, path+"["+string(rune('0'+i))+"]")
		}
	}
}

// assertNonErrorResponse verifies a result has content and is not an error.
func assertNonErrorResponse(t *testing.T, label string, result mcp.MCPToolResult) {
	t.Helper()
	if result.IsError {
		t.Errorf("%s: unexpected error response: %s", label, firstText(result))
		return
	}
	if len(result.Content) == 0 {
		t.Errorf("%s: no content blocks", label)
		return
	}
	if result.Content[0].Text == "" {
		t.Errorf("%s: empty text content", label)
	}
}

// assertIsError verifies the response is an error containing the expected substring.
func assertIsError(t *testing.T, resp mcp.JSONRPCResponse, contains string) {
	t.Helper()
	if !act.IsErrorResponse(resp) {
		var result mcp.MCPToolResult
		if err := json.Unmarshal(resp.Result, &result); err == nil {
			for _, c := range result.Content {
				if strings.Contains(c.Text, contains) {
					return
				}
			}
		}
		t.Errorf("expected error response containing %q", contains)
		return
	}
	raw, _ := json.Marshal(resp)
	if !strings.Contains(string(raw), contains) {
		t.Errorf("error response doesn't contain %q: %s", contains, raw)
	}
}

// newTestToolHandler creates a minimal ToolHandler for unit tests.
// It sets up a real Capture instance and a Server with empty entries.
//
// This lived in testgen_generate_test.go until the testgen cluster moved to
// internal/testgenhandler. Four unrelated test files (configure capabilities,
// generate validation, interact workflows, observe scope) depend on it, so it
// belongs with the other shared fixtures, not with one feature's tests.
func newTestToolHandler() *ToolHandler {
	cap := capture.NewCapture()
	srv := &Server{
		logs: logstore.New(logstore.Config{AddWarning: func(string) {}}),
	}
	h := &ToolHandler{
		MCPHandler: &MCPHandler{server: srv},
		capture:    cap,
	}
	h.testGenHandler = testgenhandler.New(buildTestGenerationDeps(h))
	h.generateDispatcher = toolgenerate.NewDispatcher(buildGenerateDeps(h), h.testGenHandler)
	h.interactActionHandler = toolinteract.NewInteractActionHandler(buildInteractDeps(h))
	h.toolCatalog = toolcatalog.New(nil, schema.AllTools())
	h.configureLocalDeps = buildConfigureLocalDeps(h)
	h.tutorialDeps = buildTutorialDeps(h)
	return h
}
