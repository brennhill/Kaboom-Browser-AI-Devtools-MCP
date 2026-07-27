// Purpose: Tests for analyze tool handler dispatch.
// Docs: docs/features/feature/analyze-tool/index.md

// tools_analyze_handler_test.go — Comprehensive unit tests for analyze tool dispatch and response fields.
// Validates all response fields, snake_case JSON convention, and dispatch logic.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsAnalyzeDispatch_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{bad json`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("invalid JSON should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "invalid_json") {
		t.Errorf("error code should be 'invalid_json', got: %s", result.Content[0].Text)
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeDispatch_MissingWhat(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("missing 'what' should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "missing_param") {
		t.Errorf("error code should be 'missing_param', got: %s", result.Content[0].Text)
	}
	// Verify hint lists valid modes
	text := result.Content[0].Text
	for _, mode := range []string{"dom", "performance", "accessibility"} {
		if !strings.Contains(text, mode) {
			t.Errorf("hint should list valid mode %q", mode)
		}
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeDispatch_UnknownMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"nonexistent_mode"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown mode should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "unknown_mode") {
		t.Errorf("error code should be 'unknown_mode', got: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "nonexistent_mode") {
		t.Error("error should mention the invalid mode name")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeDispatch_UnknownModeAliasAddsCanonicalWhatWarning(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"mode":"nonexistent_mode"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("unknown mode alias should return isError:true")
	}
	foundCanonicalWarning := false
	for _, block := range result.Content {
		if strings.Contains(block.Text, "deprecated") {
			foundCanonicalWarning = true
			break
		}
	}
	if !foundCanonicalWarning {
		t.Fatalf("expected canonical what warning block on error path, got %d content blocks", len(result.Content))
	}
}

func TestToolsAnalyzeDispatch_EmptyArgs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.toolAnalyze(req, nil)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("nil args (no 'what') should return isError:true")
	}
}

func TestToolsAnalyzeDispatch_ModeAliasAddsCanonicalWhatWarning(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"mode":"dom","selector":"body","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("mode alias should be accepted, got: %s", result.Content[0].Text)
	}
	foundCanonicalWarning := false
	for _, block := range result.Content {
		if strings.Contains(block.Text, "deprecated") {
			foundCanonicalWarning = true
			break
		}
	}
	if !foundCanonicalWarning {
		t.Fatalf("expected canonical what warning block, got %d content blocks", len(result.Content))
	}
}

func TestToolsAnalyzeDispatch_ConflictingWhatAndMode(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","mode":"performance","selector":"body"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("conflicting what/mode should return isError:true")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "invalid_param") {
		t.Fatalf("expected invalid_param, got: %s", text)
	}
	if !strings.Contains(text, "Conflicting parameters") {
		t.Fatalf("expected conflict explanation, got: %s", text)
	}
}

// ============================================
// getValidAnalyzeModes Tests
// ============================================

func TestToolsAnalyze_GetValidAnalyzeModes(t *testing.T) {
	t.Parallel()

	modes := getValidAnalyzeModes()
	modeList := strings.Split(modes, ", ")
	for i := 1; i < len(modeList); i++ {
		if modeList[i-1] > modeList[i] {
			t.Errorf("modes not sorted: %q > %q", modeList[i-1], modeList[i])
		}
	}

	for _, required := range []string{"dom", "performance", "accessibility", "api_validation", "link_health", "page_summary"} {
		if !strings.Contains(modes, required) {
			t.Errorf("valid modes missing %q: %s", required, modes)
		}
	}
}

func TestToolsAnalyzeSchema_HasFrameParam(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	tools := h.ToolsList()
	var analyzeSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "analyze" {
			analyzeSchema = tool.InputSchema
			break
		}
	}
	if analyzeSchema == nil {
		t.Fatal("analyze tool not found in ToolsList()")
	}

	props, ok := analyzeSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("analyze schema missing properties")
	}
	frameParam, exists := props["frame"]
	if !exists {
		t.Fatal("analyze schema missing 'frame' property")
	}
	frameMap, ok := frameParam.(map[string]any)
	if !ok {
		t.Fatal("frame property is not an object")
	}
	if frameMap["type"] != "string" {
		t.Fatal("frame property should be type string")
	}
}

