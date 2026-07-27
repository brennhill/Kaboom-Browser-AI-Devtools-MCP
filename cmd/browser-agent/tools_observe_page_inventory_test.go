// Purpose: Validate observe(what="page_inventory") composite command.
// Why: Prevents regressions in the single-call page inventory that combines page info + interactive elements.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// tools_observe_page_inventory_test.go — Tests for observe(what="page_inventory") mode.
package main

import (
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// ============================================
// Dispatch Tests
// ============================================

func TestToolsObservePageInventory_DispatchesQuery(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, normalizeObserveArgsForAsync(`{"what":"page_inventory"}`))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page_inventory should queue successfully, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data["status"] != "queued" {
		t.Errorf("status = %v, want 'queued'", data["status"])
	}
	corr, _ := data["correlation_id"].(string)
	if !strings.HasPrefix(corr, "page_inventory_") {
		t.Errorf("correlation_id should start with 'page_inventory_', got: %s", corr)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	if pq.Type != "page_inventory" {
		t.Errorf("pending query type = %q, want 'page_inventory'", pq.Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

func TestToolsObservePageInventory_InValidModes(t *testing.T) {
	t.Parallel()

	h, _, _ := makeToolHandler(t)
	modes := strings.Join(h.observeDispatcher.ValidModes(), ", ")
	if !strings.Contains(modes, "page_inventory") {
		t.Errorf("valid observe modes should include 'page_inventory': %s", modes)
	}
}

// ============================================
// Mode Spec Tests (via describe_capabilities)
// ============================================

func TestToolsObservePageInventory_ModeSpecExists(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"describe_capabilities","tool":"observe","mode":"page_inventory"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("describe_capabilities for observe/page_inventory should succeed, got: %s", result.Content[0].Text)
	}

	data := extractResultJSON(t, result)
	if data == nil {
		t.Fatal("describe_capabilities should return non-nil data")
	}
}

// ============================================
// Schema Tests
// ============================================

func TestToolsObserveSchema_PageInventoryInWhatEnum(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	tools := h.ToolsList()
	var observeSchema map[string]any
	for _, tool := range tools {
		if tool.Name == "observe" {
			observeSchema = tool.InputSchema
			break
		}
	}
	if observeSchema == nil {
		t.Fatal("observe tool not found in ToolsList()")
	}

	props, ok := observeSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("observe schema missing properties")
	}
	whatProp, ok := props["what"].(map[string]any)
	if !ok {
		t.Fatal("observe schema missing 'what' property")
	}
	enumValues, ok := whatProp["enum"].([]string)
	if !ok {
		t.Fatal("'what' property missing enum")
	}

	found := false
	for _, v := range enumValues {
		if v == "page_inventory" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'what' enum should include 'page_inventory', got: %v", enumValues)
	}
}

// ============================================
// Response Structure
// ============================================

func TestToolsObservePageInventory_ResponseStructure(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, normalizeObserveArgsForAsync(`{"what":"page_inventory"}`))
	if resp.Result == nil {
		t.Fatal("observe(page_inventory) returned nil result")
	}

	result := parseToolResult(t, resp)
	if len(result.Content) == 0 {
		t.Error("observe(page_inventory) should return at least one content block")
	}
	if result.Content[0].Type != "text" {
		t.Errorf("content type = %q, want 'text'", result.Content[0].Type)
	}

	assertSnakeCaseFields(t, string(resp.Result))
}

// ============================================
// VisibleOnly and Limit forwarding
// ============================================

func TestToolsObservePageInventory_ForwardsParams(t *testing.T) {
	t.Parallel()
	h, _, cap := makeToolHandler(t)

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	resp := h.observeDispatcher.Handle(req, normalizeObserveArgsForAsync(`{"what":"page_inventory","visible_only":true,"limit":50}`))
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("page_inventory with params should succeed, got: %s", result.Content[0].Text)
	}

	pq := cap.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("expected pending query to be created")
	}
	// Params should be forwarded to the pending query
	if pq.Type != "page_inventory" {
		t.Errorf("pending query type = %q, want 'page_inventory'", pq.Type)
	}
}
