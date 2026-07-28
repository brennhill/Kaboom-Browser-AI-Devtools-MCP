// Purpose: Validate get_readable and page_summary use dedicated query types instead of execute.
// Why: Prevents regression to CSP-fragile execute-based routing (#257).
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_content_test.go — Tests get_readable/get_markdown query routing.
package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// ============================================
// Test Infrastructure (interact-content specific)
// ============================================

func newContentTestEnv(t *testing.T) *interactTestEnv {
	t.Helper()
	server, err := NewServer(t.TempDir()+"/test-content.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	cap.Extension().SetPilotEnabled(true) // content extraction requires pilot
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.tools.Executor.(*ToolHandler)

	// Simulate extension connection
	httpReq := httptest.NewRequest("POST", "/sync", strings.NewReader(`{"ext_session_id":"test"}`))
	httpReq.Header.Set("X-Kaboom-Client", "test-client")
	capture.NewSyncHandler(cap).HandleSync(httptest.NewRecorder(), httpReq)

	// Simulate tab tracking
	cap.Extension().SetTrackingStatusForTest(42, "https://example.com")

	return &interactTestEnv{handler: handler, server: server, capture: cap}
}

// ============================================
// get_readable: query type is "get_readable" (not "execute")
// ============================================

func TestGetReadable_QueryType_IsGetReadable(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"get_readable"}`)
	if !ok {
		t.Fatal("get_readable should return result")
	}
	if result.IsError {
		t.Fatalf("get_readable should not error, got: %s", firstText(result))
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_readable should create a pending query")
	}
	if pq.Type != "get_readable" {
		t.Fatalf("pending query type = %q, want get_readable", pq.Type)
	}
}

// ============================================
// get_readable: params must NOT contain a "script" field
// ============================================

func TestGetReadable_NoScriptInParams(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	env.callInteract(t, `{"what":"get_readable"}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_readable should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if _, hasScript := params["script"]; hasScript {
		t.Fatal("get_readable params must NOT contain a 'script' field")
	}
}

// ============================================
// get_readable: tab_id is forwarded
// ============================================

