// Purpose: Validate describe_capabilities handler behavior and guard against regressions.
// Docs: docs/features/describe_capabilities.md

// tools_configure_capabilities_test.go — Tests for describe_capabilities handler.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/configurehandler"
)

func TestDescribeCapabilities_ResponseStructure(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := configurehandler.HandleDescribeCapabilities(h, req, json.RawMessage(`{}`), version)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal MCPToolResult: %v", err)
	}
	if result.IsError {
		t.Fatal("expected non-error response")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}

	text := result.Content[0].Text

	// Must contain the 5 core tools
	for _, tool := range []string{"observe", "generate", "configure", "interact", "analyze"} {
		if !strings.Contains(text, `"`+tool+`"`) {
			t.Errorf("expected tool %q in capabilities", tool)
		}
	}

	// Must contain version and protocol_version
	if !strings.Contains(text, `"version"`) {
		t.Error("expected version field")
	}
	if !strings.Contains(text, `"protocol_version"`) {
		t.Error("expected protocol_version field")
	}

	// Parse deeper to check structure
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("no JSON in response")
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse capabilities JSON: %v", err)
	}

	tools, ok := data["tools"].(map[string]any)
	if !ok {
		t.Fatal("expected tools map")
	}

	// Each tool should have dispatch_param, modes, params, description
	for name, toolData := range tools {
		td, ok := toolData.(map[string]any)
		if !ok {
			t.Errorf("tool %s: expected map", name)
			continue
		}
		if _, ok := td["dispatch_param"]; !ok {
			t.Errorf("tool %s: missing dispatch_param", name)
		}
		if _, ok := td["description"]; !ok {
			t.Errorf("tool %s: missing description", name)
		}
		if _, ok := td["params"]; !ok {
			t.Errorf("tool %s: missing params", name)
		}
	}
}

func TestDescribeCapabilities_ToolsHaveModes(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := configurehandler.HandleDescribeCapabilities(h, req, json.RawMessage(`{}`), version)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	tools := data["tools"].(map[string]any)

	// observe tool should have modes (what enum)
	observeTool := tools["observe"].(map[string]any)
	modes, ok := observeTool["modes"].([]any)
	if !ok || len(modes) == 0 {
		t.Error("observe tool should have non-empty modes list")
	}

	// Check that modes include known values
	modeSet := make(map[string]bool)
	for _, m := range modes {
		modeSet[m.(string)] = true
	}
	if !modeSet["errors"] {
		t.Error("observe modes should include 'errors'")
	}
	if !modeSet["logs"] {
		t.Error("observe modes should include 'logs'")
	}
}

func TestDescribeCapabilities_SummaryMode(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}

	// Full response
	fullResp := configurehandler.HandleDescribeCapabilities(h, req, json.RawMessage(`{}`), version)
	// Summary response
	summaryResp := configurehandler.HandleDescribeCapabilities(h, req, json.RawMessage(`{"summary":true}`), version)

	// Summary should be significantly smaller
	if len(summaryResp.Result) >= len(fullResp.Result) {
		t.Errorf("summary (%d bytes) should be smaller than full (%d bytes)",
			len(summaryResp.Result), len(fullResp.Result))
	}

	// Parse summary and verify it has the right structure
	var result MCPToolResult
	if err := json.Unmarshal(summaryResp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}

	tools := data["tools"].(map[string]any)
	for name, toolData := range tools {
		td := toolData.(map[string]any)
		// Summary should have description, dispatch_param, modes
		if _, ok := td["description"]; !ok {
			t.Errorf("summary tool %s: missing description", name)
		}
		if _, ok := td["dispatch_param"]; !ok {
			t.Errorf("summary tool %s: missing dispatch_param", name)
		}
		// Summary should NOT have params, param_details, mode_params
		if _, ok := td["params"]; ok {
			t.Errorf("summary tool %s: should not have params", name)
		}
		if _, ok := td["param_details"]; ok {
			t.Errorf("summary tool %s: should not have param_details", name)
		}
		if _, ok := td["mode_params"]; ok {
			t.Errorf("summary tool %s: should not have mode_params", name)
		}
	}
}

