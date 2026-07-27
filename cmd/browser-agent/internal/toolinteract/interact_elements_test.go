// Purpose: Tests for interact element listing and discovery.
// Docs: docs/features/feature/interact-explore/index.md

// tools_interact_elements_test.go — Tests for element index store and resolution.
package toolinteract

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func TestResolveIndexToSelector_Empty(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	_, ok, _, _ := h.interactAction().resolveIndexToSelector("client-a", 0, 0, "")
	if ok {
		t.Error("expected not found on empty store")
	}
}

func TestResolveIndexToSelector_AfterBuild(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Manually populate the store
	h.interactAction().elementIndexRegistry.Store("client-a", 0, "gen_1", map[int]string{
		0: "#email",
		1: "#password",
		2: "button[type=submit]",
	})

	sel, ok, _, _ := h.interactAction().resolveIndexToSelector("client-a", 0, 1, "")
	if !ok || sel != "#password" {
		t.Errorf("expected #password, got %q (ok=%v)", sel, ok)
	}

	_, ok, _, _ = h.interactAction().resolveIndexToSelector("client-a", 0, 99, "")
	if ok {
		t.Error("expected not found for missing index")
	}
}

func TestResolveIndexToSelector_ScopedByClientAndTab(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.interactAction().elementIndexRegistry.Store("client-a", 0, "gen_a", map[int]string{1: "#a"})
	h.interactAction().elementIndexRegistry.Store("client-b", 0, "gen_b", map[int]string{1: "#b"})
	h.interactAction().elementIndexRegistry.Store("client-a", 9, "gen_a9", map[int]string{1: "#a9"})

	sel, ok, _, _ := h.interactAction().resolveIndexToSelector("client-a", 0, 1, "")
	if !ok || sel != "#a" {
		t.Fatalf("client-a/tab0 selector=%q ok=%v, want #a/true", sel, ok)
	}
	sel, ok, _, _ = h.interactAction().resolveIndexToSelector("client-b", 0, 1, "")
	if !ok || sel != "#b" {
		t.Fatalf("client-b/tab0 selector=%q ok=%v, want #b/true", sel, ok)
	}
	sel, ok, _, _ = h.interactAction().resolveIndexToSelector("client-a", 9, 1, "")
	if !ok || sel != "#a9" {
		t.Fatalf("client-a/tab9 selector=%q ok=%v, want #a9/true", sel, ok)
	}
}

func TestResolveIndexToSelector_GenerationMismatch(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.interactAction().elementIndexRegistry.Store("client-a", 0, "gen_new", map[int]string{1: "#a"})

	_, ok, stale, latest := h.interactAction().resolveIndexToSelector("client-a", 0, 1, "gen_old")
	if ok {
		t.Fatal("expected no selector on generation mismatch")
	}
	if !stale {
		t.Fatal("expected stale=true on generation mismatch")
	}
	if latest != "gen_new" {
		t.Fatalf("latest generation=%q, want gen_new", latest)
	}
}

func TestHandleDOMPrimitive_IndexGenerationMismatch(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	h.interactAction().elementIndexRegistry.Store("client-a", 7, "gen_new", map[int]string{1: "#submit"})

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), ClientID: "client-a"}
	resp := h.interactAction().HandleDOMPrimitive(req, json.RawMessage(`{"index":1,"tab_id":7,"index_generation":"gen_old"}`), "click")
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatalf("expected error response, got: %s", firstText(result))
	}
	text := strings.ToLower(firstText(result))
	if !strings.Contains(text, "index_generation") || !strings.Contains(text, "mismatch") {
		t.Fatalf("expected index_generation mismatch guidance, got: %s", firstText(result))
	}
}