func TestGetReadable_ForwardsTabID(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	env.callInteract(t, `{"what":"get_readable","tab_id":99}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_readable should create a pending query")
	}
	if pq.TabID != 99 {
		t.Fatalf("pending query tab_id = %d, want 99", pq.TabID)
	}
}

// ============================================
// get_readable: default timeout_ms
// ============================================

func TestGetReadable_DefaultTimeout(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	env.callInteract(t, `{"what":"get_readable"}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_readable should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	timeoutMs, ok := params["timeout_ms"].(float64)
	if !ok {
		t.Fatal("timeout_ms should be present in params")
	}
	if timeoutMs != 10000 {
		t.Fatalf("default timeout_ms = %v, want 10000", timeoutMs)
	}
}

// ============================================
// get_readable: custom timeout_ms is forwarded
// ============================================

func TestGetReadable_CustomTimeout(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	env.callInteract(t, `{"what":"get_readable","timeout_ms":5000}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_readable should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	timeoutMs, ok := params["timeout_ms"].(float64)
	if !ok {
		t.Fatal("timeout_ms should be present in params")
	}
	if timeoutMs != 5000 {
		t.Fatalf("timeout_ms = %v, want 5000", timeoutMs)
	}
}

// ============================================
// page_summary: query type is "page_summary" (not "execute")
// ============================================

func TestPageSummary_QueryType_IsPageSummary(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	resp := callAnalyzeRaw(env.handler, `{"what":"page_summary"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page_summary should not error, got: %s", firstText(result))
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("page_summary should create a pending query")
	}
	if pq.Type != "page_summary" {
		t.Fatalf("pending query type = %q, want page_summary", pq.Type)
	}
}

// ============================================
// page_summary: params must NOT contain a "script" field
// ============================================

func TestPageSummary_NoScriptInParams(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	callAnalyzeRaw(env.handler, `{"what":"page_summary"}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("page_summary should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if _, hasScript := params["script"]; hasScript {
		t.Fatal("page_summary params must NOT contain a 'script' field")
	}
}

// ============================================
// page_summary: tab_id is forwarded
// ============================================

func TestPageSummary_ForwardsTabID(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	callAnalyzeRaw(env.handler, `{"what":"page_summary","tab_id":77}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("page_summary should create a pending query")
	}
	if pq.TabID != 77 {
		t.Fatalf("pending query tab_id = %d, want 77", pq.TabID)
	}
}

// ============================================
// page_summary: default timeout_ms
// ============================================

func TestPageSummary_DefaultTimeout(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	callAnalyzeRaw(env.handler, `{"what":"page_summary"}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("page_summary should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	timeoutMs, ok := params["timeout_ms"].(float64)
	if !ok {
		t.Fatal("timeout_ms should be present in params")
	}
	if timeoutMs != 10000 {
		t.Fatalf("default timeout_ms = %v, want 10000", timeoutMs)
	}
}

// ============================================
// page_summary: custom timeout_ms is forwarded
// ============================================

func TestPageSummary_CustomTimeout(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	callAnalyzeRaw(env.handler, `{"what":"page_summary","timeout_ms":5000}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("page_summary should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	timeoutMs, ok := params["timeout_ms"].(float64)
	if !ok {
		t.Fatal("timeout_ms should be present in params")
	}
	if timeoutMs != 5000 {
		t.Fatalf("timeout_ms = %v, want 5000", timeoutMs)
	}
}

// ============================================
// get_markdown: query type is "get_markdown" (not "execute")
// ============================================

func TestGetMarkdown_QueryType_IsGetMarkdown(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"get_markdown"}`)
	if !ok {
		t.Fatal("get_markdown should return result")
	}
	if result.IsError {
		t.Fatalf("get_markdown should not error, got: %s", firstText(result))
	}

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_markdown should create a pending query")
	}
	if pq.Type != "get_markdown" {
		t.Fatalf("pending query type = %q, want get_markdown", pq.Type)
	}
}

// ============================================
// get_markdown: params must NOT contain a "script" field
// ============================================

func TestGetMarkdown_NoScriptInParams(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	env.callInteract(t, `{"what":"get_markdown"}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_markdown should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if _, hasScript := params["script"]; hasScript {
		t.Fatal("get_markdown params must NOT contain a 'script' field")
	}
}

// ============================================
// get_markdown: tab_id is forwarded
// ============================================

func TestGetMarkdown_ForwardsTabID(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	env.callInteract(t, `{"what":"get_markdown","tab_id":88}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_markdown should create a pending query")
	}
	if pq.TabID != 88 {
		t.Fatalf("pending query tab_id = %d, want 88", pq.TabID)
	}
}

// ============================================
// get_markdown: default timeout_ms
// ============================================

func TestGetMarkdown_DefaultTimeout(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	env.callInteract(t, `{"what":"get_markdown"}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_markdown should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	timeoutMs, ok := params["timeout_ms"].(float64)
	if !ok {
		t.Fatal("timeout_ms should be present in params")
	}
	if timeoutMs != 10000 {
		t.Fatalf("default timeout_ms = %v, want 10000", timeoutMs)
	}
}

// ============================================
// get_markdown: custom timeout_ms is forwarded
// ============================================

func TestGetMarkdown_CustomTimeout(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	env.callInteract(t, `{"what":"get_markdown","timeout_ms":7000}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_markdown should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	timeoutMs, ok := params["timeout_ms"].(float64)
	if !ok {
		t.Fatal("timeout_ms should be present in params")
	}
	if timeoutMs != 7000 {
		t.Fatalf("timeout_ms = %v, want 7000", timeoutMs)
	}
}

// ============================================
// Timeout clamp: values > 30000 are clamped to 30000
// ============================================

func TestContentExtraction_TimeoutClamp(t *testing.T) {
	t.Parallel()
	env := newContentTestEnv(t)

	env.callInteract(t, `{"what":"get_readable","timeout_ms":60000}`)

	pq := env.capture.Queries().GetLastPendingQuery()
	if pq == nil {
		t.Fatal("get_readable should create a pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	timeoutMs, ok := params["timeout_ms"].(float64)
	if !ok {
		t.Fatal("timeout_ms should be present in params")
	}
	if timeoutMs != 30000 {
		t.Fatalf("timeout_ms should be clamped to 30000, got %v", timeoutMs)
	}
}