func TestToolsAnalyzePageSummary_QueuedAsync(t *testing.T) {
	t.Parallel()
	// page_summary now requires pilot, extension, and tab tracking (issue #257)
	env := newContentTestEnv(t)

	resp := callAnalyzeRaw(env.handler, `{"what":"page_summary","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page_summary with sync=false should queue, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "page_summary_") {
		t.Errorf("correlation_id should start with 'page_summary_', got: %s", corr)
	}
}

// TestToolsAnalyzePageSummary_InvalidWorld removed: world parameter was removed in #257.
// Content extractors always run in ISOLATED world — accepting world param was misleading.

// ============================================
// analyze(what:"dom") — Response Fields
// ============================================

func TestToolsAnalyzeDOM_NoSelector_FullDOMDump(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	// Issue #274: dom without selector should succeed (full DOM dump with selector="*")
	resp := callAnalyzeRaw(h, `{"what":"dom","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom without selector should succeed for full DOM dump, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "dom_") {
		t.Errorf("correlation_id should start with 'dom_', got: %s", corr)
	}

	// Verify the pending query uses "*" as default selector
	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}
	if got, ok := params["selector"].(string); !ok || got != "*" {
		t.Fatalf("selector should default to '*' for full DOM dump, got %#v", params["selector"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeDOM_Success(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"status", "correlation_id", "queued", "final"} {
		if _, ok := data[field]; !ok {
			t.Errorf("dom response missing field %q", field)
		}
	}
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "dom_") {
		t.Errorf("correlation_id should start with 'dom_', got: %s", corr)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeDOM_FrameSelectorForwardedInPendingQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main","frame":"iframe.editor","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom with frame selector should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["frame"].(string); !ok || got != "iframe.editor" {
		t.Fatalf("frame selector not forwarded correctly, got %#v", params["frame"])
	}
}

func TestToolsAnalyzeDOM_FrameIndexForwardedInPendingQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":"#main","frame":0,"sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom with frame index should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}

	if got, ok := params["frame"].(float64); !ok || got != 0 {
		t.Fatalf("frame index not forwarded correctly, got %#v", params["frame"])
	}
}

// ============================================
// analyze(what:"api_validation") — Response Fields
// ============================================

func TestToolsAnalyzeAPIValidation_Analyze(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"api_validation","operation":"analyze"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("api_validation analyze should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if data["operation"] != "analyze" {
		t.Errorf("operation = %v, want 'analyze'", data["operation"])
	}
	if _, ok := data["violations"]; !ok {
		t.Error("response missing 'violations' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeAPIValidation_Report(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"api_validation","operation":"report"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("api_validation report should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}
	if data["operation"] != "report" {
		t.Errorf("operation = %v, want 'report'", data["operation"])
	}
	if _, ok := data["endpoints"]; !ok {
		t.Error("response missing 'endpoints' field")
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeAPIValidation_Clear(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"api_validation","operation":"clear"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("api_validation clear should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["action"] != "cleared" {
		t.Errorf("action = %v, want 'cleared'", data["action"])
	}
	if data["status"] != "ok" {
		t.Errorf("status = %v, want 'ok'", data["status"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeAPIValidation_InvalidOperation(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"api_validation","operation":"invalid_op"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("api_validation with invalid operation should return isError:true")
	}
	if !strings.Contains(result.Content[0].Text, "operation") {
		t.Error("error should mention 'operation' parameter")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeAPIValidation_DefaultOperation(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// No operation param defaults to empty string, which is invalid
	resp := callAnalyzeRaw(h, `{"what":"api_validation"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("api_validation without operation should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// analyze(what:"performance") — Response Fields
// ============================================

func TestToolsAnalyzePerformance_Empty(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"performance"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("performance should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	for _, field := range []string{"snapshots", "count"} {
		if _, ok := data[field]; !ok {
			t.Errorf("performance response missing field %q", field)
		}
	}
	count, _ := data["count"].(float64)
	if count != 0 {
		t.Errorf("count = %v, want 0 (empty)", count)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// analyze(what:"link_health") — Response Fields
// ============================================

func TestToolsAnalyzeLinkHealth_ResponseFields(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"link_health"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("link_health should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "link_health_") {
		t.Errorf("correlation_id should start with 'link_health_', got: %s", corr)
	}
	if queued, ok := data["queued"].(bool); !ok || !queued {
		t.Errorf("queued = %v, want true", data["queued"])
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// analyze(what:"link_validation") — Response Fields
// ============================================

func TestToolsAnalyzeLinkValidation_MissingURLs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"link_validation"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("link_validation without urls should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeLinkValidation_EmptyURLs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"link_validation","urls":[]}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("link_validation with empty urls should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsAnalyzeLinkValidation_NonHTTPURLs(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"link_validation","urls":["ftp://files.example.com","mailto:test@test.com"]}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("link_validation with non-HTTP urls should return isError:true")
	}
	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// Helper function tests — moved to internal/tools/analyze/link_validation_test.go
// (ClampInt, FilterHTTPURLs, ClassifyHTTPStatus)
// ============================================

// ============================================
// All analyze modes safety net
// ============================================

// ============================================
// Smoke Tests: Stream 5 — selector forwarding, default selector, a11y alias
// ============================================

func TestSmoke_AnalyzeDOM_SelectorParam_ForwardedInPendingQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	resp := callAnalyzeRaw(h, `{"what":"dom","selector":".sidebar","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom with selector should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}
	if got, ok := params["selector"].(string); !ok || got != ".sidebar" {
		t.Fatalf("selector should be '.sidebar', got %#v", params["selector"])
	}
	// Confirm it is not the default "*"
	if params["selector"] == "*" {
		t.Fatal("selector should be '.sidebar', not the default '*'")
	}
}

func TestSmoke_AnalyzeDOM_DefaultSelector_IsStar(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	// No selector provided — should default to "*"
	resp := callAnalyzeRaw(h, `{"what":"dom","sync":false}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("dom without selector should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("failed to parse pending query params: %v", err)
	}
	if got, ok := params["selector"].(string); !ok || got != "*" {
		t.Fatalf("selector should default to '*' for full DOM dump, got %#v", params["selector"])
	}
}

func TestSmoke_AnalyzeA11y_Alias_Resolves(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// "a11y" is an alias for "accessibility" — should NOT return unknown_mode
	resp := callAnalyzeRaw(h, `{"what":"a11y","sync":false}`)
	result := parseToolResult(t, resp)

	// The a11y handler may produce an error for other reasons (e.g., extension
	// not connected), but it must NOT be "unknown_mode". If unknown_mode is
	// returned, the alias resolution is broken.
	if result.IsError {
		text := result.Content[0].Text
		if strings.Contains(text, "unknown_mode") {
			t.Fatalf("a11y alias should resolve to accessibility, but got unknown_mode: %s", text)
		}
		// Other errors (no data, extension disconnected) are acceptable —
		// they prove the alias resolved correctly and reached the handler.
	}
}

// ============================================
// All analyze modes safety net
// ============================================

func TestToolsAnalyze_AllModes_ResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// All modes from analyzeHandlers that can run without extension
	modes := []struct {
		what string
		args string
	}{
		{"dom", `{"what":"dom"}`},
		{"dom_with_selector", `{"what":"dom","selector":"div"}`},
		{"api_validation", `{"what":"api_validation","operation":"analyze"}`},
		{"performance", `{"what":"performance"}`},
		{"link_health", `{"what":"link_health"}`},
		{"page_summary", `{"what":"page_summary","sync":false}`},
	}

	for _, tc := range modes {
		t.Run(tc.what, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("analyze(%s) PANICKED: %v", tc.what, r)
				}
			}()

			resp := callAnalyzeRaw(h, tc.args)
			if resp.Result == nil {
				t.Fatalf("analyze(%s) returned nil result", tc.what)
			}

			result := parseToolResult(t, resp)
			if len(result.Content) == 0 {
				t.Errorf("analyze(%s) should return at least one content block", tc.what)
			}

			// Verify content type is "text"
			if result.Content[0].Type != "text" {
				t.Errorf("analyze(%s) content type = %q, want 'text'", tc.what, result.Content[0].Type)
			}

			assertSnakeCaseFields(t, string(resp.Result))
		})
	}
}