func TestBuildElementIndexFromResponse_ValidElements(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Simulate a list_interactive response with elements in the JSON
	elemData := map[string]any{
		"elements": []any{
			map[string]any{"index": float64(0), "selector": "#name", "tag": "input"},
			map[string]any{"index": float64(1), "selector": ".btn-submit", "tag": "button"},
			map[string]any{"index": float64(2), "selector": "", "tag": "div"}, // empty selector, should be skipped
		},
	}
	elemJSON, _ := json.Marshal(elemData)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "list_interactive results\n" + string(elemJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.interactAction().buildElementIndexFromResponse("client-a", 0, "gen_1", resp)

	sel, ok, _, _ := h.interactAction().resolveIndexToSelector("client-a", 0, 0, "")
	if !ok || sel != "#name" {
		t.Errorf("index 0: expected #name, got %q (ok=%v)", sel, ok)
	}

	sel, ok, _, _ = h.interactAction().resolveIndexToSelector("client-a", 0, 1, "")
	if !ok || sel != ".btn-submit" {
		t.Errorf("index 1: expected .btn-submit, got %q (ok=%v)", sel, ok)
	}

	// Index 2 had empty selector, should not be stored
	_, ok, _, _ = h.interactAction().resolveIndexToSelector("client-a", 0, 2, "")
	if ok {
		t.Error("index 2 with empty selector should not be stored")
	}
}

func TestBuildElementIndexFromResponse_NestedResult(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Elements nested under result.result.elements
	nestedData := map[string]any{
		"result": map[string]any{
			"result": map[string]any{
				"elements": []any{
					map[string]any{"index": float64(0), "selector": "a.link"},
				},
			},
		},
	}
	nestedJSON, _ := json.Marshal(nestedData)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: string(nestedJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.interactAction().buildElementIndexFromResponse("client-a", 0, "gen_1", resp)

	sel, ok, _, _ := h.interactAction().resolveIndexToSelector("client-a", 0, 0, "")
	if !ok || sel != "a.link" {
		t.Errorf("expected a.link from nested result, got %q (ok=%v)", sel, ok)
	}
}

func TestBuildElementIndexFromResponse_ErrorResponse(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	// Pre-populate store
	h.interactAction().elementIndexRegistry.Store("client-a", 0, "gen_1", map[int]string{0: "old"})

	// Error response should not clear the store
	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "error"}},
		IsError: true,
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	h.interactAction().buildElementIndexFromResponse("client-a", 0, "gen_2", resp)

	sel, ok, _, _ := h.interactAction().resolveIndexToSelector("client-a", 0, 0, "")
	if !ok || sel != "old" {
		t.Errorf("error response should not clear store, got %q (ok=%v)", sel, ok)
	}
}

func TestExtractElementList_Direct(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"elements": []any{
			map[string]any{"index": float64(0), "selector": "#a"},
		},
	}
	elems := extractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestExtractElementList_Nested(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"result": map[string]any{
			"elements": []any{
				map[string]any{"index": float64(0), "selector": "#b"},
			},
		},
	}
	elems := extractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element from nested, got %d", len(elems))
	}
}

func TestExtractElementList_NoElements(t *testing.T) {
	t.Parallel()
	data := map[string]any{"foo": "bar"}
	elems := extractElementList(data)
	if elems != nil {
		t.Error("expected nil for data without elements")
	}
}

// ============================================
// List Interactive Limit/Truncation Tests
// ============================================

func TestTruncateListInteractiveResponse_Basic(t *testing.T) {
	t.Parallel()

	elements := make([]any, 20)
	for i := range elements {
		elements[i] = map[string]any{"index": float64(i), "selector": "#el-" + string(rune('a'+i)), "tag": "div"}
	}
	elemData := map[string]any{"elements": elements}
	elemJSON, _ := json.Marshal(elemData)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "list_interactive results\n" + string(elemJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	truncated := truncateListInteractiveResponse(resp, 5)

	// Parse truncated response
	var truncResult mcp.MCPToolResult
	if err := json.Unmarshal(truncated.Result, &truncResult); err != nil {
		t.Fatal(err)
	}

	text := truncResult.Content[0].Text
	idx := 0
	for i, ch := range text {
		if ch == '{' {
			idx = i
			break
		}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse truncated JSON: %v", err)
	}

	elems, ok := data["elements"].([]any)
	if !ok {
		t.Fatal("elements not found in truncated response")
	}
	if len(elems) != 5 {
		t.Errorf("expected 5 elements, got %d", len(elems))
	}
	if data["total"] != float64(20) {
		t.Errorf("total = %v, want 20", data["total"])
	}
	if data["truncated"] != true {
		t.Errorf("truncated = %v, want true", data["truncated"])
	}
}

func TestTruncateListInteractiveResponse_NoTruncationNeeded(t *testing.T) {
	t.Parallel()

	elemData := map[string]any{
		"elements": []any{
			map[string]any{"index": float64(0), "selector": "#a"},
			map[string]any{"index": float64(1), "selector": "#b"},
		},
	}
	elemJSON, _ := json.Marshal(elemData)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: string(elemJSON)}},
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	truncated := truncateListInteractiveResponse(resp, 10)

	// Should be unchanged
	if string(truncated.Result) != string(resp.Result) {
		t.Error("response should be unchanged when limit > element count")
	}
}

func TestTruncateListInteractiveResponse_ErrorResponse(t *testing.T) {
	t.Parallel()

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "error"}},
		IsError: true,
	}
	resultJSON, _ := json.Marshal(result)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", Result: resultJSON}

	truncated := truncateListInteractiveResponse(resp, 5)
	if string(truncated.Result) != string(resp.Result) {
		t.Error("error response should be unchanged")
	}
}