func TestDescribeCapabilities_ConfigureIncludesModeParameterDetails(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := configurehandler.HandleDescribeCapabilities(h, req, json.RawMessage(`{}`), version)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("no JSON in response")
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse capabilities JSON: %v", err)
	}

	tools := data["tools"].(map[string]any)
	configureTool := tools["configure"].(map[string]any)

	modeParamsRaw, ok := configureTool["mode_params"]
	if !ok {
		t.Fatal("configure capabilities should include mode_params")
	}
	modeParams := modeParamsRaw.(map[string]any)
	storeMode := modeParams["store"].(map[string]any)

	params := storeMode["params"].(map[string]any)
	namespaceMeta := params["namespace"].(map[string]any)
	if namespaceMeta["type"] != "string" {
		t.Fatalf("store.namespace type = %v, want string", namespaceMeta["type"])
	}

	storeActionMeta := params["store_action"].(map[string]any)
	if storeActionMeta["default"] != "list" {
		t.Fatalf("store.store_action default = %v, want list", storeActionMeta["default"])
	}
}

// parseCapabilitiesJSON is a test helper that extracts the JSON payload from a capabilities response.
func parseCapabilitiesJSON(t *testing.T, resp JSONRPCResponse) map[string]any {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal MCPToolResult: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected non-error response, got error: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("no JSON in response")
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("parse capabilities JSON: %v", err)
	}
	return data
}

func TestDescribeCapabilities_FilterByTool(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := configurehandler.HandleDescribeCapabilities(h, req, json.RawMessage(`{"tool":"observe"}`), version)

	data := parseCapabilitiesJSON(t, resp)

	tools, ok := data["tools"].(map[string]any)
	if !ok {
		t.Fatal("expected tools map")
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	observeTool, ok := tools["observe"].(map[string]any)
	if !ok {
		t.Fatal("expected observe tool entry")
	}
	// Full detail should include dispatch_param, modes, params, param_details, mode_params
	for _, field := range []string{"dispatch_param", "modes", "params", "param_details", "mode_params", "description"} {
		if _, ok := observeTool[field]; !ok {
			t.Errorf("observe tool missing field %q", field)
		}
	}
}

func TestDescribeCapabilities_FilterByToolAndMode(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := configurehandler.HandleDescribeCapabilities(h, req, json.RawMessage(`{"tool":"configure","mode":"store"}`), version)

	data := parseCapabilitiesJSON(t, resp)

	// Should have tool and mode at top level
	if data["tool"] != "configure" {
		t.Errorf("expected tool=configure, got %v", data["tool"])
	}
	if data["mode"] != "store" {
		t.Errorf("expected mode=store, got %v", data["mode"])
	}

	// Should have params with store-specific params
	params, ok := data["params"].(map[string]any)
	if !ok {
		t.Fatal("expected params map")
	}
	if _, ok := params["store_action"]; !ok {
		t.Error("expected store_action in params")
	}
	if _, ok := params["namespace"]; !ok {
		t.Error("expected namespace in params")
	}
}

func TestDescribeCapabilities_FilterByTool_Unknown(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := configurehandler.HandleDescribeCapabilities(h, req, json.RawMessage(`{"tool":"nonexistent"}`), version)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error response for unknown tool")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "nonexistent") {
		t.Error("error should mention the unknown tool name")
	}
	// Should list valid tool names
	for _, name := range []string{"observe", "configure", "interact"} {
		if !strings.Contains(text, name) {
			t.Errorf("error should list valid tool %q", name)
		}
	}
}

func TestDescribeCapabilities_ModeWithoutTool(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := configurehandler.HandleDescribeCapabilities(h, req, json.RawMessage(`{"mode":"store"}`), version)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when mode is set without tool")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "tool") {
		t.Error("error should mention that tool is required")
	}
}

func TestDescribeCapabilities_FilterByToolAndMode_Unknown(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := configurehandler.HandleDescribeCapabilities(h, req, json.RawMessage(`{"tool":"configure","mode":"nonexistent"}`), version)

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for unknown mode")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "nonexistent") {
		t.Error("error should mention the unknown mode name")
	}
	// Should list valid modes
	if !strings.Contains(text, "store") {
		t.Error("error should list valid mode 'store'")
	}
}
